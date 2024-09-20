package rete

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MonthPeriodOf unary operator
type WeekPeriodOfOp struct {
}

func NewWeekPeriodOfOp() UnaryOperator {
	return &WeekPeriodOfOp{}
}
func (op *WeekPeriodOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *WeekPeriodOfOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func weekPeriodOf(t *time.Time) int {
	// secPerDay = 24 * 60 * 60 = 84400
	// secPerWeek = 7 * secPerDay = 604800
	// weekPeriod = int(unixTime/secPerWeek + 1)
	return int(t.Unix()/604800 + 1)
}

func (op *WeekPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := rdf.ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return rdf.I(weekPeriodOf(t))
	case rdf.LDate:
		return rdf.I(weekPeriodOf(rhsv.Date))
	case rdf.LDatetime:
		return rdf.I(weekPeriodOf(rhsv.Datetime))
	default:
		return nil
	}
}
