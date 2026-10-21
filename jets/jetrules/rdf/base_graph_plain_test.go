package rdf

import (
	"fmt"
	"testing"
)

// BaseGraph on plain maps (rules_benchmark G2): empty inner maps are pruned (I-11),
// and the iterators' same-goroutine write contract holds.

func TestBaseGraphPrunesEmptyInnerMaps(t *testing.T) {
	rm := NewResourceManager(nil)
	s, p1, p2, o1, o2 := rm.NewResource("s"), rm.NewResource("p1"), rm.NewResource("p2"), rm.NewResource("o1"), rm.NewResource("o2")
	g := NewBaseGraph("TEST", 's')
	g.Insert(s, p1, o1)
	g.Insert(s, p1, o2)
	g.Insert(s, p2, o1)
	g.Insert(s, p2, o1) // count 2

	if !g.Erase(s, p1, o1) || len(g.data[s][p1]) != 1 {
		t.Fatalf("Erase(s p1 o1): want (s p1) to keep o2, got %v", g.data[s][p1])
	}
	if !g.Erase(s, p1, o2) {
		t.Fatal("Erase(s p1 o2) returned false")
	}
	if _, ok := g.data[s][p1]; ok {
		t.Errorf("(s p1) is empty and still in the graph")
	}
	if g.Retract(s, p2, o1) || g.GetRefCount(s, p2, o1) != 1 {
		t.Fatalf("Retract of a count-2 triple: want it kept with count 1, got %d", g.GetRefCount(s, p2, o1))
	}
	if !g.Retract(s, p2, o1) {
		t.Fatal("Retract of a count-1 triple returned false")
	}
	if _, ok := g.data[s]; ok || len(g.data) != 0 {
		t.Errorf("s has nothing left and is still in the graph: %v", g.data)
	}
	if g.Size() != 0 || g.ContainsUV(s, p2) || g.Contains(s, p2, o1) {
		t.Errorf("empty graph: size %d, ContainsUV %v, Contains %v", g.Size(), g.ContainsUV(s, p2), g.Contains(s, p2, o1))
	}
	if g.Erase(s, p1, o1) || g.Retract(s, p1, o1) {
		t.Errorf("Erase or Retract of a triple not in the graph returned true")
	}
	if !g.Insert(s, p1, o1) || g.Size() != 1 {
		t.Errorf("re-insert after pruning: size %d", g.Size())
	}
}

// A loop body may erase and insert in the graph it walks, on the same goroutine:
// the walk finishes, an erased triple not yet reached is not returned, and every
// triple neither erased nor inserted is returned once.
func TestWalkWithWritesOnTheSameGoroutine(t *testing.T) {
	rm := NewResourceManager(nil)
	g := NewRdfGraph("TEST")
	p := rm.NewResource("p")
	var subjects []*Node
	for i := range 50 {
		s := rm.NewResource(fmt.Sprintf("s%d", i))
		subjects = append(subjects, s)
		g.Insert(s, p, rm.NewIntLiteral(i))
	}
	erased := map[*Node]bool{}
	seen := map[*Node]int{}
	for t3 := range g.FindSPO(nil, p, nil).All() {
		seen[t3[0]]++
		if erased[t3[0]] {
			t.Errorf("%s was erased before the walk reached it, and was returned", t3[0])
		}
		// erase another subject's only triple, emptying (and pruning) its inner
		// maps in all three indexes, and insert a new one
		for _, s := range subjects {
			if s != t3[0] && seen[s] == 0 && !erased[s] {
				if _, err := g.Erase(s, nil, nil); err != nil {
					t.Fatal(err)
				}
				erased[s] = true
				break
			}
		}
		g.Insert(rm.NewResource(fmt.Sprintf("new%d", len(seen))), p, rm.NewIntLiteral(-1))
	}
	for _, s := range subjects {
		if !erased[s] && seen[s] != 1 {
			t.Errorf("%s, neither erased nor inserted, returned %d times", s, seen[s])
		}
	}
	if len(erased) == 0 {
		t.Fatal("the walk erased nothing; the test exercises nothing")
	}
}

// A Find() walk reads the spo index, whose outer map is the one being walked. A
// loop body that erases every triple of the subject it is on prunes that subject
// from the walked map, and re-inserting them adds it back as a new entry, which
// the walk may produce again or not at all. The walk must still end, and every
// triple of a subject left alone must be produced exactly once.
func TestWalkErasesAndReinsertsTheWalkedSubject(t *testing.T) {
	rm := NewResourceManager(nil)
	g := NewRdfGraph("TEST")
	preds := []*Node{rm.NewResource("p1"), rm.NewResource("p2")}
	victims := map[*Node]bool{}
	var subjects []*Node
	for i := range 60 {
		s := rm.NewResource(fmt.Sprintf("s%d", i))
		subjects = append(subjects, s)
		if i%5 == 0 {
			victims[s] = true
		}
		for j, p := range preds {
			g.Insert(s, p, rm.NewIntLiteral(10*i+j))
		}
	}
	size := g.Size()
	touched := map[*Node]bool{}
	seen := map[[3]*Node]int{}
	steps := 0
	for t3 := range g.Find().All() {
		if steps++; steps > 10*size {
			t.Fatalf("the walk has produced %d triples from a graph of %d; it is not ending", steps, size)
		}
		seen[t3]++
		s := t3[0]
		if !victims[s] || touched[s] {
			continue
		}
		touched[s] = true
		// erase every triple of the walked subject, which prunes it from the walked
		// map, then put them back
		var mine [][3]*Node
		for u := range g.FindS(s).All() {
			mine = append(mine, u)
		}
		if _, err := g.Erase(s, nil, nil); err != nil {
			t.Fatal(err)
		}
		if _, ok := g.spoGraph.data[s]; ok {
			t.Fatalf("%s is still in the spo index after all its triples were erased", s)
		}
		for _, u := range mine {
			g.Insert(u[0], u[1], u[2])
		}
	}
	if len(touched) != len(victims) {
		t.Errorf("the walk reached %d of the %d subjects it was to erase and re-insert", len(touched), len(victims))
	}
	for _, s := range subjects {
		for j, p := range preds {
			o := rm.NewIntLiteral(10*indexOf(subjects, s) + j)
			n := seen[[3]*Node{s, p, o}]
			// an erased and re-inserted triple may be produced any number of times
			// (zero, if the walk was inside the pruned map when it was erased)
			if !victims[s] && n != 1 {
				t.Errorf("(%s %s %s), left alone, was produced %d times, want 1", s, p, o, n)
			}
		}
	}
	if g.Size() != size {
		t.Errorf("size %d after the walk, want %d", g.Size(), size)
	}
}

func indexOf(nodes []*Node, n *Node) int {
	for i, m := range nodes {
		if m == n {
			return i
		}
	}
	return -1
}
