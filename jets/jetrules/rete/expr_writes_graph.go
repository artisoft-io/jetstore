package rete

// Operators that write to the graph.
//
// An operator writes to the graph when evaluating it inserts triples into the rdf
// session: lookup and multi_lookup insert the looked-up rows and the cache entry
// (LookupTableSqlite3.Lookup, MultiLookup), lookup_rand and multi_lookup_rand likewise
// (LookupRand, MultiLookupRand), range inserts its jets:range_value triples, and
// create_entity inserts the entity's jets:key triple. The C++ engine's counterparts
// are LookupVisitor, MultiLookupVisitor, LookupRandVisitor, MultiLookupRandVisitor
// (jets/rete/lookup_sql_helper.cc), RangeVisitor (jets/rete/expr_op_others.h) and
// CreateEntityVisitor (jets/rete/expr_op_resources.h).
//
// Both engines evaluate an antecedent's filter while they iterate the graph to match
// that antecedent, and both rely on no rule writing to the graph during an iteration:
// consequents are inserted after the visit (ReteSession.ComputeConsequentTriples). So
// an operator in this set is allowed in a consequent and refused in a filter, by the
// compiler (jets/compilerv2/compiler/jet_rule_listener_filter_writes.go), which
// protects both engines because both run the compiled workspace.
//
// Not in the set, deliberately:
//   - The jets:exception triple a failed truth test (ReteSession.RaiseException) or the
//     C++ substring_of and replace_char_of insert on a bad argument. It is an error
//     path, it writes to jets:iState only, and refusing it would refuse every truth
//     test in a filter.
//   - raise_exception, C++ only: RaiseExceptionVisitor throws and ends the rule set
//     rather than writing.
//   - create_uuid_resource, create_resource and create_literal: they make a node in the
//     resource manager and insert nothing.
//
// ADDING AN OPERATOR THAT INSERTS INTO THE SESSION? Give it the writesGraph marker
// below. TestWritingOperatorsAreMarked in jets/compilerv2/compiler fails when an
// operator's Eval calls an inserting method and its type has no marker, and when a
// marked type no longer calls one.

// writesGraph is implemented by the operators that write to the graph when evaluated.
type writesGraph interface {
	writesGraph()
}

func (op *LookupOp) writesGraph()       {} // lookup and multi_lookup
func (op *LookupRandOp) writesGraph()   {} // lookup_rand and multi_lookup_rand
func (op *RangeOp) writesGraph()        {}
func (op *CreateEntityOp) writesGraph() {}

// IsGraphWritingOperator reports whether the operator registered under name in the
// Go factory, as a binary or a unary operator according to exprType (the
// ExpressionNode type, "binary" or "unary"), writes to the graph when evaluated. A
// name the factory does not register is reported as not writing; the compiler does
// not reject unknown operator names.
func IsGraphWritingOperator(exprType, name string) bool {
	ctx := &ReteBuilderContext{}
	var op any
	switch exprType {
	case "binary":
		if o := ctx.CreateBinaryOperator(name); o != nil {
			op = o
		}
	case "unary":
		if o := ctx.CreateUnaryOperator(name); o != nil {
			op = o
		}
	}
	_, ok := op.(writesGraph)
	return ok
}
