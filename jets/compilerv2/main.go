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

	jrCompiler, err := compiler.CompileJetRuleFiles(*basePath, *inputFileName, *trace)
	if err != nil {
		log.Println("** ERROR during compilation:")
		log.Println(jrCompiler.ErrorLog().String())
		log.Fatal(err)
	}
	log.Println("** Compilation successful")
	if *saveJson {
		log.Println("Saving json to", jrCompiler.OutJsonFileName())
		data, err := jrCompiler.JetRuleModel().ToJson()
		if err != nil {
			log.Println("** ERROR converting to json:", err.Error())
			log.Fatal(err)
		}
		err = os.WriteFile(jrCompiler.OutJsonFileName(), data, 0644)
		if err != nil {
			log.Println("** ERROR saving json:", err.Error())
			log.Fatal(err)
		}
	}
}
