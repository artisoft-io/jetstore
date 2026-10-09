package workspace_assets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The four groups are named here rather than indexed out of AssetGroups, so a
// test that stops covering a group fails to compile instead of silently
// testing nothing.
const (
	dataModel    = "data_model"
	pipesConfig  = "pipes_config"
	userFlows    = "user_flows"
	tableConfigs = "table_configs"
)

func newWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, g := range AssetGroups {
		if err := os.MkdirAll(filepath.Join(dir, g.Dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// actions keys on the installed path, since a name is only unique within its
// group.
func actions(t *testing.T, results []Result) map[string]Action {
	t.Helper()
	out := map[string]Action{}
	for _, r := range results {
		out[filepath.Join(r.Dir, r.Name)] = r.Action
	}
	return out
}

func read(t *testing.T, dir, group, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, group, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func allNames(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, g := range AssetGroups {
		names, err := Names(g.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) == 0 {
			t.Fatalf("no assets embedded for %s", g.Dir)
		}
		out[g.Dir] = names
	}
	return out
}

// A21.4: the guard's cheap case — "not installed by JetStore" — only works if
// every asset in a TokenedAssets group actually carries the token. The other
// groups are asserted not to be checked that way, which is the point of the
// flag: a .pc.json carries no header, so the manifest is the whole evidence.
func TestOwnershipHeaderMatchesEachGroupsClaim(t *testing.T) {
	for _, g := range AssetGroups {
		names, err := Names(g.Dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(names) == 0 {
			t.Fatalf("%s: no assets embedded", g.Dir)
		}
		for _, name := range names {
			data, err := Asset(g.Dir, name)
			if err != nil {
				t.Fatal(err)
			}
			if g.TokenedAssets && !strings.Contains(string(data), OwnershipToken) {
				t.Errorf("%s/%s carries no %s header, but the group claims every asset does",
					g.Dir, name, OwnershipToken)
			}
		}
		if _, err := Asset(g.Dir, "README.md"); err == nil {
			t.Errorf("%s/README.md is embedded; the asset set and the directory contents "+
				"are not the same thing", g.Dir)
		}
	}
}

// The plain path: an empty workspace, then a second run with nothing to do.
func TestInstallThenReinstallIsUnchanged(t *testing.T) {
	dir := newWorkspace(t)
	names := allNames(t)

	first, err := Install(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := actions(t, first)
	for group, groupNames := range names {
		for _, name := range groupNames {
			key := filepath.Join(group, name)
			if got[key] != Installed {
				t.Errorf("%s: first install reports %q, want %q", key, got[key], Installed)
			}
			want, _ := Asset(group, name)
			if string(read(t, dir, group, name)) != string(want) {
				t.Errorf("%s: installed bytes differ from the asset", key)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, group, ManifestName)); err != nil {
			t.Errorf("%s: no manifest written: %v", group, err)
		}
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
	if err := os.WriteFile(filepath.Join(dir, dataModel, "jets_model.jr"), stale, 0644); err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, dir, dataModel, "jets_model.jr", sum(stale))

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("a stale, unedited copy should be replaced, not refused: %v", err)
	}
	if got := actions(t, results)[filepath.Join(dataModel, "jets_model.jr")]; got != Updated {
		t.Errorf("jets_model.jr: %q, want %q", got, Updated)
	}
	want, _ := Asset(dataModel, "jets_model.jr")
	if string(read(t, dir, dataModel, "jets_model.jr")) != string(want) {
		t.Error("the stale copy was not replaced by the asset")
	}
}

// The same, in the group with no ownership token — where the manifest is not a
// tiebreaker but the only evidence there is.
func TestStalePipelineConfigIsUpdated(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	stale := []byte("{\"comment\": \"an older release\"}\n")
	if err := os.WriteFile(filepath.Join(dir, pipesConfig, "jets_loader.pc.json"), stale, 0644); err != nil {
		t.Fatal(err)
	}
	rewriteManifest(t, dir, pipesConfig, "jets_loader.pc.json", sum(stale))

	results, err := Install(dir, Options{})
	if err != nil {
		t.Fatalf("a stale, unedited pipeline config should be replaced, not refused: %v", err)
	}
	if got := actions(t, results)[filepath.Join(pipesConfig, "jets_loader.pc.json")]; got != Updated {
		t.Errorf("jets_loader.pc.json: %q, want %q", got, Updated)
	}
}

// A21.3, first clause: a JetStore file edited in place stops the build.
func TestLocallyModifiedAssetIsRefused(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	edited := append(read(t, dir, dataModel, "jets_model.jr"),
		[]byte("\nresource local:Thing = \"local:Thing\";\n")...)
	if err := os.WriteFile(filepath.Join(dir, dataModel, "jets_model.jr"), edited, 0644); err != nil {
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
	// One group conflicted, so only that group's remedy is printed.
	if strings.Contains(err.Error(), "process_config row") {
		t.Error("the pipes_config remedy is printed for a data_model conflict")
	}
	// The edit survives: a refused install writes nothing.
	if string(read(t, dir, dataModel, "jets_model.jr")) != string(edited) {
		t.Error("a refused install overwrote the file anyway")
	}
}

// The same in pipes_config, where the diagnosis must not claim the file is not
// JetStore's on the evidence of a header JetStore never writes there.
func TestModifiedPipelineConfigIsRefusedAndDiagnosedFromTheManifest(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	edited := []byte("{\"comment\": \"my own tweak\"}\n")
	if err := os.WriteFile(filepath.Join(dir, pipesConfig, "jets_loader.pc.json"), edited, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("editing an installed pipeline config was not refused: %v", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Dir != pipesConfig {
		t.Fatalf("conflicts: %+v", conflict.Conflicts)
	}
	if strings.Contains(conflict.Conflicts[0].Reason, OwnershipToken) {
		t.Errorf("a .pc.json was diagnosed on a header it never carries: %q",
			conflict.Conflicts[0].Reason)
	}
	if !strings.Contains(conflict.Conflicts[0].Reason, "modified since it was installed") {
		t.Errorf("the diagnosis does not say the file was edited: %q", conflict.Conflicts[0].Reason)
	}
	if !strings.Contains(err.Error(), "process_config row") {
		t.Error("the error does not show the client how to run a variant instead")
	}
}

// A21.3, second clause and the reason it exists: usi_ws's jets_model.jr, a
// client file at a path JetStore has since claimed. It carries no ownership
// header, so it is diagnosed as never-installed rather than as edited — and
// nothing else is installed either, in either directory, so the workspace is
// not left half-updated.
func TestUnownedFileAtAReservedPathIsRefusedWholesale(t *testing.T) {
	dir := newWorkspace(t)
	usi := []byte("# usi specific extension\nresource iWorkBook = \"iWorkBook\";\n")
	if err := os.WriteFile(filepath.Join(dir, dataModel, "jets_model.jr"), usi, 0644); err != nil {
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
	if _, err := os.Stat(filepath.Join(dir, dataModel, "jets_agentic.jr")); !os.IsNotExist(err) {
		t.Error("the other assets were installed despite the conflict; the guard is not all-or-nothing")
	}
	// The guard spans the groups, not just the one that conflicted.
	if _, err := os.Stat(filepath.Join(dir, pipesConfig, "jets_loader.pc.json")); !os.IsNotExist(err) {
		t.Error("pipes_config was installed despite a data_model conflict; " +
			"the guard does not span the groups")
	}
	if string(read(t, dir, dataModel, "jets_model.jr")) != string(usi) {
		t.Error("the client's file was overwritten")
	}
}

// The reverse direction of the same claim, and the one this change makes
// live: every workspace already carries its own jets_loader.pc.json, so a
// pipes_config conflict is the likely one — and it must not leave a workspace
// with a new data model and an old pipeline set.
func TestPipesConfigConflictRefusesDataModelToo(t *testing.T) {
	dir := newWorkspace(t)
	mine := []byte("{\"comment\": \"the workspace's own loader, predating the install\"}\n")
	if err := os.WriteFile(filepath.Join(dir, pipesConfig, "jets_loader.pc.json"), mine, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Install(dir, Options{})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("a divergent pipeline config was not refused: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, dataModel, "jets_model.jr")); !os.IsNotExist(err) {
		t.Error("data_model was installed despite a pipes_config conflict")
	}
	if string(read(t, dir, pipesConfig, "jets_loader.pc.json")) != string(mine) {
		t.Error("the workspace's own file was overwritten")
	}
}

// -force is the documented one-time adoption, and nothing else.
func TestForceAdoptsTheJetStoreVersion(t *testing.T) {
	dir := newWorkspace(t)
	usi := []byte("# usi specific extension\nresource iWorkBook = \"iWorkBook\";\n")
	if err := os.WriteFile(filepath.Join(dir, dataModel, "jets_model.jr"), usi, 0644); err != nil {
		t.Fatal(err)
	}
	mine := []byte("{\"comment\": \"the workspace's own loader\"}\n")
	if err := os.WriteFile(filepath.Join(dir, pipesConfig, "jets_loader.pc.json"), mine, 0644); err != nil {
		t.Fatal(err)
	}
	results, err := Install(dir, Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	got := actions(t, results)
	for _, key := range []string{
		filepath.Join(dataModel, "jets_model.jr"),
		filepath.Join(pipesConfig, "jets_loader.pc.json"),
	} {
		if got[key] != Updated {
			t.Errorf("%s: %q, want %q", key, got[key], Updated)
		}
	}
	want, _ := Asset(dataModel, "jets_model.jr")
	if string(read(t, dir, dataModel, "jets_model.jr")) != string(want) {
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
	for _, g := range AssetGroups {
		entries, err := os.ReadDir(filepath.Join(dir, g.Dir))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("-dry_run wrote %d file(s) into %s", len(entries), g.Dir)
		}
	}
}

// An unreadable manifest must not be guessed at: every difference becomes a
// conflict rather than an overwrite.
func TestUnreadableManifestFailsClosed(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	edited := append(read(t, dir, dataModel, "jets_model.jr"), []byte("\n# edited\n")...)
	if err := os.WriteFile(filepath.Join(dir, dataModel, "jets_model.jr"), edited, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, dataModel, ManifestName), []byte("{not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, Options{}); err == nil {
		t.Error("a corrupt manifest let a modified asset be overwritten")
	}
}

// Each group keeps its own manifest, which is what lets data_model's committed
// one stay valid across this change.
func TestEachGroupHasItsOwnManifest(t *testing.T) {
	dir := newWorkspace(t)
	if _, err := Install(dir, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, g := range AssetGroups {
		var m manifest
		data := read(t, dir, g.Dir, ManifestName)
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", g.Dir, err)
		}
		names, _ := Names(g.Dir)
		if len(m.Assets) != len(names) {
			t.Errorf("%s: manifest records %d asset(s), the group has %d",
				g.Dir, len(m.Assets), len(names))
		}
		for _, name := range names {
			if m.Assets[name] == "" {
				t.Errorf("%s: manifest has no entry for %s", g.Dir, name)
			}
		}
	}
}

func rewriteManifest(t *testing.T, dir, group, name, hash string) {
	t.Helper()
	path := filepath.Join(dir, group, ManifestName)
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

// The keys `cpipes-contract templates` writes, as distinct from the flows ported
// out of the Flutter app.
//
// **Named rather than derived, and the alternative was worse.** A projection is
// four documents and a ported flow is three, so the discriminator available on
// disk is the presence of the .apply.json — which is the very thing the test
// below is checking for. Deriving the set from it would make a projection that
// lost its .apply.json indistinguishable from a ported flow and the check would
// pass on the failure it exists to catch. So the keys are written down, adding a
// template means adding a line here, and the failure names it. This mirrors
// `jetsclient_ide/src/cpipes/projectedFlow.test.tsx`, whose TEMPLATES is the same
// list for the same reason.
var projectedKeys = map[string]bool{
	"map_claim_load_stages": true,
	"qc_metrics":            true,
	"qc_report":             true,
}

// A flow's documents are read together and a set that lost one fails at load in
// the browser — the layer this repository has least visibility into, so the glob
// is checked here instead (task U.2).
//
// **Two shapes since the eleven ported flows moved in.** FlowStore reads the
// .uf.json, .ua.json and .form.json of every flow; a projection has a fourth,
// the .apply.json, which no UserFlow schema describes and only the escape reads.
// So three documents is the rule and the fourth is a property of being generated
// from a template.
func TestEveryFlowInstallsAsAWholeSet(t *testing.T) {
	names, err := Names(userFlows)
	if err != nil {
		t.Fatal(err)
	}
	suffixes := []string{".uf.json", ".ua.json", ".form.json", ".apply.json"}
	keys := map[string]map[string]bool{}
	for _, name := range names {
		matched := false
		for _, suffix := range suffixes {
			if strings.HasSuffix(name, suffix) {
				key := strings.TrimSuffix(name, suffix)
				if keys[key] == nil {
					keys[key] = map[string]bool{}
				}
				keys[key][suffix] = true
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("%s/%s is installed and is none of the four document kinds", userFlows, name)
		}
	}
	if len(keys) == 0 {
		t.Fatalf("%s installs nothing; the embed glob matched no file", userFlows)
	}
	for key, got := range keys {
		for _, suffix := range []string{".uf.json", ".ua.json", ".form.json"} {
			if !got[suffix] {
				t.Errorf("%s: no %s — a flow key installs all three or none", key, suffix)
			}
		}
		switch {
		case projectedKeys[key] && !got[".apply.json"]:
			t.Errorf("%s: a projected flow with no .apply.json — the escape has nothing to apply", key)
		case !projectedKeys[key] && got[".apply.json"]:
			t.Errorf("%s: an .apply.json under a key that is not a projection", key)
		}
	}
	for key := range projectedKeys {
		if keys[key] == nil {
			t.Errorf("%s: named as a projection and installs nothing", key)
		}
	}
}

// Every table a JetStore flow draws installs, and nothing else does. A .tc.json
// is the only kind here: `FlowStore.loadTables` asks for `table_configs/<key>.tc.json`
// by the key a form's dataTable field names, so a file of any other suffix in
// this group is a file no flow can reach.
func TestOnlyTableDocumentsInstallAsTableConfigs(t *testing.T) {
	names, err := Names(tableConfigs)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatalf("%s installs nothing; the embed glob matched no file", tableConfigs)
	}
	for _, name := range names {
		if !strings.HasSuffix(name, ".tc.json") {
			t.Errorf("%s/%s: not a table configuration", tableConfigs, name)
		}
	}
}

// The generator's evidence is a .pc.json and must not reach a workspace's
// pipes_config by being installed as a user flow. It lives outside this package
// for that reason; this asserts the glob agrees.
func TestNoConfigIsInstalledAsAUserFlow(t *testing.T) {
	names, err := Names(userFlows)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if strings.HasSuffix(name, ".pc.json") {
			t.Errorf("%s/%s: a pipeline config is installed as a user flow", userFlows, name)
		}
	}
}
