package wsfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearStashRejectsInvalidWorkspaceName(t *testing.T) {
	t.Setenv("WORKSPACES_HOME", t.TempDir())

	tests := []string{"", "   ", ".", "..", "nested/name", "../escape"}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			if err := ClearStash(tc); err == nil {
				t.Fatalf("expected error for workspace name %q", tc)
			}
		})
	}
}

func TestClearStashRemovesOnlyWorkspaceStash(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)

	stashRoot := StashDir()
	workspace := "demo"
	otherWorkspace := "other"

	workspacePath := filepath.Join(stashRoot, workspace)
	otherWorkspacePath := filepath.Join(stashRoot, otherWorkspace)
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		t.Fatalf("setup workspace stash: %v", err)
	}
	if err := os.MkdirAll(otherWorkspacePath, 0755); err != nil {
		t.Fatalf("setup other workspace stash: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspacePath, "keep.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("setup workspace file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherWorkspacePath, "other.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("setup other workspace file: %v", err)
	}

	if err := ClearStash(workspace); err != nil {
		t.Fatalf("clear stash failed: %v", err)
	}

	if _, err := os.Stat(workspacePath); !os.IsNotExist(err) {
		t.Fatalf("expected workspace stash to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(otherWorkspacePath); err != nil {
		t.Fatalf("expected other workspace stash to remain, stat err=%v", err)
	}
}

// StashFiles keeps a stash it finds, which is right for a container and wrong
// for a workstation, whose WORKSPACES_HOME outlives a start. RefreshStash, what
// a dev-mode start takes, replaces it with the workspace as it stands, and
// neither copies the names a revert must not restore.
func TestRefreshStashRetakesTheStashAndLeavesOutGit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "demo"
	file := filepath.Join(home, ws, "jet_rules", "a.jr")
	for name, content := range map[string]string{
		file:                                         "first start",
		filepath.Join(home, ws, ".git", "HEAD"):      "ref: refs/heads/main",
		filepath.Join(home, ws, ".gitignore"):        "out/",
		filepath.Join(home, ws, "workspace.db"):      "compiled",
		filepath.Join(home, ws, "runbooks", "r.sql"): "select 1",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	stashed := filepath.Join(StashDir(), ws, "jet_rules", "a.jr")
	read := func() string {
		t.Helper()
		b, err := os.ReadFile(stashed)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("second start"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "first start" {
		t.Fatalf("StashFiles replaced an existing stash: %q", got)
	}
	if err := RefreshStash(ws); err != nil {
		t.Fatal(err)
	}
	if got := read(); got != "second start" {
		t.Errorf("RefreshStash kept the old stash: %q", got)
	}
	for _, omitted := range []string{".git", ".gitignore", "workspace.db"} {
		if _, err := os.Lstat(filepath.Join(StashDir(), ws, omitted)); !os.IsNotExist(err) {
			t.Errorf("the stash holds %s (lstat: %v)", omitted, err)
		}
	}
	if _, err := os.Stat(filepath.Join(StashDir(), ws, "runbooks", "r.sql")); err != nil {
		t.Errorf("the stash lost a workspace file: %v", err)
	}
}
