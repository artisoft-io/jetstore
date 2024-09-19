package rete

import (
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

func (op *MinMaxOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	jr := metaGraph.RootRm.JetsResources
	entityProperty := metaGraph.GetObject(rhs, jr.Jets__entity_property)
	// if op.entityProperty == null then mode is min/max of a multi value property
	if entityProperty != nil {
		op.objProperty = entityProperty
		op.dataProperty = metaGraph.GetObject(rhs, jr.Jets__value_property)
	} else {
		op.objProperty = rhs
	}
	return nil
}

// Add truth maintenance
func (op *MinMaxOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	// if op.dataProperty == null then mode is min/max of a multi value property
	var cb rdf.NotificationCallback
	if op.dataProperty != nil {
		cb = NewReteCallbackForFilter(reteSession, vertex, op.dataProperty)
	} else {
		cb = NewReteCallbackForFilter(reteSession, vertex, op.objProperty)
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
	return nil
}
func (op *MinMaxOp) String() string {
	switch {
	case op.isMin && op.retObj:
		return "min_head_of"
	case op.isMin && !op.retObj:
		return "min_of"
	case !op.isMin && op.retObj:
		return "max_head_of"
	case !op.isMin && !op.retObj:
		return "max_of"
	}
	return "UNKNOWN OP (minmax)"
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
