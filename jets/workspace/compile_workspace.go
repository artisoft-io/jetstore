package workspace

import (
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Workspace compilation function
//
// **The workspace's user-flow and table documents are validated first, and an
// invalid one fails the compile** (jetstore_maintenance_02 AG.4, 2026-10-01): see
// asset_validation.go. Every caller gets the gate unless it passes
// SkipAssetValidation with a reason -- run_reports does, and says why.

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string, opts ...CompileOption) (string, error) {

	// Load the workspace control file to determine which compiler to use
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		return err.Error(), err
	}

	var options compileOptions
	for _, opt := range opts {
		opt(&options)
	}
	// The directory compileWorkspaceV2 compiles from, so the documents checked
	// are the ones beside the rules being compiled.
	validationLog, err := validateWorkspaceAssets(
		fmt.Sprintf("%s/%s", workspaceHome, workspaceControl.WorkspaceName), options)
	if err != nil {
		return validationLog, err
	}

	log.Println("Using workspace compiler v2 with WORKSPACE_HOME=", WorkspacesHome())
	compileLog, err := compileWorkspaceV2(dbpool, workspaceControl, version)
	return validationLog + compileLog, err
}

func UploadWorkspaceAssets(dbpool *pgxpool.Pool, workspaceName, version string) error {
	// Copy the sqlite files & the tar file to db
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
			return err
		}
		_, err = fo[i].WriteObject(dbpool, data)
		if err != nil {
			return fmt.Errorf("failed to write object to db: %v", err)
		}
	}
	return nil
}
