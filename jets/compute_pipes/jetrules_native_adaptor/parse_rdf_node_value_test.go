package jetrules_native_adaptor

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// ParseRdfNodeValue is the typed-value notation rule_config and the rules unit
// test harness read ({"value": "...", "type": "..."}). These tests ran against the
// Go engine's resource manager until its retirement (rules_benchmark Phase 6,
// 2026-10-22) and run against the C++ engine's now, in a child process over a
// compiled copy of test_ws (newSession, briefing_projection_test.go). Node
// identity is Equals, the identity Contains and FindSPO use.

// TestParseRdfNodeValueUint: "uint" makes an unsigned literal, distinct from the
// signed literal of the same number.
func TestParseRdfNodeValueUint(t *testing.T) {
	if inScratchTestWs(t) {
		return
	}
	_, rm := newSession(t)
	node, err := compute_pipes.ParseRdfNodeValue(rm, "42", "uint")
	if err != nil {
		t.Fatalf("uint: unexpected error: %v", err)
	}
	if v, ok := node.Value().(uint); !ok || v != 42 {
		t.Fatalf("uint: got %T %v, want uint 42", node.Value(), node.Value())
	}
	// The engines name the type differently (Go "uint", C++ "uint32"); the property
	// is that it is the unsigned literal NewUIntLiteral makes.
	if want := rm.NewUIntLiteral(42).Type(); node.Type() != want {
		t.Errorf("uint: type %q, want %q", node.Type(), want)
	}
	if !node.Equals(rm.NewUIntLiteral(42)) {
		t.Errorf("uint: not the node NewUIntLiteral(42) makes")
	}
	signed, err := compute_pipes.ParseRdfNodeValue(rm, "42", "int")
	if err != nil {
		t.Fatalf("int: unexpected error: %v", err)
	}
	if signed.Equals(node) {
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
// and maps to the same unsigned literal as "uint" for a value that fits in 32 bits.
//
// The native adaptor made every unsigned literal through the bridge's 32-bit
// create_uint (C.uint), so 18446744073709551615 came back as 4294967295 of type
// uint32 (measured 2026-10-22, rules_benchmark Phase 6, I-150). Since Phase 7 a
// value outside 32 bits is a uint64 literal and comes back whole (Q-86); one that
// fits is a uint32 literal, as before.
func TestParseRdfNodeValueUlong(t *testing.T) {
	if inScratchTestWs(t) {
		return
	}
	_, rm := newSession(t)
	node, err := compute_pipes.ParseRdfNodeValue(rm, "4294967295", "ulong")
	if err != nil {
		t.Fatalf("ulong: unexpected error: %v", err)
	}
	if v, ok := node.Value().(uint); !ok || v != 4294967295 {
		t.Fatalf("ulong: got %T %v, want uint 4294967295", node.Value(), node.Value())
	}
	if want := rm.NewUIntLiteral(1).Type(); node.Type() != want {
		t.Errorf("ulong: type %q, want %q, the unsigned literal's", node.Type(), want)
	}
	same, err := compute_pipes.ParseRdfNodeValue(rm, "7", "ulong")
	if err != nil {
		t.Fatalf("ulong: unexpected error: %v", err)
	}
	other, _ := compute_pipes.ParseRdfNodeValue(rm, "7", "uint")
	if !same.Equals(other) {
		t.Errorf("ulong 7 and uint 7 differ; both are the one unsigned literal")
	}
	if _, err := compute_pipes.ParseRdfNodeValue(rm, "-7", "ulong"); err == nil {
		t.Errorf("ulong: \"-7\" accepted; want an error")
	}
	// The adaptor truncated a value outside 32 bits to uint 4294967295 until
	// I-150; it is now a 64-bit literal, whole.
	big, err := compute_pipes.ParseRdfNodeValue(rm, "18446744073709551615", "ulong")
	if err != nil {
		t.Fatalf("ulong max: unexpected error: %v", err)
	}
	if v, ok := big.Value().(uint); !ok || v != 18446744073709551615 || big.Type() != "uint64" {
		t.Errorf("ulong max: got %s %T %v, want uint64 18446744073709551615", big.Type(), big.Value(), big.Value())
	}
}

// TestParseRdfNodeValueBool: rule configuration reads a bool as the engines do
// (rules_unit_test_harness I-45): the accepted spellings, trimmed and in any
// case; empty is null; anything else is an error, which fails the pipeline at
// start. It read the first letter until 2026-10-11, so "tomato" was true.
func TestParseRdfNodeValueBool(t *testing.T) {
	if inScratchTestWs(t) {
		return
	}
	_, rm := newSession(t)
	for value, want := range map[string]int{"true": 1, " Yes ": 1, "y": 1, "1": 1, "FALSE": 0, "no": 0, "n": 0, "0": 0} {
		node, err := compute_pipes.ParseRdfNodeValue(rm, value, "bool")
		if err != nil || !node.Equals(rm.NewIntLiteral(want)) {
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
