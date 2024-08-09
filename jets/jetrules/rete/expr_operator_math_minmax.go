package rete

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// Max operator - with truth maintenance
type MinMaxOp struct {
	isMin        bool
	retObj       bool
	objProperty  *rdf.Node
	dataProperty *rdf.Node
}

func NewMinMaxOp(isMin, retObj bool) BinaryOperator {
	return &MinMaxOp{
		isMin:  isMin,
		retObj: retObj,
	}
}

// Add truth maintenance
func (op *MinMaxOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	jr := rdfSession.ResourceMgr.JetsResources
	entityProperty := rdfSession.GetObject(rhs, jr.Jets__entity_property)
	// if op.entityProperty == null then mode is min/max of a multi value property
	var cb rdf.NotificationCallback
	if entityProperty != nil {
		op.objProperty = entityProperty
		op.dataProperty = rdfSession.GetObject(rhs, jr.Jets__value_property)
		// value_property is the domain property to get notification for
		if op.dataProperty != nil {
			cb = NewReteCallbackForFilter(reteSession, vertex, op.dataProperty)
		} else {
			return fmt.Errorf("error: jets:value_property is nil when jets:domain_property is not")
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

// Apply the visitor to find the min/max value.
//   - case datap == nullptr: return ?o such that min/max ?o in (s, objp, ?o) with objp functional property
//   - case datap != nullptr: return ?o or ?v such that min/max ?v in (s, objp, ?o).(?o datap ?v)
//     with objp non-functional and datap functional property
//
// In the implementation we have:
//
//	?o is currentObj and ?v is currentValue with
//	(s, objp, currentObj).(currentObj, datap, currentValue), with currentObj = currentValue if datap==nullptr
func (op *MinMaxOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	gt := NewGtOp()
	itor := reteSession.RdfSession.FindSP(lhs, op.objProperty)
	if itor == nil {
		log.Println("error while calling FindSP in MinMaxOp")
		return nil
	}
	defer itor.Done()
	isFirst := true
	var resultObj, resultValue *rdf.Node
	for t3 := range itor.Itor {
		currentObj := t3[2]
		if currentObj == nil {
			log.Panicf("unexpected error: got null obj in MinMax operator with argument %s, %s", lhs, rhs)
		}
		currentValue := currentObj
		if op.dataProperty != nil {
			currentValue = reteSession.RdfSession.GetObject(currentObj, op.dataProperty)
		}
		// skip null values
		if currentValue != nil && currentValue != rdf.Null()  {
			if isFirst {
				resultObj = currentObj
				resultValue = currentValue
				isFirst = false
			} else {
				// visitor is for: ll > rr
				// to have min, do if resultValue > currentValue, then resultValue = currentValue
				// to have max, do if currentValue > resultValue, then resultValue = currentValue
				var ll, rr *rdf.Node
				if op.isMin {
					ll = resultValue
					rr = currentValue
				} else {
					ll = currentValue
					rr = resultValue
				}
				if gt.Eval(reteSession, row, ll, rr).Bool() {
					resultObj = currentObj
					resultValue = currentValue
				}
			}
		}
	}
	if op.dataProperty != nil && !op.retObj {
		return resultValue
	}
	return resultObj
}
