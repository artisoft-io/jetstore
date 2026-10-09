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
	// recordingRow is the beta row whose inferred triples ComputeConsequentTriples is
	// recording, nil otherwise
	recordingRow *BetaRow
}

// InsertInferredFor inserts a triple inferred by row's consequent terms. When row is
// the row ComputeConsequentTriples is recording (its vertex is flagged
// RecordsConsequents), a triple that reached the inferred graph -- one whose reference
// count this insert raised -- is also recorded on the row, so that retracting the row
// retracts exactly it. Called for the consequent triple itself and by create_entity for
// the entity's jets:key triple. Called with any other row, e.g. a candidate row a
// filter is evaluated on, it only inserts.
func (rs *ReteSession) InsertInferredFor(row *BetaRow, s, p, o *rdf.Node) (bool, error) {
	if o == nil {
		// as RdfSession.InsertInferred does
		o = rdf.Null()
	}
	sess := rs.RdfSession
	// Same test InsertInferred applies: a triple in the meta or asserted graph is not
	// inserted and its reference count is not raised, so there is nothing to retract.
	counted := s != nil && p != nil &&
		!sess.MetaGraph.Contains(s, p, o) && !sess.AssertedGraph.Contains(s, p, o)
	inserted, err := sess.InsertInferred(s, p, o)
	if err != nil {
		return inserted, err
	}
	if counted && row != nil && row == rs.recordingRow {
		*row.inferred = append(*row.inferred, rdf.Triple{s, p, o})
	}
	return inserted, nil
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
	// At equal salience the lower vertex goes first, as BetaRowPriorityCompare does in the
	// C++ engine (jets/rete/rete_session.h). Without the tie-break the heap pops a row
	// just pushed ahead of rows queued earlier at the same salience, so a rule taking an
	// aggregate (max_of, size_of, ...) could fire before rules compiled ahead of it had
	// supplied all its values, and the two engines would disagree on the same rules.
	a, b := (*pq)[i].NdVertex, (*pq)[j].NdVertex
	if a.Salience == b.Salience {
		return a.Vertex < b.Vertex
	}
	return a.Salience > b.Salience
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
