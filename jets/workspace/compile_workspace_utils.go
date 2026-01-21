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
var workspaceHome, wprefix, workspaceControlPath, workspaceBuildPath, workspaceVersion, localRepoVersion string
var devMode bool
var lastWorkspaceSyncCheck *time.Time

func init() {
	localRepoVersion = os.Getenv("JETS_VERSION")
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	workspaceControlPath = fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, wprefix)
	workspaceBuildPath = fmt.Sprintf("%s/%s/build", workspaceHome, wprefix)

	_, devMode = os.LookupEnv("JETSTORE_DEV_MODE")
}

func WorkspacesHome() string {
	return workspaceHome
}

func WorkspacePrefix() string {
	return wprefix
}

func WorkspaceDbPath() string {
	return fmt.Sprintf("%s/%s/workspace.db", workspaceHome, wprefix)
}

func LookupDbPath() string {
	return fmt.Sprintf("%s/%s/lookup.db", workspaceHome, wprefix)
}

func DevMode() bool {
	return devMode
}

func WorkspaceBuildDir() string {
	return workspaceBuildPath
}

func WorkspaceControlFilePath() string {
	return workspaceControlPath
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
func SyncWorkspaceFiles(dbpool *pgxpool.Pool, workspaceName, contentType string, skipSqliteFiles bool, skipTgzFiles bool) (bool, error) {
	// sync workspace files from db to locally
	if devMode {
		return false,nil
	}
	// Get all file_name that are modified
	compilationRequired := false
	if len(contentType) > 0 {
		log.Printf("Start synching overriten workspace file with content_type '%s' from database", contentType)
	} else {
		log.Printf("Start synching overriten workspace file from database")
	}
	fileObjects, err := dbutils.QueryFileObject(dbpool, workspaceName, contentType)
	if err != nil {
		return false, err
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
				return false, fmt.Errorf("while creating file directory structure: %v", err)
			}
			// Put obj to local file system
			err = fo.WriteDbObject2LocalFile(dbpool, localFileName)
			if err != nil {
				return false, err
			}
			// If FileName ends with .tgz, extract files from archive
			switch {
			case strings.HasSuffix(fo.FileName, ".tgz"):
				err = extractTgz(localFileName, fmt.Sprintf("%s/%s", workspaceHome, workspaceName))
				if err != nil {
					return false, err
				}
			case strings.HasSuffix(fo.FileName, ".db"):
			default:
				log.Printf("*** compilation required due to override of file %s", fo.FileName)
				compilationRequired = true
			}
		} else {
			log.Println("Skipping file", fo.FileName)
		}
	}
	log.Println("Done synching overriten workspace file from database, compilationRequired?", compilationRequired)
	return compilationRequired, nil
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
// Return true if a sync was performed.
// Note: run report lambdas do not need the full workspace, only the reports definitions.
// hence only sync the reports.tgz file.
// Note: in dev mode, do not sync from database.
// Note: sync is only performed if more than 1 min since last check to avoid too many db calls.
// Note: No sync if db version is same as local repo version (meaning workspace taken from local repo).
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
		workspaceVersion = version
		if len(localRepoVersion) > 0 && localRepoVersion == workspaceVersion {
			// No need to sync since workspace taken from local repo
			log.Printf("Skipping sync of reports.tgz since workspace version %s is same as local repo version", workspaceVersion)
			return false, nil
		}
		// Get the reports
		_, err = SyncWorkspaceFiles(dbpool, wprefix, "reports.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching reports.tgz file from db: %v", err)
		}
		didSync = true
	} else {
		log.Printf("No need to sync run reports workspace, version %s is same as last synced version", workspaceVersion)
	}
	return didSync, nil
}

// Sync the workspace files for cpipes lambdas if a new version of the workspace exist since the last call.
// Return true if a sync was performed.
// Synch both workspace.tgz and sqlite files.
// Note: in dev mode, do not sync from database.
// Note: sync is only performed if more than 1 min since last check to avoid too many db calls.
// Note: No sync if db version is same as local repo version (meaning workspace taken from local repo).
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
		workspaceVersion = version
		if len(localRepoVersion) > 0 && localRepoVersion == workspaceVersion {
			// No need to sync since workspace taken from local repo
			log.Printf("Skipping sync of workspace.tgz and sqlite since workspace version %s is same as local repo version", workspaceVersion)
			return false, nil
		}
		// Get the compiled rules
		_, err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "workspace.tgz", true, false)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		// Get the compiled lookups
		_, err = SyncWorkspaceFiles(dbpool, os.Getenv("WORKSPACE"), "sqlite", false, true)
		if err != nil {
			return false, fmt.Errorf("error while synching workspace file from db: %v", err)
		}
		didSync = true
	} else {
		log.Printf("No need to sync compute pipes workspace, version %s is same as last synced version", workspaceVersion)
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
