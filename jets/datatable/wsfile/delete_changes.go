package wsfile

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains function to delete workspace changes

// Function to delete workspace file changes based on rows in workspace_changes
// Delete the workspace_changes row and the associated large object
func DeleteFileChange(dbpool *pgxpool.Pool, workspaceChangesKey, workspaceName, fileName, oid string) error {
	fmt.Println("DeleteWorkspaceChanges: Deleting key", workspaceChangesKey, "file name", fileName)
	stmt := fmt.Sprintf("SELECT lo_unlink(%s); DELETE FROM jetsapi.workspace_changes WHERE key = %s",	oid, workspaceChangesKey)
	fmt.Println("DELETE stmt:", stmt)
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		return fmt.Errorf("while deleting row in workspace_changes table: %v", err)
	}
	restaureFromStash := true
	if strings.HasSuffix(fileName, ".db") || strings.HasSuffix(fileName, ".tgz") {
		restaureFromStash = false
		if strings.HasSuffix(fileName, ".tgz") {
			os.Remove(fileName)
		}
	}

	if !restaureFromStash {
		return nil
	}
	// restauring file from stash (if exists, do not report error if fails)
	stashPath := StashDir()
	source := fmt.Sprintf("%s/%s/%s", stashPath, workspaceName, fileName)
	destination := fmt.Sprintf("%s/%s/%s", os.Getenv("WORKSPACES_HOME"), workspaceName, fileName)
	log.Printf("Restauring file %s to %s", source, destination)
	if n, err2 := CopyFiles(source, destination); err2 != nil {
		log.Println("while restauring file:", err2)
	} else {
		log.Println("copied", n, "bytes")
	}
	return nil
}

// Delete all files overrides from database for workspaceName.
// if restaureFromStash is true then, replace the local file with the stash content
// if keepWorkspaceAndLookupDb is true, then don't remove files 'workspace.db', 'lookup.db', 'workspace.tgz', 'reports.tgz' from the overrides
func DeleteAllFileChanges(dbpool *pgxpool.Pool, workspaceName string, restaureFromStash, keepWorkspaceAndLookupDb bool) error {
	fmt.Println("DeleteAllWorkspaceChanges: workspace_name", workspaceName)
	var stmt string
	switch {
	case keepWorkspaceAndLookupDb:
		stmt = fmt.Sprintf(
			`SELECT lo_unlink(oid) 
			 FROM jetsapi.workspace_changes 
			 WHERE workspace_name = '%s' 
			   AND file_name NOT IN ('workspace.db', 'lookup.db', 'workspace.tgz', 'reports.tgz'); 
			 DELETE FROM jetsapi.workspace_changes 
			 WHERE workspace_name = '%s'
			   AND file_name NOT IN ('workspace.db', 'lookup.db', 'workspace.tgz', 'reports.tgz');`,
			workspaceName, workspaceName)
	default:	
		stmt = fmt.Sprintf(
			`SELECT lo_unlink(oid) 
			 FROM jetsapi.workspace_changes 
			 WHERE workspace_name = '%s'; 
			 DELETE FROM jetsapi.workspace_changes 
			 WHERE workspace_name = '%s'`,
			workspaceName, workspaceName)
	}
	fmt.Println("DELETE stmt:", stmt)
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		log.Printf("While deleting row in workspace_changes table: %v", err)
		return fmt.Errorf("while deleting row in workspace_changes table: %v", err)
	}

	if !restaureFromStash {
		return nil
	}
	// restauring file from stash (if exists, do not report error if fails)
	stashPath := StashDir()
	source := fmt.Sprintf("%s/%s", stashPath, workspaceName)
	log.Printf("Restauring all workspace files from %s", source)
	if err = RestaureFiles(source, os.Getenv("WORKSPACES_HOME")); err != nil {
		log.Println("while restauring all workspace files:", err)
	}
	return nil
}