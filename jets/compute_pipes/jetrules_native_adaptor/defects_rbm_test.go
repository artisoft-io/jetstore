package jetrules_native_adaptor

// DEFECTS READ FROM THE CODE, PINNED ON THE C++ ENGINE (rules_benchmark Phase 1, task AF.3).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these engine
// differences from the code and registered them in its plan, §10.1, without running
// them. Each test ran the C++ engine and the Go engine through the adaptor interface
// (compute_pipes.JetRulesFactory) and asserted their behaviour; both were resolved in
// Phase 4 to C++'s meaning. The Go engine was retired in Phase 6 (2026-10-22), so
// the tests now run the C++ engine alone and pin what it does.
//
// Needs the cgo environment, as engine_mode_test.go does: CGO_CFLAGS, CGO_LDFLAGS and
// LD_LIBRARY_PATH set to a libjets.so built from this commit. Each test runs in a
// child process over a compiled copy of jets/jetrules/test_ws (runInCompiledTestWs),
// and a C++ engine that does not build fails it. Run with -count=1.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// rbmMainRuleFile is the rule file the tests add to their scratch copy of test_ws.
const rbmMainRuleFile = "jet_rules/rbm_defects_main.jr"

const rbmModel = `# Data model for the rules_benchmark defect tests (defects_rbm_test.go).
class rbm:Item {
  $base_classes = [jets:Entity],
  $data_properties = [
    rbm:zip                   as text,
    rbm:zip3                  as text,
    rbm:fullName              as text,
    rbm:firstName             as text,
    rbm:lastName              as text,
    rbm:i6Flag                as text,
    rbm:i6Out                 as int
  ],
  $as_table = true
};
`

// The patterns are the five production rules' that use literal_regex: one client
// workspace's zip-code prefix rule, used in three of its processes, and another's
// two name splitters.
const rbmRules = `# Main rule file for the rules_benchmark defect tests (defects_rbm_test.go).
import "data_model/jets_model.jr"
import "data_model/rbm_defects_model.jr"

[RBM_Zip3]:
  (?x rdf:type rbm:Item).(?x rbm:zip ?z)
  ->
  (?x rbm:zip3 (?z literal_regex "^(\d\d\d)"))
;

[RBM_I6]:
  (?x rbm:i6Flag "on")
  ->
  (?x rbm:i6Out 1)
;

[RBM_FirstName]:
  (?x rdf:type rbm:Item).(?x rbm:fullName ?n)
  ->
  (?x rbm:firstName (?n literal_regex ",\s+(\w+)")).
  (?x rbm:lastName (?n literal_regex "(.*?),"))
;
`

// rbmScratchWorkspace adds the defect tests' rule file to the scratch test_ws of
// the child process and compiles it.
func rbmScratchWorkspace(t *testing.T) {
	t.Helper()
	wsDir := filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws")
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(wsDir, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("data_model/rbm_defects_model.jr", rbmModel)
	write(rbmMainRuleFile, rbmRules)
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ruleSets, _ := ctl["rule_sets"].([]any)
	if !slices.Contains(ruleSets, any(rbmMainRuleFile)) {
		ctl["rule_sets"] = append(ruleSets, rbmMainRuleFile)
	}
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "rbm",
		workspace.SkipAssetValidation("rules_benchmark defect tests: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
}

// rbmEngine builds the C++ engine on rbmMainRuleFile, with the vertex visit cap.
func rbmEngine(t *testing.T) compute_pipes.JetRuleEngine {
	t.Helper()
	native, err := NewJetRulesFactory().NewJetRuleEngine(nil, rbmMainRuleFile, false)
	if err != nil {
		t.Fatalf("building the C++ engine: %v", err)
	}
	t.Cleanup(func() { native.Release() })
	withVertexCap(t, native)
	return native
}

// TestRegression_I6_RetractDeletes was TestDefect_I6_NativeRetractErases, which
// reproduced rules_benchmark I-6 (F25): the adaptors' Retract disagreed. The native
// adaptor's erased every support of the triple without telling the rete (F331); the
// Go adaptor's called the session's inferred-only retraction, so it never removed a
// triple asserted through the adaptor.
//
// Decided in Phase 4 (Q-63): the engines retract an inferred triple themselves,
// through the session's inferred-only retraction (F333); the adaptor's Retract is
// for an asserted fact and means delete. Both adaptors then erased (the Go one
// through its session's Erase; JetRdfSessionNative.Retract through the bridge's
// erase, which since I-103 tells the rete), and the rete withdraws what
// depended on the fact at the next ExecuteRules.
//
// The rule [RBM_I6] (?x rbm:i6Flag "on") -> (?x rbm:i6Out 1): assert the flag
// twice, run the rules, Retract the flag once, run them again. The flag is gone at
// once, whatever the count, and its consequent after the second run (both engines
// agreed from Phase 4 until the Go engine's retirement).
//
// Reachability: nothing outside tests calls the adaptor's Retract (searched
// 2026-10-20 across jets/ and the four workspaces).
func TestRegression_I6_RetractDeletes(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	rbmScratchWorkspace(t)
	type outcome struct {
		inferred, retracted, flagAfterRetract, inferredAfterRun bool
	}
	engine := rbmEngine(t)
	const name = "native"
	{
		ses, err := engine.NewRdfSession()
		if err != nil {
			t.Fatalf("%s: NewRdfSession: %v", name, err)
		}
		rm := ses.GetResourceManager()
		s, flag, on := rm.NewResource("i6_s"), rm.NewResource("rbm:i6Flag"), rm.NewTextLiteral("on")
		out, one := rm.NewResource("rbm:i6Out"), rm.NewIntLiteral(1)
		for i := 0; i < 2; i++ { // asserted twice
			if err := ses.Insert(s, flag, on); err != nil {
				t.Fatalf("%s: Insert: %v", name, err)
			}
		}
		rete, err := ses.NewReteSession(rbmMainRuleFile)
		if err != nil {
			t.Fatalf("%s: NewReteSession: %v", name, err)
		}
		if err := rete.ExecuteRules(); err != nil {
			t.Fatalf("%s: ExecuteRules: %v", name, err)
		}
		var oc outcome
		oc.inferred = ses.Contains(s, out, one)
		if oc.retracted, err = ses.Retract(s, flag, on); err != nil {
			t.Fatalf("%s: Retract: %v", name, err)
		}
		oc.flagAfterRetract = ses.Contains(s, flag, on)
		if err := rete.ExecuteRules(); err != nil {
			t.Fatalf("%s: ExecuteRules after Retract: %v", name, err)
		}
		oc.inferredAfterRun = ses.Contains(s, out, one)
		rete.Release()
		ses.Release()
		want := outcome{inferred: true, retracted: true, flagAfterRetract: false, inferredAfterRun: false}
		if oc != want {
			t.Errorf("%+v, want %+v (Retract deletes the asserted fact, and the rete withdraws its consequent)", oc, want)
		}
	}
}

// describe renders an adaptor node for comparison: <absent>, <null>, or its text.
func describe(n compute_pipes.RdfNode) string {
	switch {
	case n == nil || n.IsNil():
		return "<absent>"
	case n.Type() == "null":
		return "<null>"
	default:
		return fmt.Sprintf("%v", n.Value())
	}
}

// TestDefect_I12_LiteralRegexEngines holds rules_benchmark I-12 (F53) fixed
// through a compiled rule: literal_regex returns capture group 1, or null on no
// match, C++'s meaning (RegexVisitor, jets/rete/expr_op_strings.h; Q-11). Until
// Phase 4 the Go engine, retired in Phase 6, returned the whole match, or an empty
// string on no match.
//
// Inputs: the zip-code rule's "^(\d\d\d)" on "12345" and "N/A", and the name splitters' ",\s+(\w+)"
// and "(.*?)," on "DOE, JOHN".
func TestDefect_I12_LiteralRegexEngines(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	rbmScratchWorkspace(t)
	type probe struct{ subject, inProp, in, outProp string }
	probes := []probe{
		{"zip_digits", "rbm:zip", "12345", "rbm:zip3"},
		{"zip_na", "rbm:zip", "N/A", "rbm:zip3"},
		{"name", "rbm:fullName", "DOE, JOHN", "rbm:firstName"},
		{"name", "rbm:fullName", "DOE, JOHN", "rbm:lastName"},
	}
	engine := rbmEngine(t)
	const name = "native"
	var got []string
	{
		ses, err := engine.NewRdfSession()
		if err != nil {
			t.Fatalf("%s: NewRdfSession: %v", name, err)
		}
		rm := ses.GetResourceManager()
		rdfType, item := rm.NewResource("rdf:type"), rm.NewResource("rbm:Item")
		for _, p := range probes {
			s := rm.NewResource(p.subject)
			if err := ses.Insert(s, rdfType, item); err != nil {
				t.Fatal(err)
			}
			if err := ses.Insert(s, rm.NewResource(p.inProp), rm.NewTextLiteral(p.in)); err != nil {
				t.Fatal(err)
			}
		}
		rete, err := ses.NewReteSession(rbmMainRuleFile)
		if err != nil {
			t.Fatalf("%s: NewReteSession: %v", name, err)
		}
		if err := rete.ExecuteRules(); err != nil {
			t.Fatalf("%s: ExecuteRules: %v", name, err)
		}
		for _, p := range probes {
			got = append(got, describe(ses.GetObject(rm.NewResource(p.subject), rm.NewResource(p.outProp))))
		}
		rete.Release()
		ses.Release()
	}
	want := []string{"123", "<absent>", "JOHN", "DOE"}
	if !slices.Equal(got, want) {
		t.Errorf("%q, want %q, C++'s meaning (I-12)", got, want)
	}
}
