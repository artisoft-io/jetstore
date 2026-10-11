package workspace

// Rules unit tests recorded in workspace.db at compile.
// Project rules_unit_test_harness, Phase 1, task AD.1.

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	_ "github.com/mattn/go-sqlite3"
)

const validUnitTest = `{
  "test_name": "a valid test",
  "comment": "recorded with no findings",
  "legacy": {"recompute_on_retraction": false},
  "jetrules_config": {
    "process_name": "jet_rules/test_looping_main.jr",
    "input_rdf_type": "hc:Eligibility"
  },
  "input_sessions": [{"name": "s1", "input": ["input.csv"]}]
}`

// writeUnitTests lays out one of each case under <ws>/jet_rules/unit_tests.
func writeUnitTests(t *testing.T, ws string) {
	t.Helper()
	files := map[string]string{
		"good/good.tst.json": validUnitTest,
		"good/input.csv":     "jets:key,rdf:type\nk1,hc:Eligibility\n",
		// Decodes, and refers to an input file that is not there.
		"missing_csv/missing.tst.json": strings.Replace(validUnitTest, `"a valid test"`, `"no input file"`, 1),
		// An unknown key: refused before decoding, so its name is read loosely.
		"unknown_key/unknown.tst.json": strings.Replace(validUnitTest, `"comment"`, `"commment"`, 1),
		"not_json.tst.json":             `{"test_name": `,
		// Not a test definition, so not recorded.
		"notes/readme.json": `{}`,
	}
	for rel, content := range files {
		p := filepath.Join(ws, UnitTestsDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func schemaScript(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../workspace_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type unitTestRow struct {
	path, testName, processName string
	legacyMode, findings        sql.NullString
}

func readUnitTestRows(t *testing.T, db *sql.DB) map[string]unitTestRow {
	t.Helper()
	rows, err := db.Query(`SELECT path, coalesce(test_name, ''), coalesce(process_name, ''), legacy_mode, findings FROM unit_tests`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]unitTestRow{}
	for rows.Next() {
		var r unitTestRow
		if err := rows.Scan(&r.path, &r.testName, &r.processName, &r.legacyMode, &r.findings); err != nil {
			t.Fatal(err)
		}
		out[r.path] = r
	}
	return out
}

// runnable is the predicate the rules_unit_test insert target applies (track AC,
// I-17): a test may be started when its findings are NULL, '', '[]' or 'null'.
func runnable(r unitTestRow) bool {
	return !r.findings.Valid || r.findings.String == "" || r.findings.String == "[]" || r.findings.String == "null"
}

func TestSaveUnitTestsRecordsEveryTestAndRefusesTheInvalidOnes(t *testing.T) {
	ws := t.TempDir()
	writeUnitTests(t, ws)
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(schemaScript(t)); err != nil {
		t.Fatalf("workspace_schema.sql: %v", err)
	}
	// Twice: a recompile replaces what the previous one wrote.
	for i := 0; i < 2; i++ {
		recorded, invalid, err := SaveUnitTests(context.Background(), db, ws)
		if err != nil {
			t.Fatal(err)
		}
		if recorded != 4 || invalid != 3 {
			t.Fatalf("recorded %d, %d invalid; want 4 and 3", recorded, invalid)
		}
	}
	rows := readUnitTestRows(t, db)
	if len(rows) != 4 {
		t.Fatalf("%d rows, want 4: %v", len(rows), rows)
	}

	good := rows["jet_rules/unit_tests/good/good.tst.json"]
	if !runnable(good) || good.findings.Valid {
		t.Errorf("the valid test is not runnable, or has findings that are not NULL: %+v", good)
	}
	if good.testName != "a valid test" || good.processName != "jet_rules/test_looping_main.jr" ||
		good.legacyMode.String != "fixed-1" {
		t.Errorf("the valid test was recorded as %+v", good)
	}

	for path, code := range map[string]string{
		"jet_rules/unit_tests/missing_csv/missing.tst.json": "missing_file",
		"jet_rules/unit_tests/unknown_key/unknown.tst.json": "unknown_field",
		"jet_rules/unit_tests/not_json.tst.json":            "invalid_json",
	} {
		r, ok := rows[path]
		if !ok {
			t.Errorf("%s was not recorded", path)
			continue
		}
		if runnable(r) {
			t.Errorf("%s is runnable, and its file is invalid", path)
			continue
		}
		var findings []map[string]any
		if err := json.Unmarshal([]byte(r.findings.String), &findings); err != nil {
			t.Errorf("%s findings are not a JSON array: %v", path, err)
			continue
		}
		codes := []string{}
		for _, f := range findings {
			if f["severity"] != "error" {
				t.Errorf("%s records a %v finding; only errors are recorded", path, f["severity"])
			}
			codes = append(codes, f["code"].(string))
		}
		if !strings.Contains(strings.Join(codes, ","), code) {
			t.Errorf("%s findings %v, want one coded %s", path, codes, code)
		}
	}
	// A refused definition still shows its name, read loosely.
	if r := rows["jet_rules/unit_tests/unknown_key/unknown.tst.json"]; r.testName != "a valid test" || r.legacyMode.Valid {
		t.Errorf("the refused definition was recorded as %+v", r)
	}
}

func TestReadUnitTestsWithNoUnitTestsDirectory(t *testing.T) {
	records, err := ReadUnitTests(t.TempDir())
	if err != nil || len(records) != 0 {
		t.Fatalf("got %v, %v; want no records and no error", records, err)
	}
}

// TestCompileRecordsUnitTestsAndIsNotFailedByAnInvalidOne compiles a copy of
// jets/jetrules/test_ws carrying the cases above, in a child process because
// WORKSPACES_HOME and JETS_WORKSPACE_DB_SCHEMA_SCRIPT are read at package init.
// The criterion it pins is 18: every test case is recorded, and an invalid test
// file does not fail the compile.
func TestCompileRecordsUnitTestsAndIsNotFailedByAnInvalidOne(t *testing.T) {
	if os.Getenv("RUT_AD1_CHILD") == "1" {
		ctl, err := rete.LoadWorkspaceControl(filepath.Join(workspaceHome, "test_ws", "workspace_control.json"))
		if err != nil {
			t.Fatal(err)
		}
		ctl.WorkspaceName = "test_ws"
		if log, err := compileWorkspaceV2(nil, ctl, "rut_ad1"); err != nil {
			t.Fatalf("compile failed: %v\n%s", err, log)
		} else if !strings.Contains(log, "Recorded 4 rules unit tests in workspace.db, 3 of them invalid") {
			t.Fatalf("the compile log does not report the tests:\n%s", log)
		}
		return
	}
	home := t.TempDir()
	ws := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", "../jetrules/test_ws", ws).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	_ = os.Remove(filepath.Join(ws, "workspace.db"))
	writeUnitTests(t, ws)
	schema, err := filepath.Abs("../workspace_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCompileRecordsUnitTestsAndIsNotFailedByAnInvalidOne$", "-test.count=1")
	cmd.Env = append(os.Environ(), "RUT_AD1_CHILD=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+schema)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child compile: %v\n%s", err, out)
	}
	db, err := sql.Open("sqlite3", filepath.Join(ws, "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows := readUnitTestRows(t, db)
	if len(rows) != 4 || !runnable(rows["jet_rules/unit_tests/good/good.tst.json"]) {
		t.Fatalf("workspace.db records %v", rows)
	}
	// The compile's own tables are there too: the tests are recorded beside them.
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM jet_rules`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("jet_rules has %d rows (%v); the compile did not run", n, err)
	}
}
