package main

import (
	"flag"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// Utility to invoke compute_pipes.CompileWorkspace to compile a workspace

// Command Line Arguments
// --------------------------------------------------------------------------------------
var ws = flag.String("w", "", "workspace prefix (eg: jets_ws, required)")
var version = flag.String("v", "", "workspace version (eg: 1768785867275, required)")

func main() {
	flag.Parse()
	if *ws == "" {
		panic("Must provide -w workspace")
	}
	if *version == "" {
		panic("Must provide -v version")
	}
	// Call the compile workspace function
	_, err := workspace.CompileWorkspace(nil, *ws, *version)
	if err != nil {
		panic(err)
	}
}
