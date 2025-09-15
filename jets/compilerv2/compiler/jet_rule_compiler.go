package compiler

import (
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the JetRule Compiler using a listener for transformation and validation logic

type Compiler struct {
	listener *JetRuleListener
}

func NewCompiler(basePath string, mainRuleFileName string, trace bool) *Compiler {
	c := &Compiler{
		listener: NewJetRuleListener(basePath, mainRuleFileName),
	}
	c.listener.trace = trace
	return c
}

func (c *Compiler) Compile() error {
	// Read all rule files and imports
	ruleFileReader := NewRuleFileReader(c.listener.basePath, c.listener.mainRuleFileName, readRuleFile)

	// Read all files recursively
	combinedContent, err := ruleFileReader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading rule files: %w", err)
	}
	if c.Trace() {
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
	errorListener := NewCustomErrorListener(c.ParseLog(), c.ErrorLog())
	p.AddErrorListener(errorListener)

	// Build the tree
	tree := p.Jetrule()

	// Finally walk the tree
	antlr.ParseTreeWalkerDefault.Walk(c.listener, tree)
	if c.Trace() {
		fmt.Println("** Compilation successful")
	}
	if c.Trace() && c.ParseLog().Len() > 0 {
		fmt.Println("** Parse Log:\n", c.ParseLog().String())
	}
	if c.ErrorLog().Len() > 0 {
		fmt.Println("** Compilation Errors:\n", c.ErrorLog().String())
	}

	return nil
}

func (c *Compiler) Trace() bool {
	return c.listener.trace
}

func (c *Compiler) ParseLog() *strings.Builder {
	return c.listener.parseLog
}

func (c *Compiler) JetRuleModel() *rete.JetruleModel {
	return c.listener.jetRuleModel
}

func (c *Compiler) ErrorLog() *strings.Builder {
	return c.listener.errorLog
}

func (c *Compiler) OutJsonFileName() string {
	return c.listener.outJsonFileName
}

// All in one function to compile the rules
func CompileJetRuleFiles(basePath string, mainRuleFileName string, trace bool) (*Compiler, error) {
	jrCompiler := NewCompiler(basePath, mainRuleFileName, trace)
	err := jrCompiler.Compile()
	if err != nil {
		return nil, err
	}
	return jrCompiler, nil
}
