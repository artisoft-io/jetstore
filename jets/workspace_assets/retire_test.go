package workspace_assets

// Retirement (2026-10-01): an install deletes
// a retired asset's unedited copy, refuses an edited one, and ignores an absent
// one. The three cases are the design's; the rest pin the edges the evidence for
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

// The projected wizard retired in release 4.1.25, all four documents by name:
// a projection is read as a set, and a set missing its .apply.json loads and
// then fails at its last step.
func TestTheRetiredProjectionIsRetiredAsAWholeSet(t *testing.T) {
	want := map[string]bool{}
	for _, suffix := range []string{".uf.json", ".ua.json", ".form.json", ".apply.json"} {
		want["user_flows/map_claim_load_stages"+suffix] = false
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

// A workspace that keeps a retired flow keeps it under a key of its own, and
// this is the case that says why. The copy JetStore installed under the old key
// is removed; the workspace's copy under its own key, byte for byte the same,
// is not JetStore's path and is left alone, with no conflict and no manifest
// entry. A copy kept under the retired key is the case
// TestRetiredAssetWithNoManifestEntryIsRefused pins: refused, and the build
// stops, because a workspace clone carries no manifest.
func TestARetiredFlowKeptUnderAWorkspaceKeySurvivesTheInstall(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	const ownKey = "my_workspace_map_claim_load_stages"
	var kept []string
	for _, r := range RetiredAssets {
		if r.Dir != "user_flows" || !strings.HasPrefix(r.Name, "map_claim_load_stages.") {
			continue
		}
		content := []byte(`{"schemaVersion": 1, "document": "` + r.Name + `"}` + "\n")
		if err := os.WriteFile(filepath.Join(dir, r.Dir, r.Name), content, 0644); err != nil {
			t.Fatal(err)
		}
		rewriteManifest(t, dir, r.Dir, r.Name, sum(content))
		own := ownKey + strings.TrimPrefix(r.Name, "map_claim_load_stages")
		if err := os.WriteFile(filepath.Join(dir, r.Dir, own), content, 0644); err != nil {
			t.Fatal(err)
		}
		kept = append(kept, own)
	}
	if len(kept) != 4 {
		t.Fatalf("found %d retired documents of the projection, want 4", len(kept))
	}

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("the workspace's own copy conflicted with the retirement: %v", err)
	}
	got := actions(t, results)
	for _, own := range kept {
		old := "map_claim_load_stages" + strings.TrimPrefix(own, ownKey)
		if got[filepath.Join("user_flows", old)] != Removed || exists(t, filepath.Join(dir, "user_flows", old)) {
			t.Errorf("%s: %q; want the installed copy removed", old, got[filepath.Join("user_flows", old)])
		}
		if a, ok := got[filepath.Join("user_flows", own)]; ok {
			t.Errorf("%s is the workspace's and was reported as %q", own, a)
		}
		if !exists(t, filepath.Join(dir, "user_flows", own)) {
			t.Errorf("%s, the workspace's copy, is gone", own)
		}
		if _, ok := manifestEntries(t, dir, "user_flows")[own]; ok {
			t.Errorf("%s entered JetStore's manifest", own)
		}
	}
}
