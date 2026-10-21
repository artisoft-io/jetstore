package compiler

import (
	"fmt"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// No rule writes to the graph during an iteration.
//
// Both rule engines evaluate an antecedent's filter while they iterate the graph to
// match that antecedent, and insert a rule's consequents only after the visit, by
// salience. That ordering is what lets an iterator run over a graph that does not
// change under it. A filter that calls an operator which inserts into the session
// breaks it: lookup and multi_lookup insert the looked-up rows, range its
// jets:range_value triples, create_entity the entity's jets:key triple. Nothing
// stopped a rule doing that, so the invariant held by convention.
//
// So the compiler refuses an antecedent filter that calls such an operator, anywhere
// in its expression tree. The set comes from the Go operator table: an operator is a
// writer when its type carries the writesGraph marker (rete.IsGraphWritingOperator,
// jets/jetrules/rete/expr_writes_graph.go), and TestWritingOperatorsAreMarked keeps
// the marker honest. Consequent object expressions are not checked: they are
// evaluated after the visit, which is where these operators belong.
//
// It is an error and not a warning. No rule in the four workspaces calls a writing
// operator in a filter (2026-10-21), so the error costs nothing in the field, and a
// rule that did would depend on a write the engines do not order: warnings are not
// shown to anyone who would act on them.

// filterGraphWriters returns, in tree order and without repeats, the operators in a
// filter expression that write to the graph.
func filterGraphWriters(e *rete.ExpressionNode) []string {
	var out []string
	var walk func(*rete.ExpressionNode)
	walk = func(e *rete.ExpressionNode) {
		if e == nil {
			return
		}
		if (e.Type == "binary" || e.Type == "unary") && rete.IsGraphWritingOperator(e.Type, e.Op) && !slices.Contains(out, e.Op) {
			out = append(out, e.Op)
		}
		walk(e.Lhs)
		walk(e.Arg)
		walk(e.Rhs)
	}
	walk(e)
	return out
}

// validateFilterWrites logs an error for every antecedent of rule whose filter calls
// an operator that writes to the graph, and returns false when it logged one.
func (s *JetRuleListener) validateFilterWrites(rule *rete.JetruleNode) bool {
	isValid := true
	for i := range rule.Antecedents {
		writers := filterGraphWriters(rule.Antecedents[i].Filter)
		if len(writers) == 0 {
			continue
		}
		s.LogError(fmt.Sprintf(
			"** error: rule %s (%s): the filter of antecedent %d calls %s, which writes to the graph; "+
				"a filter is evaluated while the engine iterates the graph, and no rule may write to the graph during an iteration: "+
				"compute the value in a consequent and test it in a filter",
			rule.Name, rule.SourceFileName, i+1, strings.Join(writers, ", ")))
		isValid = false
	}
	return isValid
}
