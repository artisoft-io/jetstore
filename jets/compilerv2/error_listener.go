package main

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
	RuleFileReader *RuleFileReader
	ErrorLog       *strings.Builder
}
func NewCustomErrorListener(rfr *RuleFileReader, errLog *strings.Builder) *CustomErrorListener {
	return &CustomErrorListener{
		RuleFileReader: rfr,
		ErrorLog:      errLog,
	}
}

func (l *CustomErrorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol any,
	line, column int,
	msg string,
	e antlr.RecognitionException,
) {
	fmt.Printf("Syntax error at line %d:%d - %s\n", line, column, msg)
}

func (l *CustomErrorListener) ReportAmbiguity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	exact bool,
	ambigAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	fmt.Printf("Ambiguity detected from %d to %d\n", startIndex, stopIndex)
}

func (l *CustomErrorListener) ReportAttemptingFullContext(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	conflictingAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	fmt.Printf("Attempting full context from %d to %d\n", startIndex, stopIndex)
}

func (l *CustomErrorListener) ReportContextSensitivity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex, prediction int,
	configs *antlr.ATNConfigSet,
) {
	fmt.Printf("Context sensitivity from %d to %d\n", startIndex, stopIndex)
}
