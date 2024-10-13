package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// EQ operator
type EqOp struct {
}

func NewEqOp() BinaryOperator {
	return &EqOp{}
}

func (op *EqOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *EqOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *EqOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.EQ(rhs)
}
