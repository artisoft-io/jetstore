package rete

// RETRACTION REPLAYS WHAT A MINTING CONSEQUENT INFERRED -- and only that.
//
// The Go half of jets/rete/retract_recorded_consequents_test.cc, with the same network
// and the same cases; read that file's header. In short: retracting a create_entity 0
// row minted a second entity and left its link in place, and a keyed create_entity
// re-inserted jets:key on every retraction. Found by the usi_ws workspace assessment
// (jetstore_agentic_ai, projects/workspace_assessments/02_usi_ws_assessment, Q-37 /
// R-13). The cases marked LEGACY pin two neighbouring defects that are kept by
// Michel's decision of 2026-10-09, because usi_ws's rules terminate only because of
// them: an aggregate consequent is recomputed on retraction, and a withdrawn row never
// fires again. See README.md.
//
// The network, built by hand as the C++ fixture is:
//
//	vertex 1  (?s rdf:type MainEntity).[(?s exist_not done)]  -> consequent under test
//	vertex 2  (?s pendingDone ?o)                            -> (?s done ?o)
//	vertex 3  (?s pendingSupport ?o)                         -> (?s hasSupport ?o)
//
// Salience 10, 5 and 7: vertex 1 infers first, vertex 3 then changes the aggregate's
// input, and vertex 2 retracts vertex 1's row last. The cycle cases drive the
// retraction from the test instead, inserting (main1 done 1) into the inferred graph
// and retracting it between ExecuteRules calls -- the same driver as the C++ cases,
// where RDFGraph::erase does not notify the graph callbacks.

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

type consequentKind int

const (
	kCreateEntityUuid  consequentKind = iota // (?s link create_entity(0))
	kCreateEntityKeyed                       // (?s link create_entity("K1"))
	kSumValues                               // (?s total (?s sum_values sumConfig))
	kPure                                    // (?s link ?s)
)

type retractFixture struct {
	t   *testing.T
	ms  *ReteMetaStore
	rdf *rdf.RdfSession
	rs  *ReteSession
}

func newRetractFixture(t *testing.T, kind consequentKind) *retractFixture {
	t.Helper()
	rm := rdf.NewResourceManager(nil)
	jr := rm.JetsResources
	metaGraph := rdf.NewMetaRdfGraph(rm)

	me := rm.NewResource("MainEntity")
	done := rm.NewResource("done")
	pendingDone := rm.NewResource("pendingDone")
	has := rm.NewResource("hasSupport")
	value := rm.NewResource("value")
	pendingSupport := rm.NewResource("pendingSupport")
	link := rm.NewResource("link")
	total := rm.NewResource("total")
	sumConfig := rm.NewResource("sumConfig")
	metaGraph.Insert(sumConfig, jr.Jets__entity_property, has)
	metaGraph.Insert(sumConfig, jr.Jets__value_property, value)

	ri1 := func() *BetaRowInitializer {
		return NewBetaRowInitializer([]int{brcTriple | 0}, []string{"?s"})
	}
	ri2 := func() *BetaRowInitializer {
		return NewBetaRowInitializer([]int{brcTriple | 0, brcTriple | 2}, []string{"?s", "?o"})
	}

	// [(?s exist_not done)]
	notDone := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewExistOp(true), NewExprCst(done))

	head := NewNodeVertex(0, nil, false, 100, nil, "head", nil, nil)
	nv1 := NewNodeVertex(1, head, false, 10, notDone, "v1", []string{"v1"}, ri1())
	nv2 := NewNodeVertex(2, head, false, 5, nil, "v2", []string{"v2"}, ri2())
	nv3 := NewNodeVertex(3, head, false, 7, nil, "v3", []string{"v3"}, ri2())
	nodeVertices := []*NodeVertex{head, nv1, nv2, nv3}

	alphaNodes := []*AlphaNode{
		NewRootAlphaNode(head),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: jr.Rdf__type}, &FConstant{node: me}, nv1, true, "(?s rdf:type MainEntity)"),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: pendingDone}, &FVariable{variable: "?o"}, nv2, true, "(?s pendingDone ?o)"),
		NewAlphaNode(&FVariable{variable: "?s"}, &FConstant{node: pendingSupport}, &FVariable{variable: "?o"}, nv3, true, "(?s pendingSupport ?o)"),
	}
	var consequent *AlphaNode
	switch kind {
	case kCreateEntityUuid, kCreateEntityKeyed:
		arg := NewExprCst(rdf.I(0))
		if kind == kCreateEntityKeyed {
			arg = NewExprCst(rm.NewTextLiteral("K1"))
		}
		expr := NewExprUnaryOp(NewCreateEntityOp(), arg)
		consequent = NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: link}, &FExpression{expression: expr}, nv1, false, "(?s link create_entity)")
	case kSumValues:
		expr := NewExprBinaryOp(NewExprBindedVar(0, "?s"), NewSumValuesOp(), NewExprCst(sumConfig))
		consequent = NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: total}, &FExpression{expression: expr}, nv1, false, "(?s total sum_values)")
	case kPure:
		consequent = NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: link}, &FBinded{pos: 0}, nv1, false, "(?s link ?s)")
	}
	alphaNodes = append(alphaNodes,
		consequent,
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: done}, &FBinded{pos: 1}, nv2, false, "(?s done ?o)"),
		NewAlphaNode(&FBinded{pos: 0}, &FConstant{node: has}, &FBinded{pos: 1}, nv3, false, "(?s hasSupport ?o)"),
	)
	// The wiring NewReteMetaStoreFactory does
	for _, an := range alphaNodes {
		switch {
		case an.IsAntecedent:
			an.NdVertex.ParentNodeVertex.AddChildAlphaNode(an)
		case an.IsConsequent:
			an.NdVertex.AddConsequentTerm(an)
		}
	}
	config := map[string]string{}
	ms, err := NewReteMetaStore(rm, metaGraph, nil, alphaNodes, nodeVertices, &config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	sess := rdf.NewRdfSession(rm, metaGraph)
	rs := NewReteSession(sess)
	rs.Initialize(ms)
	t.Cleanup(rs.Done)

	// main1 with three supports valued 1, 2 and 3: the sum is 6.
	srm := sess.ResourceMgr
	main1 := srm.NewResource("main1")
	sess.Insert(main1, jr.Rdf__type, me)
	for i, name := range []string{"support1", "support2", "support3"} {
		s := srm.NewResource(name)
		sess.Insert(main1, has, s)
		sess.Insert(s, value, srm.NewIntLiteral(i+1))
	}
	return &retractFixture{t: t, ms: ms, rdf: sess, rs: rs}
}

func (f *retractFixture) r(name string) *rdf.Node {
	return f.rdf.ResourceMgr.NewResource(name)
}

func (f *retractFixture) execute() {
	f.t.Helper()
	if err := f.rs.ExecuteRules(); err != nil {
		f.t.Fatalf("ExecuteRules: %v", err)
	}
}

func (f *retractFixture) countLinks() int {
	n := 0
	for range f.rdf.FindSP(f.r("main1"), f.r("link")).Itor {
		n++
	}
	return n
}

// Every jets:key triple in the inferred graph: one per entity create_entity minted
// and has not retracted.
func (f *retractFixture) countInferredKeys() int {
	jetsKey := f.rdf.ResourceMgr.JetsResources.Jets__key
	n := 0
	for t3 := range f.rdf.InferredGraph.Find().Itor {
		if t3[1] == jetsKey {
			n++
		}
	}
	return n
}

func (f *retractFixture) k1KeyRefCount() int {
	rm := f.rdf.ResourceMgr
	return f.rdf.InferredGraph.GetRefCount(f.r("K1"), rm.JetsResources.Jets__key, rm.NewTextLiteral("K1"))
}

func (f *retractFixture) contains(s, p string, o *rdf.Node) bool {
	return f.rdf.Contains(f.r(s), f.r(p), o)
}

func TestRetractOnlyMintingConsequentsRecord(t *testing.T) {
	for _, c := range []struct {
		kind consequentKind
		want bool
	}{{kCreateEntityUuid, true}, {kCreateEntityKeyed, true}, {kSumValues, false}, {kPure, false}} {
		f := newRetractFixture(t, c.kind)
		if got := f.ms.NodeVertices[1].RecordsConsequents; got != c.want {
			t.Errorf("kind %d: vertex 1 RecordsConsequents = %v, want %v", c.kind, got, c.want)
		}
		if f.ms.NodeVertices[2].RecordsConsequents || f.ms.NodeVertices[3].RecordsConsequents {
			t.Errorf("kind %d: a vertex that mints nothing records", c.kind)
		}
	}
}

// The control: nothing retracts, so the link and one jets:key are there.
func TestRetractCreateEntityUuidInfersOneEntity(t *testing.T) {
	f := newRetractFixture(t, kCreateEntityUuid)
	f.execute()
	if n := f.countLinks(); n != 1 {
		t.Errorf("links = %d, want 1", n)
	}
	if n := f.countInferredKeys(); n != 1 {
		t.Errorf("jets:key triples = %d, want 1", n)
	}
}

// THE DEFECT: before the fix the link stayed and a second entity's jets:key appeared.
func TestRetractCreateEntityUuidRetractedLeavesNothing(t *testing.T) {
	f := newRetractFixture(t, kCreateEntityUuid)
	f.rdf.Insert(f.r("main1"), f.r("pendingDone"), f.rdf.ResourceMgr.NewIntLiteral(1))
	f.execute()
	if !f.contains("main1", "done", f.rdf.ResourceMgr.NewIntLiteral(1)) {
		t.Fatal("the retraction driver did not fire")
	}
	if n := f.countLinks(); n != 0 {
		t.Errorf("links = %d, want 0: the link inferred is the link retracted", n)
	}
	if n := f.countInferredKeys(); n != 0 {
		t.Errorf("jets:key triples = %d, want 0: no second entity, and the first went with its row", n)
	}
}

func TestRetractCreateEntityKeyedReleasesKey(t *testing.T) {
	f := newRetractFixture(t, kCreateEntityKeyed)
	f.rdf.Insert(f.r("main1"), f.r("pendingDone"), f.rdf.ResourceMgr.NewIntLiteral(1))
	f.execute()
	if n := f.countLinks(); n != 0 {
		t.Errorf("links = %d, want 0", n)
	}
	if rc := f.k1KeyRefCount(); rc != 0 {
		t.Errorf("K1 jets:key ref count = %d, want 0", rc)
	}
}

// Repeated cycles on the keyed entity. LEGACY: once retracted the row never fires
// again, though its support comes back. THE FIX: each later withdrawal retracts that
// stale row again and it replays its now empty record, so jets:key stays at 0 -- before
// the record was kept empty it fell back to recomputing and re-inserted jets:key.
func TestRetractCreateEntityKeyedCyclesStayRetracted(t *testing.T) {
	f := newRetractFixture(t, kCreateEntityKeyed)
	one := f.rdf.ResourceMgr.NewIntLiteral(1)
	f.execute()
	if f.countLinks() != 1 || f.k1KeyRefCount() != 1 {
		t.Fatalf("initial inference: links %d, ref count %d", f.countLinks(), f.k1KeyRefCount())
	}
	for cycle := 0; cycle < 5; cycle++ {
		f.rdf.InsertInferred(f.r("main1"), f.r("done"), one)
		f.execute()
		if n, rc := f.countLinks(), f.k1KeyRefCount(); n != 0 || rc != 0 {
			t.Errorf("cycle %d withdrawn: links %d, ref count %d, want 0 and 0", cycle, n, rc)
		}
		f.rdf.Retract(f.r("main1"), f.r("done"), one)
		f.execute()
		if n, rc := f.countLinks(), f.k1KeyRefCount(); n != 0 || rc != 0 {
			t.Errorf("cycle %d re-admitted: links %d, ref count %d, want 0 and 0 (LEGACY: does not re-fire)", cycle, n, rc)
		}
	}
}

func TestRetractCreateEntityUuidCyclesLeaveNoStrayKeys(t *testing.T) {
	f := newRetractFixture(t, kCreateEntityUuid)
	one := f.rdf.ResourceMgr.NewIntLiteral(1)
	f.execute()
	for cycle := 0; cycle < 5; cycle++ {
		f.rdf.InsertInferred(f.r("main1"), f.r("done"), one)
		f.execute()
		if n, k := f.countLinks(), f.countInferredKeys(); n != 0 || k != 0 {
			t.Errorf("cycle %d withdrawn: links %d, keys %d, want 0 and 0", cycle, n, k)
		}
		f.rdf.Retract(f.r("main1"), f.r("done"), one)
		f.execute()
		if n, k := f.countLinks(), f.countInferredKeys(); n != 0 || k != 0 {
			t.Errorf("cycle %d re-admitted: links %d, keys %d, want 0 and 0 (LEGACY: does not re-fire)", cycle, n, k)
		}
	}
}

// LEGACY: a consequent aggregate whose input changes between inference and retraction.
// The row infers (main1 total 6), vertex 3 links support4 so the sum is 10, then vertex
// 2 retracts the row. Retraction recomputes, retracts (main1 total 10), which was never
// inferred, and leaves 6 standing.
func TestRetractConsequentAggregateRetractionRecomputes(t *testing.T) {
	f := newRetractFixture(t, kSumValues)
	rm := f.rdf.ResourceMgr
	f.rdf.Insert(f.r("support4"), f.r("value"), rm.NewIntLiteral(4))
	f.rdf.Insert(f.r("main1"), f.r("pendingSupport"), f.r("support4"))
	f.rdf.Insert(f.r("main1"), f.r("pendingDone"), rm.NewIntLiteral(1))
	f.execute()
	if !f.contains("main1", "hasSupport", f.r("support4")) {
		t.Fatal("the aggregate's input did not change before the retraction")
	}
	if !f.contains("main1", "total", rm.NewIntLiteral(6)) {
		t.Error("(main1 total 6) is gone: aggregate consequents have been given replay -- read README.md first")
	}
	if f.contains("main1", "total", rm.NewIntLiteral(10)) {
		t.Error("(main1 total 10) is standing")
	}
}

func TestRetractPureConsequentStillRetracts(t *testing.T) {
	f := newRetractFixture(t, kPure)
	f.rdf.Insert(f.r("main1"), f.r("pendingDone"), f.rdf.ResourceMgr.NewIntLiteral(1))
	f.execute()
	if n := f.countLinks(); n != 0 {
		t.Errorf("links = %d, want 0", n)
	}
}

// LEGACY, on a consequent the recording does not touch: a retracted row is left in its
// relation marked kProcessed, because RemoveBetaRow returns at once for a row marked
// kDeleted, so an equal row arriving later is taken for one already inferred.
func TestRetractPureConsequentDoesNotRefire(t *testing.T) {
	f := newRetractFixture(t, kPure)
	one := f.rdf.ResourceMgr.NewIntLiteral(1)
	f.execute()
	if f.countLinks() != 1 {
		t.Fatalf("initial inference: links %d", f.countLinks())
	}
	f.rdf.InsertInferred(f.r("main1"), f.r("done"), one)
	f.execute()
	if n := f.countLinks(); n != 0 {
		t.Errorf("withdrawn: links %d, want 0", n)
	}
	f.rdf.Retract(f.r("main1"), f.r("done"), one)
	f.execute()
	if n := f.countLinks(); n != 0 {
		t.Errorf("re-admitted: links %d, want 0 (LEGACY: does not re-fire)", n)
	}
}
