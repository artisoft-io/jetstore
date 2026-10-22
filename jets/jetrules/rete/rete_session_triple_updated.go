package rete

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteSession functions called by callback manager in response to a triple inserted or deleted from the rdf graph

// This is called by callback manager in response to a triple inserted or deleted from the rdf graph
func (rs *ReteSession) TripleUpdated(vertex int, s, p, o *rdf.Node, isInserted bool) {
	rs.reentries++
	if vertex >= len(rs.ms.NodeVertices) {
		log.Panic("ReteSession.TripleUpdated called with invalid vertex:", vertex)
	}
	// //**
	// if vertex == 42 {
	// 	log.Printf("vertex %d TripleUpdated: %s, inserted? %v", vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}), isInserted)
	// }

	// If beta node is not activated yet, ignore the notification
	betaRelation := rs.GetBetaRelation(vertex)
	if !betaRelation.IsActivated {
		// //**
		// if vertex == 42 {
		// 	log.Printf("vertex %d TripleUpdated: %s, !betaRelation.IsActivated - bailing out", vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}))
		// }
		return
	}
	nodeVertex := rs.ms.NodeVertices[vertex]
	parentBetaRelation := rs.GetBetaRelation(nodeVertex.ParentNodeVertex.Vertex)

	// Get an iterator over all applicable rows from the parent beta node
	// which is provided by the alpha node adaptor
	alphaNode := rs.GetAlphaNode(vertex)
	parentBetaRows := alphaNode.FindMatchingRows(parentBetaRelation, s, p, o)
	// //**
	// if vertex == 42 {
	// 	log.Printf("vertex %d TripleUpdated: %s, parentBetaRows == nil ? %v", vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}), parentBetaRows == nil)
	// }
	// for each BetaRow of parent beta node,
	// compute the inferred/retracted BetaRow for the added/retracted triple (s, p, o)
	t3 := rdf.Triple{s, p, o}
	betaRowInitializer := nodeVertex.RowInitializer
	for parentRow, valid := range parentBetaRows {
		if !valid {
			continue
		}
		// create the beta row to add/retract
		betaRow := NewBetaRow(nodeVertex, betaRowInitializer.RowSize())

		// initialize the beta row with parent_row and t3
		betaRow.Initialize(betaRowInitializer, parentRow, &t3)

		// evaluate the nodeVertex filter if any; a row to insert is checked again
		// if the filter re-entered the rete
		inserting := isInserted != nodeVertex.IsNegation
		if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentRow, &t3, inserting) {
			// Add/Remove row to current beta relation (betaRelation)
			switch {
			case (isInserted && !nodeVertex.IsNegation) ||
				(!isInserted && nodeVertex.IsNegation):
				// //**
				// if vertex == 42 {
				// 	log.Printf("vertex %d TripleUpdated: %s, Insert the row and propagate down the network", vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}))
				// }
				// Insert the row and propagate down the network
				betaRelation.InsertBetaRow(rs, betaRow)
				if betaRelation.HasPendingRows() {
					err := rs.VisitReteGraph(vertex, true)
					if err != nil {
						log.Panicf("while calling VisitReteGraph(inferring) from vertex %d: %v", vertex, err)
					}
				}
			case (!isInserted && !nodeVertex.IsNegation) ||
				(isInserted && nodeVertex.IsNegation):
				// //**
				// if vertex == 42 {
				// 	log.Printf("vertex %d TripleUpdated: %s, Remove the row and propagate down the network", vertex, rdf.ToString(&[3]*rdf.Node{s, p, o}))
				// }
				// Remove the row and propagate down the network
				betaRelation.RemoveBetaRow(rs, betaRow)
				if betaRelation.HasPendingRows() {
					err := rs.VisitReteGraph(vertex, false)
					if err != nil {
						log.Panicf("while calling VisitReteGraph(retracting) from vertex %d: %v", vertex, err)
					}
				}
			}
		}
	}
}

// This is called by callback manager in response to a triple inserted or deleted from the rdf graph
// Case for rule filters
// The approach is to replay all the inferrence of this node when a triple is inserted or retracted,
// this is done by first retracting all the beta rows of the current node and then infer based on the
// beta rows of the parent node
// Note: since we replay all the inferrence of this node, only the vertex argument is used.
func (rs *ReteSession) TripleUpdatedForFilter(vertex int, _, _, _ *rdf.Node, _ bool) {
	rs.reentries++
	if vertex >= len(rs.ms.NodeVertices) {
		log.Panic("ReteSession.TripleUpdatedForFilter called with invalid vertex:", vertex)
	}
	// //**
	// log.Println("TripleUpdatedForFilter: called for vertex",vertex)

	// If beta node is not activated yet, ignore the notification
	betaRelation := rs.GetBetaRelation(vertex)
	if !betaRelation.IsActivated {
		return
	}
	nodeVertex := rs.ms.NodeVertices[vertex]
	// make sure this is not the rete head node
	if nodeVertex.IsHead() {
		return
	}
	// Start by retracting all beta rows of current node
	betaRelation.ClearPendingRows()
	if betaRelation.AllRows.Size() > 0 {
		currentRowsItor := NewBetaRowSetIterator(betaRelation.AllRows)
		// Need to pull all the rows out of the iterator since the retract will update the AllRows set
		l := make([]*BetaRow, 0, betaRelation.AllRows.Size())
		for {
			if currentRowsItor.IsEnd() {
				break
			}
			l = append(l, currentRowsItor.GetRow())
			currentRowsItor.Next()
		}
		for _, row := range l {
			betaRelation.RemoveBetaRow(rs, row)
		}
		// Propagate down the network to retract the removed beta rows
		if betaRelation.HasPendingRows() {
			err := rs.VisitReteGraph(vertex, false)
			if err != nil {
				log.Panicf("while calling VisitReteGraph to retract all rows: %v", err)
			}
		}
	}
	// Replay the inference
	// Get an iterator over all rows from the parent beta node
	// which is provided by the alpha node adaptor
	// to replay the inferrence
	parentBetaRelation := rs.GetBetaRelation(nodeVertex.ParentNodeVertex.Vertex)
	if parentBetaRelation.AllRows.Size() == 0 {
		return
	}
	alphaNode := rs.GetAlphaNode(vertex)
	parentRowsItor := NewBetaRowSetIterator(parentBetaRelation.AllRows)
	for !parentRowsItor.IsEnd() {
		// for each triple from the rdf graph matching the AlphaNode
		// compute the BetaRow to infer or retract
		parentBetaRow := parentRowsItor.GetRow()
		t3Itor := alphaNode.FindMatchingTriples(rs, parentBetaRow)

		// Process the returned iterator t3_itor accordingly if AlphaNode is a negation
		betaRowInitializer := alphaNode.NdVertex.RowInitializer
		if alphaNode.NdVertex.IsNegation {
			// if t3Itor is empty then create the beta row; otherwise the condition
			// is not met, since it's a negation.
			//
			// Until rules_benchmark G1 this was a non-blocking read of the iterator's
			// channel, so goroutine timing decided it (I-2): a triple not yet sent
			// read as none, and a closed, empty channel read as a triple.
			if t3Itor.IsEmpty() {
				// Got no triples, condition met; create the beta row
				betaRow := NewBetaRow(alphaNode.NdVertex, betaRowInitializer.RowSize())
				// initialize the beta row with parent_row and place holder for t3
				t3 := rdf.NilTriple()
				err := betaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
				if err != nil {
					log.Panicf("while initializing BetaRow with NilTriple ()TripleUpdatedForFilter): %v", err)
				}
				// evaluate the current alpha node filter if any
				if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentBetaRow, &t3, true) {
					// Add row to child beta relation
					betaRelation.InsertBetaRow(rs, betaRow)
				}
			}
		} else {
			// for each triple of t3Itor create the beta row, keep it if pass filter
			for t3 := range t3Itor.All() {
				// Create the beta row
				betaRow := NewBetaRow(alphaNode.NdVertex, betaRowInitializer.RowSize())
				// initialize the beta row with parent_row and t3
				betaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
				// evaluate the current_relation filter if any, and insert the row
				// to the current beta relation
				if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentBetaRow, &t3, true) {
					// Add child beta row to child beta relation
					betaRelation.InsertBetaRow(rs, betaRow)
				}
			}
		}
		parentRowsItor.Next()
	}
	// Propagate down the rete network
	if betaRelation.HasPendingRows() {
		err := rs.VisitReteGraph(vertex, true)
		if err != nil {
			log.Panicf("while calling VisitReteGraph (TripleUpdatedForFilter): %v", err)
		}
	}
}

// keepCandidate evaluates the filter of row, a candidate built from parentRow of
// parentRelation and, for a positive antecedent, the triple t3. It reports whether
// the row is kept.
//
// A filter can write to the graph -- in a compiled rule set, the jets:exception a
// failed truth test raises -- and the write's callbacks re-enter the rete at once,
// through TripleUpdated or TripleUpdatedForFilter. The re-entry can remove
// parentRow, or make the candidate's own antecedent fail, while the filter is still
// running; a candidate kept then was inserted from a row that no longer exists and
// was never retracted, so the session's result depended on the order its triples
// arrived in (rules_benchmark I-91). So when a candidate to be inserted is kept and
// the session was re-entered during its filter, it is checked again: parentRow is
// still in parentRelation and not deleted, and the antecedent still holds -- for a
// negation no triple matches, otherwise t3 is still in the graph. A candidate to be
// removed needs no check: removing a row that is not there does nothing.
func (rs *ReteSession) keepCandidate(alphaNode *AlphaNode, row *BetaRow, parentRelation *BetaRelation,
	parentRow *BetaRow, t3 *rdf.Triple, inserting bool) bool {
	if !alphaNode.NdVertex.HasExpression() {
		return true
	}
	mark := rs.reentries
	if !alphaNode.NdVertex.FilterExpr.EvalFilter(rs, row) {
		return false
	}
	if !inserting || rs.reentries == mark {
		return true
	}
	if p := parentRelation.AllRows.Get(parentRow); p == nil || p.IsDeleted() {
		return false
	}
	if alphaNode.NdVertex.IsNegation {
		return alphaNode.FindMatchingTriples(rs, parentRow).IsEmpty()
	}
	return rs.RdfSession.Contains((*t3)[0], (*t3)[1], (*t3)[2])
}
