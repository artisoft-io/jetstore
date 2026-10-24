package compiler

import (
	"strings"
	"testing"
)

// The list parses, has no repeat, and has the size of the C++ factory on 2026-10-22:
// 33 binary and 25 unary operators. OperatorList in jets/rete/operator_list_test.cc is
// what proves the list equals the factory; this only stops a list that parsed as near
// empty from making every other test here vacuous.
func TestOperators_TheList(t *testing.T) {
	count := map[string]int{}
	seen := map[string]bool{}
	for _, op := range engineOperators {
		key := op.exprType + " " + op.name
		if seen[key] {
			t.Errorf("%s is listed twice in operators.txt", key)
		}
		seen[key] = true
		count[op.exprType]++
	}
	if count["binary"] != 33 || count["unary"] != 25 {
		t.Errorf("operators.txt lists %d binary and %d unary operators, want 33 and 25", count["binary"], count["unary"])
	}
}

func TestOperators_ParseRefusesAMalformedLine(t *testing.T) {
	for _, line := range []string{"ternary x", "binary", "binary x y", "unary x writes_graph extra"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("parseOperators accepted %q", line)
				}
			}()
			parseOperators("# comment\n\n" + line + "\n")
		}()
	}
	got := parseOperators("# c\n  binary  +  \nunary lookup_rand writes_graph\n")
	if len(got) != 2 || got[0] != (operatorSpec{"binary", "+", false}) || got[1] != (operatorSpec{"unary", "lookup_rand", true}) {
		t.Errorf("parseOperators: got %+v", got)
	}
}

func TestOperators_UnknownNamesAreErrors(t *testing.T) {
	cases := []struct {
		name, rules string
		want        []string
	}{
		// a misspelt aggregate in a consequent: sum_value for sum_values
		{"misspelt binary in a consequent",
			"[R1]: (?x rdfType ex:Item).(?x ex:code ?c) -> (?x ex:out (?c sum_value ex:code));\n",
			[]string{"rule R1", "sum_value is not a binary operator"}},
		{"unknown unary in a filter",
			"[R2]: (?x rdfType ex:Item).(?x ex:code ?c).[(to_upper_case ?c) == \"A\"] -> (?x ex:ok true);\n",
			[]string{"rule R2", "to_upper_case is not a unary operator"}},
		// names the retired Go engine registered and C++ never did
		{"an operator only the Go engine had",
			"[R3]: (?x rdfType ex:Item).(?x ex:code ?c) -> (?x ex:out (?x max_head_of ex:code));\n",
			[]string{"rule R3", "max_head_of is not a binary operator"}},
		{"another operator only the Go engine had",
			"[R4]: (?x rdfType ex:Item).(?x ex:code ?c) -> (?x ex:out (to_date ?c));\n",
			[]string{"rule R4", "to_date is not a unary operator"}},
		// toText was a grammar keyword no engine registered; it is now an
		// identifier, and still not an operator (to_text is)
		{"toText, the dropped keyword",
			"[R5]: (?x rdfType ex:Item).(?x ex:code ?c) -> (?x ex:out (toText ?c));\n",
			[]string{"rule R5", "toText is not a unary operator"}},
		{"a binary operator called as unary",
			"[R6]: (?x rdfType ex:Item).(?x ex:code ?c) -> (?x ex:out (lookup ?c));\n",
			[]string{"rule R6", "lookup is not a unary operator", "it is a binary operator"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := compileFilterWritesCase(t, c.rules)
			if len(errs) != 1 {
				t.Fatalf("expected one error, got %d: %v", len(errs), errs)
			}
			for _, w := range c.want {
				if !strings.Contains(errs[0], w) {
					t.Errorf("error does not mention %q: %s", w, errs[0])
				}
			}
			if strings.Contains(errs[0], "warning:") {
				t.Errorf("must be an error, not a warning: %s", errs[0])
			}
		})
	}
}

// One error per operator, however often the rule calls it, and every unknown one named.
func TestOperators_EachUnknownNameOnce(t *testing.T) {
	errs := compileFilterWritesCase(t,
		"[R7]: (?x rdfType ex:Item).(?x ex:code ?c).[(no_op1 ?c) and (no_op1 ?c)] -> (?x ex:out (?c no_op2 (no_op1 ?c)));\n")
	if len(errs) != 2 {
		t.Fatalf("expected two errors, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "no_op1 is not a unary") || !strings.Contains(errs[1], "no_op2 is not a binary") {
		t.Errorf("errors do not name both operators in tree order: %v", errs)
	}
}

// Every listed operator compiles, in a filter where it reads and in a consequent.
func TestOperators_EveryListedNameCompiles(t *testing.T) {
	var b strings.Builder
	for i, op := range engineOperators {
		var expr string
		if op.exprType == "binary" {
			expr = "(?c " + op.name + " ex:code)"
		} else {
			expr = "(" + op.name + " ?c)"
		}
		filter := ""
		if !op.writesGraph {
			filter = ".[" + expr + "]"
		}
		b.WriteString("[OK" + string(rune('A'+i/26)) + string(rune('A'+i%26)) + "]: (?x rdfType ex:Item).(?x ex:code ?c)" +
			filter + " -> (?x ex:out " + expr + ");\n")
	}
	if errs := compileFilterWritesCase(t, b.String()); len(errs) != 0 {
		t.Fatalf("expected no error, got %d: %v", len(errs), errs)
	}
}
