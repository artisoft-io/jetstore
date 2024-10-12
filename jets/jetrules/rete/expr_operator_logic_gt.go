package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Add operator
type GtOp struct {

}

func NewGtOp() BinaryOperator {
	return &GtOp{}
}

func (op *GtOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *GtOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *GtOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.GT(rhs)
}
