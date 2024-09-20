package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

// CREATE_UUID_RESOURCE unary operator
type CreateUuidResourceOp struct {
}

func NewCreateUuidResourceOp() UnaryOperator {
	return &CreateUuidResourceOp{}
}

func (op *CreateUuidResourceOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *CreateUuidResourceOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateUuidResourceOp) Eval(reteSession *ReteSession, _ *BetaRow, _ *rdf.Node) *rdf.Node {
	return rdf.R(uuid.NewString())
}
