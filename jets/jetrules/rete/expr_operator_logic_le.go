package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// LE operator
type LeOp struct {

}

func NewLeOp() BinaryOperator {
	return &LeOp{}
}

func (op *LeOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LeOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LeOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.LE(rhs)
}
