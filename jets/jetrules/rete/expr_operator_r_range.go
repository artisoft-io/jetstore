package rete

import (
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// RANGE aka Iterator operator
type RangeOp struct {
}

// This operator is used as: (start_value range count)
// It returns an iterator, i.e. it returns the subject (a blank node) of a set of triples:
//
//	(subject, jets:range_value, value1)
//	(subject, jets:range_value, value2)
//	              . . .
//	(subject, jets:range_value, valueN)
//
// Where value1..N is: for(i=0; i<count; i++) start_value + i;
// The values can be either int or double depending on the type of start_value.
func NewRangeOp() BinaryOperator {
	return &RangeOp{}
}

func (op *RangeOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *RangeOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	return nil
}

func (op *RangeOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil || reteSession == nil {
		return nil
	}
	sess := reteSession.RdfSession
	rm := sess.ResourceMgr
	jr := rm.JetsResources
  // The subject resource for the triples to return
	s := sess.ResourceMgr.NewBNode()

	switch lhsv := lhs.Value.(type) {
	case int:
		switch rhsv := rhs.Value.(type) {
		case int:
			for i:=0; i<rhsv; i++ {
				sess.InsertInferred(s, jr.Jets__range_value, rm.NewIntLiteral(lhsv + i))
			}
			return s
		default:
			return nil
		}
	case float64:
		switch rhsv := rhs.Value.(type) {
		case int:
			for i:=0; i<rhsv; i++ {
				sess.InsertInferred(s, jr.Jets__range_value, rm.NewDoubleLiteral(lhsv + float64(i)))
			}
			return s
		default:
			return nil
		}
	default:
		return nil
	}
}
