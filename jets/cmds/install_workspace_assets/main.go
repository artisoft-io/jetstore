// install_workspace_assets copies the JetStore-owned workspace assets into a
// client workspace, and fails rather than overwrite a local change (A21.2,
// A21.3; plan section 3.9). Two directories are installed — data_model/ and
// pipes_config/ — and a conflict in either refuses both.
//
// It runs in Dockerfile.compile_ws, immediately before /app/compile_workspace,
// against the workspace the image was built from:
//
//	install_workspace_assets -w=$WORKSPACE     # WORKSPACES_HOME names the root
//	install_workspace_assets -d /path/to/ws    # or the directory, for local use
//
// Deliberately not part of compile_workspace. The compile is reached from the
// apiserver and from the workspace IDE (jets/apiserver/server.go:288,
// jets/datatable/workspace_helper_functions.go:92) in the client's own account,
// where a knowledge engineer's working copy is not something a compile should
// silently rewrite. The install belongs to the image build, which is where
// JetStore's version of a JetStore-owned file is the authority.
//
// It links no database driver and no CGO: the assets are embedded in the binary,
// and the workspace is the only input.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

var (
	ws     = flag.String("w", "", "workspace prefix (eg: usi_ws); resolved under $WORKSPACES_HOME")
	wsDir  = flag.String("d", "", "workspace directory, as an alternative to -w")
	dryRun = flag.Bool("dry_run", false, "report what would change and write nothing")
	force  = flag.Bool("force", false,
		"adopt the JetStore version over a conflicting file — for a one-time adoption of a "+
			"workspace whose copies predate this mechanism, never for a build")
)

func main() {
	flag.Parse()
	dir, err := workspaceDir()
	if err != nil {
		log.Fatalf("install_workspace_assets: %v", err)
	}
	results, err := workspace_assets.Install(dir, workspace_assets.Options{
		DryRun: *dryRun, Force: *force,
	})
	if err != nil {
		// The conflict message is the point of the exercise; print it whole,
		// and print nothing else — the per-asset listing describes what the
		// install *would* have done, and a refused install did none of it.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// Grouped by directory, in install order, so the output reads as the
	// directory listing it produces rather than as one flat set.
	counts := make(map[string]int, len(workspace_assets.AssetGroups))
	for _, g := range workspace_assets.AssetGroups {
		fmt.Printf("%s:\n", filepath.Join(dir, g.Dir))
		for _, r := range results {
			if r.Dir != g.Dir {
				continue
			}
			counts[g.Dir]++
			fmt.Printf("  %-28s %s\n", r.Name, r.Action)
		}
	}
	var parts []string
	for _, g := range workspace_assets.AssetGroups {
		parts = append(parts, fmt.Sprintf("%d in %s", counts[g.Dir], g.Dir))
	}
	fmt.Printf("install_workspace_assets: %d asset(s) in %s (%s)\n",
		len(results), dir, strings.Join(parts, ", "))
}

func workspaceDir() (string, error) {
	switch {
	case *wsDir != "" && *ws != "":
		return "", fmt.Errorf("give -w or -d, not both")
	case *wsDir != "":
		return *wsDir, nil
	case *ws == "":
		return "", fmt.Errorf("must provide -w workspace (or -d workspace directory)")
	}
	home := os.Getenv("WORKSPACES_HOME")
	if home == "" {
		return "", fmt.Errorf("-w %s needs WORKSPACES_HOME set", *ws)
	}
	return filepath.Join(home, *ws), nil
}
