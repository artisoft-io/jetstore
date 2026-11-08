package sessionpurge

// Tests against a real PostgreSQL, skipped unless JETS_TEST_DSN names one; each
// rebuilds the database srg_sessionpurge. See jets/sessionpurge/purgetest for how
// to run them.

import (
	"context"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDB = "srg_sessionpurge"

var ctx = context.Background()

func days(n int) time.Time { return time.Now().AddDate(0, 0, -n) }

func queue(t *testing.T, pool *pgxpool.Pool, reqs ...Request) int {
	t.Helper()
	n, err := Queue(ctx, pool, reqs)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func retention(id string) Request {
	return Request{SessionID: id, Client: "c1", Reason: ReasonRetention, RequestedBy: RequestedByPurge}
}

type deletionRow struct {
	Reason, RequestedBy, Status string
	Error                       *string
	Completed                   bool
}

func deletion(t *testing.T, pool *pgxpool.Pool, id string) *deletionRow {
	t.Helper()
	var r deletionRow
	var completed *time.Time
	err := pool.QueryRow(ctx, `SELECT reason, requested_by, status, error, completed_at
		FROM jetsapi.session_deletion WHERE session_id = $1`, id).Scan(&r.Reason, &r.RequestedBy, &r.Status, &r.Error, &completed)
	if err != nil {
		return nil
	}
	r.Completed = completed != nil
	return &r
}

func count(t *testing.T, pool *pgxpool.Pool, stmt string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(ctx, stmt, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, stmt)
	}
	return n
}

// The archive, the Go list the executor copies, and the view all name every
// column of pipeline_execution_status. No database needed.
func TestStatusColumnsAgree(t *testing.T) {
	defs := purgetest.SchemaDefinitions(t)
	type col struct {
		name, dataType string
		array          bool
	}
	columns := func(table string) []col {
		for _, d := range defs {
			if d.TableName == table {
				var out []col
				for _, c := range d.Columns {
					if !c.Deleted {
						out = append(out, col{c.ColumnName, c.DataType, c.IsArray})
					}
				}
				return out
			}
		}
		t.Fatalf("jets_schema.json has no table %s", table)
		return nil
	}
	status := columns("pipeline_execution_status")
	var names []string
	for _, c := range status {
		names = append(names, c.name)
	}
	if !slices.Equal(names, statusColumns) {
		t.Errorf("statusColumns is\n%v\npipeline_execution_status has\n%v", statusColumns, names)
	}
	want := append(slices.Clone(status), col{"archived_at", "datetime", false}, col{"archive_reason", "text", false})
	if got := columns("pipeline_execution_status_archive"); !reflect.DeepEqual(got, want) {
		t.Errorf("pipeline_execution_status_archive has\n%v\nwant the status table's columns and the two archive columns\n%v", got, want)
	}
	view := purgetest.HistoryViewStatement(t)
	sides := strings.Split(view, "UNION ALL")
	if len(sides) != 2 {
		t.Fatalf("the view is not two SELECTs joined by UNION ALL:\n%s", view)
	}
	selectList := regexp.MustCompile(`(?s)SELECT(.*?)\nFROM`)
	for i, side := range sides {
		m := selectList.FindStringSubmatch(side)
		if m == nil {
			t.Fatalf("no select list in side %d of the view", i+1)
		}
		var got []string
		for _, f := range strings.Split(m[1], ",") {
			got = append(got, strings.TrimSpace(f))
		}
		wantList := append(slices.Clone(statusColumns), "NULL::timestamp AS archived_at", "NULL::text AS archive_reason")
		if i == 1 {
			wantList = append(slices.Clone(statusColumns), "archived_at", "archive_reason")
		}
		if !slices.Equal(got, wantList) {
			t.Errorf("side %d of the view selects\n%v\nwant\n%v", i+1, got, wantList)
		}
	}
}

// A deleted session's rows are gone from every table that carries its id; its
// status rows are in the archive with their keys and are returned by the view;
// another session is untouched.
func TestExecuteDeletesEverywhere(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	keys := purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true,
		LastUpdate: days(200), DomainTables: []string{"DomainA", "Domain B"}, Runs: []string{"completed", "failed"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s2", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	before := purgetest.SessionRows(t, pool, "s1")
	for _, table := range []string{"public.DomainA", "public.Domain B", "jetsapi.input_registry", "jetsapi.input_loader_status",
		"jetsapi.session_registry", "jetsapi.pipeline_execution_status", "jetsapi.pipeline_coordinator_map"} {
		if before[table] == 0 {
			t.Fatalf("seed wrote no %s row for s1: %v", table, before)
		}
	}
	keep := purgetest.SessionRows(t, pool, "s2")

	queue(t, pool, retention("s1"))
	report, err := Execute(ctx, pool, Options{Deadline: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Failed != 0 || report.Refused != 0 || report.ByReason[ReasonRetention] != 1 {
		t.Errorf("report %+v, want one retention session done", report)
	}
	if left := purgetest.SessionRows(t, pool, "s1"); len(left) > 0 {
		t.Errorf("rows of s1 left after its deletion: %v", left)
	}
	for table, n := range before {
		// Domain tables are reported by their input_registry name, unqualified.
		table = strings.TrimPrefix(table, "public.")
		if report.Rows[table] != n {
			t.Errorf("report says %d rows from %s, the seed wrote %d", report.Rows[table], table, n)
		}
	}
	if report.Rows[ArchiveTable] != 2 {
		t.Errorf("report says %d status rows archived, want 2", report.Rows[ArchiveTable])
	}
	if got := purgetest.SessionRows(t, pool, "s2"); !reflect.DeepEqual(got, keep) {
		t.Errorf("s2 changed: %v, was %v", got, keep)
	}
	rows, err := pool.Query(ctx, `SELECT key FROM jetsapi.pipeline_execution_status_archive
		WHERE session_id = 's1' AND archive_reason = 'retention' AND archived_at IS NOT NULL ORDER BY key`)
	if err != nil {
		t.Fatal(err)
	}
	var archived []int
	for rows.Next() {
		var k int
		_ = rows.Scan(&k)
		archived = append(archived, k)
	}
	rows.Close()
	if !slices.Equal(archived, keys) {
		t.Errorf("archived keys %v, want the status rows' keys %v", archived, keys)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_history
		WHERE (session_id = 's1' AND archive_reason = 'retention') OR (session_id = 's2' AND archived_at IS NULL)`); n != 3 {
		t.Errorf("the view returns %d of the 2 archived and 1 live runs", n)
	}
	if d := deletion(t, pool, "s1"); d == nil || d.Status != StatusDone || !d.Completed || d.Error != nil {
		t.Errorf("deletion row %+v, want done with completed_at and no error", d)
	}
}

// session_registry is the last table the session is deleted from.
func TestSessionRegistryDeletedLast(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	purgetest.Exec(t, pool, `CREATE TABLE public.delete_log (seq serial, tbl text);
		CREATE FUNCTION public.log_delete() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN INSERT INTO public.delete_log (tbl) VALUES (TG_TABLE_SCHEMA || '.' || TG_TABLE_NAME); RETURN OLD; END $$`)
	tables := mapKeys(purgetest.SessionRows(t, pool, "s1")) // the coordinator map included
	for i, table := range tables {
		schema, name, _ := strings.Cut(table, ".")
		purgetest.Exec(t, pool, `CREATE TRIGGER log_delete_`+string(rune('a'+i))+` AFTER DELETE ON "`+schema+`"."`+name+
			`" FOR EACH ROW EXECUTE FUNCTION public.log_delete()`)
	}
	queue(t, pool, retention("s1"))
	if _, err := Execute(ctx, pool, Options{}); err != nil {
		t.Fatal(err)
	}
	var last string
	if err := pool.QueryRow(ctx, `SELECT tbl FROM public.delete_log ORDER BY seq DESC LIMIT 1`).Scan(&last); err != nil {
		t.Fatal(err)
	}
	if last != "jetsapi.session_registry" {
		t.Errorf("the last delete was from %s, want jetsapi.session_registry", last)
	}
	if n := count(t, pool, `SELECT count(DISTINCT tbl) FROM public.delete_log`); n != int64(len(tables)) {
		t.Errorf("deletes logged from %d tables, want %d", n, len(tables))
	}
}

func mapKeys(m map[string]int64) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// A session with a run on it, or a run reading it, that is pending or submitted
// is refused and stays pending with every row in place.
func TestActiveRunRefuses(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "on", Client: "c1", Registered: true, Runs: []string{"completed", "pending"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "read", Client: "c1", Registered: true})
	purgetest.Seed(t, pool, purgetest.Session{ID: "reader", Client: "c1", Registered: true, Runs: []string{"submitted"}})
	purgetest.Exec(t, pool, `UPDATE jetsapi.pipeline_execution_status SET input_session_id = 'read' WHERE session_id = 'reader'`)
	purgetest.Seed(t, pool, purgetest.Session{ID: "free", Client: "c1", Registered: true, Runs: []string{"errors", "interrupted"}})
	before := map[string]map[string]int64{}
	for _, id := range []string{"on", "read"} {
		before[id] = purgetest.SessionRows(t, pool, id)
	}
	queue(t, pool, retention("on"), retention("read"), retention("free"))
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Refused != 2 || report.Done != 1 || !slices.Equal(report.RefusedSessions, []string{"on", "read"}) {
		t.Errorf("report %+v, want on and read refused and free done", report)
	}
	for id, rows := range before {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, rows) {
			t.Errorf("%s changed: %v, was %v", id, got, rows)
		}
		if d := deletion(t, pool, id); d == nil || d.Status != StatusPending {
			t.Errorf("%s deletion row %+v, want pending", id, d)
		}
	}
}

// A registered table that does not exist, or has no session_id column, or is a
// view, is skipped and the session is deleted.
func TestUnresolvedDomainTableSkipped(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	purgetest.Exec(t, pool, `CREATE TABLE public."NoSessionColumn" (v text); CREATE VIEW public."AView" AS SELECT 's1'::text AS session_id`)
	for _, name := range []string{"NoSuchTable", "NoSessionColumn", "AView", `odd"name`} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
			(client, object_type, source_period_key, table_name, source_type, session_id, user_email)
			VALUES ('c1', 'ObjectA', 1, $1, 'report_table', 's1', 'system')`, name)
	}
	queue(t, pool, retention("s1"))
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Failed != 0 {
		t.Errorf("report %+v, want done", report)
	}
	if left := purgetest.SessionRows(t, pool, "s1"); len(left) > 0 {
		t.Errorf("rows of s1 left: %v", left)
	}
}

// A delete that fails rolls the whole session back, its session_registry row
// included, and marks it failed with the error; the next Execute retries it.
func TestFailingDeleteRollsBack(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA", "DomainZ"}})
	purgetest.Exec(t, pool, `CREATE FUNCTION public.refuse() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'delete refused by test trigger'; END $$;
		CREATE TRIGGER refuse BEFORE DELETE ON public."DomainZ" FOR EACH ROW EXECUTE FUNCTION public.refuse()`)
	before := purgetest.SessionRows(t, pool, "s1")
	queue(t, pool, retention("s1"))
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed != 1 || report.Done != 0 || !slices.Equal(report.FailedSessions, []string{"s1"}) {
		t.Errorf("report %+v, want s1 failed", report)
	}
	if got := purgetest.SessionRows(t, pool, "s1"); !reflect.DeepEqual(got, before) {
		t.Errorf("s1 half deleted: %v, was %v", got, before)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive`); n != 0 {
		t.Errorf("%d rows archived by a rolled-back session", n)
	}
	d := deletion(t, pool, "s1")
	if d == nil || d.Status != StatusFailed || d.Error == nil || !strings.Contains(*d.Error, "delete refused by test trigger") ||
		!strings.Contains(*d.Error, "DomainZ") {
		t.Errorf("deletion row %+v, want failed with the trigger's error and the table", d)
	}

	purgetest.Exec(t, pool, `DROP TRIGGER refuse ON public."DomainZ"`)
	report, err = Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 {
		t.Errorf("retry report %+v, want s1 done", report)
	}
	if d := deletion(t, pool, "s1"); d == nil || d.Status != StatusDone || d.Error != nil {
		t.Errorf("deletion row after the retry %+v, want done with the error cleared", d)
	}
}

// A run whose deadline has passed starts no session.
func TestDeadlinePassed(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s2", Client: "c1", Registered: true})
	before := purgetest.RowCounts(t, pool)
	queue(t, pool, retention("s1"), retention("s2"))
	report, err := Execute(ctx, pool, Options{Deadline: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if report.NotReached != 2 || report.Done != 0 {
		t.Errorf("report %+v, want two not reached", report)
	}
	after := purgetest.RowCounts(t, pool)
	after["jetsapi.session_deletion"] -= 2
	if !reflect.DeepEqual(after, before) {
		t.Errorf("row counts changed beyond the two queued rows")
	}
	for _, id := range []string{"s1", "s2"} {
		if d := deletion(t, pool, id); d == nil || d.Status != StatusPending {
			t.Errorf("%s: %+v, want pending", id, d)
		}
	}
}

// A dry run changes no row count and reports what a real run does.
func TestDryRunWritesNothing(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s2", Client: "c1", Registered: true, Runs: []string{"pending"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "o1", Client: "c1", DomainTables: []string{"DomainA"}})
	// One already queued by a user: a dry run takes it as Execute would.
	queue(t, pool, Request{SessionID: "s3", Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s3", Client: "c1", Registered: true})

	reqs := []Request{retention("s1"), retention("s2"), retention("s3"),
		{SessionID: "o1", Client: "c1", Reason: ReasonOrphan, RequestedBy: RequestedByPurge}}
	if _, err := ExecuteRequests(ctx, pool, reqs, Options{}); err == nil {
		t.Error("ExecuteRequests ran without DryRun")
	}
	before := purgetest.RowCounts(t, pool)
	dry, err := ExecuteRequests(ctx, pool, reqs, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if after := purgetest.RowCounts(t, pool); !reflect.DeepEqual(after, before) {
		for k := range after {
			if after[k] != before[k] {
				t.Errorf("dry run changed %s: %d rows, was %d", k, after[k], before[k])
			}
		}
	}
	if dry.Done != 3 || dry.Refused != 1 || dry.ByReason[ReasonUser] != 1 || dry.ByReason[ReasonRetention] != 1 ||
		dry.ByReason[ReasonOrphan] != 1 {
		t.Errorf("dry run report %+v", dry)
	}

	queue(t, pool, reqs...)
	real, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	dry.RefusedSessions, real.RefusedSessions = nil, nil
	if !reflect.DeepEqual(dry, real) {
		t.Errorf("the dry run predicted\n%+v\nthe real run did\n%+v", dry, real)
	}
	// Execute with DryRun is the same dry run.
	if r, err := Execute(ctx, pool, Options{DryRun: true}); err != nil || r.Refused != 1 || r.Done != 0 {
		t.Errorf("Execute dry run on what is left: %+v, %v", r, err)
	}
}

// Queue's rule: the first request for a session wins, except that a user request
// replaces a retention or orphan one not yet done.
func TestQueueRule(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	user := func(id string) Request {
		return Request{SessionID: id, Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"}
	}
	orphan := func(id string) Request {
		return Request{SessionID: id, Client: "c1", Reason: ReasonOrphan, RequestedBy: RequestedByPurge}
	}
	if n := queue(t, pool, retention("a"), orphan("b"), user("c"), retention("d"), orphan("f")); n != 5 {
		t.Errorf("queued %d, want 5", n)
	}
	purgetest.Exec(t, pool, `UPDATE jetsapi.session_deletion SET status = 'done', completed_at = now() WHERE session_id = 'd'`)
	purgetest.Exec(t, pool, `UPDATE jetsapi.session_deletion SET status = 'failed', error = 'x' WHERE session_id = 'f'`)
	// a: retention replaced by user; b: orphan kept against retention; c: user
	// kept against orphan; d: done kept against user; f: failed orphan replaced.
	n := queue(t, pool, user("a"), retention("b"), orphan("c"), user("d"), user("f"),
		// within one call: the user request wins whichever comes first
		retention("g"), user("g"), user("h"), orphan("h"))
	if n != 4 {
		t.Errorf("second queue changed %d rows, want 4 (a, f, g, h)", n)
	}
	for id, want := range map[string]deletionRow{
		"a": {Reason: "user", RequestedBy: "someone@example.com", Status: StatusPending},
		"b": {Reason: "orphan", RequestedBy: RequestedByPurge, Status: StatusPending},
		"c": {Reason: "user", RequestedBy: "someone@example.com", Status: StatusPending},
		"d": {Reason: "retention", RequestedBy: RequestedByPurge, Status: StatusDone, Completed: true},
		"f": {Reason: "user", RequestedBy: "someone@example.com", Status: StatusPending},
		"g": {Reason: "user", RequestedBy: "someone@example.com", Status: StatusPending},
		"h": {Reason: "user", RequestedBy: "someone@example.com", Status: StatusPending},
	} {
		got := deletion(t, pool, id)
		if got == nil || got.Reason != want.Reason || got.RequestedBy != want.RequestedBy || got.Status != want.Status ||
			got.Completed != want.Completed || got.Error != nil {
			t.Errorf("%s: %+v, want %+v", id, got, want)
		}
	}
	if _, err := Queue(ctx, pool, []Request{{SessionID: "x", Client: "c1", Reason: "other", RequestedBy: "purge"}}); err == nil {
		t.Error("an unknown reason was queued")
	}
}

// The candidates: retention by the registry's last_update; orphans by their
// latest last_update across input_registry and the status table, unregistered
// and not already queued.
func TestCandidates(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	old, recent := days(100), days(1)
	purgetest.Seed(t, pool, purgetest.Session{ID: "reg-old", Client: "c1", Registered: true, LastUpdate: old})
	purgetest.Seed(t, pool, purgetest.Session{ID: "reg-new", Client: "c1", Registered: true, LastUpdate: recent})
	purgetest.Seed(t, pool, purgetest.Session{ID: "orph-input", Client: "c2", LastUpdate: old, DomainTables: []string{"DomainA"}, Runs: []string{"failed"}})
	purgetest.Exec(t, pool, `DELETE FROM jetsapi.pipeline_execution_status WHERE session_id = 'orph-input'`)
	purgetest.Seed(t, pool, purgetest.Session{ID: "orph-status", Client: "c3", LastUpdate: old})
	purgetest.Seed(t, pool, purgetest.Session{ID: "orph-recent", Client: "c1", LastUpdate: old, DomainTables: []string{"DomainA"}})
	purgetest.Exec(t, pool, `UPDATE jetsapi.pipeline_execution_status SET last_update = now() WHERE session_id = 'orph-recent'`)
	purgetest.Seed(t, pool, purgetest.Session{ID: "orph-queued", Client: "c1", LastUpdate: old})
	queue(t, pool, Request{SessionID: "orph-queued", Client: "c1", Reason: ReasonOrphan, RequestedBy: RequestedByPurge})

	before := days(30)
	ret, err := RetentionCandidates(ctx, pool, before)
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 1 || ret[0].SessionID != "reg-old" || ret[0].Reason != ReasonRetention || ret[0].Client != "c1" ||
		ret[0].SourceType != "file" || ret[0].MonthPeriod == nil || *ret[0].MonthPeriod != 313 || ret[0].RequestedBy != RequestedByPurge {
		t.Errorf("retention candidates %+v", ret)
	}
	orph, err := OrphanCandidates(ctx, pool, before)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range orph {
		ids = append(ids, r.SessionID)
		if r.Reason != ReasonOrphan || r.MonthPeriod == nil || *r.MonthPeriod != 313 {
			t.Errorf("orphan candidate %+v", r)
		}
	}
	if !slices.Equal(ids, []string{"orph-input", "orph-status"}) {
		t.Errorf("orphan candidates %v, want orph-input and orph-status", ids)
	}
	if len(orph) == 2 && (orph[0].Client != "c2" || orph[0].SourceType != "domain_table" || orph[1].Client != "c3" || orph[1].SourceType != "") {
		t.Errorf("orphan candidates' client and source type: %+v", orph)
	}
}

// The archive is purged by each run's own last_update; done tombstones go by
// age, user ones kept longer; pending and failed rows are never purged.
func TestPurgeArchiveAndTombstones(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	for i, lu := range []time.Time{days(200), days(181), days(179), days(1)} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_status_archive
			(key, pipeline_config_key, client, process_name, main_object_type, session_id, source_period_key,
			 status, user_email, last_update, archived_at, archive_reason)
			VALUES ($1, 1, 'c1', 'p', 'o', 's', 1, 'completed', 'u', $2::timestamptz, now(), 'retention')`, i+1, lu)
	}
	n, err := PurgeArchive(ctx, pool, days(180))
	if err != nil {
		t.Fatal(err)
	}
	if left := count(t, pool, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive WHERE key IN (3, 4)`); n != 2 || left != 2 {
		t.Errorf("purged %d archived runs and kept %d of the two recent ones, want 2 and 2", n, left)
	}

	for _, r := range []struct {
		id, reason, status string
		completed          *time.Time
	}{
		{"ret-old", "retention", "done", ptr(days(200))},
		{"orph-old", "orphan", "done", ptr(days(181))},
		{"ret-new", "retention", "done", ptr(days(10))},
		{"user-200", "user", "done", ptr(days(200))},
		{"user-450", "user", "done", ptr(days(450))},
		{"pending-old", "retention", "pending", nil},
		{"failed-old", "orphan", "failed", nil},
	} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion
			(session_id, client, reason, requested_by, requested_at, status, completed_at)
			VALUES ($1, 'c1', $2, 'purge', now() - interval '500 days', $3, $4::timestamptz)`, r.id, r.reason, r.status, r.completed)
	}
	n, err = PurgeTombstones(ctx, pool, days(180), days(400))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("purged %d tombstones, want 3", n)
	}
	for _, id := range []string{"ret-old", "orph-old", "user-450"} {
		if deletion(t, pool, id) != nil {
			t.Errorf("%s kept", id)
		}
	}
	for _, id := range []string{"ret-new", "user-200", "pending-old", "failed-old"} {
		if deletion(t, pool, id) == nil {
			t.Errorf("%s purged", id)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// merge: the first request wins, a user request replaces another and moves to the
// end, as Queue's new requested_at moves it.
func TestMerge(t *testing.T) {
	r := func(id string, reason Reason) Request { return Request{SessionID: id, Reason: reason} }
	got := merge([]Request{r("a", ReasonRetention), r("b", ReasonUser)},
		[]Request{r("c", ReasonOrphan), r("a", ReasonUser), r("b", ReasonRetention), r("c", ReasonRetention)})
	want := []Request{r("b", ReasonUser), r("c", ReasonOrphan), r("a", ReasonUser)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("merge gave %+v, want %+v", got, want)
	}
}
