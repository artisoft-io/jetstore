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
	mark := rs.reentries
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
		if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentRow, &t3, inserting, mark) {
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
	mark := rs.reentries
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
				if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentBetaRow, &t3, true, mark) {
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
				if rs.keepCandidate(alphaNode, betaRow, parentBetaRelation, parentBetaRow, &t3, true, mark) {
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

// A filter can write to the graph -- in a compiled rule set, the jets:exception a
// failed truth test raises -- and the write's callbacks re-enter the rete at once,
// through TripleUpdated or TripleUpdatedForFilter. The re-entry can remove a parent
// row, or a triple, that the scan evaluating the filter is building candidates
// from: those candidates, the one whose filter wrote and the others of the same
// scan, before it or after it, were then inserted from a row or a triple that no
// longer existed, and never retracted, so the session's result depended on the
// order its triples arrived in (rules_benchmark I-91).
//
// So a scan notes rs.reentries when it begins, and a candidate it inserts is
// checked again when the count has moved since (stillHolds). VisitReteGraph holds
// its candidates until the scan ends and checks them then (holdCandidate,
// insertHeldCandidates): a candidate inserted before the re-entry could not be
// withdrawn by it, since the relation it went into is not activated during its
// first scan. TripleUpdated and TripleUpdatedForFilter check each candidate as
// they insert it (keepCandidate): their relation is activated, so a re-entry
// withdraws what they inserted before it. A candidate to be removed needs no
// check: removing a row that is not there does nothing. Without a re-entry the
// cost is one comparison per candidate.

// heldCandidate is a candidate row VisitReteGraph inserts at the end of its scan,
// with what it was built from.
type heldCandidate struct {
	row, parent *BetaRow
	t3          rdf.Triple
}

// passesFilter evaluates the filter of row's vertex, if any.
func (rs *ReteSession) passesFilter(alphaNode *AlphaNode, row *BetaRow) bool {
	return !alphaNode.NdVertex.HasExpression() || alphaNode.NdVertex.FilterExpr.EvalFilter(rs, row)
}

// keepCandidate evaluates the filter of row, a candidate built from parentRow of
// parentRelation and, for a positive antecedent, the triple t3, and reports whether
// the row is kept: a row to insert is checked again when the session was re-entered
// since mark, the count when the caller's scan began.
func (rs *ReteSession) keepCandidate(alphaNode *AlphaNode, row *BetaRow, parentRelation *BetaRelation,
	parentRow *BetaRow, t3 *rdf.Triple, inserting bool, mark uint64) bool {
	if !rs.passesFilter(alphaNode, row) {
		return false
	}
	return !inserting || rs.reentries == mark || rs.stillHolds(alphaNode, parentRelation, parentRow, t3)
}

// stillHolds reports whether a candidate built from parentRow and t3 still holds:
// parentRow is still in parentRelation and not deleted, and the antecedent still
// holds -- for a negation no triple matches, otherwise t3 is still in the graph.
func (rs *ReteSession) stillHolds(alphaNode *AlphaNode, parentRelation *BetaRelation,
	parentRow *BetaRow, t3 *rdf.Triple) bool {
	if p := parentRelation.AllRows.Get(parentRow); p == nil || p.IsDeleted() {
		return false
	}
	if alphaNode.NdVertex.IsNegation {
		return alphaNode.FindMatchingTriples(rs, parentRow).IsEmpty()
	}
	return rs.RdfSession.Contains((*t3)[0], (*t3)[1], (*t3)[2])
}

// holdCandidate holds row, a candidate of VisitReteGraph's current scan that passed
// its filter, until the scan ends.
func (rs *ReteSession) holdCandidate(row, parent *BetaRow, t3 *rdf.Triple) {
	rs.heldCandidates = append(rs.heldCandidates, heldCandidate{row: row, parent: parent, t3: *t3})
}

// insertHeldCandidates inserts into childRelation the candidates the scan held
// above heldBase, each checked again if the session was re-entered since mark, and
// releases them. Inserting a row evaluates nothing, so nothing is held meanwhile.
func (rs *ReteSession) insertHeldCandidates(heldBase int, mark uint64, alphaNode *AlphaNode,
	parentRelation, childRelation *BetaRelation) {
	moved := rs.reentries != mark
	for i := heldBase; i < len(rs.heldCandidates); i++ {
		c := &rs.heldCandidates[i]
		if !moved || rs.stillHolds(alphaNode, parentRelation, c.parent, &c.t3) {
			childRelation.InsertBetaRow(rs, c.row)
		}
		*c = heldCandidate{}
	}
	rs.heldCandidates = rs.heldCandidates[:heldBase]
}
