package rete

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TRIM unary operator
type TrimOp struct {

}

func NewTrimOp() UnaryOperator {
	return &TrimOp{}
}

func (op *TrimOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *TrimOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		return rdf.S(strings.Trim(rhsv, " \r\t\n"))
	default:
		return nil
	}
}