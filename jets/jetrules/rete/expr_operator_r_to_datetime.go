package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_DATETIME unary operator
type ToDatetimeOp struct {

}

func NewToDatetimeOp() UnaryOperator {
	return &ToDatetimeOp{}
}
func (op *ToDatetimeOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToDatetimeOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToDatetimeOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		dt, err := rdf.DT(rhsv)
		if err != nil {
			return nil
		}
		return dt
	default:
		return nil
	}
}