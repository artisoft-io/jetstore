package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// EQ operator
type EqOp struct {
}

func NewEqOp() BinaryOperator {
	return &EqOp{}
}

func (op *EqOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *EqOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			if lhsv == rhsv {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if rdf.NearlyEqual(float64(lhsv), rhsv) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			if rdf.NearlyEqual(lhsv, float64(rhsv)) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if rdf.NearlyEqual(lhsv, rhsv) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case string:
			if lhsv == rhsv {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	default:
		return nil
	}
}