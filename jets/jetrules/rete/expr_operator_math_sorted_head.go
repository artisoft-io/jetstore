package rete

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// SORTED_HEAD operator - DEPRECATED - with truth maintenance
// use min_head_of or max_head_of instead
type SortedHeadOp struct {
	minMaxOp *MinMaxOp
}

func NewSortedHeadOp() BinaryOperator {
	return &SortedHeadOp{}
}

// Add truth maintenance
// Delegate to MinMaxOp
func (op *SortedHeadOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	rdfSession := reteSession.RdfSession
	jr := rdfSession.ResourceMgr.JetsResources
	// Get the operator for sorted_head (it's either '<' or '>')
	operator := rdfSession.GetObject(rhs, jr.Jets__operator)
	if operator == nil {
		return fmt.Errorf("error: sorted_head operator misconfigured, missing jets:operator configuration")
	}
	operatorValue, ok := operator.Value.(string)
	if !ok {
		return fmt.Errorf("error: sorted_head operator misconfigured, jets:operator must be a string literal")
	}
	op.minMaxOp = &MinMaxOp{
		isMin: operatorValue == "<",
		retObj: true,
	}
	return op.minMaxOp.RegisterCallback(reteSession, vertex, lhs, rhs)
}
func (op *SortedHeadOp) String() string {
	return "sorted_head (depricated)"
}

// Delegate to MinMaxOp
func (op *SortedHeadOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	return op.minMaxOp.Eval(reteSession, row, lhs, rhs)
}
