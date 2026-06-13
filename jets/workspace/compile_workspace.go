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

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string) (string, error) {

	// Load the workspace control file to determine which compiler to use
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		return err.Error(), err
	}
	log.Println("Using workspace compiler v2 with WORKSPACE_HOME=", WorkspacesHome())
	return compileWorkspaceV2(dbpool, workspaceControl, version)
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
