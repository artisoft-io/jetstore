package compiler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// This file contains the JetRule Compiler using a listener for transformation and validation logic

type Compiler struct {
	listener *JetRuleListener
	// ruleFileReader is retained rather than dropped after ReadAll, which is
	// the whole reason a diagnostic can name an authored file: the reader owns
	// the global-to-local line mapping, and by the time errors are reported it
	// used to be out of scope. Nil when CompileBuffer is called directly, in
	// which case positions stay buffer-relative and say so.
	ruleFileReader   *RuleFileReader
	saveJson         bool
	autoAddResources bool
}

func NewCompiler(basePath string, mainRuleFileName string, saveJson, trace, autoAddResources bool) *Compiler {
	c := &Compiler{
		listener:         NewJetRuleListener(basePath, mainRuleFileName),
		saveJson:         saveJson,
		autoAddResources: autoAddResources,
	}
	c.listener.trace = trace
	c.listener.autoAddResources = autoAddResources
	return c
}

func (c *Compiler) Compile() error {
	// Read all rule files and imports
	ruleFileReader := NewRuleFileReader(c.listener.basePath, c.listener.mainRuleFileName, readRuleFile)
	// Keep it: CompileBuffer resolves diagnostics through it at the end of the
	// walk, and it is the only thing that knows which file a buffer line
	// belongs to.
	c.ruleFileReader = ruleFileReader

	// Read all files recursively
	combinedContent, err := ruleFileReader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading rule files: %w", err)
	}
	// if c.Trace() {
	// 	fmt.Printf("** Combined Rule File Content (%d lines):\n%s\n", len(strings.Split(combinedContent, "\n")), combinedContent)
	// }
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
	errorListener.Diagnostics = c.listener.diagnostics
	p.AddErrorListener(errorListener)

	// Build the tree
	tree := p.Jetrule()

	// Finally walk the tree
	var hasError bool
	antlr.ParseTreeWalkerDefault.Walk(c.listener, tree)
	if c.Trace() {
		log.Println("** Compilation successful")
	}
	if c.Trace() && c.ParseLog().Len() > 0 {
		log.Println("** Parse Log:\n", c.ParseLog().String())
	}
	if c.ErrorLog().Len() > 0 {
		errors := strings.Split(c.ErrorLog().String(), "\n")
		log.Println("** Compilation Errors:")
		for _, e := range errors {
			if strings.TrimSpace(e) != "" {
				log.Println(e)
				if !strings.Contains(e, "warning:") {
					hasError = true
				}
			}
		}
	}
	// Resolve buffer lines to authored files once the walk is done, whatever
	// the outcome: a compile that only warned still has diagnostics worth
	// reading, and resolving them only on failure would make Diagnostics()
	// mean different things depending on how the compile ended. Positions the
	// reader cannot place keep their buffer line rather than acquiring a
	// plausible-looking wrong file.
	c.listener.diagnostics.resolve(c.ruleFileReader)

	if c.saveJson {
		err := c.SaveModel()
		if err != nil {
			log.Println("** ERROR saving model:", err.Error())
			return err
		}
	}
	if hasError {
		return &CompilationError{Diagnostics: c.Diagnostics()}
	}
	return nil
}

func (c *Compiler) SaveModel() error {
	// Prevent CWE-73: External Control of File Name or Path.
	outPath, err := utils.ConfineFilePath(c.listener.basePath, c.OutJsonFileName())
	if err != nil {
		log.Println("** ERROR resolving output path:", err.Error())
		return fmt.Errorf("while resolving output path: %w", err)
	}
	log.Println("Saving json to", outPath)
	data, err := c.JetRuleModel().ToJson()
	if err != nil {
		log.Println("** ERROR converting to json:", err.Error())
		return fmt.Errorf("while converting to json: %w", err)
	}
	err = os.WriteFile(outPath, data, 0644)
	if err != nil {
		log.Println("** ERROR saving json:", err.Error())
		return fmt.Errorf("while saving json: %w", err)
	}
	// Save to workspace.db file
	wDb, err := NewWorkspaceDB(context.TODO(), c.listener.basePath)
	if err != nil {
		log.Println("** ERROR creating workspace.db:", err.Error())
		return fmt.Errorf("while creating workspace.db: %w", err)
	}
	err = wDb.SaveJetRuleModel(context.TODO(), c.listener.jetRuleModel)
	if err != nil {
		log.Println("** ERROR saving to workspace.db:", err.Error())
		return fmt.Errorf("while saving to workspace.db: %w", err)
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

// Diagnostics returns the structured form of what ErrorLog carries as text,
// in emission order and including warnings. Populated on success as well as
// on failure, since a clean compile can still have warned.
func (c *Compiler) Diagnostics() []Diagnostic {
	if c.listener == nil || c.listener.diagnostics == nil {
		return nil
	}
	return c.listener.diagnostics.diagnostics
}

// RuleFileReader exposes the reader retained by Compile, so a caller holding a
// buffer line of its own can resolve it. Nil after CompileBuffer.
func (c *Compiler) RuleFileReader() *RuleFileReader {
	return c.ruleFileReader
}

func (c *Compiler) OutJsonFileName() string {
	return c.listener.outJsonFileName
}

// All in one function to compile the rules
func CompileJetRuleFiles(basePath string, mainRuleFileName string, saveJson, trace, autoAddResources bool) (*Compiler, error) {
	log.Println("Compiling JetRule file:", mainRuleFileName, "autoAddResources:", autoAddResources)
	jrCompiler := NewCompiler(basePath, mainRuleFileName, saveJson, trace, autoAddResources)
	err := jrCompiler.Compile()
	if err != nil {
		return nil, err
	}
	return jrCompiler, nil
}
