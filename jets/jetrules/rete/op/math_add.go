package op

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Add operator
type AddOp struct {

}

func NewAddOp() rete.BinaryOperator {
	return &AddOp{}
}

func (op *AddOp) RegisterCallback(reteSession *rete.ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AddOp) Eval(reteSession *rete.ReteSession, row *rete.BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case rdf.BlankNode:
		return nil
	case rdf.NamedResource:
		return nil
	case rdf.LDate:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case int64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}
	
	case rdf.LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case int64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		case float64:
			return &rdf.Node{Value: lhsv.Add(int(rhsv))}
		default:
			return nil
		}

	case int32:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + rhsv}
		case int64:
			return &rdf.Node{Value: int64(lhsv) + rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) + rhsv}
		default:
			return nil
		}
	case int64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + int64(rhsv)}
		case int64:
			return &rdf.Node{Value: int64(lhsv) + rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) + rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv + float64(rhsv)}
		case int64:
			return &rdf.Node{Value: lhsv + float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv + rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case int64:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case float64:
			return &rdf.Node{Value: fmt.Sprintf("%v%v", rhsv, rhsv)}
		case string:
			return &rdf.Node{Value: rhsv + rhsv}
		default:
			return nil
		}
	default:
		return nil
	}
}