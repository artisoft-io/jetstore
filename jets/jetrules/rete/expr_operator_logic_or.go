package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// OR operator
type OrOp struct {
}

func NewOrOp() BinaryOperator {
	return &OrOp{}
}

func (op *OrOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *OrOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int:
		if lhsv != 0 {
			return &rdf.Node{Value: 1}
		}
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv != 0 {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if !rdf.NearlyEqual(rhsv, 0) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case float64:
		if !rdf.NearlyEqual(lhsv, 0) {
			return &rdf.Node{Value: 1}
		}
		switch rhsv := rhs.Value.(type) {
		case int:
			if rhsv != 0 {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if !rdf.NearlyEqual(rhsv, 0) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case string:
		if lhsv != "" {
			return &rdf.Node{Value: 1}
		}
		switch rhsv := rhs.Value.(type) {
		case string:
			if rhsv != "" {
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
