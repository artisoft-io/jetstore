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

// The files the next install writes over are the names a group's manifest
// records, directly in that group's directory, and every group's manifest,
// recorded or not (JM-389). A file of the workspace's own beside the assets,
// a nested file, and a workspace never installed into are not among them.
func TestInstalledFilesAreTheManifestsNames(t *testing.T) {
	ws := t.TempDir()
	files, err := InstalledFiles(os.DirFS(ws))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(AssetGroups) {
		t.Fatalf("with no manifest: %d files, want the %d manifests", len(files), len(AssetGroups))
	}
	for _, g := range AssetGroups {
		if f, ok := files[g.Dir+"/"+ManifestName]; !ok || !f.Manifest || f.Group.Dir != g.Dir {
			t.Errorf("%s's manifest: %+v, %v", g.Dir, f, ok)
		}
	}

	dir := filepath.Join(ws, "user_flows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(`{"assets": {
		"a.uf.json": "abc", "empty.uf.json": "", "sub/b.uf.json": "def", "..": "ghi"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if files, err = InstalledFiles(os.DirFS(ws)); err != nil {
		t.Fatal(err)
	}
	if f, ok := files["user_flows/a.uf.json"]; !ok || f.Manifest || f.Group.Dir != "user_flows" {
		t.Errorf("user_flows/a.uf.json: %+v, %v", f, ok)
	}
	for _, name := range []string{"user_flows/empty.uf.json", "user_flows/sub/b.uf.json",
		"user_flows/..", "user_flows/mine.uf.json", "table_configs/a.uf.json"} {
		if _, ok := files[name]; ok {
			t.Errorf("%s is listed as installed", name)
		}
	}
	if want := len(AssetGroups) + 1; len(files) != want {
		t.Errorf("%d files, want %d: %v", len(files), want, files)
	}

	// An unreadable manifest is treated as absent, as Install treats it.
	if err := os.WriteFile(filepath.Join(dir, ManifestName), []byte(`not json`), 0644); err != nil {
		t.Fatal(err)
	}
	if files, err = InstalledFiles(os.DirFS(ws)); err != nil || len(files) != len(AssetGroups) {
		t.Errorf("with an unreadable manifest: %d files, err %v", len(files), err)
	}
}

// After an install, what InstalledFiles returns is what the install wrote:
// every embedded asset of every group, and each manifest.
func TestInstalledFilesAfterAnInstall(t *testing.T) {
	ws := t.TempDir()
	if _, err := Install(ws, Options{}); err != nil {
		t.Fatal(err)
	}
	files, err := InstalledFiles(os.DirFS(ws))
	if err != nil {
		t.Fatal(err)
	}
	want := len(AssetGroups)
	for _, g := range AssetGroups {
		names, err := Names(g.Dir)
		if err != nil {
			t.Fatal(err)
		}
		want += len(names)
		for _, name := range names {
			if _, ok := files[g.Dir+"/"+name]; !ok {
				t.Errorf("%s/%s was installed and is not listed", g.Dir, name)
			}
		}
	}
	if len(files) != want {
		t.Errorf("%d files, want %d", len(files), want)
	}
}
