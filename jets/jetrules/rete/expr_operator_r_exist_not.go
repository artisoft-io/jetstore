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

func (op *ExistOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
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
	if op.isExistNot {
		return lhs.ExistNot(reteSession.RdfSession, rhs)
	} else {
		return lhs.Exist(reteSession.RdfSession, rhs)
	}
}
