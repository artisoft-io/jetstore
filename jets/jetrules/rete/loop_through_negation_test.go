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
//
// The guard is the "legacy" subtest. The other three run the same network in the
// retraction modes the rules unit test harness can select (retraction_mode.go), and
// pin what each does to it, as the README measured on the C++ engine on 2026-10-09:
// with both defects turned off ("fixed") the loop rules do not terminate and the run
// ends at the vertex visit limit; with only defect 1 turned off ("fixed-1") they
// terminate and the merges are undone; with only defect 2 turned off ("fixed-2") they
// terminate as in legacy.

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// loopOutcome is what one run of the loop network ended with.
type loopOutcome struct {
	err                    error
	topMatch, merged, prio [2]bool // for m1 and m2
}

func runLoopControlRules(t *testing.T, mode RetractionMode) loopOutcome {
	t.Helper()
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
			an.NdVertex.AddConsequentTerm(an, mode)
		}
	}
	config := map[string]string{"$max_rule_exec": "1000"}
	ms, err := NewReteMetaStore(rm, metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ms.RetractionMode = mode
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
	var out loopOutcome
	out.err = rs.ExecuteRules()
	for i := 1; i <= 3 && out.err == nil; i++ {
		sess.Insert(s, loop, srm.NewIntLiteral(i))
		out.err = rs.ExecuteRules()
	}
	has := func(p, o *rdf.Node) bool { return sess.Contains(s, p, o) }
	for i, name := range []string{"m1", "m2"} {
		m := srm.NewResource(name)
		out.topMatch[i], out.merged[i], out.prio[i] = has(top, m), has(merge, m), has(priority, m)
	}
	return out
}

func TestLoopControlRulesTerminate(t *testing.T) {
	both := [2]bool{true, true}
	neither := [2]bool{false, false}
	for _, mode := range AllRetractionModes() {
		t.Run(mode.Label(), func(t *testing.T) {
			out := runLoopControlRules(t, mode)
			t.Logf("mode %s: err %v, topMatch %v, mergeList %v, priorityList %v",
				mode.Label(), out.err, out.topMatch, out.merged, out.prio)
			if mode.ReplayGraphReadingConsequents && mode.EraseWithdrawnRows {
				// No fixed point under correct truth maintenance.
				if out.err == nil || !strings.Contains(out.err.Error(), "max vertex visit") {
					t.Fatalf("err = %v, want the vertex visit limit: the shape has no fixed point with both defects off", out.err)
				}
				return
			}
			if out.err != nil {
				t.Fatalf("the loop rules cycled: %v", out.err)
			}
			// Legacy and fixed-2: both items merged. Fixed-1: topMatch is replayed away
			// when the priority list empties, which un-merges both -- and the stale v3
			// rows never re-fire, so the list stays empty.
			wantTop, wantMerged := both, both
			if mode.ReplayGraphReadingConsequents {
				wantTop, wantMerged = neither, neither
			}
			if out.topMatch != wantTop {
				t.Errorf("topMatch m1, m2 = %v, want %v", out.topMatch, wantTop)
			}
			if out.merged != wantMerged {
				t.Errorf("mergeList m1, m2 = %v, want %v: a merge was undone or kept", out.merged, wantMerged)
			}
			if out.prio != neither {
				t.Errorf("priorityList m1, m2 = %v, want neither", out.prio)
			}
		})
	}
}
