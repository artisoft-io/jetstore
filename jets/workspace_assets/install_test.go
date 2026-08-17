package workspace_assets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, AssetDir), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func actions(t *testing.T, results []Result) map[string]Action {
	t.Helper()
	out := map[string]Action{}
	for _, r := range results {
		out[r.Name] = r.Action
	}
	return out
}

func read(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, AssetDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// A21.4: the guard's cheap case — "not installed by JetStore" — only works if
// every asset actually carries the token.
func TestEveryAssetCarriesTheOwnershipHeader(t *testing.T) {
	names, err := Names()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no assets embedded")
	}
	for _, name := range names {
		data, err := Asset(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), OwnershipToken) {
			t.Errorf("%s carries no %s header", name, OwnershipToken)
		}
	}
	if _, err := Asset("README.md"); err == nil {
		t.Error("README.md is embedded; the asset set and the directory contents are not the same thing")
	}
}

// The plain path: an empty workspace, then a second run with nothing to do.
func TestInstallThenReinstallIsUnchanged(t *testing.T) {
	dir := newWorkspace(t)
	names, _ := Names()

	first, err := Install(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := actions(t, first)
	for _, name := range names {
		if got[name] != Installed {
			t.Errorf("%s: first install reports %q, want %q", name, got[name], Installed)
		}
		want, _ := Asset(name)
		if string(read(t, dir, name)) != string(want) {
			t.Errorf("%s: installed bytes differ from the asset", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, AssetDir, ManifestName)); err != nil {
		t.Errorf("no manifest written: %v", err)
	}

	second, err := Install(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for name, action := range actions(t, second) {
		if action != Unchanged {
			t.Errorf("%s: second install reports %q, want %q", name, action, Unchanged)
		}
	}
}

// A stale copy — older, but exactly what the last install left — is replaced.
// This is how a JetStore model change reaches a client workspace, and it must
// not be confused with the conflict case.
func TestStaleButUneditedIsUpdated(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	stale := []byte("# " + OwnershipToken + ": data_model/jets_model.jr — an older release\n")
	if err := os.WriteFile(filepath.Join(dir, AssetDir, "jets_model.jr"), stale, 0644); err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, dir, "jets_model.jr", sum(stale))

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("a stale, unedited copy should be replaced, not refused: %v", err)
	}
	if got := actions(t, results)["jets_model.jr"]; got != Updated {
		t.Errorf("jets_model.jr: %q, want %q", got, Updated)
	}
	want, _ := Asset("jets_model.jr")
	if string(read(t, dir, "jets_model.jr")) != string(want) {
		t.Error("the stale copy was not replaced by the asset")
	}
}

// A21.3, first clause: a JetStore file edited in place stops the build.
func TestLocallyModifiedAssetIsRefused(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	edited := append(read(t, dir, "jets_model.jr"), []byte("\nresource local:Thing = \"local:Thing\";\n")...)
	if err := os.WriteFile(filepath.Join(dir, AssetDir, "jets_model.jr"), edited, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("editing an installed asset was not refused: %v", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Name != "jets_model.jr" {
		t.Fatalf("conflicts: %+v", conflict.Conflicts)
	}
	if !strings.Contains(conflict.Conflicts[0].Reason, "modified since it was installed") {
		t.Errorf("the diagnosis does not say the file was edited: %q", conflict.Conflicts[0].Reason)
	}
	if !strings.Contains(err.Error(), "import \"data_model/jets_model.jr\"") {
		t.Error("the error does not show the client how to extend the model instead")
	}
	// The edit survives: a refused install writes nothing.
	if string(read(t, dir, "jets_model.jr")) != string(edited) {
		t.Error("a refused install overwrote the file anyway")
	}
}

// A21.3, second clause and the reason it exists: usi_ws's jets_model.jr, a
// client file at a path JetStore has since claimed. It carries no ownership
// header, so it is diagnosed as never-installed rather than as edited — and
// nothing else is installed either, so the workspace is not left half-updated.
func TestUnownedFileAtAReservedPathIsRefusedWholesale(t *testing.T) {
	dir := newWorkspace(t)
	usi := []byte("# usi specific extension\nresource iWorkBook = \"iWorkBook\";\n")
	if err := os.WriteFile(filepath.Join(dir, AssetDir, "jets_model.jr"), usi, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("an unowned file at a reserved path was not refused: %v", err)
	}
	if !strings.Contains(conflict.Conflicts[0].Reason, "carries no "+OwnershipToken) {
		t.Errorf("the diagnosis does not distinguish never-installed: %q", conflict.Conflicts[0].Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, AssetDir, "jets_agentic.jr")); !os.IsNotExist(err) {
		t.Error("the other assets were installed despite the conflict; the guard is not all-or-nothing")
	}
	if string(read(t, dir, "jets_model.jr")) != string(usi) {
		t.Error("the client's file was overwritten")
	}
}

// -force is the documented one-time adoption, and nothing else.
func TestForceAdoptsTheJetStoreVersion(t *testing.T) {
	dir := newWorkspace(t)
	usi := []byte("# usi specific extension\nresource iWorkBook = \"iWorkBook\";\n")
	if err := os.WriteFile(filepath.Join(dir, AssetDir, "jets_model.jr"), usi, 0644); err != nil {
		t.Fatal(err)
	}
	results, err := Install(dir, Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := actions(t, results)["jets_model.jr"]; got != Updated {
		t.Errorf("jets_model.jr: %q, want %q", got, Updated)
	}
	want, _ := Asset("jets_model.jr")
	if string(read(t, dir, "jets_model.jr")) != string(want) {
		t.Error("-force did not adopt the JetStore version")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dir := newWorkspace(t)
	results, err := Install(dir, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	for name, action := range actions(t, results) {
		if action != Installed {
			t.Errorf("%s: %q, want %q", name, action, Installed)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, AssetDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("-dry_run wrote %d file(s)", len(entries))
	}
}

// An unreadable manifest must not be guessed at: every difference becomes a
// conflict rather than an overwrite.
func TestUnreadableManifestFailsClosed(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	edited := append(read(t, dir, "jets_model.jr"), []byte("\n# edited\n")...)
	if err := os.WriteFile(filepath.Join(dir, AssetDir, "jets_model.jr"), edited, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, AssetDir, ManifestName), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, Options{}); err == nil {
		t.Error("a corrupt manifest let a modified asset be overwritten")
	}
}

func rewriteManifest(t *testing.T, dir, name, hash string) {
	t.Helper()
	path := filepath.Join(dir, AssetDir, ManifestName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	m.Assets[name] = hash
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		t.Fatal(err)
	}
}
