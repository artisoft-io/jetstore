package rete

import (
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

func (op *WeekPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.WeekPeriodOf()
}
