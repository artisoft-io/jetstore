package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// size_of operator - with truth maintenance
type SizeOfOp struct {
}

func NewSizeOfOp() BinaryOperator {
	return &SizeOfOp{}
}

// Add truth maintenance
func (op *SizeOfOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
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
//		lhs size_of rhs
func (op *SizeOfOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	itor := reteSession.RdfSession.FindSP(lhs, rhs)
	var count int
	if itor != nil {
		for _ = range itor.Itor {
			count += 1
		}
		itor.Done()
	}
	return rdf.I(count)
}
