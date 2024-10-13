package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// LT operator
type LtOp struct {

}

func NewLtOp() BinaryOperator {
	return &LtOp{}
}

func (op *LtOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LtOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *LtOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.LT(rhs)
}
