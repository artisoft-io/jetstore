package main

import (
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/workspace"
)

/*
WORKSPACES_HOME=/home/michel/projects/repos/workspaces \
JETSTORE_DEV_MODE=1 \
WORKSPACE=usi_ws \
WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db \
WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db \
./compilerv2_test
*/

func main() {

	os.Remove("/home/michel/projects/repos/usi_ws/workspaceV2.db")
	// Load the workspace control file to determine which compiler to use
	txtLog, err := workspace.CompileWorkspace(nil, "", "000001")
	if err != nil {
		err = fmt.Errorf("while CompileWorkspace: %v", err)
		log.Fatal(err)
	}

	log.Println(txtLog)
	log.Println("Done")
}
