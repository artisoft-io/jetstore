package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// AGE_IN_MONTHS_AS_OF operator
// Calculate the age (in months), typical use:  (dob age_as_of serviceDate)
// where dob and serviceDate are date or datetime literals
type AgeMonthsAsOfOp struct {
}

func NewAgeMonthsAsOfOp() BinaryOperator {
	return &AgeMonthsAsOfOp{}
}

func (op *AgeMonthsAsOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeMonthsAsOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeMonthsAsOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	return lhs.AgeMonthsAsOf(rhs)
}
