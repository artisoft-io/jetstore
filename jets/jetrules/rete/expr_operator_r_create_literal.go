package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CREATE_LITERAL unary operator
type CreateLiteralOp struct {

}

func NewCreateLiteralOp() UnaryOperator {
	return &CreateLiteralOp{}
}

func (op *CreateLiteralOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *CreateLiteralOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateLiteralOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.CreateLiteral(reteSession.RdfSession)
}
