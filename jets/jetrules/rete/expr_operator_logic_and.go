package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AND operator
type AndOp struct {
}

func NewAndOp() BinaryOperator {
	return &AndOp{}
}

func (op *AndOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AndOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AndOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
  return lhs.AND(rhs)
}
