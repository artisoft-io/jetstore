package rdf

// DEFECTS READ FROM THE CODE, RUN (rules_benchmark Phase 1, task AF.3).
//
// projects/rules_benchmark in artisoft-io/jetstore_agentic_ai read these defects from
// the code and registered them in its plan, §10.1, without running them. Each test
// here asserts TODAY'S behaviour, so it passes now and fails when the defect is
// fixed: a failure after a fix is the signal to flip the assertion, not a
// regression. The comment on each test says what the correct behaviour is.
//
// A defect that has been fixed keeps its test, turned into a regression test that
// asserts the correct behaviour: I-4 since rules_benchmark Phase 2 (AH.5), and I-1
// since G1 (AI.1), which removed the iterator goroutines.

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

// TestRegression_I1_NoIteratorGoroutines was TestDefect_I1_IteratorGoroutineLeak,
// which reproduced rules_benchmark I-1 (F3, F80): an iterator over a bound key that
// was closed before it was read leaked goroutines.
//
// The iterator then ran one goroutine per map level (NewBaseGraphIterator), and the
// single-key stages returned on done WITHOUT closing their output channel, so the
// next stage blocked forever on its range. Measured 2026-10-20 at GOMAXPROCS 1 to
// 16, 2,000 calls each: a base iterator closed before reading leaked about two
// goroutines a call, a session iterator 87 to 426 in all, and GetObject up to 132
// when the meta graph answered first.
//
// G1 made the iterators synchronous, so the leak is gone by construction: there is
// no goroutine to leak. This test now guards that, on the same shapes plus the two
// that G1 added (the pull cursor dropped before its end, and IsEmpty): leaked == 0
// in every case. A failure means something started a goroutine behind an iterator
// again.
func TestRegression_I1_NoIteratorGoroutines(t *testing.T) {
	const n = 2000
	rm := NewResourceManager(nil)
	meta := NewMetaRdfGraph(rm)
	sess := NewRdfSession(rm, meta)
	srm := sess.ResourceMgr
	s, p := srm.NewResource("s"), srm.NewResource("p")
	if _, err := sess.Insert(s, p, srm.NewIntLiteral(1)); err != nil {
		t.Fatal(err)
	}
	// (s p ?) in the inferred graph too, so a later graph has a value for the bound
	// subject when an earlier one has already answered.
	if _, err := sess.InsertInferred(s, p, srm.NewIntLiteral(2)); err != nil {
		t.Fatal(err)
	}

	// Made and never read, on the base graph and on the session.
	base := leakedBy(n, func() { _ = sess.AssertedGraph.FindSP(s, p) })
	session := leakedBy(n, func() { _ = sess.FindSP(s, p) })
	// Stopped after the first triple: a break, GetObject, and Exist.
	brk := leakedBy(n, func() {
		for range sess.FindSP(s, p).All() {
			break
		}
	})
	getObject := leakedBy(n, func() { sess.GetObject(s, p) })
	exist := leakedBy(n, func() { s.Exist(sess, p) })
	// The pull form, dropped after its first triple, and IsEmpty.
	cursor := leakedBy(n, func() {
		c := sess.FindSP(s, p).Cursor()
		c.Next()
	})
	isEmpty := leakedBy(n, func() { sess.FindSP(s, p).IsEmpty() })

	// GetObject answered by the meta graph, which the session reads first.
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

	for name, leaked := range map[string]int{
		"base FindSP never read": base, "session FindSP never read": session,
		"break after the first triple": brk, "GetObject": getObject, "Exist": exist,
		"cursor dropped after the first triple": cursor, "IsEmpty": isEmpty,
		"GetObject answered by meta": getObjectMeta,
	} {
		if leaked > 0 {
			t.Errorf("%s: %d goroutines left after %d calls (GOMAXPROCS %d), want 0",
				name, leaked, n, runtime.GOMAXPROCS(0))
		}
	}
}

// TestDefect_I4_CreateBNodeStoresUnderWrongKey was the reproduction of
// rules_benchmark I-4, and is its regression test since rules_benchmark Phase 2 (AH.5)
// fixed it: CreateBNode stored the node it created under rm.lastBnodeKey instead of
// under the key it was asked for, so a second CreateBNode(key) did not find the first
// one and returned a new node, and the entry a NewBNode had stored under lastBnodeKey
// was overwritten. ReifyResource calls CreateBNode for every blank node an expression
// returns (ReifyResource, resource_manager.go), so two evaluations of one blank node
// gave two different *Node, and nodes are compared by pointer.
//
// Correct behaviour, asserted here: rm.bnodeMap[key] = r, so CreateBNode(k) ==
// CreateBNode(k), GetBNode(k) finds it, and NewBNode's entries are left alone.
func TestDefect_I4_CreateBNodeStoresUnderWrongKey(t *testing.T) {
	rm := NewResourceManager(nil)

	first := rm.CreateBNode(7)
	second := rm.CreateBNode(7)
	if first != second {
		t.Errorf("CreateBNode(7) twice gave two nodes, %p and %p (I-4)", first, second)
	}
	if got := rm.GetBNode(7); got != first {
		t.Errorf("GetBNode(7) = %v, want the CreateBNode(7) node %v (I-4)", got, first)
	}
	if got := rm.GetBNode(0); got != nil {
		t.Errorf("GetBNode(0) = %v, want nil: CreateBNode(7) must not store under lastBnodeKey (I-4)", got)
	}

	// NewBNode stores under lastBnodeKey and then increments it; a CreateBNode in
	// between must not land on the key the next NewBNode uses.
	rm2 := NewResourceManager(nil)
	minted := rm2.NewBNode() // stored under 0, lastBnodeKey becomes 1
	created := rm2.CreateBNode(9)
	next := rm2.NewBNode() // stored under 1
	if rm2.GetBNode(0) != minted || rm2.GetBNode(1) != next {
		t.Errorf("NewBNode entries: GetBNode(0) = %v (want %v), GetBNode(1) = %v (want %v)",
			rm2.GetBNode(0), minted, rm2.GetBNode(1), next)
	}
	if rm2.GetBNode(9) != created {
		t.Errorf("GetBNode(9) = %v, want the CreateBNode(9) node %v (I-4)", rm2.GetBNode(9), created)
	}
	// ReifyResource of one blank node twice gives one node.
	bn := BN(11)
	if rm.ReifyResource(bn) != rm.ReifyResource(bn) {
		t.Errorf("ReifyResource(BN 11) twice gave two nodes (I-4)")
	}
}
