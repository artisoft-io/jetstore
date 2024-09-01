package rete

import (
	"container/heap"
	"testing"
)

func TestBetaRowPriorityQueue(t *testing.T) {
	pq := make(BetaRowPriorityQueue, 0)
	br0 := NewBetaRow(NewNodeVertex(0, nil, false, 100, nil, "node 0", nil, nil), 0)
	br1 := NewBetaRow(NewNodeVertex(1, nil, false, 90, nil, "node 1", nil, nil), 0)
	br2 := NewBetaRow(NewNodeVertex(2, nil, false, 80, nil, "node 2", nil, nil), 0)
	br3 := NewBetaRow(NewNodeVertex(3, nil, false, 70, nil, "node 3", nil, nil), 0)
	heap.Push(&pq, br3)
	heap.Push(&pq, br0)
	heap.Push(&pq, br1)
	for i := 0; i<4; i++ {
		br := heap.Pop(&pq).(*BetaRow)
		if br.NdVertex.Vertex != i {
			t.Error("expecting br.NdVertex.Vertex == i")
		}
		if i == 1 {
			heap.Push(&pq, br2)
		}
	}
}