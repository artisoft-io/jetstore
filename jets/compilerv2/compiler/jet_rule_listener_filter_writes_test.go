package compiler

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
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
		if !rete.IsGraphWritingOperator(typ, op) {
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
		if rete.IsGraphWritingOperator(c[0], c[1]) {
			t.Errorf("%s %s is reported as writing to the graph", c[0], c[1])
		}
	}
}

// insertingMethods are the calls that insert into the rdf session: the session's own
// inserts, and the lookup table methods that insert the rows they return.
// ReteSession.RaiseException is left out deliberately: it is the error path, see
// jets/jetrules/rete/expr_writes_graph.go.
var insertingMethods = []string{
	"Insert", "InsertInferred", "InsertInferredFor",
	"Lookup", "MultiLookup", "LookupRand", "MultiLookupRand",
}

// TestWritingOperatorsAreMarked reads the Go engine's operator sources and asserts
// that an operator type whose Eval calls an inserting method carries the writesGraph
// marker, and that a marked type still calls one. A new operator that inserts into
// the session, and has no marker, would otherwise be allowed in a filter silently.
// It sees direct calls in Eval only: an operator that inserts through a helper of its
// own is not found, and should be marked by hand.
func TestWritingOperatorsAreMarked(t *testing.T) {
	dir := filepath.Join("..", "..", "jetrules", "rete")
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no Go source in %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	inserting := map[string]bool{}
	marked := map[string]bool{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", f, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			ident, ok := star.X.(*ast.Ident)
			if !ok {
				continue
			}
			recv := ident.Name
			switch fn.Name.Name {
			case "writesGraph":
				marked[recv] = true
			case "Eval":
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if call, ok := n.(*ast.CallExpr); ok {
						if sel, ok := call.Fun.(*ast.SelectorExpr); ok && slices.Contains(insertingMethods, sel.Sel.Name) {
							inserting[recv] = true
						}
					}
					return true
				})
			}
		}
	}
	if len(inserting) == 0 {
		t.Fatal("found no operator that inserts into the session; the source scan is broken")
	}
	for op := range inserting {
		if !marked[op] {
			t.Errorf("%s.Eval inserts into the session and %s has no writesGraph marker (jets/jetrules/rete/expr_writes_graph.go)", op, op)
		}
	}
	for op := range marked {
		if !inserting[op] {
			t.Errorf("%s has the writesGraph marker and its Eval inserts nothing", op)
		}
	}
}
