package utils

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The invariant that would have caught the native lambda's cold-start failure,
// asserted at the only level where it is checkable. See workspace_modes.go for why the
// symptom itself cannot be reproduced in a test.
func TestWorkspaceModesAgree(t *testing.T) {
	// A directory must be at least as reachable as the files it holds. The
	// failure this guards is not "the mode is too tight" in the abstract — it is
	// a world-readable file behind a directory that no "other" can traverse,
	// which is a state that cannot be intentional.
	if WorkspaceFileMode&0o004 != 0 {
		if WorkspaceDirMode&0o005 != 0o005 {
			t.Errorf(
				"files are world-readable (%#o) but directories are not world-traversable (%#o): "+
					"every file written into the workspace would sit behind a door no other identity can open",
				WorkspaceFileMode, WorkspaceDirMode)
		}
	}
	// The same, one identity in: group-readable files need group-traversable
	// directories. Not the reported failure, and it is here because the reported
	// failure was the "other" half of a rule nobody had written down.
	if WorkspaceFileMode&0o040 != 0 && WorkspaceDirMode&0o050 != 0o050 {
		t.Errorf("files are group-readable (%#o) but directories are not group-traversable (%#o)",
			WorkspaceFileMode, WorkspaceDirMode)
	}
	// Directories need the owner's execute bit or nothing works at all.
	if WorkspaceDirMode&0o700 != 0o700 {
		t.Errorf("directories must be owner-rwx, got %#o", WorkspaceDirMode)
	}
}

// The regression the first fix did not catch, and the reason it is worth having.
//
// **The original defect needed a second uid to reproduce and this one does not.**
// Passing WorkspaceDirMode to os.MkdirAll fixed only directories that did not
// already exist, and the three that broke the native lambda were checked into the
// client repository months earlier. Whether MkdirAll re-modes an existing
// directory is answerable by the process that created it, so this is a real test
// rather than an invariant standing in for one.
func TestEnsureWorkspaceDirFixesAnExistingDirectory(t *testing.T) {
	base := t.TempDir()

	// The shipped case: the directory is already there, with a mode no other
	// identity can traverse. This is what `build/` looked like in the image.
	existing := filepath.Join(base, "build")
	if err := os.MkdirAll(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorkspaceDir(existing); err != nil {
		t.Fatalf("EnsureWorkspaceDir: %v", err)
	}
	if got := modeOf(t, existing); got != WorkspaceDirMode {
		t.Errorf("existing directory: got %#o, want %#o — os.MkdirAll leaves an existing "+
			"directory's mode alone, which is why the mode has to be asserted", got, WorkspaceDirMode)
	}

	// And nested, because the compiler creates build/jet_rules/<pkg> a level at a
	// time and two of the three that shipped wrong were nested.
	nested := filepath.Join(existing, "jet_rules", "clinical_intel")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorkspaceDir(nested); err != nil {
		t.Fatalf("EnsureWorkspaceDir nested: %v", err)
	}
	if got := modeOf(t, nested); got != WorkspaceDirMode {
		t.Errorf("nested directory: got %#o, want %#o", got, WorkspaceDirMode)
	}

	// The ordinary case still works.
	fresh := filepath.Join(base, "fresh", "deeper")
	if err := EnsureWorkspaceDir(fresh); err != nil {
		t.Fatalf("EnsureWorkspaceDir fresh: %v", err)
	}
	if got := modeOf(t, fresh); got != WorkspaceDirMode {
		t.Errorf("fresh directory: got %#o, want %#o", got, WorkspaceDirMode)
	}
}

// The proof that plain MkdirAll is not enough, asserted rather than described so
// that a future reader does not have to take the doc comment's word for it.
func TestMkdirAllDoesNotRemodeAnExistingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "existing")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, WorkspaceDirMode); err != nil {
		t.Fatal(err)
	}
	if got := modeOf(t, dir); got != 0o700 {
		t.Skipf("os.MkdirAll now re-modes an existing directory (got %#o); "+
			"EnsureWorkspaceDir is then belt and braces rather than the fix", got)
	}
}

func modeOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Mode().Perm()
}
