package utils

import "io/fs"

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
