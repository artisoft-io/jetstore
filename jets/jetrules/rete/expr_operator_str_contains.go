package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// CONTAINS operator
type ContainsOp struct {
}

func NewContainsOp() BinaryOperator {
	return &ContainsOp{}
}

func (op *ContainsOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ContainsOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ContainsOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
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
	if strings.Contains(lhsv, rhsv) {
		return rdf.I(1)
	}
	return rdf.I(0)
}
