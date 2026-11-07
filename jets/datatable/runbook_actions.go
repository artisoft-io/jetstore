package datatable

// The operations behind /runbook (plan §3.4): reading runbooks and runs, and a
// person's controls on a run. Project runbooks, Phase 1, track AB.
//
// Every control is one transaction that locks the run's row, checks the run's
// state and the step the request names, records the decision on the step row
// and in the audit chain, and moves the run on; the executor then picks it up.
// The actor is always the authenticated user the caller passes, never a field
// of the request.
//
// # The controls on a paused run
//
// The step a control acts on is the run's frontier: the step that is not final,
// else the last one recorded (the screen's "current step"). A request that
// names a step_key other than the frontier's is refused 409, so a control sent
// from a stale screen does nothing.
//
//   - accept: a step awaiting confirmation becomes pending with the decision,
//     and runs as the person who accepted it, exactly as shown.
//   - deny: the step is denied, the decision recorded, and the run stays
//     paused.
//   - continue: a pause step is completed. After a denied or failed step the
//     run goes on to the next step and the denied or failed record stays as it
//     is: **continue after a deny skips the denied step, and the denial stays in
//     the record.** Refused for a step awaiting confirmation (accept or deny
//     it), a manual step (record it or skip it) and an interrupted step (retry
//     it or skip it, since it may have taken effect).
//   - skip: a step awaiting a person, waiting or interrupted is marked skipped,
//     and the run goes on; after a denied or failed step it is continue.
//   - retry: a denied, failed or interrupted step is recorded again, as a new
//     row, and runs again — a ddl stops for confirmation again, with its
//     statement bound and previewed afresh. **After a deny, retry asks again.**
//   - abort: the run ends aborted; the running statement is cancelled; every
//     step not yet final, and every step not yet reached, is recorded skipped.
//
// **Continue, skip and retry reload the runbook from the workspace** (§4.4):
// when a file changed, the runbook must validate, every step that has run must
// be unchanged (its definition and its SQL file's hash), and a new revision is
// recorded, from the step that runs next. Accept does not reload: it runs what
// the person was shown.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var errRunbookWhileCompile = rbErr(http.StatusConflict,
	"a compile of this workspace is in progress; start the runbook when it has finished")

// runbookEnvironment is what a run records it ran against: the JetStore
// release and the workspace version, as the apiserver's start reads them.
func (e *RunbookExecutor) runbookEnvironment(ctx context.Context, workspaceName string) runbook.Environment {
	var env runbook.Environment
	var release, version *string
	if err := e.pool.QueryRow(ctx, `SELECT max(version) FROM jetsapi.jetstore_release`).Scan(&release); err == nil && release != nil {
		env.JetstoreRelease = *release
	}
	if env.JetstoreRelease == "" {
		env.JetstoreRelease = os.Getenv("JETS_VERSION")
	}
	if err := e.pool.QueryRow(ctx, `SELECT max(version) FROM jetsapi.workspace_version
		WHERE workspace_name = $1 OR workspace_name IS NULL`, workspaceName).Scan(&version); err == nil && version != nil {
		env.WorkspaceVersion = *version
	}
	return env
}

// --- reads --------------------------------------------------------------------

// ListRunbooks is the active workspace's runbooks, each validated, with its
// last run.
func (e *RunbookExecutor) ListRunbooks(ctx context.Context, workspaceName string) (*map[string]any, error) {
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return nil, &RunbookError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	names, err := listRunbookNames(workspaceName)
	if err != nil {
		return nil, err
	}
	last := map[string]map[string]any{}
	rows, err := e.pool.Query(ctx, `SELECT DISTINCT ON (runbook) runbook, key, status, started_at
		FROM jetsapi.runbook_run WHERE workspace_name = $1 ORDER BY runbook, key DESC`, workspaceName)
	if err != nil {
		return nil, fmt.Errorf("while reading the last runs: %v", err)
	}
	for rows.Next() {
		var name, status string
		var key int64
		var started time.Time
		if err = rows.Scan(&name, &key, &status, &started); err != nil {
			rows.Close()
			return nil, err
		}
		last[name] = map[string]any{"run_key": key, "status": status, "started_at": started.UTC().Format(time.RFC3339Nano)}
	}
	rows.Close()
	out := make([]map[string]any, 0, len(names))
	for _, n := range names {
		entry := map[string]any{"name": n, "description": "", "valid": false, "findings": []wsvalidate.Finding{}}
		lr, err := loadRunbookFromDisk(workspaceName, n)
		if err != nil {
			entry["findings"] = []wsvalidate.Finding{{Severity: wsvalidate.Error, Code: runbook.CodeName, Message: err.Error()}}
		} else {
			entry["valid"] = lr.Valid()
			entry["findings"] = lr.Findings
			if lr.Runbook != nil {
				entry["description"] = lr.Runbook.Description
			}
		}
		if l, ok := last[n]; ok {
			entry["last_run"] = l
		} else {
			entry["last_run"] = nil
		}
		out = append(out, entry)
	}
	return &map[string]any{"runbooks": out}, nil
}

// GetRunbook is one runbook decoded, its findings and its SQL files' text.
func (e *RunbookExecutor) GetRunbook(ctx context.Context, workspaceName, name string) (*map[string]any, error) {
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return nil, &RunbookError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	lr, err := loadRunbookFromDisk(workspaceName, name)
	if err != nil {
		return nil, err
	}
	sqlFiles := map[string]string{}
	for p, c := range lr.Files {
		if p != name+runbook.FileSuffix {
			sqlFiles[p] = c
		}
	}
	var rb any
	if lr.Runbook != nil {
		rb = lr.Runbook
	}
	return &map[string]any{"runbook": rb, "findings": lr.Findings, "sql_files": sqlFiles, "valid": lr.Valid()}, nil
}

// ListRuns is a workspace's runs, newest first, without their steps.
func (e *RunbookExecutor) ListRuns(ctx context.Context, workspaceName, name string, limit int) (*map[string]any, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	q := `SELECT ` + rbRunColumns + ` FROM jetsapi.runbook_run WHERE workspace_name = $1`
	args := []any{workspaceName}
	if name != "" {
		q += ` AND runbook = $2`
		args = append(args, name)
	}
	q += ` ORDER BY key DESC LIMIT ` + strconv.Itoa(limit)
	rows, err := e.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("while listing runs: %v", err)
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, runView(r, nil))
	}
	return &map[string]any{"runs": out}, rows.Err()
}

// GetRun is a run with every step row.
func (e *RunbookExecutor) GetRun(ctx context.Context, key int64) (*map[string]any, error) {
	run, err := loadRun(ctx, e.pool, key, false)
	if err != nil {
		return nil, err
	}
	steps, err := loadSteps(ctx, e.pool, key)
	if err != nil {
		return nil, err
	}
	if steps == nil {
		steps = []*rbStep{}
	}
	return &map[string]any{"run": runView(run, steps)}, nil
}

// --- start ----------------------------------------------------------------------

// StartRun starts a run of a runbook of the active workspace: refused while a
// compile of the workspace is in progress, and while another run of the same
// runbook is running or paused.
func (e *RunbookExecutor) StartRun(ctx context.Context, actor, workspaceName, name string,
	given map[string]json.RawMessage) (*map[string]any, error) {
	if err := activeWorkspaceRefusal(workspaceName); err != nil {
		return nil, &RunbookError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	lr, err := loadRunbookFromDisk(workspaceName, name)
	if err != nil {
		return nil, err
	}
	if !lr.Valid() {
		return nil, lr.refusal()
	}
	vals, err := runbook.ParameterValues(lr.Runbook, given)
	if err != nil {
		return nil, rbErr(http.StatusBadRequest, "%v", err)
	}
	params, err := paramsJSON(vals)
	if err != nil {
		return nil, err
	}
	env := e.runbookEnvironment(ctx, workspaceName)
	first := ""
	if len(lr.Runbook.Steps) > 0 {
		first = lr.Runbook.Steps[0].ID
	}
	run := &rbRun{
		WorkspaceName: workspaceName, Runbook: name, Status: runbook.RunRunning, Parameters: params,
		Revisions:     []runbook.Revision{{Revision: 1, Files: lr.Hashes(), FromStep: first}},
		RevisionFiles: map[string]map[string]string{"1": lr.Files},
		Environment:   env, StartedBy: actor, ActingUser: actor,
	}
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if err = lockWorkspaceExclusion(ctx, tx, workspaceName); err != nil {
		return nil, err
	}
	var compiling, active int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM jetsapi.workspace_registry
		WHERE workspace_name = $1 AND status = ANY($2)`, workspaceName, compileInProgressStatuses).Scan(&compiling); err != nil {
		return nil, fmt.Errorf("while checking for a compile of workspace %s: %v", workspaceName, err)
	}
	if compiling > 0 {
		return nil, errRunbookWhileCompile
	}
	var activeKey int64
	err = tx.QueryRow(ctx, `SELECT count(*), coalesce(max(key), 0) FROM jetsapi.runbook_run
		WHERE workspace_name = $1 AND runbook = $2 AND status IN ('running', 'paused')`,
		workspaceName, name).Scan(&active, &activeKey)
	if err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, rbErr(http.StatusConflict,
			"run %d of runbook %s is not finished; continue it or abort it before starting another", activeKey, name)
	}
	revs, _ := json.Marshal(run.Revisions)
	files, _ := json.Marshal(run.RevisionFiles)
	pj, _ := json.Marshal(run.Parameters)
	ej, _ := json.Marshal(run.Environment)
	err = tx.QueryRow(ctx, `INSERT INTO jetsapi.runbook_run (workspace_name, runbook, status, parameters, revisions,
		revision_files, environment, started_by, acting_user)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8, $8) RETURNING key, started_at, updated_at`,
		workspaceName, name, run.Status, string(pj), string(revs), string(files), string(ej), actor).
		Scan(&run.Key, &run.StartedAt, &run.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, rbErr(http.StatusConflict, "another run of runbook %s has just started", name)
		}
		return nil, fmt.Errorf("while recording the run: %v", err)
	}
	if err = auditStep(ctx, tx, run, nil, audit.EventIntent, "run_started", actor, ""); err != nil {
		return nil, err
	}
	if _, err = e.advance(ctx, tx, run, lr, nil, actor); err != nil {
		return nil, err
	}
	if err = saveRun(ctx, tx, run); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	e.Wake()
	return e.GetRun(ctx, run.Key)
}

func paramsJSON(vals map[string]runbook.Value) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(vals))
	for k, v := range vals {
		b, err := json.Marshal(jsonSafe(v.V))
		if err != nil {
			return nil, err
		}
		out[k] = b
	}
	return out, nil
}

// --- controls -------------------------------------------------------------------

// control runs fn on a run in one transaction holding its row lock, then
// saves the run, wakes the executor and answers the run as it now stands.
func (e *RunbookExecutor) control(ctx context.Context, key int64, fn func(tx pgx.Tx, run *rbRun, steps []*rbStep) error) (*map[string]any, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	run, err := loadRun(ctx, tx, key, true)
	if err != nil {
		return nil, err
	}
	if err = activeWorkspaceRefusal(run.WorkspaceName); err != nil {
		return nil, &RunbookError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	steps, err := loadSteps(ctx, tx, key)
	if err != nil {
		return nil, err
	}
	if err = fn(tx, run, steps); err != nil {
		return nil, err
	}
	if err = saveRun(ctx, tx, run); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	e.Wake()
	return e.GetRun(ctx, key)
}

// frontierFor is the frontier step, refused when the request names another.
func frontierFor(steps []*rbStep, stepKey *int64) (*rbStep, error) {
	f := frontierStep(steps)
	if f == nil {
		return nil, rbErr(http.StatusConflict, "the run has no step yet")
	}
	if stepKey != nil && *stepKey != f.Key {
		return nil, rbErr(http.StatusConflict,
			"step %d is not the run's current step (%s, step %d); reload the run", *stepKey, f.StepID, f.Key)
	}
	return f, nil
}

func requirePaused(run *rbRun) error {
	if run.Status != runbook.RunPaused {
		return rbErr(http.StatusConflict, "run %d is %s, not paused", run.Key, run.Status)
	}
	return nil
}

// decisionSQL is a step row's decision as jsonb, timed by the database.
const decisionSQL = `jsonb_build_object('by', $DBY::text, 'at', now(), 'accepted', $DAC::boolean, 'comment', $DCM::text)`

// Accept accepts the step awaiting confirmation.
func (e *RunbookExecutor) Accept(ctx context.Context, actor string, key, stepKey int64, comment string) (*map[string]any, error) {
	return e.control(ctx, key, func(tx pgx.Tx, run *rbRun, steps []*rbStep) error {
		f, err := e.awaitingConfirmation(run, steps, stepKey)
		if err != nil {
			return err
		}
		// Refused now rather than when it runs, where it is checked again.
		if err = e.checkStepCapability(ctx, actor, runbook.StepType(f.Type)); err != nil {
			return rbErr(http.StatusForbidden, "refused: %v", err)
		}
		if err = setStep(ctx, tx, f.Key, `status = $1, decision = `+decisionArgs(2), []any{runbook.StepPending, actor, true, comment},
			actor, "accepted", comment); err != nil {
			return err
		}
		f.Status = runbook.StepPending
		if err = auditStep(ctx, tx, run, f, audit.EventDecision, "accepted", actor, comment); err != nil {
			return err
		}
		run.Status, run.ActingUser, run.LastError = runbook.RunRunning, actor, nil
		return nil
	})
}

// Deny denies the step awaiting confirmation; the run stays paused.
func (e *RunbookExecutor) Deny(ctx context.Context, actor string, key, stepKey int64, comment string) (*map[string]any, error) {
	return e.control(ctx, key, func(tx pgx.Tx, run *rbRun, steps []*rbStep) error {
		f, err := e.awaitingConfirmation(run, steps, stepKey)
		if err != nil {
			return err
		}
		if err = setStep(ctx, tx, f.Key, `status = $1, finished_at = now(), decision = `+decisionArgs(2),
			[]any{runbook.StepDenied, actor, false, comment}, actor, "denied", comment); err != nil {
			return err
		}
		f.Status = runbook.StepDenied
		if err = auditStep(ctx, tx, run, f, audit.EventDecision, "denied", actor, comment); err != nil {
			return err
		}
		msg := fmt.Sprintf("%s: denied by %s", f.StepID, actor)
		run.LastError = &msg
		return nil
	})
}

func decisionArgs(first int) string {
	s := decisionSQL
	for i, p := range []string{"$DBY", "$DAC", "$DCM"} {
		s = strings.ReplaceAll(s, p, "$"+strconv.Itoa(first+i))
	}
	return s
}

func (e *RunbookExecutor) awaitingConfirmation(run *rbRun, steps []*rbStep, stepKey int64) (*rbStep, error) {
	if err := requirePaused(run); err != nil {
		return nil, err
	}
	f, err := frontierFor(steps, &stepKey)
	if err != nil {
		return nil, err
	}
	if f.Status != runbook.StepAwaitingConfirmation {
		return nil, rbErr(http.StatusConflict, "step %s is %s, not awaiting confirmation", f.StepID, f.Status)
	}
	return f, nil
}

// Record records a manual step's declared values and goes on.
func (e *RunbookExecutor) Record(ctx context.Context, actor string, key, stepKey int64,
	values map[string]json.RawMessage) (*map[string]any, error) {
	return e.control(ctx, key, func(tx pgx.Tx, run *rbRun, steps []*rbStep) error {
		if err := requirePaused(run); err != nil {
			return err
		}
		f, err := frontierFor(steps, &stepKey)
		if err != nil {
			return err
		}
		if f.Type != string(runbook.StepManual) || f.Status != runbook.StepAwaitingInput {
			return rbErr(http.StatusConflict, "step %s is a %s step and %s; only a manual step awaiting input takes records",
				f.StepID, f.Type, f.Status)
		}
		lr, err := run.pinned()
		if err != nil {
			return err
		}
		step := lr.Runbook.Step(f.StepID)
		if step == nil {
			return rbErr(http.StatusConflict, "step %s is not in the runbook", f.StepID)
		}
		outs := map[string]runbook.Value{}
		for name := range values {
			if _, ok := step.Records[name]; !ok {
				return rbErr(http.StatusBadRequest, "step %s does not record %q", f.StepID, name)
			}
		}
		for name, ts := range step.Records {
			raw, ok := values[name]
			if !ok {
				return rbErr(http.StatusBadRequest, "step %s needs a value for %q", f.StepID, name)
			}
			t, err := runbook.ParseType(ts)
			if err != nil {
				return err
			}
			v, err := runbook.ParseValue(t, raw)
			if err != nil {
				return rbErr(http.StatusBadRequest, "%s: %v", name, err)
			}
			outs[name] = v
		}
		b, types, err := valuesJSON(outs)
		if err != nil {
			return err
		}
		tb, _ := json.Marshal(types)
		if err = setStep(ctx, tx, f.Key, `status = $1, finished_at = now(), actor = $2, outputs = $3::jsonb,
			output_types = $4::jsonb, decision = `+decisionArgs(5),
			[]any{runbook.StepCompleted, actor, string(b), string(tb), actor, true, ""}, actor, "recorded", ""); err != nil {
			return err
		}
		f.Status = runbook.StepCompleted
		f.Outputs, f.OutputTypes = b, types
		if err = auditStep(ctx, tx, run, f, audit.EventDecision, "recorded", actor, ""); err != nil {
			return err
		}
		run.Status, run.ActingUser, run.LastError = runbook.RunRunning, actor, nil
		_, err = e.advance(ctx, tx, run, lr, steps, actor)
		return err
	})
}

// Continue, Skip and Retry act on a paused run's current step; see the file
// comment.
func (e *RunbookExecutor) Continue(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error) {
	return e.resume(ctx, "continue", actor, key, stepKey)
}

func (e *RunbookExecutor) Skip(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error) {
	return e.resume(ctx, "skip", actor, key, stepKey)
}

func (e *RunbookExecutor) Retry(ctx context.Context, actor string, key int64, stepKey *int64) (*map[string]any, error) {
	return e.resume(ctx, "retry", actor, key, stepKey)
}

func (e *RunbookExecutor) resume(ctx context.Context, action, actor string, key int64, stepKey *int64) (*map[string]any, error) {
	return e.control(ctx, key, func(tx pgx.Tx, run *rbRun, steps []*rbStep) error {
		f, err := frontierFor(steps, stepKey)
		if err != nil {
			return err
		}
		// Skip may also take a wait out of a running run.
		if !(run.Status == runbook.RunPaused ||
			(action == "skip" && run.Status == runbook.RunRunning && f.Status == runbook.StepWaiting)) {
			return rbErr(http.StatusConflict, "run %d is %s, not paused", run.Key, run.Status)
		}
		// Decide on the step before reloading, so a refusal says what to do.
		switch action {
		case "continue":
			switch {
			case f.Status == runbook.StepAwaitingInput && f.Type == string(runbook.StepPause):
			case f.Status == runbook.StepAwaitingInput:
				return rbErr(http.StatusConflict, "step %s is a manual step: record its values or skip it", f.StepID)
			case f.Status == runbook.StepAwaitingConfirmation:
				return rbErr(http.StatusConflict, "step %s awaits confirmation: accept it or deny it", f.StepID)
			case f.Status == runbook.StepInterrupted:
				return rbErr(http.StatusConflict,
					"step %s was interrupted and may have taken effect: retry it or skip it", f.StepID)
			case f.Status == runbook.StepRunning || f.Status == runbook.StepWaiting || f.Status == runbook.StepPending:
				return rbErr(http.StatusConflict, "step %s is %s", f.StepID, f.Status)
			}
		case "skip":
			if f.Status == runbook.StepRunning || f.Status == runbook.StepPending {
				return rbErr(http.StatusConflict, "step %s is %s and cannot be skipped now", f.StepID, f.Status)
			}
		case "retry":
			if f.Status != runbook.StepDenied && f.Status != runbook.StepFailed && f.Status != runbook.StepInterrupted {
				return rbErr(http.StatusConflict, "step %s is %s; only a denied, failed or interrupted step is retried",
					f.StepID, f.Status)
			}
		}
		lr, newRevision, err := e.reload(ctx, run, steps)
		if err != nil {
			return err
		}
		run.Status, run.ActingUser, run.LastError = runbook.RunRunning, actor, nil

		var next *rbStep
		switch action {
		case "continue":
			if f.Status == runbook.StepAwaitingInput {
				if err = setStep(ctx, tx, f.Key, `status = $1, finished_at = now(), actor = $2, decision = `+decisionArgs(3),
					[]any{runbook.StepCompleted, actor, actor, true, ""}, actor, "continued", ""); err != nil {
					return err
				}
				f.Status = runbook.StepCompleted
			}
			if err = auditStep(ctx, tx, run, f, audit.EventDecision, "continued", actor, ""); err != nil {
				return err
			}
			next, err = e.advance(ctx, tx, run, lr, steps, actor)
		case "skip":
			if f.Status != runbook.StepDenied && f.Status != runbook.StepFailed && f.Status != runbook.StepSkipped &&
				f.Status != runbook.StepCompleted {
				dec := ""
				if f.Decision == nil {
					dec = `, decision = ` + decisionArgs(2)
				}
				args := []any{runbook.StepSkipped}
				if dec != "" {
					args = append(args, actor, false, "skipped")
				}
				if err = setStep(ctx, tx, f.Key, `status = $1, finished_at = now(), next_check_at = NULL`+dec,
					args, actor, "skipped", ""); err != nil {
					return err
				}
				f.Status = runbook.StepSkipped
			}
			if err = auditStep(ctx, tx, run, f, audit.EventDecision, "skipped", actor, ""); err != nil {
				return err
			}
			if f.Type == string(runbook.StepRepeat) {
				// A repeat skipped between iterations: the run goes on after it.
				next, err = e.advanceAfter(ctx, tx, run, lr, f, actor)
			} else {
				next, err = e.advance(ctx, tx, run, lr, steps, actor)
			}
		case "retry":
			step := lr.Runbook.Step(f.StepID)
			if step == nil {
				return rbErr(http.StatusConflict, "step %s is no longer in the runbook; skip it instead", f.StepID)
			}
			if err = auditStep(ctx, tx, run, f, audit.EventDecision, "retried", actor, ""); err != nil {
				return err
			}
			next, err = insertStep(ctx, tx, run, lr, step, f.Iteration, runbook.StepPending, actor, "retry of step "+
				strconv.FormatInt(f.Key, 10))
		}
		if err != nil {
			return err
		}
		if newRevision {
			from := ""
			if next != nil {
				from = next.StepID
			}
			run.Revisions[len(run.Revisions)-1].FromStep = from
		}
		return nil
	})
}

// advanceAfter goes on after a top-level step that is not the last row: the
// next top-level step, or the end of the run.
func (e *RunbookExecutor) advanceAfter(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook,
	f *rbStep, actor string) (*rbStep, error) {
	top, _ := posOf(lr.Runbook, f.StepID)
	if top < 0 || top+1 >= len(lr.Runbook.Steps) {
		run.Status = runbook.RunCompleted
		return nil, auditStep(ctx, tx, run, nil, audit.EventOutcome, "run_completed", actor, "")
	}
	return insertStep(ctx, tx, run, lr, lr.Runbook.Steps[top+1], 0, runbook.StepPending, actor, "pending")
}

// reload reads the runbook from the workspace again. When a file changed it
// must validate and leave every step that has run unchanged, and a new
// revision is recorded (§4.4, criterion 6).
func (e *RunbookExecutor) reload(ctx context.Context, run *rbRun, steps []*rbStep) (*loadedRunbook, bool, error) {
	disk, err := loadRunbookFromDisk(run.WorkspaceName, run.Runbook)
	if err != nil {
		return nil, false, err
	}
	current := run.RevisionFiles[strconv.Itoa(run.currentRevision())]
	if sameFiles(disk.Files, current) {
		lr, err := run.pinned()
		return lr, false, err
	}
	if !disk.Valid() {
		return nil, false, disk.refusal()
	}
	rb := disk.Runbook
	for _, s := range steps {
		if !rbStepExecuted(s) {
			continue
		}
		step := rb.Step(s.StepID)
		if step == nil {
			return nil, false, rbErr(http.StatusConflict,
				"the edit removes step %s, which has run; a step that has run cannot change (undo the edit, or abort the run)", s.StepID)
		}
		if stepDefHash(step) != s.DefHash {
			return nil, false, rbErr(http.StatusConflict,
				"the edit changes step %s, which has run; a step that has run cannot change (undo the edit, or abort the run)", s.StepID)
		}
		if step.Type.HasSQL() && s.SQLHash != nil && sha256Of(disk.Files[step.SQLPath(rb.Runbook)]) != *s.SQLHash {
			return nil, false, rbErr(http.StatusConflict,
				"the edit changes the SQL of step %s, which has run; a step that has run cannot change (undo the edit, or abort the run)", s.StepID)
		}
	}
	// The run's parameter values stand; a parameter the edit adds needs a
	// default.
	given := map[string]json.RawMessage{}
	for k, v := range run.Parameters {
		if _, ok := rb.Parameters[k]; ok {
			given[k] = v
		}
	}
	vals, err := runbook.ParameterValues(rb, given)
	if err != nil {
		return nil, false, rbErr(http.StatusConflict, "the edited runbook's parameters: %v", err)
	}
	if run.Parameters, err = paramsJSON(vals); err != nil {
		return nil, false, err
	}
	n := run.currentRevision() + 1
	run.Revisions = append(run.Revisions, runbook.Revision{Revision: n, Files: disk.Hashes()})
	if run.RevisionFiles == nil {
		run.RevisionFiles = map[string]map[string]string{}
	}
	run.RevisionFiles[strconv.Itoa(n)] = disk.Files
	return disk, true, nil
}

func sameFiles(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// Abort ends a run: the running statement is cancelled, and every step not
// final and every step not reached is recorded skipped.
func (e *RunbookExecutor) Abort(ctx context.Context, actor string, key int64) (*map[string]any, error) {
	res, err := e.control(ctx, key, func(tx pgx.Tx, run *rbRun, steps []*rbStep) error {
		if run.Status != runbook.RunRunning && run.Status != runbook.RunPaused {
			return rbErr(http.StatusConflict, "run %d is %s", run.Key, run.Status)
		}
		msg := "aborted by " + actor
		for _, s := range steps {
			stillOpen := rbStepActive(s) && s.Status != runbook.StepRunning
			openRepeat := s.Type == string(runbook.StepRepeat) &&
				(s.Status == runbook.StepRunning || s.Status == runbook.StepWaiting)
			if stillOpen || openRepeat {
				if err := setStep(ctx, tx, s.Key, `status = $1, finished_at = now(), next_check_at = NULL, error = $2`,
					[]any{runbook.StepSkipped, msg}, actor, "skipped", "aborted"); err != nil {
					return err
				}
			}
		}
		// The steps not reached, after the last one recorded.
		if lr, err := run.pinned(); err == nil {
			last := -1
			for _, s := range steps {
				if t, _ := posOf(lr.Runbook, s.StepID); t > last {
					last = t
				}
			}
			for i := last + 1; i < len(lr.Runbook.Steps); i++ {
				s, err := insertStep(ctx, tx, run, lr, lr.Runbook.Steps[i], 0, runbook.StepSkipped, actor, "skipped")
				if err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `UPDATE jetsapi.runbook_step_run SET error = $2 WHERE key = $1`, s.Key, msg); err != nil {
					return err
				}
			}
		}
		if err := auditStep(ctx, tx, run, nil, audit.EventDecision, "aborted", actor, ""); err != nil {
			return err
		}
		run.Status, run.LastError = runbook.RunAborted, &msg
		return nil
	})
	if err != nil {
		return nil, err
	}
	e.cancelRunning(ctx, key)
	return res, nil
}

// Release marks a finished run released and answers its result. Releasing is
// what lets the result leave the database (R6); a second release answers the
// same result and keeps the first releaser.
func (e *RunbookExecutor) Release(ctx context.Context, actor string, key int64) (*map[string]any, error) {
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	run, err := loadRun(ctx, tx, key, true)
	if err != nil {
		return nil, err
	}
	switch run.Status {
	case runbook.RunCompleted, runbook.RunFailed, runbook.RunAborted:
	default:
		return nil, rbErr(http.StatusConflict, "run %d is %s; only a finished run is released", key, run.Status)
	}
	if run.ReleasedBy == nil {
		if _, err = tx.Exec(ctx, `UPDATE jetsapi.runbook_run SET released_by = $2, released_at = now(), updated_at = now()
			WHERE key = $1`, key, actor); err != nil {
			return nil, err
		}
		if err = auditStep(ctx, tx, run, nil, audit.EventApproval, "released", actor, ""); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return e.Export(ctx, key)
}

// Export answers a released run's result; nothing is exported before release.
func (e *RunbookExecutor) Export(ctx context.Context, key int64) (*map[string]any, error) {
	run, err := loadRun(ctx, e.pool, key, false)
	if err != nil {
		return nil, err
	}
	if run.ReleasedBy == nil {
		return nil, rbErr(http.StatusConflict, "run %d has not been released; release it before exporting it", key)
	}
	steps, err := loadSteps(ctx, e.pool, key)
	if err != nil {
		return nil, err
	}
	res, err := resultOf(run, steps)
	if err != nil {
		return nil, err
	}
	return &map[string]any{"result": res}, nil
}
