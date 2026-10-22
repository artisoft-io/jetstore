package rete

import (
	"fmt"
	"log"
	"math"
	"math/big"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// SUM_VALUES operator - with truth maintenance
type SumValuesOp struct {
	objProperty  *rdf.Node
	dataProperty *rdf.Node
}

func NewSumValuesOp() BinaryOperator {
	return &SumValuesOp{}
}

func (op *SumValuesOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	jr := metaGraph.RootRm.JetsResources
	entityProperty := metaGraph.GetObject(rhs, jr.Jets__entity_property)
	// if op.entityProperty == null then do sum of ?v in (lhs, rhs, ?v)
	//  where rhs is non functional data property
	//
	// if op.entityProperty != null then do sum of ?v in (lhs, objP, ?o).(?o, dataP, ?v)
	//	where objP is entity property (non functional obj property) and
	//        dataP is value property (functional data property)
	if entityProperty != nil {
		op.objProperty = entityProperty
		op.dataProperty = metaGraph.GetObject(rhs, jr.Jets__value_property)
	} else {
		op.objProperty = rhs
	}
	return nil
}

// Add truth maintenance
func (op *SumValuesOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	// Register the callback with the rhs domain property
	rdfSession := reteSession.RdfSession
	// if op.dataProperty == null then do sum of ?v in (lhs, rhs, ?v)
	// if op.dataProperty != null then do sum of ?v in (lhs, objP, ?o).(?o, dataP, ?v)
	//	where objP is entity property (non functional property) and
	//        dataP is value property (functional property)
	var cb rdf.NotificationCallback
	if op.dataProperty != nil {
		// dataProperty is the domain property to get notification for
		if op.dataProperty != nil {
			cb = NewReteCallbackForFilter(reteSession, vertex, op.dataProperty)
		} else {
			return fmt.Errorf("error: jets:value_property is nil when jets:domain_property is not (SumValueOp)")
		}
	} else {
		// op.objProperty is the domain property to get notification for
		cb = NewReteCallbackForFilter(reteSession, vertex, op.objProperty)
	}
	rdfSession.AssertedGraph.CallbackMgr.AddCallback(cb)
	rdfSession.InferredGraph.CallbackMgr.AddCallback(cb)
	return nil
}
func (op *SumValuesOp) String() string {
	return fmt.Sprintf("sum_values of %s with %s",op.objProperty, op.dataProperty)
}

// Apply the visitor to find:
//   - case datap is nullptr: the sum of ?v in (lhs, objp, ?v)
//   - case datap is not nullptr: the sum of ?v in (lhs, objp, ?o).(?o, datap, ?v)
//
// As the C++ ApplySumValuesVisitor (SumOfValues), so that the sum does not
// depend on the order the graph holds the values in (rules_benchmark I-30):
//   - a null is skipped wherever it comes; no value, or only nulls, is null;
//   - numbers alone: a double present makes the sum a double, every value
//     converted, summed in the order met, so the order can move the last bit
//     only. Integers alone are summed exactly and take C++'s widest integer
//     type present (int32 < uint32 < int64 < uint64), or int64 when the sum
//     does not fit it, then a double. A Go int reaches C++ as an int32 and a
//     uint as a uint32 (the bridge's create_int and create_uint), so: a uint
//     when a uint is present and the sum fits 32 unsigned bits, else an int
//     when it fits 64 bits, else a double;
//   - a value that is not a number (text, a date): null, wherever it comes and
//     whatever else is there. The + fold the engines made over such values
//     depended on which came first (text + 2 is "text2", 2 + text has no
//     answer), so it had no order-free answer.
//
// Until rules_benchmark I-30 an int accumulator truncated every double added
// to it, and a null made the whole sum null.
func (op *SumValuesOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	itor := reteSession.RdfSession.FindSP(lhs, op.objProperty)
	if itor == nil {
		log.Println("error while calling FindSP in SumValuesOp")
		return nil
	}
	var values []*rdf.Node
	numbers := true
	for t3 := range itor.All() {
		v := t3[2]
		if op.dataProperty != nil {
			// do sum ?v: (lhs, objp, ?o).(?o, datap, ?v)
			// NOTE: datap is a functional property
			v = reteSession.RdfSession.GetObject(v, op.dataProperty)
		}
		if v == nil || v.IsNull() {
			continue
		}
		switch v.Value.(type) {
		case int, uint, float64:
		default:
			numbers = false
		}
		values = append(values, v)
	}
	if len(values) == 0 || !numbers {
		return rdf.Null()
	}
	return sumOfNumbers(values)
}

// sumOfNumbers sums int, uint and float64 values as SumOfValues does in C++.
func sumOfNumbers(values []*rdf.Node) *rdf.Node {
	var isum, term big.Int
	var dsum float64
	hasDouble, hasUint := false, false
	for _, v := range values {
		switch vv := v.Value.(type) {
		case int:
			isum.Add(&isum, term.SetInt64(int64(vv)))
			dsum += float64(vv)
		case uint:
			hasUint = true
			isum.Add(&isum, term.SetUint64(uint64(vv)))
			dsum += float64(vv)
		case float64:
			hasDouble = true
			dsum += vv
		}
	}
	switch {
	case hasDouble:
		return rdf.F(dsum)
	case hasUint && isum.Sign() >= 0 && isum.IsUint64() && isum.Uint64() <= math.MaxUint32:
		return rdf.UI(uint(isum.Uint64()))
	case isum.IsInt64():
		return rdf.I(int(isum.Int64()))
	}
	f, _ := new(big.Float).SetInt(&isum).Float64()
	return rdf.F(f)
}
