package rdf

// DEFECTS READ FROM THE CODE, RUN (rules_benchmark Phase 1, task AF.3).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these defects from
// the code and registered them in its plan, §10.1, without running them. Each test
// here asserts TODAY'S behaviour, so it passes now and fails when the defect is
// fixed: a failure after a fix is the signal to flip the assertion, not a
// regression. The comment on each test says what the correct behaviour is.

import (
	"runtime"
	"testing"
	"time"
)

// settledGoroutines waits for the goroutine count to stop falling and returns it.
// The iterator goroutines of a finished iteration exit asynchronously, so a count
// taken at once would include goroutines that are about to exit.
func settledGoroutines() int {
	n := runtime.NumGoroutine()
	for stable := 0; stable < 20; {
		time.Sleep(time.Millisecond)
		m := runtime.NumGoroutine()
		if m < n {
			n, stable = m, 0
			continue
		}
		n = m
		stable++
	}
	return n
}

// leakedBy runs f n times and returns how many goroutines are left behind.
func leakedBy(n int, f func()) int {
	before := settledGoroutines()
	for i := 0; i < n; i++ {
		f()
	}
	return settledGoroutines() - before
}

// TestDefect_I1_IteratorGoroutineLeak reproduces rules_benchmark I-1 (F3): an
// iterator over a bound key that is stopped early leaks goroutines.
//
// NewBaseGraphIterator (base_graph_iterator.go:38) runs one goroutine per map
// level. When the subject or the predicate is bound, the first two stages send
// their single value in a select with done, and on done they return WITHOUT
// closing their output channel (base_graph_iterator.go:66 and :92). The next stage
// is ranging over that channel, so it blocks forever, and so does the stage after
// it. The path is taken whenever Done() reaches a stage before it has sent: most
// directly when an iterator is closed before anything is read from it, which is
// what TripleUpdatedForFilter's negation branch does on its default case (I-2,
// rete_session_triple_updated.go:166 then :204).
//
// Measured 2026-10-20 at GOMAXPROCS 1 to 16, 2,000 calls each: a base iterator
// closed before reading leaks about two goroutines a call (3,842 to 3,982), a
// session iterator 87 to 426 in all; GetObject and Exist leak none when the
// asserted graph answers, and up to 132 when the meta graph answers first and the
// other two iterators are closed before they have sent.
//
// Correct behaviour: every stage closes its output channel on every exit (a
// `defer close(...)`), and no goroutine outlives Done(); leaked == 0 in every case
// below. G1, the synchronous iterator, removes the goroutines altogether.
func TestDefect_I1_IteratorGoroutineLeak(t *testing.T) {
	const n = 2000
	rm := NewResourceManager(nil)
	meta := NewMetaRdfGraph(rm)
	sess := NewRdfSession(rm, meta)
	srm := sess.ResourceMgr
	s, p := srm.NewResource("s"), srm.NewResource("p")
	if _, err := sess.Insert(s, p, srm.NewIntLiteral(1)); err != nil {
		t.Fatal(err)
	}
	// (s p ?) in the inferred graph too, so the inferred iterator has a value to
	// send for the bound subject when the asserted one has already answered.
	if _, err := sess.InsertInferred(s, p, srm.NewIntLiteral(2)); err != nil {
		t.Fatal(err)
	}

	// Closed before the first read, on the base graph: stage 1 has a value for the
	// bound subject and finds done.
	base := leakedBy(n, func() {
		itor := sess.AssertedGraph.FindSP(s, p)
		itor.Done()
	})
	// Closed before the first read, on the session: the session's merging goroutine
	// stops and closes its three base iterators before they have sent.
	session := leakedBy(n, func() {
		itor := sess.FindSP(s, p)
		itor.Done()
	})
	// Stopped after the first triple: GetObject, and Exist, which calls it
	// (ast_r_operators.go:246).
	getObject := leakedBy(n, func() { sess.GetObject(s, p) })
	exist := leakedBy(n, func() { s.Exist(sess, p) })

	// GetObject answered by the meta graph, which the session reads first: the
	// asserted and inferred iterators are closed as soon as the first triple is
	// read, possibly before they have sent.
	rm2 := NewResourceManager(nil)
	meta2 := NewMetaRdfGraph(rm2)
	ms, mp := rm2.NewResource("ms"), rm2.NewResource("mp")
	if _, err := meta2.Insert(ms, mp, rm2.NewIntLiteral(0)); err != nil {
		t.Fatal(err)
	}
	sess2 := NewRdfSession(rm2, meta2)
	if _, err := sess2.Insert(ms, mp, sess2.ResourceMgr.NewIntLiteral(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := sess2.InsertInferred(ms, mp, sess2.ResourceMgr.NewIntLiteral(2)); err != nil {
		t.Fatal(err)
	}
	getObjectMeta := leakedBy(n, func() { sess2.GetObject(ms, mp) })

	t.Logf("goroutines left after %d early stops (GOMAXPROCS %d): base FindSP+Done %d, session FindSP+Done %d, GetObject %d, Exist %d, GetObject answered by meta %d",
		n, runtime.GOMAXPROCS(0), base, session, getObject, exist, getObjectMeta)

	// Today's behaviour: closing an iterator before reading it leaks. The counts
	// depend on scheduling, so the assertion is that some leak, not how many.
	if base <= 0 {
		t.Errorf("base FindSP closed before reading left %d goroutines, want > 0 while I-1 is open", base)
	}
	if session <= 0 {
		t.Errorf("session FindSP closed before reading left %d goroutines, want > 0 while I-1 is open", session)
	}
}

// TestDefect_I4_CreateBNodeStoresUnderWrongKey reproduces rules_benchmark I-4:
// CreateBNode stores the node it creates under rm.lastBnodeKey instead of under
// the key it was asked for (resource_manager.go:168).
//
// So a second CreateBNode(key) does not find the first one and returns a new node,
// and the node a NewBNode stored under lastBnodeKey is overwritten. ReifyResource
// calls CreateBNode for every blank node an expression returns
// (resource_manager.go:178), so two evaluations of one blank node give two
// different *Node, and nodes are compared by pointer.
//
// Correct behaviour: rm.bnodeMap[key] = r, so CreateBNode(k) == CreateBNode(k) and
// GetBNode(k) finds it.
func TestDefect_I4_CreateBNodeStoresUnderWrongKey(t *testing.T) {
	rm := NewResourceManager(nil)

	first := rm.CreateBNode(7)
	second := rm.CreateBNode(7)
	if first == second {
		t.Errorf("CreateBNode(7) twice gave one node; I-4 is fixed, flip this test")
	}
	if got := rm.GetBNode(7); got != nil {
		t.Errorf("GetBNode(7) = %v, want nil while I-4 is open (the node is stored under lastBnodeKey)", got)
	}
	// It is stored under lastBnodeKey, which is 0 for a fresh manager.
	if got := rm.GetBNode(0); got != second {
		t.Errorf("GetBNode(0) = %v, want the last CreateBNode(7) node %v while I-4 is open", got, second)
	}

	// And it overwrites what NewBNode stored there: NewBNode stores under
	// lastBnodeKey and then increments it, so the next CreateBNode lands on the key
	// the next NewBNode will use.
	rm2 := NewResourceManager(nil)
	minted := rm2.NewBNode() // stored under 0, lastBnodeKey becomes 1
	rm2.CreateBNode(9)       // stored under 1
	next := rm2.NewBNode()   // stored under 1, replacing the CreateBNode(9) entry
	if rm2.GetBNode(0) != minted || rm2.GetBNode(1) != next {
		t.Errorf("NewBNode entries: GetBNode(0) = %v (want %v), GetBNode(1) = %v (want %v)",
			rm2.GetBNode(0), minted, rm2.GetBNode(1), next)
	}
	if rm2.GetBNode(9) != nil {
		t.Errorf("GetBNode(9) found the CreateBNode(9) node; I-4 is fixed, flip this test")
	}
	// ReifyResource of one blank node twice gives two nodes.
	bn := BN(11)
	if rm.ReifyResource(bn) == rm.ReifyResource(bn) {
		t.Errorf("ReifyResource(BN 11) twice gave one node; I-4 is fixed, flip this test")
	}
}
