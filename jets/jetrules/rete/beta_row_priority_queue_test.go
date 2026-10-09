package rete

import (
	"container/heap"
	"testing"
)

// The activation queue must pop the same order as the C++ engine's
// BetaRowPriorityCompare (jets/rete/rete_session.h): higher salience first, and at
// equal salience the lower vertex first, whatever the order the rows were pushed in.
func TestBetaRowPriorityQueueOrder(t *testing.T) {
	row := func(vertex, salience int) *BetaRow {
		return NewBetaRow(NewNodeVertex(vertex, nil, false, salience, nil, "", nil, nil), 0)
	}
	pq := make(BetaRowPriorityQueue, 0)
	heap.Init(&pq)
	// Ten rows of one rule (vertex 16), then a row of a later rule (vertex 17) pushed
	// after the first of them fired: the shape of ReversalClaim01 and ReversalClaim10.
	for range 10 {
		heap.Push(&pq, row(16, 100))
	}
	got := []int{heap.Pop(&pq).(*BetaRow).NdVertex.Vertex}
	heap.Push(&pq, row(17, 100))
	heap.Push(&pq, row(3, 60))
	heap.Push(&pq, row(2, 10))
	heap.Push(&pq, row(5, 100))
	for pq.Len() > 0 {
		got = append(got, heap.Pop(&pq).(*BetaRow).NdVertex.Vertex)
	}
	want := []int{16, 5, 16, 16, 16, 16, 16, 16, 16, 16, 16, 17, 3, 2}
	if len(got) != len(want) {
		t.Fatalf("popped %d rows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop order %v, want %v", got, want)
		}
	}
}
