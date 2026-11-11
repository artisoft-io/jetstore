package datatable

// Paging is deterministic when rows tie on the sort column. buildQuery pages
// with LIMIT/OFFSET, and ordered by the requested sort column alone, so rows
// that tie on it came back in plan order: a page could repeat a row of the
// previous page and the next could skip one. buildQuery now orders by every
// plain selected column after the requested sort (sortTiebreaker).
//
// The string test and the SQLite test always run. The PostgreSQL test is
// skipped unless JETS_TEST_DSN names one; see the header of
// rules_unit_test_runs_test.go.

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestSortTiebreakerFollowsTheRequestedSort(t *testing.T) {
	cases := []struct {
		name   string
		action *DataTableAction
		want   string
	}{
		{"every plain column, after the sort", &DataTableAction{
			FromClauses: []FromClause{{Schema: "s", Table: "t"}},
			Columns:     []Column{{Column: "a"}, {Column: "b"}, {Column: "c"}},
			SortColumn:  "b", SortAscending: true,
		}, `"t"."b", 1, 2, 3`},
		{"descending keeps the sort first", &DataTableAction{
			FromClauses: []FromClause{{Schema: "s", Table: "t"}},
			Columns:     []Column{{Column: "a"}, {Column: "b"}},
			SortColumn:  "a",
		}, `"t"."a" DESC , 1, 2`},
		{"a calculated column is not a tiebreaker", &DataTableAction{
			FromClauses: []FromClause{{Schema: "s", Table: "t"}},
			Columns:     []Column{{Column: "a"}, {Column: "j", CalculatedAs: "to_json(a)"}, {Column: "c"}},
			SortColumn:  "a", SortAscending: true,
		}, `"t"."a", 1, 3`},
		{"a DISTINCT ON prefix is kept", &DataTableAction{
			FromClauses:       []FromClause{{Schema: "s", Table: "t"}},
			Columns:           []Column{{Column: "a"}, {Column: "b"}},
			DistinctOnClauses: []string{"a"},
			SortColumn:        "a", SortAscending: true,
		}, `"t"."a", 1, 2`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, _ := c.action.buildQuery()
			_, after, _ := strings.Cut(query, " ORDER BY ")
			got, _, _ := strings.Cut(after, " LIMIT ")
			if got = strings.TrimSpace(got); got != c.want {
				t.Errorf("ORDER BY %s, want ORDER BY %s", got, c.want)
			}
		})
	}

	// No sort requested, or no columns named: no ORDER BY is added.
	for _, a := range []*DataTableAction{
		{FromClauses: []FromClause{{Schema: "s", Table: "t"}}, Columns: []Column{{Column: "a"}}},
		{FromClauses: []FromClause{{Schema: "s", Table: "t"}}, SortColumn: "a"},
	} {
		query, _ := a.buildQuery()
		if a.SortColumn == "" && strings.Contains(query, "ORDER BY") {
			t.Errorf("no sort requested, yet %s", query)
		}
		if len(a.Columns) == 0 && strings.Contains(query, ", 1") {
			t.Errorf("SELECT * has no positions, yet %s", query)
		}
	}
}

// pageThrough pages through a request whose sort column ties on every row and
// returns the "id" column of each page, joined.
func pageThrough(t *testing.T, action *DataTableAction, pageSize, total int,
	read func(query string) [][]any) []string {
	t.Helper()
	ids := []string{}
	for offset := 0; offset < total; offset += pageSize {
		action.Offset, action.Limit = offset, pageSize
		query, _ := action.buildQuery()
		for _, row := range read(query) {
			ids = append(ids, fmt.Sprint(row[1]))
		}
	}
	return ids
}

// assertEachOnce fails unless the pages, read in order, hold each of 1..n
// exactly once: no row repeated on a later page and none skipped.
func assertEachOnce(t *testing.T, ids []string, n int) {
	t.Helper()
	got := append([]string(nil), ids...)
	want := []string{}
	for i := 1; i <= n; i++ {
		want = append(want, fmt.Sprint(i))
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("paged ids %v, want each of 1..%d once", ids, n)
	}
}

func TestTiedRowsPageOnceEachOnSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`CREATE TABLE tied (empty TEXT, id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	// Inserted in reverse, so the plan order is not the tiebreaker order.
	for i := 23; i >= 1; i-- {
		if _, err = db.Exec(`INSERT INTO tied VALUES ('', ?)`, i); err != nil {
			t.Fatal(err)
		}
	}
	action := &DataTableAction{
		FromClauses: []FromClause{{Table: "tied"}},
		Columns:     []Column{{Column: "empty"}, {Column: "id"}},
		SortColumn:  "empty", SortAscending: true,
	}
	ids := pageThrough(t, action, 5, 23, func(query string) [][]any {
		rows, err := execWorkspaceQuery(db, action, &query)
		if err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
		return *rows
	})
	assertEachOnce(t, ids, 23)
}

func TestTiedRowsPageOnceEachOnPostgreSQL(t *testing.T) {
	pool := rutDB(t)
	bg := context.Background()
	for _, s := range []string{
		`DROP VIEW IF EXISTS public.tied_view`,
		`DROP TABLE IF EXISTS public.tied`,
		// No primary key, no unique column but id, and a sort column that is
		// empty for every row.
		`CREATE TABLE public.tied (empty text, id int, payload text)`,
		`INSERT INTO public.tied SELECT '', k, md5(k::text) FROM generate_series(23, 1, -1) AS k`,
		`CREATE VIEW public.tied_view AS SELECT empty, id, payload FROM public.tied`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	read := func(action *DataTableAction) func(string) [][]any {
		return func(query string) [][]any {
			rows, _, err := execQuery(pool, action, &query)
			if err != nil {
				t.Fatalf("%v\n%s", err, query)
			}
			return *rows
		}
	}
	for _, table := range []string{"tied", "tied_view"} {
		t.Run(table, func(t *testing.T) {
			action := &DataTableAction{
				FromClauses: []FromClause{{Schema: "public", Table: table}},
				Columns: []Column{{Column: "empty"}, {Column: "id"},
					// A calculated json column has no ordering operator; it is
					// selected but is not part of the tiebreaker.
					{Column: "doc", CalculatedAs: "json_build_object('id', id)"}},
				SortColumn: "empty", SortAscending: true,
			}
			assertEachOnce(t, pageThrough(t, action, 5, 23, read(action)), 23)
		})
	}

	// DISTINCT ON the sort column still parses, and keeps one row per value.
	action := &DataTableAction{
		FromClauses:       []FromClause{{Schema: "public", Table: "tied"}},
		Columns:           []Column{{Column: "empty"}, {Column: "id"}},
		DistinctOnClauses: []string{"empty"},
		SortColumn:        "empty", SortAscending: true, Limit: 10,
	}
	query, _ := action.buildQuery()
	rows, _, err := execQuery(pool, action, &query)
	if err != nil {
		t.Fatalf("DISTINCT ON: %v\n%s", err, query)
	}
	if len(*rows) != 1 || fmt.Sprint((*rows)[0][1]) != "1" {
		t.Errorf("DISTINCT ON empty: got %v, want the one row with the lowest id in text order, 1", *rows)
	}
}
