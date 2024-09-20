package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// LENGTH_OF unary operator
type LengthOfOp struct {
}

func NewLengthOfOp() UnaryOperator {
	return &LengthOfOp{}
}

func (op *LengthOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *LengthOfOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *LengthOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	return rdf.I(len(rhsv))
}
