package workspace

import (
	"fmt"
	"os"
)

// The directories a compute-pipes node reads out of the extracted workspace, and
// the one that is present only in some workspaces.
//
// **`workspace.tgz` is the whole of what a node sees.** `SyncComputePipesWorkspace`
// fetches it and `sqlite`, nothing else — so a workspace file that is not in this
// list does not exist as far as a node is concerned, and the operator that wants it
// fails at build time with a path that looks like a deployment problem rather than
// a packaging one.
//
// **`provenance/` was missing and that is what this function exists to fix.** An
// infer operator naming a `provenance_schema_name` loads
// `provenance/<name>.pv.json` at build time to ground its answers against the input
// entity, and the archive carried `workspace_control.json`, `build/` and
// `pipes_config/` and nothing else. The document validated on save, compiled into
// no artefact, and never reached the machine that reads it.
//
// **It is conditional because the directory is new**: one of the four workspaces
// has a `provenance/` today, and `filepath.Walk` on a path that does not exist
// returns an error that fails the whole archive — so listing it unconditionally
// would stop the other three compiling at all. Presence is the test rather than a
// per-workspace setting, because a workspace that has the directory wants it
// shipped and one that does not has nothing to ship.
func workspaceArchivePaths(workspaceHome, workspaceName string) []string {
	root := fmt.Sprintf("%s/%s", workspaceHome, workspaceName)
	paths := []string{
		fmt.Sprintf("%s/workspace_control.json", root),
		fmt.Sprintf("%s/build/", root),
		fmt.Sprintf("%s/pipes_config/", root),
	}
	// Conditional directories: shipped when the workspace has them, silently absent
	// when it does not. Add a row here rather than a branch, so that the next one is
	// a name rather than a change of shape.
	for _, dir := range []string{"provenance"} {
		path := fmt.Sprintf("%s/%s", root, dir)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			paths = append(paths, path+"/")
		}
	}
	return paths
}
