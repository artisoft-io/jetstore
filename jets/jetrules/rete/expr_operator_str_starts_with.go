package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// STARTS_WITH operator
type StartWithOp struct {
}

func NewStartWithOp() BinaryOperator {
	return &StartWithOp{}
}

func (op *StartWithOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *StartWithOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *StartWithOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
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
	if strings.HasPrefix(lhsv, rhsv) {
		return rdf.I(1)
	}
	return rdf.I(0)
}
