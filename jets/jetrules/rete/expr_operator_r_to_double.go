package rete

import (
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_DOUBLE unary operator
type ToDoubleOp struct {

}

func NewToDoubleOp() UnaryOperator {
	return &ToDoubleOp{}
}
func (op *ToDoubleOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToDoubleOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToDoubleOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		return &rdf.Node{Value: float64(rhsv)}
	case float64:
		return rhs
	case string:
		f, err := strconv.ParseFloat(rhsv, 64)
		if err != nil {
			// log.Printf("***to_double: arg string is not a double: %s", rhsv)
			return nil
		}
		return &rdf.Node{Value: f}
	default:
		return nil
	}
}