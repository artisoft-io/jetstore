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

func TestRestaureFilesValidatesInput(t *testing.T) {
	dstDir := t.TempDir()

	if err := RestaureFiles("", dstDir); err == nil {
		t.Fatal("expected error for empty source directory")
	}
	if err := RestaureFiles(t.TempDir(), ""); err == nil {
		t.Fatal("expected error for empty destination directory")
	}
}

func TestRestaureFilesCopiesIntoTargetDirectory(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "sourcews")
	dstDir := filepath.Join(root, "restore")

	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "data.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("setup source file: %v", err)
	}

	if err := RestaureFiles(srcDir, dstDir); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	restoredFile := filepath.Join(dstDir, filepath.Base(srcDir), "nested", "data.txt")
	content, err := os.ReadFile(restoredFile)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected restored content: %q", string(content))
	}
}

func TestRestaureFilesPreservesSymlink(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "sourcews")
	dstDir := filepath.Join(root, "restore")

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("setup source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "target.txt"), []byte("target"), 0644); err != nil {
		t.Fatalf("setup target file: %v", err)
	}
	if err := os.Symlink("target.txt", filepath.Join(srcDir, "link.txt")); err != nil {
		t.Fatalf("setup symlink: %v", err)
	}

	if err := RestaureFiles(srcDir, dstDir); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	restoredLink := filepath.Join(dstDir, filepath.Base(srcDir), "link.txt")
	info, err := os.Lstat(restoredLink)
	if err != nil {
		t.Fatalf("lstat restored symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected restored path to be symlink, mode=%v", info.Mode())
	}
	linkTarget, err := os.Readlink(restoredLink)
	if err != nil {
		t.Fatalf("readlink restored symlink: %v", err)
	}
	if linkTarget != "target.txt" {
		t.Fatalf("unexpected symlink target: %q", linkTarget)
	}
}
