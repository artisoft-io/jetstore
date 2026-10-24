package jetrules_native_adaptor

// THE RULE OPTIMIZER COMPUTES WHAT THE AUTHORED ORDER COMPUTES (rules_benchmark Phase 8,
// task BS.1).
//
// The compiler reorders a rule's antecedents unless the rule sets o=false
// (OptimizeJetruleNode, jets/compilerv2/compiler/jet_rule_listener_optimization.go).
// Each rule below has a twin with o=false and its own output property, and both run on
// one graph: whatever the optimizer does to the order, the two must infer the same.
// jets/compilerv2/compiler/jet_rule_listener_optimization_pinned_test.go pins the order
// itself; this pins what the C++ engine computes from it.
//
// Needs the cgo environment, as engine_mode_test.go does, and runs in a child process
// over a compiled copy of jets/jetrules/test_ws (runInCompiledTestWs). Run with -count=1.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const optMainRuleFile = "jet_rules/rbm_optimizer_main.jr"

const optModel = `# Data model for the rule optimizer test (optimizer_rbm_test.go).
class rbm:OptItem {
  $base_classes = [jets:Entity],
  $data_properties = [
    rbm:amount                as int,
    rbm:limit                 as int,
    rbm:openOn                as int,
    rbm:openOff               as int,
    rbm:overOn                as int,
    rbm:overOff               as int
  ],
  $as_table = true
};
`

// Each pair: the rule as the optimizer orders it, and its twin as authored. OPT_NEG has a
// negated term, and OPT_FLT a filter reading three bound variables.
const optRules = `# Main rule file for the rule optimizer test (optimizer_rbm_test.go).
import "data_model/jets_model.jr"
import "data_model/rbm_optimizer_model.jr"

resource rbm:Closed = "rbm:Closed";

[OPT_NEG]:
  (?c rdf:type rbm:OptItem).(?c rbm:amount ?v).not(?c rdf:type rbm:Closed)
  ->
  (?c rbm:openOn ?v)
;
[OPT_NEG_AUTHORED, o=false]:
  (?c rdf:type rbm:OptItem).(?c rbm:amount ?v).not(?c rdf:type rbm:Closed)
  ->
  (?c rbm:openOff ?v)
;

[OPT_FLT]:
  (?c rbm:amount ?v).(?c rbm:limit ?l).[?v > ?l].(?c rdf:type rbm:OptItem)
  ->
  (?c rbm:overOn ?v)
;
[OPT_FLT_AUTHORED, o=false]:
  (?c rbm:amount ?v).(?c rbm:limit ?l).[?v > ?l].(?c rdf:type rbm:OptItem)
  ->
  (?c rbm:overOff ?v)
;
`

func TestOptimizerComputesWhatTheAuthoredOrderComputes(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	wsDir := filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws")
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(wsDir, rel), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("data_model/rbm_optimizer_model.jr", optModel)
	write(optMainRuleFile, optRules)
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["rule_sets"] = append(ctl["rule_sets"].([]any), optMainRuleFile)
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	write("workspace_control.json", string(raw))
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "rbm-opt",
		workspace.SkipAssetValidation("rules_benchmark optimizer test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, optMainRuleFile, false)
	if err != nil {
		t.Fatalf("building the C++ engine: %v", err)
	}
	defer engine.Release()
	withVertexCap(t, engine)

	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	defer ses.Release()
	rm := ses.GetResourceManager()
	r := rm.NewResource
	insert := func(s, p string, o compute_pipes.RdfNode) {
		t.Helper()
		if err := ses.Insert(r(s), r(p), o); err != nil {
			t.Fatalf("Insert(%s %s %v): %v", s, p, o, err)
		}
	}
	// c1 is open, c2 is closed: OPT_NEG infers c1's amount and not c2's
	insert("c1", "rdf:type", r("rbm:OptItem"))
	insert("c1", "rbm:amount", rm.NewIntLiteral(10))
	insert("c2", "rdf:type", r("rbm:OptItem"))
	insert("c2", "rbm:amount", rm.NewIntLiteral(20))
	insert("c2", "rdf:type", r("rbm:Closed"))
	// c4 is over its limit, c5 is not
	for _, c := range []struct {
		s             string
		amount, limit int
	}{{"c4", 50, 40}, {"c5", 30, 40}} {
		insert(c.s, "rdf:type", r("rbm:OptItem"))
		insert(c.s, "rbm:amount", rm.NewIntLiteral(c.amount))
		insert(c.s, "rbm:limit", rm.NewIntLiteral(c.limit))
	}

	rete, err := ses.NewReteSession(optMainRuleFile)
	if err != nil {
		t.Fatalf("NewReteSession: %v", err)
	}
	defer rete.Release()
	if err := rete.ExecuteRules(); err != nil {
		t.Fatalf("ExecuteRules: %v", err)
	}

	// GetObject of a pair with no object gives a node whose IsNil dereferences nil, so
	// ContainsSP answers first
	object := func(s, p string) string {
		if !ses.ContainsSP(r(s), r(p)) {
			return "<absent>"
		}
		return describe(ses.GetObject(r(s), r(p)))
	}
	for _, pair := range []struct{ on, off string }{
		{"rbm:openOn", "rbm:openOff"}, {"rbm:overOn", "rbm:overOff"},
	} {
		for _, s := range []string{"c1", "c2", "c4", "c5"} {
			if on, off := object(s, pair.on), object(s, pair.off); on != off {
				t.Errorf("(%s %s ?o) is %s with the optimizer, (%s %s ?o) %s as authored", s, pair.on, on, s, pair.off, off)
			}
		}
	}
	// and the authored rules computed what they should, so the comparison is not of two
	// empty sets
	for _, want := range []struct{ s, p, o string }{
		{"c1", "rbm:openOff", "10"}, {"c4", "rbm:openOff", "50"}, {"c4", "rbm:overOff", "50"},
	} {
		if got := object(want.s, want.p); got != want.o {
			t.Errorf("(%s %s ?o) is %s, want %s", want.s, want.p, got, want.o)
		}
	}
	for _, none := range []struct{ s, p string }{{"c2", "rbm:openOff"}, {"c5", "rbm:overOff"}} {
		if got := object(none.s, none.p); got != "<absent>" {
			t.Errorf("(%s %s ?o) is %s, want none", none.s, none.p, got)
		}
	}
}
