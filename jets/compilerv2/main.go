package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var inputFileName = flag.String("in_file", "", "JetRule file (required)")
var basePath = flag.String("base_path", "", "Base path for in_file, out_file and all imported files (required)")
var saveJson = flag.Bool("save_json", false, "Save JetRule json output file")
var reImportPattern = regexp.MustCompile(`import\s*"([a-zA-Z0-9_\/.-]*)"`)

// JetRuleListener to build the tree
type JetRuleListener struct {
	*parser.BaseJetRuleListener
	// Input
	mainRuleFileName string
	basePath         string

	// Output
	outJsonFileName string

	// Internal
	parseLog *strings.Builder
	errorLog *strings.Builder
}

func NewJetRuleListener(basePath string, mainRuleFileName string) *JetRuleListener {
	outJsonFileName := strings.TrimSuffix(mainRuleFileName, ".jetrule") + ".json"
	return &JetRuleListener{
		mainRuleFileName: mainRuleFileName,
		basePath:         basePath,
		outJsonFileName:  outJsonFileName,
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

func readRuleFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func main() {
	saveJson = flag.Bool("s", false, "Save JetRule json output file (short name)")
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	hasErr := false
	var errMsg []string
	if *inputFileName == "" {
		hasErr = true
		errMsg = append(errMsg, "Must provide -in_file")
	}
	if *basePath == "" {
		hasErr = true
		errMsg = append(errMsg, "Must provide -base_path")
	}
	// Add more arg check HERE

	if hasErr {
		for _, msg := range errMsg {
			fmt.Println("**", msg)
		}
		panic("Invalid argument(s)")
	}

	compiler := NewJetRuleListener(*basePath, *inputFileName)
	err := compiler.Compile()
	if err != nil {
		fmt.Println("** ERROR during compilation:")
		fmt.Println(compiler.errorLog.String())
		panic(err)
	}
	fmt.Println("** Compilation successful")
	if *saveJson {
		fmt.Println("** Saving json to", compiler.outJsonFileName)
		err = os.WriteFile(compiler.outJsonFileName, []byte(compiler.parseLog.String()), 0644)
		if err != nil {
			fmt.Println("** ERROR saving json:", err.Error())
			panic(err)
		}
	}
}

func (j *JetRuleListener) logError(msg string) {
	j.errorLog.WriteString(msg)
	j.errorLog.WriteString("\n")
}
func (j *JetRuleListener) logParse(msg string) {
	j.parseLog.WriteString(msg)
	j.parseLog.WriteString("\n")
}
