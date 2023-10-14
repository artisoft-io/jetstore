package workspace

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains functions to compile and sync the workspace
// between jetstore database and the local container
// WORKSPACE Workspace default currently in use
// WORKSPACES_HOME Home dir of workspaces

// Function to pull override workspace files from databse to the
// container workspace (local copy).
// Need this when:
//	- starting a task requiring local workspace (e.g. run_report to get latest report definition)
//	- starting apiserver to get latest override files (e.g. lookup csv files) to compile workspace
//	- starting rule server to get the latest lookup.db and workspace.db
func SyncWorkspaceFiles(dbpool *pgxpool.Pool, workspaceName, status, contentType string, skipSqliteFiles bool) error {
	wh := os.Getenv("WORKSPACES_HOME")
	// sync workspace files from db to locally
	// Get all file_name that are modified
	if len(contentType) > 0 {
		log.Printf("Start synching overriten workspace file with status '%s' and content_type '%s' from database", status, contentType)
	} else {
		log.Printf("Start synching overriten workspace file with status '%s' from database", status)
	}
	fileObjects, err := dbutils.QueryFileObject(dbpool, workspaceName, status, contentType)
	if err != nil {
		return err
	}
	for _,fo := range fileObjects {
		// When in skipSqliteFiles == true, do not override lookup.db and workspace.db
		if !skipSqliteFiles || !strings.HasSuffix(fo.FileName, ".db") {
			fileHd, err := os.Create(fmt.Sprintf("%s/%s/%s", wh, workspaceName, fo.FileName))
			if err != nil {
				return fmt.Errorf("failed to open local workspace file %s for write: %v", fo.FileName, err)
			}
			n, err := fo.ReadObject(dbpool, fileHd)
			if err != nil {
				return fmt.Errorf("failed to read file object %s from database for write: %v", fo.FileName, err)
			}
			log.Println("Updated file", fo.FileName,"size",n)
		} else {
			log.Println("Skipping file", fo.FileName)
		}
	}
	log.Println("Done synching overriten workspace file from database")
	return nil
}

func UpdateWorkspaceVersionDb(dbpool *pgxpool.Pool, workspaceName, version string) error {

	if version == "" {
		log.Println("Error: attempting to write empty version to table workspace_version, skipping")
		return nil
	}
	// insert the new workspace version in jetsapi db
	log.Println("Updating workspace version in database to",version)
	stmt := "INSERT INTO jetsapi.workspace_version (version) VALUES ($1)"
	_, err := dbpool.Exec(context.Background(), stmt, version)
	if err != nil {
		return fmt.Errorf("while inserting workspace version into workspace_version table: %v", err)
	}

	return nil
}

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string) (string, error) {

	wh := os.Getenv("WORKSPACES_HOME")
	compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", wh, workspaceName)

	// Compile the workspace locally
	cmd := exec.Command(compilerPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("WORKSPACE=%s", workspaceName),
	)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	log.Printf("Executing compile_workspace command '%v'", compilerPath)
	buf.WriteString(fmt.Sprintf("Compiling workspace %s at version %s\n",workspaceName, version))
	err := cmd.Run()
	cmdLog := buf.String()
	if err != nil {
		log.Printf("while executing compile_workspace command '%v': %v", compilerPath, err)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println(cmdLog)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return cmdLog, fmt.Errorf("while executing compile_workspace command '%v': %v", compilerPath, err)
	}
	log.Println("============================")
	log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
	log.Println("============================")
	log.Println(cmdLog)
	log.Println("============================")
	log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
	log.Println("============================")

	// Copy the sqlite file to db
	buf.WriteString("\nCopy the sqlite file to db\n")
	sourcesPath := []string{
		fmt.Sprintf("%s/%s/lookup.db", wh, workspaceName),
		fmt.Sprintf("%s/%s/workspace.db", wh, workspaceName),
	}
	fileNames := []string{ "lookup.db", "workspace.db" }
	fo := dbutils.FileDbObject{
		WorkspaceName: workspaceName,
		ContentType: "sqlite",
		Status: dbutils.FO_Open,
		UserEmail: "system",
	}
	for i := range sourcesPath {
		// Copy the file to db as large objects
		file, err := os.Open(sourcesPath[i])
		if err != nil {
			buf.WriteString("While opening local output file:")
			buf.WriteString(err.Error())
			buf.WriteString("\n")
			log.Printf("While opening local output file: %v", err)
			return buf.String(), err
		}
		fo.FileName = fileNames[i]
		fo.Oid = 0
		_,err = fo.WriteObject(dbpool, file)
		file.Close()
		if err != nil {
			buf.WriteString("Failed to upload file to db:")
			buf.WriteString(err.Error())
			buf.WriteString("\n")
			return buf.String(), fmt.Errorf("failed to upload file to db: %v", err)
		}
	}
	err = UpdateWorkspaceVersionDb(dbpool, workspaceName, version)
	if err != nil {
		buf.WriteString("Failed to update worspace version to db:")
		buf.WriteString(err.Error())
		buf.WriteString("\n")
	}

	return buf.String(), err
}