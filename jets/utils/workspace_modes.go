package utils

import (
	"fmt"
	"io/fs"
	"os"
)

// Permissions for the workspace tree, and the invariant that makes them a pair.
//
// # The invariant
//
// **A directory must be at least as reachable as the files inside it.** The
// workspace's files are world-readable, deliberately, because the workspace is
// read by whatever process happens to be running the pipeline. A directory that
// is less permissive than its contents puts every one of those readable files
// behind a door no other identity can open — which is a state that cannot be
// intentional, and is exactly the state the tree was in.
//
// `TestWorkspaceModesAgree` asserts it rather than describing it.
//
// # Why these live here rather than in jets/workspace
//
// **Because three packages write this one tree and two of them disagreed.**
//
//	jets/workspace      compile output            0770 dirs, 0644 files
//	jets/awsi           files fetched from S3     0750 dirs, 0644 files
//	jets/workspace_assets  installed assets       0755 dirs, 0644 files
//
// The first cost a production incident (see jets/workspace/README.md). Fixing it
// where it broke would have left the second as the identical trap one package
// over, waiting for the next caller that reads the tree as a different identity —
// which is precisely how the first one waited eleven months.
//
// `jets/workspace` and `jets/awsi` do not import each other and both already
// import this package, so this is the one place both can name. workspace_assets
// is left alone: it was already correct, it is the odd one out only in that it
// got it right, and changing a correct call site to import a constant is churn.
// If it grows a second mode, move it here too.
const (
	// WorkspaceDirMode is the mode for directories created under
	// WORKSPACES_HOME. The o+rx bits are load-bearing — see above.
	WorkspaceDirMode fs.FileMode = 0755

	// WorkspaceFileMode is the mode for files written there. It is the literal
	// already used at the OpenFile call sites, named here so the invariant has
	// two sides to compare.
	WorkspaceFileMode fs.FileMode = 0644
)

// EnsureWorkspaceDir creates a workspace directory and makes sure it actually
// has WorkspaceDirMode, which `os.MkdirAll` alone does not guarantee.
//
// **`os.MkdirAll`'s mode is a request for directories it creates and says
// nothing about one that is already there** — it returns nil immediately and
// leaves the existing mode untouched. That is documented behaviour and it is
// why passing the right mode was not enough:
//
//	existing  after MkdirAll(0755): -rwx------
//	fresh     after MkdirAll(0755): -rwxr-xr-x
//
// The workspace's `build/` is checked into, or left in, the client repository
// that `Dockerfile.compile_ws` copies wholesale, so it exists before the
// compiler runs and keeps whatever mode the client's checkout gave it. In the
// image that shipped 2026-08-31 that was 0770 on three directories — `build`,
// `build/jet_rules` and `build/jet_rules/clinical_intel`, dated months before
// the compile that rewrote the files inside them — while the 222 files in the
// tree were all world-readable and `table_configs/`, freshly created by the
// asset installer, was correctly 0755. Same image, same run: the mode a
// directory ends up with depends only on whether it had to be created.
//
// So the mode is asserted rather than requested. This is the compiler's own
// output directory and its permissions are the compiler's to state.
func EnsureWorkspaceDir(path string) error {
	if err := os.MkdirAll(path, WorkspaceDirMode); err != nil {
		return err
	}
	// Not conditional on having created it: that is the whole point.
	if err := os.Chmod(path, WorkspaceDirMode); err != nil {
		return fmt.Errorf("while setting mode on %s: %w", path, err)
	}
	return nil
}
