package jetrules_go_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// RdfNode.Equals is node identity, the same RDF node, which is what Contains and
// FindSPO use. It compared EQ's result with rdf.TRUE() by pointer, and TRUE()
// makes a new node on every call, so it was false even for a node and itself
// (rules_unit_test_harness I-11). The native adaptor's twin is in
// jetrules_native_adaptor/rdf_node_test.go.
func TestRdfNodeEquals(t *testing.T) {
	rm := newTestResourceManager()
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
}
