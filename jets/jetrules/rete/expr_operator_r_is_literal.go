package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// IS_LITERAL unary operator
type IsLiteralOp struct {
}

func NewIsLiteralOp() UnaryOperator {
	return &IsLiteralOp{}
}

func (op *IsLiteralOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *IsLiteralOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	if rhs.IsLiteral() {
		return rdf.I(1)
	}
	return rdf.I(0)
}
