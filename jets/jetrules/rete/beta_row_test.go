package rete

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// This file contains test cases for the BetaRow
func TestBetaRow(t *testing.T) {
	rMgr := rdf.NewResourceManager(nil)

	// Configure the first row: the initializer and the node_vertex
	initializer1 := NewBetaRowInitializer(
		[]int{
			0 | brcTriple,
			1 | brcTriple,
			2 | brcTriple,
		}, 
		[]string{"s", "p", "o"})
	nvertex1 := NewNodeVertex(0, nil, false, 100, nil, "test node vertex 0", nil, initializer1)

	// first row
	br1 := NewBetaRow(nvertex1, len(initializer1.InitData))
	t3 := rdf.T3(rMgr.NewResource("s1"), rMgr.NewResource("p1"), rMgr.NewResource("o1"))
	err := br1.Initialize(initializer1, nil, &t3)
	if err != nil {
		t.Error("while initializing row1:", err)
	}
	r := br1.Get(0)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "s1" {
			t.Errorf("error expecting s1 got %v", r)
		}
	}
	r = br1.Get(1)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "p1" {
			t.Errorf("error expecting p1 got %v", r)
		}
	}
	r = br1.Get(2)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "o1" {
			t.Errorf("error expecting o1 got %v", r)
		}
	}

	// second row
	initializer2 := NewBetaRowInitializer(
		[]int{
			0 | brcParentNode,
			1 | brcParentNode,
			0 | brcTriple,
			2 | brcTriple,
		}, 
		[]string{"s", "p", "s", "o"})
	nvertex2 := NewNodeVertex(1, nvertex1, false, 100, nil, "test node vertex 1", nil, initializer2)
	br2 := NewBetaRow(nvertex2, len(initializer2.InitData))
	t3 = rdf.T3(rMgr.NewResource("s2"), rMgr.NewResource("p2"), rMgr.NewResource("o2"))
	err = br2.Initialize(initializer2, br1, &t3)
	if err != nil {
		t.Error("while initializing row2:", err)
	}
	r = br2.Get(0)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "s1" {
			t.Errorf("error expecting s1 got %v", r)
		}
	}
	r = br2.Get(1)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "p1" {
			t.Errorf("error expecting p1 got %v", r)
		}
	}
	r = br2.Get(2)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "s2" {
			t.Errorf("error expecting s2 got %v", r)
		}
	}
	r = br2.Get(3)
	if r == nil {
		t.Error("error while calling Get on beta row")
	} else {
		if r.Name() != "o2" {
			t.Errorf("error expecting o2 got %v", r)
		}
	}
}