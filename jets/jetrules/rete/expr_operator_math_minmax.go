package rete

import (
	"cmp"
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
	isFirst := true
	var resultObj, resultValue *rdf.Node
	for t3 := range itor.All() {
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
				switch {
				case gt.Eval(reteSession, row, ll, rr).Bool():
					resultObj = currentObj
					resultValue = currentValue
				case tieEqual(currentValue, resultValue) && compareObjects(currentObj, resultObj) < 0:
					// A tie: the object that orders first wins, whatever the iteration
					// order (rules_benchmark I-33, Q-61), as in C++'s ApplyMinMaxVisitor.
					// Only equal values tie: a NaN or two values of types that do not
					// compare keep the first met, which is order-dependent.
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

// tieEqual is the test that makes two values a tie for compareObjects: the
// engine's own equality, the EQ operator, as C++ uses its EqVisitor. Since
// rules_benchmark Q-68 the two agree on numbers, and a NaN or an infinity is
// never equal. Values EQ does not compare are not a tie.
func tieEqual(a, b *rdf.Node) bool {
	return a != nil && b != nil && a.EQ(b).Bool()
}

// compareObjects is the total order that breaks a tie between two candidates of
// equal value in min_head_of, max_head_of and sorted_head, the same in both
// engines (rules_benchmark I-33, Q-61, agreed with the C++ change): a named
// resource before a blank node before a literal; named resources by name,
// byte-wise; literals by their text form (rdf.TextOf, the I-16 and I-32
// rendering), byte-wise, and on equal text by type (int, uint, double, text,
// date, datetime); blank nodes by key, which is not stable across engines or
// runs and is a last resort. It returns -1, 0 or +1.
func compareObjects(a, b *rdf.Node) int {
	ka, kb := objectRank(a), objectRank(b)
	if ka != kb {
		return cmp.Compare(ka, kb)
	}
	switch av := a.Value.(type) {
	case rdf.NamedResource:
		return cmp.Compare(av.Name, b.Value.(rdf.NamedResource).Name)
	case rdf.BlankNode:
		return cmp.Compare(av.Key, b.Value.(rdf.BlankNode).Key)
	}
	if c := cmp.Compare(rdf.TextOf(a), rdf.TextOf(b)); c != 0 {
		return c
	}
	return cmp.Compare(literalRank(a), literalRank(b))
}

// objectRank orders the kinds of node for compareObjects.
func objectRank(v *rdf.Node) int {
	switch {
	case v.IsNull():
		// nil or rdf null: after every literal, as C++'s tie_break_kind
		return 3
	case v.IsResource():
		if _, ok := v.Value.(rdf.BlankNode); ok {
			return 1
		}
		return 0
	}
	return 2
}

// literalRank orders the literal types for compareObjects, as C++'s variant
// order does: int, uint, double, text, date, datetime.
func literalRank(v *rdf.Node) int {
	switch v.Value.(type) {
	case int:
		return 0
	case uint:
		return 1
	case float64:
		return 2
	case string:
		return 3
	case rdf.LDate:
		return 4
	case rdf.LDatetime:
		return 5
	}
	return 6
}
