package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for strings operators

func TestSubstring1(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	jr := rm.JetsResources
	config := rm.NewResource("config")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__from, rm.NewIntLiteral(0))
	metaGraph.Insert(config, jr.Jets__length, rm.NewIntLiteral(-2))
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	value := rm.NewTextLiteral("value1-1")
	reteSession := NewReteSession(rdfSession)

	op := NewSubstringOfOp()
	op.InitializeOperator(metaGraph, config, value)
	if !op.Eval(reteSession, nil, config, value).EQ(rdf.S("value1")).Bool() {
		t.Error("operator failed")
	}
}

func TestSubstring2(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	jr := rm.JetsResources
	config := rm.NewResource("config")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__from, rm.NewIntLiteral(1))
	metaGraph.Insert(config, jr.Jets__length, rm.NewIntLiteral(3))
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	value := rm.NewTextLiteral(" value1-1")
	reteSession := NewReteSession(rdfSession)

	op := NewSubstringOfOp()
	op.InitializeOperator(metaGraph, config, value)
	if !op.Eval(reteSession, nil, config, value).EQ(rdf.S("val")).Bool() {
		t.Error("operator failed")
	}
}

func TestSubstring3(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	jr := rm.JetsResources
	config := rm.NewResource("config")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__from, rm.NewIntLiteral(1))
	metaGraph.Insert(config, jr.Jets__length, rm.NewIntLiteral(-2))
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	value := rm.NewTextLiteral(" value1-1")
	reteSession := NewReteSession(rdfSession)

	op := NewSubstringOfOp()
	op.InitializeOperator(metaGraph, config, value)
	if !op.Eval(reteSession, nil, config, value).EQ(rdf.S("value1")).Bool() {
		t.Error("operator failed")
	}
}

// rules_benchmark I-32: to_text writes a date, a datetime and a null as C++'s
// ToTextVisitor does.
func TestToTextDateIsIso(t *testing.T) {
	op := NewToTextOp()
	cases := []struct {
		rhs  *rdf.Node
		want string
	}{
		{rdf.DD("2025-06-05"), "2025-06-05"},
		{rdf.DDT("2025-06-05T13:04:05"), "2025-06-05T13:04:05"},
		{rdf.Null(), "null"},
		{rdf.I(42), "42"},
		{rdf.S("text"), "text"},
	}
	for _, c := range cases {
		if got := op.Eval(nil, nil, c.rhs); got == nil || got.Value != c.want {
			t.Errorf("to_text(%v) = %v, want %q", c.rhs.Value, got, c.want)
		}
	}
}
