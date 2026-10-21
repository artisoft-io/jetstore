package jetrules_go_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

var (
	_ compute_pipes.NodeKeyer      = (*RdfNodeGo)(nil)
	_ compute_pipes.PredicateKeyer = (*TripleIteratorGo)(nil)
)

// NodeKey is the identity extraction groups a subject's triples by (rules_benchmark
// G11): equal keys exactly when Equals is true. The native twin is in
// jetrules_native_adaptor/node_key_test.go.
func TestNodeKeyIsEquals(t *testing.T) {
	rm := newTestResourceManager()
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
	if k := (&RdfNodeGo{}).NodeKey(); k != nil {
		t.Errorf("a node with no rdf node has key %v, want nil", k)
	}
}

// PredicateKey is NodeKey of GetPredicate for every triple of a FindS.
func TestPredicateKeyIsNodeKeyOfGetPredicate(t *testing.T) {
	s, rm := newSession(t)
	for _, p := range []string{"p1", "p2", "p3"} {
		insert(t, s, "subject", p, rm.NewTextLiteral("v"), rm)
	}
	ses := &JetRdfSessionGo{rdfSession: s}
	itor := ses.FindS(ses.GetResourceManager().NewResource("subject"))
	n := 0
	for !itor.IsEnd() {
		pk := itor.(compute_pipes.PredicateKeyer).PredicateKey()
		if want := itor.GetPredicate().(compute_pipes.NodeKeyer).NodeKey(); pk == nil || pk != want {
			t.Errorf("PredicateKey %v, NodeKey of GetPredicate %v", pk, want)
		}
		n++
		itor.Next()
	}
	itor.Release()
	if n != 3 {
		t.Errorf("%d triples, want 3", n)
	}
}
