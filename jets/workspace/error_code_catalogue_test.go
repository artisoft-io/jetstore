package workspace

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/errorcode"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const raisingRules = `
text configError = "Configuration Error: missing property";
[R_Literal]:
  (?s rdf:type jets:State)
->
  (?s jets:exception "plain literal").
  (?s jets:exception configError)
;
[R_Chain]:
  (?s rdf:type jets:State).
  (?s ex:v ?v)
->
  (?s jets:exception ("error A: " + ?v)).
  (?s jets:exception "head " + "two: " + ?v + " tail").
  (?s jets:exception ?v + " after a value").
  (?s jets:exception ?v).
  (?s jets:exception "two" + " literals")
;
[R_NoRaise]:
  (?s rdf:type jets:State)
->
  (?s ex:done "yes")
;
`

func compileRules(t *testing.T, src string) *rete.JetruleModel {
	t.Helper()
	c := compiler.NewCompiler(t.TempDir(), "x.jr", false, false, true)
	if err := c.CompileBuffer(src); err != nil {
		t.Fatalf("compile: %v\n%s", err, c.ErrorLog().String())
	}
	return c.JetRuleModel()
}

func TestExceptionRaises(t *testing.T) {
	got := ExceptionRaises(compileRules(t, raisingRules))
	want := []errorcode.Raise{
		{Rule: "R_Literal", Head: "plain literal"},
		{Rule: "R_Literal", Head: "Configuration Error: missing property"},
		{Rule: "R_Chain", Head: "error A: ", HasValue: true},
		{Rule: "R_Chain", Head: "head two: ", HasValue: true},
		{Rule: "R_Chain", Head: "", HasValue: true},
		{Rule: "R_Chain", Head: "", HasValue: true},
		{Rule: "R_Chain", Head: "two literals"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d raises, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("raise %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if ExceptionRaises(nil) != nil {
		t.Error("a nil model raises nothing")
	}
	noRaise := compileRules(t, "[R]:\n  (?s rdf:type jets:State)\n->\n  (?s ex:done \"yes\")\n;\n")
	if r := ExceptionRaises(noRaise); len(r) != 0 {
		t.Errorf("no raise expected: %+v", r)
	}
}

func TestAddRaisesDeduplicates(t *testing.T) {
	var dst []errorcode.Raise
	a := errorcode.Raise{File: "a.jr", Rule: "R", Head: "x"}
	b := errorcode.Raise{File: "b.jr", Rule: "R", Head: "x"}
	addRaises(&dst, []errorcode.Raise{a, b, a})
	addRaises(&dst, []errorcode.Raise{b})
	if len(dst) != 2 {
		t.Errorf("got %+v", dst)
	}
}

// The compile collects the raises of every main rule file, with the file each
// rule is in, once when a file is imported by several main files. Run as a
// child, because the workspace home is read from the environment at start.
func TestCompileCollectsTheRaises(t *testing.T) {
	if os.Getenv("ERRCODE_RAISES_CHILD") == "1" {
		ctl, err := rete.LoadWorkspaceControl(filepath.Join(workspaceHome, "test_ws", "workspace_control.json"))
		if err != nil {
			t.Fatal(err)
		}
		ctl.WorkspaceName = "test_ws"
		var raises []errorcode.Raise
		out, err := compileWorkspaceV2(nil, ctl, "errcode_raises", CollectExceptionRaises(&raises))
		if err != nil {
			t.Fatalf("compile: %v\n%s", err, out)
		}
		SortRaises(raises)
		got := map[string]errorcode.Raise{}
		for _, r := range raises {
			key := r.File + "|" + r.Rule + "|" + r.Head
			if _, dup := got[key]; dup {
				t.Errorf("raise listed twice: %+v", r)
			}
			got[key] = r
		}
		var heads []string
		for _, r := range raises {
			heads = append(heads, r.Ref()+" '"+r.Head+"'")
		}
		joined := strings.Join(heads, "\n")
		for _, want := range []string{
			"jet_rules/test_execute_rules_main.jr ",
			"'xr exception raised'",
			"jet_rules/test_execute_rules_second.jr ",
			"'xr second exception raised'",
		} {
			if !strings.Contains(joined, want) {
				t.Errorf("the raises do not include %q:\n%s", want, joined)
			}
		}
		rep := errorcode.Check(raises, nil, 0)
		if len(rep.Uncatalogued) == 0 || !strings.Contains(rep.InsertSQL(), "'xr second exception raised'") {
			t.Errorf("no row for the raises:\n%s", rep)
		}
		return
	}
	home := t.TempDir()
	ws := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", "../jetrules/test_ws", ws).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	_ = os.Remove(filepath.Join(ws, "workspace.db"))
	schemaScript, err := filepath.Abs("../workspace_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCompileCollectsTheRaises$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "ERRCODE_RAISES_CHILD=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+schemaScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child compile: %v\n%s", err, out)
	}
}

func TestCheckErrorCodeCatalogueWithoutADatabase(t *testing.T) {
	got := CheckErrorCodeCatalogue(context.Background(), nil, []errorcode.Raise{{Head: "x"}})
	if !strings.Contains(got, "not checked, no database (1 jets:exception raises)") {
		t.Errorf("got %q", got)
	}
}

// Against PostgreSQL (JETS_TEST_DSN): a database with no catalogue table is a
// log line, and the table as jets_schema.json declares it is read and checked.
func TestCheckErrorCodeCatalogueAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; point it at a throwaway PostgreSQL")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err = pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi.error_code_catalogue"); err != nil {
		t.Fatal(err)
	}
	raises := []errorcode.Raise{
		{File: "a.jr", Rule: "R1", Head: "error SKU not found, SKU: ", HasValue: true},
		{File: "b.jr", Rule: "R2", Head: "new message"},
	}

	// Not migrated yet
	got := CheckErrorCodeCatalogue(ctx, pool, raises)
	if !strings.Contains(got, "not checked, the database has no table jetsapi.error_code_catalogue yet") {
		t.Errorf("missing table: %q", got)
	}

	// Migrated, from the shipped definition
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	var def *schema.TableDefinition
	for i := range defs {
		if defs[i].TableName == "error_code_catalogue" {
			def = &defs[i]
		}
	}
	if def == nil {
		t.Fatal("jets_schema.json has no error_code_catalogue")
	}
	if err = def.UpdateTableSchema(pool, true); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi.error_code_catalogue")
	// The table a second migration leaves as it is
	if err = def.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating again: %v", err)
	}
	// In the base init script's style; code, kind and match_prefix defaulted
	_, err = pool.Exec(ctx, `
DELETE FROM jetsapi.error_code_catalogue;
INSERT INTO jetsapi.error_code_catalogue
  (operator_type, match_prefix, code, kind, report_threshold, source, source_ref, description) VALUES
  ('*', '', NULL, 'error', 10, 'rules', NULL, 'workspace default'),
  ('jetrules', 'error SKU not found', 'SKU not found', 'error', NULL, 'rules', 'a.jr R1', NULL),
  ('jetrules', 'Old message', NULL, 'error', 50, 'rules', 'c.jr R9', NULL)
ON CONFLICT DO NOTHING
;
INSERT INTO jetsapi.error_code_catalogue (operator_type, source) VALUES ('map_record', 'pipeline_config');`)
	if err != nil {
		t.Fatal(err)
	}
	var kind, prefix string
	if err = pool.QueryRow(ctx, `SELECT kind, match_prefix FROM jetsapi.error_code_catalogue WHERE operator_type = 'map_record'`).
		Scan(&kind, &prefix); err != nil || kind != "error" || prefix != "" {
		t.Errorf("column defaults: kind %q prefix %q err %v", kind, prefix, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO jetsapi.error_code_catalogue (operator_type, match_prefix, source)
		VALUES ('jetrules', 'error SKU not found', 'rules')`); err == nil {
		t.Error("the key (operator_type, match_prefix) is not unique")
	}

	got = CheckErrorCodeCatalogue(ctx, pool, raises)
	for _, want := range []string{
		"2 jets:exception raises with 2 distinct heads, 4 catalogue rows",
		"'new message' (b.jr R2)",
		"row (jetrules, 'Old message'): a 'rules' row no raise in the rule files matches",
		"row (jetrules, 'Old message'): report_threshold 50 is above the 20 rows",
		"('jetrules', 'new message', 'new message', 'error', NULL, 'rules', 'b.jr R2', NULL)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the check does not say %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "error SKU not found, SKU") {
		t.Errorf("a catalogued raise was reported:\n%s", got)
	}

	// And a reader matches with what it read
	rows, err := pool.Query(ctx, errorcode.SelectCatalogueSQL)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := errorcode.ReadCatalogue(rows)
	rows.Close()
	if err != nil {
		t.Fatal(err)
	}
	m := errorcode.NewCatalogue(entries).Match("", "jets:exception caught: error SKU not found, SKU: 1")
	if m.Code() != "SKU not found" || m.Threshold != 10 {
		t.Errorf("match from the table: %+v", m)
	}
}
