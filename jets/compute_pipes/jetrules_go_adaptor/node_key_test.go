package jetrules_go_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

var _ compute_pipes.NodeKeyer = (*RdfNodeGo)(nil)

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
