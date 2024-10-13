package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// DIV operator
type DivOp struct {
}

func NewDivOp() BinaryOperator {
	return &DivOp{}
}

func (op *DivOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *DivOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *DivOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.DIV(rhs)
}
