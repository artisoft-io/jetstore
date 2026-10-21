package rdf

import (
	"fmt"
	"slices"
	"testing"
)

// The pull form of the synchronous iterators (rules_benchmark G1, I-23) must return
// what the range form returns, on every binding pattern, for a base graph and for a
// session whose triples are spread over its three graphs.

func cursorTriples(c TripleCursor) []string {
	var out []string
	for c.Next() {
		t3 := c.Value()
		out = append(out, fmt.Sprintf("(%s, %s, %s)", t3[0], t3[1], t3[2]))
	}
	slices.Sort(out)
	return out
}

func seqTriples(all func(func([3]*Node) bool)) []string {
	var out []string
	for t3 := range all {
		out = append(out, fmt.Sprintf("(%s, %s, %s)", t3[0], t3[1], t3[2]))
	}
	slices.Sort(out)
	return out
}

// cursorTestSession puts some triples in each graph, sharing subjects, predicates
// and objects across graphs, and retracts one so that an emptied inner map is left
// behind (I-11).
func cursorTestSession(t *testing.T, withMeta bool) (*RdfSession, []*Node) {
	t.Helper()
	rm := NewResourceManager(nil)
	meta := NewMetaRdfGraph(rm)
	var nodes []*Node
	for i := range 4 {
		nodes = append(nodes, rm.NewResource(fmt.Sprintf("n%d", i)))
	}
	if withMeta {
		if _, err := meta.Insert(nodes[0], nodes[1], nodes[2]); err != nil {
			t.Fatal(err)
		}
		if _, err := meta.Insert(nodes[3], nodes[1], nodes[0]); err != nil {
			t.Fatal(err)
		}
	}
	sess := NewRdfSession(rm, meta)
	for _, t3 := range [][3]int{{0, 1, 3}, {0, 2, 3}, {1, 1, 2}, {2, 3, 0}, {3, 1, 1}} {
		if _, err := sess.Insert(nodes[t3[0]], nodes[t3[1]], nodes[t3[2]]); err != nil {
			t.Fatal(err)
		}
	}
	for _, t3 := range [][3]int{{0, 1, 0}, {1, 2, 3}, {2, 2, 2}, {1, 3, 1}} {
		if _, err := sess.InsertInferred(nodes[t3[0]], nodes[t3[1]], nodes[t3[2]]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sess.Retract(nodes[1], nodes[3], nodes[1]); err != nil {
		t.Fatal(err)
	}
	return sess, nodes
}

func TestTripleCursorMatchesAll(t *testing.T) {
	for _, withMeta := range []bool{false, true} {
		sess, nodes := cursorTestSession(t, withMeta)
		// every pattern, each position nil or one of the nodes
		keys := append([]*Node{nil}, nodes...)
		patterns := 0
		for _, s := range keys {
			for _, p := range keys {
				for _, o := range keys {
					itor := sess.FindSPO(s, p, o)
					want := seqTriples(itor.All())
					if got := cursorTriples(itor.Cursor()); !slices.Equal(got, want) {
						t.Errorf("meta %v, session FindSPO(%s, %s, %s): cursor %v, All %v", withMeta, s, p, o, got, want)
					}
					if itor.IsEmpty() != (len(want) == 0) {
						t.Errorf("meta %v, session FindSPO(%s, %s, %s): IsEmpty %v with %d triples", withMeta, s, p, o, itor.IsEmpty(), len(want))
					}
					bitor := sess.AssertedGraph.FindSPO(s, p, o)
					want = seqTriples(bitor.All())
					if got := cursorTriples(bitor.Cursor()); !slices.Equal(got, want) {
						t.Errorf("meta %v, asserted FindSPO(%s, %s, %s): cursor %v, All %v", withMeta, s, p, o, got, want)
					}
					patterns++
				}
			}
		}
		// the whole session, against each graph's triples
		want := slices.Concat(sess.MetaGraph.ToTriples(), sess.AssertedGraph.ToTriples(), sess.InferredGraph.ToTriples())
		slices.Sort(want)
		if got := cursorTriples(sess.Find().Cursor()); !slices.Equal(got, want) {
			t.Errorf("meta %v, session Find: cursor %v, want %v", withMeta, got, want)
		}
		if patterns != 125 {
			t.Fatalf("checked %d patterns, want 125", patterns)
		}
	}
}

// A cursor is lazy: dropping it after the first triple must not have read the rest.
// Observable as: a triple inserted into a later graph after the first Next is seen.
func TestTripleCursorIsLazyAcrossGraphs(t *testing.T) {
	sess, nodes := cursorTestSession(t, false)
	s, p := nodes[0], nodes[1]
	c := sess.FindSP(s, p).Cursor()
	if !c.Next() {
		t.Fatal("no first triple")
	}
	// the asserted graph answered (0 1 3); now add to the inferred graph
	if _, err := sess.InsertInferred(s, p, nodes[2]); err != nil {
		t.Fatal(err)
	}
	var objs []*Node
	for c.Next() {
		objs = append(objs, c.Value()[2])
	}
	if !slices.Contains(objs, nodes[2]) {
		t.Errorf("the inferred triple inserted after the first Next was not returned: %v", objs)
	}
}

func TestGetObjectAndExist(t *testing.T) {
	sess, nodes := cursorTestSession(t, true)
	for _, s := range nodes {
		for _, p := range nodes {
			var first *Node
			for t3 := range sess.FindSP(s, p).All() {
				first = t3[2]
				break
			}
			if got := sess.GetObject(s, p); got != first {
				t.Errorf("GetObject(%s, %s) = %v, the session iterator's first object is %v", s, p, got, first)
			}
			want := FALSE()
			if first != nil {
				want = TRUE()
			}
			if got := s.Exist(sess, p); got.Value != want.Value {
				t.Errorf("Exist(%s, %s) = %v, want %v", s, p, got, want)
			}
			if got := s.ExistNot(sess, p); got.Value == want.Value {
				t.Errorf("ExistNot(%s, %s) = %v, want not %v", s, p, got, want)
			}
		}
	}
}
