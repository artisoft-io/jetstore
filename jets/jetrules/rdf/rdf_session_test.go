package rdf

import (
	"log"
	"testing"
)

// This file contains test cases for the BaseGraph in rdf package
func TestRdfSessionGetObject(t *testing.T) {

	// test RdfSession type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	metaGraph := NewRdfGraph("META")
	metaGraph.Insert(s, p, o)
	rdfSession := NewRdfSession(rm, nil)
	if rdfSession != nil {
		t.Errorf("error: expected nil rdfSession")
	}
	rdfSession = NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	obj := rdfSession.GetObject(s, p)
	if obj != o {
		t.Errorf("error: GetObject did not returned expected obj, got %v", obj)
	}
	obj = rdfSession.GetObject(s, o)
	if obj != nil {
		t.Errorf("error: GetObject did not returned expected nil, got %v", obj)
	}
}

func TestRdfSessionInsert(t *testing.T) {

	// test RdfSession type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Fatal("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	metaGraph := NewRdfGraph("META")
	metaGraph.Insert(s, p, o)
	rdfSession := NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}
	// Check that the rm is locked
	if !rm.isLocked {
		t.Errorf("error: expected ResourceManager to be locked by rdf session")
	}
	vv := rm.NewResource("vv")
	if vv != nil {
		t.Errorf("error: expected ResourceManager to be locked by rdf session")
	}
	rm = rdfSession.ResourceMgr
	if rm == nil {
		t.Errorf("error: unexpected nil ResourceManager from rdf session")
	}
	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	inserted, err := rdfSession.InsertInferred(u, v, w)
	if err != nil {
		t.Errorf("while calling InsertInferred: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	if !rdfSession.InferredGraph.Contains(u, v, w) {
		t.Errorf("error: expected triple to be in inferred graph of rdf session")
	}
	if !rdfSession.Contains(s, p, o) {
		t.Errorf("error: expected triple (s, p, o) to be in rdf session")
	}
	if !rdfSession.Contains(u, v, w) {
		t.Errorf("error: expected triple (s, p, o) to be in rdf session")
	}
	// Move the triple to the asserted graph
	inserted, err = rdfSession.Insert(u, v, w)
	if err != nil {
		t.Errorf("while calling Insert: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	if !rdfSession.AssertedGraph.Contains(u, v, w) {
		t.Errorf("error: expected triple (u, v, w) to be in asserted graph of rdf session")
	}
	if rdfSession.InferredGraph.Contains(u, v, w) {
		t.Errorf("error: not expecting triple (u, v, w) to be in inferred graph of rdf session")
	}

	x := rm.NewResource("x")
	y := rm.NewResource("y")
	z := rm.NewResource("z")
	inserted, err = rdfSession.Insert(x, y, z)
	if err != nil {
		t.Errorf("while calling Insert: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	inserted, err = rdfSession.InsertInferred(x, y, z)
	if err != nil {
		t.Errorf("while calling InsertInferred: %v", err)
	}
	if inserted {
		t.Errorf("error: expected triple NOT to be inserted in rdf session")
	}
	if !rdfSession.AssertedGraph.Contains(x, y, z) {
		t.Errorf("error: expected triple (x, y, z) to be in asserted graph of rdf session")
	}
	if rdfSession.InferredGraph.Contains(x, y, z) {
		t.Errorf("error: not expecting triple (x, y, z) to be in inferred graph of rdf session")
	}
}

func TestRdfSessionIterator(t *testing.T) {

	// test RdfGraph type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	metaGraph := NewRdfGraph("META")
	metaGraph.Insert(s, p, o)
	rdfSession := NewRdfSession(rm, metaGraph)
	if rdfSession == nil {
		t.Fatal("error: unexpected nil rdfSession")
	}

	rm = rdfSession.ResourceMgr
	if rm == nil {
		t.Errorf("error: unexpected nil ResourceManager from rdf session")
	}
	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	inserted, err := rdfSession.Insert(u, v, w)
	if err != nil {
		t.Errorf("while calling Insert: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	pp := rm.NewResource("p")
	if pp != p {
		t.Errorf("error: expected resurce pp to be same as p")
	}
	inserted, err = rdfSession.Insert(u, pp, w)
	if err != nil {
		t.Errorf("while calling Insert: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	rdfSession.Insert(u, v, o)

	x := rm.NewResource("x")
	y := rm.NewResource("y")
	z := rm.NewResource("z")
	inserted, err = rdfSession.InsertInferred(x, y, z)
	if err != nil {
		t.Errorf("while calling InsertInferred: %v", err)
	}
	if !inserted {
		t.Errorf("error: expected triple to be inserted in rdf session")
	}
	rdfSession.InsertInferred(x, y, o)
	rdfSession.InsertInferred(x, p, z)
	rdfSession.InsertInferred(x, v, z)

	// do case 1
	itor := rdfSession.FindSPO(nil, p, nil)
	count := 0
	if itor == nil {
		t.Errorf("error: nil returned by FindSPO")
	} else {
		for t3 := range itor.Itor {
			count += 1
			if t3[1] != p {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
		itor.Done()
		if count != 3 {
			t.Errorf("error: unexpecting 3 triples from FindSPO, got %d", count)
		}
	}
	// do case 2
	itor = rdfSession.FindSPO(nil, nil, o)
	count = 0
	if itor == nil {
		t.Errorf("error: nil returned by FindSPO")
	} else {
		for t3 := range itor.Itor {
			count += 1
			if t3[2] != o {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
		itor.Done()
		if count != 3 {
			t.Errorf("error: unexpecting 3 triples from FindSPO, got %d", count)
		}
	}
	// do case 3
	itor = rdfSession.FindSPO(x, nil, z)
	count = 0
	if itor == nil {
		t.Errorf("error: nil returned by FindSPO")
	} else {
		for t3 := range itor.Itor {
			count += 1
			if t3[0] != x || t3[2] != z {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
		itor.Done()
		if count != 3 {
			t.Errorf("error: unexpecting 3 triples from FindSPO, got %d", count)
		}
	}
}

func TestRdfSessionIterator2(t *testing.T) {

	// test RdfGraph type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	metaGraph := NewRdfGraph("META")
	metaGraph.Insert(s, p, o)
	rdfSession := NewRdfSession(rm, metaGraph)
	rm = rdfSession.ResourceMgr
	o2 := rm.NewResource("o2")
	rdfSession.Insert(s, p, o2)
	itor := rdfSession.Find()

	count := 0
	for vv := range itor.Itor {
		log.Println("Got value:", vv)
		if vv[2] != o && vv[2] != o2 {
			t.Errorf("Unexpected triple (%s, %s, %s)", vv[0], vv[1], vv[2])
		}
		count += 1
	}
	itor.Done()
	if count != 2 {
		t.Errorf("Expected count == 2, got %d", count)
	}
}
