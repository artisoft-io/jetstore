package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CREATE_ENTITY unary operator
type CreateEntityOp struct {

}

func NewCreateEntityOp() UnaryOperator {
	return &CreateEntityOp{}
}

func (op *CreateEntityOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *CreateEntityOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateEntityOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.CreateEntity(reteSession.RdfSession)
}
