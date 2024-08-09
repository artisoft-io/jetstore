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

func (op *ToUpperOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToUpperOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		return rdf.S(strings.ToUpper(rhsv))
	default:
		return nil
	}
}