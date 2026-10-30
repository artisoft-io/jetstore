package datatable

// A numeric column sorts as a number when its column names no table. Project
// jetstore_maintenance_02, Phase 3, task AJ.3 (I-49; rules_unit_test_harness I-33).
//
// makeSelectColumns casts every plain column to text and keeps its name
// ("shard_id"::text), and PostgreSQL resolves a bare ORDER BY name to an output
// column before a table column. So ORDER BY "shard_id" sorted the text, 10
// before 8, on every single-table query whose sort column names no table: the
// two shipped tables below by default, and a header click on any numeric column
// of a table that does not name it. buildQuery now qualifies a plain selected
// sort column with the only FROM entry (sortColumnIdentifier).
//
// The string tests always run. The PostgreSQL test is skipped unless
// JETS_TEST_DSN names one, and shares track AC's database; see the header of
// rules_unit_test_runs_test.go.

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// The two tables I-49 names, as the client bundles them.
const (
	pipelineExecDetailsDoc = "../../jetsclient_ide/src/datatable/tables/pipelineExecDetailsTable.tc.json"
	cpipesExecDetailsDoc   = "../../jetsclient_ide/src/datatable/tables/cpipesExecDetailsTable.tc.json"
)

// orderByOf returns what follows ORDER BY in the query buildQuery emits.
func orderByOf(t *testing.T, action *DataTableAction) string {
	t.Helper()
	query, _ := action.buildQuery()
	_, after, ok := strings.Cut(query, " ORDER BY ")
	if !ok {
		t.Fatalf("no ORDER BY in %s", query)
	}
	clause, _, _ := strings.Cut(after, " LIMIT ")
	return strings.TrimSpace(clause)
}

func TestSortColumnIdentifier(t *testing.T) {
	formState := map[string]string{"session_id": "s1"}
	cases := []struct {
		name   string
		action func(t *testing.T) *DataTableAction
		want   string
	}{
		{"the configured sort of pipelineExecDetailsTable", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
			return a
		}, `"pipeline_execution_details"."shard_id"`},
		{"the configured sort of cpipesExecDetailsTable", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, cpipesExecDetailsDoc, formState)
			return a
		}, `"cpipes_execution_status_details"."total_input_files_size_mb" DESC`},
		{"a header click on a column that names no table", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
			a.SortColumn, a.SortColumnTable = "input_records_count", ""
			return a
		}, `"pipeline_execution_details"."input_records_count"`},
		{"a calculated column keeps its output name", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
			a.SortColumn, a.SortColumnTable = "run_duration", ""
			return a
		}, `"run_duration"`},
		{"the table the request names wins", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
			a.SortColumnTable = "other"
			return a
		}, `"other"."shard_id"`},
		{"a column's own table", func(t *testing.T) *DataTableAction {
			a, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
			a.SortColumn, a.SortColumnTable = "key", ""
			return a
		}, `"pipeline_execution_details"."key"`},
		{"an aliased FROM entry", func(t *testing.T) *DataTableAction {
			return &DataTableAction{
				FromClauses: []FromClause{{Schema: "jetsapi", Table: "users", AsTable: "u"}},
				Columns:     []Column{{Column: "name"}},
				SortColumn:  "name", SortAscending: true,
			}
		}, `"u"."name"`},
		{"two FROM entries and no table stay bare", func(t *testing.T) *DataTableAction {
			return &DataTableAction{
				FromClauses: []FromClause{{Schema: "jetsapi", Table: "a"}, {Schema: "jetsapi", Table: "b"}},
				Columns:     []Column{{Column: "n"}},
				SortColumn:  "n", SortAscending: true,
			}
		}, `"n"`},
		{"a sort column that is not selected stays bare", func(t *testing.T) *DataTableAction {
			return &DataTableAction{
				FromClauses: []FromClause{{Schema: "jetsapi", Table: "a"}},
				Columns:     []Column{{Column: "m"}},
				SortColumn:  "n", SortAscending: true,
			}
		}, `"n"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := orderByOf(t, c.action(t)); got != c.want {
				t.Errorf("ORDER BY %s, want ORDER BY %s", got, c.want)
			}
		})
	}
}

func TestNumericColumnsSortAsNumbers(t *testing.T) {
	pool := rutDB(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"pipeline_execution_details", "cpipes_execution_status_details"} {
		if _, err := pool.Exec(bg, "DROP TABLE IF EXISTS jetsapi."+name); err != nil {
			t.Fatal(err)
		}
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	// shard_id and total_input_files_size_mb run 8 to 12, which sort 10, 11, 12,
	// 8, 9 as text. input_records_count is 100, 20, 3, 1000, 45 for shards 8 to
	// 12, which sort 8, 11, 9, 10, 12 as text.
	for _, s := range []string{
		`INSERT INTO jetsapi.pipeline_execution_details
			(key, pipeline_config_key, pipeline_execution_status_key, client, process_name,
			 main_input_session_id, session_id, source_period_key, status, user_email,
			 shard_id, input_records_count, start_time, last_update)
			SELECT k, 1, 1, 'c', 'p', 's0', 's1', 1, 'completed', 'u@test', k,
				(ARRAY[100, 20, 3, 1000, 45])[k - 7], now(), now() + k * interval '1 minute'
			FROM generate_series(8, 12) AS k`,
		`INSERT INTO jetsapi.cpipes_execution_status_details
			(key, session_id, process_name, cpipes_step_id, nbr_nodes, total_input_files_size_mb,
			 total_input_records_count, total_output_records_count)
			SELECT k, 's1', 'p', 'reducing0', 1, k, 0, 0 FROM generate_series(8, 12) AS k`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
	// column returns one column of the rows the request reads, in order.
	column := func(t *testing.T, action *DataTableAction, name string) string {
		t.Helper()
		pos := -1
		for i, c := range action.Columns {
			if c.Column == name {
				pos = i
			}
		}
		if pos < 0 {
			t.Fatalf("the request selects no %s", name)
		}
		query, _ := action.buildQuery()
		t.Log(query)
		rows, _, err := execQuery(pool, action, &query)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		values := []string{}
		for _, row := range *rows {
			values = append(values, fmt.Sprint(row[pos]))
		}
		return strings.Join(values, " ")
	}
	formState := map[string]string{"session_id": "s1"}

	action, _ := readRequestFromFile(t, pipelineExecDetailsDoc, formState)
	if action.SortColumn != "shard_id" || action.SortColumnTable != "" {
		t.Fatalf("pipelineExecDetailsTable sorts on %q of %q, want shard_id of no table", action.SortColumn, action.SortColumnTable)
	}
	if got := column(t, action, "shard_id"); got != "8 9 10 11 12" {
		t.Errorf("pipelineExecDetailsTable by shard_id: got %s, want 8 9 10 11 12", got)
	}

	action, _ = readRequestFromFile(t, cpipesExecDetailsDoc, formState)
	if got := column(t, action, "total_input_files_size_mb"); got != "12 11 10 9 8" {
		t.Errorf("cpipesExecDetailsTable by total_input_files_size_mb, descending: got %s, want 12 11 10 9 8", got)
	}
	action.SortAscending = true
	if got := column(t, action, "total_input_files_size_mb"); got != "8 9 10 11 12" {
		t.Errorf("cpipesExecDetailsTable by total_input_files_size_mb, ascending: got %s, want 8 9 10 11 12", got)
	}

	// A header click, as resolveSortColumn sends it: the column's own table,
	// which input_records_count does not name.
	action, _ = readRequestFromFile(t, pipelineExecDetailsDoc, formState)
	action.SortColumn, action.SortColumnTable = "input_records_count", ""
	if got := column(t, action, "shard_id"); got != "10 9 12 8 11" {
		t.Errorf("by input_records_count: got shards %s, want 10 9 12 8 11", got)
	}

	// A calculated column is still sorted by its output name.
	action, _ = readRequestFromFile(t, pipelineExecDetailsDoc, formState)
	action.SortColumn, action.SortColumnTable, action.SortAscending = "run_duration", "", false
	if got := column(t, action, "shard_id"); got != "12 11 10 9 8" {
		t.Errorf("by run_duration, descending: got shards %s, want 12 11 10 9 8", got)
	}

	// The mechanism, so the tests above are not passing for another reason: the
	// bare name against the text cast still sorts as text in PostgreSQL.
	var bare []string
	rows, err := pool.Query(bg, `SELECT "shard_id"::text FROM "jetsapi"."pipeline_execution_details" ORDER BY "shard_id"`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		bare = append(bare, s)
	}
	rows.Close()
	if got := strings.Join(bare, " "); got != "10 11 12 8 9" {
		t.Errorf("the bare ORDER BY: got %s, want the text order 10 11 12 8 9", got)
	}
}
