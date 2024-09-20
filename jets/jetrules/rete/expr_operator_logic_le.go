package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// LE operator
type LeOp struct {

}

func NewLeOp() BinaryOperator {
	return &LeOp{}
}

func (op *LeOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LeOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LeOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			if lhsv <= rhsv {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if float64(lhsv) <= rhsv {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			if lhsv <= float64(rhsv) {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		case float64:
			if lhsv <= rhsv {
				return &rdf.Node{Value: 1}
			}
			return &rdf.Node{Value: 0}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case string:
			if lhsv <= rhsv {
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
