package rete

import (
	"container/heap"
	"log"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteSession type -- main session class for the rete network

type ReteSession struct {
	RdfSession               *rdf.RdfSession
	ms                       *ReteMetaStore
	betaRelations            []*BetaRelation
	VertexVisits             []VisitCount
	pendingComputeConsequent *BetaRowPriorityQueue
	maxVertexVisits          int
	maxVertexVisitReached    bool
}

type VisitCount struct {
	Label        string
	InferCount   int
	RetractCount int
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
		maxVertexVisits:          10000,
	}
}

func (rs *ReteSession) Initialize(ms *ReteMetaStore) {
	if ms == nil {
		log.Panic("error: ReteSession.Initialize requires a non nil ReteMetaStore")
	}
	rs.ms = ms
	// log.Println("Initializing the ReteSession")

	// Initializing the BetaRelations
	// Initialize the VertexVisits
	rs.VertexVisits = make([]VisitCount, len(ms.NodeVertices))
	rs.betaRelations = make([]*BetaRelation, len(ms.NodeVertices))
	for i := range ms.NodeVertices {
		nodeVertex := ms.NodeVertices[i]
		bn := NewBetaRelation(nodeVertex)
		if nodeVertex.IsHead() {
			// put an empty BetaRow to kick start the propagation in the rete network
			bn.InsertBetaRow(rs, NewBetaRow(nodeVertex, 0))
		}
		rs.betaRelations[i] = bn
		rs.VertexVisits[i].Label = strings.Join(nodeVertex.AssociatedRules, ",")
	}

	// Get the max_vertex_visit from the meta store properties
	p := (*ms.JetStoreConfig)["$max_rule_exec"]
	if len(p) > 0 && p != "0" {
		c, err := strconv.Atoi(p)
		if err == nil {
			rs.maxVertexVisits = c
		} else {
			log.Printf("JetStoreConfig has invalid $max_rule_exec: %v, using default value of 10,000", err)
		}
	}
	// Check for a pipeline or rule_config override to $max_rule_exec via jets:max_vertex_visits
	mvv := ms.MetaGraph.GetObject(ms.ResourceMgr.JetsResources.Jets__istate,	ms.ResourceMgr.JetsResources.Jets__max_vertex_visits)
	if mvv != nil {
		c, ok := mvv.Value.(int)
		if ok {
			rs.maxVertexVisits = c
		} else {
			log.Printf("Rule config has invalid jets:max_vertex_visits, expecting an int, using value set in rule file or default")
		}
	}

	// Set the callbacks
	for i := range ms.AlphaNodes {
		alphaNode := ms.AlphaNodes[i]
		if alphaNode.IsAntecedent {
			alphaNode.RegisterCallback(rs)
			nodeVertex := alphaNode.NdVertex
			if nodeVertex.HasExpression() {
				// log.Printf("Set Callbacks for vertex %d with filter",  nodeVertex.Vertex)
				err := nodeVertex.FilterExpr.InitializeExpression(rs)
				if err != nil {
					log.Panicf("configuration error found while initializing rete session: %v", err)
				}
				nodeVertex.FilterExpr.RegisterCallback(rs, nodeVertex.Vertex)
			}
		} else {
			alphaNode.Fw.InitializeExpression(rs)
		}
	}
}

func (rs *ReteSession) Done() {
	rs.RdfSession.AssertedGraph.CallbackMgr.ClearCallbacks()
	rs.RdfSession.InferredGraph.CallbackMgr.ClearCallbacks()
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
