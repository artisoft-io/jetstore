package rete

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// JOIN_VALUES operator - with truth maintenance
//
// Fold a multi-valued property into ONE ordered string.
//
// Mirrors SumValuesOp rather than inventing a shape: the same config forms and the same
// truth-maintenance callback. What it adds is a jets:separator triple on the config and
// a sort. The C++ RETE engine carries the same operator (JoinValuesVisitor,
// jets/rete/expr_op_arithmetics.h) and the two must agree, since a rule that compiles
// against one engine and not the other is the failure this operator was written to avoid.
//
// THE OUTPUT IS SORTED, AND THE REASON IS REPRODUCIBILITY RATHER THAN TASTE.
// An rdf multi-valued property is a *set* and has no order, so iteration order is an
// implementation detail of the graph container. An unsorted join would make the same
// entity produce a different string -- and, downstream, a different LLM prompt --
// between two runs over identical data, which is noise injected straight into an
// experiment whose design is one variable. The sort is on the *rendered* strings,
// byte-wise ascending, which is what the C++ engine does too; ISO-8601 dates sort
// lexically into chronological order, so the sort costs nothing for the date case that
// motivated the operator.
//
// BEHAVIOUR, DECIDED AND DOCUMENTED:
//   - empty set    -> nil, so nothing is asserted. That is what MinMaxOp does when it
//     finds no value, and it is the honest answer: an empty string would assert "this
//     entity has an empty list", which is a different claim from "this entity has no
//     list", and a rule can still test for the absence.
//   - single value -> that value rendered, with no separator. A separator separates.
//   - null/absent  -> skipped, leaving no empty slot. In the entity_property +
//     values          value_property form an object carrying no value property
//     contributes nothing rather than an empty string, which is what min_of, max_of and
//     sum_values all do.
//   - duplicates   -> kept. This is a fold, not a distinct: two pharmacy claims filled
//     on the same day are two fills, and dropping one would lose a fact.
//   - non-string   -> rendered by renderJoinValue below. NOT by Node.String(), which
//     values          formats a date as a Go time struct ({2025-06-15 00:00:00 +0000
//     UTC}) rather than as ISO-8601 and would therefore disagree with the C++ engine.
//
// The separator comes from (config, jets:separator, "..."). It is OPTIONAL and defaults
// to ", ": a missing separator running the values together is a worse failure than a
// sensible default, and the default is the one the fill-date case wants.
type JoinValuesOp struct {
	objProperty  *rdf.Node
	dataProperty *rdf.Node
	separator    string
}

func NewJoinValuesOp() BinaryOperator {
	return &JoinValuesOp{}
}

// Three config forms, all of which the C++ engine accepts:
//
//   - rhs carries jets:entity_property and jets:value_property: join ?v in
//     (lhs, objP, ?o).(?o, dataP, ?v) with objP non functional and dataP functional.
//   - rhs carries jets:value_property alone: join ?v in (lhs, dataP, ?v).
//   - rhs is itself the non functional data property: join ?v in (lhs, rhs, ?v).
//
// A config carrying jets:entity_property and no jets:value_property is a
// misconfiguration and is reported rather than silently yielding nothing.
func (op *JoinValuesOp) InitializeOperator(metaGraph *rdf.RdfGraph, lhs, rhs *rdf.Node) error {
	jr := metaGraph.RootRm.JetsResources
	op.separator = ", "
	separator := metaGraph.GetObject(rhs, jr.Jets__separator)
	if separator != nil && !separator.IsNull() {
		s, ok := separator.Value.(string)
		if !ok {
			return fmt.Errorf(
				"error: join_values operator misconfigured, jets:separator must be a text literal, got %v", separator)
		}
		op.separator = s
	}
	entityProperty := metaGraph.GetObject(rhs, jr.Jets__entity_property)
	valueProperty := metaGraph.GetObject(rhs, jr.Jets__value_property)
	switch {
	case entityProperty != nil:
		if valueProperty == nil {
			return fmt.Errorf(
				"error: join_values operator misconfigured, %v has jets:entity_property and no jets:value_property", rhs)
		}
		op.objProperty = entityProperty
		op.dataProperty = valueProperty
	case valueProperty != nil:
		op.objProperty = valueProperty
	default:
		op.objProperty = rhs
	}
	return nil
}

// Add truth maintenance -- same shape as SumValuesOp.RegisterCallback: watch the
// property the values actually live on, so the joined value is recomputed when the
// underlying triples change.
func (op *JoinValuesOp) RegisterCallback(reteSession *ReteSession, vertex int, lhs, rhs *rdf.Node) error {
	if reteSession == nil {
		return nil
	}
	rdfSession := reteSession.RdfSession
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

func (op *JoinValuesOp) String() string {
	return fmt.Sprintf("join_values of %s with %s separated by %q", op.objProperty, op.dataProperty, op.separator)
}

// renderJoinValue renders one rdf value the way join_values wants it in the joined
// string. Deliberately not Node.String(): that renders a date with %v on the LDate
// struct, and a double with %v, neither of which matches what the C++ JoinTextVisitor
// produces. Two engines rendering a date differently is the same defect as two engines
// supporting different operators, one layer down.
func renderJoinValue(v *rdf.Node) string {
	if v == nil {
		return ""
	}
	switch vv := v.Value.(type) {
	case rdf.LDate:
		if vv.Date == nil {
			return ""
		}
		// boost::gregorian::to_iso_extended_string
		return vv.Date.Format("2006-01-02")
	case rdf.LDatetime:
		if vv.Datetime == nil {
			return ""
		}
		// boost::posix_time::to_iso_extended_string, which omits the fractional part
		// when it is zero.
		if vv.Datetime.Nanosecond() == 0 {
			return vv.Datetime.Format("2006-01-02T15:04:05")
		}
		return vv.Datetime.Format("2006-01-02T15:04:05.999999")
	case float64:
		// std::ostream default formatting is %g with 6 significant digits, which is
		// what the C++ side emits for a double.
		return strconv.FormatFloat(vv, 'g', 6, 64)
	default:
		return v.String()
	}
}

// Apply the operator to find:
//   - case dataProperty is nil: the sorted join of ?v in (lhs, objProperty, ?v)
//   - case dataProperty is not nil: the sorted join of ?v in
//     (lhs, objProperty, ?o).(?o, dataProperty, ?v)
func (op *JoinValuesOp) Eval(reteSession *ReteSession, row *BetaRow, lhs, rhs *rdf.Node) *rdf.Node {
	if lhs == nil || rhs == nil {
		return nil
	}
	itor := reteSession.RdfSession.FindSP(lhs, op.objProperty)
	if itor == nil {
		log.Println("error while calling FindSP in JoinValuesOp")
		return nil
	}
	defer itor.Done()
	items := make([]string, 0)
	for t3 := range itor.Itor {
		value := t3[2]
		if op.dataProperty != nil {
			// join ?v: (lhs, objProperty, ?o).(?o, dataProperty, ?v)
			// NOTE: dataProperty is a functional property
			value = reteSession.RdfSession.GetObject(value, op.dataProperty)
		}
		// skip null values, leaving no empty slot in the joined string
		if value == nil || value.IsNull() {
			continue
		}
		items = append(items, renderJoinValue(value))
	}
	if len(items) == 0 {
		// nothing to join: assert nothing rather than an empty string
		return nil
	}
	sort.Strings(items)
	return rdf.S(strings.Join(items, op.separator))
}
