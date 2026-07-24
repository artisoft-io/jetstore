package main

import (
	"flag"
	"log"

	"github.com/artisoft-io/jetstore/jets/utils"

	"github.com/artisoft-io/jetstore/jets/workspace"
)

// Utility to invoke compute_pipes.CompileWorkspace to compile a workspace

// Command Line Arguments
// --------------------------------------------------------------------------------------
var ws = flag.String("w", "", "workspace prefix (eg: jets_ws, required)")
var version = flag.String("v", "", "workspace version (eg: 1768785867275, required)")

func main() {
	utils.UseJetStoreLogger()
	flag.Parse()
	if *ws == "" {
		log.Panic("Must provide -w workspace")
	}
	if *version == "" {
		log.Panic("Must provide -v version")
	}
	// Call the compile workspace function
	_, err := workspace.CompileWorkspace(nil, *ws, *version)
	if err != nil {
		log.Panic(err)
	}
}
