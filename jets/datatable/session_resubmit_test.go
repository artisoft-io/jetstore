package datatable

// Session Registry's Resubmit as Pending (JM-185), against a real PostgreSQL.
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm --name srg-e-pg -e POSTGRES_PASSWORD=pw -p 55432:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55432/postgres \
//	  go test -count=1 ./jets/datatable/ -run ResubmitSessions

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

// newResubmitFixture is newStartFixture's file session f1 and m1, the
// domain_table session of a completed run that mapped it.
func newResubmitFixture(t *testing.T) *startFixture {
	t.Helper()
	f := newStartFixture(t)
	if _, _, err := f.start("m1"); err != nil {
		t.Fatal(err)
	}
	purgetest.Exec(t, f.pool, `UPDATE jetsapi.pipeline_execution_status SET status = 'completed' WHERE session_id = 'm1'`)
	purgetest.Exec(t, f.pool, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
		VALUES ('m1', 'domain_table', 'c1', 1, 313, 1357, 9497)`)
	return f
}

func resubmit(f *startFixture, ids ...any) (*map[string]any, int, error) {
	return f.ctx.ResubmitSessionsAsPending(&DataTableAction{
		Action: "resubmit_sessions_as_pending",
		Data:   []map[string]any{{"session_id": ids}},
	}, "u@example.com", "ws")
}

// pendingCopies are the pending runs of m1's pipeline on f1, other than m1's own.
func pendingCopies(t *testing.T, f *startFixture) []string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `SELECT session_id FROM jetsapi.pipeline_execution_status
		WHERE session_id <> 'm1' AND input_session_id = 'f1' AND status = 'pending' AND user_email = 'u@example.com'
		ORDER BY session_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

// A domain_table session's run is copied as pending and nothing starts it; a
// file session, which has no run, is reported and skipped; a second press finds
// the pending copy and adds none; and the copy does not read m1, so deleting m1
// does not reach it.
func TestResubmitSessionsAsPendingCopiesTheRunsOnce(t *testing.T) {
	f := newResubmitFixture(t)
	res, code, err := resubmit(f, "m1", "f1")
	if err != nil || code != http.StatusOK {
		t.Fatalf("resubmit answered %d, %v", code, err)
	}
	copies := pendingCopies(t, f)
	if len(copies) != 1 {
		t.Fatalf("pending copies after one press: %v, want one", copies)
	}
	want := map[string]any{"resubmitted": 1, "sessions": copies, "no_run": []string{"f1"}, "already_pending": []string{}}
	if !reflect.DeepEqual(*res, want) {
		t.Errorf("answer %v, want %v", *res, want)
	}
	var workspace string
	if err := f.pool.QueryRow(context.Background(), `SELECT coalesce(workspace_name, '')
		FROM jetsapi.pipeline_execution_status WHERE session_id = $1`, copies[0]).Scan(&workspace); err != nil {
		t.Fatal(err)
	}
	if workspace != "ws" {
		t.Errorf("the copy is bound to workspace %q, want ws", workspace)
	}

	res, code, err = resubmit(f, "m1")
	if err != nil || code != http.StatusOK {
		t.Fatalf("second resubmit answered %d, %v", code, err)
	}
	if got := pendingCopies(t, f); !reflect.DeepEqual(got, copies) {
		t.Errorf("pending copies after a second press: %v, want %v", got, copies)
	}
	if (*res)["resubmitted"] != 0 || !reflect.DeepEqual((*res)["already_pending"], []string{"m1"}) {
		t.Errorf("second answer %v, want m1 already pending and nothing resubmitted", *res)
	}

	closure, err := sessionpurge.Closure(context.Background(), f.pool, []string{"m1"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(closure); !reflect.DeepEqual(got, []string{"m1"}) {
		t.Errorf("deleting m1 after the resubmission deletes %v, want m1 alone", got)
	}
}

// A run whose input session is queued for deletion is refused with 409, as
// resubmit_pipeline refuses it, and the refusal writes nothing for any session.
func TestResubmitSessionsAsPendingRefusesAQueuedInput(t *testing.T) {
	f := newResubmitFixture(t)
	queueDeletion(t, f.pool, "f1", sessionpurge.ReasonUser)
	_, code, err := resubmit(f, "m1")
	if code != http.StatusConflict || err == nil {
		t.Errorf("resubmitting a run on a queued session answered %d, %v", code, err)
	}
	if got := pendingCopies(t, f); len(got) != 0 {
		t.Errorf("a refused resubmission left pending runs %v", got)
	}
}

// A run whose main input was purged is refused with 400, as resubmit_pipeline
// refuses it.
func TestResubmitSessionsAsPendingRefusesAPurgedInput(t *testing.T) {
	f := newResubmitFixture(t)
	purgetest.Exec(t, f.pool, `DELETE FROM jetsapi.input_registry WHERE session_id = 'f1'`)
	if _, code, err := resubmit(f, "m1"); code != http.StatusBadRequest || err == nil {
		t.Errorf("resubmitting a run whose input was purged answered %d, %v", code, err)
	}
}

// No user, or no session id, is refused before anything is read.
func TestResubmitSessionsAsPendingRefusesAMalformedRequest(t *testing.T) {
	f := newResubmitFixture(t)
	if _, code, _ := f.ctx.ResubmitSessionsAsPending(&DataTableAction{
		Data: []map[string]any{{"session_id": "m1"}}}, "", ""); code != http.StatusUnauthorized {
		t.Errorf("no user answered %d, want 401", code)
	}
	if _, code, _ := f.ctx.ResubmitSessionsAsPending(&DataTableAction{
		Data: []map[string]any{{}}}, "u@example.com", ""); code != http.StatusBadRequest {
		t.Errorf("no session id answered %d, want 400", code)
	}
}
