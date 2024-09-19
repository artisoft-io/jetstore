package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// ENDS_WITH operator
type EndsWithOp struct {
}

func NewEndsWithOp() BinaryOperator {
	return &EndsWithOp{}
}

func (op *EndsWithOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *EndsWithOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *EndsWithOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	lhsv, ok := lhs.Value.(string)
	if !ok {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	if strings.HasSuffix(lhsv, rhsv) {
		return rdf.I(1)
	}
	return rdf.I(0)
}
