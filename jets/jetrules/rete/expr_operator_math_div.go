package rete

import (
	"math"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type DivOp struct {

}

func NewDivOp() BinaryOperator {
	return &DivOp{}
}

func (op *DivOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *DivOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	switch lhsv := lhs.Value.(type) {
	case int32:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: lhsv / rhsv}
		case int64:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: int64(lhsv) / rhsv}
		case float64:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: float64(lhsv) / rhsv}
		default:
			return nil
		}
	case int64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: lhsv / int64(rhsv)}
		case int64:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: int64(lhsv) / rhsv}
		case float64:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: float64(lhsv) / rhsv}
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int32:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: lhsv / float64(rhsv)}
		case int64:
			if rhsv == 0 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: lhsv / float64(rhsv)}
		case float64:
			if math.Abs(rhsv) < math.SmallestNonzeroFloat64 {
				return &rdf.Node{Value: math.NaN()}	
			}
			return &rdf.Node{Value: lhsv * rhsv}
		default:
			return nil
		}
	default:
		return nil
	}
}