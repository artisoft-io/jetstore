package rdf

import (
	"testing"
)

type NotificationCallbackTest struct {
	inserted map[Triple]bool
	deleted  map[Triple]bool
}

func (n *NotificationCallbackTest) TripleInserted(s, p, o *Node) {
	n.inserted[T3(s, p, o)] = true
}

func (n *NotificationCallbackTest) TripleDeleted(s, p, o *Node) {
	n.deleted[T3(s, p, o)] = true
}

// This file contains test cases for the BaseGraph in rdf package
func TestNotifications(t *testing.T) {
	// create RdfSession
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	rdfSession := NewRdfSession(rm, NewRdfGraph("META"))
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	// Add the NotificationCallback
	cback := &NotificationCallbackTest{
		inserted: make(map[[3]*Node]bool),
		deleted: make(map[[3]*Node]bool),
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cback)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cback)
	// Switch to RdfSession ResourceManager and insert triples
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	o2 := rm.NewResource("o2")
	rdfSession.Insert(s, p, o)
	if !cback.inserted[T3(s, p, o)] {
		t.Error("cback.inserted missing T3(s, p, o)")
	}
	rdfSession.InsertInferred(s, p, o2)
	if !cback.inserted[T3(s, p, o2)] {
		t.Error("cback.inserted missing T3(s, p, o2)")
	}
	rdfSession.Retract(s, p, o2)
	if !cback.deleted[T3(s, p, o2)] {
		t.Error("cback.deleted missing T3(s, p, o2)")
	}
	rdfSession.Retract(s, p, o)
	if cback.deleted[T3(s, p, o)] {
		t.Error("cback.deleted should not be called on retract to asserted triple T3(s, p, o)")
	}
	rdfSession.Erase(s, p, o)
	if !cback.deleted[T3(s, p, o)] {
		t.Error("cback.deleted missing T3(s, p, o)")
	}
}
func TestNotifications2(t *testing.T) {
	// create RdfSession
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	rdfSession := NewRdfSession(rm, NewRdfGraph("META"))
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	// Add the NotificationCallback
	cback := &NotificationCallbackTest{
		inserted: make(map[[3]*Node]bool),
		deleted: make(map[[3]*Node]bool),
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cback)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cback)
	// Switch to RdfSession ResourceManager and insert triples
	rm = rdfSession.ResourceMgr
	s := rm.NewResource("s")
	// s2 := rm.NewResource("s")
	p := rm.NewResource("p")
	// p2 := rm.NewResource("p2")
	o := rm.NewResource("o")
	o2 := rm.NewResource("o2")
	rdfSession.Insert(s, p, o)
	if !cback.inserted[T3(s, p, o)] {
		t.Error("cback.inserted missing T3(s, p, o)")
	}
	rdfSession.InsertInferred(s, p, o2)
	if !cback.inserted[T3(s, p, o2)] {
		t.Error("cback.inserted missing T3(s, p, o2)")
	}
	rdfSession.Erase(s, p, nil)
	if !cback.deleted[T3(s, p, o)] {
		t.Error("cback.deleted missing T3(s, p, o)")
	}
	if !cback.deleted[T3(s, p, o2)] {
		t.Error("cback.deleted missing T3(s, p, o2)")
	}
}
