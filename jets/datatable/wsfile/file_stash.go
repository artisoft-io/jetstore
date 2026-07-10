package wsfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// This file contains functions to stash workspace in the jetstore ui container

// StashFiles --------------------------------------------------------------------------
// Function to copy all workspace files to a stash location
// The stash is used when deleting workspace changes to restore the file to original content
func StashFiles(workspaceName string) error {
	workspaceName, err := validateWorkspaceName(workspaceName)
	if err != nil {
		return err
	}

	workspacesHome := os.Getenv("WORKSPACES_HOME")
	if workspacesHome == "" {
		return fmt.Errorf("WORKSPACES_HOME is not set")
	}

	workspacePath := filepath.Join(workspacesHome, workspaceName)
	stashPath := StashDir()
	log.Printf("Stashing workspace files from %s to %s", workspacePath, stashPath)

	// make sure the stash directory exists
	if err := os.MkdirAll(stashPath, 0755); err != nil {
		return fmt.Errorf("while creating workspace stash directory %s: %w", stashPath, err)
	}

	// copy all files if targetDir does not exists
	stashWorkspacePath := filepath.Join(stashPath, workspaceName)
	if _, err2 := os.Stat(stashWorkspacePath); errors.Is(err2, os.ErrNotExist) {
		log.Println("Stashing workspace files")
		if err := copyDirNoDereference(workspacePath, stashWorkspacePath); err != nil {
			log.Printf("while stashing workspace files: %v", err)
			return err
		}

		// Removing files that we don't want to be restaured
		_ = os.RemoveAll(filepath.Join(stashWorkspacePath, ".git"))
		_ = os.RemoveAll(filepath.Join(stashWorkspacePath, ".github"))
		_ = os.RemoveAll(filepath.Join(stashWorkspacePath, ".gitignore"))
		_ = os.RemoveAll(filepath.Join(stashWorkspacePath, "lookup.db"))
		_ = os.RemoveAll(filepath.Join(stashWorkspacePath, "workspace.db"))
	} else if err2 == nil {
		log.Println("Workspace files already stashed, not overriting them")
	} else {
		return fmt.Errorf("while checking stash workspace path %s: %w", stashWorkspacePath, err2)
	}

	return nil
}

func validateWorkspaceName(workspaceName string) (string, error) {
	workspaceName = strings.TrimSpace(workspaceName)
	if workspaceName == "" {
		return "", fmt.Errorf("workspace name is required")
	}
	if workspaceName != filepath.Base(workspaceName) || workspaceName == "." || workspaceName == ".." {
		return "", fmt.Errorf("invalid workspace name: %q", workspaceName)
	}
	return workspaceName, nil
}

// confinePath joins fileName onto baseDir and verifies the cleaned result stays
// within baseDir, mitigating external control of file name or path (CWE-73).
// fileName may legitimately contain subdirectories (e.g. "process_config/foo.sql"),
// so the path is confined to baseDir rather than reduced with filepath.Base.
func confinePath(baseDir, fileName string) (string, error) {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("while resolving base dir %q: %w", baseDir, err)
	}
	joined := filepath.Join(absBase, fileName)
	if joined != absBase && !strings.HasPrefix(joined, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid file path %q: escapes directory %q", fileName, baseDir)
	}
	return joined, nil
}

// ResolveWorkspacePath validates workspaceName and confines fileName within the
// workspace directory under WORKSPACES_HOME, mitigating external control of file
// name or path (CWE-73). It returns the safe absolute file path.
func ResolveWorkspacePath(workspaceName, fileName string) (string, error) {
	_, path, err := resolveWorkspacePath(workspaceName, fileName)
	return path, err
}

// resolveWorkspacePath validates workspaceName and confines fileName within the
// workspace directory under WORKSPACES_HOME, mitigating external control of file
// name or path (CWE-73). It returns the validated workspace name and the safe
// absolute file path.
func resolveWorkspacePath(workspaceName, fileName string) (string, string, error) {
	workspaceName, err := validateWorkspaceName(workspaceName)
	if err != nil {
		return "", "", err
	}
	workspacesHome := strings.TrimSpace(os.Getenv("WORKSPACES_HOME"))
	if workspacesHome == "" {
		return "", "", fmt.Errorf("WORKSPACES_HOME is not set")
	}
	path, err := confinePath(filepath.Join(workspacesHome, workspaceName), fileName)
	if err != nil {
		return "", "", err
	}
	return workspaceName, path, nil
}

func copyDirNoDereference(srcDir, dstDir string) error {
	srcInfo, err := os.Lstat(srcDir)
	if err != nil {
		return fmt.Errorf("while stating source dir %s: %w", srcDir, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source path %s is not a directory", srcDir)
	}

	if err := os.MkdirAll(dstDir, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("while creating destination dir %s: %w", dstDir, err)
	}

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == srcDir {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()

		switch {
		case mode&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		case d.IsDir():
			return os.MkdirAll(targetPath, mode.Perm())
		case mode.IsRegular():
			if _, err := CopyFiles(path, targetPath); err != nil {
				return err
			}
			return os.Chmod(targetPath, mode.Perm())
		default:
			// Skip special files (devices, sockets, etc.).
			return nil
		}
	})
}

// Function to remove the stash
func ClearStash(workspaceName string) error {
	workspaceName, err := validateWorkspaceName(workspaceName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(os.Getenv("WORKSPACES_HOME")) == "" {
		return fmt.Errorf("WORKSPACES_HOME is not set")
	}

	log.Printf("Clearing workspace '%s' stash", workspaceName)
	return os.RemoveAll(filepath.Join(StashDir(), workspaceName))
}

func StashDir() string {
	return fmt.Sprintf("%s/ws:stash", os.Getenv("WORKSPACES_HOME"))
}

// Function to restore file from stash, it copy src file to dst
func CopyFiles(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

// Restaure (copy dir recursively) srcDir to dstDir
func RestaureFiles(srcDir, dstDir string) error {
	srcDir = filepath.Clean(strings.TrimSpace(srcDir))
	dstDir = filepath.Clean(strings.TrimSpace(dstDir))
	if srcDir == "" || srcDir == "." {
		return fmt.Errorf("source directory is required")
	}
	if dstDir == "" || dstDir == "." {
		return fmt.Errorf("destination directory is required")
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("while creating restore destination %s: %w", dstDir, err)
	}

	targetPath := filepath.Join(dstDir, filepath.Base(srcDir))
	err := copyDirNoDereference(srcDir, targetPath)
	if err != nil {
		log.Printf("while executing restaure from stash all the workspace files: %v", err)
	} else {
		log.Println("restaure all workspace files from stash completed")
	}
	return err
}
