package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// antlr v4 JetRuleListener interface implementation

// All in one method to compile the main rule file and all its imports and return the listener
func CompileJetRuleFiles(basePath string, mainRuleFileName string, trace bool) (*JetRuleListener, error) {
	compiler := NewJetRuleListener(basePath, mainRuleFileName)
	compiler.trace = trace
	err := compiler.Compile()
	if err != nil {
		return nil, fmt.Errorf("error compiling %s: %w", mainRuleFileName, err)
	}
	if trace {
		fmt.Println("** Compilation successful")
	}
	if trace && compiler.parseLog.Len() > 0 {
		fmt.Println("** Parse Log:\n", compiler.parseLog.String())
	}
	if compiler.errorLog.Len() > 0 {
		fmt.Println("** Errors:\n", compiler.errorLog.String())
	}
	// if trace {
	// 	fmt.Printf("** Generated model has %d rules, %d resources, %d lookup tables\n",
	// 		len(compiler.jetRuleModel.Jetrules),
	// 		len(compiler.jetRuleModel.Resources),
	// 		len(compiler.jetRuleModel.LookupTables))
	// }
	return compiler, nil
}

// JetRuleListener to build the tree
type JetRuleListener struct {
	*parser.BaseJetRuleListener
	// Input
	mainRuleFileName string
	basePath         string

	// Parse Tree Output
	outJsonFileName string
	jetRuleModel    *rete.JetruleModel

	// Internal state
	currentRuleFileName string
	currentClass         rete.ClassNode
	parseLog            *strings.Builder
	errorLog            *strings.Builder
	trace               bool
}

func NewJetRuleListener(basePath string, mainRuleFileName string) *JetRuleListener {
	outJsonFileName := strings.TrimSuffix(mainRuleFileName, ".jetrule") + ".json"
	return &JetRuleListener{
		mainRuleFileName: mainRuleFileName,
		basePath:         basePath,
		outJsonFileName:  outJsonFileName,
		jetRuleModel:     rete.NewJetruleModel(),
		parseLog:         &strings.Builder{},
		errorLog:         &strings.Builder{},
	}
}

func (j *JetRuleListener) Compile() error {
	// Read all rule files and imports
	ruleFileReader := NewRuleFileReader(j.basePath, j.mainRuleFileName, readRuleFile)

	// Read all files recursively
	combinedContent, err := ruleFileReader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading rule files: %w", err)
	}
	if j.trace {
		fmt.Printf("** Combined Rule File Content (%d lines):\n%s\n", len(strings.Split(combinedContent, "\n")), combinedContent)
	}

	// Setup the input
	is := antlr.NewInputStream(combinedContent)

	// Create the Lexer
	lexer := parser.NewJetRuleLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Create the Parser
	p := parser.NewJetRuleParser(stream)
	p.BuildParseTrees = true
	p.RemoveErrorListeners() // remove default ConsoleErrorListener
	errorListener := NewCustomErrorListener(ruleFileReader, j.errorLog)
	p.AddErrorListener(errorListener)

	// Build the tree
	tree := p.Jetrule()

	// Finally walk the tree
	antlr.ParseTreeWalkerDefault.Walk(j, tree)

	// Check for errors
	if j.errorLog.Len() > 0 {
		return fmt.Errorf("compilation errors:\n%s", j.errorLog.String())
	}
	return nil
}

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
	// 	fmt.Fprintf(l.parseLog, "Entering rule: %s\n", ctx.GetText())
	// }
}

// Trace: Override ExitEveryRule
func (l *JetRuleListener) ExitEveryRule(ctx antlr.ParserRuleContext) {
	// if l.trace {
	// 	fmt.Fprintf(l.parseLog, "EXITING RULE: %s\n", ctx.GetText())
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
