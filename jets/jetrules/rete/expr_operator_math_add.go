package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type AddOp struct {

}

func NewAddOp() BinaryOperator {
	return &AddOp{}
}

func (op *AddOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AddOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AddOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.ADD(rhs)
}