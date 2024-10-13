package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CREATE_RESOURCE unary operator
type CreateResourceOp struct {
}

func NewCreateResourceOp() UnaryOperator {
	return &CreateResourceOp{}
}

func (op *CreateResourceOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *CreateResourceOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateResourceOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.CreateResource(reteSession.RdfSession)
}
