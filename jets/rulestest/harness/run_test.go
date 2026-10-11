package harness

// harness.Run end to end on the Go engine: a scratch workspace with test
// definitions and CSVs, compiled in a child process (the rete, workspace and
// compute_pipes packages bind the workspace in init, definition F16), then each
// test run through Run as the rules_unit_test command runs it.
//
// The C++ engine is not linked into this package's tests, which keeps them
// buildable without libjets.so. The same fixtures were run through the command
// on C++; see the command's README.
//
// -count=1: these tests compile files outside the module's cache key, and
// TestRunOnJetsWsEligibility reads workspaces/jets_ws.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/workspace"
	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// stageWorkspace copies src to a scratch WORKSPACES_HOME as ws, sets its
// workspace_name, adds ruleSets to its rule_sets, and writes files (paths
// relative to the workspace root).
func stageWorkspace(t *testing.T, src, ws string, ruleSets []string, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ws)
	if out, err := exec.Command("cp", "-r", src, dir).CombinedOutput(); err != nil {
		t.Fatalf("copying %s: %v\n%s", src, err, out)
	}
	ctlPath := filepath.Join(dir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = ws
	sets, _ := ctl["rule_sets"].([]any)
	for _, rs := range ruleSets {
		sets = append(sets, rs)
	}
	ctl["rule_sets"] = sets
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func moduleRoot(t *testing.T) string {
	wd, err := os.Getwd() // jets/rulestest/harness
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// runChild re-runs test name in a child with the workspace in its environment.
func runChild(t *testing.T, name, home, ws string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "RULESTEST_HARNESS_CHILD=1", "WORKSPACES_HOME="+home, "WORKSPACE="+ws,
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(moduleRoot(t), "jets", "workspace_schema.sql"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: "+name) {
		t.Fatalf("child run failed: %v\n%s", err, out)
	}
}

func compileInChild(t *testing.T, ws string) string {
	t.Helper()
	if _, err := workspace.CompileWorkspace(nil, ws, "rulestest",
		workspace.SkipAssetValidation("a scratch workspace compiled for its rules only")); err != nil {
		t.Fatalf("compiling %s: %v", ws, err)
	}
	return filepath.Join(os.Getenv("WORKSPACES_HOME"), ws)
}

func goOnly(root, test string) *rulestest.Result {
	return Run(Options{
		WorkspaceRoot: root,
		TestPath:      test,
		Factories:     Factories{Go: jetrules_go_adaptor.NewJetRulesFactory()},
	})
}

func wantResult(t *testing.T, r *rulestest.Result, status, reasonPart string) {
	t.Helper()
	if r.Status != status || !strings.Contains(r.Reason, reasonPart) {
		b, _ := json.MarshalIndent(r, "", "  ")
		t.Errorf("%s: status %q reason %q; want %q containing %q\n%s", r.TestPath, r.Status, r.Reason, status, reasonPart, b)
	}
}

const lpRules = `# A rules unit test fixture: greets each person.
import "data_model/jets_model.jr"
import "data_model/test_looping_model.jr"

resource lp:greeting = "lp:greeting";

[LP01]:
  (?p rdf:type lp:Person).
  (?p lp:name ?n)
  ->
  (?p lp:greeting ("hello " + ?n))
;
`

// The CSV writes rdf:type both ways a pipeline reads it, and leaves a cell blank.
const lpPersons = `jets:key,rdf:type,lp:name
p1,"{""lp:Person""}",Ann
p2,lp:Person,Bob
p3,lp:Person,
`

func lpTest(mod func(map[string]any)) string {
	d := map[string]any{
		"test_name": "greetings",
		"jetrules_config": map[string]any{
			"process_name": "jet_rules/unit_tests/lp/lp_test.jr", "use_jet_rules_go": true,
		},
		"input_sessions": []any{map[string]any{
			"name": "three persons", "input": []any{"persons.csv"},
			"expected_triples": []any{map[string]any{"s": "p1", "p": "lp:greeting", "o": "hello Ann"}},
		}},
		"expected_triples": []any{
			map[string]any{"pattern": []any{
				map[string]any{"s": "?p", "p": "rdf:type", "o": map[string]any{"value": "lp:Person", "type": "resource"}},
				map[string]any{"s": "?p", "p": "lp:greeting", "o": "hello Bob"},
			}},
			// p3 has no name, so no greeting: a blank cell asserts nothing.
			map[string]any{"s": "p3", "p": "lp:greeting", "count": map[string]any{
				"lhs": map[string]any{"type": "select", "expr": "count"}, "op": "==",
				"rhs": map[string]any{"type": "value", "expr": "0"}}},
		},
		"excluded_triples": []any{map[string]any{"s": "p3", "p": "lp:name", "o": "?any"}},
	}
	if mod != nil {
		mod(d)
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	return string(b)
}

func cfg(d map[string]any) map[string]any { return d["jetrules_config"].(map[string]any) }

func TestRunOnTestWs(t *testing.T) {
	if os.Getenv("RULESTEST_HARNESS_CHILD") == "1" {
		runOnTestWsChild(t)
		return
	}
	if testing.Short() {
		t.Skip("compiles a workspace")
	}
	u := "jet_rules/unit_tests/lp/"
	files := map[string]string{
		u + "lp_test.jr":          lpRules,
		u + "persons.csv":         lpPersons,
		u + "persons_badcol.csv":  "jets:key,rdf:type,lp:nme\np1,lp:Person,Ann\n",
		u + "persons_badcell.csv": "jets:key,rdf:type,lp:name\np1,lp:Person\n",
		u + "state.csv":           "jets:key,rdf:type\njets:iState,jets:State\n",
		u + "pass.tst.json":       lpTest(nil),
		u + "fail.tst.json": lpTest(func(d map[string]any) {
			d["excluded_triples"] = []any{map[string]any{"s": "p1", "p": "lp:greeting", "o": "?g"}}
		}),
		u + "badcol.tst.json": lpTest(func(d map[string]any) {
			d["input_sessions"].([]any)[0].(map[string]any)["input"] = []any{"persons_badcol.csv"}
		}),
		u + "badcell.tst.json": lpTest(func(d map[string]any) {
			d["input_sessions"].([]any)[0].(map[string]any)["input"] = []any{"persons_badcell.csv"}
		}),
		u + "native.tst.json":     lpTest(func(d map[string]any) { delete(cfg(d), "use_jet_rules_go") }),
		u + "legacy_off.tst.json": lpTest(func(d map[string]any) { d["legacy"] = map[string]any{"recompute_on_retraction": false} }),
		u + "invalid.tst.json":    lpTest(func(d map[string]any) { cfg(d)["pool_size"] = 2 }),
		u + "no_process.tst.json": lpTest(func(d map[string]any) { cfg(d)["process_name"] = "jet_rules/nothing.jr" }),
		u + "bad_check.tst.json": lpTest(func(d map[string]any) {
			d["expected_triples"] = []any{map[string]any{"s": "p1", "p": "lp:x", "o": map[string]any{"value": "abc", "type": "int"}}}
		}),
		u + "loop_expect.tst.json":    loopTest(2, true),
		u + "loop_ends.tst.json":      loopTest(5, true),
		u + "loop_other.tst.json":     strings.Replace(loopTest(2, true), "maximum loop count", "max vertex visit", 1),
		u + "loop_no_expect.tst.json": loopTest(2, false),
	}
	home := stageWorkspace(t, filepath.Join(moduleRoot(t), "jets", "jetrules", "test_ws"), "test_ws",
		[]string{u + "lp_test.jr"}, files)
	runChild(t, "TestRunOnTestWs", home, "test_ws")
}

// loopTest runs test_looping_main.jr, which completes at loop 3, with a
// maximum of max, and expects it to reach the maximum.
func loopTest(max int, expect bool) string {
	d := map[string]any{
		"test_name": "loop to the limit",
		"expect":    map[string]any{"status": "error", "reason": "maximum loop count"},
		"jetrules_config": map[string]any{
			"process_name": "jet_rules/test_looping_main.jr", "use_jet_rules_go": true, "max_looping": max,
		},
		"input_sessions": []any{map[string]any{"name": "state", "input": []any{"state.csv"}}},
		"expected_triples": []any{map[string]any{"pattern": []any{
			map[string]any{"s": "?p", "p": "rdf:type", "o": map[string]any{"value": "lp:Person", "type": "resource"}},
			map[string]any{"s": "?p", "p": "lp:name", "o": "person_2"},
		}}},
	}
	if !expect {
		delete(d, "expect")
	}
	b, _ := json.Marshal(d)
	return string(b)
}

func runOnTestWsChild(t *testing.T) {
	root := compileInChild(t, "test_ws")
	u := "jet_rules/unit_tests/lp/"

	// Criterion 4's content: a result with its engine, mode, timings and checks.
	r := goOnly(root, u+"pass.tst.json")
	wantResult(t, r, rulestest.StatusPass, "")
	if r.Engine != rulestest.EngineGo || r.LegacyMode != rulestest.LegacyModeLegacy || r.Sessions != 1 ||
		len(r.Checks) != 4 || r.TestPath != u+"pass.tst.json" || r.TestName != "greetings" {
		t.Errorf("pass result: %+v", r)
	}
	for _, c := range r.Checks {
		if c.SessionName != "three persons" || c.Outcome != rulestest.OutcomePass {
			t.Errorf("check %+v", c)
		}
	}

	wantResult(t, goOnly(root, u+"fail.tst.json"), rulestest.StatusFail, "1 of 4 checks failed")

	// Criterion 6: a misspelt column names the closest property; a short row is refused.
	wantResult(t, goOnly(root, u+"badcol.tst.json"), rulestest.StatusError,
		"input file persons_badcol.csv: column 'lp:nme' is not a data property of the workspace; did you mean 'lp:name'?")
	wantResult(t, goOnly(root, u+"badcell.tst.json"), rulestest.StatusError, "wrong number of fields")

	// Criterion 10, the Go half: the default engine is C++, and a harness
	// without it refuses rather than running Go in its place.
	r = goOnly(root, u+"native.tst.json")
	wantResult(t, r, rulestest.StatusError, "the native engine is not available")
	if r.Engine != rulestest.EngineNative {
		t.Errorf("engine %q, want native", r.Engine)
	}

	// A LEGACY switch turned off reaches the engine (AF.6): the greetings involve no
	// retraction, so the test passes as in legacy, and the result records the mode
	// the engine reports it was built in.
	r = goOnly(root, u+"legacy_off.tst.json")
	wantResult(t, r, rulestest.StatusPass, "")
	if r.LegacyMode != rulestest.LegacyModeFixed1 {
		t.Errorf("legacy mode %q", r.LegacyMode)
	}

	r = goOnly(root, u+"invalid.tst.json")
	wantResult(t, r, rulestest.StatusError, "invalid test definition: /jetrules_config/pool_size")
	if len(r.Findings) == 0 {
		t.Errorf("an invalid definition's result carries its findings")
	}
	wantResult(t, goOnly(root, u+"no_process.tst.json"), rulestest.StatusError, "jet_rules/nothing.jr")
	wantResult(t, goOnly(root, u+"bad_check.tst.json"), rulestest.StatusError, "cannot be evaluated")

	// Criterion 9: expect turns the expected error into a pass, and a run that
	// terminates into a fail; another error stays an error.
	r = goOnly(root, u+"loop_expect.tst.json")
	wantResult(t, r, rulestest.StatusPass, "the expected error occurred")
	if r.Loops != 2 || len(r.SessionResults) != 1 || !r.SessionResults[0].MaxLoopReached {
		t.Errorf("loop result: loops %d sessions %+v", r.Loops, r.SessionResults)
	}
	wantResult(t, goOnly(root, u+"loop_ends.tst.json"), rulestest.StatusFail, "the rules ran to completion")
	wantResult(t, goOnly(root, u+"loop_other.tst.json"), rulestest.StatusError, `expected an error containing "max vertex visit"; got:`)
	wantResult(t, goOnly(root, u+"loop_no_expect.tst.json"), rulestest.StatusError, "maximum loop count reached (2)")

	// An engine override wins over the test, and a C++-only flag is refused on Go.
	r = Run(Options{WorkspaceRoot: root, TestPath: u + "native.tst.json", Engine: rulestest.EngineGo,
		Factories: Factories{Go: jetrules_go_adaptor.NewJetRulesFactory()}})
	wantResult(t, r, rulestest.StatusPass, "")
}

// TestRunOnJetsWsEligibility asserts the input document's own example row,
// an hc:Eligibility export with the properties in alphabetical order and no
// jets:source_period_sequence (definition F26), through Run on jets_ws.
func TestRunOnJetsWsEligibility(t *testing.T) {
	if os.Getenv("RULESTEST_HARNESS_CHILD") == "1" {
		jetsWsChild(t)
		return
	}
	if testing.Short() {
		t.Skip("compiles a workspace")
	}
	src := filepath.Join(moduleRoot(t), "..", "workspaces", "jets_ws")
	if _, err := os.Stat(filepath.Join(src, "workspace_control.json")); err != nil {
		t.Skipf("workspaces/jets_ws is not checked out beside this repository: %v", err)
	}
	u := "jet_rules/unit_tests/elig/"
	files := map[string]string{
		u + "elig.jr": "import \"data_model/jets_model.jr\"\nimport \"data_model/hc_model.jr\"\n",
		// The input document's row has 30 cells for 31 columns (rules_unit_test_harness
		// F98). Verbatim, it is refused; with the missing cell restored as a trailing
		// blank, CI is jets:client and jets:org is blank.
		u + "eligibility_verbatim.csv": `jets:key,rdf:type,hc:Effective_Date,hc:Group_Number,hc:Medicare_Indicator,hc:Member_Address_Line_1,hc:Member_Address_Line_2,hc:Member_City,hc:Member_DOB,hc:Member_First_Name,hc:Member_Gender,hc:Member_ID,hc:Member_Last_Name,hc:Member_Relationship,hc:Member_State_Code,hc:Member_Zip,hc:Plan_Name,hc:Plan_Type,hc:Subscriber_Address_Line_1,hc:Subscriber_Address_Line_2,hc:Subscriber_City,hc:Subscriber_DOB,hc:Subscriber_First_Name,hc:Subscriber_Gender,hc:Subscriber_ID,hc:Subscriber_Last_Name,hc:Subscriber_State_Code,hc:Subscriber_Zip,hc:Termination_Date,jets:client,jets:org
b10305943c9a5e5e,"{""hc:Eligibility""}",2024-01-01T00:00:00,G01,,,,,1953-04-14T00:00:00,Patient5,F,M005,Last_Name,self,,10023,,,,,,,,,,,,,,CI
`,
		u + "eligibility.csv": `jets:key,rdf:type,hc:Effective_Date,hc:Group_Number,hc:Medicare_Indicator,hc:Member_Address_Line_1,hc:Member_Address_Line_2,hc:Member_City,hc:Member_DOB,hc:Member_First_Name,hc:Member_Gender,hc:Member_ID,hc:Member_Last_Name,hc:Member_Relationship,hc:Member_State_Code,hc:Member_Zip,hc:Plan_Name,hc:Plan_Type,hc:Subscriber_Address_Line_1,hc:Subscriber_Address_Line_2,hc:Subscriber_City,hc:Subscriber_DOB,hc:Subscriber_First_Name,hc:Subscriber_Gender,hc:Subscriber_ID,hc:Subscriber_Last_Name,hc:Subscriber_State_Code,hc:Subscriber_Zip,hc:Termination_Date,jets:client,jets:org
b10305943c9a5e5e,"{""hc:Eligibility""}",2024-01-01T00:00:00,G01,,,,,1953-04-14T00:00:00,Patient5,F,M005,Last_Name,self,,10023,,,,,,,,,,,,,,CI,
`,
		u + "elig.tst.json": `{
  "test_name": "the input document's example row",
  "jetrules_config": {"process_name": "jet_rules/unit_tests/elig/elig.jr", "use_jet_rules_go": true},
  "input_sessions": [{"name": "one member", "input": ["eligibility.csv"]}],
  "expected_triples": [
    {"s": "b10305943c9a5e5e", "p": "rdf:type", "o": {"value": "hc:Eligibility", "type": "resource"}},
    {"s": "b10305943c9a5e5e", "p": "hc:Effective_Date", "o": {"value": "2024-01-01", "type": "date"}},
    {"s": "b10305943c9a5e5e", "p": "hc:Member_DOB", "o": {"value": "1953-04-14", "type": "date"}},
    {"s": "b10305943c9a5e5e", "p": "hc:Group_Number", "o": "G01"},
    {"s": "b10305943c9a5e5e", "p": "jets:client", "o": "CI"}
  ],
  "excluded_triples": [
    {"s": "b10305943c9a5e5e", "p": "hc:Plan_Type", "o": "?any"},
    {"s": "b10305943c9a5e5e", "p": "jets:org", "o": "?any"},
    {"s": "b10305943c9a5e5e", "p": "hc:Effective_Date", "o": {"value": "2024-01-01T00:00:00", "type": "datetime"}}
  ]
}`,
	}
	files[u+"elig_verbatim.tst.json"] = strings.Replace(files[u+"elig.tst.json"], `"eligibility.csv"`, `"eligibility_verbatim.csv"`, 1)
	home := stageWorkspace(t, src, "jets_ws", []string{u + "elig.jr"}, files)
	// The JetStore-owned data_model/ and pipes_config/ assets are not tracked
	// in a workspace repository; a deployment installs them before compiling.
	if _, err := workspace_assets.Install(filepath.Join(home, "jets_ws"), workspace_assets.Options{}); err != nil {
		t.Fatalf("installing the workspace assets: %v", err)
	}
	runChild(t, "TestRunOnJetsWsEligibility", home, "jets_ws")
}

func jetsWsChild(t *testing.T) {
	root := compileInChild(t, "jets_ws")
	r := goOnly(root, "jet_rules/unit_tests/elig/elig.tst.json")
	wantResult(t, r, rulestest.StatusPass, "")
	if len(r.Checks) != 8 {
		t.Errorf("%d checks, want 8: %+v", len(r.Checks), r.Checks)
	}
	r = goOnly(root, "jet_rules/unit_tests/elig/elig_verbatim.tst.json")
	wantResult(t, r, rulestest.StatusError, "input file eligibility_verbatim.csv: record on line 2: wrong number of fields")
}
