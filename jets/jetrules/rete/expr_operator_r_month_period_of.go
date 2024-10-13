package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// MonthPeriodOf unary operator
type MonthPeriodOfOp struct {
}

func NewMonthPeriodOfOp() UnaryOperator {
	return &MonthPeriodOfOp{}
}

func (op *MonthPeriodOfOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *MonthPeriodOfOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *MonthPeriodOfOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	return rhs.MonthPeriodOf()
}
