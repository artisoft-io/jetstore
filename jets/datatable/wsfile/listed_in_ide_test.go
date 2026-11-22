package wsfile

// ListedInIde is the file tree's predicate asked of a path. It is held to the
// tree itself: over a workspace holding listed and unlisted files, a file is
// in BuildWorkspaceFileStructure's answer exactly when ListedInIde says so.

import (
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestListedInIdeAgreesWithTheTree(t *testing.T) {
	root := t.TempDir()
	files := map[string]bool{
		"runbooks/health_check.rb.json":        true,
		"runbooks/health_check/count.sql":      true,
		"runbooks/health_check/deep/more.sql":  true,
		"runbooks/health_check/notes.txt":      false,
		"jet_rules/a.jr":                       true,
		"jet_rules/unit_tests/t.tst.json":      true,
		"jet_rules/a.jr.json":                  false,
		"data_model/jets_assets_manifest.json": false,
		"pipes_config/a.pc.json":               true,
		"pipes_config/README.md":               true,
		"reports/config.json":                  true,
		"docs/guide/page.mdx":                  true,
		"user_flows/a.uf.json":                 true,
		"scratch/a.sql":                        false,
		"workspace_control.json":               true,
		"compile_workspace.sh":                 true,
		"README.md":                            true,
		"lookup.db":                            false,
		"other.sql":                            false,
	}
	for name := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	nodes, err := BuildWorkspaceFileStructure(root, "ws")
	if err != nil {
		t.Fatal(err)
	}
	inTree := map[string]bool{}
	var walk func([]*WorkspaceNode)
	walk = func(ns []*WorkspaceNode) {
		for _, n := range ns {
			if n.Type == "file" {
				name, err := url.QueryUnescape(n.RouteParams["file_name"])
				if err != nil {
					t.Fatal(err)
				}
				inTree[name] = true
			}
			if n.Children != nil {
				walk(*n.Children)
			}
		}
	}
	walk(nodes)

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if inTree[name] != files[name] {
			t.Errorf("%s: in the tree %v, the test expects %v", name, inTree[name], files[name])
		}
		if got := ListedInIde(name); got != inTree[name] {
			t.Errorf("ListedInIde(%q) = %v, and the tree lists it: %v", name, got, inTree[name])
		}
	}
}

func TestListedInIdeRefusesWhatIsNotAWorkspacePath(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../runbooks/a.sql", "/runbooks/a.sql", "runbooks", "runbooks/"} {
		if ListedInIde(name) {
			t.Errorf("ListedInIde(%q) = true", name)
		}
	}
}
