package op

import (
	"math"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Add unary operator
type AbsOp struct {

}

func NewAbsOp() rete.UnaryOperator {
	return &AbsOp{}
}

func (op *AbsOp) RegisterCallback(reteSession *rete.ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *AbsOp) Eval(reteSession *rete.ReteSession, row *rete.BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int32:
		if rhsv < 0 {
			return &rdf.Node{Value: -rhsv}
		}
		return &rdf.Node{Value: rhsv}
	case int64:
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