package rete

import (
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// VisitReteGraph reuses the session's stack (rules_benchmark G6). A visit started
// inside another, as a callback on an insert made during the visit would start one,
// must work above the outer visit's entries and leave them as it found them. Simulated
// here by entries already on the stack when the rules run.
func TestVisitReteGraphLeavesAnOuterVisitsEntries(t *testing.T) {
	n := newBenchNetwork(t)
	const claims = 40
	sess := n.newSession(claims)
	rs := NewReteSession(sess)
	rs.Initialize(n.ms)
	defer rs.Done()
	outer := IntStack{4, 1}
	rs.visitStack = &outer
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	if got := *rs.visitStack; !slices.Equal(got, IntStack{4, 1}) {
		t.Errorf("visit stack after ExecuteRules: got %v, want the outer entries [4 1]", got)
	}
	count := func(p *rdf.Node) int {
		c := 0
		itor := sess.InferredGraph.FindSPO(nil, p, nil)
		for range itor.All() {
			c++
		}
		return c
	}
	if got := count(n.hasClaim); got != claims {
		t.Errorf("hasClaim: inferred %d, want %d", got, claims)
	}
	if got := count(n.linked); got != claims-(claims+9)/10 {
		t.Errorf("linked: inferred %d, want %d", got, claims-(claims+9)/10)
	}
}

// The stack is allocated once per session and is empty between visits.
func TestVisitReteGraphReusesTheStack(t *testing.T) {
	n := newBenchNetwork(t)
	rs := NewReteSession(n.newSession(8))
	rs.Initialize(n.ms)
	defer rs.Done()
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	stack := rs.visitStack
	if stack == nil || stack.Len() != 0 {
		t.Fatalf("visit stack after ExecuteRules: got %v, want empty and allocated", stack)
	}
	if err := rs.VisitReteGraph(0, true); err != nil {
		t.Fatal(err)
	}
	if rs.visitStack != stack || stack.Len() != 0 {
		t.Errorf("a second visit replaced the stack or left entries: %v", *rs.visitStack)
	}
}

// writingFilter is a filter that inserts a triple when evaluated, as a filter's
// lookup or create_entity does (G1's write contract): the insert's callback starts
// a visit inside the visit evaluating the filter. It records what that nested
// visit left on the outer visit's stack and whether it reached nested.
type writingFilter struct {
	s, p, o  *rdf.Node
	nested   int
	evals    int
	stackOK  bool
	reachedV int
}

func (f *writingFilter) InitializeExpression(*ReteSession) error  { return nil }
func (f *writingFilter) RegisterCallback(*ReteSession, int) error { return nil }
func (f *writingFilter) Eval(*ReteSession, *BetaRow) *rdf.Node    { return nil }
func (f *writingFilter) StaticEval(*ReteSession) *rdf.Node        { return nil }
func (f *writingFilter) EvalFilter(rs *ReteSession, _ *BetaRow) bool {
	f.evals++
	before := slices.Clone(*rs.visitStack)
	if _, err := rs.RdfSession.InsertInferred(f.s, f.p, f.o); err != nil {
		panic(err)
	}
	f.stackOK = slices.Equal(before, *rs.visitStack)
	f.reachedV = rs.GetBetaRelation(f.nested).AllRows.Size()
	return true
}

// A visit started by a callback while another visit evaluates a filter: the
// nested visit finds its rows, and the outer visit's entries are as it left them.
//
//	v2  (?y rdf:type B)                    first child of the head
//	v3  (?y rdf:type B).(?y hasTag ?t) -> (?y tagged ?t)
//	v1  (?x rdf:type A).[filter inserts (x rdf:type B)]
func TestVisitReteGraphNestedInAFilter(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	rdfType := rm.JetsResources.Rdf__type
	a, b := rm.NewResource("t:A"), rm.NewResource("t:B")
	hasTag, tagged := rm.NewResource("t:hasTag"), rm.NewResource("t:tagged")
	x, tag := rm.NewResource("t:x"), rm.NewTextLiteral("tag")
	filter := &writingFilter{s: x, p: rdfType, o: b, nested: 3}

	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv2 := NewNodeVertex(1, head, false, 100, nil, "v2", []string{"r2"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?y"}))
	nv1 := NewNodeVertex(2, head, false, 100, filter, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?x"}))
	nv3 := NewNodeVertex(3, nv2, false, 100, nil, "v3", []string{"r2"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcTriple | 2}, []string{"?y", "?t"}))
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?y"}, &FConstant{node: rdfType}, &FConstant{node: b}, nv2, true, "(?y rdf:type B)"),
		NewAlphaNode(&FVariable{variable: "?x"}, &FConstant{node: rdfType}, &FConstant{node: a}, nv1, true, "(?x rdf:type A)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: hasTag}, &FVariable{variable: "?t"}, nv3, true, "(?y hasTag ?t)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: tagged}, &FBinded{pos: 1}, nv3, false, "(?y tagged ?t)"),
	}
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an, RetractionMode{})
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, rdf.NewMetaRdfGraph(rm), nil, alphaNodes,
		[]*NodeVertex{head, nv2, nv1, nv3}, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := rdf.NewRdfSession(rm, ms.MetaGraph)
	sess.Insert(x, rdfType, a)
	sess.Insert(x, hasTag, tag)
	rs := NewReteSession(sess)
	rs.Initialize(ms)
	defer rs.Done()
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	if filter.evals == 0 {
		t.Fatal("the filter was not evaluated")
	}
	if filter.reachedV != 1 {
		t.Errorf("the nested visit left %d rows at v3 during the filter, want 1", filter.reachedV)
	}
	if !filter.stackOK {
		t.Errorf("the nested visit changed the outer visit's stack")
	}
	if !sess.InferredGraph.Contains(x, tagged, tag) {
		t.Errorf("(x tagged tag) was not inferred")
	}
	if rs.visitStack.Len() != 0 {
		t.Errorf("visit stack after ExecuteRules: %v, want empty", *rs.visitStack)
	}
}

// A visit that returns an error leaves the stack as it found it.
func TestVisitReteGraphErrorLeavesTheStack(t *testing.T) {
	n := newBenchNetwork(t)
	rs := NewReteSession(n.newSession(8))
	rs.Initialize(n.ms)
	defer rs.Done()
	outer := IntStack{4, 1}
	rs.visitStack = &outer
	rs.betaRelations[1] = nil // the head's first child
	if err := rs.VisitReteGraph(0, true); err == nil {
		t.Fatal("no error for a vertex with no beta relation")
	}
	if got := *rs.visitStack; !slices.Equal(got, IntStack{4, 1}) {
		t.Errorf("visit stack after the error: %v, want [4 1]", got)
	}
}
