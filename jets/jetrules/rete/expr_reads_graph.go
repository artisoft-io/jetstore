package rete

// Consequents that read the graph.
//
// An expression reads the graph when an operator in it is exist, exist_not, size_of,
// min_of, max_of, min_head_of, max_head_of, sorted_head, sum_values, join_values,
// lookup or multi_lookup: evaluating it twice on the same beta row gives two different
// values once the graph has moved in between.
//
// This is the wide half of the predicate 1d993844 called row purity (its
// expr_row_purity.go): a consequent is row pure when it neither mints a resource
// (expr_mints_resource.go) nor reads the graph. c2de6953 narrowed recording to the
// minting half, which is LEGACY defect 1 (README.md, 2026-10-09). The wide half is
// consulted only when a meta store is built with
// RetractionMode.ReplayGraphReadingConsequents, which only the rules unit test harness
// sets. The C++ engine makes the same split, with the marker struct ReadsGraph in
// jets/rete/expr.h.

// readsGraph is implemented by the operators whose result depends on the graph as it
// stands when they are evaluated.
type readsGraph interface {
	readsGraph()
}

func (op *ExistOp) readsGraph()      {} // exist and exist_not
func (op *SizeOfOp) readsGraph()     {}
func (op *MinMaxOp) readsGraph()     {} // min_of, max_of, min_head_of, max_head_of
func (op *SortedHeadOp) readsGraph() {}
func (op *SumValuesOp) readsGraph()  {}
func (op *JoinValuesOp) readsGraph() {}
func (op *LookupOp) readsGraph()     {} // lookup and multi_lookup

func isGraphReadingOperator(op any) bool {
	_, ok := op.(readsGraph)
	return ok
}

// ReadsGraph reports whether expr reads the graph. An expression type it does not know
// is taken to read it, which costs a record and never a wrong retraction.
func ReadsGraph(expr Expression) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ExprCst, *ExprBindedVar:
		return false
	case *ExprBinaryOp:
		return isGraphReadingOperator(e.op) || ReadsGraph(e.lhs) || ReadsGraph(e.rhs)
	case *ExprUnaryOp:
		return isGraphReadingOperator(e.op) || ReadsGraph(e.rhs)
	default:
		return true
	}
}

// ReadsGraph reports whether computing the alpha node's consequent triple reads the
// graph, so that it may give a different triple on the same beta row later.
func (an *AlphaNode) ReadsGraph() bool {
	for _, f := range []AlphaFunctor{an.Fu, an.Fv, an.Fw} {
		if fe, ok := f.(*FExpression); ok && ReadsGraph(fe.expression) {
			return true
		}
	}
	return false
}
