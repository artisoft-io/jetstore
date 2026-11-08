package datatable

// The start guard and the two Delete actions (session_registry Phase 2, E.1 and
// E.3), against a real PostgreSQL. Needs JETS_TEST_DSN; skipped otherwise. Each
// test rebuilds the database srg_session_delete. Locally:
//
//	docker run -d --rm --name srg-e-pg -e POSTGRES_PASSWORD=pw -p 55432:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55432/postgres \
//	  go test -count=1 ./jets/datatable/ -run 'Start|Resubmission|SessionDeletion|DeleteSessions'

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const sessionDeleteDB = "srg_session_delete"

// startFixture is a database with a registered file session f1, its
// input_registry row, and a process_config row for the runs started on it.
type startFixture struct {
	pool        *pgxpool.Pool
	ctx         *DataTableContext
	registryKey int
}

func newStartFixture(t *testing.T) *startFixture {
	t.Helper()
	pool := purgetest.DB(t, sessionDeleteDB)
	f := &startFixture{pool: pool, ctx: &DataTableContext{Dbpool: pool}}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
		VALUES ('f1', 'file', 'c1', 1, 313, 1357, 9497)`)
	if err := pool.QueryRow(context.Background(), `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('c1', 'Claim', 'k', 1, 'claim_table', 'file', 'f1', 'system') RETURNING key`).Scan(&f.registryKey); err != nil {
		t.Fatal(err)
	}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.process_config
		(process_name, main_rules, is_rule_set, output_tables, user_email)
		VALUES ('proc', 'main.jr', 0, '{}', 'system')`)
	return f
}

// start inserts a pending run reading the file session through
// InsertPipelineExecutionStatus, as the UI's Start does. Pending, so nothing is
// started: the throttling and the state machine are not this test's.
func (f *startFixture) start(sessionId string) (int, int, error) {
	action := &DataTableAction{
		Action:      "insert_rows",
		FromClauses: []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
		Data: []map[string]any{{
			"pipeline_config_key": 1, "main_input_registry_key": f.registryKey, "main_input_file_key": "k",
			"merged_input_registry_keys": "{}",
			"client":                     "c1", "process_name": "proc", "main_object_type": "Claim",
			"session_id": sessionId, "source_period_key": 1, "status": "pending", "user_email": "u@example.com",
		}},
	}
	results := map[string]any{}
	return f.ctx.InsertPipelineExecutionStatus(action, 0, &results, "")
}

func (f *startFixture) runs(t *testing.T, sessionId string) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM jetsapi.pipeline_execution_status
		WHERE session_id = $1 AND input_session_id = 'f1'`, sessionId).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func queueDeletion(t *testing.T, pool *pgxpool.Pool, id string, reason sessionpurge.Reason) {
	t.Helper()
	if _, err := sessionpurge.Queue(context.Background(), pool, []sessionpurge.Request{
		{SessionID: id, Client: "c1", Reason: reason, RequestedBy: "someone@example.com"}}); err != nil {
		t.Fatal(err)
	}
}

// waitForLockWait returns once a backend of the test database waits on a lock,
// or fails after five seconds.
func waitForLockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no backend came to wait on a lock within five seconds")
}

// A start on a session nobody is deleting is as it was: the row is inserted,
// with the input session read from input_registry, and its key returned.
func TestStartOnAnUnqueuedSessionIsUnchanged(t *testing.T) {
	f := newStartFixture(t)
	key, code, err := f.start("run1")
	if err != nil || code != http.StatusOK || key == 0 {
		t.Fatalf("the start answered key %d, %d, %v", key, code, err)
	}
	if n := f.runs(t, "run1"); n != 1 {
		t.Errorf("%d runs of run1 reading f1, want 1", n)
	}
	// A duplicate key is still a 409 of its own.
	purgetest.Exec(t, f.pool, `CREATE UNIQUE INDEX one_run_per_session ON jetsapi.pipeline_execution_status (session_id)`)
	if _, code, err = f.start("run1"); code != http.StatusConflict || err == nil || err.Error() != "duplicate key value" {
		t.Errorf("a duplicate start answered %d, %v", code, err)
	}
}

// Q-16: a start on a session queued for deletion, for any reason, is refused with
// 409 and inserts nothing; on one deleted, likewise, with its own message.
func TestStartOnAQueuedSessionIsRefused(t *testing.T) {
	f := newStartFixture(t)
	queueDeletion(t, f.pool, "f1", sessionpurge.ReasonRetention)
	_, code, err := f.start("run1")
	if code != http.StatusConflict || err == nil || err.Error() != "input session f1 is queued for deletion" {
		t.Errorf("a start on a session queued for retention answered %d, %v", code, err)
	}
	purgetest.Exec(t, f.pool, `UPDATE jetsapi.session_deletion SET status = 'done', reason = 'user' WHERE session_id = 'f1'`)
	_, code, err = f.start("run2")
	if code != http.StatusConflict || err == nil || err.Error() != "input session f1 was deleted" {
		t.Errorf("a start on a deleted session answered %d, %v", code, err)
	}
	if n := f.runs(t, "run1") + f.runs(t, "run2"); n != 0 {
		t.Errorf("%d refused runs were inserted", n)
	}
}

// E.1: a deletion holding the session lock makes the start wait, and the start
// then finds the session queued and is refused. The deletion here queues the
// session inside the transaction that holds the lock, so a start that did not
// wait would read no deletion row and go ahead.
func TestADeletionHoldingTheLockMakesTheStartWaitThenRefuse(t *testing.T) {
	f := newStartFixture(t)
	bg := context.Background()
	del, err := f.pool.Begin(bg)
	if err != nil {
		t.Fatal(err)
	}
	defer del.Rollback(bg)
	if err = sessionpurge.LockSession(bg, del, "f1"); err != nil {
		t.Fatal(err)
	}
	if _, err = del.Exec(bg, `INSERT INTO jetsapi.session_deletion (session_id, client, reason, requested_by, status)
		VALUES ('f1', 'c1', 'user', 'someone@example.com', 'pending')`); err != nil {
		t.Fatal(err)
	}
	type out struct {
		code int
		err  error
	}
	done := make(chan out, 1)
	go func() {
		_, code, err := f.start("run1")
		done <- out{code, err}
	}()
	waitForLockWait(t, f.pool)
	select {
	case o := <-done:
		t.Fatalf("the start finished while the deletion held the lock: %d, %v", o.code, o.err)
	default:
	}
	if err = del.Commit(bg); err != nil {
		t.Fatal(err)
	}
	o := <-done
	if o.code != http.StatusConflict || o.err == nil || !strings.Contains(o.err.Error(), "queued for deletion") {
		t.Errorf("the start answered %d, %v; want 409, queued for deletion", o.code, o.err)
	}
	if n := f.runs(t, "run1"); n != 0 {
		t.Errorf("%d runs inserted, want 0", n)
	}
}

// resubmit_pipeline copies a run rather than inserting through
// InsertPipelineExecutionStatus, and is guarded on the input sessions the copied
// runs read.
func TestResubmissionOfARunReadingAQueuedSessionIsRefused(t *testing.T) {
	f := newStartFixture(t)
	if _, _, err := f.start("run1"); err != nil {
		t.Fatal(err)
	}
	bg := context.Background()
	guard := func() (int, error) {
		tx, err := f.pool.Begin(bg)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(bg)
		return GuardResubmission(bg, tx, "run1")
	}
	if code, err := guard(); code != http.StatusOK || err != nil {
		t.Errorf("resubmitting a run on a live session answered %d, %v", code, err)
	}
	queueDeletion(t, f.pool, "f1", sessionpurge.ReasonUser)
	if code, err := guard(); code != http.StatusConflict || err == nil || err.Error() != "input session f1 is queued for deletion" {
		t.Errorf("resubmitting a run on a queued session answered %d, %v", code, err)
	}
}

// deleteFixture is a file session f1, the session m1 that mapped it, and two
// sessions reading m1: r1 finished, r2 pending.
func newDeleteFixture(t *testing.T) *startFixture {
	t.Helper()
	f := newStartFixture(t)
	for _, s := range []struct{ id, reads, status string }{
		{"m1", "f1", "completed"}, {"r1", "m1", "completed"}, {"r2", "m1", "pending"},
	} {
		purgetest.Exec(t, f.pool, `INSERT INTO jetsapi.session_registry
			(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
			VALUES ($1, 'domain_table', 'c1', 1, 313, 1357, 9497)`, s.id)
		purgetest.Exec(t, f.pool, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id,
			 source_period_key, status, user_email)
			VALUES (1, 'c1', 'proc', 'Claim', $2, $1, 1, $3, 'system')`, s.id, s.reads, s.status)
	}
	return f
}

func ids(rows []sessionpurge.Dependent) []string {
	var out []string
	for _, d := range rows {
		out = append(out, d.SessionID)
	}
	return out
}

// E.3: the preview of a file session lists the session that mapped it and that
// session's readers, and writes nothing; delete_sessions with the previewed set
// queues all of them, user and requested by the caller, and reports the session
// with an active run as waiting; with a set that has changed since, 409.
func TestDeleteSessionsQueuesThePreviewedClosure(t *testing.T) {
	f := newDeleteFixture(t)
	// One row carrying the list, as a flow with rows "fields" sends it.
	preview, code, err := f.ctx.PreviewSessionDeletion(&DataTableAction{
		Data: []map[string]any{{"session_id": []any{"f1"}}}})
	if err != nil || code != http.StatusOK {
		t.Fatalf("preview answered %d, %v", code, err)
	}
	rows := (*preview)["rows"].([]sessionpurge.Dependent)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"f1", "m1", "r1", "r2"}) {
		t.Fatalf("the preview lists %v", got)
	}
	if rows[1].ReadsSession != "f1" || rows[2].ReadsSession != "m1" || !rows[1].Active || !rows[3].Active || rows[2].Active {
		t.Errorf("the preview's rows are %+v", rows)
	}
	var n int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM jetsapi.session_deletion`).Scan(&n)
	if n != 0 {
		t.Errorf("the preview wrote %d deletion rows", n)
	}

	// A reader added since the preview: the confirmed set is stale.
	purgetest.Exec(t, f.pool, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id,
		 source_period_key, status, user_email)
		VALUES (1, 'c1', 'proc', 'Claim', 'r1', 'r3', 1, 'completed', 'system')`)
	// One row per selected session, expected repeated on each, as a fan-out sends it.
	stale := &DataTableAction{Data: []map[string]any{
		{"session_id": "f1", "expected": []any{"f1", "m1", "r1", "r2"}}}}
	if _, code, err = f.ctx.DeleteSessions(stale, "admin@example.com"); code != http.StatusConflict ||
		!errors.Is(err, ErrSessionsChanged) {
		t.Errorf("a stale confirmation answered %d, %v", code, err)
	}

	confirmed := &DataTableAction{Data: []map[string]any{
		{"session_id": "f1", "expected": "f1,m1,r1,r2,r3"}}}
	res, code, err := f.ctx.DeleteSessions(confirmed, "admin@example.com")
	if err != nil || code != http.StatusOK {
		t.Fatalf("delete_sessions answered %d, %v", code, err)
	}
	if (*res)["queued"] != 5 || !reflect.DeepEqual((*res)["waiting"], []string{"m1", "r2"}) {
		t.Errorf("delete_sessions answered %v", *res)
	}
	rowsQ, err := f.pool.Query(context.Background(), `SELECT session_id, reason, requested_by, status, client,
		COALESCE(source_type, ''), COALESCE(month_period, 0) FROM jetsapi.session_deletion ORDER BY session_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rowsQ.Close()
	var got []string
	for rowsQ.Next() {
		var id, reason, by, status, client, st string
		var mp int
		if err := rowsQ.Scan(&id, &reason, &by, &status, &client, &st, &mp); err != nil {
			t.Fatal(err)
		}
		got = append(got, strings.Join([]string{id, reason, by, status, client, st}, "|"))
		if mp != 313 {
			t.Errorf("%s queued with month period %d, want 313", id, mp)
		}
	}
	want := []string{
		"f1|user|admin@example.com|pending|c1|file",
		"m1|user|admin@example.com|pending|c1|domain_table",
		"r1|user|admin@example.com|pending|c1|domain_table",
		"r2|user|admin@example.com|pending|c1|domain_table",
		"r3|user|admin@example.com|pending|c1|",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("session_deletion holds\n%v\nwant\n%v", got, want)
	}
}

// What a malformed request answers.
func TestDeleteSessionsRefusesAMalformedRequest(t *testing.T) {
	f := newDeleteFixture(t)
	for _, c := range []struct {
		name string
		data []map[string]any
		code int
	}{
		{"no data", nil, http.StatusBadRequest},
		{"no expected", []map[string]any{{"session_id": "f1"}}, http.StatusBadRequest},
		{"a number", []map[string]any{{"session_id": 7, "expected": "f1"}}, http.StatusBadRequest},
		{"an empty id", []map[string]any{{"session_id": "f1,", "expected": "f1"}}, http.StatusBadRequest},
		{"not registered", []map[string]any{{"session_id": "nosuch", "expected": []any{"nosuch"}}}, http.StatusNotFound},
	} {
		if _, code, err := f.ctx.DeleteSessions(&DataTableAction{Data: c.data}, "admin@example.com"); code != c.code || err == nil {
			t.Errorf("%s: answered %d, %v; want %d", c.name, code, err, c.code)
		}
	}
	if _, code, err := f.ctx.DeleteSessions(&DataTableAction{Data: []map[string]any{{"session_id": "f1", "expected": "f1"}}}, ""); code != http.StatusUnauthorized || err == nil {
		t.Errorf("no user answered %d, %v", code, err)
	}
}
