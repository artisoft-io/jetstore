package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_TEXT unary operator
type ToTextOp struct {
}

func NewToTextOp() UnaryOperator {
	return &ToTextOp{}
}
func (op *ToTextOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToTextOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToTextOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	return &rdf.Node{Value: rhs.String()}
}
