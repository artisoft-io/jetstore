package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_DATE unary operator
type ToDateOp struct {

}

func NewToDateOp() UnaryOperator {
	return &ToDateOp{}
}

func (op *ToDateOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToDateOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToDateOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		d, err := rdf.D(rhsv)
		if err != nil {
			return nil
		}
		return d
	default:
		return nil
	}
}