package jetrules_native_adaptor

// A NODE THE ENGINE DID NOT CREATE CARRIES WHY.
//
// The resource manager's New* discarded the bridge's error and returned a node
// holding nothing, with nothing said why. A meta literal or
// resource made after the first RDF session opened is one: the meta graph is
// locked, and only one it already holds is returned. Now the node carries a
// NodeCreationError naming the value: Err returns it, ParseRdfNodeValue and
// NewRdfNode return it, Insert returns it, Contains is false and a find matches
// nothing.
//
// Runs in a child process over a compiled copy of jets/jetrules/test_ws, as
// engine_mode_test.go does. Needs the cgo environment of
// docs/src/content/docs/developer/testing/go-and-cpp.md; run it with -count=1.

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

func TestANodeNotCreatedCarriesTheError(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "nce1",
		workspace.SkipAssetValidation("node creation test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, "jet_rules/test_join_values_main.jr", false)
	if err != nil {
		t.Fatalf("building the native engine: %v", err)
	}
	defer engine.Release()
	meta := engine.GetMetaResourceManager()
	// made before any session: created, and carries no error
	held := meta.NewResource("nce_meta_held")
	if err := compute_pipes.NodeErr(held); err != nil || held.IsNil() {
		t.Fatalf("a meta resource made before any session: err %v, nil %v", err, held.IsNil())
	}

	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatalf("NewRdfSession: %v", err)
	}
	defer ses.Release()

	// After a session opened, a new meta literal or resource is not created.
	failed := map[string]compute_pipes.RdfNode{
		`text literal "nce never seen"`: meta.NewTextLiteral("nce never seen"),
		`resource "nce_meta_new"`:       meta.NewResource("nce_meta_new"),
		"int literal 918273":            meta.NewIntLiteral(918273),
		"double literal 9.5e+07":        meta.NewDoubleLiteral(9.5e7),
		`date literal "1999-07-04"`:     meta.NewDateLiteral("1999-07-04"),
	}
	for want, n := range failed {
		err := compute_pipes.NodeErr(n)
		var cerr *NodeCreationError
		if !errors.As(err, &cerr) {
			t.Errorf("%s: NodeErr %v, want a *NodeCreationError", want, err)
			continue
		}
		if !strings.Contains(err.Error(), "did not create the "+want) {
			t.Errorf("error %q does not name %s", err, want)
		}
		if !n.IsNil() || n.Type() != "null" || n.Value() != nil || n.String() != "" {
			t.Errorf("%s: a node not created reads as %s %v %q", want, n.Type(), n.Value(), n.String())
		}
	}
	// one it already holds is still returned
	if again := meta.NewResource("nce_meta_held"); compute_pipes.NodeErr(again) != nil || !again.Equals(held) {
		t.Errorf("a meta resource the graph holds is not returned after a session opened: %v", compute_pipes.NodeErr(again))
	}

	// The error reaches whoever uses the node.
	rm := ses.GetResourceManager()
	s, p := rm.NewResource("nce_s"), rm.NewResource("nce_p")
	if err := ses.Insert(s, p, rm.NewTextLiteral("x")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	bad := failed[`resource "nce_meta_new"`]
	if err := ses.Insert(s, bad, rm.NewTextLiteral("y")); err == nil || !strings.Contains(err.Error(), "nce_meta_new") {
		t.Errorf("Insert with a node not created: %v, want its error", err)
	}
	if err := engine.Insert(held, held, bad); err == nil || !strings.Contains(err.Error(), "nce_meta_new") {
		t.Errorf("meta Insert with a node not created: %v, want its error", err)
	}
	if _, err := ses.Erase(s, bad, nil); err == nil {
		t.Errorf("Erase with a node not created: no error")
	}
	if ses.Contains(s, bad, rm.NewTextLiteral("x")) || ses.ContainsSP(s, bad) {
		t.Errorf("Contains with a node not created is true")
	}
	if ses.GetObject(s, bad) != nil {
		t.Errorf("GetObject with a node not created is not nil")
	}
	// A find matches nothing, and returns an empty iterator rather than nil
	for name, it := range map[string]compute_pipes.TripleIterator{
		"FindSP":  ses.FindSP(s, bad),
		"FindSPO": ses.FindSPO(nil, bad, nil),
		"FindS":   ses.FindS(bad),
	} {
		if it == nil {
			t.Errorf("%s: nil iterator", name)
			continue
		}
		if !it.IsEnd() || it.Next() {
			t.Errorf("%s with a node not created matches %v", name, it.GetObject().Value())
		}
		it.Release()
	}
	// and the find of a node it created still matches
	it := ses.FindSP(s, p)
	if it == nil || it.IsEnd() {
		t.Fatalf("FindSP(s, p) matches nothing")
	}
	it.Release()

	// ParseRdfNodeValue and NewRdfNode return it, as rule_config and a row's values go through them.
	if _, err := compute_pipes.ParseRdfNodeValue(meta, "nce config text", "text"); err == nil ||
		!strings.Contains(err.Error(), `"nce config text"`) {
		t.Errorf("ParseRdfNodeValue of a meta literal not created: %v", err)
	}
	if _, err := compute_pipes.NewRdfNode("nce row text", meta); err == nil ||
		!strings.Contains(err.Error(), `"nce row text"`) {
		t.Errorf("NewRdfNode of a meta literal not created: %v", err)
	}
	if n, err := compute_pipes.NewRdfNode("nce row text", rm); err != nil || n.Value() != "nce row text" {
		t.Errorf("NewRdfNode through a session's manager: %v %v", n, err)
	}
	// a date the engine cannot read is not created either, through a session's manager too
	if _, err := compute_pipes.ParseRdfNodeValue(rm, "not a date", "date"); err == nil {
		t.Errorf("ParseRdfNodeValue of a date the engine cannot read: no error")
	}
}
