package jetrules_native_adaptor

// DEFECTS READ FROM THE CODE, RUN ON BOTH ENGINES (rules_benchmark Phase 1, task AF.3).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these engine
// differences from the code and registered them in its plan, §10.1, without running
// them. Each test runs the C++ engine and the Go engine through the adaptor
// interface (compute_pipes.JetRulesFactory) and asserts TODAY'S behaviour, so it
// passes now and fails when the difference is resolved: a failure after a fix is the
// signal to flip the assertion, not a regression.
//
// Needs the cgo environment, as engine_mode_test.go does: CGO_CFLAGS, CGO_LDFLAGS and
// LD_LIBRARY_PATH set to a libjets.so built from this commit. Each test runs in a
// child process over a compiled copy of jets/jetrules/test_ws (runInCompiledTestWs),
// and skips when the C++ engine cannot be built there. Run with -count=1.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
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
    rbm:lastName              as text
  ],
  $as_table = true
};
`

// The patterns are the five production rules' that use literal_regex: usi_ws z004
// (1_CM01_Mapping_SM_Main2.jr:70, in CM, MSK and IM), and walrus_ws's two name
// splitters (merge_claim_header_and_details_main.jr:129-130).
const rbmRules = `# Main rule file for the rules_benchmark defect tests (defects_rbm_test.go).
import "data_model/jets_model.jr"
import "data_model/rbm_defects_model.jr"

[RBM_Zip3]:
  (?x rdf:type rbm:Item).(?x rbm:zip ?z)
  ->
  (?x rbm:zip3 (?z literal_regex "^(\d\d\d)"))
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

// rbmEngines builds both engines on rbmMainRuleFile, skipping the test when the C++
// engine cannot be built.
func rbmEngines(t *testing.T) map[string]compute_pipes.JetRuleEngine {
	t.Helper()
	native, err := NewJetRulesFactory().NewJetRuleEngine(nil, rbmMainRuleFile, false)
	if err != nil {
		t.Skipf("the C++ engine cannot be built here (needs libjets.so from this commit): %v", err)
	}
	t.Cleanup(func() { native.Release() })
	goEngine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, rbmMainRuleFile, false)
	if err != nil {
		t.Fatalf("building the Go engine: %v", err)
	}
	t.Cleanup(func() { goEngine.Release() })
	return map[string]compute_pipes.JetRuleEngine{"native": native, "go": goEngine}
}

// TestDefect_I6_NativeRetractErases reproduces rules_benchmark I-6 (F25): the native
// adaptor's Retract calls Erase (JetRdfSessionNative.Retract,
// jetrules_native_func.go:338-345), so a triple asserted twice loses every support
// at once on C++.
//
// What it does on Go is narrower than the plan read it. JetRdfSessionGo.Retract
// (jetrules_go_adaptor/jetrules_golang_func.go:254) calls RdfSession.Retract, which
// retracts from the INFERRED graph only (jets/jetrules/rdf/rdf_session.go:117), and
// the adaptor's Insert asserts. So through the adaptor interface, which has no way
// to insert an inferred triple, Go's Retract never removes anything: the asserted
// triple keeps both supports, and Retract returns false.
//
// Reachability: nothing outside tests calls the adaptor's Retract (searched
// 2026-10-20 across jets/ and the four workspaces). The rule engines retract inside
// themselves, below the adaptor. So the difference is real and latent.
//
// Correct behaviour: one decision for both engines (Phase 4). If Retract is a
// reference-counted retract of whatever the session holds, then after one Retract
// the triple is still there on both engines, and after a second it is gone on both.
func TestDefect_I6_NativeRetractErases(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	rbmScratchWorkspace(t)
	type outcome struct {
		retracted1, after1, retracted2, after2 bool
	}
	got := map[string]outcome{}
	for name, engine := range rbmEngines(t) {
		ses, err := engine.NewRdfSession()
		if err != nil {
			t.Fatalf("%s: NewRdfSession: %v", name, err)
		}
		rm := ses.GetResourceManager()
		s, p, o := rm.NewResource("i6_s"), rm.NewResource("i6_p"), rm.NewIntLiteral(6)
		for i := 0; i < 2; i++ { // two supports
			if err := ses.Insert(s, p, o); err != nil {
				t.Fatalf("%s: Insert: %v", name, err)
			}
		}
		var oc outcome
		if oc.retracted1, err = ses.Retract(s, p, o); err != nil {
			t.Fatalf("%s: Retract: %v", name, err)
		}
		oc.after1 = ses.Contains(s, p, o)
		if oc.retracted2, err = ses.Retract(s, p, o); err != nil {
			t.Fatalf("%s: Retract: %v", name, err)
		}
		oc.after2 = ses.Contains(s, p, o)
		got[name] = oc
		ses.Release()
	}
	t.Logf("I-6: insert twice, then Retract, Contains, Retract, Contains: native %+v, go %+v", got["native"], got["go"])
	if want := (outcome{true, false, false, false}); got["native"] != want {
		t.Errorf("native: %+v, want %+v while I-6 is open (the first Retract erases both supports)", got["native"], want)
	}
	if want := (outcome{false, true, false, true}); got["go"] != want {
		t.Errorf("go: %+v, want %+v while I-6 is open (Retract touches only the inferred graph)", got["go"], want)
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

// TestDefect_I12_LiteralRegexEngines reproduces rules_benchmark I-12 (F53) through a
// compiled rule: literal_regex returns capture group 1, or null on no match, on C++
// (RegexVisitor, jets/rete/expr_op_strings.h:30), and the whole match, or an empty
// string on no match, on Go (ApplyRegexOp.Eval,
// jets/jetrules/rete/expr_operator_str_apply_regex.go:33).
//
// Inputs: CM z004's "^(\d\d\d)" on "12345" and "N/A", and walrus_ws's ",\s+(\w+)"
// and "(.*?)," on "DOE, JOHN".
//
// Correct behaviour, as Q-11 decided for Phase 4: C++'s on both engines, so the Go
// column below becomes the native one.
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
	got := map[string][]string{}
	for name, engine := range rbmEngines(t) {
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
			got[name] = append(got[name], describe(ses.GetObject(rm.NewResource(p.subject), rm.NewResource(p.outProp))))
		}
		rete.Release()
		ses.Release()
	}
	today := map[string][]string{
		"native": {"123", "<absent>", "JOHN", "DOE"},
		"go":     {"123", "", ", JOHN", "DOE,"},
	}
	for i, p := range probes {
		t.Logf("I-12: %q literal_regex -> %s: native %q, go %q", p.in, p.outProp, got["native"][i], got["go"][i])
	}
	for _, name := range []string{"native", "go"} {
		if !slices.Equal(got[name], today[name]) {
			t.Errorf("%s: %q, want %q while I-12 is open", name, got[name], today[name])
		}
	}
}
