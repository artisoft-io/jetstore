package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// TO_TIMESTAMP unary operator
type ToTimestampOp struct {

}

func NewToTimestampOp() UnaryOperator {
	return &ToTimestampOp{}
}
func (op *ToTimestampOp) InitializeOperator(metaGraph *rdf.RdfGraph, rhs *rdf.Node) error {
	return nil
}

func (op *ToTimestampOp) RegisterCallback(reteSession *ReteSession, vertex int, rhs *rdf.Node) error {
	return nil
}

func (op *ToTimestampOp) Eval(reteSession *ReteSession, row *BetaRow, rhs *rdf.Node) *rdf.Node {
	if rhs == nil {
		return nil
	}

	switch rhsv := rhs.Value.(type) {
	case string:
		t, err := rdf.ParseDatetime(rhsv)
		if err != nil {
			return nil
		}
		return rdf.I(int(t.Unix()))
	case rdf.LDate:
		return rdf.I(int(rhsv.Date.Unix()))
	case rdf.LDatetime:
		return rdf.I(int(rhsv.Datetime.Unix()))
	default:
		return nil
	}
}