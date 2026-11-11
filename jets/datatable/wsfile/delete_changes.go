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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file contains function to delete workspace changes

// ErrNoStashCopy is returned, wrapped, when a change cannot be reverted because
// the workspace stash holds no copy of the file to restore.
var ErrNoStashCopy = errors.New("the workspace stash holds no copy of the file")

// CompiledAssets are the files a compile writes to the workspace's top level and
// saves as workspace changes (workspace.UploadWorkspaceAssets). A revert of
// every change keeps them, in the database and in the workspace: until
// 2026-11-11 it deleted their rows and copied the start-up versions of the two
// .tgz files over the compiled ones, so the installation lost its last compile.
var CompiledAssets = []string{"lookup.db", "workspace.db", "workspace.tgz", "reports.tgz"}

// stashOmits are the top-level names StashFiles removes from the stash on
// purpose: a file under one of them is absent from the stash without having
// been added since it was taken, so a revert never deletes it.
var stashOmits = []string{".git", ".github", ".gitignore", "lookup.db", "workspace.db"}

// addedSinceStash says whether fileName was added to the workspace after the
// stash was taken: the workspace has a stash, the stash holds no copy of the
// file, and the name is not one the stash leaves out on purpose. A workspace
// with no stash at all (a workstation, where dev mode takes none) has every file
// absent from it, so it answers false there and the revert is refused as before.
func addedSinceStash(workspaceName, fileName string) (bool, error) {
	stashRoot := filepath.Join(StashDir(), workspaceName)
	if info, err := os.Stat(stashRoot); err != nil || !info.IsDir() {
		return false, nil
	}
	first := strings.SplitN(filepath.ToSlash(filepath.Clean(fileName)), "/", 2)[0]
	for _, omitted := range stashOmits {
		if first == omitted {
			return false, nil
		}
	}
	source, err := confinePath(stashRoot, fileName)
	if err != nil {
		return false, err
	}
	if _, err = os.Lstat(source); errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

// DeleteFileChange reverts one workspace file change: it restores the file from
// the workspace stash, then deletes its jetsapi.workspace_changes row. A .db
// file is not restored and a .tgz file is removed; for those the row is simply
// deleted.
//
// **A file added since the stash was taken is deleted** (addedSinceStash): its
// original state is not to exist. The screen asks before every revert and says
// so. Until 2026-11-11 such a revert was refused and the file kept.
//
// **The row is deleted only once the file has been restored or deleted.** The
// row holds the user's edit, so deleting it while the file keeps the edit loses
// the edit at the next start-up and leaves a working file that no longer
// matches any recorded change. When the workspace has no stash, or the copy or
// the deletion fails, the error is returned and both the row and the file are
// left as they were.
func DeleteFileChange(dbpool *pgxpool.Pool, workspaceName, fileName string) error {
	// Validate workspace name and confine fileName to the workspace directory (CWE-73)
	workspaceName, destination, err := resolveWorkspacePath(workspaceName, fileName)
	if err != nil {
		return err
	}
	log.Println("DeleteWorkspaceChanges: Deleting workspaceName", workspaceName, "file name", fileName)
	restaureFromStash := !strings.HasSuffix(fileName, ".db") && !strings.HasSuffix(fileName, ".tgz")
	if restaureFromStash {
		added, err := addedSinceStash(workspaceName, fileName)
		if err != nil {
			return fmt.Errorf("cannot revert %s, the change is kept: %w", fileName, err)
		}
		if added {
			if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("Not reverting %s: while deleting the added file: %v", fileName, err)
				return fmt.Errorf("cannot revert %s, the change is kept: while deleting the file: %w", fileName, err)
			}
			log.Printf("Reverted %s: added since the stash was taken, deleted", fileName)
		} else if err := restoreFileFromStash(workspaceName, fileName, destination); err != nil {
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
	stmt := `DELETE FROM jetsapi.workspace_changes WHERE workspace_name = $1`
	args := []any{workspaceName}
	if keepWorkspaceAndLookupDb {
		stmt += ` AND file_name <> ALL($2)`
		args = append(args, CompiledAssets)
	}
	// Restore first, and keep the rows when the restore fails: deleting them
	// while the files keep the edits would lose the edits (see DeleteFileChange).
	// A changed file added since the stash was taken is deleted first, as a
	// single revert deletes it: copying the stash back does not remove it.
	if restaureFromStash {
		if err = deleteAddedFiles(dbpool, workspaceName); err != nil {
			log.Println("Not reverting the workspace changes:", err)
			return fmt.Errorf("cannot revert the changes of workspace %s, the changes are kept: %w", workspaceName, err)
		}
		log.Println("Restauring all workspace files from stash")
		if err = RestaureWorkspaceFiles(workspaceName); err != nil {
			log.Println("Not reverting the workspace changes: while restauring all workspace files:", err)
			return fmt.Errorf("cannot revert the changes of workspace %s from its stash, the changes are kept: %w",
				workspaceName, err)
		}
	}
	log.Println("DELETE stmt:", stmt)
	_, err = dbpool.Exec(context.Background(), stmt, args...)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		return fmt.Errorf("while deleting row in workspace_changes table: %v", err)
	}
	return nil
}

// deleteAddedFiles deletes each file of workspaceName that has a change row and
// was added since the stash was taken (addedSinceStash). .db and .tgz files,
// the compiled assets among them, are never deleted here.
func deleteAddedFiles(dbpool *pgxpool.Pool, workspaceName string) error {
	rows, err := dbpool.Query(context.Background(),
		"SELECT file_name FROM jetsapi.workspace_changes WHERE workspace_name = $1", workspaceName)
	if err != nil {
		return fmt.Errorf("while listing the workspace changes: %w", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("while listing the workspace changes: %w", err)
	}
	for _, fileName := range names {
		if strings.HasSuffix(fileName, ".db") || strings.HasSuffix(fileName, ".tgz") {
			continue
		}
		added, err := addedSinceStash(workspaceName, fileName)
		if err != nil {
			return err
		}
		if !added {
			continue
		}
		_, destination, err := resolveWorkspacePath(workspaceName, fileName)
		if err != nil {
			return err
		}
		if err := os.Remove(destination); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("while deleting %s, added since the stash was taken: %w", fileName, err)
		}
		log.Printf("Reverted %s: added since the stash was taken, deleted", fileName)
	}
	return nil
}
