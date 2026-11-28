package workspace_assets

import (
	"os"
	"path/filepath"
	"testing"
)

// A file is a JetStore-owned asset when it is directly in an asset group's
// directory and that directory's manifest records it; the hash is the
// manifest's. Nothing else is: a workspace never installed into, a file of the
// workspace's own beside the assets, the manifest itself, a nested file, or a
// directory that is not an asset group's.
func TestInstalledHashReadsTheManifest(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, "user_flows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, owned, err := InstalledHash(ws, "user_flows/a.uf.json"); err != nil || owned {
		t.Fatalf("with no manifest: owned %v, err %v", owned, err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName),
		[]byte(`{"assets": {"a.uf.json": "abc"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	for fileName, want := range map[string]string{
		"user_flows/a.uf.json":               "abc",
		"./user_flows/a.uf.json":             "abc",
		"user_flows\\a.uf.json":              "abc",
		"data_model/../user_flows/a.uf.json": "abc",
		"user_flows/mine.uf.json":            "",
		"user_flows/" + ManifestName:         "",
		"user_flows/sub/a.uf.json":           "",
		"jet_rules/a.uf.json":                "",
		"a.uf.json":                          "",
	} {
		hash, owned, err := InstalledHash(ws, fileName)
		if err != nil {
			t.Fatalf("%s: %v", fileName, err)
		}
		if hash != want || owned != (want != "") {
			t.Errorf("InstalledHash(%q) = %q, %v; want %q", fileName, hash, owned, want)
		}
	}
}
