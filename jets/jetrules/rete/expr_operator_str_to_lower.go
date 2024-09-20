package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_LOWER unary operator
type ToLowerOp struct {
}

func NewToLowerOp() UnaryOperator {
	return &ToLowerOp{}
}

func (op *ToLowerOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToLowerOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToLowerOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	rhsv, ok := rhs.Value.(string)
	if !ok {
		return nil
	}
	return rdf.S(strings.ToLower(rhsv))
}
