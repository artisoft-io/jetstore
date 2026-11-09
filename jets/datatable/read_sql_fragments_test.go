package datatable

// A data-table read is built from fragments the client sends. These tests pin
// what the server does with them: the where clause's values and like pattern
// are quoted literals, joinWith is an identifier, and the read runs READ ONLY,
// so a WITH clause that writes is refused; so do the app's raw queries.
//
// The first test needs nothing. The second is skipped unless JETS_TEST_DSN names
// a throwaway PostgreSQL, as for rules_unit_test_runs_test.go:
//
//	docker run -d --rm --name jm092-pg -e POSTGRES_PASSWORD=pw -p 5472:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5472/postgres go test ./datatable -run ReadSql -count=1

import (
	"context"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

func TestReadSqlWhereClauseQuotesWhatTheClientSends(t *testing.T) {
	cases := []struct {
		name string
		wc   WhereClause
		want string
	}{
		{"one value", WhereClause{Table: "t", Column: "c", Values: []string{"O'Brien"}},
			` WHERE "t"."c" = 'O''Brien'`},
		{"an injected value stays a value", WhereClause{Column: "c", Values: []string{"x' OR '1'='1"}},
			` WHERE "c" = 'x'' OR ''1''=''1'`},
		{"several values", WhereClause{Column: "c", Values: []string{"a'", "NULL", "b"}},
			` WHERE "c" IN ('a''',  NULL , 'b') `},
		{"a pg array in one value", WhereClause{Column: "c", Values: []string{"{a',b}"}},
			` WHERE "c" IN ('a''', 'b') `},
		{"not in", WhereClause{Column: "c", NotInValues: []string{"a'"}},
			` WHERE "c" != 'a'''`},
		{"null", WhereClause{Column: "c", Values: []string{"NULL"}},
			` WHERE "c" is NULL `},
		{"like", WhereClause{Column: "c", Like: "O'B%"},
			` WHERE "c" LIKE 'O''B%' `},
		{"joinWith", WhereClause{Table: "a", Column: "k", JoinWith: "source_period.key"},
			` WHERE "a"."k" = "source_period"."key"`},
		{"joinWith cannot carry SQL", WhereClause{Column: "k", JoinWith: "b.key OR true"},
			` WHERE "k" = "b"."key OR true"`},
		{"ge is an expression, as the client writes it", WhereClause{Column: "d", Ge: "now()-interval '1 day'"},
			` WHERE "d" >= now()-interval '1 day'`},
	}
	for _, c := range cases {
		got := (&DataTableAction{WhereClauses: []WhereClause{c.wc}}).makeWhereClause()
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestReadSqlRunsReadOnly(t *testing.T) {
	pool := rutDB(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"users", "roles", "role_capability"} {
		if _, err := pool.Exec(bg, "DROP TABLE IF EXISTS jetsapi."+name); err != nil {
			t.Fatal(err)
		}
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	setup := []string{
		`DROP TABLE IF EXISTS jetsapi.jm092_probe`,
		`CREATE TABLE jetsapi.jm092_probe (key integer PRIMARY KEY, name text NOT NULL)`,
		`INSERT INTO jetsapi.jm092_probe VALUES (1, 'O''Brien'), (2, 'Smith')`,
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ('reader@test', 'Reader', 'x', 1, '{reader}')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('reader', 'jetstore_read')`,
	}
	for _, s := range setup {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("setting up: %v\n%s", err, s)
		}
	}
	t.Cleanup(func() { _, _ = pool.Exec(bg, `DROP TABLE IF EXISTS jetsapi.jm092_probe`) })

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "jm092-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	token, err := user.CreateToken("reader@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &DataTableContext{Dbpool: pool}
	read := func(where []WhereClause, with []WithClause) (*map[string]any, error) {
		res, _, err := ctx.DoReadAction(&DataTableAction{
			Action:       "read",
			FromClauses:  []FromClause{{Schema: "jetsapi", Table: "jm092_probe"}},
			Columns:      []Column{{Table: "jm092_probe", Column: "key"}, {Table: "jm092_probe", Column: "name"}},
			WhereClauses: where,
			WithClauses:  with,
			SortColumn:   "key",
			Limit:        10,
		}, token)
		return res, err
	}
	count := func() int {
		var n int
		if err := pool.QueryRow(bg, `SELECT count(*) FROM jetsapi.jm092_probe`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	// A quote in a filter value is a value: the read finds the row and does not fail.
	res, err := read([]WhereClause{{Table: "jm092_probe", Column: "name", Values: []string{"O'Brien"}}}, nil)
	if err != nil {
		t.Fatalf("a value with a quote: %v", err)
	}
	if n := (*res)["totalRowCount"]; n != 1 {
		t.Errorf("a value with a quote: totalRowCount %v, want 1", n)
	}
	res, err = read([]WhereClause{{Table: "jm092_probe", Column: "name", Values: []string{"x' OR '1'='1"}}}, nil)
	if err != nil {
		t.Fatalf("an injected value: %v", err)
	}
	if n := (*res)["totalRowCount"]; n != 0 {
		t.Errorf("an injected value: totalRowCount %v, want 0", n)
	}

	// A WITH clause that writes is refused, and nothing is written.
	_, err = read(nil, []WithClause{{Name: "gone", Stmt: "DELETE FROM jetsapi.jm092_probe RETURNING key"}})
	if err == nil || !strings.Contains(err.Error(), "read-only transaction") {
		t.Errorf("a writing WITH clause: err %v, want a read-only refusal", err)
	}
	if n := count(); n != 2 {
		t.Errorf("after a writing WITH clause: %d rows, want 2", n)
	}

	// The app's raw queries, on the read capability, cannot write either.
	if _, _, err = ctx.ExecRawQuery(&DataTableAction{Action: "raw_query",
		RawQuery: "DELETE FROM jetsapi.jm092_probe RETURNING key"}, token, CapabilityReadData); err == nil ||
		!strings.Contains(err.Error(), "read-only transaction") {
		t.Errorf("a writing raw_query: err %v, want a read-only refusal", err)
	}
	if _, _, err = ctx.ExecRawQueryMap(&DataTableAction{Action: "raw_query_map",
		RawQueryMap: map[string]string{"q": "UPDATE jetsapi.jm092_probe SET name = 'x' RETURNING key"}}, token); err == nil ||
		!strings.Contains(err.Error(), "read-only transaction") {
		t.Errorf("a writing raw_query_map: err %v, want a read-only refusal", err)
	}
	if n := count(); n != 2 {
		t.Errorf("after the writing raw queries: %d rows, want 2", n)
	}
	var smith int
	if err := pool.QueryRow(bg, `SELECT count(*) FROM jetsapi.jm092_probe WHERE name = 'Smith'`).Scan(&smith); err != nil || smith != 1 {
		t.Errorf("after the writing raw queries: Smith %d (%v), want 1", smith, err)
	}
	res, _, err = ctx.ExecRawQuery(&DataTableAction{Action: "raw_query",
		RawQuery: "SELECT name FROM jetsapi.jm092_probe ORDER BY key"}, token, CapabilityReadData)
	if err != nil || len(*((*res)["rows"].(*[][]any))) != 2 {
		t.Errorf("a reading raw_query: %v, %v", res, err)
	}

	// A WITH clause that reads still works.
	res, err = read(nil, []WithClause{{Name: "names", Stmt: "SELECT name FROM jetsapi.jm092_probe"}})
	if err != nil {
		t.Fatalf("a reading WITH clause: %v", err)
	}
	if n := (*res)["totalRowCount"]; n != 2 {
		t.Errorf("a reading WITH clause: totalRowCount %v, want 2", n)
	}
}
