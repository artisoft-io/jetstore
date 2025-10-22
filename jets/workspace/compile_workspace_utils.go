package workspace

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Active workspace prefix and control file path
var workspaceHome, wprefix, workspaceControlPath, workspaceBuildPath, workspaceVersion string
var devMode bool
var lastWorkspaceSyncCheck *time.Time

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	workspaceControlPath = fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, wprefix)
	workspaceBuildPath = fmt.Sprintf("%s/%s/build", workspaceHome, wprefix)

	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")

}

// This file contains functions to compile and sync the workspace
// between jetstore database and the local container
// WORKSPACE Workspace default currently in use
// WORKSPACES_HOME Home dir of workspaces

// Function to pull override workspace files from databse to the
// container workspace (local copy).
// Need this when:
//   - starting a task requiring local workspace (e.g. run_report to get latest report definition)
//   - starting apiserver to get latest override files (e.g. lookup csv files) to compile workspace
//   - starting rule server to get the latest lookup.db and workspace.db
func SyncWorkspaceFiles(dbpool *pgxpool.Pool, workspaceName, contentType string, skipSqliteFiles bool, skipTgzFiles bool) error {
	// sync workspace files from db to locally
	if devMode {
		return nil
	}
	// Get all file_name that are modified
	if len(contentType) > 0 {
		log.Printf("Start synching overriten workspace file with content_type '%s' from database", contentType)
	} else {
		log.Printf("Start synching overriten workspace file from database")
	}
	fileObjects, err := dbutils.QueryFileObject(dbpool, workspaceName, contentType)
	if err != nil {
		return err
	}
	for _, fo := range fileObjects {
		// When in skipSqliteFiles == true, do not override lookup.db and workspace.db
		// When in skipTgzFiles == true, do not override *.tgz files
		if (!skipSqliteFiles || !strings.HasSuffix(fo.FileName, ".db")) &&
			(!skipTgzFiles || !strings.HasSuffix(fo.FileName, ".tgz")) {
			localFileName := fmt.Sprintf("%s/%s/%s", workspaceHome, workspaceName, fo.FileName)
			// create workspace.tgz file and dir structure
			fileDir := filepath.Dir(localFileName)
			if err = os.MkdirAll(fileDir, 0770); err != nil {
				return fmt.Errorf("while creating file directory structure: %v", err)
			}
			// Put obj to local file system
			err = fo.WriteDbObject2LocalFile(dbpool, localFileName)
			if err != nil {
				return err
			}
			// If FileName ends with .tgz, extract files from archive
			if strings.HasSuffix(fo.FileName, ".tgz") {
				err = extractTgz(localFileName, fmt.Sprintf("%s/%s", workspaceHome, workspaceName))
				if err != nil {
					return err
				}
			}
		} else {
			log.Println("Skipping file", fo.FileName)
		}
	}
	log.Println("Done synching overriten workspace file from database")
	return nil
}

func extractTgz(sourceFileName, destBaseDir string) error {
	fileHd, err := os.Open(sourceFileName)
	if err != nil {
		return fmt.Errorf("failed to open tgz file %s for read: %v", sourceFileName, err)
	}
	defer fileHd.Close()
	err = tarextract.ExtractTarGz(fileHd, destBaseDir)
	if err != nil {
		return fmt.Errorf("failed to extract content from tgz file %s for read: %v", sourceFileName, err)
	}
	return nil
}

// Sync the workspace files for run report lambdas if a new version of the workspace exist since the last call.
// Return true if a sync was performed
func SyncRunReportsWorkspace(dbpool *pgxpool.Pool) (bool, error) {
	if devMode {
		return false, nil
	}
	// See if it worth to do a check
	if lastWorkspaceSyncCheck != nil && time.Since(*lastWorkspaceSyncCheck) < time.Duration(1)*time.Minute {
		// No need to check since it was check less than a min ago
		return false, nil
	}
	now := time.Now()
	lastWorkspaceSyncCheck = &now

	// Get the latest workspace version
	// Check the workspace release in database vs current release
	version, err := GetWorkspaceVersion(dbpool)
	if err != nil {
		return false, err
	}
	didSync := false
	if version != workspaceVersion {
		// Get the reports
		err = SyncWorkspaceFiles(dbpool, wprefix, "reports.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching reports.tgz file from db: %v", err)
		}
		workspaceVersion = version
		didSync = true
	}
	return didSync, nil
}

// Sync the workspace files for cpipes lambdas if a new version of the workspace exist since the last call.
// Return true if a sync was performed
func SyncComputePipesWorkspace(dbpool *pgxpool.Pool) (bool, error) {
	if devMode {
		return false, nil
	}
	// See if it worth to do a check
	if lastWorkspaceSyncCheck != nil && time.Since(*lastWorkspaceSyncCheck) < time.Duration(1)*time.Minute {
		// No need to check since it was check less than a min ago
		return false, nil
	}
	now := time.Now()
	lastWorkspaceSyncCheck = &now

	// Get the latest workspace version
	// Check the workspace release in database vs current release
	version, err := GetWorkspaceVersion(dbpool)
	if err != nil {
		return false, err
	}
	didSync := false
	if version != workspaceVersion {
		// Get the compiled rules
		err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "workspace.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		// Get the compiled lookups
		err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "sqlite", false, true)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		workspaceVersion = version
		didSync = true
	}
	return didSync, nil
}

func GetWorkspaceVersion(dbpool *pgxpool.Pool) (string, error) {
	var version string
	stmt := "SELECT MAX(version) FROM jetsapi.workspace_version"
	err := dbpool.QueryRow(context.Background(), stmt).Scan(&version)
	if err != nil {
		return "", fmt.Errorf("while checking latest workspace version: %v", err)
	}
	return version, nil
}

func UpdateWorkspaceVersionDb(dbpool *pgxpool.Pool, workspaceName, version string) error {

	if version == "" {
		log.Println("Error: attempting to write empty version to table workspace_version, skipping")
		return nil
	}
	// insert the new workspace version in jetsapi db
	log.Println("Updating workspace version in database to", version)
	stmt := "INSERT INTO jetsapi.workspace_version (version) VALUES ($1) ON CONFLICT DO NOTHING"
	_, err := dbpool.Exec(context.Background(), stmt, version)
	if err != nil {
		return fmt.Errorf("while inserting workspace version into workspace_version table: %v", err)
	}

	return nil
}
