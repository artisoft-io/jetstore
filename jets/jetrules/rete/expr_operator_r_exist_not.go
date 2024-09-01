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
func (op *ExistOp) String() string {
	if op.isExistNot {
		return "exist_not"
	}
	return "exist"
}

// Apply the operator:
//		lhs exist rhs     (case op.isExistNot is false)
//		lhs exist_not rhs (case op.isExistNot is true)
func (op *ExistOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	obj := reteSession.RdfSession.GetObject(lhs, rhs)
	var result *rdf.Node
	if op.isExistNot {
		if obj == nil {
			result = rdf.I(1)
		} else {
			result = rdf.I(0)
		}
	} else {
		if obj == nil {
			result = rdf.I(0)
		} else {
			result = rdf.I(1)
		}
	}
	// //**
	// log.Printf("Eval: %s %s %s returning %s", lhs, op.String(), rhs, result)
	return result
}
