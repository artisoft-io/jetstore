package main

// ONE RETRACTION BEHAVIOUR AND THE VERTEX VISIT LIMIT (rules_unit_test_harness Phase 1,
// task AF.6; rules_benchmark Phase 8, BQ.2).
//
// harness.Run with the C++ adaptor, as this command runs it, over a scratch copy of
// jets/jetrules/test_ws with three fixtures added:
//
//   - LEGACY defect 1 in definitions sharing one rule file. Until October 2026 a test's
//     legacy object chose between legacy, where the total inferred stays after its row
//     is withdrawn, and fixed-1, where it is retracted. The LEGACY retraction mode is
//     deleted: the total is retracted, a legacy object whose switches are false is
//     accepted and changes nothing, and one with a switch true is refused.
//   - a rule that never stops inferring, with jets:max_vertex_visits = 50 set through
//     rule_config: the run must end at the vertex visit limit. ReteSession::initialize
//     reads it from the RDF session, which sees the meta graph the harness asserted
//     rule_config into before the first session opened. Measured 2026-10-11: C++ stops
//     at exactly 50 visits of the rule's vertex, with "Maximum vertex visits reached".
//
// Until the Go engine's retirement (October 2026) these ran on both engines, defect 1
// with compare_engines; they now run on C++ alone.
//
// Needs libjets.so: set CGO_CFLAGS, CGO_LDFLAGS and LD_LIBRARY_PATH to a build of this
// commit. Runs in a child process, because the
// rete, workspace and compute_pipes packages bind the workspace in init. -count=1.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const legacyModeChildEnv = "JETS_RULES_UNIT_TEST_LEGACY_MODE_CHILD"

const afDir = "jet_rules/unit_tests/af/"

// Defect 1, as the C++ case ConsequentAggregateRetractionReplays
// (jets/rete/retract_recorded_consequents_test.cc) builds it by hand, in rules: AF_D1_10 sums while done is absent, AF_D1_20 then adds a
// value, AF_D1_30 then infers done and withdraws AF_D1_10's row.
const afDefect1Rules = `import "data_model/jets_model.jr"

class af_d1:Main {
  $base_classes = [jets:Entity],
  $data_properties = [
    af_d1:value        as array of int,
    af_d1:pendingValue as int,
    af_d1:pendingDone  as int,
    af_d1:done         as int,
    af_d1:total        as int
  ],
  $as_table = false
};

[AF_D1_10, s=10, aggregate_check=false]:
  (?s rdf:type af_d1:Main).
  [(?s exist_not af_d1:done)]
  ->
  (?s af_d1:total (?s sum_values af_d1:value))
;

[AF_D1_20, s=7]:
  (?s af_d1:pendingValue ?v)
  ->
  (?s af_d1:value ?v)
;

[AF_D1_30, s=5]:
  (?s af_d1:pendingDone ?d)
  ->
  (?s af_d1:done ?d)
;
`

const afDefect1Csv = `jets:key,rdf:type,af_d1:pendingDone,af_d1:pendingValue,af_d1:value
main1,"{""af_d1:Main""}",1,4,"{""1"",""2"",""3""}"
`

// A rule that never stops: each inferred n is a new row of the same vertex.
const afRunawayRules = `import "data_model/jets_model.jr"

class af_rv:Counter {
  $base_classes = [jets:Entity],
  $data_properties = [ af_rv:n as array of int ],
  $as_table = false
};

[AF_RV_10]:
  (?s rdf:type af_rv:Counter).
  (?s af_rv:n ?n)
  ->
  (?s af_rv:n (?n + 1))
;
`

const afRunawayCsv = `jets:key,rdf:type,af_rv:n
c1,af_rv:Counter,0
`

func afDefect1Test(legacy map[string]any, total6 bool) string {
	d := map[string]any{
		"test_name":       "defect 1, " + map[bool]string{true: "legacy", false: "fixed"}[total6],
		"jetrules_config": map[string]any{"process_name": afDir + "af_defect1.jr"},
		"input_sessions": []any{map[string]any{
			"name":  "withdrawn",
			"input": []any{"defect1.csv"},
			"expected_triples": []any{
				map[string]any{"s": "main1", "p": "af_d1:done", "o": map[string]any{"value": "1", "type": "int"}},
				map[string]any{"s": "main1", "p": "af_d1:value", "o": map[string]any{"value": "4", "type": "int"}},
			},
			"excluded_triples": []any{
				map[string]any{"s": "main1", "p": "af_d1:total", "o": map[string]any{"value": "10", "type": "int"}},
			},
		}},
	}
	s := d["input_sessions"].([]any)[0].(map[string]any)
	six := map[string]any{"s": "main1", "p": "af_d1:total", "o": map[string]any{"value": "6", "type": "int"}}
	if total6 {
		s["expected_triples"] = append(s["expected_triples"].([]any), six)
	} else {
		s["excluded_triples"] = append(s["excluded_triples"].([]any), map[string]any{"s": "main1", "p": "af_d1:total", "o": "?t"})
	}
	if legacy != nil {
		d["legacy"] = legacy
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	return string(b)
}

func afRunawayTest() string {
	cfg := map[string]any{
		"process_name": afDir + "af_runaway.jr",
		"rule_config": []any{map[string]any{
			"jets:key":               "jets:iState",
			"jets:max_vertex_visits": map[string]any{"value": "50", "type": "int"},
		}},
	}
	d := map[string]any{
		"test_name":       "runaway",
		"expect":          map[string]any{"status": "error", "reason": "Maximum vertex visits reached"},
		"jetrules_config": cfg,
		"input_sessions":  []any{map[string]any{"name": "one counter", "input": []any{"runaway.csv"}}},
	}
	b, _ := json.MarshalIndent(d, "", "  ")
	return string(b)
}

func TestOneRetractionBehaviourAndVisitLimit(t *testing.T) {
	if os.Getenv(legacyModeChildEnv) == "" {
		stageAndRerun(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "af6",
		workspace.SkipAssetValidation("AF.6 test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	root := filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws")
	run := func(test string) *rulestest.Result {
		return harness.Run(harness.Options{
			WorkspaceRoot: root,
			TestPath:      afDir + test,
			Factory:       jetrules_native_adaptor.NewJetRulesFactory(),
		})
	}
	want := func(r *rulestest.Result, status, mode, reasonPart string) {
		t.Helper()
		if r.Status != status || r.LegacyMode != mode || !strings.Contains(r.Reason, reasonPart) {
			b, _ := json.MarshalIndent(r, "", "  ")
			t.Errorf("%s: status %q mode %q reason %q; want %q, %q, reason containing %q\n%s",
				r.TestPath, r.Status, r.LegacyMode, r.Reason, status, mode, reasonPart, b)
		}
	}

	// Defect 1 is gone: the total is retracted with its row, with no legacy object.
	r := run("defect1.tst.json")
	want(r, rulestest.StatusPass, rulestest.LegacyModeFixed, "")
	if r.Engine != rulestest.EngineNative {
		t.Errorf("defect1: engine %q, want native", r.Engine)
	}
	// The same rules expecting the legacy answer, the total that stays, fail.
	want(run("defect1_legacy_answer.tst.json"), rulestest.StatusFail, rulestest.LegacyModeFixed, "checks failed")
	// A legacy object whose switches are false names the one behaviour: accepted, the
	// same answer (rules_benchmark Q-102).
	want(run("defect1_switches_off.tst.json"), rulestest.StatusPass, rulestest.LegacyModeFixed, "")
	// A switch set true asks for a deleted behaviour, and is refused before any session.
	r = run("defect1_switch_on.tst.json")
	want(r, rulestest.StatusError, rulestest.LegacyModeFixed, rulestest.LegacyRetractionDeleted)
	if len(r.SessionResults) != 0 {
		t.Errorf("defect1_switch_on: %d sessions ran", len(r.SessionResults))
	}

	// jets:max_vertex_visits from rule_config stops a runaway rule.
	r = run("runaway.tst.json")
	want(r, rulestest.StatusPass, rulestest.LegacyModeFixed, "the expected error occurred")
	for _, s := range r.SessionResults {
		t.Logf("runaway: session %q: %s", s.Name, s.Reason)
	}
}

// stageAndRerun copies test_ws with the fixtures to a scratch WORKSPACES_HOME and
// re-runs the test in a child pointed at it.
func stageAndRerun(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd() // jets/cmds/rules_unit_test
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	home := t.TempDir()
	ws := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), ws).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	ctlPath := filepath.Join(ws, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws"
	ctl["rule_sets"] = append(ctl["rule_sets"].([]any), afDir+"af_defect1.jr", afDir+"af_runaway.jr")
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"af_defect1.jr":                  afDefect1Rules,
		"defect1.csv":                    afDefect1Csv,
		"defect1.tst.json":               afDefect1Test(nil, false),
		"defect1_legacy_answer.tst.json": afDefect1Test(nil, true),
		"defect1_switches_off.tst.json":  afDefect1Test(map[string]any{"recompute_on_retraction": false, "withdrawn_row_stays": false}, false),
		"defect1_switch_on.tst.json":     afDefect1Test(map[string]any{"recompute_on_retraction": true}, false),
		"af_runaway.jr":                  afRunawayRules,
		"runaway.csv":                    afRunawayCsv,
		"runaway.tst.json":               afRunawayTest(),
	}
	dir := filepath.Join(ws, afDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), legacyModeChildEnv+"=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: "+t.Name()) {
		t.Fatalf("child run failed: %v\n%s", err, out)
	}
	if testing.Verbose() {
		var keep []string
		for line := range strings.SplitSeq(string(out), "\n") {
			if strings.Contains(line, "--- ") || strings.Contains(line, "_test.go:") {
				keep = append(keep, line)
			}
		}
		t.Logf("child process:\n%s", strings.Join(keep, "\n"))
	}
}

// -engine names the only engine or is refused: go naming the retirement
// (rules_benchmark Q-77), anything else as unknown.
func TestEngineFlag(t *testing.T) {
	for _, ok := range []string{"", rulestest.EngineNative} {
		if err := checkEngineFlag(ok); err != nil {
			t.Errorf("-engine %q: %v", ok, err)
		}
	}
	if err := checkEngineFlag("go"); err == nil || !strings.Contains(err.Error(), rulestest.GoEngineRetired) {
		t.Errorf("-engine go: want the retirement, got %v", err)
	}
	if err := checkEngineFlag("python"); err == nil || !strings.Contains(err.Error(), "unknown engine") {
		t.Errorf("-engine python: %v", err)
	}
}
