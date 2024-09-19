package rete

import (
	"log"
	"reflect"

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
	if rhs == nil {
		return nil
	}

	switch reflect.TypeOf(rhs.Value).Kind() {
	case reflect.Int:
		return rhs
	case reflect.Float64:
		return rhs
	case reflect.String:
		return rhs
	default:
		log.Printf("Argment is not a literal (create_literal): %v", rhs.Value)
		return nil
	}
}
