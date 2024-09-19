package rete

import (
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ReteSession functions called by callback manager in response to a triple inserted or deleted from the rdf graph

// This is called by callback manager in response to a triple inserted or deleted from the rdf graph
func (rs *ReteSession) TripleUpdated(vertex int, s, p, o *rdf.Node, isInserted bool) {
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

		// evaluate the nodeVertex filter if any
		keepIt := true
		if nodeVertex.HasExpression() {
			keepIt = nodeVertex.FilterExpr.EvalFilter(rs, betaRow)
		}
		if keepIt {
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
			// if t3Itor.Itor is empty then create the beta row
			select {
			case <-t3Itor.Itor:
				// Got a triple, condition not met since it's a negation
			default:
				// Got no triples, condition met; create the beta row
				betaRow := NewBetaRow(alphaNode.NdVertex, betaRowInitializer.RowSize())
				// initialize the beta row with parent_row and place holder for t3
				t3 := rdf.NilTriple()
				err := betaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
				if err != nil {
					log.Panicf("while initializing BetaRow with NilTriple ()TripleUpdatedForFilter): %v", err)
				}
				// evaluate the current alpha node filter if any
				keepIt := true
				if alphaNode.NdVertex.HasExpression() {
					keepIt = betaRow.NdVertex.FilterExpr.EvalFilter(rs, betaRow)
				}
				if keepIt {
					// Add row to child beta relation
					betaRelation.InsertBetaRow(rs, betaRow)
				}
			}
		} else {
			// for each t3Itor.Itor create the beta row, keep it if pass filter
			for t3 := range t3Itor.Itor {
				// Create the beta row
				betaRow := NewBetaRow(alphaNode.NdVertex, betaRowInitializer.RowSize())
				// initialize the beta row with parent_row and t3
				betaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
				// evaluate the current_relation filter if any
				keepIt := true
				if alphaNode.NdVertex.HasExpression() {
					keepIt = betaRelation.NdVertex.FilterExpr.EvalFilter(rs, betaRow)
				}
				// insert the row to the current beta relation
				if keepIt {
					// Add child beta row to child beta relation
					betaRelation.InsertBetaRow(rs, betaRow)
				}
			}
		}
		t3Itor.Done()
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
