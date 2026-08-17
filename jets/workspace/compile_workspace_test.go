package workspace

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestCompileWorkspaceV2_Workspace1(t *testing.T) {

	// Load the workspace control file to determine which compiler to use
	workspaceControlPath := "/home/michel/projects/repos/usi_ws/workspace_control.json"
	if _, err := os.Stat(workspaceControlPath); err != nil {
		t.Skipf("no workspace at %s", workspaceControlPath)
	}
	workspaceControl, err := rete.LoadWorkspaceControl(workspaceControlPath)
	if err != nil {
		err = fmt.Errorf("while loading workspace_control.json: %v", err)
		t.Fatal(err)
	}

	log.Println("Using workspace compiler v2")
	txtLog, err := compileWorkspaceV2(nil, workspaceControl, "000001")
	if err != nil {
		t.Fatal(err)
	}
	log.Println(txtLog)
	t.Error("Done")
}
