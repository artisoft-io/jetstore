package jetrules_native_adaptor

import (
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

var _ compute_pipes.NodeKeyer = (*RdfNodeNative)(nil)

// NodeKey is the identity extraction groups a subject's triples by (rules_benchmark
// G11): equal keys exactly when Equals is true, including for a predicate read back
// from an iterator, which is a new wrapper on every read. Same environment as
// rdf_node_test.go.
func TestNativeNodeKeyIsEquals(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "g11",
		workspace.SkipAssetValidation("native NodeKey test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, "jet_rules/test_join_values_main.jr", false)
	if err != nil {
		t.Fatalf("building the native engine: %v", err)
	}
	defer engine.Release()
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	defer ses.Release()
	rm := ses.GetResourceManager()
	s, p, q := rm.NewResource("g11_s"), rm.NewResource("g11_p"), rm.NewResource("g11_q")
	for _, pred := range []compute_pipes.RdfNode{p, q} {
		if err := ses.Insert(s, pred, rm.NewIntLiteral(1)); err != nil {
			t.Fatal(err)
		}
	}
	seen := 0
	itor := ses.FindS(s)
	for !itor.IsEnd() {
		got := itor.GetPredicate()
		for _, want := range []compute_pipes.RdfNode{p, q} {
			sameKey := got.(compute_pipes.NodeKeyer).NodeKey() == want.(compute_pipes.NodeKeyer).NodeKey()
			if sameKey != got.Equals(want) {
				t.Errorf("predicate %s against %s: same key %v, Equals %v", got, want, sameKey, got.Equals(want))
			}
			if sameKey {
				seen++
			}
		}
		itor.Next()
	}
	itor.Release()
	if seen != 2 {
		t.Errorf("%d predicates matched by key, want 2", seen)
	}
	if k := rm.NewResource("g11_p").(compute_pipes.NodeKeyer).NodeKey(); k == nil || k != p.(compute_pipes.NodeKeyer).NodeKey() {
		t.Errorf("one resource made twice has keys %v and %v", k, p.(compute_pipes.NodeKeyer).NodeKey())
	}
}
