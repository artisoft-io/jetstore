package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AND operator
type AndOp struct {
}

func NewAndOp() BinaryOperator {
	return &AndOp{}
}

func (op *AndOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AndOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AndOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
  // Text is read as a bool (I-45): text outside the accepted spellings
	// raises a jets:exception and makes the result null.
	l, okl := reteSession.truthOf(lhs)
	r, okr := reteSession.truthOf(rhs)
	if !okl || !okr {
		return rdf.Null()
	}
	return rdf.B(l && r)
}
