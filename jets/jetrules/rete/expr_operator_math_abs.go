package rete

import (
	"math"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add unary operator
type AbsOp struct {

}

func NewAbsOp() UnaryOperator {
	return &AbsOp{}
}

func (op *AbsOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *AbsOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		if rhsv < 0 {
			return &rdf.Node{Value: -rhsv}
		}
		return &rdf.Node{Value: rhsv}
	case float64:
		return &rdf.Node{Value: math.Abs(rhsv)}
	default:
		return nil
	}
}