package rete

import (
	"log"

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

// Eval inserts the entity's jets:key triple through the rete session so that, in a
// consequent, it is recorded on the beta row with the consequent triple and retracted
// with it, rather than re-inserted when the row is retracted.
func (op *CreateEntityOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	entity, key := rhs.CreateEntityKey(reteSession.RdfSession)
	if entity == nil {
		return nil
	}
	jetsKey := reteSession.RdfSession.ResourceMgr.JetsResources.Jets__key
	if _, err := reteSession.InsertInferredFor(row, entity, jetsKey, key); err != nil {
		log.Panicf("while calling InsertInferred (createEntity operator): %v", err)
	}
	return entity
}
