package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MULT operator
type MultOp struct {

}

func NewMultOp() BinaryOperator {
	return &MultOp{}
}

func (op *MultOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *MultOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv * rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) * rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv * float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv * rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv > 0 && rhsv < 1000000 {
				return &rdf.Node{Value: strings.Repeat(lhsv, rhsv)}
			}
		default:
			return nil
		}
	default:
		return nil
	}
	return nil
}
