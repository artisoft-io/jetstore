package delegate

// DOMAIN_TABLES_RETENTION_DAYS, the table data's clock, as the purge runs it.
// Against a real PostgreSQL, skipped unless JETS_TEST_DSN names one; see
// jets/sessionpurge/purgetest. Run with -count=1.

import (
	"bytes"
	"context"
	"log"
	"maps"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDomainTablesRetentionDays(t *testing.T) {
	for _, tc := range []struct {
		value   string
		days    int
		capped  bool
		wantErr bool
	}{
		{value: "", days: 30},
		{value: "  ", days: 30},
		{value: "1", days: 1},
		{value: " 7 ", days: 7},
		{value: "30", days: 30},
		{value: "31", days: 30, capped: true},
		{value: "400", days: 30, capped: true},
		{value: "0", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "1.5", wantErr: true},
		{value: "seven", wantErr: true},
	} {
		days, capped, err := DomainTablesRetentionDays(tc.value, 30)
		if (err != nil) != tc.wantErr || days != tc.days || capped != tc.capped {
			t.Errorf("DomainTablesRetentionDays(%q, 30) = (%d, %v, %v), want (%d, %v, error %v)",
				tc.value, days, capped, err, tc.days, tc.capped, tc.wantErr)
		}
	}
}

// seedTableData writes, with RETENTION_DAYS 30 and DOMAIN_TABLES_RETENTION_DAYS 7
// in mind: a session past both clocks, one past the table data's only, one
// with a run submitted past the table data's only, and a recent one. The one
// past the table data's clock has a summary row.
func seedTableData(t *testing.T, pool *pgxpool.Pool) {
	domains := []string{"DomainA", "DomainB"}
	purgetest.Seed(t, pool, purgetest.Session{ID: "old", Client: "c1", Registered: true, LastUpdate: daysAgo(40),
		DomainTables: domains})
	purgetest.Seed(t, pool, purgetest.Session{ID: "mid", Client: "c1", Registered: true, LastUpdate: daysAgo(10),
		DomainTables: domains})
	purgetest.Seed(t, pool, purgetest.Session{ID: "busy", Client: "c1", Registered: true, LastUpdate: daysAgo(10),
		DomainTables: domains, Runs: []string{"submitted"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "new", Client: "c1", Registered: true, LastUpdate: daysAgo(2),
		DomainTables: domains})
	purgetest.Exec(t, pool, `CREATE TABLE jetsapi.claim_set_summary (
		delivery_session_id text NOT NULL, session_id text NOT NULL, table_name text NOT NULL,
		PRIMARY KEY (table_name, delivery_session_id))`)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.claim_set_summary VALUES ('mid', 'mid', 'DomainA'), ('old', 'old', 'DomainA')`)
}

// runTableData runs purgeSessions with RETENTION_DAYS 30 and
// DOMAIN_TABLES_RETENTION_DAYS domainDays, and returns its log and its error.
func runTableData(t *testing.T, pool *pgxpool.Pool, domainDays string, dry bool) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	now := time.Now()
	err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, DomainTablesRetention: domainDays,
		DryRun: dry, Now: now, Deadline: now.Add(runBudget)})
	log.SetFlags(flags)
	log.SetOutput(out)
	return buf.String(), err
}

var (
	tableDataSummaryRE = regexp.MustCompile(`(?m)^Table data purge: (?:DRY RUN, nothing written: )?(\d+) sessions' table data deleted of (\d+) due, (\d+) refused`)
	tableDataRowsRE    = regexp.MustCompile(`(?m)^   table data: (?:DRY RUN, nothing written: )?(\d+) rows from (.+)$`)
)

// tableDataReport parses the table-data lines of a run's log: done, due,
// refused, and the rows per table.
func tableDataReport(t *testing.T, logged string) (done, due, refused int, rows map[string]int64) {
	t.Helper()
	m := tableDataSummaryRE.FindStringSubmatch(logged)
	if m == nil {
		t.Fatalf("no table data purge in the log:\n%s", logged)
	}
	rows = map[string]int64{}
	for _, r := range tableDataRowsRE.FindAllStringSubmatch(logged, -1) {
		rows[r[2]] = int64(atoi(r[1]))
	}
	return atoi(m[1]), atoi(m[2]), atoi(m[3]), rows
}

func markedSessions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT session_id FROM jetsapi.session_registry
		WHERE table_data_purged_at IS NOT NULL ORDER BY session_id`)
	if err != nil {
		t.Fatal(err)
	}
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

// Unset, empty or equal to RETENTION_DAYS, the purge is today's: the table-data
// purge does not run, nothing is marked, and the session past the shorter
// clock keeps every row. Above RETENTION_DAYS, the same, and the cap is logged.
func TestTableDataDefaultIsRetentionDays(t *testing.T) {
	for _, value := range []string{"", "30", "45"} {
		t.Run("value "+value, func(t *testing.T) {
			pool := jobFailureTestDB(t)
			seedTableData(t, pool)
			keep := map[string]map[string]int64{}
			for _, id := range []string{"mid", "busy", "new"} {
				keep[id] = purgetest.SessionRows(t, pool, id)
			}
			logged, err := runTableData(t, pool, value, false)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(logged, "Table data purge:") {
				t.Errorf("the table-data purge ran on RETENTION_DAYS's clock:\n%s", logged)
			}
			wantLine := "Table data is kept as long as its session: DOMAIN_TABLES_RETENTION_DAYS is 30"
			if value == "45" {
				wantLine = "DOMAIN_TABLES_RETENTION_DAYS 45 is above RETENTION_DAYS 30 and is capped to it"
			}
			if !strings.Contains(logged, wantLine) {
				t.Errorf("the log does not say %q:\n%s", wantLine, logged)
			}
			for id, want := range keep {
				if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, want) {
					t.Errorf("%s changed: %v, was %v", id, got, want)
				}
			}
			if left := purgetest.SessionRows(t, pool, "old"); len(left) > 0 {
				t.Errorf("old was not deleted whole: %v", left)
			}
			if m := markedSessions(t, pool); len(m) > 0 {
				t.Errorf("sessions marked: %v", m)
			}
			if got := purgeFailures(t, pool); len(got) != 0 {
				t.Errorf("failures recorded: %+v", got)
			}
		})
	}
}

// On a clock of its own, a real run deletes the session past both clocks whole,
// its summary with it; deletes the domain-table and process_errors rows of the
// session past the shorter one, keeping its registration, runs, details, input
// registry and summary, and marks it; refuses the one with a run submitted;
// keeps the recent one. A second run does not select the marked session again.
// The dry run before it reports the same table data and writes nothing.
func TestTableDataOnItsOwnClock(t *testing.T) {
	pool := jobFailureTestDB(t)
	seedTableData(t, pool)
	before := map[string]map[string]int64{}
	for _, id := range []string{"mid", "busy", "new"} {
		before[id] = purgetest.SessionRows(t, pool, id)
	}

	counts := purgetest.RowCounts(t, pool)
	dryLog, err := runTableData(t, pool, "7", true)
	if err != nil {
		t.Fatal(err)
	}
	if after := purgetest.RowCounts(t, pool); !reflect.DeepEqual(after, counts) {
		t.Errorf("the dry run wrote: %v, was %v", after, counts)
	}
	if m := markedSessions(t, pool); len(m) > 0 {
		t.Errorf("the dry run marked %v", m)
	}
	if !strings.Contains(dryLog, "Purging the table data of sessions prior to") ||
		!strings.Contains(dryLog, "DOMAIN_TABLES_RETENTION_DAYS 7") || !strings.Contains(dryLog, "RETENTION_DAYS 30") {
		t.Errorf("the dry run's log does not name both clocks:\n%s", dryLog)
	}
	dryDone, dryDue, dryRefused, dryRows := tableDataReport(t, dryLog)

	realLog, err := runTableData(t, pool, "7", false)
	if err != nil {
		t.Fatal(err)
	}
	done, due, refused, rows := tableDataReport(t, realLog)
	wantRows := map[string]int64{"DomainA": 2, "DomainB": 2, "jetsapi.process_errors": 1}
	if done != 1 || due != 2 || refused != 1 || !reflect.DeepEqual(rows, wantRows) {
		t.Errorf("real run: %d done of %d due, %d refused, rows %v; want 1 of 2, 1 refused, %v",
			done, due, refused, rows, wantRows)
	}
	if dryDone != done || dryDue != due || dryRefused != refused || !reflect.DeepEqual(dryRows, rows) {
		t.Errorf("the dry run reported %d of %d, %d refused, %v; the real run %d of %d, %d refused, %v",
			dryDone, dryDue, dryRefused, dryRows, done, due, refused, rows)
	}
	// The session deletions' report does not count the table data.
	if m := regexp.MustCompile(`(?m)^   (\d+) rows from DomainA$`).FindStringSubmatch(realLog); m == nil || m[1] != "2" {
		t.Errorf("the session deletions' line for DomainA is %v, want old's 2 rows alone", m)
	}

	if left := purgetest.SessionRows(t, pool, "old"); len(left) > 0 {
		t.Errorf("old was not deleted whole: %v", left)
	}
	want := maps.Clone(before["mid"])
	delete(want, "public.DomainA")
	delete(want, "public.DomainB")
	delete(want, "jetsapi.process_errors")
	if got := purgetest.SessionRows(t, pool, "mid"); !reflect.DeepEqual(got, want) {
		t.Errorf("mid holds %v, want %v", got, want)
	}
	for _, table := range []string{"jetsapi.session_registry", "jetsapi.pipeline_execution_status",
		"jetsapi.pipeline_execution_details", "jetsapi.input_registry", "jetsapi.claim_set_summary"} {
		if want[table] == 0 {
			t.Errorf("mid lost its rows in %s", table)
		}
	}
	for _, id := range []string{"busy", "new"} {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
			t.Errorf("%s changed: %v, was %v", id, got, before[id])
		}
	}
	if m := markedSessions(t, pool); !reflect.DeepEqual(m, []string{"mid"}) {
		t.Errorf("marked %v, want mid alone", m)
	}

	// Marked once: the next run selects busy alone and leaves a row written
	// for mid since.
	purgetest.Exec(t, pool, `INSERT INTO public."DomainA" (session_id, v) VALUES ('mid', 'late')`)
	nextLog, err := runTableData(t, pool, "7", false)
	if err != nil {
		t.Fatal(err)
	}
	if done, due, refused, _ := tableDataReport(t, nextLog); done != 0 || due != 1 || refused != 1 {
		t.Errorf("next run: %d done of %d due, %d refused; want busy alone, refused", done, due, refused)
	}
	if got := purgetest.SessionRows(t, pool, "mid")["public.DomainA"]; got != 1 {
		t.Errorf("the next run touched mid: %d rows in DomainA, want 1", got)
	}
	if got := purgeFailures(t, pool); len(got) != 0 {
		t.Errorf("failures recorded: %+v", got)
	}
}

// A value that is not a whole number of days refuses the run before it deletes
// anything, and the refusal is recorded as the purge's failure.
func TestTableDataBadValueRefusesTheRun(t *testing.T) {
	for _, dry := range []bool{false, true} {
		pool := jobFailureTestDB(t)
		seedTableData(t, pool)
		before := purgetest.SessionRows(t, pool, "old")
		_, err := runTableData(t, pool, "a week", dry)
		if err == nil || !strings.Contains(err.Error(), "DOMAIN_TABLES_RETENTION_DAYS") {
			t.Fatalf("dry %v: a bad value gave %v, want the run refused", dry, err)
		}
		if got := purgetest.SessionRows(t, pool, "old"); !reflect.DeepEqual(got, before) {
			t.Errorf("dry %v: the refused run deleted: %v, was %v", dry, got, before)
		}
		got := purgeFailures(t, pool)
		if len(got) != 1 {
			t.Fatalf("dry %v: %d failures recorded, want 1: %+v", dry, len(got), got)
		}
		checkJobRow(t, got[0])
		if !strings.Contains(got[0].details, "the purge stopped: DOMAIN_TABLES_RETENTION_DAYS must be") {
			t.Errorf("dry %v: failure_details %q", dry, got[0].details)
		}
	}
}

// A session whose rows in a profiled feed's claim table feed_detect has not yet
// summarised is held: the dry run reports it held and writes nothing, the real
// run leaves it unmarked with its claim rows, and the other sessions are purged
// as before.
func TestTableDataHoldsUnsummarisedClaims(t *testing.T) {
	pool := jobFailureTestDB(t)
	seedTableData(t, pool)
	purgetest.Seed(t, pool, purgetest.Session{ID: "held", Client: "c1", Registered: true, LastUpdate: daysAgo(10),
		DomainTables: []string{"Claims"}})
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.source_config (object_type, client, table_name, user_email, feed_profile_json)
		VALUES ('Claims', 'c1', 'Claims', 'u@x', '{"claim_set": {"table": "Claims"}}'::jsonb)`)
	before := purgetest.RowCounts(t, pool)

	dryLog, err := runTableData(t, pool, "7", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dryLog, "1 held (claim rows not yet summarised)") ||
		!strings.Contains(dryLog, "table data: held, claim rows feed_detect has not yet summarised: held") {
		t.Errorf("the dry run does not report the held session:\n%s", dryLog)
	}
	if got := purgetest.RowCounts(t, pool); !maps.Equal(got, before) {
		t.Errorf("the dry run wrote: %v, was %v", got, before)
	}

	realLog, err := runTableData(t, pool, "7", false)
	if err != nil {
		t.Fatal(err)
	}
	if done, due, refused, _ := tableDataReport(t, realLog); done != 1 || due != 3 || refused != 1 {
		t.Errorf("done %d of %d due, %d refused; want mid done, busy refused, held held", done, due, refused)
	}
	if got := markedSessions(t, pool); !reflect.DeepEqual(got, []string{"mid"}) {
		t.Errorf("marked %v, want mid alone", got)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM public."Claims" WHERE session_id = 'held'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("the held session has %d claim rows, want 2", n)
	}
}
