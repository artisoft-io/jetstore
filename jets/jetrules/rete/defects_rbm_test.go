package rete

// DEFECTS READ FROM THE CODE, RUN (rules_benchmark Phase 1, tasks AF.3 and AE.5).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these defects from
// the code and registered them in its plan, §10.1, without running them. Each test
// here asserts TODAY'S behaviour, so it passes now and fails when the defect is
// fixed: a failure after a fix is the signal to flip the assertion, not a
// regression. The comment on each test says what the correct behaviour is.
//
// Two tests are opt-in, with RBM_DEFECTS=1:
//   - TestDefect_I2_NegationFilterTiming, which runs a network a thousand times;
//   - TestDefect_I5_SharedMetaStoreWrites under -race, where the race detector
//     fails it on purpose until I-5 is fixed.

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func rbmDefectsOptIn() bool { return os.Getenv("RBM_DEFECTS") == "1" }

// negationFilterNet is the network of TestDefect_I2_NegationFilterTiming:
//
//	v1  (?s rdf:type State)
//	v2  not(?s hasX ?x).[(?s exist flag)]  ->  (?s result ok)
type negationFilterNet struct {
	rm                              *rdf.ResourceManager
	metaGraph                       *rdf.RdfGraph
	ms                              *ReteMetaStore
	state, hasX, flag, result, okay *rdf.Node
}

func newNegationFilterNet(t *testing.T) *negationFilterNet {
	t.Helper()
	n := &negationFilterNet{rm: rdf.NewResourceManager(nil)}
	rm := n.rm
	jr := rm.JetsResources
	n.metaGraph = rdf.NewMetaRdfGraph(rm)
	n.state, n.hasX, n.flag = rm.NewResource("State"), rm.NewResource("hasX"), rm.NewResource("flag")
	n.result, n.okay = rm.NewResource("result"), rm.NewResource("ok")

	hasFlag := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewExistOp(false), NewExprCst(n.flag))
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?s"}))
	nv2 := NewNodeVertex(2, nv1, true, 100, hasFlag, "v2", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0}, []string{"?s"}))
	nodeVertices := []*NodeVertex{head, nv1, nv2}
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: jr.Rdf__type}, &FConstant{node: n.state}, nv1, true, "(?s rdf:type State)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: n.hasX}, &FVariable{variable: "?x"}, nv2, true, "not(?s hasX ?x)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: n.result}, &FConstant{node: n.okay}, nv2, false, "(?s result ok)"),
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
	ms, err := NewReteMetaStore(rm, n.metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	n.ms = ms
	return n
}

// run is one session: (s hasX x1) is asserted, the rules run, then (s flag 1) is
// asserted, which calls TripleUpdatedForFilter on v2, and the rules run again. It
// reports whether (s result ok) was inferred.
func (n *negationFilterNet) run(t *testing.T) bool {
	sess := rdf.NewRdfSession(n.rm, n.metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(n.ms)
	defer rs.Done()
	srm := sess.ResourceMgr
	s := srm.NewResource("s")
	sess.Insert(s, n.rm.JetsResources.Rdf__type, n.state)
	sess.Insert(s, n.hasX, srm.NewResource("x1"))
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	if sess.Contains(s, n.result, n.okay) {
		t.Fatalf("(s result ok) inferred by the first ExecuteRules: the negation's blocking path is wrong too")
	}
	sess.Insert(s, n.flag, srm.NewIntLiteral(1))
	if err := rs.ExecuteRules(); err != nil {
		t.Fatal(err)
	}
	return sess.Contains(s, n.result, n.okay)
}

// TestDefect_I2_NegationFilterTiming reproduces rules_benchmark I-2: a negated
// antecedent whose filter is replayed through TripleUpdatedForFilter decides
// "no matching triple" by a non-blocking receive on the iterator's channel
// (rete_session_triple_updated.go:163-166). The channel is fed by goroutines
// started a moment earlier (rdf_session_iterator.go:34), so the receive almost
// always finds nothing yet, and the negation is taken as met.
//
// The network: v2 is not(?s hasX ?x) with the filter [(?s exist flag)], whose
// callback on flag (ExistOp.RegisterCallback, expr_operator_r_exist_not.go:23)
// replays v2 through TripleUpdatedForFilter when (s flag 1) is inserted. s has
// hasX x1, so the negation is false and (s result ok) must not be inferred. The
// first ExecuteRules, through VisitReteGraph, gets this right; the replay does not.
//
// Correct behaviour: the replay reads the iterator (blocking) before deciding, so
// (s result ok) is never inferred: 0 of n. Today's: inferred in most runs.
//
// Opt-in with RBM_DEFECTS=1, because the outcome depends on scheduling: if a run
// ever got the right answer on every try, the test would fail without any fix.
func TestDefect_I2_NegationFilterTiming(t *testing.T) {
	if !rbmDefectsOptIn() {
		t.Skip("timing-dependent: set RBM_DEFECTS=1 to run I-2's reproduction")
	}
	const runs = 1000
	n := newNegationFilterNet(t)
	wrong := 0
	for i := 0; i < runs; i++ {
		if n.run(t) {
			wrong++
		}
	}
	t.Logf("I-2: (s result ok) wrongly inferred in %d of %d sessions (%.1f%%)", wrong, runs, 100*float64(wrong)/runs)
	if wrong == 0 {
		t.Errorf("the negation was right in all %d sessions; I-2 may be fixed, flip this test", runs)
	}
}

// TestDefect_I3_EraseIndex3LeavesEntry reproduces rules_benchmark I-3:
// EraseIndex4BetaRow passes an.Fu.Eval for all three positions when all three are
// bound (alpha_node.go:212), so it erases the key (u, u, u) and the row stays
// indexed under (u, v, w). A parent row that was removed is then still found by
// FindMatchingRows3 when a matching triple arrives, and a child row is built on a
// row that no longer exists.
//
// Reachability: the case needs an antecedent whose subject, predicate and object
// are all variables bound by earlier terms, such as (?a p ?b).(?b q ?c).(?a ?b ?c).
// A text scan of the four workspaces on 2026-10-20 found no such term among the
// 9,601 antecedent terms of 2,781 rules (54 have the subject and a variable
// predicate bound, none the object as well). So the path is real and unreached.
//
// Correct behaviour: EraseIndex3(key, Fu, Fv, Fw), and FindMatchingRows3 returns
// no row after the erase.
func TestDefect_I3_EraseIndex3LeavesEntry(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	a, b, c := rm.NewResource("a"), rm.NewResource("b"), rm.NewResource("c")

	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 1, brcTriple | 2}, []string{"?a", "?b", "?c"}))
	nv2 := NewNodeVertex(2, nv1, false, 100, nil, "v2", []string{"r1"},
		NewBetaRowInitializer([]int{brcParentNode | 0, brcParentNode | 1, brcParentNode | 2}, []string{"?a", "?b", "?c"}))
	an := NewAlphaNode(&FBinded{pos: 0}, &FBinded{pos: 1}, &FBinded{pos: 2}, nv2, true, "(?a ?b ?c)")

	parent := NewBetaRelation(nv1)
	an.InitializeIndexes(parent)
	row := NewBetaRow(nv1, 3)
	row.Data[0], row.Data[1], row.Data[2] = a, b, c

	an.AddIndex4BetaRow(parent, row)
	key := nv2.AntecedentQueryKey
	if !parent.FindMatchingRows3(key, a, b, c)[row] {
		t.Fatalf("AddIndex4BetaRow did not index the row under (a, b, c)")
	}
	an.EraseIndex4BetaRow(parent, row)
	if !parent.FindMatchingRows3(key, a, b, c)[row] {
		t.Errorf("the row is no longer indexed under (a, b, c) after EraseIndex4BetaRow; I-3 is fixed, flip this test")
	}
}

// sortedHeadNet is the network of TestDefect_I5_SharedMetaStoreWrites, built to put
// an operator that writes its own fields in InitializeOperator into a shared meta
// store:
//
//	v1  (?s rdf:type State).[(?s exist priorityList)]
//	      ->  (?s topMatch (?s sorted_head cfg))
type sortedHeadNet struct {
	rm                      *rdf.ResourceManager
	metaGraph               *rdf.RdfGraph
	ms                      *ReteMetaStore
	op                      *SortedHeadOp
	state, priority, weight *rdf.Node
	top                     *rdf.Node
}

func newSortedHeadNet(t *testing.T) *sortedHeadNet {
	t.Helper()
	n := &sortedHeadNet{rm: rdf.NewResourceManager(nil)}
	rm := n.rm
	jr := rm.JetsResources
	n.metaGraph = rdf.NewMetaRdfGraph(rm)
	n.state, n.priority = rm.NewResource("State"), rm.NewResource("priorityList")
	n.weight, n.top = rm.NewResource("weight"), rm.NewResource("topMatch")
	cfg := rm.NewResource("cfg")
	n.metaGraph.Insert(cfg, jr.Jets__entity_property, n.priority)
	n.metaGraph.Insert(cfg, jr.Jets__value_property, n.weight)
	n.metaGraph.Insert(cfg, jr.Jets__operator, rm.NewTextLiteral("<"))

	hasPriority := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewExistOp(false), NewExprCst(n.priority))
	n.op = NewSortedHeadOp().(*SortedHeadOp)
	sortedHead := NewExprBinaryOp(NewExprBindedVar(0, "?s"), n.op, NewExprCst(cfg))
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, hasPriority, "v1", []string{"r1"},
		NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?s"}))
	nodeVertices := []*NodeVertex{head, nv1}
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: jr.Rdf__type}, &FConstant{node: n.state}, nv1, true, "(?s rdf:type State)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: n.top}, &FExpression{expression: sortedHead}, nv1, false, "(?s topMatch sorted_head)"),
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
	ms, err := NewReteMetaStore(rm, n.metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	n.ms = ms
	return n
}

// session runs one session on the shared meta store, as a pool worker does, and
// returns the topMatch it inferred, and whether it is m1, the lighter of two items.
func (n *sortedHeadNet) session() (*rdf.Node, bool, error) {
	sess := rdf.NewRdfSession(n.rm, n.metaGraph) // writes metaGraph.isLocked
	rs := NewReteSession(sess)
	rs.Initialize(n.ms) // InitializeExpression -> SortedHeadOp.InitializeOperator
	defer rs.Done()
	srm := sess.ResourceMgr
	s := srm.NewResource("s")
	sess.Insert(s, n.rm.JetsResources.Rdf__type, n.state)
	for i, name := range []string{"m1", "m2"} {
		m := srm.NewResource(name)
		sess.Insert(s, n.priority, m)
		sess.Insert(m, n.weight, srm.NewIntLiteral(i+1))
	}
	if err := rs.ExecuteRules(); err != nil {
		return nil, false, err
	}
	top := sess.GetObject(s, n.top)
	return top, top == srm.NewResource("m1"), nil
}

// TestDefect_I5_SharedMetaStoreWrites reproduces rules_benchmark I-5: every rete
// session writes to the ReteMetaStore that all sessions of a pool share, with no
// lock.
//
//   - ReteSession.Initialize calls InitializeExpression on the shared expressions
//     (rete_session.go:159, :166), which calls the shared operator's
//     InitializeOperator (ExprBinaryOp.InitializeExpression, expression.go:130).
//     SortedHeadOp.InitializeOperator replaces its minMaxOp with a new MinMaxOp
//     whose objProperty and dataProperty are nil (expr_operator_math_sorted_head.go:30)
//     and only then sets them (:34, MinMaxOp.InitializeOperator,
//     expr_operator_math_minmax.go:24). MinMaxOp, SumValuesOp and JoinValuesOp write
//     their own fields the same way.
//   - NewRdfSession sets isLocked on the shared meta graph (rdf_session.go:27).
//
// The first half is deterministic and always runs: a second session's Initialize
// writes a new minMaxOp into the operator the first session is using.
//
// The second half, opt-in with RBM_DEFECTS=1, runs 1,000 sessions on ten workers,
// as a pool of 10 does, and counts wrong answers. They happen: a session whose
// Eval reads minMaxOp between the two writes above calls FindSP(s, nil)
// (MinMaxOp.Eval, expr_operator_math_minmax.go:83), so sorted_head takes the head
// of ALL of s's objects and answers State, the object of (s rdf:type State), instead
// of m1. Under -race the same half also fails on the detector's reports, which is
// the point of running it there. It is opt-in because the wrong-answer rate depends
// on scheduling and can be zero on a quiet run.
//
// Correct behaviour: the meta store is written once, when the rules load, and is
// read-only afterwards. A second session's Initialize leaves the operator's fields
// as they were, every concurrent session answers m1, and -race reports nothing.
func TestDefect_I5_SharedMetaStoreWrites(t *testing.T) {
	n := newSortedHeadNet(t)

	// Deterministic half: a session's Initialize writes the shared operator.
	if top, isM1, err := n.session(); err != nil || !isM1 {
		t.Fatalf("first session: topMatch %v, err %v; want m1", top, err)
	}
	firstMinMax := n.op.minMaxOp
	if _, _, err := n.session(); err != nil {
		t.Fatal(err)
	}
	if n.op.minMaxOp == firstMinMax {
		t.Errorf("a second session left SortedHeadOp.minMaxOp unchanged; I-5 may be fixed, flip this test")
	}

	if !rbmDefectsOptIn() {
		t.Log("concurrent half skipped: set RBM_DEFECTS=1 (and -race) to run it")
		return
	}
	// Concurrent half: ten workers on one meta store.
	const workers, perWorker = 10, 100
	var wg sync.WaitGroup
	var mu sync.Mutex
	wrong := map[string]int{}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				top, isM1, err := n.session()
				if err != nil {
					t.Error(err)
					return
				}
				if !isM1 {
					mu.Lock()
					wrong[fmt.Sprint(top)]++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	total := 0
	for _, c := range wrong {
		total += c
	}
	t.Logf("I-5: %d of %d concurrent sessions (%d workers, race build %v) answered a topMatch other than m1: %v",
		total, workers*perWorker, workers, rbmRaceEnabled, wrong)
	if total == 0 && !rbmRaceEnabled {
		t.Errorf("every concurrent session answered m1; I-5 may be fixed (or this run was lucky): rerun, then flip this test")
	}
}

// TestDefect_I12_LiteralRegexGoMeaning pins the Go half of rules_benchmark I-12
// (F53): Go's literal_regex, ApplyRegexOp.Eval
// (expr_operator_str_apply_regex.go:33), returns the WHOLE match through
// FindString, and an empty string when nothing matches. C++'s RegexVisitor
// (jets/rete/expr_op_strings.h:30) returns capture group 1, and null when nothing
// matches; jetrules_native_adaptor's TestDefect_I12_LiteralRegexEngines runs both
// engines through a compiled rule.
//
// The patterns are the five rules' that use it: CM's z004 "^(\d\d\d)" (usi_ws
// 1_CM01_Mapping_SM_Main2.jr:70), and walrus_ws's two name splitters
// (merge_claim_header_and_details_main.jr:129-130).
//
// Correct behaviour, as Q-11 decided for Phase 4: C++'s, so "123", null, "JOHN"
// and "DOE" below.
func TestDefect_I12_LiteralRegexGoMeaning(t *testing.T) {
	op := NewApplyRegexOp()
	cases := []struct {
		text, pattern, today, correct string
	}{
		{"12345", `^(\d\d\d)`, "123", "123"},         // agree: the group is the whole pattern
		{"N/A", `^(\d\d\d)`, "", "<null>"},           // Go "" (then a GEOZIP lookup), C++ no claimZip3
		{"DOE, JOHN", `,\s+(\w+)`, ", JOHN", "JOHN"}, // walrus pt_fname
		{"DOE, JOHN", `(.*?),`, "DOE,", "DOE"},       // walrus pt_lname
	}
	for _, c := range cases {
		got := op.Eval(nil, nil, rdf.S(c.text), rdf.S(c.pattern))
		if got == nil {
			t.Errorf("%q literal_regex %q = nil, want %q today", c.text, c.pattern, c.today)
			continue
		}
		if got.Value != c.today {
			t.Errorf("%q literal_regex %q = %q, want %q today (correct, as on C++: %s); I-12 may be fixed, flip this case",
				c.text, c.pattern, got.Value, c.today, c.correct)
		}
	}
}
