package wsfile

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/utils"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file contains function to get and save file content & execute command in local workspace

// Run command in workspace
func RunCommand(buf *strings.Builder, command string, args *[]string, workspaceName string) error {
	var cmd *exec.Cmd
	var err error
	if args != nil {
		// Sanitize the arguments to prevent injection of options/flags
		*args = utils.SanitizeArgs(*args)
		cmd = exec.Command(command, (*args)...)
	} else {
		cmd = exec.Command(command)
	}
	if workspaceName != "" {
		workspaceName, err = utils.ValidateWorkspaceName(workspaceName)
		if err != nil {
			return err
		}
		path := filepath.Join(os.Getenv("WORKSPACES_HOME"), workspaceName)
		fmt.Fprintf(buf, "Executing command %s in %s\n", command, path)
		cmd.Dir = path
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("WORKSPACE=%s", workspaceName),
		)
	} else {
		fmt.Fprintf(buf, "Executing command %s (not workspace specific or path specified)\n", command)
	}
	cmd.Stdout = buf
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		msg := fmt.Sprintf("while executing command '%v': %v\n", command, err)
		log.Print(msg)
		buf.WriteString(msg)
	}
	buf.WriteString("Done executing command\n")
	return err
}

// GetWorkspaceFileContent --------------------------------------------------------------------------
// Function to get the workspace file content based on relative file name
// Read the file from the workspace on file system since it's already in sync with database
func GetContent(workspaceName, fileName string) (string, error) {

	// Read file from local workspace, confining the path to the workspace dir (CWE-73)
	_, path, err := resolveWorkspacePath(workspaceName, fileName)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read local workspace file %s: %v", fileName, err)
	}
	return string(content), nil
}

// RootContentType is the content type of an override of a file at the
// workspace's root, workspace_control.json or compile_workspace.sh, which has
// no first path segment to take one from (overrideContentType).
const RootContentType = "root"

// overrideContentType is the content_type a workspace_changes row records for
// fileName: its first path segment, the workspace directory it is in, or
// RootContentType for a file at the root. Only the compiled files' content
// types select anything (SyncWorkspaceFiles, jets/workspace); every other
// override is applied at a start whatever its content type.
func overrideContentType(fileName string) string {
	if p := strings.Index(fileName, "/"); p > 0 {
		return fileName[0:p]
	}
	return RootContentType
}

// SaveContent saves a workspace file from the IDE: to the database, as the
// override jetsapi.workspace_changes keeps and every start applies, then to
// the workspace's local copy. userEmail is who made the change, recorded as the
// workspace_changes row's user_email; empty, the change is recorded as made by
// "system".
//
// **The database first, since 2026-11-14.** The disk was written first, and a
// save the database then refused stood on disk until the next start and was
// gone after it, with an error the IDE showed as a failed save of a file that
// had in fact changed. A root file was refused that way every time: the
// content type was the path's first segment and a root file has none, so
// workspace_control.json and compile_workspace.sh could not be saved
// (overrideContentType). Now a refused database write leaves the disk alone,
// and a failed disk write after it leaves an override the next start, or the
// sync a compile runs first, applies.
func SaveContent(dbpool *pgxpool.Pool, workspaceName, fileName, fileContent, userEmail string) error {

	// Confine the path to the workspace dir before writing anything (CWE-73)
	workspaceName, path, err := resolveWorkspacePath(workspaceName, fileName)
	if err != nil {
		return err
	}
	data := []byte(fileContent)

	// Write file and metadata to database
	fo := dbutils.FileDbObject{
		WorkspaceName: workspaceName,
		FileName:      fileName,
		ContentType:   overrideContentType(fileName),
		UserEmail:     changeAuthor(userEmail),
	}
	n, err := fo.WriteObject(dbpool, data)
	if err != nil {
		return fmt.Errorf("failed to save local workspace file %s in database: %v", fileName, err)
	}
	log.Println("uploaded", fo.FileName, "size", n, "bytes to database")

	// Write file to local workspace
	if err = os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write local workspace file %s: %v", fileName, err)
	}
	return nil
}

// changeAuthor is the user_email a workspace change is recorded with: the
// signed-in user's, or "system" for a change no user made.
func changeAuthor(userEmail string) string {
	if strings.TrimSpace(userEmail) == "" {
		return "system"
	}
	return userEmail
}
