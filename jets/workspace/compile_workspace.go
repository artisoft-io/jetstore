package workspace

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains functions to compile and sync the workspace
// between s3 and the local container
// JETS_BUCKET
// JETS_REGION
// WORKSPACE_DB_PATH location of workspace db (sqlite db)
// WORKSPACE_LOOKUPS_DB_PATH location of lookup db (sqlite db)

// DELETE THIS
// // Function to copy lookup.db and workspace.db from s3 locally if present
// // Used by rule execution and apiserver upon startup
// func SyncWorkspaceLocalDB() error {
// 	bucket := os.Getenv("JETS_BUCKET")
// 	region := os.Getenv("JETS_REGION")
// 	wh := os.Getenv("WORKSPACES_HOME")
// 	wk := os.Getenv("WORKSPACE")

// 	// Copy the sqlite file from s3
// 	localPath := []string{
// 		fmt.Sprintf("%s/%s/lookup.db", wh, wk),
// 		fmt.Sprintf("%s/%s/workspace.db", wh, wk),
// 	}
// 	sourcesKey := []string{
// 		fmt.Sprintf("jetstore/workspaces/%s/lookup.db", wk),
// 		fmt.Sprintf("jetstore/workspaces/%s/workspace.db", wk),
// 	}
// 	for i := range localPath {
// 		// aws integration: Copy the file to awsBucket
// 		file, err := os.Create(localPath[i])
// 		if err != nil {
// 			log.Printf("While opening local output file: %v", err)
// 			return err
// 		}
// 		nsz, err := awsi.DownloadFromS3(bucket, region, sourcesKey[i], file)
// 		file.Close()
// 		if err != nil {
// 			return fmt.Errorf("failed to download file from s3: %v", err)
// 		}
// 		fmt.Println("downloaded", nsz,"bytes from s3")
// 	}
// 	return nil
// }

// Function to pull override workspace files from s3 to the
// container workspace (local copy).
// Need this when:
//	- starting a task requiring local workspace (e.g. run_report to get latest report definition)
//	- starting apiserver to get latest override files (e.g. lookup csv files) to compile workspace
//	- starting rule server to get the latest lookup.db and workspace.db
func SyncWorkspaceFiles() error {
	bucket := os.Getenv("JETS_BUCKET")
	region := os.Getenv("JETS_REGION")
	wh := os.Getenv("WORKSPACES_HOME")
	wk := os.Getenv("WORKSPACE")
	// sync workspace files from s3 to locally
	//* TODO more prefix: sync workspace files from s3 to locally to compile workspace
	prefixes := []string{
		fmt.Sprintf("jetstore/workspaces/%s/lookups", wk),
		fmt.Sprintf("jetstore/workspaces/%s/process_config", wk),
		fmt.Sprintf("jetstore/workspaces/%s/reports", wk),
		fmt.Sprintf("jetstore/workspaces/%s/lookup.db", wk),
		fmt.Sprintf("jetstore/workspaces/%s/workspace.db", wk),
	}
	log.Println("Synching overriten workspace file from s3")
	for i := range prefixes {
		keys, err := awsi.ListS3Objects(&prefixes[i], bucket, region)
		if err != nil {
			return err
		}
		if keys != nil {
			for _,key := range *keys {
				fileHd, err := os.Create(strings.Replace(key, "jetstore/workspaces", wh, 1))
				if err != nil {
					return fmt.Errorf("failed to open local workspace file for write: %v", err)
				}
		
				// Download the object
				nsz, err := awsi.DownloadFromS3(bucket, region, key, fileHd)
				fileHd.Close()
				if err != nil {
					return fmt.Errorf("failed to download input file: %v", err)
				}
				fmt.Println("downloaded",key,"size", nsz,"bytes from s3")
			}
		}
	}
	log.Println("Done synching overriten workspace file from s3")
	return nil
}

func CompileWorkspace(dbpool *pgxpool.Pool, version string) error {

		bucket := os.Getenv("JETS_BUCKET")
		region := os.Getenv("JETS_REGION")
		wh := os.Getenv("WORKSPACES_HOME")
		wk := os.Getenv("WORKSPACE")
		compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", wh, wk)

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

		// Copy the sqlite file to s3
		sourcesPath := []string{
			fmt.Sprintf("%s/%s/lookup.db", wh, wk),
			fmt.Sprintf("%s/%s/workspace.db", wh, wk),
		}
		sourcesKey := []string{
			fmt.Sprintf("jetstore/workspaces/%s/lookup.db", wk),
			fmt.Sprintf("jetstore/workspaces/%s/workspace.db", wk),
		}
		for i := range sourcesPath {
			// aws integration: Copy the file to awsBucket
			file, err := os.Open(sourcesPath[i])
			if err != nil {
				log.Printf("While opening local output file: %v", err)
				return err
			}
			err = awsi.UploadToS3(bucket, region, sourcesKey[i], file)
			file.Close()
			if err != nil {
				return fmt.Errorf("failed to upload file to s3: %v", err)
			}
		}

		// insert the new workspace version in jetsapi db
		stmt := "INSERT INTO jetsapi.workspace_version (version) VALUES ($1)"
		_, err = dbpool.Exec(context.Background(), stmt, version)
		if err != nil {
			return fmt.Errorf("while inserting workspace version into workspace_version table: %v", err)
		}
	

	return nil
}