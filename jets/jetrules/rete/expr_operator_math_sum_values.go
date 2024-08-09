package rete

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Max operator - with truth maintenance
type SumValuesOp struct {
	objProperty  *rdf.Node
	dataProperty *rdf.Node
}

func NewSumValuesOp() BinaryOperator {
	return &SumValuesOp{}
}

// Add truth maintenance
func (op *SumValuesOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	jr := rdfSession.ResourceMgr.JetsResources
	entityProperty := rdfSession.GetObject(rhs, jr.Jets__entity_property)
	// if op.entityProperty == null then do sum of ?v in (lhs, rhs, ?v)
	// if op.entityProperty != null then do sum of ?v in (lhs, objP, ?o).(?o, dataP, ?v)
	//	where objP is entity property (non functional property) and
	//        dataP is value property (functional property)
	var cb rdf.NotificationCallback
	if entityProperty != nil {
		op.objProperty = entityProperty
		op.dataProperty = rdfSession.GetObject(rhs, jr.Jets__value_property)
		// value_property is the domain property to get notification for
		if op.dataProperty != nil {
			cb = NewReteCallbackForFilter(reteSession, vertex, op.dataProperty)
		} else {
			return fmt.Errorf("error: jets:value_property is nil when jets:domain_property is not (SumValueOp)")
		}
	} else {
		// rhs is the domain property to get notification for
		op.objProperty = rhs
		cb = NewReteCallbackForFilter(reteSession, vertex, rhs)
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
	return nil
}

// Apply the visitor to find:
//   - case datap is nullptr: the sum of ?v in (lhs, objp, ?v)
//   - case datap is not nullptr: the sum of ?v in (lhs, objp, ?o).(?o, datap, ?v)
func (op *SumValuesOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	addOp := NewAddOp()
	itor := reteSession.RdfSession.FindSP(lhs, op.objProperty)
	if itor == nil {
		log.Println("error while calling FindSP in SumValuesOp")
		return nil
	}
	defer itor.Done()
	sumValue := rdf.I(0)
	for t3 := range itor.Itor {
		objV := t3[2]
		if op.dataProperty == nil {
			// do sum ?v: (lhs, objP, ?v)
			sumValue = addOp.Eval(reteSession, row, sumValue, objV)
		} else {
			// do sum ?v: (lhs, objp, ?o).(?o, datap, ?v)
			// NOTE: datap is a functional property
			dataV := reteSession.RdfSession.GetObject(objV, op.dataProperty)
			sumValue = addOp.Eval(reteSession, row, sumValue, dataV)
		}
	}
	return sumValue
}
