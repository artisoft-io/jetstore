package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// OR operator
type OrOp struct {
}

func NewOrOp() BinaryOperator {
	return &OrOp{}
}

func (op *OrOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *OrOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *OrOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	if lhs.Bool() || rhs.Bool() {
		return rdf.TRUE()
	}
	return rdf.FALSE()
}
