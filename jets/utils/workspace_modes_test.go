package utils

import "testing"

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
