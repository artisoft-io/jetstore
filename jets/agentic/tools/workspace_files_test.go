// T.1 and T.2: the two workspace reads, and the path-valued alternative to the
// two verifiers' content arguments.
//
// The fixture is the same compiled workspace the rest of this package uses, so
// these exercise a real checkout's layout rather than a directory arranged to
// suit them — which matters most for the path arm of compile_rule_file, whose
// whole claim is that a file compiles from its own relative path with its own
// imports.
package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListWorkspaceFiles(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	var all FileListing
	if err := json.Unmarshal(call(t, reg, ws, "list_workspace_files", "{}"), &all); err != nil {
		t.Fatal(err)
	}
	if all.Count == 0 {
		t.Fatal("list_workspace_files reports an empty workspace")
	}
	for _, f := range all.Files {
		if filepath.IsAbs(f.Path) {
			t.Errorf("listing reports an absolute path %q; paths are workspace-relative", f.Path)
		}
		// build/ holds compiled output and .git is version-control metadata.
		// Neither is workspace content, and build/classes.json in particular
		// is already reachable through list_domain_classes.
		if strings.HasPrefix(f.Path, "build/") || strings.HasPrefix(f.Path, ".git/") {
			t.Errorf("listing reports %q, which is not authored workspace content", f.Path)
		}
	}

	// `**/*.jr` is the pattern a caller writes for "every rule file", and the
	// reason compileGlob exists rather than path.Match: path.Match's `*` stops
	// at a separator, so this would have matched the top level only.
	var jr FileListing
	if err := json.Unmarshal(call(t, reg, ws, "list_workspace_files",
		`{"pattern":"**/*.jr"}`), &jr); err != nil {
		t.Fatal(err)
	}
	if jr.Count == 0 {
		t.Fatal(`pattern "**/*.jr" matched nothing in a workspace that compiles rules`)
	}
	nested := false
	for _, f := range jr.Files {
		if !strings.HasSuffix(f.Path, ".jr") {
			t.Errorf(`pattern "**/*.jr" matched %q`, f.Path)
		}
		if strings.Contains(f.Path, "/") {
			nested = true
		}
	}
	if !nested {
		t.Error(`pattern "**/*.jr" matched only top-level files; ** did not cross a separator`)
	}
	if jr.Count >= all.Count {
		t.Errorf("the .jr filter (%d) did not narrow the unfiltered listing (%d)", jr.Count, all.Count)
	}

	// A single `*` must not cross a separator, or the two forms mean the same
	// thing and the distinction the description advertises is a fiction.
	var top FileListing
	if err := json.Unmarshal(call(t, reg, ws, "list_workspace_files",
		`{"pattern":"*.jr"}`), &top); err != nil {
		t.Fatal(err)
	}
	for _, f := range top.Files {
		if strings.Contains(f.Path, "/") {
			t.Errorf(`pattern "*.jr" matched %q across a separator`, f.Path)
		}
	}

	// Truncation is reported, because a caller cannot otherwise tell a
	// workspace with exactly max_results matches from one that was cut.
	var one FileListing
	if err := json.Unmarshal(call(t, reg, ws, "list_workspace_files",
		`{"max_results":1}`), &one); err != nil {
		t.Fatal(err)
	}
	if one.Count != 1 || !one.Truncated {
		t.Errorf("max_results 1 gave count %d truncated %v", one.Count, one.Truncated)
	}
}

func TestReadWorkspaceFile(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	var listing FileListing
	if err := json.Unmarshal(call(t, reg, ws, "list_workspace_files",
		`{"pattern":"data_model/jets_agentic.jr"}`), &listing); err != nil {
		t.Fatal(err)
	}
	if listing.Count != 1 {
		t.Fatalf("expected the installed agentic model in the fixture, listing gave %+v", listing)
	}

	// The listed path is the read path — that is the whole contract between
	// the two tools, and a caller that has to transform one into the other has
	// been given a discovery mechanism that does not answer its own question.
	var got FileContent
	args, _ := json.Marshal(map[string]string{"path": listing.Files[0].Path})
	if err := json.Unmarshal(call(t, reg, ws, "read_workspace_file", string(args)), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Content, "jetsa:Incident") {
		t.Errorf("read of %s did not return the agentic model", got.Path)
	}
	if got.Truncated {
		t.Errorf("read of %s reported truncation at the default limit", got.Path)
	}
	if got.TotalBytes != listing.Files[0].Bytes {
		t.Errorf("read reports %d bytes, the listing reported %d", got.TotalBytes, listing.Files[0].Bytes)
	}

	// Truncation again, and it is the same argument: the reply says so.
	args, _ = json.Marshal(map[string]any{"path": listing.Files[0].Path, "max_bytes": 64})
	if err := json.Unmarshal(call(t, reg, ws, "read_workspace_file", string(args)), &got); err != nil {
		t.Fatal(err)
	}
	if got.Bytes != 64 || !got.Truncated {
		t.Errorf("max_bytes 64 gave %d bytes truncated %v", got.Bytes, got.Truncated)
	}
}

// A path argument that could name anything on the host is A§5.2's open-proxy
// hazard, and I-69's deferral rests on the argument that it cannot. That is a
// claim about this function, so it is tested rather than asserted.
func TestWorkspacePathConfinement(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"/etc/passwd",
		"../../../etc/passwd",
		"data_model/../../outside.jr",
		"~/secrets.json",
		".",
	} {
		args, _ := json.Marshal(map[string]string{"path": bad})
		if _, err := reg.Call(t.Context(), ws, "read_workspace_file", json.RawMessage(args)); err == nil {
			t.Errorf("read_workspace_file accepted %q", bad)
		}
	}
	// A workspace file that is not authored text — the compiled SQLite
	// database is the one every workspace has — is refused by suffix rather
	// than read as bytes.
	if _, err := reg.Call(t.Context(), ws, "read_workspace_file",
		json.RawMessage(`{"path":"workspace.db"}`)); err == nil {
		t.Error("read_workspace_file returned workspace.db as text")
	}
}

// T.2: the two verifiers accept a path, refuse both arguments at once, and
// refuse neither.
func TestVerifiersTakeAPath(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	root, err := ws.LocalDir()
	if err != nil {
		t.Fatal(err)
	}

	// compile_rule_file by path. The file is one the workspace already
	// compiles, so a diagnostic here would be a defect in the arm rather than
	// in the rule.
	rel := "data_model/jets_agentic.jr"
	if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
		t.Fatalf("fixture lacks %s: %v", rel, err)
	}
	var report CompileReport
	if err := json.Unmarshal(call(t, reg, ws, "compile_rule_file",
		`{"rule_path":"`+rel+`"}`), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Valid {
		t.Errorf("compile_rule_file by path rejected a file the workspace compiles: %+v", report.Diagnostics)
	}

	// The arm's actual claim: a file whose imports are relative to its own
	// directory compiles by path and would not compile as text under a bare
	// name, because copyRuleSources stages it where its imports resolve.
	// jets_agentic.jr imports jets_model.jr from data_model/.
	src, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "import") {
		t.Skip("the fixture's agentic model carries no import; the arm's claim cannot be shown here")
	}

	// Exactly one of the two. Both is two intentions and neither is none.
	for _, args := range []string{
		`{"rule_path":"` + rel + `","rule_text":"[r]: (?x jets:key ?k) -> (?x hc:f true);"}`,
		`{}`,
		`{"rule_path":"` + rel + `","file_name":"other.jr"}`,
		`{"rule_path":"data_model/jets_agentic.meta.json"}`,
		`{"rule_path":"data_model/no_such_rule.jr"}`,
	} {
		if _, err := reg.Call(t.Context(), ws, "compile_rule_file", json.RawMessage(args)); err == nil {
			t.Errorf("compile_rule_file accepted %s", args)
		}
	}

	// validate_cpipes_config by path, against a config this test writes into
	// the fixture. test_ws's own pipes_config files are stale against the
	// current contract (the note on TestRegistryDrivesAllThreeTools says so),
	// so reading one of those would measure the corpus rather than the arm.
	cfgRel := "pipes_config/t2_path_arm.pc.json"
	cfg := `{
	  "channels": [{"name": "out_spec", "columns": ["a"]}],
	  "reducing_pipes_config": [[
	    {"type": "fan_out",
	     "input_channel": {"type": "input", "name": "input_row"},
	     "apply": [
	       {"type": "map_record", "new_record": true,
	        "columns": [{"name": "a", "type": "select", "expr": "a"}],
	        "output_channel": {"type": "memory", "name": "out1", "channel_spec_name": "out_spec"}}
	     ]}
	  ]]
	}`
	cfgAbs := filepath.Join(root, filepath.FromSlash(cfgRel))
	if err := os.MkdirAll(filepath.Dir(cfgAbs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgAbs, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(cfgAbs) })

	var vr ValidationReport
	if err := json.Unmarshal(call(t, reg, ws, "validate_cpipes_config",
		`{"config_path":"`+cfgRel+`"}`), &vr); err != nil {
		t.Fatal(err)
	}
	if !vr.Valid || vr.StepsValidated != 1 {
		t.Errorf("validate_cpipes_config by path rejected a config it accepts by value: %+v", vr)
	}

	// The two arms agree on the same content, which is the property the remedy
	// rests on: a path is a way of not carrying the payload, not a second
	// validator.
	var byValue ValidationReport
	if err := json.Unmarshal(call(t, reg, ws, "validate_cpipes_config",
		`{"config":`+cfg+`}`), &byValue); err != nil {
		t.Fatal(err)
	}
	if byValue.Valid != vr.Valid || byValue.StepsValidated != vr.StepsValidated {
		t.Errorf("the two arms disagree: by value %+v, by path %+v", byValue, vr)
	}

	for _, args := range []string{
		`{"config_path":"` + cfgRel + `","config":{"channels":[]}}`,
		`{}`,
		`{"config_path":"data_model/jets_agentic.jr"}`,
		`{"config_path":"pipes_config/no_such_config.json"}`,
	} {
		if _, err := reg.Call(t.Context(), ws, "validate_cpipes_config", json.RawMessage(args)); err == nil {
			t.Errorf("validate_cpipes_config accepted %s", args)
		}
	}
}

func TestCompileGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.jr", "main.jr", true},
		{"**/*.jr", "jet_rules/main.jr", true},
		{"**/*.jr", "a/b/c/main.jr", true},
		{"**/*.jr", "jet_rules/main.json", false},
		{"*.jr", "main.jr", true},
		{"*.jr", "jet_rules/main.jr", false},
		{"pipes_config/*.pc.json", "pipes_config/loader.pc.json", true},
		{"pipes_config/*.pc.json", "pipes_config/nested/loader.pc.json", false},
		{"data_model/jets_?gentic.jr", "data_model/jets_agentic.jr", true},
		{"**", "anything/at/all.csv", true},
		// A literal dot is a dot, not "any character" — the failure a naive
		// translation to a regexp makes, and the one that would quietly widen
		// every pattern in this table.
		{"main.jr", "mainXjr", false},
	}
	for _, c := range cases {
		re, err := compileGlob(c.pattern)
		if err != nil {
			t.Fatalf("compileGlob(%q): %v", c.pattern, err)
		}
		if got := re.MatchString(c.path); got != c.want {
			t.Errorf("compileGlob(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
	if _, err := compileGlob(`jet_rules\main.jr`); err == nil {
		t.Error("compileGlob accepted a backslash-separated pattern")
	}
}
