package compiler

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const filterWritesHeader = `
resource rdfType = "rdf:type";
resource ex:Item = "ex:Item";
resource ex:code = "ex:code";
resource ex:out = "ex:out";
resource ex:ok = "ex:ok";
resource ex:CFG = "ex:CFG";
`

func compileFilterWritesCase(t *testing.T, rules string) (errs []string) {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "main.jr", filterWritesHeader+rules)
	c := NewCompiler(dir, "main.jr", false, false, true)
	err := c.Compile()
	if err == nil {
		return nil
	}
	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %T: %v", err, err)
	}
	for _, d := range cerr.Errors() {
		errs = append(errs, d.Message)
	}
	return errs
}

func TestFilterWrites_LookupInAFilterIsAnError(t *testing.T) {
	errs := compileFilterWritesCase(t,
		"[R1]: (?x rdfType ex:Item).(?x ex:code ?c).[(ex:CFG lookup ?c) != ?x] -> (?x ex:ok true);\n")
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %d: %v", len(errs), errs)
	}
	for _, want := range []string{"rule R1", "antecedent 2", "calls lookup", "writes to the graph", "during an iteration"} {
		if !strings.Contains(errs[0], want) {
			t.Errorf("error does not mention %q: %s", want, errs[0])
		}
	}
	if strings.Contains(errs[0], "warning:") {
		t.Errorf("must be an error, not a warning: %s", errs[0])
	}
}

// A writing operator nested inside other calls, and a unary one, are found too.
func TestFilterWrites_NestedCallsAreFound(t *testing.T) {
	errs := compileFilterWritesCase(t,
		"[R2]: (?x rdfType ex:Item).(?x ex:code ?c).[(?c > 0) and (not ((to_text (1 range ?c)) == (create_entity ?c)))] -> (?x ex:ok true);\n")
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0], "rule R2") || !strings.Contains(errs[0], "calls range, create_entity,") {
		t.Errorf("error does not name the rule and both operators: %s", errs[0])
	}
}

// The same operators in a consequent object expression, where they belong, compile,
// and so does a filter of operators that only read.
func TestFilterWrites_ConsequentsAndReadingFiltersAreAllowed(t *testing.T) {
	errs := compileFilterWritesCase(t,
		"[R3]: (?x rdfType ex:Item).(?x ex:code ?c).[(?c > 0) and (?x exist_not ex:out)] -> "+
			"(?x ex:out (ex:CFG lookup ?c)).(?x ex:out (ex:CFG multi_lookup ?c)).(?x ex:out (1 range ?c)).(?x ex:out (create_entity ?c));\n")
	if len(errs) != 0 {
		t.Fatalf("expected no error, got %v", errs)
	}
}

func TestFilterWrites_TheWritingOperators(t *testing.T) {
	writers := map[string]string{
		"lookup": "binary", "multi_lookup": "binary", "range": "binary",
		"lookup_rand": "unary", "multi_lookup_rand": "unary", "create_entity": "unary",
	}
	for op, typ := range writers {
		if !isGraphWritingOperator(typ, op) {
			t.Errorf("%s %s writes to the graph and is not reported as writing", typ, op)
		}
	}
	for _, c := range [][2]string{
		{"binary", "exist"}, {"binary", "exist_not"}, {"binary", "size_of"}, {"binary", "max_of"},
		{"binary", "substring_of"}, {"binary", "=="}, {"unary", "create_uuid_resource"},
		{"unary", "create_resource"}, {"unary", "create_literal"}, {"unary", "not"},
		{"unary", "raise_exception"}, {"binary", "no_such_operator"},
		// a name under the wrong arity is not that operator
		{"unary", "lookup"}, {"binary", "create_entity"}, {"identifier", "lookup"},
	} {
		if isGraphWritingOperator(c[0], c[1]) {
			t.Errorf("%s %s is reported as writing to the graph", c[0], c[1])
		}
	}
}

// TestWritingOperatorsAreMarked asserts that operators.txt marks exactly the six
// operators that insert into the session, so that removing a mark, or marking an
// operator by mistake, is a test failure and not a silent change to what a filter may
// call. An operator added to the engine that inserts into the session must be marked,
// and this list extended, in the same change: operators.txt says why each is marked.
func TestWritingOperatorsAreMarked(t *testing.T) {
	want := []string{
		"binary lookup", "binary multi_lookup", "binary range",
		"unary create_entity", "unary lookup_rand", "unary multi_lookup_rand",
	}
	var got []string
	for _, op := range engineOperators {
		if op.writesGraph {
			got = append(got, op.exprType+" "+op.name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("operators marked writes_graph in operators.txt:\n got  %v\n want %v", got, want)
	}
}
