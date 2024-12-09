package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the Node's logic operators

func TestMathSumValues1(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Fatalf("error: nil returned by NewResourceManager")
	}
	jr := rm.JetsResources
	config := rm.NewResource("config")
	objP := rm.NewResource("objP")
	dataP := rm.NewResource("dataP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	metaGraph.Insert(config, jr.Jets__entity_property, objP)
	metaGraph.Insert(config, jr.Jets__value_property, dataP)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	s1 := rm.NewResource("s1")
	s2 := rm.NewResource("s2")
	s3 := rm.NewResource("s3")
	rdfSession.Insert(s, objP, s1)
	rdfSession.Insert(s, objP, s2)
	rdfSession.Insert(s, objP, s3)
	rdfSession.Insert(s1, dataP, rm.NewIntLiteral(1))
	rdfSession.Insert(s2, dataP, rm.NewIntLiteral(2))
	rdfSession.Insert(s3, dataP, rm.NewIntLiteral(3))
	reteSession := NewReteSession(rdfSession)

	op := NewSumValuesOp()
	op.InitializeOperator(metaGraph, s, config)
	if !op.Eval(reteSession, nil, s, config).EQ(rdf.I(6)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, config).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}
}

func TestMathSumValues2(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	dataP := rm.NewResource("dataP")
	metaGraph := rdf.NewMetaRdfGraph(rm)
	rdfSession := rdf.NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(1))
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(2))
	rdfSession.Insert(s, dataP, rm.NewIntLiteral(3))
	reteSession := NewReteSession(rdfSession)

	op := NewSumValuesOp()
	op.InitializeOperator(metaGraph, s, dataP)
	if !op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(6)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(1)).Bool() {
		t.Error("operator failed")
	}
	if op.Eval(reteSession, nil, s, dataP).EQ(rdf.I(2)).Bool() {
		t.Error("operator failed")
	}
}
