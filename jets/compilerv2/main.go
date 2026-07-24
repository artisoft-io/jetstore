package main

import (
	"flag"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/compilerv2/analyzer"
	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var inputFileName = flag.String("in_file", "", "JetRule file (required)")
var basePath = flag.String("base_path", "", "Base path for in_file, out_file and all imported files (required)")
var runOptions = flag.String("run_options", "predicates,dependencies-rules,dependencies-properties", "Comma-separated list of analysis options: predicates, dependencies-rules, dependencies-properties")
var dependencyPropertyName = flag.String("dependency_property_name", "", "The property name to analyze dependencies for when run_options contains dependencies-rules or dependencies-properties")
var saveJson = flag.Bool("save_json", false, "Save JetRule json output file")
var trace = flag.Bool("trace", false, "Enable trace logging")
var autoAddResources = flag.Bool("a", false, "Enable automatic resource addition when an identifier is not defined")

func main() {
	utils.UseJetStoreLogger()
	inputFileNameSP := flag.String("f", "", "JetRule file (required) short name")
	basePathSP := flag.String("b", "", "Base path for in_file, out_file and all imported files (required) short name")
	saveJson = flag.Bool("s", false, "Save JetRule json output file (short name)")
	trace = flag.Bool("t", false, "Enable trace logging (short name)")
	log.Println("CMD LINE ARGS:", os.Args[1:])
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
	// Check that if len(dependencyPropertyName) > 0 then len(runOptions) > 0
	if *dependencyPropertyName != "" && *runOptions == "" {
		hasErr = true
		errMsg = append(errMsg, "Must provide -run_options when -dependency_property_name is provided")
	}

	if hasErr {
		for _, msg := range errMsg {
			log.Println("**", msg)
		}
		log.Panic("Invalid argument(s)")
	}

	jrCompiler := compiler.NewCompiler(*basePath, *inputFileName, false, *trace, *autoAddResources)
	err := jrCompiler.Compile()
	if err != nil {
		log.Println("** FATAL ERROR during compilation:")
		log.Println(jrCompiler.ErrorLog().String())
		log.Fatal(err)
	}
	if jrCompiler.ErrorLog().Len() > 0 {
		log.Println("** ERROR during compilation:")
		log.Println(jrCompiler.ErrorLog().String())
	}
	log.Println("** Compilation successful")
	if len(*runOptions) > 0 {
		analyzer := analyzer.NewAnalyzer(*basePath, *inputFileName, *runOptions, *dependencyPropertyName, *saveJson, jrCompiler)
		err = analyzer.Analyze()
		if err != nil {
			log.Println("** FATAL ERROR during analysis:")
			log.Fatal(err)
		}
		if *saveJson {
			err := analyzer.SaveModel()
			if err != nil {
				log.Println("** ERROR saving analysis model:", err.Error())
				log.Fatal(err)
			}
		}
		log.Println("** Analysis successful")
	}
	if *saveJson {
		err := jrCompiler.SaveModel()
		if err != nil {
			log.Println("** ERROR saving model:", err.Error())
			log.Fatal(err)
		}
	}
}
