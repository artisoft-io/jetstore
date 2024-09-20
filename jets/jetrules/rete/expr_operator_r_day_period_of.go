package rete

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MonthPeriodOf unary operator
type DayPeriodOfOp struct {
}

func NewDayPeriodOfOp() UnaryOperator {
	return &DayPeriodOfOp{}
}

func (op *DayPeriodOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *DayPeriodOfOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func dayPeriodOf(t *time.Time) int {
	// secPerDay = 24 * 60 * 60 = 84400
	// dayPeriod = int(unixTime/secPerDay + 1)
	return int(t.Unix()/84400 + 1)
}

func (op *DayPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := rdf.ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return rdf.I(dayPeriodOf(t))
	case rdf.LDate:
		return rdf.I(dayPeriodOf(rhsv.Date))
	case rdf.LDatetime:
		return rdf.I(dayPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}
