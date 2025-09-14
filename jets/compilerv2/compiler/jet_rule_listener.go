package compiler

import (
	"fmt"
	"os"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
)

// antlr v4 JetRuleListener interface implementation

func (j *JetRuleListener) LogError(msg string) {
	j.errorLog.WriteString(msg)
	j.errorLog.WriteString("\n")
}

func (j *JetRuleListener) LogParse(msg string) {
	j.parseLog.WriteString(msg)
	j.parseLog.WriteString("\n")
}

func readRuleFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Trace: Override EnterEveryRule
func (l *JetRuleListener) EnterEveryRule(ctx antlr.ParserRuleContext) {
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
}
