package compiler

// antlr v4 ErrorListner interface implementation

// Example of a custom antlr.ErrorListener implementation in Go (ANTLR v4)
// filepath: /home/michel/projects/repos/jetstore/jets/compilerv2/error_listener.go

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
}

func NewCustomErrorListener(parseLog, errorLog *strings.Builder) *CustomErrorListener {
	return &CustomErrorListener{
		ParseLog: parseLog,
		ErrorLog: errorLog,
	}
}

func (l *CustomErrorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol any,
	line, column int,
	msg string,
	e antlr.RecognitionException,
) {
	fmt.Fprintf(l.ErrorLog, "Syntax error at line %d:%d - %s\n", line, column, msg)
}

func (l *CustomErrorListener) ReportAmbiguity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	exact bool,
	ambigAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	fmt.Fprintf(l.ParseLog, "Ambiguity detected from %d to %d\n", startIndex, stopIndex)
}

func (l *CustomErrorListener) ReportAttemptingFullContext(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	conflictingAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	fmt.Fprintf(l.ParseLog, "Attempting full context from %d to %d\n", startIndex, stopIndex)
}

func (l *CustomErrorListener) ReportContextSensitivity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex, prediction int,
	configs *antlr.ATNConfigSet,
) {
	fmt.Fprintf(l.ParseLog, "Context sensitivity from %d to %d\n", startIndex, stopIndex)
}
