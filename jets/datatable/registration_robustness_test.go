package datatable

// Three defects of registration and start, each a request that was answered
// badly rather than refused:
//
//   - a schema event with no file_date, or one that does not parse, registered
//     under 1970-01-01 whatever date its file key carried (schemaEventDate);
//   - a submitted start read main_input_registry_key only after inserting the
//     run, so a missing key was a 500 with the row committed, and a JSON number
//     of 1,000,000 or more, decoded as a float64, was formatted 1e+06 and failed
//     the start's ParseInt (registryKeyOf);
//   - a pasted process mapping naming no table panicked the handler on a type
//     assertion (processMappingTableName).
//
// The unit tests need nothing; the database tests need JETS_TEST_DSN and are
// skipped otherwise (see register_file_key_action_test.go and
// session_delete_test.go, whose fixtures they use).

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/user"
)

func TestSchemaEventDate(t *testing.T) {
	dated := "client=c/object_type=t/year=2025/month=3/day=7/in.csv"
	cases := []struct {
		name, fileDate, fileKey string
		want                    [3]int
	}{
		{"file_date wins", "2024-11-30", dated, [3]int{2024, 11, 30}},
		{"no file_date: the file key's", "", dated, [3]int{2025, 3, 7}},
		{"unparseable file_date: the file key's", "not a date", dated, [3]int{2025, 3, 7}},
		{"year only: month and day 1", "", "client=c/year=2023/in.csv", [3]int{2023, 1, 1}},
		{"no date anywhere: 1970", "", "client=c/in.csv", [3]int{1970, 1, 1}},
		{"unparseable both: 1970", "bad", "client=c/year=20x5/in.csv", [3]int{1970, 1, 1}},
		{"impossible key date: 1970", "", "year=2025/month=2/day=30/in.csv", [3]int{1970, 1, 1}},
		{"no file key: 1970", "", "", [3]int{1970, 1, 1}},
	}
	for _, c := range cases {
		y, m, d := schemaEventDate(c.fileDate, c.fileKey)
		if got := [3]int{y, m, d}; got != c.want {
			t.Errorf("%s: schemaEventDate(%q, %q) = %v, want %v", c.name, c.fileDate, c.fileKey, got, c.want)
		}
	}
}

func TestRegistryKeyOf(t *testing.T) {
	for _, c := range []struct {
		in   any
		want int64
	}{
		{"42", 42},
		{" 1000000 ", 1000000},
		{float64(1000000), 1000000}, // was formatted 1e+06 and refused
		{float64(123456789012), 123456789012},
		{json.Number("2000000"), 2000000},
		{7, 7},
		{int64(8), 8},
		{int32(9), 9},
	} {
		got, err := registryKeyOf(c.in)
		if err != nil || got != c.want {
			t.Errorf("registryKeyOf(%#v) = %d, %v; want %d", c.in, got, err, c.want)
		}
	}
	for _, bad := range []any{"", "abc", "1e6", 1.5, math.NaN(), math.Inf(1), float64(1 << 60), json.Number("1.5"), true, []any{1}} {
		if got, err := registryKeyOf(bad); err == nil {
			t.Errorf("registryKeyOf(%#v) = %d, want an error", bad, got)
		}
	}
}

func TestProcessMappingTableName(t *testing.T) {
	for _, c := range []struct {
		row     map[string]any
		want    string
		derived bool
	}{
		{map[string]any{"client": "c", "object_type": "T"}, "c_T", true},
		{map[string]any{"client": "c", "org": "o", "object_type": "T", "table_name": "x"}, "c_o_T", true},
		{map[string]any{"client": "c", "org": nil, "object_type": "T"}, "c_T", true},
		{map[string]any{"client": "c", "table_name": "x"}, "x", false},
		{map[string]any{"table_name": "x"}, "x", false},
	} {
		got, derived, err := processMappingTableName(c.row)
		if err != nil || got != c.want || derived != c.derived {
			t.Errorf("processMappingTableName(%v) = %q, %v, %v; want %q, %v", c.row, got, derived, err, c.want, c.derived)
		}
	}
	for _, c := range []struct {
		row     map[string]any
		missing string
	}{
		{map[string]any{}, "no client, no object_type, no table_name"},
		{map[string]any{"client": "c"}, "no object_type, no table_name"},
		{map[string]any{"object_type": "T", "table_name": nil}, "no client, no table_name"},
	} {
		_, _, err := processMappingTableName(c.row)
		if err == nil || !strings.Contains(err.Error(), c.missing) {
			t.Errorf("processMappingTableName(%v) error = %v, want one naming %q", c.row, err, c.missing)
		}
	}
}

// A schema event registers under the date of its file key when its file_date is
// absent or does not parse, and under its file_date when it parses.
func TestSchemaEventRegistersUnderItsFileKeyDate(t *testing.T) {
	ctx := registerFileKeysFixture(t)
	seedSource(t, ctx, "Claims", 0, "{Claims}")
	startPipelinesOverride = func(*DataTableContext, int, int, string, string, string, string, string, bool) error {
		return nil
	}
	t.Cleanup(func() { startPipelinesOverride = nil })

	for _, c := range []struct {
		name, fileDate, fileKey, want string
	}{
		{"no file_date", "", "in/year=2025/month=3/day=7/a.csv", "2025-03-07"},
		{"unparseable file_date", "unknown", "in/year=2025/month=4/day=8/b.csv", "2025-04-08"},
		{"file_date", "2024-11-30", "in/year=2025/month=5/day=9/c.csv", "2024-11-30"},
		{"neither", "", "in/d.csv", "1970-01-01"},
	} {
		t.Run(c.name, func(t *testing.T) {
			ev := &compute_pipes.SchemaProviderSpec{Type: "default", Client: testClient, Vendor: testOrg,
				ObjectType: "Claims", FileDate: c.fileDate}
			ev.FileKey = c.fileKey
			if err := ctx.RegisterSchemaEvent(ctx.Dbpool, ev, ""); err != nil {
				t.Fatal(err)
			}
			var got string
			if err := ctx.Dbpool.QueryRow(context.Background(), `
				SELECT make_date(sp.year, sp.month, sp.day)::text
				FROM jetsapi.input_registry ir JOIN jetsapi.source_period sp ON sp.key = ir.source_period_key
				WHERE ir.file_key = $1`, c.fileKey).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("registered under %s, want %s", got, c.want)
			}
		})
	}
}

// A submitted start whose main_input_registry_key is missing or is not an
// integer is refused with 400 before the run is inserted.
func TestSubmittedStartRegistryKeyIsCheckedFirst(t *testing.T) {
	f := newStartFixture(t)
	f.ctx.DevMode = true
	for i, c := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"no key", func(r map[string]any) { delete(r, "main_input_registry_key") }},
		{"nil key", func(r map[string]any) { r["main_input_registry_key"] = nil }},
		{"text key", func(r map[string]any) { r["main_input_registry_key"] = "abc" }},
		{"fractional key", func(r map[string]any) { r["main_input_registry_key"] = 1.5 }},
	} {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "runk" + string(rune('a'+i))
			code, err, panicked := f.submit(sessionId, c.edit)
			if panicked != nil {
				t.Fatalf("the handler panicked: %v", panicked)
			}
			if code != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), "main_input_registry_key") {
				t.Errorf("answered %d, %v; want 400 naming main_input_registry_key", code, err)
			}
			// Counted by session alone: a run inserted without a key has no
			// input session, which f.runs filters on.
			var n int
			if err := f.pool.QueryRow(context.Background(), `SELECT count(*)
				FROM jetsapi.pipeline_execution_status WHERE session_id = $1`, sessionId).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d runs inserted for a refused start, want 0", n)
			}
		})
	}
}

// A key of 1,000,000 sent as a JSON number, a float64 once decoded, is read as
// the integer it is: the run is inserted with that key and its input session,
// and started. It was formatted 1e+06, and the start failed after the insert.
func TestALargeNumericRegistryKeyIsReadExactly(t *testing.T) {
	f := newStartFixture(t)
	bg := context.Background()
	for _, s := range []string{
		`UPDATE jetsapi.input_registry SET key = 1000000 WHERE key = ` + fmt.Sprint(f.registryKey),
		`UPDATE jetsapi.process_config SET state_machine_name = 'cpipesSM'`,
	} {
		if _, err := f.pool.Exec(bg, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	var started []string
	saved := startExecution
	startExecution = func(arn string, input map[string]any, name string) (string, error) {
		started = append(started, name)
		return name, nil
	}
	t.Cleanup(func() { startExecution = saved })

	var row map[string]any
	if err := json.Unmarshal([]byte(`{
		"pipeline_config_key": 1, "main_input_registry_key": 1000000, "main_input_file_key": "k",
		"merged_input_registry_keys": "{}", "file_key": "k", "client": "c1", "process_name": "proc",
		"main_object_type": "Claim", "session_id": "runbig", "source_period_key": 1,
		"status": "submitted", "user_email": "u@example.com"}`), &row); err != nil {
		t.Fatal(err)
	}
	if _, ok := row["main_input_registry_key"].(float64); !ok {
		t.Fatalf("the decoder gave a %T, the premise of this test is a float64", row["main_input_registry_key"])
	}
	results := map[string]any{}
	_, code, err := f.ctx.InsertPipelineExecutionStatus(&DataTableAction{
		Action:         "insert_rows",
		FromClauses:    []FromClause{{Schema: "jetsapi", Table: "pipeline_execution_status"}},
		Data:           []map[string]any{row},
		SkipThrottling: true,
	}, 0, &results, "")
	if code != http.StatusOK || err != nil {
		t.Fatalf("answered %d, %v", code, err)
	}
	var key int64
	if err := f.pool.QueryRow(bg, `SELECT main_input_registry_key FROM jetsapi.pipeline_execution_status
		WHERE session_id = 'runbig' AND input_session_id = 'f1'`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	if key != 1000000 {
		t.Errorf("main_input_registry_key = %d, want 1000000", key)
	}
	if len(started) != 1 || started[0] != "runbig" {
		t.Errorf("state machines started: %q, want runbig", started)
	}
}

// A pasted process mapping with no client and object_type and no table_name is
// a 400 naming what is missing, and deletes nothing: it panicked the handler.
func TestProcessMappingNamingNoTableIsRefused(t *testing.T) {
	pool := createTablesFromSchema(t, "users", "process_mapping")
	bg := context.Background()
	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "jm102-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	for _, s := range []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles) VALUES
			('admin@test', 'Admin', 'hash', 1, '{}')`,
		`INSERT INTO jetsapi.process_mapping (table_name, input_column, data_property, user_email)
			VALUES ('c_T', 'a', 'ex:a', 'admin@test')`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	token, err := user.CreateToken("admin@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &DataTableContext{Dbpool: pool}
	insert := func(raw string) (code int, err error, panicked any) {
		defer func() { panicked = recover() }()
		_, code, err = ctx.InsertRawRows(&DataTableAction{
			Action:      "insert_raw_rows",
			FromClauses: []FromClause{{Schema: "jetsapi", Table: "raw_rows/process_mapping"}},
			Data:        []map[string]any{{"raw_rows": raw, "user_email": "admin@test"}},
		}, token)
		return
	}
	code, err, panicked := insert("input_column,data_property\nb,ex:b\n")
	if panicked != nil {
		t.Fatalf("the handler panicked: %v", panicked)
	}
	if code != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), "no client, no object_type, no table_name") {
		t.Errorf("answered %d, %v; want 400 naming client, object_type and table_name", code, err)
	}
	var n int
	if err := pool.QueryRow(bg, `SELECT count(*) FROM jetsapi.process_mapping`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("process_mapping has %d rows, want the 1 it had", n)
	}

	// The same paste naming its table replaces that table's mapping.
	code, err, panicked = insert("client,object_type,input_column,data_property\nc,T,b,ex:b\n")
	if code != http.StatusOK || err != nil || panicked != nil {
		t.Fatalf("a mapping naming its table answered %d, %v (panic: %v)", code, err, panicked)
	}
	var col string
	if err := pool.QueryRow(bg, `SELECT string_agg(input_column, ',') FROM jetsapi.process_mapping WHERE table_name = 'c_T'`).Scan(&col); err != nil {
		t.Fatal(err)
	}
	if col != "b" {
		t.Errorf("c_T maps %q, want the pasted b alone", col)
	}
}
