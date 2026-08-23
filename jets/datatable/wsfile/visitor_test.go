package wsfile

import (
	"os"
	"path/filepath"
	"testing"
)

// The failure this file exists for: a workspace section whose directory is not
// there used to fail the *whole* workspace_file_structure request rather than
// its own heading, because fs.WalkDir calls its callback with a stat error and
// the callback returned it (task F.0a, 2026-08-23).
//
// It was live rather than hypothetical. `user_flows` has been in the section list
// since S.3 and no workspace in the corpus has the directory, so the IDE's file
// tree was answering 500 for every workspace — and the symptom names the server,
// not the folder.

func TestVisitDirWrapperOnAMissingDirectoryYieldsAnEmptySection(t *testing.T) {
	root := t.TempDir()

	node, err := VisitDirWrapper(root, "user_flows", "User Flows", &[]string{".uf.json"}, "ws")
	if err != nil {
		t.Fatalf("a missing section directory must not be an error: %v", err)
	}
	if node.Type != "section" || node.Label != "User Flows" {
		t.Fatalf("want the section node anyway, got %+v", node)
	}
	if node.Children == nil || len(*node.Children) != 0 {
		t.Fatalf("want an empty section, got %v children", node.Children)
	}
}

func TestVisitDirWrapperStillReadsADirectoryThatExists(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "table_configs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// One file that matches the filter and one that does not, so the assertion
	// below is about the filter rather than about the count.
	for name, content := range map[string]string{
		"lfSourceConfigTable.tc.json": "{}",
		"notes.md":                    "ignore me",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	node, err := VisitDirWrapper(root, "table_configs", "Table Configurations", &[]string{".tc.json"}, "ws")
	if err != nil {
		t.Fatal(err)
	}
	if node.Children == nil || len(*node.Children) != 1 {
		t.Fatalf("want exactly the .tc.json file, got %v", node.Children)
	}
	child := (*node.Children)[0]
	if child.Type != "file" || child.Label != "lfSourceConfigTable.tc.json" {
		t.Fatalf("unexpected child %+v", child)
	}
	// The escaped relative path is what get_workspace_file_content unescapes, and
	// what the client passes straight back — see readWorkspaceFile in
	// jetsclient_ide/src/api/workspace.ts.
	if got, want := child.RouteParams["file_name"], "table_configs%2FlfSourceConfigTable.tc.json"; got != want {
		t.Fatalf("file_name = %q, want %q", got, want)
	}
}

// A path that exists and is not a directory is a real fault and still fails: the
// change above swallows fs.ErrNotExist and nothing else.
func TestVisitDirWrapperStillFailsWhenTheSectionIsNotADirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lookups"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VisitDirWrapper(root, "lookups", "Lookups", &[]string{".csv"}, "ws"); err == nil {
		t.Fatal("want an error when the section path is a file")
	}
}
