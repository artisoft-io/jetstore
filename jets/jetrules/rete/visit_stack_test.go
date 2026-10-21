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
		for range itor.Itor {
			c++
		}
		itor.Done()
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
