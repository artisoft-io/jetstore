package compiler

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

// The tests that pass saveJson=true write their model to testdata/workspace.db,
// and SaveJetRuleModel refuses to save a main rule file that is already in the
// database (workspace_db.go:73) — the same guard the workspace compile answers
// by deleting workspace.db before it starts. The database is git-ignored and
// survives between runs, so without this the suite passes once and then fails
// with "main source file ... already exists in workspace db" on every run after.
func TestMain(m *testing.M) {
	for _, name := range []string{"workspace.db", "workspaceV2.db"} {
		path := filepath.Join("testdata", name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Fatalf("while clearing %s: %v", path, err)
		}
	}
	os.Exit(m.Run())
}
