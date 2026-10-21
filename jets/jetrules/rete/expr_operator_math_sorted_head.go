package rete

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// SORTED_HEAD operator - with truth maintenance
// Returns the entity with the smallest (jets:operator "<") or largest (">") value,
// in one pass. sorted_head builds the same MinMaxOp that min_head_of and
// max_head_of are, choosing min or max from jets:operator; it is the head form the
// C++ engine has: C++ has no min_head_of or max_head_of.
type SortedHeadOp struct {
	minMaxOp *MinMaxOp
}

func NewSortedHeadOp() BinaryOperator {
	return &SortedHeadOp{}
}

func (op *SortedHeadOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	jr := metaGraph.RootRm.JetsResources
	// Get the operator for sorted_head (it's either '<' or '>')
	operator := metaGraph.GetObject(rhs, jr.Jets__operator)
	if operator == nil {
		return fmt.Errorf("error: sorted_head operator misconfigured, missing jets:operator configuration")
	}
	operatorValue, ok := operator.Value.(string)
	if !ok {
		return fmt.Errorf("error: sorted_head operator misconfigured, jets:operator argument must be a string literal")
	}
	// Built in full before it is published: a MinMaxOp assigned first and configured
	// after was evaluated half-built by concurrent sessions (rules_benchmark I-5, F84).
	minMaxOp := &MinMaxOp{
		isMin:  operatorValue == "<",
		retObj: true,
	}
	if err := minMaxOp.InitializeOperator(metaGraph, lhs, rhs); err != nil {
		return err
	}
	op.minMaxOp = minMaxOp
	return nil
}

// Add truth maintenance
// Delegate to MinMaxOp
func (op *SortedHeadOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	return op.minMaxOp.RegisterCallback(reteSession, vertex, lhs, rhs)
}
func (op *SortedHeadOp) String() string {
	return "sorted_head"
}

// Delegate to MinMaxOp
func (op *SortedHeadOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	return op.minMaxOp.Eval(reteSession, row, lhs, rhs)
}
