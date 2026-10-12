package jetrules_go_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ParseRdfNodeValue is the typed-value notation rule_config and the rules unit
// test harness read ({"value": "...", "type": "..."}). These tests run it against
// the Go engine's resource manager, which is pure Go and needs no workspace.

func newTestResourceManager() compute_pipes.JetResourceManager {
	return &JetResourceManagerGo{rm: rdf.NewResourceManager(nil)}
}

// goNode compares interned literals by identity, which is what RdfNodeGo.Equals
// does since rules_unit_test_harness I-11 (rdf_node_equals_test.go).
func goNode(n compute_pipes.RdfNode) *rdf.Node {
	return n.Hdle().(*RdfNodeGo).node
}

// TestParseRdfNodeValueUint: "uint" makes an unsigned literal, distinct from the
// signed literal of the same number (literals are interned by Go value).
func TestParseRdfNodeValueUint(t *testing.T) {
	rm := newTestResourceManager()
	node, err := compute_pipes.ParseRdfNodeValue(rm, "42", "uint")
	if err != nil {
		t.Fatalf("uint: unexpected error: %v", err)
	}
	if v, ok := node.Value().(uint); !ok || v != 42 {
		t.Fatalf("uint: got %T %v, want uint 42", node.Value(), node.Value())
	}
	if node.Type() != "uint" {
		t.Errorf("uint: type %q, want \"uint\"", node.Type())
	}
	if goNode(node) != goNode(rm.NewUIntLiteral(42)) {
		t.Errorf("uint: not the interned NewUIntLiteral(42)")
	}
	signed, err := compute_pipes.ParseRdfNodeValue(rm, "42", "int")
	if err != nil {
		t.Fatalf("int: unexpected error: %v", err)
	}
	if goNode(signed) == goNode(node) {
		t.Errorf("uint 42 and int 42 are the same node; they must be distinct literals")
	}
	if _, err := compute_pipes.ParseRdfNodeValue(rm, "-1", "uint"); err == nil {
		t.Errorf("uint: \"-1\" accepted; want an error")
	}
	if _, err := compute_pipes.ParseRdfNodeValue(rm, "abc", "uint"); err == nil {
		t.Errorf("uint: \"abc\" accepted; want an error")
	}
}

// TestParseRdfNodeValueUlong: "ulong" is the rule language's 64-bit unsigned type
// and maps to the same unsigned literal; a value above the int64 range is kept.
func TestParseRdfNodeValueUlong(t *testing.T) {
	rm := newTestResourceManager()
	node, err := compute_pipes.ParseRdfNodeValue(rm, "18446744073709551615", "ulong")
	if err != nil {
		t.Fatalf("ulong: unexpected error: %v", err)
	}
	if v, ok := node.Value().(uint); !ok || v != ^uint(0) {
		t.Fatalf("ulong: got %T %v, want uint %d", node.Value(), node.Value(), ^uint(0))
	}
	if node.Type() != "uint" {
		t.Errorf("ulong: type %q, want \"uint\"", node.Type())
	}
	same, err := compute_pipes.ParseRdfNodeValue(rm, "7", "ulong")
	if err != nil {
		t.Fatalf("ulong: unexpected error: %v", err)
	}
	other, _ := compute_pipes.ParseRdfNodeValue(rm, "7", "uint")
	if goNode(same) != goNode(other) {
		t.Errorf("ulong 7 and uint 7 differ; both are the one unsigned literal")
	}
	if _, err := compute_pipes.ParseRdfNodeValue(rm, "-7", "ulong"); err == nil {
		t.Errorf("ulong: \"-7\" accepted; want an error")
	}
}

// TestParseRdfNodeValueBool: rule configuration reads a bool as the engines do
// (rules_unit_test_harness I-45): the accepted spellings, trimmed and in any
// case; empty is null; anything else is an error, which fails the pipeline at
// start. It read the first letter until 2026-10-11, so "tomato" was true.
func TestParseRdfNodeValueBool(t *testing.T) {
	rm := newTestResourceManager()
	for value, want := range map[string]int{"true": 1, " Yes ": 1, "y": 1, "1": 1, "FALSE": 0, "no": 0, "n": 0, "0": 0} {
		node, err := compute_pipes.ParseRdfNodeValue(rm, value, "bool")
		if err != nil || goNode(node) != goNode(rm.NewIntLiteral(want)) {
			t.Errorf("bool %q: got %v, %v; want int %d", value, node, err, want)
		}
	}
	node, err := compute_pipes.ParseRdfNodeValue(rm, "", "bool")
	if err != nil || node.Type() != rm.RdfNull().Type() {
		t.Errorf("bool \"\": got %v, %v; want null", node, err)
	}
	for _, value := range []string{"tomato", "maybe", "nope", "2"} {
		if _, err := compute_pipes.ParseRdfNodeValue(rm, value, "bool"); err == nil {
			t.Errorf("bool %q accepted; want an error", value)
		}
	}
}
