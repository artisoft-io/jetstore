package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// IS_LITERAL unary operator
type IsResourceOp struct {
}

func NewIsResourceOp() UnaryOperator {
	return &IsResourceOp{}
}

func (op *IsResourceOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *IsResourceOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	if rhs.IsResource() {
		return rdf.I(1)
	}
	return rdf.I(0)
}
