package jetrules_native_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// RdfNode.Equals is node identity, the same RDF node, which is what Contains and
// FindSPO use: the same resource, or a literal of the same type and value, with no
// coercion. TestNativeRdfNode (rdf_node_test.go) checks a node read twice; this is
// the identity table the Go adaptor's twin checked (rules_unit_test_harness I-11),
// moved onto the C++ engine with the Go engine's retirement (rules_benchmark
// Phase 6, 2026-10-22). And NodeKey, the identity extraction groups a subject's
// triples by (rules_benchmark G11), is equal exactly when Equals is true.
func TestNativeRdfNodeEqualsIsIdentity(t *testing.T) {
	if inScratchTestWs(t) {
		return
	}
	_, rm := newSession(t)
	r := rm.NewResource("r")
	if !r.Equals(r) || !r.Equals(rm.NewResource("r")) {
		t.Errorf("a resource is not Equal to itself")
	}
	if r.Equals(rm.NewResource("s")) {
		t.Errorf("two resources are Equal")
	}
	i2 := rm.NewIntLiteral(2)
	if !i2.Equals(rm.NewIntLiteral(2)) {
		t.Errorf("int 2 made twice is not Equal")
	}
	// no coercion: EQ, which the rules' == uses, says int 2 == double 2
	for name, other := range map[string]compute_pipes.RdfNode{
		"uint 2":   rm.NewUIntLiteral(2),
		"double 2": rm.NewDoubleLiteral(2),
		"text 2":   rm.NewTextLiteral("2"),
		"int 3":    rm.NewIntLiteral(3),
	} {
		if i2.Equals(other) {
			t.Errorf("int 2 Equals %s", name)
		}
	}
	d := rm.NewDateLiteral("2026-10-11")
	if !d.Equals(rm.NewDateLiteral("2026-10-11")) {
		t.Errorf("one date made twice is not Equal")
	}
	if i2.Equals(nil) {
		t.Errorf("Equals(nil) is true")
	}

	nodes := []compute_pipes.RdfNode{
		rm.NewResource("r"), rm.NewResource("r"), rm.NewResource("s"),
		rm.NewIntLiteral(2), rm.NewIntLiteral(2), rm.NewDoubleLiteral(2), rm.NewTextLiteral("2"),
	}
	for _, a := range nodes {
		for _, b := range nodes {
			sameKey := a.(compute_pipes.NodeKeyer).NodeKey() == b.(compute_pipes.NodeKeyer).NodeKey()
			if sameKey != a.Equals(b) {
				t.Errorf("%s (%s) against %s (%s): same key %v, Equals %v", a, a.Type(), b, b.Type(), sameKey, a.Equals(b))
			}
		}
	}
	if k := (&RdfNodeNative{}).NodeKey(); k != nil {
		t.Errorf("a node with no rdf node has key %v, want nil", k)
	}
}
