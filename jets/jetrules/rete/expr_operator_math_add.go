package rete

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type AddOp struct {

}

func NewAddOp() BinaryOperator {
	return &AddOp{}
}

func (op *AddOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AddOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case rdf.LDate:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv.Add(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}
	
	case rdf.LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv.Add(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}

	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv + rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) + rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv + float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv + rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", lhsv, rhsv)}
		case float64:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", lhsv, rhsv)}
		case string:
			return &rdf.Node{Value: lhsv + rhsv}
		default:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", lhs, rhs)}
		}
	default:
		return nil
	}
}