package sessionpurge

import (
	"fmt"
	"maps"
	"reflect"
	"strings"
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

// profileClaims gives a source_config row whose feed profile names table as its
// claim table.
func profileClaims(t *testing.T, pool *pgxpool.Pool, table string) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.source_config (object_type, client, table_name, user_email, feed_profile_json)
		VALUES ($1, 'c1', $1, 'u@x', jsonb_build_object('claim_set', jsonb_build_object('table', $1::text)))`, table)
}

// feedDetectRun records a feed_detect run started at started, as detectrun
// records one, with status and a summary saying whether it was a dry run.
func feedDetectRun(t *testing.T, pool *pgxpool.Pool, ref string, started time.Time, status string, dry bool) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.agentic_detection_run
		(task_ref, command, trigger, status, started_at, summary)
		VALUES ($1, 'feed_detect', 'schedule', $2, $3::timestamptz, $4)`, ref, status, started,
		fmt.Sprintf(`{"dryRun":%v,"mode":"window"}`, dry))
}

// A session with rows in a profiled feed's claim table, no summary, and no
// feed_detect run since its load keeps its table data, counted as held, and is
// selected again. A session with no claim rows is purged beside it.
func TestPurgeTableDataHoldsUnsummarisedClaims(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "claims", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"Claims", "DomainA"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "other", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"DomainA"}})
	profileClaims(t, pool, "Claims")
	profileClaims(t, pool, "public.Absent") // a table that does not exist is skipped
	createClaimSetSummary(t, pool)
	// Runs that release nothing: one before the load, one failed, one a dry run.
	feedDetectRun(t, pool, "before", days(11), "ok", false)
	feedDetectRun(t, pool, "failed", days(9), "failed", false)
	feedDetectRun(t, pool, "dry", days(9), "ok", true)

	before := purgetest.SessionRows(t, pool, "claims")
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Candidates != 2 || report.Held != 1 || report.Done != 1 ||
		!reflect.DeepEqual(report.HeldSessions, []string{"claims"}) {
		t.Errorf("report %+v; want 2 due, claims held, other done", report)
	}
	if got := purgetest.SessionRows(t, pool, "claims"); !reflect.DeepEqual(got, before) {
		t.Errorf("a held session changed: %v, was %v", got, before)
	}
	if n := count(t, pool, `SELECT count(*) FROM public."DomainA" WHERE session_id = 'other'`); n != 0 {
		t.Errorf("other kept %d rows in DomainA, want 0", n)
	}
	if !strings.Contains(report.Summary(), "1 held") {
		t.Errorf("the summary line does not count the held session: %s", report.Summary())
	}

	// Selected again by the next run, and held again.
	report, err = PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Candidates != 1 || report.Held != 1 || report.Done != 0 {
		t.Errorf("second run %+v; want claims due and held again", report)
	}
}

// A held session is released once a summary names it, and its table data goes.
func TestPurgeTableDataReleasedBySummary(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "claims", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"Claims"}})
	profileClaims(t, pool, "Claims")
	createClaimSetSummary(t, pool, "claims")
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Held != 0 || report.Done != 1 {
		t.Errorf("report %+v; want the summarised session done", report)
	}
	if n := count(t, pool, `SELECT count(*) FROM public."Claims" WHERE session_id = 'claims'`); n != 0 {
		t.Errorf("%d claim rows left, want 0", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.claim_set_summary WHERE session_id = 'claims'`); n != 1 {
		t.Errorf("the summary went with the table data: %d rows, want 1", n)
	}
}

// A held session is released by a feed_detect run that started after its load
// and ended ok without summarising it: it never will.
func TestPurgeTableDataReleasedByLaterRun(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "claims", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"Claims"}})
	profileClaims(t, pool, "Claims")
	createClaimSetSummary(t, pool)
	feedDetectRun(t, pool, "later", days(9), "ok", false)
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Held != 0 || report.Done != 1 {
		t.Errorf("report %+v; want the session released and done", report)
	}
}

// Without claim_set_summary, or without agentic_detection_run, the feature is
// not deployed and nothing is held.
func TestPurgeTableDataHoldsNothingUndeployed(t *testing.T) {
	for _, missing := range []string{"claim_set_summary", "agentic_detection_run"} {
		t.Run(missing, func(t *testing.T) {
			pool := purgetest.DB(t, testDB)
			purgetest.Seed(t, pool, purgetest.Session{ID: "claims", Client: "c1", Registered: true,
				LastUpdate: days(10), DomainTables: []string{"Claims"}})
			profileClaims(t, pool, "Claims")
			if missing == "agentic_detection_run" {
				createClaimSetSummary(t, pool)
				purgetest.Exec(t, pool, `DROP TABLE jetsapi.agentic_detection_run`)
			}
			now := time.Now()
			report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
			if err != nil {
				t.Fatal(err)
			}
			if report.Held != 0 || report.Done != 1 {
				t.Errorf("report %+v; want nothing held, the session done", report)
			}
		})
	}
}

// A session with no rows in a claim table is not held, even with no summary
// and no feed_detect run; nor is anything held where no profile names a table.
func TestPurgeTableDataHoldsOnlyClaimRows(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "plain", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"DomainA"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "unprofiled", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"Claims"}})
	purgetest.Exec(t, pool, `CREATE TABLE public."Profiled" (session_id text, v text)`)
	profileClaims(t, pool, "Profiled")
	createClaimSetSummary(t, pool)
	now := time.Now()
	report, err := PurgeTableData(ctx, pool, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Held != 0 || report.Done != 2 {
		t.Errorf("report %+v; want nothing held, both done", report)
	}
}

// A dry run reports a held session and writes nothing; and the session
// deletion still deletes a held session whole at RETENTION_DAYS, claim rows
// included.
func TestPurgeTableDataHoldDryRunAndRetention(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "held", Client: "c1", Registered: true, LastUpdate: days(10),
		DomainTables: []string{"Claims"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "old", Client: "c1", Registered: true, LastUpdate: days(40),
		DomainTables: []string{"Claims"}})
	profileClaims(t, pool, "Claims")
	createClaimSetSummary(t, pool)
	before := purgetest.RowCounts(t, pool)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	report, err := PurgeTableData(ctx, tx, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), Options{DryRun: true})
	if rbErr := tx.Rollback(ctx); rbErr != nil {
		t.Fatal(rbErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if report.Candidates != 1 || report.Held != 1 || report.Done != 0 ||
		!reflect.DeepEqual(report.HeldSessions, []string{"held"}) {
		t.Errorf("dry run %+v; want held reported", report)
	}
	if got := purgetest.RowCounts(t, pool); !reflect.DeepEqual(got, before) {
		t.Errorf("the dry run wrote: %v, was %v", got, before)
	}

	// The whole-session deletion is unchanged: a held session past
	// RETENTION_DAYS goes, its claim rows with it.
	queue(t, pool, retention("old"))
	whole, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if whole.Done != 1 {
		t.Fatalf("whole deletion %+v; want old deleted", whole)
	}
	if n := count(t, pool, `SELECT count(*) FROM public."Claims" WHERE session_id = 'old'`); n != 0 {
		t.Errorf("%d claim rows of a deleted session left, want 0", n)
	}
}
