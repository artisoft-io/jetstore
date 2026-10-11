package compiler

// antlr v4 ErrorListner interface implementation

// Example of a custom antlr.ErrorListener implementation in Go (ANTLR v4)
// filepath: /home/michel/projects/repos/jetstore_agentic_ai/jetstore_ai/jets/compilerv2/error_listener.go

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
)

// CustomErrorListener implements antlr.ErrorListener
type CustomErrorListener struct {
	antlr.DefaultErrorListener // Embeds default implementation
	ParseLog                   *strings.Builder
	ErrorLog                   *strings.Builder
	// Diagnostics receives the structured form of each syntax error. This is
	// the only producer that has a position, since ANTLR reports one; nil is
	// tolerated so the listener stays usable on its own.
	Diagnostics *diagnosticSink
	// Reader, when set, turns the buffer line ANTLR reports into the authored
	// file and a line within it, for the text log as well as the diagnostic.
	// Nil when a buffer is compiled directly, and the log then says buffer line.
	Reader *RuleFileReader
	Trace  bool
}

func NewCustomErrorListener(parseLog, errorLog *strings.Builder, trace bool) *CustomErrorListener {
	return &CustomErrorListener{
		ParseLog: parseLog,
		ErrorLog: errorLog,
		Trace:    trace,
	}
}

func (l *CustomErrorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol any,
	line, column int,
	msg string,
	e antlr.RecognitionException,
) {
	// Always report syntax errors, and where the author can find them. The line
	// ANTLR reports is into the buffer the reader built by inlining every
	// import, so for a rule file that imports anything it is a line nobody
	// wrote: an error at line 90 of a file importing a 62-line model was
	// reported as line 154. The column is ANTLR's, counted from the line's first
	// non-blank character, because the reader trims each line it copies.
	if fileName, localLine, ok := l.locate(line); ok {
		fmt.Fprintf(l.ErrorLog, "Syntax error in %s at line %d:%d - %s\n", fileName, localLine, column, msg)
	} else {
		fmt.Fprintf(l.ErrorLog, "Syntax error at buffer line %d:%d - %s\n", line, column, msg)
	}
	// The line ANTLR reports is into the concatenated buffer, not into any file
	// anyone wrote. It is recorded as GlobalLine here and mapped to an authored
	// file and local line once the compile finishes and the reader is at hand.
	l.Diagnostics.add(Diagnostic{
		Severity:   SeverityError,
		GlobalLine: line,
		Column:     column,
		Message:    msg,
	})
}

// locate maps a buffer line to its authored file, or reports that it cannot.
func (l *CustomErrorListener) locate(globalLine int) (string, int, bool) {
	if l.Reader == nil {
		return "", 0, false
	}
	fileName, localLine, err := l.Reader.GetLocalFileAndLine(globalLine)
	if err != nil {
		return "", 0, false
	}
	return fileName, localLine, true
}

func (l *CustomErrorListener) ReportAmbiguity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	exact bool,
	ambigAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	if l.Trace {
		fmt.Fprintf(l.ParseLog, "Ambiguity detected from %d to %d\n", startIndex, stopIndex)
	}
}

func (l *CustomErrorListener) ReportAttemptingFullContext(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	conflictingAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	if l.Trace {
		fmt.Fprintf(l.ParseLog, "Attempting full context from %d to %d\n", startIndex, stopIndex)
	}
}

func (l *CustomErrorListener) ReportContextSensitivity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex, prediction int,
	configs *antlr.ATNConfigSet,
) {
	if l.Trace {
		fmt.Fprintf(l.ParseLog, "Context sensitivity from %d to %d\n", startIndex, stopIndex)
	}
}
