package compiler

import (
	"fmt"
	"strings"
)

// Diagnostics as data, alongside the text error log rather than instead of it.
//
// The compiler was built for a command-line tool that prints its log and exits,
// and for that consumer returning a bare "compilation failed with errors" is
// the right design. It stops being the right design the moment a second
// consumer wants the diagnostics as data — the workspace IDE, which has a file
// open and a place to put a marker, and the agentic repair loop, which has to
// tell a model what to fix and where. Feeding back "somewhere in this workspace"
// is a materially worse repair prompt than a file and a line.
//
// Two properties matter and are easy to lose:
//
//   - The text log is unchanged, byte for byte. jets/compilerv2/main.go and
//     jets/workspace/compile_workspace_v2.go both read it, and neither should
//     have to care that this exists.
//   - A diagnostic names the *authored* file and a line within it. Rule files
//     are textually concatenated with their imports before parsing, so ANTLR
//     reports a line in a buffer nobody wrote; line 4,213 of that buffer is not
//     a location anyone can act on. RuleFileReader.GetLocalFileAndLine maps it
//     back, and until now had no callers at all.

// Severity of a diagnostic. The compiler's own convention is textual — a
// message containing "warning:" is a warning and everything else fails the
// compile — and that convention is preserved rather than replaced, so the two
// cannot disagree.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Diagnostic is one compiler complaint, located where it can be acted on.
//
// Positions are optional because not every complaint has one. Syntax errors
// come from ANTLR with a buffer line and column; semantic complaints are
// written by the listener as free text with no position at all, and inventing
// one for them would be worse than admitting there isn't one. File is empty
// and Line is zero in that case, and a consumer should say "in this
// compilation" rather than pointing somewhere arbitrary.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	// File is the authored rule file, resolved from GlobalLine. Empty when the
	// diagnostic carries no position, or when the mapping could not resolve it.
	File string `json:"file,omitempty"`
	// Line is 1-based and local to File. Zero when File is empty.
	Line int `json:"line,omitempty"`
	// Column is 0-based, as ANTLR reports it.
	Column int `json:"column,omitempty"`
	// GlobalLine is 1-based into the concatenated buffer the parser saw. Kept
	// after resolution because it is what reproduces a report against a dumped
	// buffer, which is the only way to debug the mapping itself.
	GlobalLine int    `json:"global_line,omitempty"`
	Message    string `json:"message"`
}

func (d Diagnostic) String() string {
	switch {
	case d.File != "":
		return fmt.Sprintf("%s: %s:%d:%d: %s", d.Severity, d.File, d.Line, d.Column, d.Message)
	case d.GlobalLine > 0:
		return fmt.Sprintf("%s: <buffer>:%d:%d: %s", d.Severity, d.GlobalLine, d.Column, d.Message)
	default:
		return fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}
}

// severityOf classifies a message the way CompileBuffer already does, so the
// structured severity and the text log's pass/fail decision cannot drift.
func severityOf(msg string) Severity {
	if strings.Contains(msg, "warning:") {
		return SeverityWarning
	}
	return SeverityError
}

// CompilationError is what a failing Compile returns. Its Error() string is
// exactly what the compiler returned before diagnostics existed, so callers
// comparing or logging the message are unaffected; the diagnostics ride
// alongside for callers that want them.
type CompilationError struct {
	Diagnostics []Diagnostic
}

func (e *CompilationError) Error() string {
	return "compilation failed with errors"
}

// Errors returns just the diagnostics that failed the compile, which is what a
// repair prompt should be built from — warnings are noise in that context.
func (e *CompilationError) Errors() []Diagnostic {
	out := make([]Diagnostic, 0, len(e.Diagnostics))
	for _, d := range e.Diagnostics {
		if d.Severity == SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// diagnosticSink collects diagnostics from the two places that produce them —
// the ANTLR error listener, which has positions, and the tree listener, which
// does not. Shared by pointer so both write to one list in emission order.
type diagnosticSink struct {
	diagnostics []Diagnostic
}

func (s *diagnosticSink) add(d Diagnostic) {
	if s == nil {
		return
	}
	s.diagnostics = append(s.diagnostics, d)
}

// addMessage records a positionless complaint, classifying it the same way the
// text log is classified.
func (s *diagnosticSink) addMessage(msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	s.add(Diagnostic{Severity: severityOf(msg), Message: msg})
}

// resolve maps each diagnostic's buffer line back to the authored file and a
// line within it. Diagnostics without a position are left alone, and a line the
// reader cannot place keeps its GlobalLine rather than acquiring a wrong file —
// an unresolvable position is reported honestly, never guessed.
func (s *diagnosticSink) resolve(r *RuleFileReader) {
	if s == nil || r == nil {
		return
	}
	for i := range s.diagnostics {
		d := &s.diagnostics[i]
		if d.GlobalLine <= 0 {
			continue
		}
		fileName, localLine, err := r.GetLocalFileAndLine(d.GlobalLine)
		if err != nil {
			continue
		}
		d.File, d.Line = fileName, localLine
	}
}
