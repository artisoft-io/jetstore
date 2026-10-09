package rete

// Row purity of consequent expressions.
//
// An expression is row pure when evaluating it twice on the same beta row is
// guaranteed to give the same value. It is not when an operator in it mints a
// resource (create_entity, create_uuid_resource, range), reads the graph (exist,
// exist_not, size_of, the aggregates) or reads a lookup table.
//
// A node vertex with a consequent term that is not row pure records, on each beta
// row, the triples its consequents inferred -- the jets:key triple create_entity
// inserts included -- and retraction replays that record instead of evaluating the
// consequents again (see ReteSession.ComputeConsequentTriples). Recomputing is what
// made retracting a create_entity 0 rule mint a second entity, a keyed create_entity
// re-insert jets:key on every retraction, and a consequent aggregate retract a
// triple it never inferred. The C++ engine makes the same split, with the marker
// struct NotRowPure in jets/rete/expr.h.

// notRowPure is implemented by the operators whose result is not a pure function of
// their arguments.
type notRowPure interface {
	notRowPure()
}

func (op *CreateEntityOp) notRowPure()       {}
func (op *CreateUuidResourceOp) notRowPure() {}
func (op *RangeOp) notRowPure()              {}
func (op *ExistOp) notRowPure()              {}
func (op *SizeOfOp) notRowPure()             {}
func (op *MinMaxOp) notRowPure()             {}
func (op *SortedHeadOp) notRowPure()         {}
func (op *SumValuesOp) notRowPure()          {}
func (op *JoinValuesOp) notRowPure()         {}
func (op *LookupOp) notRowPure()             {}
func (op *LookupRandOp) notRowPure()         {}

func isRowPureOperator(op any) bool {
	_, impure := op.(notRowPure)
	return !impure
}

// IsRowPure reports whether expr is row pure. An expression type it does not know is
// taken to be impure, which costs a record and never a wrong retraction.
func IsRowPure(expr Expression) bool {
	switch e := expr.(type) {
	case nil:
		return true
	case *ExprCst, *ExprBindedVar:
		return true
	case *ExprBinaryOp:
		return isRowPureOperator(e.op) && IsRowPure(e.lhs) && IsRowPure(e.rhs)
	case *ExprUnaryOp:
		return isRowPureOperator(e.op) && IsRowPure(e.rhs)
	default:
		return false
	}
}

// IsRowPure reports whether the consequent triple of the alpha node is the same every
// time it is computed on the same beta row.
func (an *AlphaNode) IsRowPure() bool {
	for _, f := range []AlphaFunctor{an.Fu, an.Fv, an.Fw} {
		if fe, ok := f.(*FExpression); ok && !IsRowPure(fe.expression) {
			return false
		}
	}
	return true
}
