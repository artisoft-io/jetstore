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
//	v3  (?g q ?v).[not(?v) != 1]  ->  (?s outP ?v)
//
// ?v is the text "maybe", which is not a bool: not(?v) raises a jets:exception
// and is null, and null != 1 is true, so the filter keeps the candidate after
// its own write has removed P from v2. The expression is the one both engines
// raise on: C++'s `and` does not raise on text (AndVisitor, expr_op_logics.h),
// its `not` does (NotVisitor, :546).

import (
	"fmt"
	"slices"
	"sort"
	"strings"
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

	filter := NewExprBinaryOp(NewExprUnaryOp(NewNotOp(), NewExprBindedVar(2, "?v")),
		NewNeOp(), NewExprCst(rm.NewIntLiteral(1)))
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

// The C++ engine's I-91 test, run on Go: the same networks, data and orders as
// CandidateRowReentryTest (jets/rete/rete_session_test.cc), so that both engines
// pass one case. The filter is create_entity(?v): hand-built here, since the
// compiler refuses a graph-writing operator in a filter. Each subject xi has
// (xi q "xi"), so the entity the filter mints is xi itself, and its
// (xi jets:key "xi") triple removes xi's own row from not(?s jets:key ?k) while
// the filter runs.
//
//	variant 1  v1 (?s inGroup ?g)  v2 not(?s jets:key ?k)  v3 (?s q ?v).[create_entity(?v)] -> (?s outP ?v)
//	variant 2  as variant 1 without v3's filter, then
//	           v4 not(?s r ?z).[create_entity(?v)] -> (?s outP ?v)
type reentryOrder int

const (
	kAllAtOnce           reentryOrder = iota // one ExecuteRules: the relations' first visit
	kJoinAfterActivation                     // inGroup, execute, then q, execute: TripleUpdated
	kKeysFirst                               // the jets:key triples asserted first: the reference
)

func newReentryNet(t *testing.T, filterOnNegation bool) (*rdf.ResourceManager, *rdf.RdfGraph, *ReteMetaStore) {
	t.Helper()
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	metaGraph := rdf.NewMetaRdfGraph(rm)
	inGroup, q, r, outP := rm.NewResource("inGroup"), rm.NewResource("q"), rm.NewResource("r"), rm.NewResource("outP")
	createEntity := NewExprUnaryOp(NewCreateEntityOp(), NewExprBindedVar(2, "?v"))
	var v3Filter, v4Filter Expression
	if filterOnNegation {
		v4Filter = createEntity
	} else {
		v3Filter = createEntity
	}
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?s", "?g"}))
	nv2 := NewNodeVertex(2, nv1, true, 100, nil, "v2", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1}, []string{"?s", "?g"}))
	nv3 := NewNodeVertex(3, nv2, false, 100, v3Filter, "v3", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1, brcTriple | 2}, []string{"?s", "?g", "?v"}))
	vertices := []*NodeVertex{head, nv1, nv2, nv3}
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: inGroup}, &FVariable{variable: "?g"}, nv1, true, "(?s inGroup ?g)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: jr.Jets__key}, &FVariable{variable: "?k"}, nv2, true, "not(?s jets:key ?k)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: q}, &FVariable{variable: "?v"}, nv3, true, "(?s q ?v)"),
	}
	consequentVertex := nv3
	if filterOnNegation {
		nv4 := NewNodeVertex(4, nv3, true, 100, v4Filter, "v4", []string{"r1"},
			NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1, brcParentNode | 2}, []string{"?s", "?g", "?v"}))
		vertices = append(vertices, nv4)
		alphaNodes = append(alphaNodes,
			NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: r}, &FVariable{variable: "?z"}, nv4, true, "not(?s r ?z)"))
		consequentVertex = nv4
	}
	alphaNodes = append(alphaNodes,
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: outP}, &FBinded{pos: 2}, consequentVertex, false, "(?s outP ?v)"))
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an, LegacyRetraction)
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, metaGraph, nil, alphaNodes, vertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return rm, metaGraph, ms
}

func runReentry(t *testing.T, rm *rdf.ResourceManager, metaGraph *rdf.RdfGraph, ms *ReteMetaStore, order reentryOrder) []string {
	t.Helper()
	sess := rdf.NewRdfSession(rm, metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(ms)
	defer rs.Done()
	srm := sess.ResourceMgr
	g := srm.NewResource("g")
	inGroup, q, jetsKey := srm.NewResource("inGroup"), srm.NewResource("q"), srm.JetsResources.Jets__key
	execute := func() {
		if err := rs.ExecuteRules(); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 4 {
		name := fmt.Sprintf("x%d", i)
		sess.Insert(srm.NewResource(name), inGroup, g)
		if order == kKeysFirst {
			sess.Insert(srm.NewResource(name), jetsKey, srm.NewTextLiteral(name))
		}
	}
	if order == kJoinAfterActivation {
		execute()
	}
	for i := range 4 {
		name := fmt.Sprintf("x%d", i)
		sess.Insert(srm.NewResource(name), q, srm.NewTextLiteral(name))
	}
	execute()
	var lines []string
	for t3 := range sess.Find().All() {
		lines = append(lines, rdf.ToString(&t3))
	}
	sort.Strings(lines)
	return lines
}

func TestRegression_I91_CandidateRowReentry(t *testing.T) {
	for _, variant := range []struct {
		name             string
		filterOnNegation bool
	}{{"filter on the join", false}, {"filter on a negation below", true}} {
		t.Run(variant.name, func(t *testing.T) {
			rm, metaGraph, ms := newReentryNet(t, variant.filterOnNegation)
			reference := runReentry(t, rm, metaGraph, ms, kKeysFirst)
			keys, outs := 0, 0
			for _, l := range reference {
				switch {
				case strings.Contains(l, ", jets:key, "):
					keys++
				case strings.Contains(l, ", outP, "):
					outs++
				}
			}
			if keys != 4 || outs != 0 {
				t.Fatalf("reference: %d jets:key and %d outP triples, want 4 and 0:\n  %v", keys, outs, reference)
			}
			for _, order := range []struct {
				name  string
				order reentryOrder
			}{{"all at once", kAllAtOnce}, {"join after activation", kJoinAfterActivation}} {
				if got := runReentry(t, rm, metaGraph, ms, order.order); !slices.Equal(got, reference) {
					t.Errorf("%s: the graph differs from the reference (the jets:key triples asserted first):\n  got  %v\n  want %v", order.name, got, reference)
				}
			}
		})
	}
}

// I-91, second half (AW.1b): the re-entry is checked for every candidate of a
// scan, not only for the candidate whose filter re-entered. Subjects s and s2
// are in group g, which has (g q "maybe"), whose filter raises, and
// (g q "true"), whose filter writes nothing; the raise removes both subjects'
// rows from v2. A check taken around each filter alone missed:
//   - in the first visit, a candidate of the same scan after the raise, built
//     from a removed row; and one before it, inserted into v3 while v3 was not
//     yet activated, so the removal never reached it;
//   - in TripleUpdated, the second subject's candidate for (g q "maybe"): its
//     filter raises the same exception again, which is no insert and no
//     re-entry, so it was kept, built from a removed row.
//
// Go walks triples and rows in map order, so the test runs enough sessions to
// meet the candidates in each order, in each of the two ways they are made (the
// first visit and TripleUpdated). Every session must end with the reference
// graph: the exception asserted first, so v2 never has a row and nothing is
// inferred.
func TestRegression_I91_LaterCandidateOfARemovedParent(t *testing.T) {
	n := newI91Net(t)
	jr := n.rm.JetsResources
	run := func(t *testing.T, before [][3]*rdf.Node, split bool) ([]string, *rdf.Node) {
		t.Helper()
		sess := rdf.NewRdfSession(n.rm, n.metaGraph)
		rs := NewReteSession(sess)
		rs.Initialize(n.ms)
		defer rs.Done()
		for _, t3 := range before {
			sess.Insert(t3[0], t3[1], t3[2])
		}
		srm := sess.ResourceMgr
		sess.Insert(n.s, n.inGroup, n.g)
		sess.Insert(srm.NewResource("s2"), n.inGroup, n.g)
		if split {
			if err := rs.ExecuteRules(); err != nil {
				t.Fatal(err)
			}
		}
		sess.Insert(n.g, n.q, srm.NewTextLiteral("maybe"))
		sess.Insert(n.g, n.q, srm.NewTextLiteral("true"))
		if err := rs.ExecuteRules(); err != nil {
			t.Fatal(err)
		}
		var lines []string
		for t3 := range sess.Find().All() {
			lines = append(lines, rdf.ToString(&t3))
		}
		sort.Strings(lines)
		return lines, sess.GetObject(jr.Jets__istate, jr.Jets__exception)
	}
	for _, split := range []bool{false, true} {
		name := "first visit"
		if split {
			name = "TripleUpdated"
		}
		t.Run(name, func(t *testing.T) {
			var reference []string
			wrong := 0
			const sessions = 100
			for range sessions {
				got, msg := run(t, nil, split)
				if msg == nil {
					t.Fatal("no jets:exception raised: the test no longer reaches the re-entry")
				}
				if reference == nil {
					reference, _ = run(t, [][3]*rdf.Node{{jr.Jets__istate, jr.Jets__exception, msg}}, split)
					for _, l := range reference {
						if strings.Contains(l, ", outP, ") {
							t.Fatalf("the reference infers %s", l)
						}
					}
				}
				if !slices.Equal(got, reference) {
					if wrong == 0 {
						t.Errorf("the graph depends on the order of the triples:\n  got  %v\n  want %v", got, reference)
					}
					wrong++
				}
			}
			if wrong > 0 {
				t.Errorf("%d of %d sessions differ from the reference", wrong, sessions)
			}
		})
	}
}
