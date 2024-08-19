package rete

import (
	"container/heap"
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteSession type -- main session class for the rete network

type ReteSession struct {
	RdfSession               *rdf.RdfSession
	ms                       *ReteMetaStore
	betaRelations            []*BetaRelation
	vertexVisits             []VisitCount
	pendingComputeConsequent *BetaRowPriorityQueue
	maxVertexVisits          int
	maxVertexVisitReached    bool
}

type VisitCount struct {
	inferCount   int
	retractCount int
}

// Priority queue that implements the heap.Interface
type BetaRowPriorityQueue []*BetaRow

// implementing the heap.Interface
func (pq *BetaRowPriorityQueue) Len() int { return len(*pq) }

func (pq *BetaRowPriorityQueue) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return (*pq)[i].NdVertex.Salience > (*pq)[j].NdVertex.Salience
}

func (pq *BetaRowPriorityQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
}

func (pq *BetaRowPriorityQueue) Push(x any) {
	item := x.(*BetaRow)
	*pq = append(*pq, item)
}

func (pq *BetaRowPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // don't stop the GC from reclaiming the item eventually
	*pq = old[0 : n-1]
	return item
}

// Create an uninitialized the ReteSession
func NewReteSession(rdfSession *rdf.RdfSession) *ReteSession {
	pq := make(BetaRowPriorityQueue, 0)
	return &ReteSession{
		RdfSession:               rdfSession,
		pendingComputeConsequent: &pq,
	}
}

func (rs *ReteSession) Initialize(ms *ReteMetaStore) {
	if ms == nil {
		log.Panic("error: ReteSession.Initialize requires a non nil ReteMetaStore")
	}
	rs.ms = ms
	log.Println("Initializing the ReteSession")

	// Initializing the BetaRelations
	rs.betaRelations = make([]*BetaRelation, len(ms.NodeVertices))
	for i := range ms.NodeVertices {
		nodeVertex := ms.NodeVertices[i]
		bn := NewBetaRelation(nodeVertex)
		if nodeVertex.IsHead() {
			// put an empty BetaRow to kick start the propagation in the rete network
			bn.InsertBetaRow(rs, NewBetaRow(nodeVertex, 0))
		}
		rs.betaRelations[i] = bn
	}

	// Initialize the VertexVisits
	rs.vertexVisits = make([]VisitCount, len(ms.NodeVertices))

	// Get the max_vertex_visit from the meta store properties
	p := (*ms.JetStoreConfig)["$max_looping"]
	if len(p) > 0 && p != "0" {
		c, err := strconv.Atoi(p)
		if err == nil {
			rs.maxVertexVisits = c
		} else {
			log.Printf("JetStoreConfig has invalid $max_looping value: %v", err)
		}
	}

	// Set the callbacks
	for i := range ms.NodeVertices {
		nodeVertex := ms.NodeVertices[i]
		if nodeVertex.HasExpression() {
			// log.Printf("Set Callbacks for vertex %d with filter",  nodeVertex.Vertex)
			nodeVertex.FilterExpr.RegisterCallback(rs, nodeVertex.Vertex)
		}
	}
}

func (rs *ReteSession) ScheduleConsequentTerms(row *BetaRow) {
	if row == nil {
		log.Panic("ReteSession.ScheduleConsequentTerms called with nil BetaRow")
	}
	heap.Push(rs.pendingComputeConsequent, row)
}

func (rs *ReteSession) GetBetaRelation(vertex int) *BetaRelation {
	if vertex >= len(rs.betaRelations) {
		return nil
	}
	return rs.betaRelations[vertex]
}

func (rs *ReteSession) GetNodeVertex(vertex int) *NodeVertex {
	if vertex >= len(rs.ms.NodeVertices) {
		return nil
	}
	return rs.ms.NodeVertices[vertex]
}

func (rs *ReteSession) GetAlphaNode(vertex int) *AlphaNode {
	if vertex >= len(rs.ms.AlphaNodes) {
		return nil
	}
	return rs.ms.AlphaNodes[vertex]
}
