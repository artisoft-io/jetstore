package rete

// rules_benchmark I-91: a stale child row after a filter's re-entry.
//
// A candidate row is built from parent row P and its filter runs. The filter
// writes to the graph -- in a compiled rule set only the jets:exception a failed
// truth test raises (expr_writes_graph.go) -- and the write's callbacks run at
// once (RdfGraph.Insert), re-entering the rete through TripleUpdated. If that
// removes P, the candidate was built from a row that no longer exists; the
// filter still returned true, and the row was inserted and never retracted.
//
// The test is an order dependence, decided with the C++ engine's writer so that
// both engines run the same case (Q-64): run A reaches the re-entry, run B
// asserts the blocking triple first, and both must end with the same graph.
//
//	v1  (?s inGroup ?g)
//	v2  not(jets:iState jets:exception ?e)
//	v3  (?g q ?v).[not(?v and true)]  ->  (?s outP ?v)
//
// ?v is the text "maybe", which is not a bool: `?v and true` raises a
// jets:exception and is null, and not(null) is true, so the filter keeps the
// candidate after its own write has removed P from v2.

import (
	"slices"
	"sort"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

type i91Net struct {
	rm                     *rdf.ResourceManager
	metaGraph              *rdf.RdfGraph
	ms                     *ReteMetaStore
	inGroup, q, outP, s, g *rdf.Node
}

func newI91Net(t *testing.T) *i91Net {
	t.Helper()
	n := &i91Net{rm: rdf.NewResourceManager(nil)}
	rm := n.rm
	jr := rm.JetsResources
	n.metaGraph = rdf.NewMetaRdfGraph(rm)
	n.inGroup, n.q, n.outP = rm.NewResource("inGroup"), rm.NewResource("q"), rm.NewResource("outP")
	n.s, n.g = rm.NewResource("s"), rm.NewResource("g")

	filter := NewExprUnaryOp(NewNotOp(),
		NewExprBinaryOp(NewExprBindedVar(2, "?v"), NewAndOp(), NewExprCst(rdf.B(true))))
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?s", "?g"}))
	nv2 := NewNodeVertex(2, nv1, true, 100, nil, "v2", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1}, []string{"?s", "?g"}))
	nv3 := NewNodeVertex(3, nv2, false, 100, filter, "v3", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1, brcTriple | 2}, []string{"?s", "?g", "?v"}))
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: n.inGroup}, &FVariable{variable: "?g"}, nv1, true, "(?s inGroup ?g)"),
		NewAlphaNode(&FConstant{node: jr.Jets__istate}, &FConstant{node: jr.Jets__exception}, &FVariable{variable: "?e"}, nv2, true, "not(jets:iState jets:exception ?e)"),
		NewAlphaNode(&FBinded{pos: 1}, &FConstant{node: n.q}, &FVariable{variable: "?v"}, nv3, true, "(?g q ?v)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: n.outP}, &FBinded{pos: 2}, nv3, false, "(?s outP ?v)"),
	}
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an, LegacyRetraction)
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, n.metaGraph, nil, alphaNodes, []*NodeVertex{head, nv1, nv2, nv3}, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	n.ms = ms
	return n
}

// run is one session. before are the triples asserted before the data; with
// split, the rules run between (s inGroup g) and (g q "maybe"), so the
// candidate is made by TripleUpdated rather than by the first visit. It returns
// the session's graph, one line per triple, sorted, and its jets:exception.
func (n *i91Net) run(t *testing.T, before [][3]*rdf.Node, split bool) ([]string, *rdf.Node) {
	t.Helper()
	sess := rdf.NewRdfSession(n.rm, n.metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(n.ms)
	defer rs.Done()
	for _, t3 := range before {
		sess.Insert(t3[0], t3[1], t3[2])
	}
	sess.Insert(n.s, n.inGroup, n.g)
	if split {
		if err := rs.ExecuteRules(); err != nil {
			t.Fatal(err)
		}
	}
	sess.Insert(n.g, n.q, sess.ResourceMgr.NewTextLiteral("maybe"))
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	var lines []string
	for t3 := range sess.Find().All() {
		lines = append(lines, rdf.ToString(&t3))
	}
	sort.Strings(lines)
	jr := n.rm.JetsResources
	return lines, sess.GetObject(jr.Jets__istate, jr.Jets__exception)
}

func TestRegression_I91_ReentryRemovesParent(t *testing.T) {
	n := newI91Net(t)
	jr := n.rm.JetsResources
	for _, split := range []bool{false, true} {
		name := "first visit"
		if split {
			name = "TripleUpdated"
		}
		t.Run(name, func(t *testing.T) {
			a, msg := n.run(t, nil, split)
			if msg == nil {
				t.Fatal("run A raised no jets:exception: the test no longer reaches the re-entry")
			}
			b, _ := n.run(t, [][3]*rdf.Node{{jr.Jets__istate, jr.Jets__exception, msg}}, split)
			if !slices.Equal(a, b) {
				t.Errorf("the graph depends on the order of the triples:\nrun A (the filter raises):\n  %v\nrun B (the exception asserted first):\n  %v", a, b)
			}
		})
	}
}
