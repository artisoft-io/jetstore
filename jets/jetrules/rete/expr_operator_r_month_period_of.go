package rete

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MonthPeriodOf unary operator
type MonthPeriodOfOp struct {
}

func NewMonthPeriodOfOp() UnaryOperator {
	return &MonthPeriodOfOp{}
}

func (op *MonthPeriodOfOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func monthPeriodOf(t *time.Time) int {
	// monthPeriod = (year-1970)*12 + month
	return (t.Year()-1970)*12 + int(t.Month())
}

func (op *MonthPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := rdf.ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return rdf.I(monthPeriodOf(t))
	case rdf.LDate:
		return rdf.I(monthPeriodOf(rhsv.Date))
	case rdf.LDatetime:
		return rdf.I(monthPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}
