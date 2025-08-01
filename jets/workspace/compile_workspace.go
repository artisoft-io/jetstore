package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable/wsfile"
	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Active workspace prefix and control file path
var workspaceHome, wprefix, workspaceControlPath, workspaceVersion string
var devMode bool
var lastWorkspaceSyncCheck *time.Time

func init() {
	workspaceHome = os.Getenv("WORKSPACES_HOME")
	wprefix = os.Getenv("WORKSPACE")
	workspaceControlPath = fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, wprefix)
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
	version, err := getWorkspaceVersion(dbpool)
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
	version, err := getWorkspaceVersion(dbpool)
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

func getWorkspaceVersion(dbpool *pgxpool.Pool) (string, error) {
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

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string) (string, error) {

	compilerPath := fmt.Sprintf("%s/%s/compile_workspace.sh", workspaceHome, workspaceName)

	// Compile the workspace locally
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("Compiling workspace %s at version %s\n", workspaceName, version))
	err := wsfile.RunCommand(&buf, compilerPath, nil, workspaceName)
	if err != nil {
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		cmdLog := buf.String()
		log.Println(cmdLog)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return cmdLog, fmt.Errorf("while executing compile_workspace command '%v': %v", compilerPath, err)
	}

	// Archive reports
	inputPath := []string{fmt.Sprintf("%s/%s/reports/", workspaceHome, workspaceName)}
	outputPath := fmt.Sprintf("%s/%s/reports.tgz", workspaceHome, workspaceName)
	err = tarextract.CreateTarGz(fmt.Sprintf("%s/%s", workspaceHome, workspaceName), inputPath, outputPath)
	// command := "tar"
	// args := []string{"cfvz", "reports.tgz", "reports/"}
	buf.WriteString("\nArchiving the reports\n")
	// err = wsfile.RunCommand(&buf, command, &args, workspaceName)
	// cmdLog := buf.String()
	if err != nil {
		buf.WriteString(fmt.Sprintf("While creating reports.tgz: %v", err))
		log.Println(err)
		return buf.String(), err
	}
	log.Println("Workspace reports archived in retports.tgz")

	// Compile the workspace-wide classes and tables, save it in the workspace build directory
	// Get all the main rule files from the workspace_control.json
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		return err.Error(), err
	}
	// Collect all the main rule files
	mainRules := make(map[string]bool)
	for _, name := range workspaceControl.RuleSets {
		mainRules[name] = true
	}
	for i := range workspaceControl.RuleSequences {
		for _, name := range workspaceControl.RuleSequences[i].RuleSets {
			mainRules[name] = true
		}
	}
	// For each main rule file, read the compiled rete network to load only the domain classes and tables
	// definition
	domainClasses := make(map[string]*rete.ClassNode)
	domainTables := make(map[string]*rete.TableNode)
	domainProperties := make(map[string]*rete.DataPropertyNode)
	for name := range mainRules {
		fpath := fmt.Sprintf("%s/%s/build/%s.model.json", workspaceHome,
			wprefix, strings.TrimSuffix(name, ".jr"))
		log.Println("Reading JetStore Model of", name, "from:", fpath)
		file, err := os.ReadFile(fpath)
		if err != nil {
			err = fmt.Errorf("while reading json file (.model.json):%v", err)
			return err.Error(), err
		}
		model := rete.JetruleModel{}
		err = json.Unmarshal(file, &model)
		if err != nil {
			err = fmt.Errorf("while unmarshaling .model.json (compile_workspace):%v", err)
			return err.Error(), err
		}
		for i := range model.Classes {
			class := &model.Classes[i]
			domainClasses[class.Name] = class
			for j := range class.DataProperties {
				class.DataProperties[j].ClassName = class.Name
				domainProperties[class.DataProperties[j].Name] = &class.DataProperties[j]
			}
		}
		for i := range model.Tables {
			table := &model.Tables[i]
			domainTables[table.TableName] = table
		}
	}

	// Save the indexed list of classes, properties and tables to the root of build directory
	fpath := fmt.Sprintf("%s/%s/build/classes.json", workspaceHome,	wprefix)
	log.Println("Writing JetStore Classes to:", fpath)
	file, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening classes.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder := json.NewEncoder(file)
	encoder.Encode(domainClasses)
	file.Close()

	// Properties
	fpath = fmt.Sprintf("%s/%s/build/properties.json", workspaceHome,	wprefix)
	log.Println("Writing JetStore Properties to:", fpath)
	file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening properties.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder = json.NewEncoder(file)
	encoder.Encode(domainProperties)
	file.Close()

	// Tables
	fpath = fmt.Sprintf("%s/%s/build/tables.json", workspaceHome,	wprefix)
	log.Println("Writing JetStore Tables to:", fpath)
	file, err = os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		err = fmt.Errorf("while opening tables.json for write (compile_workspace):%v", err)
		return err.Error(), err
	}
	encoder = json.NewEncoder(file)
	encoder.Encode(domainTables)
	file.Close()

	// Archive the build rules and cpipes config
	inputPath = []string{
		fmt.Sprintf("%s/%s/workspace_control.json", workspaceHome, workspaceName),
		fmt.Sprintf("%s/%s/build/", workspaceHome, workspaceName),
		fmt.Sprintf("%s/%s/pipes_config/", workspaceHome, workspaceName),
	}
	outputPath = fmt.Sprintf("%s/%s/workspace.tgz", workspaceHome, workspaceName)
	buf.WriteString("\nArchiving the build and cpipes config directories\n")
	err = tarextract.CreateTarGz(fmt.Sprintf("%s/%s", workspaceHome, workspaceName), inputPath, outputPath)
	if err != nil {
		buf.WriteString("Error:")
		buf.WriteString(err.Error())
	}
	// command := "tar"
	// args := []string{"cfvz", "workspace.tgz", "workspace_control.json", "build/", "pipes_config/"}
	// buf.WriteString("\nArchiving the build and cpipes config directories\n")
	// err = wsfile.RunCommand(&buf, command, &args, workspaceName)
	cmdLog := buf.String()
	if err != nil {
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		log.Println(cmdLog)
		log.Println("=*=*=*=*=*=*=*=*=*=*=*=*=*=*")
		return cmdLog, fmt.Errorf("while archiving the jet_rules folder : %v", err)
	}

	log.Println("COMPILE WORKSPACE CAPTURED OUTPUT:")
	log.Println("============================")
	log.Println(cmdLog)
	log.Println("============================")

	_, globalDevMode := os.LookupEnv("JETSTORE_DEV_MODE")
	if globalDevMode {
		log.Println("IN DEV MODE = Skipping copy large object to DB")
	} else {
		// Copy the sqlite files & the tar file to db
		buf.WriteString("\nCopy the sqlite file to db\n")
		sourcesPath := []string{
			fmt.Sprintf("%s/%s/lookup.db", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/workspace.db", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/workspace.tgz", workspaceHome, workspaceName),
			fmt.Sprintf("%s/%s/reports.tgz", workspaceHome, workspaceName),
		}
		fileNames := []string{"lookup.db", "workspace.db", "workspace.tgz", "reports.tgz"}
		fo := []dbutils.FileDbObject{
			{WorkspaceName: workspaceName, ContentType: "sqlite", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "sqlite", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "workspace.tgz", UserEmail: "system"},
			{WorkspaceName: workspaceName, ContentType: "reports.tgz", UserEmail: "system"}}
		for i := range sourcesPath {
			fo[i].FileName = fileNames[i]
			data, err := os.ReadFile(sourcesPath[i])
			if err != nil {
				return "", err
			}
			_, err = fo[i].WriteObject(dbpool, data)
			if err != nil {
				buf.WriteString("Failed to upload file to db:")
				buf.WriteString(err.Error())
				buf.WriteString("\n")
				return buf.String(), fmt.Errorf("failed to upload file to db: %v", err)
			}
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
