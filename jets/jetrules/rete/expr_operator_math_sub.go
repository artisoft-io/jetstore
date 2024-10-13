package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type SubOp struct {

}

func NewSubOp() BinaryOperator {
	return &SubOp{}
}

func (op *SubOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *SubOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *SubOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.SUB(rhs)
}