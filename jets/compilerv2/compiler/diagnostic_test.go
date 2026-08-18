package compiler

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Criterion 14 of the agentic_ai Phase 1 plan: a failing compile returns
// diagnostics, each carrying the authored file name and a line local to that
// file.
//
// The error goes in an *imported* file deliberately. An error in the main file
// passes this test for the wrong reason — the main file starts at the top of
// the buffer, so a wrong mapping and a right one agree on roughly the same
// number and the bug hides behind an offset of zero. Only an imported file
// separates "the line ANTLR saw" from "the line someone wrote".
func TestDiagnostics_ErrorInImportedFileNamesThatFile(t *testing.T) {
	dir := t.TempDir()
	// The main file is padded so that a buffer-relative line and a
	// file-relative line cannot coincide by accident.
	write(t, dir, "main.jr", strings.Join([]string{
		"# main line 1",
		"# main line 2",
		"# main line 3",
		"import \"bad.jr\"",
		"# main line 5",
	}, "\n"))
	// Line 3 of the imported file is the offending one: 'class' is not a
	// declaration the grammar accepts here, so ANTLR raises a syntax error.
	write(t, dir, "bad.jr", strings.Join([]string{
		"# imported line 1",
		"# imported line 2",
		"this is not valid jetrule syntax @@@",
		"# imported line 4",
	}, "\n"))

	c := NewCompiler(dir, "main.jr", false, false, true)
	err := c.Compile()
	if err == nil {
		t.Fatal("expected the compile to fail")
	}

	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %T: %v", err, err)
	}
	// The message is unchanged, so callers that log or compare it are
	// unaffected by diagnostics existing.
	if got := err.Error(); got != "compilation failed with errors" {
		t.Errorf("error message changed to %q; existing callers depend on it", got)
	}

	errs := cerr.Errors()
	if len(errs) == 0 {
		t.Fatalf("no error diagnostics; got %d in total: %v", len(cerr.Diagnostics), cerr.Diagnostics)
	}

	var located []Diagnostic
	for _, d := range errs {
		if d.File != "" {
			located = append(located, d)
		}
	}
	if len(located) == 0 {
		t.Fatalf("no diagnostic carries a file; positions never resolved: %v", errs)
	}

	for _, d := range located {
		if d.File != "bad.jr" {
			t.Errorf("diagnostic %v names %q, but the error is in bad.jr", d, d.File)
			continue
		}
		// Line 3 of bad.jr as its author counts, not as the buffer counts.
		// The buffer line is larger, which is exactly what this asserts is
		// not being reported.
		if d.Line != 3 {
			t.Errorf("diagnostic %v reports line %d of %s; the offending line is 3", d, d.Line, d.File)
		}
		if d.GlobalLine <= d.Line {
			t.Errorf("diagnostic %v has global line %d not greater than local line %d, so the "+
				"two cannot have been distinguished", d, d.GlobalLine, d.Line)
		}
	}
}

// A diagnostic that cannot be placed keeps its buffer line rather than
// acquiring a wrong file. CompileBuffer has no reader at all, which is the
// cheapest way to reach that path — and it is a real path, since several
// callers compile a buffer directly.
func TestDiagnostics_UnresolvablePositionIsNotGuessed(t *testing.T) {
	c := NewCompiler("", "buffer.jr", false, false, true)
	err := c.CompileBuffer("this is not valid jetrule syntax @@@\n")
	if err == nil {
		t.Fatal("expected the compile to fail")
	}
	var cerr *CompilationError
	if !errors.As(err, &cerr) {
		t.Fatalf("expected a *CompilationError, got %T", err)
	}
	for _, d := range cerr.Errors() {
		if d.File != "" {
			t.Errorf("diagnostic %v invented file %q with no reader to resolve against", d, d.File)
		}
		if d.GlobalLine == 0 {
			t.Errorf("diagnostic %v dropped its buffer position, leaving nothing to report", d)
		}
	}
}

// The structured severity and the text log's pass/fail decision are derived
// from the same rule, so a message the log treats as a warning must not appear
// as an error in the diagnostics. They are two views of one decision; if they
// disagree, one of them is lying to somebody.
func TestDiagnostics_SeverityMatchesTheTextLogConvention(t *testing.T) {
	for _, tc := range []struct {
		msg  string
		want Severity
	}{
		{"** error: rule without a name encountered, skipping", SeverityError},
		{"warning: identifier 'x' must be defined in a declaration section before use", SeverityWarning},
		{"Syntax error at line 4:0 - mismatched input", SeverityError},
	} {
		if got := severityOf(tc.msg); got != tc.want {
			t.Errorf("severityOf(%q) = %q, want %q", tc.msg, got, tc.want)
		}
		// The compiler decides failure by this same substring; assert the two
		// agree rather than trusting that they do.
		failsCompile := !strings.Contains(tc.msg, "warning:")
		if failsCompile != (tc.want == SeverityError) {
			t.Errorf("message %q: the text log and the severity disagree", tc.msg)
		}
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
