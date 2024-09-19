package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_UPPER unary operator
type ToUpperOp struct {
}

func NewToUpperOp() UnaryOperator {
	return &ToUpperOp{}
}

func (op *ToUpperOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToUpperOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToUpperOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	return rdf.S(strings.ToUpper(rhsv))
}
