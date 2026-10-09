package rete

// THE usi_ws LOOP-CONTROL SHAPE, reduced to five rule terms -- the Go half of
// jets/rete/loop_through_negation_test.cc; read that file's header.
//
//	v3  (?s rdf:type State).(?s matchList ?m).not(?s mergeList ?m) -> (?s priorityList ?m)
//	v4  (?s rdf:type State).(?s loop ?n).[(?s exist priorityList)]
//	      -> (?s adjLoop ?n).(?s topMatch (?s sorted_head cfg))
//	v5  (?s topMatch ?m) -> (?s mergeList ?m)
//
// An odd loop through negation, which terminates in both engines only because an
// aggregate consequent is recomputed on retraction and a withdrawn row never fires
// again -- two known defects kept by Michel's decision of 2026-10-09 (README.md). THIS
// IS A GUARD, NOT A SPECIFICATION: if a change to truth maintenance makes it fail, that
// change needs option (b) or (c) of the README entry, not a new expectation here.

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func TestLoopControlRulesTerminate(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	metaGraph := rdf.NewMetaRdfGraph(rm)

	state := rm.NewResource("State")
	match := rm.NewResource("matchList")
	merge := rm.NewResource("mergeList")
	priority := rm.NewResource("priorityList")
	loop := rm.NewResource("loop")
	adjLoop := rm.NewResource("adjLoop")
	top := rm.NewResource("topMatch")
	weight := rm.NewResource("weight")
	cfg := rm.NewResource("cfg")
	metaGraph.Insert(cfg, jr.Jets__entity_property, priority)
	metaGraph.Insert(cfg, jr.Jets__value_property, weight)
	metaGraph.Insert(cfg, jr.Jets__operator, rm.NewTextLiteral("<"))

	hasPriority := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewExistOp(false), NewExprCst(priority))

	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"v1"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?s"}))
	nv2 := NewNodeVertex(2, nv1, false, 100, nil, "v2", []string{"v2"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcTriple | 2}, []string{"?s", "?m"}))
	nv3 := NewNodeVertex(3, nv2, true, 100, nil, "v3", []string{"v3"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1}, []string{"?s", "?m"}))
	nv4 := NewNodeVertex(4, nv1, false, 60, hasPriority, "v4", []string{"v4"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcTriple | 2}, []string{"?s", "?n"}))
	nv5 := NewNodeVertex(5, head, false, 100, nil, "v5", []string{"v5"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?s", "?m"}))
	nodeVertices := []*NodeVertex{head, nv1, nv2, nv3, nv4, nv5}

	sortedHead := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewSortedHeadOp(), NewExprCst(cfg))
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: jr.Rdf__type}, &FConstant{node: state}, nv1, true, "(?s rdf:type State)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: match}, &FVariable{variable: "?m"}, nv2, true, "(?s matchList ?m)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: merge}, &FBinded{pos: 1}, nv3, true, "not(?s mergeList ?m)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: loop}, &FVariable{variable: "?n"}, nv4, true, "(?s loop ?n)"),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: top}, &FVariable{variable: "?m"}, nv5, true, "(?s topMatch ?m)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: priority}, &FBinded{pos: 1}, nv3, false, "(?s priorityList ?m)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: adjLoop}, &FBinded{pos: 1}, nv4, false, "(?s adjLoop ?n)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: top}, &FExpression{expression: sortedHead}, nv4, false, "(?s topMatch sorted_head)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: merge}, &FBinded{pos: 1}, nv5, false, "(?s mergeList ?m)"),
	}
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an)
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := rdf.NewRdfSession(rm, metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(ms)
	t.Cleanup(rs.Done)

	srm := sess.ResourceMgr
	s := srm.NewResource("s")
	sess.Insert(s, jr.Rdf__type, state)
	for i, name := range []string{"m1", "m2"} {
		m := srm.NewResource(name)
		sess.Insert(s, match, m)
		sess.Insert(m, weight, srm.NewIntLiteral(i+1))
	}

	// The jetrules_pool_worker loop, without the jets:completed rule.
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		sess.Insert(s, loop, srm.NewIntLiteral(i))
		if err := rs.ExecuteRules(); err != nil {
			t.Fatalf("loop %d: the loop rules cycled: %v", i, err)
		}
	}
	has := func(p, o *rdf.Node) bool { return sess.Contains(s, p, o) }
	m1, m2 := srm.NewResource("m1"), srm.NewResource("m2")
	if !has(top, m1) || !has(top, m2) {
		t.Errorf("topMatch m1 %v m2 %v, want both", has(top, m1), has(top, m2))
	}
	if !has(merge, m1) || !has(merge, m2) {
		t.Errorf("a merge was undone: mergeList m1 %v m2 %v", has(merge, m1), has(merge, m2))
	}
	if has(priority, m1) || has(priority, m2) {
		t.Errorf("priorityList m1 %v m2 %v, want neither", has(priority, m1), has(priority, m2))
	}
}
