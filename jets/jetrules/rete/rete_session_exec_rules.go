package rete

import (
	"fmt"
	"runtime/debug"
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
			return nil
		}
		parentVertex := stack.Pop()
		parentVertexNode := rs.GetNodeVertex(parentVertex)
		parentBetaRelation := rs.GetBetaRelation(parentVertex)
		if parentBetaRelation == nil {
			return fmt.Errorf("error: got nil parentBetaRelation at vertex %d (VisitReteGraph)", parentVertex)
		}

		var itor BetaRowIterator
		allParentBetaRowItor := NewBetaRowSetIterator(parentBetaRelation.AllRows)
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
				} else {
					// for each t3Itor.Itor create the beta row, keep it if pass filter, add/remove row when infer/retract

				}

			}

		}

	}
}
