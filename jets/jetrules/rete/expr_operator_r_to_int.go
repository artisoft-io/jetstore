package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_INT unary operator
type ToIntOp struct {

}

func NewToIntOp() UnaryOperator {
	return &ToIntOp{}
}

func (op *ToIntOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToIntOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		return rhs
	case float64:
		return &rdf.Node{Value: int(rhsv)}
	default:
		return nil
	}
}