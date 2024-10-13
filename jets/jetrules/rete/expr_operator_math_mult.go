package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MULT operator
type MultOp struct {

}

func NewMultOp() BinaryOperator {
	return &MultOp{}
}

func (op *MultOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *MultOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *MultOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.MUL(rhs)
}
