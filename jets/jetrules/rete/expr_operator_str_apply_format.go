package rete

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// APPLY_FORMAT operator
type ApplyFormatOp struct {
}

func NewApplyFormatOp() BinaryOperator {
	return &ApplyFormatOp{}
}

func (op *ApplyFormatOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *ApplyFormatOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}

	format, ok := rhs.Value.(string)
	if !ok {
		return nil
	}

	switch vv := rhs.Value.(type) {
	case rdf.BlankNode:
		return rdf.S(fmt.Sprintf(format, vv.Key))
	case rdf.NamedResource:
		return rdf.S(fmt.Sprintf(format, vv.Name))
	case rdf.RdfNull:
		return rdf.S(fmt.Sprintf(format, "null"))
	default:
		return rdf.S(fmt.Sprintf(format, lhs.Value))
	}
}
