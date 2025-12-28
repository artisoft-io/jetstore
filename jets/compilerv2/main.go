package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var inputFileName = flag.String("in_file", "", "JetRule file (required)")
var basePath = flag.String("base_path", "", "Base path for in_file, out_file and all imported files (required)")
var saveJson = flag.Bool("save_json", false, "Save JetRule json output file")
var trace = flag.Bool("trace", false, "Enable trace logging")
var autoAddResources = flag.Bool("a", false, "Enable automatic resource addition when an identifier is not defined")

func main() {
	inputFileNameSP := flag.String("f", "", "JetRule file (required) short name")
	basePathSP := flag.String("b", "", "Base path for in_file, out_file and all imported files (required) short name")
	saveJson = flag.Bool("s", false, "Save JetRule json output file (short name)")
	trace = flag.Bool("t", false, "Enable trace logging (short name)")
	fmt.Println("CMD LINE ARGS:", os.Args[1:])
	flag.Parse()
	if *inputFileName == "" && *inputFileNameSP != "" {
		inputFileName = inputFileNameSP
	}
	if *basePath == "" && *basePathSP != "" {
		basePath = basePathSP
	}
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

	jrCompiler, err := compiler.CompileJetRuleFiles(*basePath, *inputFileName, *saveJson, *trace, *autoAddResources)
	if err != nil {
		log.Println("** FATAL ERROR during compilation:")
		log.Println(jrCompiler.ErrorLog().String())
		log.Fatal(err)
	} else {
		if jrCompiler.ErrorLog().Len() > 0 {
			log.Println("** ERROR during compilation:")
			log.Println(jrCompiler.ErrorLog().String())
		}
	}
	log.Println("** Compilation successful")
}
