package wsfile

// The `docs` section: a workspace's documentation pages and the guide's
// settings file, listed, read and saved like any other workspace file
// (2026-11-13). The save check itself is in package datatable
// (TestTheSavePathSavesADocsPage, workspace_file_sections_test.go).

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docsWorkspace writes a workspace under a temporary WORKSPACES_HOME holding a
// docs/ folder, a README beside the pipelines, and files the section must not
// list. It returns the workspace's name and root.
func docsWorkspace(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "docs_ws"
	root := filepath.Join(home, ws)
	for path, content := range map[string]string{
		"docs/index.md":                 "---\ntitle: ws\n---\n",
		"docs/customisation/index.md":   "---\ntitle: Customisation\n---\n",
		"docs/customisation/observe.md": "---\ntitle: The observe operator\n---\nobserve\n",
		"docs/customisation/chart.mdx":  "---\ntitle: A chart\n---\n",
		"docs/workspace_docs.json":      `{"rule_text": false}`,
		"docs/diagram.png":              "png",
		"docs/notes.txt":                "notes",
		"pipes_config/README.md":        "# pipes",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws, root
}

// filesUnder returns the file nodes below n, by escaped file_name, with their
// read_only flag.
func filesUnder(n *WorkspaceNode, into map[string]bool) {
	if n.Type == "file" {
		into[n.RouteParams["file_name"]] = n.ReadOnly
	}
	if n.Children != nil {
		for _, c := range *n.Children {
			filesUnder(c, into)
		}
	}
}

func TestTheDocsSectionListsPagesAndSettingsWritable(t *testing.T) {
	ws, root := docsWorkspace(t)
	nodes, err := BuildWorkspaceFileStructure(root, ws)
	if err != nil {
		t.Fatal(err)
	}
	var section *WorkspaceNode
	files := map[string]bool{}
	for _, n := range nodes {
		if n.Key == "docs" {
			section = n
		}
		filesUnder(n, files)
	}
	if section == nil || section.Type != "section" || section.Label != "Documentation" || section.CompiledView != "" {
		t.Fatalf("docs section: %+v", section)
	}

	docs := map[string]bool{}
	filesUnder(section, docs)
	want := []string{
		"docs/customisation/chart.mdx",
		"docs/customisation/index.md",
		"docs/customisation/observe.md",
		"docs/index.md",
		"docs/workspace_docs.json",
	}
	if len(docs) != len(want) {
		t.Errorf("docs files: got %v, want %v", docs, want)
	}
	for _, path := range want {
		readOnly, listed := docs[url.QueryEscape(path)]
		switch {
		case !listed:
			t.Errorf("%s is not listed in the docs section", path)
		case readOnly:
			t.Errorf("%s is listed read-only; a page under docs/ is saved", path)
		}
	}
	// A README elsewhere stays read-only.
	if ro, listed := files[url.QueryEscape("pipes_config/README.md")]; !listed || !ro {
		t.Errorf("pipes_config/README.md: listed %v, read_only %v, want listed and read-only", listed, ro)
	}
}

func TestIsReadOnlyExemptsTheDocsFolderOnly(t *testing.T) {
	for path, want := range map[string]bool{
		"README.md":                         true,
		"pipes_config/README.md":            true,
		"docs/index.md":                     false,
		"docs/customisation/observe.md":     false,
		"./docs/index.md":                   false,
		"docs/workspace_docs.json":          false,
		"docs/../pipes_config/README.md":    true,
		"docs/../README.md":                 true,
		"pipes_config/docs/README.md":       true,
		"mydocs/README.md":                  true,
		"jet_rules/a.jr":                    false,
		"docs\\customisation\\observe.md":   false,
		"docs\\..\\pipes_config\\README.md": true,
		"docs/index.mdx":                    false,
		"pipes_config/notes.mdx":            true,
		"docs/../pipes_config/notes.mdx":    true,
	} {
		if got := IsReadOnly(path); got != want {
			t.Errorf("IsReadOnly(%q) = %v, want %v", path, got, want)
		}
	}
}

// Reading: the IDE reads a file by its unescaped workspace path, and a page in a
// subfolder of docs/ is read like any other.
func TestADocsPageIsReadByItsWorkspacePath(t *testing.T) {
	ws, _ := docsWorkspace(t)
	got, err := GetContent(ws, "docs/customisation/observe.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "observe\n") {
		t.Errorf("content %q, want the page", got)
	}
}

// Saving: a page is written to the workspace and recorded as an override whose
// content type is its first path segment, `docs`, which is what
// SyncWorkspaceFiles restores after a restart. Skipped unless JETS_TEST_DSN
// names a PostgreSQL (revertDB, delete_changes_test.go).
func TestADocsPageSavesAsAnOverride(t *testing.T) {
	pool := revertDB(t)
	bg := context.Background()
	for _, s := range []string{
		"DROP TABLE jetsapi.workspace_changes",
		`CREATE TABLE jetsapi.workspace_changes (key serial PRIMARY KEY, workspace_name text NOT NULL,
			oid bigint NOT NULL, data bytea, file_name text NOT NULL, content_type text NOT NULL,
			status text NOT NULL DEFAULT '', user_email text NOT NULL, last_update timestamp NOT NULL DEFAULT now(),
			CONSTRAINT workspace_changes_unique_cstraint UNIQUE (workspace_name, file_name))`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatal(err)
		}
	}
	ws, root := docsWorkspace(t)
	const page = "docs/customisation/observe.md"
	const edited = "---\ntitle: The observe operator\n---\nedited\n"
	if err := SaveContent(pool, ws, page, edited, "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(root, page)); got != edited {
		t.Errorf("workspace file %q, want %q", got, edited)
	}
	var contentType string
	if err := pool.QueryRow(bg, "SELECT content_type FROM jetsapi.workspace_changes WHERE file_name = $1", page).
		Scan(&contentType); err != nil {
		t.Fatal(err)
	}
	if contentType != "docs" {
		t.Errorf("content_type %q, want docs", contentType)
	}
}
