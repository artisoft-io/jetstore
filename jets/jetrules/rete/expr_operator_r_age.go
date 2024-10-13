package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AGE_AS_OF operator
// Calculate the age (in years), typical use:  (dob age_as_of serviceDate)
// where dob and serviceDate are date or datetime literals
type AgeAsOfOp struct {
}

func NewAgeAsOfOp() BinaryOperator {
	return &AgeAsOfOp{}
}

func (op *AgeAsOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeAsOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeAsOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.AgeAsOf(rhs)
}
