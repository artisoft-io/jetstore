package rete

import (

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// GE operator
type GeOp struct {

}

func NewGeOp() BinaryOperator {
	return &GeOp{}
}

func (op *GeOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *GeOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *GeOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.GE(rhs)
}
