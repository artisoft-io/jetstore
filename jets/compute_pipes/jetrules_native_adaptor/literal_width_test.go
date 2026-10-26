package jetrules_native_adaptor

// INTEGER LITERALS KEEP 64 BITS (rules_benchmark I-150, Q-86).
//
// NewIntLiteral and NewUIntLiteral went through the bridge's 32-bit calls, so a
// value outside 32 bits was truncated silently. A value that fits is still a
// 32-bit literal, since C++ matches int32 and int64 as different resources; one
// that does not is a 64-bit literal and reads back whole. Both resource managers
// are checked, a session's and the meta graph's, which rule_config writes to:
// each literal's type and value, and its triple found from a session.
//
// Runs in a child process over a compiled copy of jets/jetrules/test_ws, as
// engine_mode_test.go does. Needs the cgo environment of
// jets/cmds/rules_unit_test/README.md; run it with -count=1.

import (
	"math"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

func TestIntegerLiteralWidth(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "i150",
		workspace.SkipAssetValidation("literal width test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, "jet_rules/test_join_values_main.jr", false)
	if err != nil {
		t.Fatalf("building the native engine: %v", err)
	}
	defer engine.Release()
	ints := []struct {
		v        int
		wantType string
	}{
		{math.MaxInt32, "int32"},     // 2^31-1
		{math.MinInt32, "int32"},     // -2^31
		{math.MaxInt32 + 1, "int64"}, // 2^31
		{1 << 32, "int64"},           // 2^32
		{math.MinInt32 - 1, "int64"}, // -2^31-1
		{math.MaxInt64, "int64"},
		{5, "int32"},
	}
	uints := []struct {
		v        uint
		wantType string
	}{
		{math.MaxUint32, "uint32"},     // 2^32-1
		{math.MaxUint32 + 1, "uint64"}, // 2^32
		{math.MaxUint64, "uint64"},
		{7, "uint32"},
	}
	check := func(name string, rm compute_pipes.JetResourceManager) {
		for _, c := range ints {
			n := rm.NewIntLiteral(c.v)
			if n.Type() != c.wantType {
				t.Errorf("%s NewIntLiteral(%d): type %s, want %s", name, c.v, n.Type(), c.wantType)
			}
			if got, ok := n.Value().(int); !ok || got != c.v {
				t.Errorf("%s NewIntLiteral(%d): value %v (%T)", name, c.v, n.Value(), n.Value())
			}
		}
		for _, c := range uints {
			n := rm.NewUIntLiteral(c.v)
			if n.Type() != c.wantType {
				t.Errorf("%s NewUIntLiteral(%d): type %s, want %s", name, c.v, n.Type(), c.wantType)
			}
			if got, ok := n.Value().(uint); !ok || got != c.v {
				t.Errorf("%s NewUIntLiteral(%d): value %v (%T)", name, c.v, n.Value(), n.Value())
			}
		}
	}

	// The meta graph's manager before any session opens: a meta literal made
	// after one has opened is not made at all, whatever its width (I-15's
	// territory, and not this test's).
	meta := engine.GetMetaResourceManager()
	check("meta", meta)
	// And into the meta graph, as rule_config writes there, to be found from a
	// session below.
	ms, mp := meta.NewResource("i150_meta_s"), meta.NewResource("i150_meta_p")
	for _, c := range ints {
		if err := engine.Insert(ms, mp, meta.NewIntLiteral(c.v)); err != nil {
			t.Fatalf("meta Insert %d: %v", c.v, err)
		}
	}
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	defer ses.Release()
	check("session", ses.GetResourceManager())

	// Through the graph: inserted, through either manager, and found, the object
	// keeps its value and its width.
	rm := ses.GetResourceManager()
	for _, c := range ints {
		if !ses.Contains(rm.NewResource("i150_meta_s"), rm.NewResource("i150_meta_p"), rm.NewIntLiteral(c.v)) {
			t.Errorf("a session does not find the meta graph's (s p %d) as %s", c.v, c.wantType)
		}
	}
	s, p := rm.NewResource("i150_s"), rm.NewResource("i150_p")
	for _, c := range ints {
		o := rm.NewIntLiteral(c.v)
		if err := ses.Insert(s, p, o); err != nil {
			t.Fatalf("Insert %d: %v", c.v, err)
		}
		if !ses.Contains(s, p, o) {
			t.Errorf("the graph does not contain (s p %d) as %s", c.v, c.wantType)
		}
	}
	if ses.Contains(s, p, rm.NewIntLiteral(3)) {
		t.Errorf("the graph contains a literal never inserted")
	}

	// A 64-bit literal is not the 32-bit one of the same digits: why only a
	// value that does not fit is widened. The int64 5 is made through the bridge
	// directly, since NewIntLiteral makes an int32 for a value that fits.
	native := rm.(*JetResourceManagerNative)
	long5, err := native.rdfs.NewLongLiteral(5)
	if err != nil {
		t.Fatal(err)
	}
	wide5, narrow5 := &RdfNodeNative{node: long5}, rm.NewIntLiteral(5)
	if wide5.Type() != "int64" || narrow5.Type() != "int32" {
		t.Fatalf("5 made as %s and %s, want int64 and int32", wide5.Type(), narrow5.Type())
	}
	pw, pn := rm.NewResource("i150_wide"), rm.NewResource("i150_narrow")
	if err := ses.Insert(s, pw, wide5); err != nil {
		t.Fatal(err)
	}
	if err := ses.Insert(s, pn, narrow5); err != nil {
		t.Fatal(err)
	}
	if !ses.Contains(s, pw, wide5) || ses.Contains(s, pw, narrow5) {
		t.Errorf("the int64 5: found as itself %v, as the int32 5 %v; want true, false",
			ses.Contains(s, pw, wide5), ses.Contains(s, pw, narrow5))
	}
	if !ses.Contains(s, pn, narrow5) || ses.Contains(s, pn, wide5) {
		t.Errorf("the int32 5: found as itself %v, as the int64 5 %v; want true, false",
			ses.Contains(s, pn, narrow5), ses.Contains(s, pn, wide5))
	}
}
