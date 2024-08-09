package rete

import (
	"time"

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

func (op *AgeAsOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeAsOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	var lhsTime, rhsTime *time.Time
	switch vv := lhs.Value.(type) {
	case rdf.LDate:
		lhsTime = vv.Date
	case rdf.LDatetime:
		lhsTime = vv.Datetime
	default:
		return nil
	}

	switch vv := rhs.Value.(type) {
	case rdf.LDate:
		rhsTime = vv.Date
	case rdf.LDatetime:
		rhsTime = vv.Datetime
	default:
		return nil
	}

	age := rhsTime.Year() - lhsTime.Year()
	if rhsTime.YearDay() < lhsTime.YearDay() {
		age -= 1
	}
	return rdf.I(age)
}
