// jets_mcp_server serves the agentic tool registry over stdio MCP — the
// second exposure of the one Go registry (plan section 6). Spawned as a
// subprocess by its client (Claude Code, criterion 12); it is not how the
// Phase-1 loop reaches a tool.
//
// Usage:
//
//	jets_mcp_server -workspace /path/to/compiled/workspace
//
// The workspace must be compiled (build/classes.json and workspace.db
// present); the adapter resolves the path here so the tools never see one.
package main

import (
	"context"
	"flag"
	"log"

	mcpadapter "github.com/artisoft-io/jetstore/jets/agentic/mcp"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
)

var workspaceDir = flag.String("workspace", "", "path to a compiled workspace checkout (required)")

func main() {
	flag.Parse()
	if *workspaceDir == "" {
		log.Fatal("must provide -workspace, the path to a compiled workspace checkout")
	}
	ws, err := tools.ResolveWorkspace(*workspaceDir)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()
	reg, err := tools.DefaultRegistry()
	if err != nil {
		log.Fatal(err)
	}
	// Serve until the client closes stdin.
	if err := mcpadapter.Run(context.Background(), reg, ws); err != nil {
		log.Fatal(err)
	}
}
