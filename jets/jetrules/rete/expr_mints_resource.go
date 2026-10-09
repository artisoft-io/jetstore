package rete

// Consequents that mint a resource.
//
// An expression mints a resource when an operator in it is create_entity,
// create_uuid_resource, range, lookup_rand or multi_lookup_rand: evaluating it twice on
// the same beta row gives two different results.
//
// A node vertex with such a consequent term records, on each beta row, the triples its
// consequents inferred -- the jets:key triple create_entity inserts included -- and
// retraction replays that record instead of evaluating the consequents again (see
// ReteSession.ComputeConsequentTriples). Recomputing is what made retracting a
// create_entity 0 rule mint a second entity and leave its link in place, and a keyed
// create_entity re-insert jets:key on every retraction.
//
// Operators that read the graph -- exist, exist_not, size_of, the aggregates, lookup --
// are deliberately NOT in the set, so their consequents are still recomputed on
// retraction. That is a known defect that usi_ws's rules depend on; see README.md,
// 2026-10-09. The C++ engine makes the same split, with the marker struct MintsResource
// in jets/rete/expr.h.

// mintsResource is implemented by the operators that mint a resource.
type mintsResource interface {
	mintsResource()
}

func (op *CreateEntityOp) mintsResource()       {}
func (op *CreateUuidResourceOp) mintsResource() {}
func (op *RangeOp) mintsResource()              {}
func (op *LookupRandOp) mintsResource()         {} // lookup_rand and multi_lookup_rand

func isMintingOperator(op any) bool {
	_, ok := op.(mintsResource)
	return ok
}

// MintsResource reports whether expr mints a resource. An expression type it does not
// know is taken to mint one, which costs a record and never a wrong retraction.
func MintsResource(expr Expression) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ExprCst, *ExprBindedVar:
		return false
	case *ExprBinaryOp:
		return isMintingOperator(e.op) || MintsResource(e.lhs) || MintsResource(e.rhs)
	case *ExprUnaryOp:
		return isMintingOperator(e.op) || MintsResource(e.rhs)
	default:
		return true
	}
}

// MintsResource reports whether computing the alpha node's consequent triple mints a
// resource, so that it gives a different triple each time on the same beta row.
func (an *AlphaNode) MintsResource() bool {
	for _, f := range []AlphaFunctor{an.Fu, an.Fv, an.Fw} {
		if fe, ok := f.(*FExpression); ok && MintsResource(fe.expression) {
			return true
		}
	}
	return false
}
