package sessionpurge

// Phase 2's executor tests (session_registry E.1, E.2, E.3, E.4), against a real
// PostgreSQL as the rest of this package's; see jets/sessionpurge/purgetest.

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedRun inserts one pipeline_execution_status row of session id reading input
// ("" for none), with status, and returns its key.
func seedRun(t *testing.T, pool *pgxpool.Pool, id, input, status string) int {
	t.Helper()
	var key int
	var in any
	if input != "" {
		in = input
	}
	err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id,
		 source_period_key, status, user_email)
		VALUES (1, 'c1', 'proc_' || $1, 'ObjectA', $2, $1, 1, $3, 'system') RETURNING key`,
		id, in, status).Scan(&key)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func register(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
		VALUES ($1, 'file', 'c1', 1, 313, 1357, 9497)`, id)
}

// waitForLockWait returns once a backend of the test database is waiting on a
// lock, or fails after five seconds.
func waitForLockWait(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity
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

// A file session's closure holds the session that mapped it and that session's
// readers, selected first, then by depth, then by id; a cycle ends; an active
// run is reported; a selected session that is not registered is an error.
func TestClosureFollowsInputSession(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	register(t, pool, "file1")
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('c1', 'Claim', 'k', 1, 'claim_table', 'file', 'file1', 'system')`)
	register(t, pool, "map1")
	seedRun(t, pool, "map1", "file1", "completed")
	register(t, pool, "readB")
	seedRun(t, pool, "readB", "map1", "completed")
	// readA is not registered (its run failed before it could be) and is active.
	seedRun(t, pool, "readA", "map1", "pending")
	// A cycle back to the file's session ends the walk.
	seedRun(t, pool, "file1", "readB", "completed")
	register(t, pool, "other")
	seedRun(t, pool, "other", "", "completed")

	got, err := Closure(ctx, pool, []string{"file1"})
	if err != nil {
		t.Fatal(err)
	}
	m313 := 313
	want := []Dependent{
		{SessionID: "file1", Client: "c1", SourceType: "file", MonthPeriod: &m313, ProcessName: "proc_file1", ObjectType: "Claim"},
		{SessionID: "map1", Client: "c1", SourceType: "file", MonthPeriod: &m313, ReadsSession: "file1", ProcessName: "proc_map1", ObjectType: "ObjectA", Active: true},
		{SessionID: "readA", Client: "c1", MonthPeriod: &m313, ReadsSession: "map1", ProcessName: "proc_readA", ObjectType: "ObjectA", Active: true},
		{SessionID: "readB", Client: "c1", SourceType: "file", MonthPeriod: &m313, ReadsSession: "map1", ProcessName: "proc_readB", ObjectType: "ObjectA"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("closure of file1 is\n%+v\nwant\n%+v", got, want)
	}

	// Selected sessions come first, even one that reads another.
	got, err = Closure(ctx, pool, []string{"readB", "map1"})
	if err != nil {
		t.Fatal(err)
	}
	var ids, readsOf []string
	for _, d := range got {
		ids = append(ids, d.SessionID)
		readsOf = append(readsOf, d.ReadsSession)
	}
	if !reflect.DeepEqual(ids, []string{"map1", "readB", "file1", "readA"}) ||
		!reflect.DeepEqual(readsOf, []string{"", "", "readB", "map1"}) {
		t.Errorf("closure of map1 and readB is %v reading %v", ids, readsOf)
	}

	if _, err = Closure(ctx, pool, []string{"file1", "nosuch"}); !errors.Is(err, ErrSessionNotRegistered) ||
		!strings.Contains(err.Error(), "nosuch") {
		t.Errorf("a selected session that is not registered gave %v", err)
	}
	if _, err = Closure(ctx, pool, nil); err == nil {
		t.Error("an empty selection gave no error")
	}
}

// Execute with Reasons [user] takes the user rows and leaves the retention rows
// queued; with no Reasons it takes every row.
func TestExecuteReasonsLeavesRetentionQueued(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "ret", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "usr", Client: "c1", Registered: true})
	queue(t, pool, retention("ret"),
		Request{SessionID: "usr", Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})

	report, err := Execute(ctx, pool, Options{Reasons: []Reason{ReasonUser}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.ByReason[ReasonUser] != 1 {
		t.Errorf("Execute with Reasons [user] did %+v", report)
	}
	if d := deletion(t, pool, "ret"); d == nil || d.Status != StatusPending {
		t.Errorf("the retention row is %+v, want still pending", d)
	}
	if d := deletion(t, pool, "usr"); d == nil || d.Status != StatusDone {
		t.Errorf("the user row is %+v, want done", d)
	}
	if _, err = Execute(ctx, pool, Options{Reasons: []Reason{"bogus"}}); err == nil {
		t.Error("an unknown reason gave no error")
	}
	if report, err = Execute(ctx, pool, Options{}); err != nil || report.Done != 1 || report.ByReason[ReasonRetention] != 1 {
		t.Errorf("Execute with no Reasons did %+v, %v", report, err)
	}
}

// I-26: two executors deleting the last two sessions of one coordinator request
// at the same time leave no map row. The second waits on the map row the first
// locked, and finds the first one's item gone when it gets it.
func TestTwoExecutorsLeaveNoMapRow(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s2", Client: "c1", Registered: true})
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, nbr_tasks) VALUES ('req-shared', 2);
		INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-shared', 's1'), ('req-shared', 's2')`)
	user := func(id string) Request {
		return Request{SessionID: id, Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"}
	}
	queue(t, pool, user("s1"), user("s2"))
	tables, err := resolveTables(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	r1, r2 := user("s1"), user("s2")
	if res, err := deleteRows(ctx, tx1, &r1, tables, true); err != nil || res.refused || res.held {
		t.Fatalf("deleting s1: %+v, %v", res, err)
	}

	done := make(chan error, 1)
	go func() {
		tx2, err := pool.Begin(ctx)
		if err != nil {
			done <- err
			return
		}
		defer tx2.Rollback(ctx)
		if _, err = deleteRows(ctx, tx2, &r2, tables, true); err != nil {
			done <- err
			return
		}
		done <- tx2.Commit(ctx)
	}()
	waitForLockWait(t, pool)
	select {
	case err := <-done:
		t.Fatalf("the second executor finished while the first held the map row: %v", err)
	default:
	}
	if err = tx1.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = 'req-shared'`); n != 0 {
		t.Errorf("%d map rows of req-shared remain, want 0", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map_items WHERE request_id = 'req-shared'`); n != 0 {
		t.Errorf("%d map items of req-shared remain, want 0", n)
	}
}

// E.1, the executor's half: a start holding the session lock makes the executor
// wait for it, before its claim, and then refuse the session for the run the
// start committed. The
// start is played here by its two statements, LockSession and the insert, in one
// transaction; jets/datatable tests the start itself.
func TestStartHoldingTheLockMakesTheExecutorRefuse(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "in1", Client: "c1", Registered: true})

	start, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer start.Rollback(ctx)
	if err = LockSession(ctx, start, "in1"); err != nil {
		t.Fatal(err)
	}
	if _, err = start.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id,
		 source_period_key, status, user_email)
		VALUES (1, 'c1', 'proc', 'ObjectA', 'in1', 'run1', 1, 'submitted', 'system')`); err != nil {
		t.Fatal(err)
	}

	queue(t, pool, Request{SessionID: "in1", Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})
	type out struct {
		r   Report
		err error
	}
	done := make(chan out, 1)
	go func() {
		r, err := Execute(ctx, pool, Options{Reasons: []Reason{ReasonUser}})
		done <- out{r, err}
	}()
	waitForLockWait(t, pool)
	if err = start.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	o := <-done
	if o.err != nil || o.r.Refused != 1 || o.r.Done != 0 {
		t.Errorf("the executor did %+v, %v; want the session refused", o.r, o.err)
	}
	if d := deletion(t, pool, "in1"); d == nil || d.Status != StatusPending {
		t.Errorf("the deletion row is %+v, want still pending", d)
	}
}

// A dry run takes no session lock and no coordinator map-row lock: with another
// transaction holding the session lock on S and a FOR UPDATE on the map row of a
// request S shares with a live session, a dry run over S completes, inside a short
// deadline, and still predicts S's rows, its own request's map row included and
// the shared one kept.
func TestDryRunTakesNoSessionOrMapLock(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "d1", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "live", Client: "c1", Registered: true})
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_coordinator_map (request_id, nbr_tasks) VALUES ('req-shared', 2);
		INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-shared', 'd1'), ('req-shared', 'live')`)
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback(ctx)
	if err = LockSession(ctx, holder, "d1"); err != nil {
		t.Fatal(err)
	}
	if _, err = holder.Exec(ctx, `SELECT request_id FROM jetsapi.pipeline_coordinator_map
		WHERE request_id = 'req-shared' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}

	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	report, err := ExecuteRequests(short, pool, []Request{retention("d1")}, Options{DryRun: true})
	if err != nil {
		t.Fatalf("the dry run did not complete while another transaction held d1's locks: %v", err)
	}
	if report.Done != 1 || report.Rows["jetsapi.session_registry"] != 1 ||
		report.Rows["jetsapi.pipeline_coordinator_map_items"] != 2 || report.Rows["jetsapi.pipeline_coordinator_map"] != 1 {
		t.Errorf("the dry run predicted %+v; want d1 done, its registry row, its two map items and its own request's map row", report)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.session_deletion`); n != 0 {
		t.Errorf("the dry run wrote %d deletion rows", n)
	}
}

// Two executors on one session serialise on the session lock, taken before the
// claim: the second waits, then finds the deletion row done and counts the
// session held.
func TestTwoExecutorsOnOneSessionSerialise(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true})
	queue(t, pool, Request{SessionID: "s1", Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})
	tables, err := resolveTables(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx)
	if res, err := deleteRows(ctx, first, &Request{SessionID: "s1", Reason: ReasonUser}, tables, true); err != nil || res.held || res.refused {
		t.Fatalf("the first executor: %+v, %v", res, err)
	}
	type out struct {
		r   Report
		err error
	}
	done := make(chan out, 1)
	go func() {
		r, err := Execute(ctx, pool, Options{Reasons: []Reason{ReasonUser}})
		done <- out{r, err}
	}()
	waitForLockWait(t, pool)
	if err = first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	o := <-done
	if o.err != nil || o.r.Skipped != 1 || o.r.Done != 0 || o.r.Failed != 0 {
		t.Errorf("the second executor did %+v, %v; want the session held", o.r, o.err)
	}
	if d := deletion(t, pool, "s1"); d == nil || d.Status != StatusDone {
		t.Errorf("the deletion row is %+v, want done", d)
	}
}
