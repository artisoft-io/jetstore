package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_DOUBLE unary operator
type ToDoubleOp struct {

}

func NewToDoubleOp() UnaryOperator {
	return &ToDoubleOp{}
}

func (op *ToDoubleOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToDoubleOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		return &rdf.Node{Value: float64(rhsv)}
	case float64:
		return rhs
	default:
		return nil
	}
}