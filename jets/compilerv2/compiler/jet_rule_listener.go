package compiler

import (
	"fmt"
	"os"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// antlr v4 JetRuleListener interface implementation

func (j *JetRuleListener) LogError(msg string) {
	j.errorLog.WriteString(msg)
	j.errorLog.WriteString("\n")
	// The structured record rides alongside the text rather than replacing it.
	// These complaints carry no position: they are raised while walking the
	// tree, from context that knows what is wrong but not where the offending
	// token was, so they resolve to a file only if one is ever attached here.
	j.diagnostics.addMessage(msg)
}

// LogErrorAt logs a semantic error at a buffer line, naming the authored file
// and line when the reader can place it, as a syntax error is named
// (CustomErrorListener.SyntaxError). The diagnostic keeps the buffer line and is
// resolved with the rest at the end of the compile.
func (j *JetRuleListener) LogErrorAt(globalLine int, msg string) {
	text := fmt.Sprintf("error at buffer line %d: %s", globalLine, msg)
	if j.ruleFileReader != nil {
		if fileName, localLine, err := j.ruleFileReader.GetLocalFileAndLine(globalLine); err == nil {
			text = fmt.Sprintf("error in %s at line %d: %s", fileName, localLine, msg)
		}
	}
	j.errorLog.WriteString(text)
	j.errorLog.WriteString("\n")
	j.diagnostics.add(Diagnostic{Severity: SeverityError, GlobalLine: globalLine, Message: "error: " + msg})
}

func (j *JetRuleListener) LogParse(msg string) {
	j.parseLog.WriteString(msg)
	j.parseLog.WriteString("\n")
}

func readRuleFile(basePath, fileName string) (string, error) {
	filePath, err := utils.ConfineFilePath(basePath, fileName)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Trace: Override EnterEveryRule
func (l *JetRuleListener) EnterEveryRule(ctx antlr.ParserRuleContext) {
	// The line LogErrorAt reports for an error found while exiting a node: the
	// last node entered is inside it.
	if ctx != nil && ctx.GetStart() != nil {
		l.currentLine = ctx.GetStart().GetLine()
	}
	// if l.trace {
	// 	fmt.Fprintf(l.parseLog, "Entering rule (%T): %s\n", ctx, ctx.GetText())
	// }
}

// Trace: Override ExitEveryRule
func (l *JetRuleListener) ExitEveryRule(ctx antlr.ParserRuleContext) {
	// if l.trace {
	// 	fmt.Fprintf(l.parseLog, "EXITING RULE (%T): %s\n", ctx, ctx.GetText())
	// }
}
func (l *JetRuleListener) EnterJetrule(ctx *parser.JetruleContext) {
	if l.trace {
		fmt.Fprintf(l.parseLog, "** EnterJetrule\n")
	}
}

func (l *JetRuleListener) ExitJetrule(ctx *parser.JetruleContext) {
	if l.trace {
		fmt.Fprintf(l.parseLog, "** ExitJetrule\n")
	}
	// Compiler post processing and validation
	l.PostProcessJetruleModel()
}
