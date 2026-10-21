package workspace

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestCompileWorkspaceV2_Workspace1(t *testing.T) {

	// Load the workspace control file to determine which compiler to use. A manual
	// test: it runs only on the workspace named by JETS_COMPILE_TEST_WORKSPACE (a
	// directory), and skips without one.
	workspaceControlPath := filepath.Join(os.Getenv("JETS_COMPILE_TEST_WORKSPACE"), "workspace_control.json")
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
