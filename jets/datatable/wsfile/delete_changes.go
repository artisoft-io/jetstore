package wsfile

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file contains function to delete workspace changes

// ErrNoStashCopy is returned, wrapped, when a change cannot be reverted because
// the workspace stash holds no copy of the file to restore.
var ErrNoStashCopy = errors.New("the workspace stash holds no copy of the file")

// DeleteFileChange reverts one workspace file change: it restores the file from
// the workspace stash, then deletes its jetsapi.workspace_changes row. A .db
// file is not restored and a .tgz file is removed; for those the row is simply
// deleted.
//
// **The row is deleted only once the file has been restored.** The row holds
// the user's edit, so deleting it while the file keeps the edit loses the edit
// at the next start-up and leaves a working file that no longer matches any
// recorded change. When the stash has no copy of the file (one added after the
// stash was taken, or a workspace never stashed), or the copy fails, the error
// is returned and both the row and the file are left as they were.
func DeleteFileChange(dbpool *pgxpool.Pool, workspaceName, fileName string) error {
	// Validate workspace name and confine fileName to the workspace directory (CWE-73)
	workspaceName, destination, err := resolveWorkspacePath(workspaceName, fileName)
	if err != nil {
		return err
	}
	log.Println("DeleteWorkspaceChanges: Deleting workspaceName", workspaceName, "file name", fileName)
	restaureFromStash := !strings.HasSuffix(fileName, ".db") && !strings.HasSuffix(fileName, ".tgz")
	if restaureFromStash {
		if err := restoreFileFromStash(workspaceName, fileName, destination); err != nil {
			log.Printf("Not reverting %s: %v", fileName, err)
			return fmt.Errorf("cannot revert %s, the change is kept: %w", fileName, err)
		}
	}
	stmt := "DELETE FROM jetsapi.workspace_changes WHERE workspace_name = $1 and file_name = $2"
	_, err = dbpool.Exec(context.Background(), stmt, workspaceName, fileName)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		return fmt.Errorf("while deleting row in workspace_changes table: %v", err)
	}
	if strings.HasSuffix(fileName, ".tgz") {
		os.Remove(destination)
	}
	return nil
}

// restoreFileFromStash copies the stashed copy of fileName over destination.
// It writes a temporary file beside destination and renames it into place, so a
// copy that fails part way leaves destination as it was.
func restoreFileFromStash(workspaceName, fileName, destination string) error {
	source, err := confinePath(filepath.Join(StashDir(), workspaceName), fileName)
	if err != nil {
		return err
	}
	if info, err := os.Stat(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w (it may have been added after the stash was taken)", ErrNoStashCopy)
		}
		return fmt.Errorf("while reading the stash copy: %w", err)
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("the stash copy %s is not a regular file", fileName)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("while restoring from the stash: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".revert-*")
	if err != nil {
		return fmt.Errorf("while restoring from the stash: %w", err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName) // a no-op once renamed
	log.Printf("Restauring file %s to %s", source, destination)
	n, err := CopyFiles(source, tmpName)
	if err != nil {
		return fmt.Errorf("while restoring from the stash: %w", err)
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(destination); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("while restoring from the stash: %w", err)
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("while restoring from the stash: %w", err)
	}
	log.Println("copied", n, "bytes")
	return nil
}

// Delete all files overrides from database for workspaceName.
// if restaureFromStash is true then, replace the local file with the stash content
// if keepWorkspaceAndLookupDb is true, then don't remove files 'workspace.db', 'lookup.db', 'workspace.tgz', 'reports.tgz' from the overrides
func DeleteAllFileChanges(dbpool *pgxpool.Pool, workspaceName string, restaureFromStash, keepWorkspaceAndLookupDb bool) error {
	// Validate workspace name before using it to build filesystem paths (CWE-73)
	workspaceName, err := utils.ValidateWorkspaceName(workspaceName)
	if err != nil {
		return err
	}
	log.Println("DeleteAllWorkspaceChanges: workspace_name", workspaceName)
	var stmt string
	switch {
	case keepWorkspaceAndLookupDb:
		stmt = `DELETE FROM jetsapi.workspace_changes 
			 WHERE workspace_name = $1
			   AND file_name NOT IN ('workspace.db', 'lookup.db', 'workspace.tgz', 'reports.tgz');`
	default:
		stmt = `DELETE FROM jetsapi.workspace_changes 
			 WHERE workspace_name = $1`
	}
	// Restore first, and keep the rows when the restore fails: deleting them
	// while the files keep the edits would lose the edits (see DeleteFileChange).
	if restaureFromStash {
		log.Println("Restauring all workspace files from stash")
		if err = RestaureWorkspaceFiles(workspaceName); err != nil {
			log.Println("Not reverting the workspace changes: while restauring all workspace files:", err)
			return fmt.Errorf("cannot revert the changes of workspace %s from its stash, the changes are kept: %w",
				workspaceName, err)
		}
	}
	log.Println("DELETE stmt:", stmt)
	_, err = dbpool.Exec(context.Background(), stmt, workspaceName)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		return fmt.Errorf("while deleting row in workspace_changes table: %v", err)
	}
	return nil
}
