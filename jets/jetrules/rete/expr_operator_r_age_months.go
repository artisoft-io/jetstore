package rete

import (
	"time"

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

func (op *AgeMonthsAsOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *AgeMonthsAsOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
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

	years := rhsTime.Year() - lhsTime.Year()
	var months int

	// Add the number of months in the last year
	if rhsTime.YearDay() <= lhsTime.YearDay() {
		years -= 1
		months += int(rhsTime.Month())
	} else {
		months += int(rhsTime.Month() - lhsTime.Month())
	}
	return rdf.I(months)
}
