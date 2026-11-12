package datatable

// Session Registry's Resubmit as Pending (JM-185, 2026-11-12): copies the runs
// of the selected sessions into new sessions with status pending, and starts
// nothing. The use case is re-processing: put the runs in pending, delete the
// sessions they replace, then start them with Check Pending on Pipeline Status.
// Starting them here would let a run read the sessions about to be deleted.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"
)

// ResubmitPipelineStmt copies the runs of session $3 into session $1 with status
// pending, for user $2 and workspace $4. resubmit_pipeline (jets/apiserver) and
// ResubmitSessionsAsPending both use it. The workspace binding is taken as it
// stands now rather than copied from the run being resubmitted: a resubmission
// runs under today's compiled workspace, and copying the old row's version would
// assert the opposite. merged_input_registry_keys is not copied: the merged
// inputs are resolved when the run starts.
const ResubmitPipelineStmt = `INSERT INTO jetsapi.pipeline_execution_status (
		pipeline_config_key, main_input_registry_key, main_input_file_key,
		client, process_name, main_object_type, input_session_id, request_id, session_id, source_period_key, status, user_email,
		workspace_name, workspace_version)
	(SELECT
		pipeline_config_key, main_input_registry_key, main_input_file_key,
		client, process_name, main_object_type, input_session_id, request_id, $1, source_period_key, 'pending', $2,
		NULLIF($4, ''), (SELECT MAX(version) FROM jetsapi.workspace_version)
	FROM jetsapi.pipeline_execution_status WHERE session_id = $3 )`

// ResubmitSessionsAsPending is resubmit_sessions_as_pending. It reads the
// selected session ids as delete_sessions does (session_id on each data row, a
// string or a list) and, in one transaction, copies each session's own runs
// (pipeline_execution_status rows whose session_id is the session) into a new
// session with status pending. It answers
//
//	{"resubmitted": n, "sessions": [new ids], "no_run": [ids], "already_pending": [ids]}
//
// A session with no run of its own (a file's session) is listed in no_run and
// skipped. A session whose run already has a pending copy (same pipeline
// configuration and main input) is listed in already_pending and skipped, so a
// second press does not queue the work twice. Anything else that stops one
// session stops all of them: a run whose main input was purged (400, as
// resubmit_pipeline), or an input session queued for deletion or deleted (409,
// GuardResubmission). The caller has checked run_pipelines.
func (ctx *DataTableContext) ResubmitSessionsAsPending(action *DataTableAction, user, workspace string) (*map[string]any, int, error) {
	if user == "" {
		return nil, http.StatusUnauthorized, errors.New("error: no user in the request's token")
	}
	ids, err := sessionIDsOf(action.Data)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)

	bg := context.Background()
	tx, err := ctx.Dbpool.Begin(bg)
	if err != nil {
		log.Printf("while starting resubmit_sessions_as_pending's transaction: %v", err)
		return nil, http.StatusInternalServerError, errors.New("error while resubmitting the sessions")
	}
	defer tx.Rollback(context.Background())

	noRun := []string{}
	alreadyPending := []string{}
	created := []string{}
	baseSessionId := time.Now().UnixMilli()
	for _, id := range ids {
		var runs int
		var pending bool
		err = tx.QueryRow(bg, `
			SELECT count(*),
				EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status c
					JOIN jetsapi.pipeline_execution_status p
						ON p.pipeline_config_key = c.pipeline_config_key
						AND p.main_input_registry_key IS NOT DISTINCT FROM c.main_input_registry_key
					WHERE p.session_id = $1 AND c.session_id <> $1 AND c.status = 'pending')
			FROM jetsapi.pipeline_execution_status WHERE session_id = $1`, id).Scan(&runs, &pending)
		if err != nil {
			log.Printf("resubmit_sessions_as_pending: reading the runs of %s: %v", id, err)
			return nil, http.StatusInternalServerError, fmt.Errorf("error while reading the runs of session %s", id)
		}
		switch {
		case runs == 0:
			noRun = append(noRun, id)
			continue
		case pending:
			alreadyPending = append(alreadyPending, id)
			continue
		}
		if err = CheckResubmittable(bg, ctx.Dbpool, id); err != nil {
			return nil, http.StatusBadRequest, err
		}
		if code, err := GuardResubmission(bg, tx, id); err != nil {
			return nil, code, fmt.Errorf("session %s: %w", id, err)
		}
		newId, err := reserveSessionId(ctx.Dbpool, &baseSessionId)
		if err != nil {
			return nil, http.StatusInternalServerError, err
		}
		if _, err = tx.Exec(bg, ResubmitPipelineStmt, newId, user, id, workspace); err != nil {
			log.Printf("resubmit_sessions_as_pending: copying the runs of %s: %v", id, err)
			return nil, http.StatusInternalServerError, fmt.Errorf("error while resubmitting session %s", id)
		}
		created = append(created, newId)
	}
	if err = tx.Commit(bg); err != nil {
		log.Printf("while committing resubmit_sessions_as_pending: %v", err)
		return nil, http.StatusInternalServerError, errors.New("error while resubmitting the sessions")
	}
	log.Printf("resubmit_sessions_as_pending: %s resubmitted %d of %d sessions as pending (%s); no run: %v; already pending: %v",
		user, len(created), len(ids), strings.Join(created, ", "), noRun, alreadyPending)
	return &map[string]any{
		"resubmitted":     len(created),
		"sessions":        created,
		"no_run":          noRun,
		"already_pending": alreadyPending,
	}, http.StatusOK, nil
}
