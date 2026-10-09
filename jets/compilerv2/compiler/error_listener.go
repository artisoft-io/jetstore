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
	Trace       bool
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
	// allways report syntax errors
	fmt.Fprintf(l.ErrorLog, "Syntax error at line %d:%d - %s\n", line, column, msg)
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
