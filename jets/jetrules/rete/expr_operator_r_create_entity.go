package rete

import (
	"log"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/google/uuid"
)

// CREATE_ENTITY unary operator
type CreateEntityOp struct {

}

func NewCreateEntityOp() UnaryOperator {
	return &CreateEntityOp{}
}

func (op *CreateEntityOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *CreateEntityOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case int:
		if rhsv == 0 {
			return createEntity(reteSession, "")
		}
		return createEntity(reteSession, strconv.Itoa(rhsv))
	case float64:
		if rdf.NearlyEqual(rhsv, 0) {
			return createEntity(reteSession, "")
		}
		return createEntity(reteSession, strconv.FormatFloat(rhsv, 'G', 15, 64))
	default:
		return nil
	}
}

func createEntity(reteSession *ReteSession, name string) *rdf.Node {
	if name == "" {
		name = uuid.NewString()
	}
	sess := reteSession.RdfSession
	rm := sess.ResourceMgr
	entity := rm.NewResource(name)
	_, err := sess.InsertInferred(entity, rm.JetsResources.Jets__key, rm.NewTextLiteral(name))
	if err != nil {
		log.Panicf("wile calling InsertInferred (createEntity operator): %v", err)
	}
	return entity
}