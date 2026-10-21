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
		for t3 := range itor.All() {
			count += 1
			if t3[1] != p {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
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
		for t3 := range itor.All() {
			count += 1
			if t3[2] != o {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
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
		for t3 := range itor.All() {
			count += 1
			if t3[0] != x || t3[2] != z {
				t.Errorf("error: unexpected triple returned by FindSPO: (%s, %s, %s)", t3[0], t3[1], t3[2])
			}
		}
		if count != 3 {
			t.Errorf("error: unexpecting 3 triples from FindSPO, got %d", count)
		}
	}
}

// TestRetractKeepsATripleWithASupportLeftInEveryIndex is rules_benchmark I-7's
// regression test: a triple inserted twice and retracted once keeps one support,
// so every binding pattern still finds it, through all three indexes; retracted
// again, none does.
func TestRetractKeepsATripleWithASupportLeftInEveryIndex(t *testing.T) {
	rm := NewResourceManager(nil)
	g := NewRdfGraph("INFERRED")
	s, p, o := rm.NewResource("s"), rm.NewResource("p"), rm.NewResource("o")
	g.Insert(s, p, o)
	g.Insert(s, p, o)
	found := func() (n int, missing []string) {
		keys := map[string][3]*Node{
			"(s,p,o)": {s, p, o}, "(s,p,?)": {s, p, nil}, "(s,?,?)": {s, nil, nil}, "(s,?,o)": {s, nil, o},
			"(?,p,o)": {nil, p, o}, "(?,p,?)": {nil, p, nil}, "(?,?,o)": {nil, nil, o}, "(?,?,?)": {nil, nil, nil},
		}
		for name, k := range keys {
			if g.FindSPO(k[0], k[1], k[2]).IsEmpty() {
				missing = append(missing, name)
			} else {
				n++
			}
		}
		return n, missing
	}
	if erased, err := g.Retract(s, p, o); err != nil || erased {
		t.Fatalf("first Retract: erased %v, err %v; want the triple kept", erased, err)
	}
	if n, missing := found(); n != 8 || !g.Contains(s, p, o) {
		t.Errorf("after one Retract of two supports: found by %d of 8 patterns, missing %v", n, missing)
	}
	if erased, err := g.Retract(s, p, o); err != nil || !erased {
		t.Fatalf("second Retract: erased %v, err %v; want the triple removed", erased, err)
	}
	if n, _ := found(); n != 0 || g.Contains(s, p, o) {
		t.Errorf("after the last support is retracted: still found by %d patterns", n)
	}
}
