package compiler

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains the JetRule Compiler using a listener for transformation and validation logic

type Compiler struct {
	listener *JetRuleListener
	saveJson bool
	autoAddResources bool
}

func NewCompiler(basePath string, mainRuleFileName string, saveJson, trace, autoAddResources bool) *Compiler {
	c := &Compiler{
		listener: NewJetRuleListener(basePath, mainRuleFileName),
		saveJson: saveJson,
		autoAddResources: autoAddResources,
	}
	c.listener.trace = trace
	c.listener.autoAddResources = autoAddResources
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
	return c.CompileBuffer(combinedContent)
}

func (c *Compiler) CompileBuffer(combinedContent string) error {
	// Setup the input
	is := antlr.NewInputStream(combinedContent)

	// Create the Lexer
	lexer := parser.NewJetRuleLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Create the Parser
	p := parser.NewJetRuleParser(stream)
	p.BuildParseTrees = true
	p.RemoveErrorListeners() // remove default ConsoleErrorListener
	errorListener := NewCustomErrorListener(c.ParseLog(), c.ErrorLog(), false /* c.Trace */)
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
	if c.saveJson {
		outPath := fmt.Sprintf("%s/%s", c.listener.basePath, c.OutJsonFileName())
		log.Println("Saving json to", outPath)
		data, err := c.JetRuleModel().ToJson()
		if err != nil {
			log.Println("** ERROR converting to json:", err.Error())
			log.Fatal(err)
		}
		err = os.WriteFile(outPath, data, 0644)
		if err != nil {
			log.Println("** ERROR saving json:", err.Error())
			log.Fatal(err)
		}
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
func CompileJetRuleFiles(basePath string, mainRuleFileName string, saveJson, trace, autoAddResources bool) (*Compiler, error) {
	jrCompiler := NewCompiler(basePath, mainRuleFileName, saveJson, trace, autoAddResources)
	err := jrCompiler.Compile()
	if err != nil {
		return nil, err
	}
	return jrCompiler, nil
}
