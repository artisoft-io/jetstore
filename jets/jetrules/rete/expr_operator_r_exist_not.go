package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// exist / exist_not operators - with truth maintenance
type ExistOp struct {
	isExistNot   bool
}

func NewExistOp(isExistNot bool) BinaryOperator {
	return &ExistOp{
		isExistNot:  isExistNot,
	}
}

// Add truth maintenance
func (op *ExistOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	cb := NewReteCallbackForFilter(reteSession, vertex, rhs)
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
	return nil
}

// Apply the operator:
//		lhs exist rhs     (case op.isExistNot is false)
//		lhs exist_not rhs (case op.isExistNot is true)
func (op *ExistOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	obj := reteSession.RdfSession.GetObject(lhs, rhs)
	if op.isExistNot {
		if obj == nil {
			return rdf.I(1)
		} else {
			return rdf.I(0)
		}
	} else {
		if obj == nil {
			return rdf.I(0)
		} else {
			return rdf.I(1)
		}
	}
}
