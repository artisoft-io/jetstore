package jetrules_native_adaptor

import "testing"

// GetObject wraps the bridge's nil when the session has no object; IsNil,
// like Type and Value, reads that as null rather than dereferencing it
// (rules_benchmark I-221).
func TestIsNilOfANodeHoldingNothing(t *testing.T) {
	var none *RdfNodeNative
	for _, n := range []*RdfNodeNative{none, {}} {
		if !n.IsNil() {
			t.Errorf("%#v: IsNil false", n)
		}
		if n.Type() != "null" || n.Value() != nil {
			t.Errorf("%#v: type %q value %v", n, n.Type(), n.Value())
		}
	}
}
