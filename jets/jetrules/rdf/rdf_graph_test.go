package rdf

import (
	"testing"
)

// This file contains test cases for the BaseGraph in rdf package
func TestRdfGraph(t *testing.T) {

	// test RdfGraph type
	rm := NewResourceManager(nil)
	if rm == nil {
		t.Errorf("error: nil returned by NewResourceManager")
	}
	s := rm.NewResource("s")
	p := rm.NewResource("p")
	o := rm.NewResource("o")
	u := rm.NewResource("u")
	v := rm.NewResource("v")
	w := rm.NewResource("w")
	x := rm.NewResource("x")
	y := rm.NewResource("y")
	z := rm.NewResource("z")
	graph := NewRdfGraph("META")
	// case 1
	graph.Insert(s, p, o)
	graph.Insert(u, p, w)
	// case 2
	graph.Insert(s, p, o)
	graph.Insert(u, v, o)
	graph.Insert(x, y, o)
	// case 3
	graph.Insert(x, y, z)
	graph.Insert(x, p, z)
	graph.Insert(x, v, z)
	// do case 1
	itor := graph.FindSPO(nil, p, nil)
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
	itor = graph.FindSPO(nil, nil, o)
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
	itor = graph.FindSPO(x, nil, z)
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
