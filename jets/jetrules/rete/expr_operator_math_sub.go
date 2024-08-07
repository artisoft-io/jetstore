package rete

import (
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type SubOp struct {

}

func NewSubOp() BinaryOperator {
	return &SubOp{}
}

func (op *SubOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *SubOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
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
		case int:
			return &rdf.Node{Value: lhsv.Add(-rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv.Add(-int(rhsv))}
		default:
			return nil
		}
	
	case rdf.LDatetime:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv.Add(-rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv.Add(-int(rhsv))}
		default:
			return nil
		}

	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv - rhsv}
		case float64:
			return &rdf.Node{Value: float64(lhsv) - rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			return &rdf.Node{Value: lhsv - float64(rhsv)}
		case float64:
			return &rdf.Node{Value: lhsv - rhsv}
		default:
			return nil
		}
	case string:
		switch rhsv := rhs.Value.(type) {
		case int:
			s, _ := strings.CutSuffix(lhsv, strconv.Itoa(int(rhsv)))
			return &rdf.Node{Value: s}
		case string:
			s, _ := strings.CutSuffix(lhsv, rhsv)
			return &rdf.Node{Value: s}
		default:
			return nil
		}
	default:
		return nil
	}
}