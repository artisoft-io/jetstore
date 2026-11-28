package wsfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// installedWorkspace is a workspace JetStore has installed into, by
// workspace_assets.Install, plus a pipeline configuration of the workspace's
// own beside the installed ones, its README, and a rule file.
func installedWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := workspace_assets.Install(root, workspace_assets.Options{}); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"pipes_config/mine.pc.json": "{}",
		"pipes_config/README.md":    "# pipes",
		"jet_rules/a.jr":            "# rules",
	} {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

type nodeMark struct {
	readOnly  bool
	installed bool
	note      string
}

func treeMarks(t *testing.T, root string) (map[string]nodeMark, []*WorkspaceNode) {
	t.Helper()
	nodes, err := BuildWorkspaceFileStructure(root, "ws")
	if err != nil {
		t.Fatal(err)
	}
	marks := map[string]nodeMark{}
	var walk func(*WorkspaceNode)
	walk = func(n *WorkspaceNode) {
		if n.Type == "file" {
			marks[n.PageMatchKey] = nodeMark{n.ReadOnly, n.Installed, n.InstalledNote}
		}
		if n.Children != nil {
			for _, c := range *n.Children {
				walk(c)
			}
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return marks, nodes
}

// Every file the install wrote is marked installed, with the note, and stays
// editable: read_only is IsReadOnly's alone. The workspace's own files beside
// them are not marked (JM-389).
func TestTheFilesJetStoreInstallsAreMarkedAndStayEditable(t *testing.T) {
	root := installedWorkspace(t)
	marks, nodes := treeMarks(t, root)

	installed := 0
	for _, g := range workspace_assets.AssetGroups {
		names, err := workspace_assets.Names(g.Dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			m, listed := marks[g.Dir+"%2F"+name]
			if !listed {
				// TestEveryInstalledAssetIsVisibleInTheIde owns that failure.
				continue
			}
			installed++
			if !m.installed || m.note != InstalledNote || m.readOnly {
				t.Errorf("%s/%s: %+v, want installed with the note, not read-only", g.Dir, name, m)
			}
		}
	}
	if installed == 0 {
		t.Fatal("no installed file is in the tree")
	}
	for key, want := range map[string]nodeMark{
		"pipes_config%2Fmine.pc.json": {},
		"jet_rules%2Fa.jr":            {},
		"pipes_config%2FREADME.md":    {readOnly: true},
	} {
		if got := marks[key]; got != want {
			t.Errorf("%s, the workspace's own: %+v, want %+v", key, got, want)
		}
	}

	// On the wire the mark is `installed` and `installed_note`, and absent
	// when the file is not installed.
	for _, n := range nodes {
		encoded, err := json.Marshal(n)
		if err != nil {
			t.Fatal(err)
		}
		switch n.Key {
		case "user_flows":
			if !strings.Contains(string(encoded), `"installed":true,"installed_note":"Installed by JetStore`) ||
				strings.Contains(string(encoded), "read_only") {
				t.Errorf("the user_flows section encodes as %s", encoded)
			}
		case "jet_rules":
			if strings.Contains(string(encoded), "installed") {
				t.Errorf("the jet_rules section encodes with installed: %s", encoded)
			}
		}
	}
}

// Only the names a manifest records are marked: a workspace JetStore never
// installed into has nothing marked, and a file nested under a group's
// directory, though named as an asset, is not one.
func TestOnlyWhatAManifestRecordsIsMarked(t *testing.T) {
	root := installedWorkspace(t)
	names, err := workspace_assets.Names("table_configs")
	if err != nil || len(names) == 0 {
		t.Fatalf("table_configs: %v, %v", names, err)
	}
	nested := filepath.Join(root, "table_configs", "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, names[0]), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	marks, _ := treeMarks(t, root)
	if m := marks["table_configs%2Fsub%2F"+names[0]]; m.installed {
		t.Errorf("a nested namesake is marked: %+v", m)
	}
	if m := marks["table_configs%2F"+names[0]]; !m.installed {
		t.Errorf("table_configs/%s is not marked: %+v", names[0], m)
	}

	for _, g := range workspace_assets.AssetGroups {
		if err := os.Remove(filepath.Join(root, g.Dir, workspace_assets.ManifestName)); err != nil {
			t.Fatal(err)
		}
	}
	marks, _ = treeMarks(t, root)
	for key, m := range marks {
		if m.installed {
			t.Errorf("with no manifest, %s is marked", key)
		}
	}
}
