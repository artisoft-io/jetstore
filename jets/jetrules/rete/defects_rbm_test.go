package rete

// DEFECTS READ FROM THE CODE, RUN (rules_benchmark Phase 1, tasks AF.3 and AE.5).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these defects from
// the code and registered them in its plan, §10.1, without running them. Each test
// here asserts TODAY'S behaviour, so it passes now and fails when the defect is
// fixed: a failure after a fix is the signal to flip the assertion, not a
// regression. The comment on each test says what the correct behaviour is.
//
// A defect that has been fixed keeps its test, turned into a regression test that
// asserts the correct behaviour: I-3 and I-5 (with I-24) since rules_benchmark Phase 2
// (AH.1, AH.3, AH.5), and I-2 since G1 (AI.1). None of the tests here is opt-in
// any more: I-2's was, while its outcome depended on goroutine timing.

import (
	"fmt"
	"sync"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// negationFilterNet is the network of TestRegression_I2_NegationFilter:
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

// TestRegression_I2_NegationFilter was TestDefect_I2_NegationFilterTiming, which
// reproduced rules_benchmark I-2 (F81): a negated antecedent whose filter is
// replayed through TripleUpdatedForFilter decided "no matching triple" by a
// non-blocking receive on the iterator's channel. The channel was fed by goroutines
// started a moment earlier, so the receive almost always found nothing yet, and the
// negation was taken as met: wrongly, in 1,000 of 1,000 sessions on 2026-10-20.
//
// The network: v2 is not(?s hasX ?x) with the filter [(?s exist flag)], whose
// callback on flag (ExistOp.RegisterCallback, expr_operator_r_exist_not.go:23)
// replays v2 through TripleUpdatedForFilter when (s flag 1) is inserted. s has
// hasX x1, so the negation is false and (s result ok) must not be inferred.
//
// G1 made the iterator synchronous and the replay asks it IsEmpty(), so the answer
// no longer depends on scheduling, and this test runs by default: (s result ok) is
// inferred in 0 of n sessions. It was opt-in (RBM_DEFECTS=1) only because its
// outcome depended on timing.
func TestRegression_I2_NegationFilter(t *testing.T) {
	const runs = 200
	n := newNegationFilterNet(t)
	wrong := 0
	for i := 0; i < runs; i++ {
		if n.run(t) {
			wrong++
		}
	}
	if wrong > 0 {
		t.Errorf("I-2: (s result ok) wrongly inferred in %d of %d sessions, want 0", wrong, runs)
	}
}

// TestDefect_I3_EraseIndex3LeavesEntry was the reproduction of rules_benchmark I-3,
// and is its regression test since Phase 2 (AH.5) fixed it: EraseIndex4BetaRow passed
// an.Fu.Eval for all three positions when all three are bound, so it erased the key
// (u, u, u) and the row stayed indexed under (u, v, w). A parent row that was removed
// was then still found by FindMatchingRows3 when a matching triple arrived, and a
// child row was built on a row that no longer existed.
//
// Reachability: the case needs an antecedent whose subject, predicate and object
// are all variables bound by earlier terms, such as (?a p ?b).(?b q ?c).(?a ?b ?c).
// A text scan of the four workspaces on 2026-10-20 found no such term among the
// 9,601 antecedent terms of 2,781 rules (54 have the subject and a variable
// predicate bound, none the object as well). So the path was real and unreached.
//
// Correct behaviour, asserted here: EraseIndex3(key, Fu, Fv, Fw), and
// FindMatchingRows3 returns no row after the erase.
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
	if parent.FindMatchingRows3(key, a, b, c)[row] {
		t.Errorf("the row is still indexed under (a, b, c) after EraseIndex4BetaRow (I-3)")
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

// TestDefect_I5_SharedMetaStoreWrites was the reproduction of rules_benchmark I-5,
// and is its regression test since Phase 2 (AH.1, AH.2) fixed it. Every rete session
// wrote to the ReteMetaStore that all sessions of a pool share, with no lock:
//
//   - ReteSession.Initialize called InitializeExpression on the shared expressions,
//     which called the shared operator's InitializeOperator
//     (ExprBinaryOp.InitializeExpression, expression.go). SortedHeadOp.InitializeOperator
//     replaced its minMaxOp with a new MinMaxOp whose objProperty and dataProperty
//     were nil, and only then set them (expr_operator_math_sorted_head.go,
//     MinMaxOp.InitializeOperator). A session whose Eval read minMaxOp between the
//     two writes called FindSP(s, nil) (MinMaxOp.Eval), so sorted_head took the head
//     of ALL of s's objects and answered State, the object of (s rdf:type State),
//     instead of m1: 98 to 176 of 1,000 sessions on ten workers (F84).
//   - NewRdfSession set isLocked on the shared meta graph (rdf_session.go), and
//     NewResourceManager on the shared root manager (resource_manager.go).
//
// Correct behaviour, asserted here: the operators are initialised once per meta
// store (ReteMetaStore.initializeExpressions), so a second session's Initialize
// leaves the operator's fields as they were; every concurrent session answers m1;
// and under -race the detector reports nothing, which fails the test if it does.
func TestDefect_I5_SharedMetaStoreWrites(t *testing.T) {
	n := newSortedHeadNet(t)

	// Deterministic half: a session's Initialize does not write the shared operator.
	if top, isM1, err := n.session(); err != nil || !isM1 {
		t.Fatalf("first session: topMatch %v, err %v; want m1", top, err)
	}
	firstMinMax := n.op.minMaxOp
	if _, _, err := n.session(); err != nil {
		t.Fatal(err)
	}
	if n.op.minMaxOp != firstMinMax {
		t.Errorf("a second session replaced SortedHeadOp.minMaxOp (I-5)")
	}

	// Concurrent half: ten workers on one meta store, as a pool of 10 runs.
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
	if total != 0 {
		t.Errorf("%d of %d concurrent sessions answered a topMatch other than m1 (I-5)", total, workers*perWorker)
	}
}

// panickingOp is a binary operator whose InitializeOperator panics.
type panickingOp struct{ SortedHeadOp }

func (op *panickingOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	panic("misconfigured operator")
}

// TestInitializeExpressionsPanicFailsEverySession: an operator whose initialisation
// panics fails every session's Initialize, not only the first. sync.Once counts a
// panicking function as done, so without the recover in initializeExpressions the
// second session would run on half-initialised operators with no error.
func TestInitializeExpressionsPanicFailsEverySession(t *testing.T) {
	n := newSortedHeadNet(t)
	bad := NewExprBinaryOp(NewExprBindedVar(0, "?s"), &panickingOp{}, NewExprCst(n.top))
	n.ms.AlphaNodes[1].NdVertex.FilterExpr = bad
	for i := 0; i < 2; i++ {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("session %d: Initialize did not panic on a panicking operator", i)
				}
			}()
			rs := NewReteSession(rdf.NewRdfSession(n.rm, n.metaGraph))
			rs.Initialize(n.ms)
		}()
	}
	if n.ms.exprErr == nil {
		t.Errorf("exprErr is nil after a panicking initialisation")
	}
}

// TestAntecedentQueryKeysAreAssignedOnce is rules_benchmark I-24's regression test:
// NewReteMetaStore assigns each antecedent term's AntecedentQueryKey, and a session's
// NewBetaRelation adds the same queries in the same order and writes nothing to the
// shared NodeVertex. Three children of one parent, two querying one bound position
// and one querying two, have keys 0, 1 and 0.
func TestAntecedentQueryKeysAreAssignedOnce(t *testing.T) {
	rm := rdf.NewResourceManager(nil)
	p, q, r := rm.NewResource("p"), rm.NewResource("q"), rm.NewResource("r")
	two := NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?a", "?b"})
	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 100, nil, "v1", []string{"r1"}, two)
	nv2 := NewNodeVertex(2, nv1, false, 100, nil, "v2", []string{"r1"}, two)
	nv3 := NewNodeVertex(3, nv1, false, 100, nil, "v3", []string{"r2"}, two)
	nv4 := NewNodeVertex(4, nv1, false, 100, nil, "v4", []string{"r3"}, two)
	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?a"}, &FConstant{node: p}, &FVariable{variable: "?b"}, nv1, true, "(?a p ?b)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: q}, &FVariable{variable: "?c"}, nv2, true, "(?a q ?c)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: r}, &FBinded{pos: 1}, nv3, true, "(?a r ?b)"),
		NewAlphaNode(&FVariable{variable: "?c"}, &FConstant{node: q}, &FBinded{pos: 1}, nv4, true, "(?c q ?b)"),
	}
	for _, an := range alphaNodes[1:] {
		an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
	}
	config := map[string]string{}
	if _, err := NewReteMetaStore(rm, rdf.NewMetaRdfGraph(rm), nil, alphaNodes,
		[]*NodeVertex{head, nv1, nv2, nv3, nv4}, &config, nil, nil); err != nil {
		t.Fatal(err)
	}
	want := map[*NodeVertex]int{nv2: 0, nv3: 0, nv4: 1}
	for nv, key := range want {
		if !nv.queryKeyAssigned || nv.AntecedentQueryKey != key {
			t.Errorf("vertex %d: AntecedentQueryKey %d (assigned %v), want %d", nv.Vertex, nv.AntecedentQueryKey, nv.queryKeyAssigned, key)
		}
	}
	// Two sessions' beta relations: the same indexes, and the keys untouched (a
	// mismatch panics in InitializeIndexes).
	for i := 0; i < 2; i++ {
		br := NewBetaRelation(nv1)
		if len(br.rowIndexes1) != 2 || len(br.rowIndexes2) != 1 || len(br.rowIndexes3) != 0 {
			t.Errorf("session %d: indexes %d/%d/%d, want 2/1/0", i, len(br.rowIndexes1), len(br.rowIndexes2), len(br.rowIndexes3))
		}
	}
	for nv, key := range want {
		if nv.AntecedentQueryKey != key {
			t.Errorf("vertex %d: AntecedentQueryKey %d after two sessions, want %d", nv.Vertex, nv.AntecedentQueryKey, key)
		}
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
