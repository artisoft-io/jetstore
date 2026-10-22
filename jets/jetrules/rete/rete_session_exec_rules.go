package rete

import (
	"container/heap"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func (rs *ReteSession) ExecuteRules() (err error) {
	// log.Println("Entering ReteSession.ExecuteRules")
	defer func() {
		if r := recover(); r != nil {
			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("ExecuteRules: recovered error: %v\n", r))
			buf.WriteString(string(debug.Stack()))
			err = errors.New(buf.String())
			log.Println(err)
		}
	}()

	err = rs.VisitReteGraph(0, true)
	if err != nil {
		return err
	}
	return rs.ComputeConsequentTriples()
}

// Simple stack of int, FILO
type IntStack []int

func NewIntStack(reserve int) *IntStack {
	s := make(IntStack, 0, reserve)
	return &s
}

func (s IntStack) Len() int {
	return len(s)
}

func (s *IntStack) Push(x int) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	*s = append(*s, x)
}

func (s *IntStack) Pop() int {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

func (rs *ReteSession) VisitReteGraph(fromVertex int, isInferring bool) error {
	// log.Println("Entering ReteSession.VisitReteGraph @ vertex",fromVertex)
	// The visit stack is the session's, reused across calls (rules_benchmark G6). A visit
	// started inside another -- by a callback, on an insert made during the visit --
	// works above the outer visit's entries and ends when it is back down to them.
	if rs.visitStack == nil {
		rs.visitStack = NewIntStack(rs.ms.NbrVertices())
	}
	stack := rs.visitStack
	base := stack.Len()
	// The candidates a scan holds are the session's too (heldCandidates), released
	// when the scan ends, and here when the visit ends early or panics.
	heldBase := len(rs.heldCandidates)
	defer func() {
		*stack = (*stack)[:base]
		rs.releaseHeldCandidates(heldBase)
	}()
	stack.Push(fromVertex)
	// idebug := 0
	for {
		if stack.Len() == base {
			// Main exit point
			// log.Println("Exiting ReteSession.VisitReteGraph @ vertex",fromVertex)
			return nil
		}
		parentVertex := stack.Pop()
		parentVertexNode := rs.GetNodeVertex(parentVertex)
		parentBetaRelation := rs.GetBetaRelation(parentVertex)
		if parentBetaRelation == nil {
			return fmt.Errorf("error: got nil parentBetaRelation at vertex %d (VisitReteGraph)", parentVertex)
		}
		// //**
		// log.Printf("Pop parent vertex %d, stack len is %d", parentVertex, stack.Len())

		var itor BetaRowIterator
		var allParentBetaRowItor BetaRowIterator
		pendingParentBetaRowItor := NewBetaRowSliceIterator(parentBetaRelation.pendingRows)

		for i, childAlphaNode := range parentVertexNode.ChildAlphaNodes {

			// Compute beta relation between `parent_vertex` and `child_vertex`
			if childAlphaNode == nil {
				log.Panic("childAlphaNode IS NIL!!!!!")
			}
			if childAlphaNode.NdVertex == nil {
				log.Panic("childAlphaNode.NdVertex IS NIL!!!!! AT", i)
			}
			childVertex := childAlphaNode.NdVertex.Vertex
			childBetaRelation := rs.GetBetaRelation(childVertex)
			if childBetaRelation == nil {
				return fmt.Errorf("error: got nil childBetaRelation at vertex %d (VisitReteGraph)", childVertex)
			}

			// Clear the pending rows in current_relation, since they were for the last pass
			childBetaRelation.ClearPendingRows()

			// Get an iterator over all applicable rows from the parent beta node
			if !childBetaRelation.IsActivated {
				// //**
				// log.Printf("VisitReteGraph @ <%d|%d> all rows (%d rows :: %d keys)", parentVertex, childVertex, parentBetaRelation.AllRows.Size(), len(parentBetaRelation.AllRows.data))
				// Need all rows
				if allParentBetaRowItor == nil {
					allParentBetaRowItor = NewBetaRowSetIterator(parentBetaRelation.AllRows)
				}
				itor = allParentBetaRowItor
			} else {
				// //**
				// log.Printf("VisitReteGraph @ <%d|%d> pending rows only", parentVertex, childVertex)
				itor = pendingParentBetaRowItor
			}
			// A filter evaluated in this scan can re-enter the session, and the
			// re-entry can remove a parent row the scan's candidates were built from,
			// or change what another candidate's filter reads (rules_benchmark I-91).
			// A relation that is activated takes its candidates at once, each checked
			// against the scan's mark (keepCandidate): a re-entry reaches what it holds
			// already. A relation's first scan is not reached by a re-entry, since the
			// relation is not activated yet, so it holds its candidates until the scan
			// ends, and scans again from the parent's rows as they are now if the
			// session was re-entered meanwhile (scanCandidate, heldCandidates,
			// insertHeldCandidates). Only a filter can re-enter, so a vertex without
			// one inserts at once.
			holding := isInferring && !childBetaRelation.IsActivated && childAlphaNode.NdVertex.HasExpression()
			heldBase := len(rs.heldCandidates)
			betaRowInitializer := childAlphaNode.NdVertex.RowInitializer
			for rescans := 0; ; rescans++ {
				mark := rs.reentries
				itor.Reset()

				// process rows from parent beta node:
				// for each BetaRow of parent beta node, compute the inferred/retracted BetaRow for childBetaRelation
				for !itor.IsEnd() {
					// for each triple from the rdf graph matching the AlphaNode
					// compute the BetaRow to infer or retract
					parentBetaRow := itor.GetRow()
					t3Itor := childAlphaNode.FindMatchingTriples(rs, parentBetaRow)

					// Process the returned iterator t3_itor accordingly if AlphaNode is a negation
					if childAlphaNode.NdVertex.IsNegation {
						// if t3Itor is empty then create the beta row
						if t3Itor.IsEmpty() {
							// Got no triples, condition met; create the beta row
							childBetaRow := NewBetaRow(childAlphaNode.NdVertex, betaRowInitializer.RowSize())
							// initialize the beta row with parent_row and place holder for t3
							t3 := rdf.NilTriple()
							err := childBetaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
							if err != nil {
								return fmt.Errorf("while initializing BetaRow with NilTriple: %v", err)
							}
							// evaluate the current_relation filter if any, and insert or remove
							// the row from current_relation based on is_inferring
							rs.scanCandidate(childAlphaNode, childBetaRow, parentBetaRelation, parentBetaRow, &t3,
								childBetaRelation, isInferring, holding, mark)
						}
					} else {
						// for each triple of t3Itor create the beta row, keep it if pass filter, add/remove row when infer/retract
						for t3 := range t3Itor.All() {
							// Create the beta row
							childBetaRow := NewBetaRow(childAlphaNode.NdVertex, betaRowInitializer.RowSize())
							// initialize the beta row with parent_row and t3
							childBetaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
							// evaluate the current_relation filter if any, and insert or remove
							// the row from current_relation based on is_inferring
							rs.scanCandidate(childAlphaNode, childBetaRow, parentBetaRelation, parentBetaRow, &t3,
								childBetaRelation, isInferring, holding, mark)
						}
					}
					itor.Next()
				}
				if !holding || rs.reentries == mark {
					break
				}
				// Re-entered during the first scan: what it held may rest on a removed row
				// or on a filter whose answer changed. Scan again.
				rs.releaseHeldCandidates(heldBase)
				if rescans == maxReentryRetries {
					return fmt.Errorf("error: the first scan of vertex %d was re-entered by its filters %d times in a row (VisitReteGraph)",
						childVertex, maxReentryRetries+1)
				}
				allParentBetaRowItor = NewBetaRowSetIterator(parentBetaRelation.AllRows)
				itor = allParentBetaRowItor
			}
			if holding {
				rs.insertHeldCandidates(heldBase, childBetaRelation)
			}

			// Mark current beta node as activated (if was not already) and push it on the stack so to visit it's childrens
			childBetaRelation.IsActivated = true
			stack.Push(childVertex)
			// //**
			// idebug += 1
			// log.Printf("Pushed child vertex %d, stack len is now %d", childVertex, stack.Len())
			// if idebug == 200 {
			// 	log.Panic("That's enough!")
			// }
		}
		// Clear the pending rows of parent node since we propagated to all it's children
		parentBetaRelation.ClearPendingRows()
	}
}

func (rs *ReteSession) ComputeConsequentTriples() error {
	for {
		if rs.pendingComputeConsequent.Len() == 0 {
			// Main exit point
			return nil
		}
		betaRow, ok := heap.Pop(rs.pendingComputeConsequent).(*BetaRow)
		if !ok {
			return fmt.Errorf("error: heap.Pop(rs.pendingComputeConsequent) did not return the expected *BetaRow type")
		}
		if betaRow.IsProcessed() {
			// Already processed
			continue
		}
		vertex := betaRow.NdVertex.Vertex
		// get the beta node and the vertex_node associated with the beta_row
		betaRelation := rs.GetBetaRelation(vertex)
		if betaRelation == nil {
			return fmt.Errorf("error: got nil beta relation for vertex %d", vertex)
		}
		//*TODO Log infer/retract event here to trace inferrence process (aka explain why)
		//*TODO Track how many times a rule infer/retract triples here (aka rule stat collector)

		// Check for max visit allowed for a vertex
		currentVisit := &rs.VertexVisits[vertex]
		if rs.maxVertexVisits > 0 && currentVisit.InferCount >= rs.maxVertexVisits {
			// Max vertex visit reached, return error
			rs.maxVertexVisitReached = true
			return fmt.Errorf("error: max vertex visit reached")
		}

		if betaRow.IsInserted() {
			// Infer consequent triples
			currentVisit.InferCount += 1
			// A vertex whose consequents mint a resource records what this row infers,
			// including the jets:key triple create_entity inserts, for retraction to replay.
			if betaRow.NdVertex.RecordsConsequents {
				betaRow.startRecording()
				rs.recordingRow = betaRow
			}
			for _, consequentAlphaNode := range betaRow.NdVertex.ConsequentAlphaNodes {
				t3 := consequentAlphaNode.ComputeConsequentTriple(rs, betaRow)
				_, err := rs.InsertInferredFor(betaRow, t3[0], t3[1], t3[2])
				if err != nil {
					rs.recordingRow = nil
					return fmt.Errorf("while calling ReteSession.InsertInferred (ComputeConsequentTriples) @ vertex %d: %v", vertex, err)
				}
			}
			rs.recordingRow = nil
			// Mark row as Processed
			betaRow.Status = kProcessed
		} else {
			// beta_row status must be kDeleted, meaning retracting mode
			if !betaRow.IsDeleted() {
				return fmt.Errorf("error: invalid beta row at vertex %d, expecting status kDeleted (ComputeConsequentTriples)", vertex)
			}
			// Retract consequent triples
			currentVisit.RetractCount += 1
			if betaRow.isRecording() {
				// Replay what the row inferred: no expression is evaluated, so no entity
				// is minted and no jets:key is inserted. The record is emptied, not
				// released: the row stays in its relation (see below) and can be
				// retracted again, and must then replay nothing rather than recompute.
				for _, t3 := range betaRow.takeRecorded() {
					_, err := rs.RdfSession.Retract(t3[0], t3[1], t3[2])
					if err != nil {
						return fmt.Errorf("while calling ReteSession.Retract (ComputeConsequentTriples): %v", err)
					}
				}
			} else {
				// Consequents that mint nothing are recomputed. For one that reads the
				// graph (an aggregate, exist, size_of) that is a known defect -- see
				// README.md, 2026-10-09 -- unless the meta store was built with
				// RetractionMode.ReplayGraphReadingConsequents, when such a vertex
				// records and is replayed above.
				for _, consequentAlphaNode := range betaRow.NdVertex.ConsequentAlphaNodes {
					t3 := consequentAlphaNode.ComputeConsequentTriple(rs, betaRow)
					_, err := rs.RdfSession.Retract(t3[0], t3[1], t3[2])
					if err != nil {
						return fmt.Errorf("while calling ReteSession.Retract (ComputeConsequentTriples): %v", err)
					}
				}
			}
			// Remove row from beta node. KNOWN DEFECT, kept deliberately (2026-10-09):
			// RemoveBetaRow returns at once for a row marked kDeleted -- which this row
			// is -- so the retracted row stays in the relation as kProcessed and an
			// equal row inserted later is taken for one already inferred: it never
			// fires again. A client workspace's rules terminated only because of it; see README.md.
			// A meta store built with RetractionMode.EraseWithdrawnRows, which only the
			// rules unit test harness builds, erases the row instead.
			if rs.ms.RetractionMode.EraseWithdrawnRows {
				betaRelation.EraseRetractedBetaRow(betaRow)
			} else {
				betaRelation.RemoveBetaRow(rs, betaRow)
			}
			betaRow.Status = kProcessed
		}
	}
}
