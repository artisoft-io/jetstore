package op

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Add operator
type GtOp struct {

}

func NewGtOp() rete.BinaryOperator {
	return &GtOp{}
}

func (op *GtOp) RegisterCallback(reteSession *rete.ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *GtOp) Eval(reteSession *rete.ReteSession, row *rete.BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int32:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if lhsv > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case int64:
			if int64(lhsv) > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case float64:
			if float64(lhsv) > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		default:
			return nil
		}
	case int64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if lhsv > int64(rhsv) {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case int64:
			if lhsv > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case float64:
			if float64(lhsv) > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if lhsv > float64(rhsv) {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case int64:
			if lhsv > float64(rhsv) {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		case float64:
			return &rdf.Node{Value: lhsv * rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case string:
			if lhsv > rhsv {
				return &rdf.Node{Value: int32(1)}
			}
			return &rdf.Node{Value: int32(0)}
		default:
			return nil
		}
	default:
		return nil
	}
}
