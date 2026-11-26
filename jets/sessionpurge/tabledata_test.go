package sessionpurge

import (
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// createClaimSetSummary creates jetsapi.claim_set_summary with the columns the
// purge reads (the agentic layer's DDL has more), and gives each session one row.
func createClaimSetSummary(t *testing.T, pool *pgxpool.Pool, ids ...string) {
	t.Helper()
	purgetest.Exec(t, pool, `CREATE TABLE jetsapi.claim_set_summary (
		delivery_session_id text NOT NULL, session_id text NOT NULL, table_name text NOT NULL,
		row_count bigint, PRIMARY KEY (table_name, delivery_session_id))`)
	for _, id := range ids {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.claim_set_summary (delivery_session_id, session_id, table_name, row_count)
			VALUES ($1, $1, 'DomainA', 2)`, id)
	}
}

// A session's summary goes with it, by the purge's retention deletion and by a
// user's, which take the same executor; another session's is kept.
func TestClaimSetSummaryDeletedWithItsSession(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	for _, id := range []string{"ret", "usr", "keep"} {
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "c1", Registered: true, LastUpdate: days(200),
			DomainTables: []string{"DomainA"}})
	}
	createClaimSetSummary(t, pool, "ret", "usr", "keep")
	queue(t, pool, retention("ret"), Request{SessionID: "usr", Client: "c1", Reason: ReasonUser, RequestedBy: "u@x"})
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 2 || report.Rows["jetsapi.claim_set_summary"] != 2 {
		t.Errorf("done %d, %d summary rows deleted; want 2 and 2", report.Done, report.Rows["jetsapi.claim_set_summary"])
	}
	for id, want := range map[string]int64{"ret": 0, "usr": 0, "keep": 1} {
		if n := count(t, pool, `SELECT count(*) FROM jetsapi.claim_set_summary WHERE session_id = $1`, id); n != want {
			t.Errorf("%s has %d summary rows, want %d", id, n, want)
		}
	}
}

// A database without the summary table deletes a session as before, and
// reports nothing for the table.
func TestClaimSetSummaryAbsentIsSkipped(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, LastUpdate: days(200)})
	queue(t, pool, retention("s1"))
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Failed != 0 {
		t.Fatalf("done %d, failed %d; want 1 and 0", report.Done, report.Failed)
	}
	if _, ok := report.Rows["jetsapi.claim_set_summary"]; ok {
		t.Error("a database without claim_set_summary reported rows for it")
	}
}

// PurgeTableData deletes a due session's rows from the domain tables and
// process_errors only, marks it, and keeps its summary and every other row. It
// leaves a session registered after the table-data clock, one past the
// session's own clock, one queued for deletion and one already marked; it
// refuses one with a run submitted, and leaves it unmarked.
func TestPurgeTableData(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	domains := []string{"DomainA", "DomainB"}
	for id, age := range map[string]int{"due": 10, "recent": 2, "whole": 40, "queued": 10, "marked": 10} {
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "c1", Registered: true, LastUpdate: days(age),
			DomainTables: domains})
	}
	purgetest.Seed(t, pool, purgetest.Session{ID: "busy", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: domains, Runs: []string{"submitted"}})
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion (session_id, client, reason, requested_by, status)
		VALUES ('queued', 'c1', 'user', 'u@x', 'failed')`)
	purgetest.Exec(t, pool, `UPDATE jetsapi.session_registry SET table_data_purged_at = now() - interval '1 day'
		WHERE session_id = 'marked'`)
	createClaimSetSummary(t, pool, "due")

	before := map[string]map[string]int64{}
	for _, id := range []string{"due", "recent", "whole", "queued", "marked", "busy"} {
		before[id] = purgetest.SessionRows(t, pool, id)
	}
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Candidates != 2 || report.Done != 1 || report.Refused != 1 || report.Failed != 0 ||
		!reflect.DeepEqual(report.RefusedSessions, []string{"busy"}) {
		t.Errorf("report %+v; want 2 due (due and busy), 1 done, busy refused", report)
	}
	wantRows := map[string]int64{"DomainA": 2, "DomainB": 2, ProcessErrorsTable: 1}
	if !reflect.DeepEqual(report.Rows, wantRows) {
		t.Errorf("rows %v, want %v", report.Rows, wantRows)
	}

	want := maps.Clone(before["due"])
	delete(want, "public.DomainA")
	delete(want, "public.DomainB")
	delete(want, "jetsapi.process_errors")
	if got := purgetest.SessionRows(t, pool, "due"); !reflect.DeepEqual(got, want) {
		t.Errorf("due holds %v after its table data went, want %v", got, want)
	}
	if want["jetsapi.session_registry"] != 1 || want["jetsapi.pipeline_execution_status"] != 1 ||
		want["jetsapi.pipeline_execution_details"] != 1 || want["jetsapi.input_registry"] != 2 ||
		want["jetsapi.claim_set_summary"] != 1 {
		t.Errorf("the seed does not hold what the test means to keep: %v", want)
	}
	for _, id := range []string{"recent", "whole", "queued", "marked", "busy"} {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
			t.Errorf("%s changed: %v, was %v", id, got, before[id])
		}
	}
	marked := func(id string) bool {
		return count(t, pool, `SELECT count(*) FROM jetsapi.session_registry
			WHERE session_id = $1 AND table_data_purged_at IS NOT NULL`, id) == 1
	}
	for id, want := range map[string]bool{"due": true, "recent": false, "whole": false, "queued": false, "busy": false} {
		if marked(id) != want {
			t.Errorf("%s marked %v, want %v", id, !want, want)
		}
	}

	// Marked once: a row written for the session later is not deleted by the
	// next run, which selects busy alone.
	purgetest.Exec(t, pool, `INSERT INTO public."DomainA" (session_id, v) VALUES ('due', 'late')`)
	report, err = PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Candidates != 1 || report.Done != 0 || report.Refused != 1 {
		t.Errorf("second run %+v; want busy alone, refused", report)
	}
	if n := count(t, pool, `SELECT count(*) FROM public."DomainA" WHERE session_id = 'due'`); n != 1 {
		t.Errorf("the second run touched a marked session: %d rows left, want 1", n)
	}
}

// A deadline already passed starts no session, and the sessions are left for
// the next run, unmarked.
func TestPurgeTableDataDeadline(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "due", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"DomainA"}})
	before := purgetest.SessionRows(t, pool, "due")
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30),
		Options{Deadline: now.Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if report.NotReached != 1 || report.Done != 0 {
		t.Errorf("report %+v; want 1 not reached", report)
	}
	if got := purgetest.SessionRows(t, pool, "due"); !reflect.DeepEqual(got, before) {
		t.Errorf("due changed: %v, was %v", got, before)
	}
}
