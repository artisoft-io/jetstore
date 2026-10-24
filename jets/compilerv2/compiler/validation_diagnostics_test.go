package compiler

import (
	"errors"
	"strings"
	"testing"
)

// The rule validation and the class checks wrote to the text log only, so a
// compile they failed returned a CompilationError whose Errors() was empty
// (I-63): a repair loop reading the diagnostics was told the compile failed and
// not why. Each case fails the compile for one reason, and the diagnostic must
// carry the message the text log carries.
func TestValidationErrorsReachDiagnostics(t *testing.T) {
	const header = `
resource rdfType = "rdf:type";
resource ex:Item = "ex:Item";
resource ex:code = "ex:code";
resource ex:out = "ex:out";
`
	cases := []struct {
		name, rules, want string
		inLog             int // times the text log carries it, as before diagnostics
	}{
		{"a consequent variable not in the antecedents",
			"[R1]: (?x rdfType ex:Item) -> (?x ex:out ?y);\n",
			"consequent object expression variable var|?y not found in antecedents", 1},
		{"an antecedent without a variable",
			"[R2]: (ex:Item rdfType ex:Item) -> (ex:Item ex:out 1);\n",
			"antecedent must have at least one of subject, predicate, object as variable", 2},
		{"a filter variable not bound before it",
			"[R3]: (?x rdfType ex:Item).[?z == 1] -> (?x ex:out 1);\n",
			"antecedent filter expression variable var|?z not found in previous antecedents", 1},
		{"a salience that is not an integer",
			"[R4, s=\"high\"]: (?x rdfType ex:Item) -> (?x ex:out 1);\n",
			"invalid salience value '\"high\"' in rule R4", 1},
		{"a base class that is not declared",
			"class ex:Thing2 {\n  $base_classes = [ex:Missing],\n  $data_properties = [ex:code as text]\n};\n" +
				"[R5]: (?x rdfType ex:Item) -> (?x ex:out 1);\n",
			"base class ex:Missing not found for class ex:Thing2", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "main.jr", header+c.rules)
			comp := NewCompiler(dir, "main.jr", false, false, true)
			err := comp.Compile()
			var cerr *CompilationError
			if !errors.As(err, &cerr) {
				t.Fatalf("expected a *CompilationError, got %T: %v", err, err)
			}
			if n := strings.Count(comp.ErrorLog().String(), c.want); n != c.inLog {
				t.Fatalf("the text log carries %q %d times, want %d:\n%s", c.want, n, c.inLog, comp.ErrorLog().String())
			}
			// Once: two of these are written to the text log twice, by two
			// checks of the same condition, and Errors() carries each once
			found := 0
			for _, d := range cerr.Errors() {
				if strings.Contains(d.Message, c.want) {
					found++
				}
			}
			if found != 1 {
				t.Errorf("Errors() carries %q %d times, want once: %v", c.want, found, cerr.Errors())
			}
		})
	}
}
