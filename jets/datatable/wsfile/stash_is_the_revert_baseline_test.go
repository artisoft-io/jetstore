package wsfile

import (
	"os"
	"path/filepath"
	"testing"
)

// The stash is what a revert restores *from*, so whether a revert is a revert
// or a silent loss is decided by what the stash holds -- not by the delete.
//
// **These two tests are a pair and neither means much alone.** The first says a
// revert works when the stash was taken from the pristine tree. The second says
// it stops working, without failing, when the stash is re-taken after the tree
// has been edited. That second case is the one Phase 6 met: `pullWorkspaceAction`
// used to clear the stash and re-take it on every pull, which is right when a
// pull has just put the tree at the remote's content and wrong when there was no
// pull, because the tree then still carries the database overrides. It is now
// skipped when git is off, and this file is the reason that fix is load-bearing
// rather than tidy.
//
// Written 2026-09-08 with the revert buttons re-enabled for a deployment with no
// repository (Q-89). Re-enabling them is only defensible while the first test
// passes, so it is here rather than in a plan.

// stashScenario builds a workspace tree under a temporary WORKSPACES_HOME and
// returns the workspace name and the path of the one file it contains.
func stashScenario(t *testing.T, pristine string) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	t.Setenv("JETS_TEMP_DATA", t.TempDir())
	const ws = "test_ws"
	dir := filepath.Join(home, ws)
	if err := os.MkdirAll(filepath.Join(dir, "process_config"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "process_config", "a_config.json")
	if err := os.WriteFile(file, []byte(pristine), 0644); err != nil {
		t.Fatal(err)
	}
	return ws, file
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

// TestAStashTakenBeforeTheOverridesIsWhatMakesARevertARevert is the invariant a
// no-git deployment relies on. The startup stash is taken from the tree cbooter
// copied out of the image and before SyncWorkspaceFiles applies the database
// overrides, so it holds content no user has edited.
func TestAStashTakenBeforeTheOverridesIsWhatMakesARevertARevert(t *testing.T) {
	ws, file := stashScenario(t, "pristine from the image")

	// Startup: stash first, overrides after.
	if err := StashFiles(ws); err != nil {
		t.Fatalf("StashFiles: %v", err)
	}
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := RestaureWorkspaceFiles(ws); err != nil {
		t.Fatalf("RestaureWorkspaceFiles: %v", err)
	}
	if got := read(t, file); got != "pristine from the image" {
		t.Errorf("revert did not restore the pristine content: got %q", got)
	}
}

// TestReStashingAfterAnEditSilentlyTurnsARevertIntoANoOp characterises the
// failure rather than guarding against it, because nothing in this package can
// prevent a caller clearing the stash at the wrong moment.
//
// **The point is that nothing errors.** Every call below succeeds, the revert
// reports success, and the file keeps the edit -- so a user who asked to discard
// their change is told it worked and finds it still there. That is why the
// remedy had to be at the call site (skip the clear when git is off) rather than
// here.
func TestReStashingAfterAnEditSilentlyTurnsARevertIntoANoOp(t *testing.T) {
	ws, file := stashScenario(t, "pristine from the image")

	if err := StashFiles(ws); err != nil {
		t.Fatalf("StashFiles: %v", err)
	}
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}

	// What pullWorkspaceAction used to do unconditionally. With a pull in front
	// of it this is correct; without one it captures the edit.
	if err := ClearStash(ws); err != nil {
		t.Fatalf("ClearStash: %v", err)
	}
	if err := StashFiles(ws); err != nil {
		t.Fatalf("StashFiles: %v", err)
	}

	if err := RestaureWorkspaceFiles(ws); err != nil {
		t.Fatalf("RestaureWorkspaceFiles: %v", err)
	}
	if got := read(t, file); got != "the user's edit" {
		t.Fatalf("expected the characterised failure -- the stash holding the edit -- but got %q."+
			" If this now restores the pristine content the mechanism has changed and"+
			" pullWorkspaceAction's git-off skip may no longer be needed.", got)
	}
}

// TestStashFilesWillNotOverwriteAnExistingStash is the property that makes the
// clear rather than the re-stash the dangerous half of that pair, and it is
// asserted here because the remedy was chosen on it.
func TestStashFilesWillNotOverwriteAnExistingStash(t *testing.T) {
	ws, file := stashScenario(t, "pristine from the image")

	if err := StashFiles(ws); err != nil {
		t.Fatalf("StashFiles: %v", err)
	}
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}
	// No ClearStash this time: the second call must decline.
	if err := StashFiles(ws); err != nil {
		t.Fatalf("StashFiles (second): %v", err)
	}

	if err := RestaureWorkspaceFiles(ws); err != nil {
		t.Fatalf("RestaureWorkspaceFiles: %v", err)
	}
	if got := read(t, file); got != "pristine from the image" {
		t.Errorf("a second StashFiles overwrote the stash; got %q", got)
	}
}
