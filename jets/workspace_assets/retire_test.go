package workspace_assets

// Retirement, jetstore_maintenance_02 task AD.5 (2026-10-01): an install deletes
// a retired asset's unedited copy, refuses an edited one, and ignores an absent
// one. The three cases are the plan's; the rest pin the edges the evidence for
// "unedited" has, which is the manifest and nothing else.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The retired asset the cases below use: the first one listed, whatever it is.
func aRetiredAsset(t *testing.T) RetiredAsset {
	t.Helper()
	if len(RetiredAssets) == 0 {
		t.Skip("nothing is retired")
	}
	return RetiredAssets[0]
}

// workspaceWithRetiredCopy is a workspace as an install before the retirement
// left it: everything current installed, plus the retired file on disk and the
// hash `recorded` in its group's manifest. recorded == "" leaves no entry.
func workspaceWithRetiredCopy(t *testing.T, r RetiredAsset, content []byte, recorded string) string {
	t.Helper()
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, r.Dir, r.Name), content, 0644); err != nil {
		t.Fatal(err)
	}
	if recorded != "" {
		rewriteManifest(t, dir, r.Dir, r.Name, recorded)
	}
	return dir
}

func manifestEntries(t *testing.T, dir, group string) map[string]string {
	t.Helper()
	var m manifest
	if err := json.Unmarshal(read(t, dir, group, ManifestName), &m); err != nil {
		t.Fatal(err)
	}
	return m.Assets
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}

var retiredContent = []byte(`{"schemaVersion": 1, "title": "as a release before the retirement shipped it"}` + "\n")

// Plan case 1: the copy is what the last install left, so it goes, and so does
// its manifest entry.
func TestUneditedRetiredAssetIsRemoved(t *testing.T) {
	r := aRetiredAsset(t)
	dir := workspaceWithRetiredCopy(t, r, retiredContent, sum(retiredContent))

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("an unedited retired copy should be removed, not refused: %v", err)
	}
	if got := actions(t, results)[filepath.Join(r.Dir, r.Name)]; got != Removed {
		t.Errorf("%s: %q, want %q", r.Name, got, Removed)
	}
	if exists(t, filepath.Join(dir, r.Dir, r.Name)) {
		t.Errorf("%s is still installed", r.Name)
	}
	if _, ok := manifestEntries(t, dir, r.Dir)[r.Name]; ok {
		t.Errorf("%s is still in %s's manifest", r.Name, r.Dir)
	}
	// And the next install has nothing to say about it.
	again, err := Install(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := actions(t, again)[filepath.Join(r.Dir, r.Name)]; ok {
		t.Errorf("a second install reported %s as %q", r.Name, got)
	}
}

// Every retired asset, not only the first: a flow is three documents and a set
// that lost two of them is a flow that fails to load rather than one that is gone.
func TestEveryUneditedRetiredAssetIsRemoved(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, r := range RetiredAssets {
		if err := os.WriteFile(filepath.Join(dir, r.Dir, r.Name), retiredContent, 0644); err != nil {
			t.Fatal(err)
		}
		rewriteManifest(t, dir, r.Dir, r.Name, sum(retiredContent))
	}
	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := actions(t, results)
	for _, r := range RetiredAssets {
		if got[filepath.Join(r.Dir, r.Name)] != Removed || exists(t, filepath.Join(dir, r.Dir, r.Name)) {
			t.Errorf("%s: %q, on disk %v; want removed and gone",
				filepath.Join(r.Dir, r.Name), got[filepath.Join(r.Dir, r.Name)],
				exists(t, filepath.Join(dir, r.Dir, r.Name)))
		}
	}
}

// Plan case 2: an edited copy is a conflict, as for an update, and a refused
// install writes nothing — the file and its manifest entry both survive.
func TestEditedRetiredAssetIsRefused(t *testing.T) {
	r := aRetiredAsset(t)
	edited := append(append([]byte{}, retiredContent...), []byte("{\"local\": true}\n")...)
	dir := workspaceWithRetiredCopy(t, r, edited, sum(retiredContent))

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("an edited retired copy was not refused: %v", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Name != r.Name {
		t.Fatalf("conflicts: %+v", conflict.Conflicts)
	}
	if !strings.Contains(conflict.Conflicts[0].Reason, "retired") ||
		!strings.Contains(conflict.Conflicts[0].Reason, "modified since it was installed") {
		t.Errorf("the diagnosis does not say a retired file was edited: %q", conflict.Conflicts[0].Reason)
	}
	if string(read(t, dir, r.Dir, r.Name)) != string(edited) {
		t.Error("a refused install deleted or changed the edited file")
	}
	if manifestEntries(t, dir, r.Dir)[r.Name] != sum(retiredContent) {
		t.Error("a refused install rewrote the manifest")
	}
}

// Plan case 3: nothing to delete is nothing to report, and the stale entry a
// manifest may still carry is dropped.
func TestAbsentRetiredAssetIsIgnored(t *testing.T) {
	r := aRetiredAsset(t)
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, dir, r.Dir, r.Name, sum(retiredContent))

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("an absent retired asset should be ignored: %v", err)
	}
	if got, ok := actions(t, results)[filepath.Join(r.Dir, r.Name)]; ok {
		t.Errorf("an absent retired asset was reported as %q", got)
	}
	if _, ok := manifestEntries(t, dir, r.Dir)[r.Name]; ok {
		t.Errorf("%s's stale manifest entry survived the install", r.Name)
	}
}

// With no manifest entry the install cannot tell JetStore's copy from somebody's
// file that happens to have the name, so it refuses rather than deletes — the
// same call it makes for an update with no entry.
func TestRetiredAssetWithNoManifestEntryIsRefused(t *testing.T) {
	r := aRetiredAsset(t)
	dir := workspaceWithRetiredCopy(t, r, retiredContent, "")

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 ||
		!strings.Contains(conflict.Conflicts[0].Reason, "no "+ManifestName+" entry") {
		t.Fatalf("want one conflict naming the missing manifest entry, got %v", err)
	}
	if !exists(t, filepath.Join(dir, r.Dir, r.Name)) {
		t.Error("a refused install deleted the file")
	}
}

// -force adopts the JetStore version, and for a retired path that is absence.
func TestForceRemovesAnEditedRetiredAsset(t *testing.T) {
	r := aRetiredAsset(t)
	dir := workspaceWithRetiredCopy(t, r, []byte("edited\n"), sum(retiredContent))

	results, err := Install(dir, Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := actions(t, results)[filepath.Join(r.Dir, r.Name)]; got != Removed {
		t.Errorf("%s: %q, want %q", r.Name, got, Removed)
	}
	if exists(t, filepath.Join(dir, r.Dir, r.Name)) {
		t.Errorf("%s survived -force", r.Name)
	}
}

func TestDryRunReportsARemovalAndDeletesNothing(t *testing.T) {
	r := aRetiredAsset(t)
	dir := workspaceWithRetiredCopy(t, r, retiredContent, sum(retiredContent))

	results, err := Install(dir, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := actions(t, results)[filepath.Join(r.Dir, r.Name)]; got != Removed {
		t.Errorf("%s: %q, want %q reported", r.Name, got, Removed)
	}
	if !exists(t, filepath.Join(dir, r.Dir, r.Name)) {
		t.Error("a dry run deleted the file")
	}
	if manifestEntries(t, dir, r.Dir)[r.Name] != sum(retiredContent) {
		t.Error("a dry run rewrote the manifest")
	}
}

// The list's own invariants: every entry names a real group, and none names an
// asset that still ships — which Install also refuses at run time, because it
// would install the file and delete it in one pass.
func TestRetiredAssetsAreNotShipped(t *testing.T) {
	groups := map[string]bool{}
	for _, g := range AssetGroups {
		groups[g.Dir] = true
	}
	seen := map[string]bool{}
	for _, r := range RetiredAssets {
		p := filepath.Join(r.Dir, r.Name)
		if !groups[r.Dir] {
			t.Errorf("%s: %s is not an asset group", p, r.Dir)
		}
		if seen[p] {
			t.Errorf("%s is listed twice", p)
		}
		seen[p] = true
		if _, err := Asset(r.Dir, r.Name); err == nil {
			t.Errorf("%s is retired and still embedded", p)
		}
		if r.Why == "" {
			t.Errorf("%s carries no reason", p)
		}
	}
}

// The three documents of registerFileKeyUF, by name, so that the retirement this
// mechanism was built for cannot be dropped from the list without a failure that
// says which flow came back.
func TestRegisterFileKeyUFIsRetiredAsAWholeSet(t *testing.T) {
	want := map[string]bool{
		"user_flows/registerFileKeyUF.uf.json":   false,
		"user_flows/registerFileKeyUF.ua.json":   false,
		"user_flows/registerFileKeyUF.form.json": false,
	}
	for _, r := range RetiredAssets {
		if _, ok := want[r.Dir+"/"+r.Name]; ok {
			want[r.Dir+"/"+r.Name] = true
		}
	}
	for p, found := range want {
		if !found {
			t.Errorf("%s is not in RetiredAssets", p)
		}
	}
}
