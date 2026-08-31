package workspace

import "io/fs"

// The permissions this package puts on the workspace it writes, and the reason
// the directory mode is not the obvious one.
//
// # The invariant
//
// **A directory must be at least as reachable as the files inside it.** Every
// file this package writes is 0644 — world-readable, deliberately, because the
// workspace is read by whatever process happens to be running the pipeline.
// Directories were 0770 from e84a2af2 (2025-09-28, "Compiler v2 integrated in
// apiserver") until this constant, which means every one of those world-readable
// files sat behind a directory no "other" could traverse. `workspaceModesAgree`
// in modes_test.go is that invariant, asserted rather than described.
//
// The asymmetry is the evidence that 0770 was an oversight rather than a policy.
// A package that meant to restrict the workspace to its owner and group would
// have written 0640 on the files; this one writes 0644 and then locks the door
// in front of them.
//
// # What it cost, and why nothing caught it for eleven months
//
// 0770 is invisible while the process that writes the workspace is the process
// that reads it, which is every case there was until 2026-08-22. `5f9b1bc8` added
// `ensureLocalRepoSeeded`, which copies the image's baked workspace into
// WORKSPACES_HOME on the native lambda's cold start — the first time anything
// read this directory as a *different* identity from the one that compiled it,
// inside `compile_ws`, as root. It failed on the first entry of the walk:
//
//	error while synching workspace files from db: while seeding
//	/tmp/workspaces/jets_ws from /workspaces/jets_ws: open build: permission denied
//
// `build` is not special. It is first because `os.CopyFS` walks in lexical order
// and `build` is the first directory the compiler made. Every other directory it
// creates would have failed next.
//
// # Why no test reproduces it, and what the test does instead
//
// **A unit test cannot see this**, and it is worth knowing why before writing one
// that pretends to. A test process is the owner of the directories it creates, and
// the owner's permission bits are consulted first — so 0770 behaves exactly like
// 0755 for the process that made it, whatever the mode says. Reproducing the
// failure needs a second uid, which a `go test` run does not have.
//
// `seed_local_repo_test.go` illustrates the trap rather than avoiding it: its
// fixture builds `build/` with **0755**, a mode production never used. The test
// was green against a permission the code did not write.
//
// So the guard is the invariant above rather than the symptom: the modes are
// compared to each other, which is checkable without a second identity, and it
// is the comparison that was wrong.
const (
	// workspaceDirMode is the mode for directories this package creates under
	// WORKSPACES_HOME. o+rx is load-bearing — see above.
	workspaceDirMode fs.FileMode = 0755

	// workspaceFileMode is the mode for files this package writes there. It is
	// the literal already used at every OpenFile call in
	// compile_workspace_v2.go, named here so the invariant has two sides to
	// compare. Those call sites are left spelling it out: rewriting nine of them
	// to reference this would be a larger diff than the defect.
	workspaceFileMode fs.FileMode = 0644
)
