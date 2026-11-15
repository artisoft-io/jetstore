package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// An override of a page of the workspace guide, or of its settings file, or of
// the workspace's root README (since 2026-11-14), does not make the
// apiserver's start-up compile the workspace; every other override still does,
// including one whose path only names docs/ on the way elsewhere, and another
// README.
func TestOnlyTheWorkspaceGuideAndRootReadmeAreExemptFromTheCompile(t *testing.T) {
	for fileName, want := range map[string]bool{
		"README.md":                            false,
		"readme.md":                            false,
		"./README.md":                          false,
		"CHANGES.md":                           true,
		"jet_rules/README.md":                  true,
		"docs/index.md":                        false,
		"docs/customisation/observe.md":        false,
		"docs/index.mdx":                       false,
		"docs/workspace_docs.json":             false,
		"docs/images/diagram.png":              false,
		"jet_rules/main_rules.jr":              true,
		"data_model/classes.jr":                true,
		"lookups/codes.csv":                    true,
		"pipes_config/README.md":               true,
		"pipes_config/docs/notes.md":           true,
		"docs/../jet_rules/main_rules.jr":      true,
		"mydocs/index.md":                      true,
		"workspace_control.json":               true,
		"reports/report.json":                  true,
		"process_config/workspace_init_db.sql": true,
		"docs\\..\\lookups\\codes.csv":         true,
		"docs\\customisation\\observe.md":      false,
		"./docs/customisation/observe.md":      false,
	} {
		if got := overrideRequiresCompile(fileName); got != want {
			t.Errorf("overrideRequiresCompile(%q) = %v, want %v", fileName, got, want)
		}
	}
}

// An override of a document JetStore has retired is not written back at a
// start: the install deleted the document, and the override would restore it.
// Every other override is applied, and the retired one's row is kept. Needs
// JETS_TEST_DSN (workspaceChangesPool, upload_workspace_assets_test.go).
func TestSyncDoesNotRestoreARetiredDocument(t *testing.T) {
	pool := workspaceChangesPool(t)
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	savedHome, savedDev := workspaceHome, devMode
	workspaceHome, devMode = home, false
	t.Cleanup(func() { workspaceHome, devMode = savedHome, savedDev })

	const ws = "sync_ws"
	retired := workspace_assets.RetiredAssets[0]
	retiredFile := retired.Dir + "/" + retired.Name
	for _, name := range []string{retiredFile, "table_configs/kept.tc.json"} {
		fo := dbutils.FileDbObject{WorkspaceName: ws, FileName: name,
			ContentType: strings.SplitN(name, "/", 2)[0], UserEmail: "ke@example.com"}
		if _, err := fo.WriteObject(pool, []byte(`{"edited": true}`)); err != nil {
			t.Fatal(err)
		}
	}
	compile, err := SyncWorkspaceFiles(pool, ws, "", true, true)
	if err != nil {
		t.Fatalf("SyncWorkspaceFiles: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ws, retiredFile)); !os.IsNotExist(err) {
		t.Errorf("the retired document %s was written back (stat: %v)", retiredFile, err)
	}
	if b, err := os.ReadFile(filepath.Join(home, ws, "table_configs", "kept.tc.json")); err != nil || string(b) != `{"edited": true}` {
		t.Errorf("the other override was not applied: %q %v", b, err)
	}
	if !compile {
		t.Error("the applied override requires a compile, and the sync reported none")
	}
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.workspace_changes WHERE file_name = $1`, retiredFile).Scan(&n); err != nil || n != 1 {
		t.Errorf("the retired document's row: %d rows, %v; want it kept", n, err)
	}
}

func TestIsRetiredNamesOnlyRetiredPaths(t *testing.T) {
	r := workspace_assets.RetiredAssets[0]
	for name, want := range map[string]bool{
		r.Dir + "/" + r.Name:         true,
		"./" + r.Dir + "/" + r.Name:  true,
		r.Name:                       false,
		"other/" + r.Name:            false,
		r.Dir + "/kept_" + r.Name:    false,
		"table_configs/kept.tc.json": false,
	} {
		if got := workspace_assets.IsRetired(name); got != want {
			t.Errorf("IsRetired(%q) = %v, want %v", name, got, want)
		}
	}
}
