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
