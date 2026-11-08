package datatable

// A user's Delete of a session (session_registry, JM-081, Phase 2 E): the two
// server actions, and the guard a pipeline start passes before it reads a
// session.
//
// The deletion itself is jets/sessionpurge's, the one path by which a session is
// deleted. These actions compute what it would delete (sessionpurge.Closure) and
// queue it; the apiserver's runner executes the queue after answering.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/jackc/pgx/v5"
)

// ErrSessionsChanged is delete_sessions' answer, with 409, when the closure it
// recomputes is not the one the caller confirmed.
var ErrSessionsChanged = errors.New("the sessions to delete changed; review them again")

// GuardInputSession is what a pipeline start does, in the transaction that
// inserts its status row, before a run reads inputSessionID: it takes the session
// lock (sessionpurge.LockSession), then refuses with 409 if session_deletion has
// a row for the session, whatever its reason and status (Q-16). It answers 200
// and nil when the run may start, and does nothing for an empty id.
//
// The lock orders the start against the executor, which takes the same lock
// before its refusal check: a start holding it commits its row before the
// executor reads, and a start waiting for it finds the session queued.
func GuardInputSession(ctx context.Context, tx pgx.Tx, inputSessionID string) (int, error) {
	if inputSessionID == "" {
		return http.StatusOK, nil
	}
	if err := sessionpurge.LockSession(ctx, tx, inputSessionID); err != nil {
		return http.StatusInternalServerError, err
	}
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM jetsapi.session_deletion WHERE session_id = $1`,
		inputSessionID).Scan(&status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return http.StatusOK, nil
	case err != nil:
		return http.StatusInternalServerError, fmt.Errorf("while reading session_deletion for input session %s: %w",
			inputSessionID, err)
	case status == sessionpurge.StatusDone:
		return http.StatusConflict, fmt.Errorf("input session %s was deleted", inputSessionID)
	default:
		return http.StatusConflict, fmt.Errorf("input session %s is queued for deletion", inputSessionID)
	}
}

// GuardResubmission is GuardInputSession for resubmit_pipeline, which copies the
// runs of sessionId into a new session rather than inserting through
// InsertPipelineExecutionStatus: it guards every input session those runs read,
// in order, in tx, the transaction of the copy.
func GuardResubmission(ctx context.Context, tx pgx.Tx, sessionId string) (int, error) {
	rows, err := tx.Query(ctx, `SELECT DISTINCT input_session_id FROM jetsapi.pipeline_execution_status
		WHERE session_id = $1 AND input_session_id IS NOT NULL AND input_session_id <> ''
		ORDER BY input_session_id`, sessionId)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while reading the input sessions of %s: %w", sessionId, err)
	}
	inputs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("while reading the input sessions of %s: %w", sessionId, err)
	}
	for _, in := range inputs {
		if code, err := GuardInputSession(ctx, tx, in); err != nil {
			return code, err
		}
	}
	return http.StatusOK, nil
}

// sessionIDsOf reads the selected session ids from a request's data rows. Each
// row's session_id is a string or a list of strings, so a flow may send one row
// per selected session or one row carrying the list.
func sessionIDsOf(rows []map[string]any) ([]string, error) {
	var ids []string
	for i, row := range rows {
		got, err := stringsOf(row["session_id"])
		if err != nil {
			return nil, fmt.Errorf("data row %d: session_id: %w", i, err)
		}
		ids = append(ids, got...)
	}
	if len(ids) == 0 {
		return nil, errors.New("no session_id in the request's data")
	}
	return ids, nil
}

// expectedOf reads delete_sessions' expected: the union of the expected key of
// every data row that has one, so a flow may repeat it on each row or send it
// once. Absent from every row is an error, not an empty set.
func expectedOf(rows []map[string]any) ([]string, error) {
	var out []string
	found := false
	for i, row := range rows {
		v, ok := row["expected"]
		if !ok || v == nil {
			continue
		}
		found = true
		got, err := stringsOf(v)
		if err != nil {
			return nil, fmt.Errorf("data row %d: expected: %w", i, err)
		}
		out = append(out, got...)
	}
	if !found {
		return nil, errors.New("no expected in the request's data: post the session ids preview_session_deletion returned")
	}
	return out, nil
}

// stringsOf accepts a non-empty string, which may be a comma-separated list, or a
// JSON array of non-empty strings.
func stringsOf(v any) ([]string, error) {
	var parts []string
	switch x := v.(type) {
	case string:
		parts = strings.Split(x, ",")
	case []any:
		for _, e := range x {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf("a list holding %T, want strings", e)
			}
			parts = append(parts, s)
		}
	case []string:
		parts = x
	default:
		return nil, fmt.Errorf("%T, want a string or a list of strings", v)
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return nil, errors.New("an empty session id")
		}
		out = append(out, p)
	}
	return out, nil
}

// closureFor is the closure of the request's selected sessions, with the status
// a failure answers.
func (ctx *DataTableContext) closureFor(action *DataTableAction) ([]sessionpurge.Dependent, int, error) {
	ids, err := sessionIDsOf(action.Data)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	closure, err := sessionpurge.Closure(context.Background(), ctx.Dbpool, ids)
	switch {
	case errors.Is(err, sessionpurge.ErrSessionNotRegistered):
		return nil, http.StatusNotFound, err
	case err != nil:
		log.Printf("while computing the closure of %v: %v", ids, err)
		return nil, http.StatusInternalServerError, errors.New("error while reading the sessions to delete")
	}
	return closure, http.StatusOK, nil
}

// PreviewSessionDeletion is preview_session_deletion: the sessions a Delete of
// the selected ones would delete, as {"rows": [Dependent...]}, selected first,
// then by depth, then by id. It writes nothing. The caller has checked
// CapabilityDeleteSession.
//
// Request: data rows each carrying session_id, a string or a list of strings.
func (ctx *DataTableContext) PreviewSessionDeletion(action *DataTableAction) (*map[string]any, int, error) {
	closure, code, err := ctx.closureFor(action)
	if err != nil {
		return nil, code, err
	}
	return &map[string]any{"rows": closure}, http.StatusOK, nil
}

// DeleteSessions is delete_sessions: it recomputes the closure of the selected
// sessions and, when it is the set the caller confirmed, queues every session of
// it with reason user and requested_by requestedBy. It answers
// {"queued": n, "waiting": [ids]}: n the deletion rows inserted or replaced
// (sessionpurge.Queue: a session already queued by a user keeps its row and is
// not counted), waiting the sessions of the closure with an active run, which the
// executor leaves queued until the run ends. It answers 409 with
// ErrSessionsChanged when the closure differs from expected. The caller has
// checked CapabilityDeleteSession, and starts the executor after answering.
//
// Request: data rows each carrying session_id, a string or a list of strings,
// and expected, the session ids preview_session_deletion returned, as a list or a
// comma-separated string, on one row or repeated on each (the union is taken).
func (ctx *DataTableContext) DeleteSessions(action *DataTableAction, requestedBy string) (*map[string]any, int, error) {
	if requestedBy == "" {
		return nil, http.StatusUnauthorized, errors.New("error: no user in the request's token")
	}
	expected, err := expectedOf(action.Data)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	closure, code, err := ctx.closureFor(action)
	if err != nil {
		return nil, code, err
	}
	got := make([]string, len(closure))
	waiting := []string{}
	for i, d := range closure {
		got[i] = d.SessionID
		if d.Active {
			waiting = append(waiting, d.SessionID)
		}
	}
	slices.Sort(got)
	slices.Sort(expected)
	if !slices.Equal(got, slices.Compact(expected)) {
		return nil, http.StatusConflict, ErrSessionsChanged
	}
	n, err := sessionpurge.Queue(context.Background(), ctx.Dbpool,
		sessionpurge.Requests(closure, sessionpurge.ReasonUser, requestedBy))
	if err != nil {
		log.Printf("while queueing the deletion of %v: %v", got, err)
		return nil, http.StatusInternalServerError, errors.New("error while queueing the sessions to delete")
	}
	log.Printf("delete_sessions: %s queued %d of %d sessions for deletion, %d waiting for a run: %s",
		requestedBy, n, len(closure), len(waiting), strings.Join(got, ", "))
	return &map[string]any{"queued": n, "waiting": waiting}, http.StatusOK, nil
}
