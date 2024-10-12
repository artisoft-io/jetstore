package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// NOT unary operator
type NotOp struct {
}

func NewNotOp() UnaryOperator {
	return &NotOp{}
}

func (op *NotOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *NotOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *NotOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.NOT()
}