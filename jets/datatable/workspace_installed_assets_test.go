package datatable

// A file JetStore installs into a workspace is marked in the IDE's tree and
// stays saveable (JM-389, revised 2026-11-27): a save of one is a local
// override, as for any other file, through the same checks.

import (
	"os"
	"path/filepath"
	"testing"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

func TestAFileJetStoreInstallsIsStillSaved(t *testing.T) {
	root := t.TempDir()
	if _, err := workspace_assets.Install(root, workspace_assets.Options{}); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, g := range workspace_assets.AssetGroups {
		names, err := workspace_assets.Names(g.Dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			content, err := os.ReadFile(filepath.Join(root, g.Dir, name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := checkWorkspaceFileIn(g.Dir+"/"+name, string(content), os.DirFS(root)); err != nil {
				t.Errorf("saving %s/%s as installed: %v", g.Dir, name, err)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("nothing was installed")
	}
}
