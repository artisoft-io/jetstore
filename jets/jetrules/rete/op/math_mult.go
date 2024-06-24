package op

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Add operator
type MultOp struct {

}

func NewMultOp() rete.BinaryOperator {
	return &MultOp{}
}

func (op *MultOp) RegisterCallback(reteSession *rete.ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *MultOp) Eval(reteSession *rete.ReteSession, row *rete.BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int32:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv * rhsv}
		case int64:
			return &rdf.Node{Value: int64(lhsv) * rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) * rhsv}
		default:
			return nil
		}
	case int64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv * int64(rhsv)}
		case int64:
			return &rdf.Node{Value: int64(lhsv) * rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) * rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			return &rdf.Node{Value: lhsv * float64(rhsv)}
		case int64:
			return &rdf.Node{Value: lhsv * float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv * rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if rhsv > 0 && rhsv < 1000000 {
				return &rdf.Node{Value: strings.Repeat(lhsv, int(rhsv))}
			}
		case int64:
			if rhsv > 0 && rhsv < 1000000 {
				return &rdf.Node{Value: strings.Repeat(lhsv, int(rhsv))}
			}
		default:
			return nil
		}
	default:
		return nil
	}
	return nil
}