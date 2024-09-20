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

func (op *ApplyFormatOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
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

	switch vv := lhs.Value.(type) {
	case rdf.BlankNode:
		return rdf.S(fmt.Sprintf(format, vv.Key))
	case rdf.NamedResource:
		return rdf.S(fmt.Sprintf(format, vv.Name))
	case rdf.RdfNull:
		return rdf.S(fmt.Sprintf(format, "null"))
	case rdf.LDate:
		return rdf.S(fmt.Sprintf(format, vv.Date.Year(), int(vv.Date.Month()), vv.Date.Day()))
	case rdf.LDatetime:
		return rdf.S(fmt.Sprintf(format, 
			vv.Datetime.Year(), int(vv.Datetime.Month()), vv.Datetime.Day(),
			vv.Datetime.Hour(), vv.Datetime.Minute(), vv.Datetime.Second(), vv.Datetime.Nanosecond()))
	default:
		return rdf.S(fmt.Sprintf(format, lhs.Value))
	}
}
