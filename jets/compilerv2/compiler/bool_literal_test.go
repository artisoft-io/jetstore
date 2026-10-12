package compiler

import (
	"errors"
	"strings"
	"testing"
)

// A bool literal outside the accepted spellings is a compile error at the line
// it was written (rules_unit_test_harness I-45, row 1), both as a declaration
// and as a typed literal in a rule. Empty is null and compiles. The lines are in
// an imported file so that a buffer line and a file line cannot coincide.
func TestBoolLiteralOutsideTheSpellingsIsALocatedError(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.jr", strings.Join([]string{
		"# main line 1",
		"# main line 2",
		"import \"lits.jr\"",
	}, "\n"))
	write(t, dir, "lits.jr", strings.Join([]string{
		"# lits line 1",
		"bool okFlag = \"No\";",
		"bool badFlag = \"maybe\";",
		"bool nullFlag = \"\";",
		"[R1]: (?s rdf:type acme:A) -> (?s acme:flag bool(\"nope\"));",
	}, "\n"))

	c := NewCompiler(dir, "main.jr", false, false, true)
	err := c.Compile()
	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %v", err)
	}
	log := c.ErrorLog().String()
	for _, want := range []string{
		`error in lits.jr at line 3: bool literal: "maybe" is not a bool`,
		`error in lits.jr at line 5: bool literal: "nope" is not a bool`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("error log lacks %q:\n%s", want, log)
		}
	}
	located := 0
	for _, d := range cerr.Errors() {
		if d.File == "lits.jr" && (d.Line == 3 || d.Line == 5) {
			located++
		}
	}
	if located != 2 {
		t.Errorf("want two diagnostics located in lits.jr at lines 3 and 5, got %v", cerr.Errors())
	}
	if strings.Contains(log, "okFlag") || strings.Contains(log, "nullFlag") {
		t.Errorf("a valid or empty literal was refused:\n%s", log)
	}
}

// A declared bool is stored without its quotes, as every other typed
// declaration is. It kept them until 2026-10-11, so `bool x = "false";`
// reached the engines as "\"false\"": true on C++, false on Go.
func TestBoolDeclarationIsStoredUnquoted(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.jr", "bool offFlag = \"false\";\n")
	c := NewCompiler(dir, "main.jr", false, false, true)
	if err := c.Compile(); err != nil {
		t.Fatalf("compile: %v\n%s", err, c.ErrorLog().String())
	}
	r, ok := c.listener.resourceManager.ResourceById["offFlag"]
	if !ok {
		t.Fatal("offFlag not declared")
	}
	if r.Type != "bool" || r.Value != "false" {
		t.Errorf("offFlag = (%s, %q), want (bool, \"false\")", r.Type, r.Value)
	}
}
