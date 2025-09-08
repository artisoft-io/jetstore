package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
)

// Command Line Arguments
// --------------------------------------------------------------------------------------
var inputFileName = flag.String("in_file", "", "JetRule file (required)")
var basePath = flag.String("base_path", "", "Base path for in_file, out_file and all imported files (required)")
var saveJson = flag.Bool("save_json", false, "Save JetRule json output file")
var trace = flag.Bool("trace", false, "Enable trace logging")
var reImportPattern = regexp.MustCompile(`import\s*"([a-zA-Z0-9_\/.-]*)"`)

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
	compiler.trace = *trace
	fmt.Println("** Compiling", *inputFileName, "in base path", *basePath, "trace =", *trace)
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
