package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// IS_NULL unary operator
type IsNullOp struct {
}

func NewIsNullOp() UnaryOperator {
	return &IsNullOp{}
}

func (op *IsNullOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *IsNullOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *IsNullOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}
	if rhs.IsNull() {
		return rdf.I(1)
	}
	return rdf.I(0)
}
