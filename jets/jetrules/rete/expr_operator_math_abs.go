package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ABS unary operator
type AbsOp struct {

}

func NewAbsOp() UnaryOperator {
	return &AbsOp{}
}

func (op *AbsOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *AbsOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *AbsOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.ABS()
}