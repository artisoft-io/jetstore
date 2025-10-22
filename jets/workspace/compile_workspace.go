package workspace

import (
	"fmt"
	"log"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Workspace compilation function

func CompileWorkspace(dbpool *pgxpool.Pool, workspaceName, version string) (string, error) {

	// Load the workspace control file to determine which compiler to use
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		return err.Error(), err
	}

	if workspaceControl.UseCompilerV2 {
		log.Println("Using workspace compiler v2")
		return compileWorkspaceV2(dbpool, workspaceControl, version)
	}
	log.Println("Using workspace compiler v1")
	return compileWorkspaceV1(dbpool, workspaceName, version)
}
