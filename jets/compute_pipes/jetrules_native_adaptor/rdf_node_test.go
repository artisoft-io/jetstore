package jetrules_native_adaptor

// THE NATIVE RdfNode AGREES WITH THE GO ONE (rules_unit_test_harness I-11, I-12, I-15).
//
// Equals compared two *bridge.Resource wrappers, and every read allocates one, so a
// node was not Equal to itself read twice (I-11). Value had no case for blank nodes
// or unsigned literals and returned nil (I-12). And a new meta resource made after
// the first RDF session opened threw across cgo and aborted the process (I-15).
//
// Needs libjets.so with get_ulong_literal: set CGO_CFLAGS, CGO_LDFLAGS and
// LD_LIBRARY_PATH to a build of this commit. Runs in a child process over a compiled
// copy of jets/jetrules/test_ws, as engine_mode_test.go does. Run it with -count=1.

import (
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

func TestNativeRdfNode(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "i11",
		workspace.SkipAssetValidation("native RdfNode test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, "jet_rules/test_join_values_main.jr", false)
	if err != nil {
		t.Fatalf("building the native engine: %v", err)
	}
	defer engine.Release()

	meta := engine.GetMetaResourceManager()
	madeBefore := meta.NewResource("i15_made_before_the_session")

	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	defer ses.Release()
	rm := ses.GetResourceManager()
	s, p := rm.NewResource("i11_s"), rm.NewResource("i11_p")
	o := rm.NewUIntLiteral(7)
	if err := ses.Insert(s, p, o); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// I-11: one node read twice is Equal, to itself and to what was inserted
	obj1, obj2 := ses.GetObject(s, p), ses.GetObject(s, p)
	if !obj1.Equals(obj2) || !obj1.Equals(o) {
		t.Errorf("the object read twice: Equals false")
	}
	if s.Equals(p) || obj1.Equals(rm.NewIntLiteral(7)) {
		t.Errorf("different nodes are Equal")
	}
	i2 := rm.NewIntLiteral(2)
	if !i2.Equals(rm.NewIntLiteral(2)) || i2.Equals(rm.NewDoubleLiteral(2)) {
		t.Errorf("int 2: Equal to another int 2 %v, to double 2 %v (want true, false)",
			i2.Equals(rm.NewIntLiteral(2)), i2.Equals(rm.NewDoubleLiteral(2)))
	}
	if i2.Equals(nil) {
		t.Errorf("Equals(nil) is true")
	}

	// I-12: unsigned literals and blank nodes have values, as on the Go engine
	if v, ok := obj1.Value().(uint); !ok || v != 7 {
		t.Errorf("uint literal Value = %v (%T), want uint 7", obj1.Value(), obj1.Value())
	}
	bn := rm.CreateBNode(3)
	if v, ok := bn.Value().(rdf.BlankNode); !ok || v.Key != 3 {
		t.Errorf("blank node Value = %v (%T), want rdf.BlankNode{Key: 3}", bn.Value(), bn.Value())
	}
	if got := compute_pipes.GetRdfNodeValue(bn); got != "BN3" {
		t.Errorf("GetRdfNodeValue(blank node) = %v, want BN3 as on the Go engine", got)
	}

	// I-15: the meta graph is locked now; what it holds comes back, a new item is
	// refused, and the process is still here
	if again := meta.NewResource("i15_made_before_the_session"); !again.Equals(madeBefore) {
		t.Errorf("an existing meta resource after the lock is not the same node")
	}
	if v := meta.NewResource("i15_made_after_the_session").Value(); v != nil {
		t.Errorf("a new meta resource after the lock: Value %v, want nil", v)
	}
}
