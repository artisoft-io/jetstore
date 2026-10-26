package compiler

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Operator names are checked at compile time.
//
// An operator name reaches the engine as an opaque string: the grammar reduces
// binaryOp and unaryOp to Identifier, and the compiler writes the name to workspace.db.
// Before this check a misspelt name, or one that only the retired Go engine knew,
// compiled cleanly and raised when the engine built the rule
// (create_binary_expr, jets/rete/expr_operator_factory.h).
//
// So the compiler refuses a rule whose filter or consequent calls an operator that the
// engine does not register under that arity. The names are in operators.txt, beside
// this file, which a C++ test (jets/rete/operator_list_test.cc) checks against the
// factory in both directions.

//go:embed operators.txt
var operatorsTxt string

// operatorSpec is one line of operators.txt.
type operatorSpec struct {
	exprType    string // "binary" or "unary", as ExpressionNode.Type
	name        string
	writesGraph bool
}

// engineOperators is operators.txt, parsed once.
var engineOperators = parseOperators(operatorsTxt)

// parseOperators reads operators.txt: comments and blank lines are skipped, and every
// other line is an arity, a name and optionally writes_graph. A malformed line panics,
// since the file is embedded and a test reads it.
func parseOperators(text string) []operatorSpec {
	var out []operatorSpec
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		ok := len(fields) == 2 || (len(fields) == 3 && fields[2] == "writes_graph")
		if !ok || (fields[0] != "binary" && fields[0] != "unary") {
			panic(fmt.Sprintf("operators.txt line %d: want \"binary|unary <name> [writes_graph]\", got %q", i+1, line))
		}
		out = append(out, operatorSpec{exprType: fields[0], name: fields[1], writesGraph: len(fields) == 3})
	}
	return out
}

// lookupOperator returns the operator registered under name for exprType.
func lookupOperator(exprType, name string) (operatorSpec, bool) {
	for _, op := range engineOperators {
		if op.exprType == exprType && op.name == name {
			return op, true
		}
	}
	return operatorSpec{}, false
}

// IsEngineOperator reports whether the rule engine registers an operator named name
// as a binary or a unary operator according to exprType ("binary" or "unary"), which
// is what the compiler accepts. The symbolic renderer asks it too.
func IsEngineOperator(exprType, name string) bool {
	_, ok := lookupOperator(exprType, name)
	return ok
}

// isGraphWritingOperator reports whether the operator registered under name, as a
// binary or a unary operator according to exprType, writes to the graph when
// evaluated. A name not registered under that arity is reported as not writing.
func isGraphWritingOperator(exprType, name string) bool {
	op, ok := lookupOperator(exprType, name)
	return ok && op.writesGraph
}

// unknownOperators returns, in tree order and without repeats, the operators in an
// expression that the engine does not register under their arity, each as
// "<arity> <name>".
func unknownOperators(e *rete.ExpressionNode) []string {
	var out []string
	var walk func(*rete.ExpressionNode)
	walk = func(e *rete.ExpressionNode) {
		if e == nil {
			return
		}
		if e.Type == "binary" || e.Type == "unary" {
			if _, ok := lookupOperator(e.Type, e.Op); !ok {
				if s := e.Type + " " + e.Op; !slices.Contains(out, s) {
					out = append(out, s)
				}
			}
		}
		walk(e.Lhs)
		walk(e.Arg)
		walk(e.Rhs)
	}
	walk(e)
	return out
}

// validateOperatorNames logs an error for every operator a rule's filters and
// consequent expressions call that the engine does not register, and returns false
// when it logged one.
func (s *JetRuleListener) validateOperatorNames(rule *rete.JetruleNode) bool {
	var unknown []string
	add := func(e *rete.ExpressionNode) {
		for _, u := range unknownOperators(e) {
			if !slices.Contains(unknown, u) {
				unknown = append(unknown, u)
			}
		}
	}
	for i := range rule.Antecedents {
		add(rule.Antecedents[i].Filter)
	}
	for i := range rule.Consequents {
		add(rule.Consequents[i].ObjectExpr)
	}
	for _, u := range unknown {
		exprType, name, _ := strings.Cut(u, " ")
		msg := fmt.Sprintf("** error: rule %s (%s): %s is not a %s operator of the rule engine",
			rule.Name, rule.SourceFileName, name, exprType)
		other := map[string]string{"binary": "unary", "unary": "binary"}[exprType]
		if _, ok := lookupOperator(other, name); ok {
			msg += fmt.Sprintf("; it is a %s operator", other)
		}
		s.LogError(msg + " (see jets/compilerv2/compiler/operators.txt)")
	}
	return len(unknown) == 0
}
