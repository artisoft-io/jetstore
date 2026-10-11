package rete

import (
	"errors"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// NOT unary operator
type NotOp struct {
}

func NewNotOp() UnaryOperator {
	return &NotOp{}
}

func (op *NotOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *NotOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *NotOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	// Text is read as a bool (I-45): empty text is null, and text outside the
	// accepted spellings raises a jets:exception; both make the result null.
	if t, isText := rhs.Value.(string); isText {
		if _, err := rdf.ParseBool(t); errors.Is(err, rdf.ErrNullBool) {
			return rdf.Null()
		}
	}
	b, ok := reteSession.truthOf(rhs)
	if !ok {
		return rdf.Null()
	}
	return rdf.B(!b)
}