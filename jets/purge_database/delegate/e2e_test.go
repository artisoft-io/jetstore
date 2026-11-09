package delegate

// End-to-end tests of the session purge: each seeds a database built from
// jets_schema.json with the shapes a site holds, and runs purgeSessions, the
// purge as the lambda runs it, with a real configuration. They are skipped
// unless JETS_TEST_DSN names a PostgreSQL, and rebuild the database
// srg_purge_e2e. See jets/sessionpurge/purgetest for how to run them, and run
// them with -count=1.
//
// The report is read from the log, as a site reads it, so the dry run and the
// real run are compared on what the lambda actually prints.

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	e2eDB            = "srg_purge_e2e"
	e2eRetentionDays = 30
)

var bg = context.Background()

// e2eReport is one run's report, parsed from the lines logReport writes.
type e2eReport struct {
	Done, Refused, Failed, NotReached, Skipped int
	Archived                                   int64
	ByReason                                   map[string]int
	Rows                                       map[string]int64 // nonzero entries only
	RefusedSessions, FailedSessions            []string
}

var (
	summaryRE = regexp.MustCompile(`^Session purge: (?:DRY RUN, nothing written: )?(\d+) sessions deleted(?: \(([^)]*)\))?, ` +
		`(\d+) refused \(a run pending or submitted\), (\d+) failed, (\d+) not reached before the deadline, ` +
		`(\d+) held by another executor; (\d+) pipeline_execution_status rows archived$`)
	rowsRE    = regexp.MustCompile(`^   (?:DRY RUN, nothing written: )?(\d+) rows from (.+)$`)
	refusedRE = regexp.MustCompile(`^   refused, a run pending or submitted: (.*)$`)
	failedRE  = regexp.MustCompile(`^   failed, retried by the next run: (.*)$`)
)

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// runPurge runs purgeSessions as DoPurgeSessions does, with the dry-run switch
// read from PURGE_DRY_RUN by IsDryRun; unset when env is nil. It returns the
// report the run logged and the whole log.
func runPurge(t *testing.T, pool *pgxpool.Pool, env *string, now, deadline time.Time) (e2eReport, string) {
	t.Helper()
	if env == nil {
		old, had := os.LookupEnv("PURGE_DRY_RUN")
		os.Unsetenv("PURGE_DRY_RUN")
		if had {
			t.Cleanup(func() { os.Setenv("PURGE_DRY_RUN", old) })
		}
	} else {
		t.Setenv("PURGE_DRY_RUN", *env)
	}
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	err := purgeSessions(bg, pool, purgeConfig{RetentionDays: e2eRetentionDays,
		DryRun: IsDryRun(os.Getenv("PURGE_DRY_RUN")), Now: now, Deadline: deadline})
	log.SetFlags(flags)
	log.SetOutput(out)
	if err != nil {
		t.Fatalf("purgeSessions: %v\n%s", err, buf.String())
	}
	r := e2eReport{ByReason: map[string]int{}, Rows: map[string]int64{}}
	found := false
	for _, line := range strings.Split(buf.String(), "\n") {
		if m := summaryRE.FindStringSubmatch(line); m != nil {
			found = true
			r.Done, r.Refused, r.Failed, r.NotReached, r.Skipped = atoi(m[1]), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6])
			r.Archived = int64(atoi(m[7]))
			if m[2] != "" {
				for _, part := range strings.Split(m[2], ", ") {
					reason, n, _ := strings.Cut(part, " ")
					r.ByReason[reason] = atoi(n)
				}
			}
		} else if m := rowsRE.FindStringSubmatch(line); m != nil {
			if n := int64(atoi(m[1])); n != 0 {
				r.Rows[m[2]] = n
			}
		} else if m := refusedRE.FindStringSubmatch(line); m != nil {
			r.RefusedSessions = strings.Split(m[1], ", ")
		} else if m := failedRE.FindStringSubmatch(line); m != nil {
			r.FailedSessions = strings.Split(m[1], ", ")
		}
	}
	if !found {
		t.Fatalf("no report in the log:\n%s", buf.String())
	}
	return r, buf.String()
}

var (
	falseStr = "false"
	realRun  = &falseStr
	dryUnset = (*string)(nil)
)

// msID is a session id as JetStore reserves one: epoch milliseconds.
func msID(at time.Time, n int) string { return strconv.FormatInt(at.UnixMilli()+int64(n), 10) }

func ago(now time.Time, days int) time.Time { return now.AddDate(0, 0, -days) }

// seeder writes rows the way JetStore's writers do, each with the given
// last_update.
type seeder struct {
	t    *testing.T
	pool *pgxpool.Pool
}

func (s seeder) exec(stmt string, args ...any) {
	s.t.Helper()
	purgetest.Exec(s.t, s.pool, stmt, args...)
}

func (s seeder) registry(id, client string, lu time.Time) {
	s.exec(`INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
		VALUES ($1, 'file', $2, 1, 313, 1357, 9497, $3::timestamptz)`, id, client, lu)
}

// domainRows creates the domain table as the loader does, if missing, and
// writes n rows of the session.
func (s seeder) domainRows(table, id string, n int) {
	ident := pgx.Identifier{table}.Sanitize()
	s.exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s ("jets:key" text DEFAULT gen_random_uuid()::text NOT NULL,
		file_key text, session_id text, v text)`, ident))
	s.exec(fmt.Sprintf(`INSERT INTO %s (session_id, v) SELECT $1, g::text FROM generate_series(1, $2) g`,
		ident), id, n)
}

func (s seeder) inputRegistry(id, client, objectType, table, sourceType string, lu time.Time) {
	s.exec(`INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
		VALUES ($1, $2, 'client1/in/file.csv', 1, $3, $4, $5, 'user@example.com', $6::timestamptz)`,
		client, objectType, table, sourceType, id, lu)
}

func (s seeder) loaderStatus(id, client, objectType, table, status string, lu time.Time) {
	s.exec(`INSERT INTO jetsapi.input_loader_status
		(object_type, table_name, client, file_key, source_period_key, session_id, status, user_email, last_update)
		VALUES ($1, $2, $3, 'client1/in/file.csv', 1, $4, $5, 'user@example.com', $6::timestamptz)`,
		objectType, table, client, id, status, lu)
}

func (s seeder) reservation(id string) {
	s.exec(`INSERT INTO jetsapi.session_reservation (session_id) VALUES ($1)`, id)
}

// run writes a pipeline_execution_status row and its details row, and returns
// its key.
func (s seeder) run(id, client, status string, lu time.Time, inputSession string) int {
	s.t.Helper()
	var in *string
	if inputSession != "" {
		in = &inputSession
	}
	var key int
	if err := s.pool.QueryRow(bg, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id, source_period_key,
		 status, user_email, start_time, last_update, workspace_name)
		VALUES (1, $1, 'proc', 'ObjectA', $2, $3, 1, $4, 'user@example.com', $5::timestamptz, $5::timestamptz, 'ws1')
		RETURNING key`, client, in, id, status, lu).Scan(&key); err != nil {
		s.t.Fatal(err)
	}
	s.exec(`INSERT INTO jetsapi.pipeline_execution_details
		(pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
		 session_id, source_period_key, status, user_email, last_update)
		VALUES (1, $1, $2, 'proc', $3, $3, 1, $4, 'user@example.com', $5::timestamptz)`, key, client, id, status, lu)
	return key
}

// m3 writes the rows a session leaves in the tables with no last_update:
// its reservation, a pipeline lock and a coordinator request.
func (s seeder) m3(id string) {
	s.reservation(id)
	s.exec(`INSERT INTO jetsapi.pipeline_lock (state_machine_name, session_id) VALUES ('sm-' || $1, $1)`, id)
	s.exec(`INSERT INTO jetsapi.pipeline_coordinator_map (request_id, nbr_tasks) VALUES ('req-' || $1, 1)`, id)
	s.exec(`INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-' || $1, $1)`, id)
	s.exec(`INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ('req-' || $1, $1)`, id)
}

// The domain tables the orphans write. Names as JetStore makes them: a ':' in
// one, two that differ only by case, and a mixed-case one.
const (
	tblColon   = "ns1:ObjectA"
	tblLower   = "ns1:objecta"
	tblMixed   = "MixedCase_ObjectB"
	tblOutputs = "ns1:Output Table"
)

// seedOrphans writes one unregistered session of each shape the purge must
// sweep, every row last updated at lu, with ids made from lu. It returns their
// ids by shape.
func seedOrphans(s seeder, lu time.Time) map[string]string {
	ids := map[string]string{
		"manual-load": msID(lu, 1),
		"reg-died":    msID(lu, 2),
		"m1":          msID(lu, 3),
		"m2":          msID(lu, 4),
		"m3":          msID(lu, 5),
	}
	// A file loaded by hand: the loader's rows and the loaded rows, no
	// session_registry row ever.
	id := ids["manual-load"]
	s.inputRegistry(id, "client1", "ObjectA", tblColon, "file", lu)
	s.domainRows(tblColon, id, 3)
	s.loaderStatus(id, "client1", "ObjectA", tblColon, "completed", lu)

	// A registration that died after the file was registered and its session
	// reserved.
	id = ids["reg-died"]
	s.inputRegistry(id, "client1", "ObjectB", tblMixed, "file", lu)
	s.reservation(id)

	// M1: a pipeline session that never registered.
	id = ids["m1"]
	s.inputRegistry(id, "client1", "ObjectA", tblColon, "file", lu)
	s.inputRegistry(id, "client1", "ObjectA", tblLower, "domain_table", lu)
	s.domainRows(tblColon, id, 2)
	s.domainRows(tblLower, id, 4)
	s.run(id, "client1", "failed", lu, "")

	// M2: its session_registry row is gone and its rows are not.
	id = ids["m2"]
	s.inputRegistry(id, "client2", "ObjectB", tblMixed, "domain_table", lu)
	s.domainRows(tblMixed, id, 5)

	// M3: only the rows that carry no last_update.
	s.m3(ids["m3"])
	return ids
}

func values(m map[string]string) []string {
	var out []string
	for _, v := range m {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func deletionState(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	err := pool.QueryRow(bg, `SELECT status || ' ' || reason || ' ' || client FROM jetsapi.session_deletion
		WHERE session_id = $1`, id).Scan(&s)
	if err != nil {
		return "none"
	}
	return s
}

func one(t *testing.T, pool *pgxpool.Pool, stmt string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(bg, stmt, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, stmt)
	}
	return n
}

// expectRows sums the seeded rows of the sessions a run deleted, keyed as the
// report keys them: domain tables by their input_registry name.
func expectRows(before map[string]map[string]int64, ids []string) map[string]int64 {
	out := map[string]int64{}
	for _, id := range ids {
		for table, n := range before[id] {
			out[strings.TrimPrefix(table, "public.")] += n
		}
	}
	return out
}

// reportRowsWithoutArchive drops the archive entry, which no SessionRows count
// carries.
func reportRowsWithoutArchive(r e2eReport) map[string]int64 {
	out := map[string]int64{}
	for k, v := range r.Rows {
		if k != sessionpurge.ArchiveTable {
			out[k] = v
		}
	}
	return out
}

// C.1. Each way rows outlive their session is swept by one real run once older
// than the retention window; the same shapes recent are kept; a live session
// with rows in the same domain tables is untouched; a session with a run pending
// or submitted on it, or reading it, is refused and stays queued until the run
// is over.
func TestE2EOrphansSweptByOneRun(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	old, recent := ago(now, 40), ago(now, 5)
	oldIDs := seedOrphans(s, old)
	newIDs := seedOrphans(s, recent)

	live := msID(recent, 100)
	purgetest.Seed(t, pool, purgetest.Session{ID: live, Client: "client1", Registered: true, LastUpdate: recent,
		DomainTables: []string{tblColon, tblLower, tblMixed}})
	// An M1 session whose run is still submitted.
	busy := msID(old, 101)
	s.inputRegistry(busy, "client1", "ObjectA", tblColon, "file", old)
	s.domainRows(tblColon, busy, 2)
	busyKey := s.run(busy, "client1", "submitted", old, "")
	// A manual load that the live session's pending run reads.
	read := msID(old, 102)
	s.inputRegistry(read, "client1", "ObjectA", tblColon, "file", old)
	s.domainRows(tblColon, read, 2)
	s.loaderStatus(read, "client1", "ObjectA", tblColon, "completed", old)
	readerKey := s.run(live, "client1", "pending", recent, read)

	all := append(append(values(oldIDs), values(newIDs)...), live, busy, read)
	before := map[string]map[string]int64{}
	for _, id := range all {
		before[id] = purgetest.SessionRows(t, pool, id)
		if len(before[id]) == 0 {
			t.Fatalf("seed wrote nothing for %s", id)
		}
	}

	report, logged := runPurge(t, pool, realRun, now, now.Add(runBudget))

	swept := values(oldIDs)
	for shape, id := range oldIDs {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("old %s orphan %s: rows left %v", shape, id, left)
		}
		want := "done orphan client1"
		switch shape {
		case "m2":
			want = "done orphan client2"
		case "m3":
			want = "done orphan " + sessionpurge.ClientUnknown
		}
		if got := deletionState(t, pool, id); got != want {
			t.Errorf("old %s orphan: deletion row %q, want %q", shape, got, want)
		}
	}
	for _, id := range append(values(newIDs), busy, read) {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
			t.Errorf("%s changed: %v, was %v", id, got, before[id])
		}
	}
	for shape, id := range newIDs {
		if got := deletionState(t, pool, id); got != "none" {
			t.Errorf("recent %s orphan queued: %q", shape, got)
		}
	}
	for _, id := range []string{busy, read} {
		if got := deletionState(t, pool, id); got != "pending orphan client1" {
			t.Errorf("refused %s: deletion row %q, want pending", id, got)
		}
	}
	if got := purgetest.SessionRows(t, pool, live); !reflect.DeepEqual(got, before[live]) {
		t.Errorf("the live session changed: %v, was %v", got, before[live])
	}

	if report.Done != len(swept) || report.ByReason["orphan"] != len(swept) || report.Failed != 0 || report.NotReached != 0 {
		t.Errorf("report %+v, want %d orphans done", report, len(swept))
	}
	if want := []string{busy, read}; report.Refused != 2 || !slices.Equal(report.RefusedSessions, want) {
		t.Errorf("refused %d %v, want %v", report.Refused, report.RefusedSessions, want)
	}
	if want := expectRows(before, swept); !reflect.DeepEqual(reportRowsWithoutArchive(report), want) {
		t.Errorf("report rows %v\nwant (the seed of the swept sessions) %v", report.Rows, want)
	}
	if report.Archived != 1 || report.Rows[sessionpurge.ArchiveTable] != 1 {
		t.Errorf("archived %d, want the M1 run", report.Archived)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive
		WHERE session_id = $1 AND archive_reason = 'orphan' AND status = 'failed'`, oldIDs["m1"]); n != 1 {
		t.Errorf("the M1 run is not in the archive (%d)", n)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = 'req-' || $1`,
		oldIDs["m3"]); n != 0 {
		t.Errorf("the M3 session's coordinator map row is left")
	}
	if t.Failed() {
		t.Log(logged)
	}

	// The runs end. The next run deletes the two it refused.
	s.exec(`UPDATE jetsapi.pipeline_execution_status SET status = 'completed', last_update = now()
		WHERE key IN ($1, $2)`, busyKey, readerKey)
	liveBefore := purgetest.SessionRows(t, pool, live)
	report, logged = runPurge(t, pool, realRun, time.Now(), time.Now().Add(runBudget))
	if report.Done != 2 || report.Refused != 0 || report.ByReason["orphan"] != 2 {
		t.Errorf("second run report %+v, want the two refused sessions done", report)
	}
	for _, id := range []string{busy, read} {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("%s: rows left after its run completed: %v", id, left)
		}
		if got := deletionState(t, pool, id); got != "done orphan client1" {
			t.Errorf("%s: deletion row %q, want done", id, got)
		}
	}
	if got := purgetest.SessionRows(t, pool, live); !reflect.DeepEqual(got, liveBefore) {
		t.Errorf("the live session changed: %v, was %v", got, liveBefore)
	}
	if t.Failed() {
		t.Log(logged)
	}
}

// C.1, M3 (after A.6). A session that left rows only in session_reservation,
// pipeline_lock and the coordinator tables, none of which has a last_update, is
// dated by its id: an old numeric id is swept with client (unknown), a recent one
// is kept, and an id that is not numeric is never a candidate and is counted in
// the log.
func TestE2EOrphanM3(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	oldID, recentID, word := msID(ago(now, 40), 0), msID(ago(now, 5), 0), "m3-not-a-number"
	for _, id := range []string{oldID, recentID, word} {
		s.m3(id)
	}
	// An all-digit id too long for epoch milliseconds is not dated either.
	long := "12345678901234567890"
	s.m3(long)
	before := map[string]map[string]int64{}
	for _, id := range []string{oldID, recentID, word, long} {
		before[id] = purgetest.SessionRows(t, pool, id)
	}

	dry, dryLog := runPurge(t, pool, dryUnset, now, now.Add(runBudget))
	report, logged := runPurge(t, pool, realRun, now, now.Add(runBudget))
	if !reflect.DeepEqual(dry, report) {
		t.Errorf("dry run %+v\nreal run %+v", dry, report)
	}
	if left := purgetest.SessionRows(t, pool, oldID); len(left) > 0 {
		t.Errorf("old M3 rows left: %v", left)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = 'req-' || $1`, oldID); n != 0 {
		t.Errorf("old M3 coordinator map row left")
	}
	if got := deletionState(t, pool, oldID); got != "done orphan "+sessionpurge.ClientUnknown {
		t.Errorf("old M3 deletion row %q", got)
	}
	for _, id := range []string{recentID, word, long} {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
			t.Errorf("%s changed: %v, was %v", id, got, before[id])
		}
		if got := deletionState(t, pool, id); got != "none" {
			t.Errorf("%s queued: %q", id, got)
		}
	}
	if report.Done != 1 || report.ByReason["orphan"] != 1 {
		t.Errorf("report %+v, want one orphan", report)
	}
	want := map[string]int64{"jetsapi.session_reservation": 1, "jetsapi.pipeline_lock": 1,
		"jetsapi.pipeline_coordinator_map": 1, "jetsapi.pipeline_coordinator_map_items": 1,
		"jetsapi.pipeline_coordinator_lock": 1}
	if !reflect.DeepEqual(report.Rows, want) {
		t.Errorf("report rows %v, want %v", report.Rows, want)
	}
	if !strings.Contains(logged, "2 unregistered session ids are not numeric") ||
		!strings.Contains(dryLog, "2 unregistered session ids are not numeric") {
		t.Errorf("the two undatable ids are not counted in the log:\n%s", logged)
	}
}

// Every jetsapi table with a session_id column, bar the archive and the
// deletion queue, holds a row of the session before the run and none after.
// The table list is read from the database, so a table added to jets_schema.json
// with a session_id column fails here until it is seeded and purged.
func TestE2EEverySessionTableCleared(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	for _, c := range []struct {
		id         string
		registered bool
	}{{msID(ago(now, 40), 1), true}, {msID(ago(now, 40), 2), false}} {
		keys := purgetest.Seed(t, pool, purgetest.Session{ID: c.id, Client: "client1", Registered: c.registered,
			LastUpdate: ago(now, 40), DomainTables: []string{tblColon, tblOutputs}, Runs: []string{"completed", "errors"}})
		s.exec(`INSERT INTO jetsapi.cpipes_execution_status (pipeline_execution_status_key, session_id) VALUES ($1, $2)`, keys[0], c.id)
		s.exec(`INSERT INTO jetsapi.cpipes_execution_status_details
			(session_id, process_name, cpipes_step_id, nbr_nodes, total_input_files_size_mb, total_input_records_count,
			 total_output_records_count) VALUES ($1, 'proc', 'reducing0', 1, 1, 10, 10)`, c.id)
		s.exec(`UPDATE jetsapi.pipeline_execution_status SET input_session_id = $1 WHERE session_id = $1`, c.id)
	}
	ids := []string{msID(ago(now, 40), 1), msID(ago(now, 40), 2)}
	rows, err := pool.Query(bg, `SELECT c.table_name FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = 'jetsapi' AND c.column_name = 'session_id' AND t.table_type = 'BASE TABLE'
			AND c.table_name NOT IN ('pipeline_execution_status_archive', 'session_deletion')`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		tables = append(tables, "jetsapi."+n)
	}
	rows.Close()
	for _, id := range ids {
		have := purgetest.SessionRows(t, pool, id)
		for _, table := range tables {
			if table == "jetsapi.session_registry" && id == ids[1] {
				continue
			}
			if have[table] == 0 {
				t.Errorf("the seed wrote no %s row for %s: seed it, so this test covers it", table, id)
			}
		}
	}
	runPurge(t, pool, realRun, now, now.Add(runBudget))
	for _, id := range ids {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("%s: rows left %v", id, left)
		}
	}
}

// statusSnapshot reads every row of a table with pipeline_execution_status's
// columns, by key, as jsonb text without the archive's own two columns.
func statusSnapshot(t *testing.T, pool *pgxpool.Pool, from string) map[int]string {
	t.Helper()
	rows, err := pool.Query(bg, `SELECT key, (to_jsonb(s) - 'archived_at' - 'archive_reason')::text FROM `+from+` s`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var k int
		var v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		if _, dup := out[k]; dup {
			t.Errorf("%s holds key %d twice", from, k)
		}
		out[k] = v
	}
	return out
}

func rowCountsWithView(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	counts := purgetest.RowCounts(t, pool)
	counts["jetsapi.pipeline_execution_history (view)"] = one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_history`)
	return counts
}

func diffCounts(t *testing.T, what string, before, after map[string]int64) {
	t.Helper()
	for k := range after {
		if after[k] != before[k] {
			t.Errorf("%s changed %s: %d rows, was %d", what, k, after[k], before[k])
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			t.Errorf("%s removed table %s", what, k)
		}
	}
}

// C.2. The first run at a site, replayed: about six months of runs across many
// sessions, a third of them for sessions whose session_registry rows the old
// purge already removed, some registered past the retention window, some recent,
// one past the window with a run still submitted, and the orphan backlog of C.1.
// A dry run (PURGE_DRY_RUN unset) writes nothing and its report is the real
// run's, field by field; after the real run the live table holds only the runs of
// registered or recent sessions, every removed run is in the archive with its key
// and content, the view loses nothing, and session_registry lost exactly the
// retention candidates. A second run changes nothing but the archive purge.
func TestE2EFirstRunReplay(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	cutoff := ago(now, e2eRetentionDays)

	var registered, retained []string // registered: all; retained: those past the window
	var goneIDs []string
	gone := 0
	for i := 0; i < 88; i++ {
		lu := ago(now, 2*i).Add(-time.Hour) // 0 to 174 days, never on the cutoff
		id := msID(lu, 0)
		client := []string{"client1", "client2"}[i%2]
		statuses := []string{"completed", "failed", "errors"}[:1+i%3]
		if i%3 == 0 && lu.Before(cutoff) {
			// The old purge deleted the session and left its runs.
			for _, st := range statuses {
				s.run(id, client, st, lu, "")
			}
			s.exec(`DELETE FROM jetsapi.pipeline_execution_details WHERE session_id = $1`, id)
			goneIDs = append(goneIDs, id)
			gone++
			continue
		}
		tables := []string{tblColon}
		if i%2 == 1 {
			tables = append(tables, tblLower, tblOutputs)
		}
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: client, Registered: true, LastUpdate: lu,
			DomainTables: tables, Runs: statuses})
		registered = append(registered, id)
		if !lu.After(cutoff) {
			retained = append(retained, id)
		}
	}
	busy := msID(ago(now, 60), 7)
	purgetest.Seed(t, pool, purgetest.Session{ID: busy, Client: "client1", Registered: true, LastUpdate: ago(now, 60),
		DomainTables: []string{tblMixed}, Runs: []string{"completed", "submitted"}})
	registered = append(registered, busy)
	oldOrphans := seedOrphans(s, ago(now, 40))
	seedOrphans(s, ago(now, 3))
	if gone < 25 || len(retained) < 25 {
		t.Fatalf("seed: %d gone, %d retained", gone, len(retained))
	}
	t.Logf("seeded %d registered sessions (%d past retention), %d sessions already gone, %d status rows",
		len(registered), len(retained), gone, one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status`))

	deleted := append(append(slices.Clone(retained), goneIDs...), values(oldOrphans)...)
	sessionBefore := map[string]map[string]int64{}
	for _, id := range append(slices.Clone(registered), deleted...) {
		sessionBefore[id] = purgetest.SessionRows(t, pool, id)
	}
	statusBefore := statusSnapshot(t, pool, "jetsapi.pipeline_execution_status")
	countsBefore := rowCountsWithView(t, pool)

	// The dry run, as a site's first run after the release.
	dry, dryLog := runPurge(t, pool, dryUnset, now, now.Add(runBudget))
	if !strings.Contains(dryLog, "dry run, PURGE_DRY_RUN is not false") {
		t.Errorf("an unset PURGE_DRY_RUN did not run dry:\n%s", dryLog)
	}
	diffCounts(t, "the dry run", countsBefore, rowCountsWithView(t, pool))
	if got := statusSnapshot(t, pool, "jetsapi.pipeline_execution_status"); !reflect.DeepEqual(got, statusBefore) {
		t.Errorf("the dry run changed pipeline_execution_status")
	}
	wantDone := len(retained) + gone + len(oldOrphans)
	if dry.Done != wantDone || dry.ByReason["retention"] != len(retained) || dry.ByReason["orphan"] != gone+len(oldOrphans) ||
		!slices.Equal(dry.RefusedSessions, []string{busy}) || dry.Failed != 0 || dry.NotReached != 0 {
		t.Errorf("dry run report %+v, want %d retention, %d orphans and %s refused", dry, len(retained), gone+len(oldOrphans), busy)
	}

	report, logged := runPurge(t, pool, realRun, now, now.Add(runBudget))
	if !reflect.DeepEqual(dry, report) {
		t.Errorf("the dry run predicted\n%+v\nthe real run did\n%+v", dry, report)
		for k, v := range report.Rows {
			if dry.Rows[k] != v {
				t.Errorf("  %s: dry %d, real %d", k, dry.Rows[k], v)
			}
		}
	}

	// Every deleted session's rows are gone from every table, and the report
	// counts exactly them; every other session is untouched.
	for _, id := range deleted {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("%s: rows left %v", id, left)
		}
	}
	for _, id := range registered {
		if slices.Contains(retained, id) {
			continue
		}
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, sessionBefore[id]) {
			t.Errorf("%s, not a candidate, changed: %v, was %v", id, got, sessionBefore[id])
		}
	}
	if want := expectRows(sessionBefore, deleted); !reflect.DeepEqual(reportRowsWithoutArchive(report), want) {
		t.Errorf("report rows %v\nwant (the seed of the deleted sessions) %v", report.Rows, want)
	}

	// The live table holds only the runs of registered sessions, or recent ones.
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status s
		WHERE last_update <= $1::timestamptz
			AND NOT EXISTS (SELECT 1 FROM jetsapi.session_registry r WHERE r.session_id = s.session_id)`, cutoff); n != 0 {
		t.Errorf("%d runs of unregistered sessions past the window are still live", n)
	}
	// Every removed run is in the archive, key and content; nothing else is.
	live := statusSnapshot(t, pool, "jetsapi.pipeline_execution_status")
	archive := statusSnapshot(t, pool, "jetsapi.pipeline_execution_status_archive")
	for k, v := range statusBefore {
		if _, ok := live[k]; ok {
			continue
		}
		if archive[k] != v {
			t.Errorf("run %d left the live table and is not in the archive as it was:\n live was %s\n archived %s", k, v, archive[k])
		}
	}
	for k := range archive {
		if _, ok := statusBefore[k]; !ok {
			t.Errorf("the archive holds run %d, which was not a live run", k)
		}
		if _, ok := live[k]; ok {
			t.Errorf("run %d is both live and archived", k)
		}
	}
	if int64(len(archive)) != report.Archived {
		t.Errorf("the archive holds %d runs, the report says %d", len(archive), report.Archived)
	}
	// The view returns live and archived runs with no loss.
	if got := statusSnapshot(t, pool, "jetsapi.pipeline_execution_history"); !reflect.DeepEqual(got, statusBefore) {
		t.Errorf("pipeline_execution_history returns %d runs, there were %d", len(got), len(statusBefore))
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive
		WHERE archived_at IS NULL OR archive_reason NOT IN ('retention', 'orphan')`); n != 0 {
		t.Errorf("%d archived runs with no archived_at or an unexpected reason", n)
	}
	// session_registry lost exactly the retention candidates, the refused one aside.
	rows, err := pool.Query(bg, `SELECT session_id FROM jetsapi.session_registry ORDER BY session_id`)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		left = append(left, id)
	}
	rows.Close()
	var want []string
	for _, id := range registered {
		if !slices.Contains(retained, id) {
			want = append(want, id)
		}
	}
	sort.Strings(want)
	if !slices.Equal(left, want) {
		t.Errorf("session_registry holds %d sessions, want the %d not past retention (busy included)", len(left), len(want))
	}
	if t.Failed() {
		t.Log(logged)
	}

	// The second run. One archived run is aged past 180 days first, so the archive
	// purge has something to do; nothing else may change.
	var aged int
	if err := pool.QueryRow(bg, `UPDATE jetsapi.pipeline_execution_status_archive SET last_update = now() - interval '200 days'
		WHERE key = (SELECT min(key) FROM jetsapi.pipeline_execution_status_archive) RETURNING key`).Scan(&aged); err != nil {
		t.Fatal(err)
	}
	countsBefore = rowCountsWithView(t, pool)
	second, logged := runPurge(t, pool, realRun, now, now.Add(runBudget))
	countsAfter := rowCountsWithView(t, pool)
	countsBefore["jetsapi.pipeline_execution_status_archive"]--
	countsBefore["jetsapi.pipeline_execution_history (view)"]--
	diffCounts(t, "the second run", countsBefore, countsAfter)
	if second.Done != 0 || second.Refused != 1 || second.Archived != 0 {
		t.Errorf("second run report %+v, want nothing done and the busy session refused again", second)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive WHERE key = $1`, aged); n != 0 {
		t.Errorf("the aged archived run %d was not purged", aged)
	}
	if t.Failed() {
		t.Log(logged)
	}
}

// A run that passes its deadline between sessions finishes the session in hand
// and starts no other; those stay pending with every row, the rest of the run
// (archive and tombstone purges) still happens, and the next run takes them.
func TestE2EDeadlineBetweenSessions(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	now := time.Now()
	old := ago(now, 40)
	ids := []string{msID(old, 1), msID(old, 2), msID(old, 3)}
	for _, id := range ids {
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "client1", Registered: true, LastUpdate: old,
			DomainTables: []string{tblColon}})
	}
	// The first session (lowest id, taken first) deletes slowly: two rows of a
	// table whose row trigger sleeps a second.
	s := seeder{t, pool}
	s.domainRows("SlowTable", ids[0], 2)
	s.inputRegistry(ids[0], "client1", "ObjectA", "SlowTable", "domain_table", old)
	s.exec(`CREATE FUNCTION public.slow() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM pg_sleep(1); RETURN OLD; END $$;
		CREATE TRIGGER slow BEFORE DELETE ON public."SlowTable" FOR EACH ROW EXECUTE FUNCTION public.slow()`)
	before := map[string]map[string]int64{}
	for _, id := range ids {
		before[id] = purgetest.SessionRows(t, pool, id)
	}

	start := time.Now()
	report, logged := runPurge(t, pool, realRun, now, start.Add(time.Second))
	if report.Done != 1 || report.NotReached != 2 {
		t.Errorf("report %+v, want one done and two not reached", report)
	}
	if left := purgetest.SessionRows(t, pool, ids[0]); len(left) > 0 {
		t.Errorf("the session in hand at the deadline was not finished: %v", left)
	}
	for _, id := range ids[1:] {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
			t.Errorf("%s changed after the deadline: %v, was %v", id, got, before[id])
		}
		if got := deletionState(t, pool, id); got != "pending retention client1" {
			t.Errorf("%s: deletion row %q, want pending", id, got)
		}
	}
	if !strings.Contains(logged, "Purged 0 archived pipeline runs") || !strings.Contains(logged, "session deletion tombstones") {
		t.Errorf("the run stopped at the deadline instead of finishing its purges:\n%s", logged)
	}

	report, _ = runPurge(t, pool, realRun, time.Now(), time.Now().Add(runBudget))
	if report.Done != 2 || report.NotReached != 0 {
		t.Errorf("next run report %+v, want the two left done", report)
	}
}

// A failed deletion is retried by every run, keeps its reason, is not re-queued
// over, and goes once the cause is gone; until then nothing of it is deleted.
func TestE2EFailedDeletionRetriedEveryRun(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	old := ago(now, 40)
	reg, orphan := msID(old, 1), msID(old, 2)
	purgetest.Seed(t, pool, purgetest.Session{ID: reg, Client: "client1", Registered: true, LastUpdate: old,
		DomainTables: []string{"RefuseTable"}})
	s.inputRegistry(orphan, "client1", "ObjectA", "RefuseTable", "file", old)
	s.domainRows("RefuseTable", orphan, 2)
	s.run(orphan, "client1", "failed", old, "")
	s.exec(`CREATE FUNCTION public.refuse() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'delete refused by test trigger'; END $$;
		CREATE TRIGGER refuse BEFORE DELETE ON public."RefuseTable" FOR EACH ROW EXECUTE FUNCTION public.refuse()`)
	before := map[string]map[string]int64{reg: purgetest.SessionRows(t, pool, reg), orphan: purgetest.SessionRows(t, pool, orphan)}

	for run := 1; run <= 2; run++ {
		report, logged := runPurge(t, pool, realRun, time.Now(), time.Now().Add(runBudget))
		if report.Failed != 2 || report.Done != 0 || !slices.Equal(report.FailedSessions, []string{reg, orphan}) {
			t.Errorf("run %d: report %+v, want both failed", run, report)
		}
		if !strings.Contains(logged, "delete refused by test trigger") {
			t.Errorf("run %d: the error is not in the log", run)
		}
		for id, want := range map[string]string{reg: "failed retention client1", orphan: "failed orphan client1"} {
			if got := deletionState(t, pool, id); got != want {
				t.Errorf("run %d: %s deletion row %q, want %q", run, id, got, want)
			}
			if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, before[id]) {
				t.Errorf("run %d: %s half deleted: %v, was %v", run, id, got, before[id])
			}
			var e *string
			_ = pool.QueryRow(bg, `SELECT error FROM jetsapi.session_deletion WHERE session_id = $1`, id).Scan(&e)
			if e == nil || !strings.Contains(*e, "delete refused by test trigger") {
				t.Errorf("run %d: %s error %v", run, id, e)
			}
		}
	}
	s.exec(`DROP TRIGGER refuse ON public."RefuseTable"`)
	report, _ := runPurge(t, pool, realRun, time.Now(), time.Now().Add(runBudget))
	if report.Done != 2 || report.Failed != 0 || report.ByReason["retention"] != 1 || report.ByReason["orphan"] != 1 {
		t.Errorf("after the cause is gone: %+v, want both done", report)
	}
	for _, id := range []string{reg, orphan} {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("%s: rows left %v", id, left)
		}
	}
}

// A coordinator request whose items are two sessions, both past retention. The
// request's map row goes with the last of its items, so with the second session;
// the dry run, which runs every session under a savepoint of one transaction,
// sees the first session's item gone as the real run does. The dry run's report
// must be the real run's. (Skipped until jetstore#2406's defect was fixed: the dry
// run rolled each session back and counted the map row twice.)
func TestE2ECoordinatorRequestSharedBySessions(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	old := ago(now, 40)
	a, b := msID(old, 1), msID(old, 2)
	for _, id := range []string{a, b} {
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "client1", Registered: true, LastUpdate: old})
	}
	s.exec(`INSERT INTO jetsapi.pipeline_coordinator_map (request_id, nbr_tasks) VALUES ('req-shared', 2)`)
	s.exec(`INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-shared', $1), ('req-shared', $2)`, a, b)

	dry, _ := runPurge(t, pool, dryUnset, now, now.Add(runBudget))
	done, _ := runPurge(t, pool, realRun, now, now.Add(runBudget))
	if !reflect.DeepEqual(dry, done) {
		t.Errorf("the dry run predicted\n%+v\nthe real run did\n%+v", dry, done)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map`); n != 0 {
		t.Errorf("%d coordinator map rows left", n)
	}
}

// An archived run already past 180 days when its session is deleted is archived
// and purged in the same run: the plan keeps an archived run as long as the live
// table would have (§3.4, Q-2), so it is gone from the view too. This records the
// behaviour; it is what the plan asks for.
func TestE2EArchivedPast180DaysPurgedSameRun(t *testing.T) {
	pool := purgetest.DB(t, e2eDB)
	s := seeder{t, pool}
	now := time.Now()
	id := msID(ago(now, 40), 1)
	purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "client1", Registered: true, LastUpdate: ago(now, 40)})
	oldKey := s.run(id, "client1", "completed", ago(now, 200), "")
	report, logged := runPurge(t, pool, realRun, now, now.Add(runBudget))
	if report.Archived != 2 {
		t.Errorf("archived %d, want both runs", report.Archived)
	}
	if !strings.Contains(logged, "Purged 1 archived pipeline runs") {
		t.Errorf("the 200-day run was not purged from the archive in the same run:\n%s", logged)
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_history WHERE key = $1`, oldKey); n != 0 {
		t.Errorf("the 200-day run is still in the view")
	}
	if n := one(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_history WHERE session_id = $1`, id); n != 1 {
		t.Errorf("the view holds %d runs of the session, want the recent one", n)
	}
}
