package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/run_reports/tarextract"
)

func hasSuffix(paths []string, suffix string) bool {
	for _, p := range paths {
		if strings.HasSuffix(p, suffix) {
			return true
		}
	}
	return false
}

func TestWorkspaceArchivePaths(t *testing.T) {
	t.Run("always carries what every workspace has", func(t *testing.T) {
		home := t.TempDir()
		paths := workspaceArchivePaths(home, "ws")
		for _, want := range []string{"/workspace_control.json", "/build/", "/pipes_config/"} {
			if !hasSuffix(paths, want) {
				t.Errorf("%s is not in the archive; a node sees this list and nothing else", want)
			}
		}
	})

	t.Run("omits provenance when the workspace has none", func(t *testing.T) {
		home := t.TempDir()
		if hasSuffix(workspaceArchivePaths(home, "ws"), "/provenance/") {
			// Three of the four workspaces have no provenance/, and CreateTarGz walks
			// every listed path — so naming a missing one fails the whole compile.
			t.Error("provenance/ was listed for a workspace that has none, which fails the archive")
		}
	})

	t.Run("carries provenance when the workspace has one", func(t *testing.T) {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "ws", "provenance"), 0o755); err != nil {
			t.Fatal(err)
		}
		if !hasSuffix(workspaceArchivePaths(home, "ws"), "/provenance/") {
			t.Error("provenance/ was not listed for a workspace that has one")
		}
	})

	t.Run("a file named provenance is not a directory", func(t *testing.T) {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "ws"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, "ws", "provenance"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if hasSuffix(workspaceArchivePaths(home, "ws"), "/provenance/") {
			t.Error("a plain file was listed as a directory, which addFolder would fail on")
		}
	})
}

// The end of the claim the function above only asserts: that a .pv.json listed
// this way actually lands in the archive. Without this the test suite proves the
// path list and not the packaging, and the bug was in the packaging.
func TestProvenanceDocumentReachesTheArchive(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "ws")
	for _, dir := range []string{"build", "pipes_config", "provenance"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "workspace_control.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "provenance", "patient_briefing.pv.json"), []byte(`{"k":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "workspace.tgz")
	if err := tarextract.CreateTarGz(root, workspaceArchivePaths(home, "ws"), out); err != nil {
		t.Fatalf("CreateTarGz: %v", err)
	}
	dest := t.TempDir()
	archive, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if err := tarextract.ExtractTarGz(archive, dest); err != nil {
		t.Fatalf("ExtractTarGz: %v", err)
	}
	// The path the operator opens, relative to the extracted workspace.
	if _, err := os.Stat(filepath.Join(dest, "provenance", "patient_briefing.pv.json")); err != nil {
		t.Errorf("the provenance document is not in the extracted workspace: %v", err)
	}
}
