package workspace

import (
	"bytes"
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
// WORKSPACE_DB_PATH location of workspace db (sqlite db)
// WORKSPACE_LOOKUPS_DB_PATH location of lookup db (sqlite db)

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

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string) error {

		wh := os.Getenv("WORKSPACES_HOME")
		compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", wh, workspaceName)

		// Compile the workspace locally
		cmd := exec.Command(compilerPath)
		var b2 bytes.Buffer
		cmd.Stdout = &b2
		cmd.Stderr = &b2
		log.Printf("Executing compile_workspace command '%v'", compilerPath)
		err := cmd.Run()
		if err != nil {
			log.Printf("while executing compile_workspace command '%v': %v", compilerPath, err)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			b2.WriteTo(os.Stdout)
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
			log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
			return err
		}
		log.Println("============================")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT BEGIN")
		log.Println("============================")
		b2.WriteTo(os.Stdout)
		log.Println("============================")
		log.Println("COMPILE WORKSPACE CAPTURED OUTPUT END")
		log.Println("============================")

		// Copy the sqlite file to db
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
			// aws integration: Copy the file to awsBucket
			file, err := os.Open(sourcesPath[i])
			if err != nil {
				log.Printf("While opening local output file: %v", err)
				return err
			}
			fo.FileName = fileNames[i]
			fo.Oid = 0
			_,err = fo.WriteObject(dbpool, file)
			file.Close()
			if err != nil {
				return fmt.Errorf("failed to upload file to db: %v", err)
			}
		}

		// insert the new workspace version in jetsapi db
		log.Println("Updating workspace version in database to",version)
		stmt := "INSERT INTO jetsapi.workspace_version (version) VALUES ($1)"
		_, err = dbpool.Exec(context.Background(), stmt, version)
		if err != nil {
			return fmt.Errorf("while inserting workspace version into workspace_version table: %v", err)
		}
	

	return nil
}