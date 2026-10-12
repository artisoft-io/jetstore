package harness

// The matcher against a real Go engine session, on rules that mint subjects.
//
// test_looping_main.jr (jets/jetrules/test_ws) infers (?state newPerson
// (create_entity 0)) and then names each new person, so its output can only be
// checked as "some lp:Person has lp:name person_2" (definition F57). This test
// compiles a scratch copy of test_ws, runs that rule file through the looping
// protocol on the Go engine, snapshots the session and evaluates checks on it.
//
// WHY A CHILD PROCESS. The rete package and compute_pipes read WORKSPACES_HOME
// and WORKSPACE in init (definition F16), and only package-internal variables
// repoint them, so this package can point them at the scratch workspace only by
// running the test binary again with the environment set.
//
// The loop here is a stand-in for the one track AA extracts from executeRules
// (RunRuleSets); the harness will call that, and this test will too once it is
// exported. What this test is about is the snapshot and the match.
//
// -count=1: it compiles files outside the module's cache key.

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
)

const loopingRuleFile = "jet_rules/test_looping_main.jr"

// copyTestWorkspace copies test_ws to a scratch WORKSPACES_HOME; the child
// compiles it, because jets/workspace binds its paths in init too.
func copyTestWorkspace(t *testing.T) (home, root string) {
	t.Helper()
	wd, err := os.Getwd() // jets/rulestest/harness
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	home = t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	// The compiler builds its paths from workspace_control.json's workspace_name,
	// which the checked-in file lacks (see rerunOverCompiledTestWs in
	// jets/jetrules/rete/retraction_mode_factory_test.go).
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws"
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return home, root
}

func TestGoEngineCreateEntitySubjects(t *testing.T) {
	if os.Getenv("RULESTEST_HARNESS_CHILD") == "1" {
		goEngineChild(t)
		return
	}
	if testing.Short() {
		t.Skip("compiles a workspace")
	}
	home, root := copyTestWorkspace(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestGoEngineCreateEntitySubjects$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "RULESTEST_HARNESS_CHILD=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestGoEngineCreateEntitySubjects") {
		t.Fatalf("child run failed: %v\n%s", err, out)
	}
	t.Logf("child:\n%s", out)
}

func goEngineChild(t *testing.T) {
	// In-process, without the user-flow asset validation, which needs a node
	// script beside the binary and checks documents this test does not touch.
	if _, err := workspace.CompileWorkspace(nil, "test_ws", "rulestest",
		workspace.SkipAssetValidation("a scratch copy of test_ws compiled for its rules only")); err != nil {
		t.Fatalf("compiling test_ws: %v", err)
	}
	engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, loopingRuleFile, false)
	if err != nil {
		t.Fatalf("building the engine: %v", err)
	}
	defer engine.Release()
	// JetResources before the first session: the meta graph locks when a
	// session opens, and a new meta resource is refused after that on both
	// engines (on C++ it aborted the process until I-15 was fixed).
	jr := engine.JetResources()
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatal(err)
	}
	defer ses.Release()
	rete, err := ses.NewReteSession(loopingRuleFile)
	if err != nil {
		t.Fatal(err)
	}
	defer rete.Release()
	rm := ses.GetResourceManager()
	// LOOP01 needs (jets:iState rdf:type jets:State), which the rule file gets
	// from importing data_model/jets_model.jr, where it is a triple(...).
	completedAt := -1
	for iloop := 0; iloop <= 5; iloop++ { // $max_looping = 5 in the rule file
		if iloop > 0 {
			if err := ses.Insert(jr.Jets__istate, jr.Jets__loop, rm.NewIntLiteral(iloop)); err != nil {
				t.Fatal(err)
			}
		}
		if err := rete.ExecuteRules(); err != nil {
			t.Fatalf("ExecuteRules at loop %d: %v", iloop, err)
		}
		if ses.ContainsSP(jr.Jets__istate, jr.Jets__completed) {
			completedAt = iloop
			break
		}
	}
	if completedAt != 3 {
		t.Fatalf("completed at loop %d, want 3 (LOOP03)", completedAt)
	}
	snap, err := TakeSnapshot(ses)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	for _, tr := range snap.sortedTriples() {
		if tr.P.Text == "lp:name" || tr.P.Text == "jets:completed" || tr.O.Text == "lp:Person" {
			t.Logf("%s", tr)
		}
	}

	persons := `[{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}}]`
	cases := []struct {
		list, check, outcome, found string
	}{
		{rulestest.KindExpected, `{"s": "jets:iState", "p": "jets:completed", "o": {"value": "1", "type": "int"}}`, rulestest.OutcomePass, "found"},
		// Criterion 7: a subject made by create_entity, reached through a variable.
		{rulestest.KindExpected, `{"pattern": [
			{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
			{"s": "?p", "p": "lp:name", "o": "person_2"}]}`, rulestest.OutcomePass, "3 matches: {?p="},
		{rulestest.KindExcluded, `{"pattern": [
			{"s": "?p", "p": "rdf:type", "o": {"value": "lp:Person", "type": "resource"}},
			{"s": "?p", "p": "lp:name", "o": "person_4"}]}`, rulestest.OutcomePass, "no match"},
		{rulestest.KindExcluded, `{"pattern": [{"s": "?p", "p": "lp:name", "o": "?n"}]}`, rulestest.OutcomeFail, "match"},
		// newPerson is a volatile_resource: what LOOP01 inferred on it is gone
		// when the run ends, so a count states it.
		{rulestest.KindExpected, `{"s": "jets:iState", "p": "newPerson", "count": ` +
			strings.Replace(countEq, "%s", "0", 1) + `}`, rulestest.OutcomePass, "count=0"},
		// Each of the three persons has three names, because loop values are not
		// erased between iterations (definition F2) and LOOP02 fires for each.
		{rulestest.KindExpected, `{"s": "?p", "p": "lp:name", "pattern": ` + persons + `, "count": ` +
			strings.Replace(countEq, "%s", "3", 1) + `}`, rulestest.OutcomePass, "all 3 bindings pass"},
		{rulestest.KindExpected, `{"s": "?p", "p": "lp:name", "pattern": ` + persons + `, "count": ` +
			strings.Replace(countEq, "%s", "1", 1) + `}`, rulestest.OutcomeFail, "3 of 3 bindings fail"},
	}
	for _, c := range cases {
		r, err := snap.EvaluateCheck(check(t, c.check), c.list)
		if err != nil {
			t.Errorf("%s: %v", c.check, err)
			continue
		}
		t.Logf("%s %s -> %s: %s", c.list, r.Check, r.Outcome, r.Found)
		expectOutcome(t, r, c.outcome, c.found)
	}
}
