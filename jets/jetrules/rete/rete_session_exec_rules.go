package rete

import (
	"container/heap"
	"fmt"
	"runtime/debug"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

func (rs *ReteSession) ExecuteRules() (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ExecuteRules recovered error: %v", r)
			debug.PrintStack()
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
	stack := NewIntStack(rs.ms.NbrVertices())
	stack.Push(fromVertex)
	for {
		if stack.Len() == 0 {
			// Main exit point
			return nil
		}
		parentVertex := stack.Pop()
		parentVertexNode := rs.GetNodeVertex(parentVertex)
		parentBetaRelation := rs.GetBetaRelation(parentVertex)
		if parentBetaRelation == nil {
			return fmt.Errorf("error: got nil parentBetaRelation at vertex %d (VisitReteGraph)", parentVertex)
		}

		var itor BetaRowIterator
		var allParentBetaRowItor BetaRowIterator
		pendingParentBetaRowItor := NewBetaRowSliceIterator(parentBetaRelation.pendingRows)

		for _, childAlphaNode := range parentVertexNode.ChildAlphaNodes {

			// Compute beta relation between `parent_vertex` and `child_vertex`
			childVertex := childAlphaNode.NdVertex.Vertex
			childBetaRelation := rs.GetBetaRelation(childVertex)
			if childBetaRelation == nil {
				return fmt.Errorf("error: got nil childBetaRelation at vertex %d (VisitReteGraph)", childVertex)
			}

			// Clear the pending rows in current_relation, since they were for the last pass
			childBetaRelation.ClearPendingRows()

			// Get an iterator over all applicable rows from the parent beta node
			if !childBetaRelation.IsActivated {
				// Need all rows
				if allParentBetaRowItor == nil {
					allParentBetaRowItor = NewBetaRowSetIterator(parentBetaRelation.AllRows)
				}
				itor = allParentBetaRowItor
			} else {
				itor = pendingParentBetaRowItor
			}
			itor.Reset()

			// process rows from parent beta node:
			// for each BetaRow of parent beta node, compute the inferred/retracted BetaRow for childBetaRelation
			betaRowInitializer := childAlphaNode.NdVertex.RowInitializer
			for {
				if itor.IsEnd() {
					break
				}
				// for each triple from the rdf graph matching the AlphaNode
				// compute the BetaRow to infer or retract
				parentBetaRow := itor.GetRow()
				t3Itor := childAlphaNode.FindMatchingTriples(rs, parentBetaRow)

				// Process the returned iterator t3_itor accordingly if AlphaNode is a negation
				if childAlphaNode.NdVertex.IsNegation {
					// if t3Itor.Itor is empty then create the beta row
					select {
					case <-t3Itor.Itor:
						// Got a triple, condition not met since it's a negation
					default:
						// Got no triples, condition met; create the beta row
						childBetaRow := NewBetaRow(childAlphaNode.NdVertex, betaRowInitializer.RowSize())
						// initialize the beta row with parent_row and place holder for t3
						t3 := rdf.NilTriple()
						err := childBetaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
						if err != nil {
							t3Itor.Done()
							return fmt.Errorf("while initializing BetaRow with NilTriple: %v", err)
						}
						// evaluate the current_relation filter if any
						keepIt := true
						if childAlphaNode.NdVertex.HasExpression() {
							keepIt = childBetaRow.NdVertex.FilterExpr.EvalFilter(rs, childBetaRow)
						}
						// insert or remove the row from current_relation based on is_inferring
						if keepIt {
							if isInferring {
								// Add row to child beta relation
								childBetaRelation.InsertBetaRow(rs, childBetaRow)
							} else {
								// Remove row from child beta relation
								childBetaRelation.RemoveBetaRow(rs, childBetaRow)
							}
						}
					}
				} else {
					// for each t3Itor.Itor create the beta row, keep it if pass filter, add/remove row when infer/retract
					for t3 := range t3Itor.Itor {
						// Create the beta row
						childBetaRow := NewBetaRow(childAlphaNode.NdVertex, betaRowInitializer.RowSize())
						// initialize the beta row with parent_row and t3
						childBetaRow.Initialize(betaRowInitializer, parentBetaRow, &t3)
						// evaluate the current_relation filter if any
						keepIt := true
						if childAlphaNode.NdVertex.HasExpression() {
							keepIt = childBetaRow.NdVertex.FilterExpr.EvalFilter(rs, childBetaRow)
						}
						// insert or remove the row from current_relation based on is_inferring
						if keepIt {
							if isInferring {
								// Add child beta row to child beta relation
								childBetaRelation.InsertBetaRow(rs, childBetaRow)
							} else {
								// Remove the row from the child beta relation
								childBetaRelation.RemoveBetaRow(rs, childBetaRow)
							}
						}
					}
				}
				t3Itor.Done()
				itor.Next()
			}

			// Mark current beta node as activated (if was not already) and push it on the stack so to visit it's childrens
			childBetaRelation.IsActivated = true
			stack.Push(childVertex)
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
		currentVisit := &rs.vertexVisits[vertex]
		if rs.maxVertexVisits > 0 && currentVisit.inferCount >= rs.maxVertexVisits {
			// Max vertex visit reached, return error
			rs.maxVertexVisitReached = true
			return fmt.Errorf("error: max vertex visit reached")
		}

		if betaRow.IsInserted() {
			// Infer consequent triples
			currentVisit.inferCount += 1
			for _, consequentAlphaNode := range betaRow.NdVertex.ConsequentAlphaNodes {
				t3 := consequentAlphaNode.ComputeConsequentTriple(rs, betaRow)
				_, err := rs.RdfSession.InsertInferred(t3[0], t3[1], t3[2])
				if err != nil {
					return fmt.Errorf("while calling ReteSession.InsertInferred (ComputeConsequentTriples): %v", err)
				}
			}
			// Mark row as Processed
			betaRow.Status = kProcessed
		} else {
			// beta_row status must be kDeleted, meaning retracting mode
			if !betaRow.IsDeleted() {
				return fmt.Errorf("error: invalid beta row at vertex %d, expecting status kDeleted (ComputeConsequentTriples)", vertex)
			}
			// Retract consequent triples
			currentVisit.retractCount += 1
			for _, consequentAlphaNode := range betaRow.NdVertex.ConsequentAlphaNodes {
				t3 := consequentAlphaNode.ComputeConsequentTriple(rs, betaRow)
				_, err := rs.RdfSession.Retract(t3[0], t3[1], t3[2])
				if err != nil {
					return fmt.Errorf("while calling ReteSession.Retract (ComputeConsequentTriples): %v", err)
				}
			}
      // Remove row from beta node
			betaRelation.RemoveBetaRow(rs, betaRow)
			betaRow.Status = kProcessed
		}
	}
}
