package rete

import (
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

func (op *DayPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.DayPeriodOf()
}
