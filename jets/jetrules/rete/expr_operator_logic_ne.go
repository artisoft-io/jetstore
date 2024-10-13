package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// NE operator
type NeOp struct {
}

func NewNeOp() BinaryOperator {
	return &NeOp{}
}

func (op *NeOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *NeOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *NeOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.NE(rhs)
}
