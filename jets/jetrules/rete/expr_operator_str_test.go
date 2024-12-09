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
