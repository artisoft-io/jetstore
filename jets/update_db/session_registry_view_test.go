package main

// jetsapi.session_registry_view, which jets_init_db.sql creates and this
// program runs: the Home screen's Session Registry tab reads it.
//
// TestSessionRegistryViewColumns needs no database. TestSessionRegistryView runs
// the statement on a scratch PostgreSQL named by JETS_TEST_DSN, skipped when it
// is unset, in a database of its own built by jets/sessionpurge/purgetest:
//
//	docker run -d --rm --name srg-g-pg -e POSTGRES_PASSWORD=pw -p 55434:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55434/postgres \
//	  go test -count=1 -run SessionRegistryView ./jets/update_db/
//
// Run with -count=1: the tests read jets_init_db.sql and jets_schema.json, which
// the test cache does not see.

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

var sessionRegistryViewRe = regexp.MustCompile(`(?s)CREATE OR REPLACE VIEW jetsapi\.session_registry_view AS.*?\n;`)

// sessionRegistryViewStatement is the statement creating the view, read from
// jets_init_db.sql.
func sessionRegistryViewStatement(t *testing.T) string {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(purgetest.JetsDir(), "jets_init_db.sql"))
	if err != nil {
		t.Fatal(err)
	}
	stmt := sessionRegistryViewRe.FindString(string(buf))
	if stmt == "" {
		t.Fatal("jets_init_db.sql has no CREATE OR REPLACE VIEW jetsapi.session_registry_view")
	}
	return stmt
}

// The columns the view adds to session_registry's, in order. The client's
// sessionRegistryTable.tc.json names them; a change here is a change there.
var sessionRegistryViewExtras = []string{
	"session_num", "object_type", "file_key", "process_name", "status",
	"deletion_status", "deletion_reason", "deletion_requested_by", "deletion_requested_at",
	"year", "month", "day", "file_key_date",
}

// selectNames returns the output names of a select list: the alias after AS,
// else the column after its table prefix. Commas inside parentheses or a
// string literal do not split.
func selectNames(list string) []string {
	var items []string
	depth, start, quoted := 0, 0, false
	for i, r := range list {
		switch {
		case r == '\'':
			quoted = !quoted
		case quoted:
		case r == '(':
			depth++
		case r == ')':
			depth--
		case r == ',':
			if depth == 0 {
				items = append(items, list[start:i])
				start = i + 1
			}
		}
	}
	items = append(items, list[start:])
	var names []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if i := strings.LastIndex(item, " AS "); i >= 0 {
			names = append(names, strings.TrimSpace(item[i+4:]))
			continue
		}
		if i := strings.LastIndex(item, "."); i >= 0 {
			item = item[i+1:]
		}
		names = append(names, item)
	}
	return names
}

// Both sides of the view select session_registry's columns, in jets_schema.json's
// order, then the extras, in order. No database needed.
func TestSessionRegistryViewColumns(t *testing.T) {
	var want []string
	for _, d := range purgetest.SchemaDefinitions(t) {
		if d.TableName == "session_registry" {
			for _, c := range d.Columns {
				if !c.Deleted {
					want = append(want, c.ColumnName)
				}
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("jets_schema.json has no table session_registry")
	}
	want = append(want, sessionRegistryViewExtras...)

	sides := strings.Split(sessionRegistryViewStatement(t), "UNION ALL")
	if len(sides) != 2 {
		t.Fatalf("the view is not two SELECTs joined by UNION ALL")
	}
	selectList := regexp.MustCompile(`(?s)SELECT\n(.*?)\nFROM`)
	for i, side := range sides {
		m := selectList.FindStringSubmatch(side)
		if m == nil {
			t.Fatalf("no select list in side %d of the view", i+1)
		}
		if got := selectNames(m[1]); !slices.Equal(got, want) {
			t.Errorf("side %d of the view selects\n%v\nwant\n%v", i+1, got, want)
		}
	}
}

type viewRow struct {
	sessionID, client                string
	sessionNum                       *int64
	objectType, fileKey, processName *string
	status, deletionStatus, reason   *string
	sourcePeriodKey                  *int
	year, month, day                 *int
}

// One row per live session, with its input and its own run; the tombstones of
// user deletions, and only those; session_num numeric or NULL; and the
// where-clause shapes the tab sends.
func TestSessionRegistryView(t *testing.T) {
	pool := purgetest.DB(t, "srg_session_registry_view")
	ctx := context.Background()
	purgetest.Exec(t, pool, sessionRegistryViewStatement(t))
	// The period every registered session below names (key 1), and a second day
	// of the same month, so a tombstone's month_period finds two rows that agree.
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.source_period (key, year, month, day, month_period, week_period, day_period)
		VALUES (1, 2026, 2, 3, 313, 1357, 9497), (2, 2026, 2, 4, 313, 1357, 9498)
		ON CONFLICT (key) DO UPDATE SET year = EXCLUDED.year, month = EXCLUDED.month, day = EXCLUDED.day,
			month_period = EXCLUDED.month_period, week_period = EXCLUDED.week_period, day_period = EXCLUDED.day_period`)
	// Created twice, as update_db -initDb runs the script on every deployment.
	purgetest.Exec(t, pool, sessionRegistryViewStatement(t))

	register := func(id, client, sourceType string) {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
			(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
			VALUES ($1, $2, $3, 1, 313, 1357, 9497)`, id, sourceType, client)
	}
	input := func(id, objectType, fileKey string) {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
			(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
			VALUES ('c1', $2, $3, 1, 't', 'file', $1, 'u')`, id, objectType, fileKey)
	}
	run := func(id, inputSession, status string) {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, client, process_name, main_object_type, input_session_id, session_id,
			 source_period_key, status, user_email)
			VALUES (1, 'c1', 'map_records', 'ObjectA', $2, $1, 1, $3, 'u')`, id, inputSession, status)
	}
	deletion := func(id, reason, status string) {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion
			(session_id, client, source_type, month_period, reason, requested_by, status)
			VALUES ($1, 'c1', 'file', 313, $2, 'a@b.c', $3)`, id, reason, status)
	}

	// A file session: two input rows, no run of its own.
	register("1001", "c1", "file")
	input("1001", "ObjectB", "c1/b.csv")
	input("1001", "ObjectA", "c1/a.csv")
	// A mapping session reading it, queued for deletion by a user.
	register("2002", "c1", "domain_table")
	input("2002", "ObjectA", "")
	run("2002", "1001", "completed")
	deletion("2002", "user", "pending")
	// Ids that are not 18 digits or fewer.
	register("abc-3", "(unknown)", "file")
	register("1234567890123456789", "c2", "file")
	// Gone: a user's tombstone is listed, a retention one and an orphan one are not.
	deletion("3003", "user", "done")
	deletion("4004", "retention", "done")
	deletion("5005", "orphan", "done")

	rows, err := pool.Query(ctx, `SELECT session_id, client, session_num, object_type, file_key,
		process_name, status, deletion_status, deletion_reason, source_period_key, year, month, day
		FROM jetsapi.session_registry_view ORDER BY session_id`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]viewRow{}
	var order []string
	for rows.Next() {
		var r viewRow
		if err := rows.Scan(&r.sessionID, &r.client, &r.sessionNum, &r.objectType, &r.fileKey,
			&r.processName, &r.status, &r.deletionStatus, &r.reason, &r.sourcePeriodKey,
			&r.year, &r.month, &r.day); err != nil {
			t.Fatal(err)
		}
		got[r.sessionID] = r
		order = append(order, r.sessionID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"1001", "1234567890123456789", "2002", "3003", "abc-3"}; !slices.Equal(order, want) {
		t.Fatalf("the view lists %v, want %v: one row per live session and the user tombstone", order, want)
	}

	str := func(p *string) string {
		if p == nil {
			return "<nil>"
		}
		return *p
	}
	num := func(p *int64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	check := func(id, field string, got, want any) {
		t.Helper()
		if got != want {
			t.Errorf("%s: %s is %v, want %v", id, field, got, want)
		}
	}
	f := got["1001"]
	check("1001", "session_num", num(f.sessionNum), int64(1001))
	check("1001", "object_type", str(f.objectType), "ObjectA")
	check("1001", "file_key", str(f.fileKey), "c1/a.csv")
	check("1001", "process_name", str(f.processName), "<nil>")
	check("1001", "deletion_status", str(f.deletionStatus), "<nil>")
	ival := func(p *int) any {
		if p == nil {
			return nil
		}
		return *p
	}
	check("1001", "year", ival(f.year), 2026)
	check("1001", "month", ival(f.month), 2)
	check("1001", "day", ival(f.day), 3)

	m := got["2002"]
	check("2002", "process_name", str(m.processName), "map_records")
	check("2002", "status", str(m.status), "completed")
	check("2002", "deletion_status", str(m.deletionStatus), "pending")
	check("2002", "deletion_reason", str(m.reason), "user")

	check("abc-3", "session_num", num(got["abc-3"].sessionNum), nil)
	check("abc-3", "client", got["abc-3"].client, "(unknown)")
	check("1234567890123456789", "session_num", num(got["1234567890123456789"].sessionNum), nil)

	d := got["3003"]
	check("3003", "deletion_status", str(d.deletionStatus), "done")
	check("3003", "session_num", num(d.sessionNum), int64(3003))
	check("3003", "year", ival(d.year), 2026)
	check("3003", "month", ival(d.month), 2)
	check("3003", "day", ival(d.day), nil)
	if d.sourcePeriodKey != nil {
		t.Errorf("3003: a tombstone has no source_period_key, got %d", *d.sourcePeriodKey)
	}

	// The where clauses the tab sends, as the server writes them
	// (jets/datatable/data_table_action.go, visitWhereClause): the range on
	// session_num with ge and le spliced, and the client filter.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.session_registry_view
		WHERE "session_registry_view"."session_num" >= 2000 AND "session_registry_view"."session_num" <= 3003
		AND "session_registry_view"."client" = 'c1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("session_num between 2000 and 3003 for client c1: %d rows, want 2 (2002 and the tombstone 3003)", n)
	}

	// file_key_date (JM-185): the period as a date, NULL for a tombstone.
	dates := map[string]string{}
	drows, err := pool.Query(ctx, `SELECT session_id, coalesce(file_key_date::text, '<nil>')
		FROM jetsapi.session_registry_view`)
	if err != nil {
		t.Fatal(err)
	}
	for drows.Next() {
		var id, d string
		if err := drows.Scan(&id, &d); err != nil {
			t.Fatal(err)
		}
		dates[id] = d
	}
	drows.Close()
	check("1001", "file_key_date", dates["1001"], "2026-02-03")
	check("3003", "file_key_date", dates["3003"], "<nil>")

	// A period whose day is 0, as an unparseable day is recorded (JM-170): the
	// view still reads, with no date, rather than raising on make_date.
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.source_period (key, year, month, day, month_period, week_period, day_period)
		VALUES (3, 2026, 2, 0, 313, 1357, 9496) ON CONFLICT (key) DO UPDATE SET day = 0`)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period)
		VALUES ('6006', 'file', 'c1', 3, 313, 1357, 9496)`)
	var day0 string
	if err := pool.QueryRow(ctx, `SELECT coalesce(file_key_date::text, '<nil>')
		FROM jetsapi.session_registry_view WHERE session_id = '6006'`).Scan(&day0); err != nil {
		t.Fatal(err)
	}
	check("6006", "file_key_date", day0, "<nil>")

	// The Set Filters dialog's new clauses, as the server writes them: process
	// names as IN, the file key fragment as LIKE, and the file key dates as ge
	// and le spliced, which compare a date with a timestamp.
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.session_registry_view
		WHERE "session_registry_view"."process_name" IN ('map_records', 'other')
		AND "session_registry_view"."file_key_date" >= timestamp '2026-02-01'
		AND "session_registry_view"."file_key_date" <= timestamp '2026-02-03'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("process map_records with a file key date in 2026-02-01..03: %d rows, want 1 (2002)", n)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.session_registry_view
		WHERE "session_registry_view"."file_key" LIKE '%a.csv%'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("file key like %%a.csv%%: %d rows, want 1 (1001)", n)
	}
}
