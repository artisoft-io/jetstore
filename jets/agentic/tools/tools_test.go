// Criterion 12, second clause: the same three tools reachable from a Go
// test with no adapter — which is what proves the registry, not MCP, is the
// interface (Q-4). The fixture is a real compiled workspace: test_ws plus
// the installed jets_agentic assets, compiled offline by compile_workspace
// (no database — Postgres is only for the upload), so build/classes.json is
// the genuine F5 view, not a hand-made stand-in.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	fixtureOnce sync.Once
	fixtureDir  string
	fixtureErr  error
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

// fixture builds (once) a compiled workspace: test_ws + the jets_agentic
// assets + a main rule file importing both models, compiled with
// compile_workspace under a scratch WORKSPACES_HOME.
func fixture(t *testing.T) *Workspace {
	t.Helper()
	root := repoRoot(t)
	fixtureOnce.Do(func() {
		home, err := os.MkdirTemp("", "jets_tools_ws_")
		if err != nil {
			fixtureErr = err
			return
		}
		wsDir := filepath.Join(home, "test_ws")
		if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
			fixtureErr = fmt.Errorf("copying test_ws: %v\n%s", err, out)
			return
		}
		for _, name := range []string{"jets_agentic.jr", "jets_agentic.meta.json"} {
			data, err := os.ReadFile(filepath.Join(root, "jets", "workspace_assets", "data_model", name))
			if err != nil {
				fixtureErr = err
				return
			}
			if err := os.WriteFile(filepath.Join(wsDir, "data_model", name), data, 0644); err != nil {
				fixtureErr = err
				return
			}
		}
		mainJr := "# Fixture main file joining the platform base model and the agentic model.\n" +
			"import \"data_model/jets_model.jr\"\nimport \"data_model/jets_agentic.jr\"\n"
		if err := os.WriteFile(filepath.Join(wsDir, "jet_rules", "jets_agentic_main.jr"), []byte(mainJr), 0644); err != nil {
			fixtureErr = err
			return
		}
		ctlPath := filepath.Join(wsDir, "workspace_control.json")
		raw, err := os.ReadFile(ctlPath)
		if err != nil {
			fixtureErr = err
			return
		}
		var ctl map[string]any
		if err := json.Unmarshal(raw, &ctl); err != nil {
			fixtureErr = err
			return
		}
		ctl["workspace_name"] = "test_ws" // compileWorkspaceV2 builds every path from it
		ctl["rule_sets"] = append(anySlice(ctl["rule_sets"]), "jet_rules/jets_agentic_main.jr")
		raw, _ = json.MarshalIndent(ctl, "", "  ")
		if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
			fixtureErr = err
			return
		}
		cmd := exec.Command("go", "run", "./jets/cmds/compile_workspace", "-w", "test_ws", "-v", "fixture")
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"WORKSPACES_HOME="+home,
			"WORKSPACE=test_ws",
			"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			fixtureErr = fmt.Errorf("compile_workspace failed: %v\n%s", err, out)
			return
		}
		fixtureDir = wsDir
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	ws, err := ResolveWorkspace(fixtureDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	return ws
}

func anySlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}

func call(t *testing.T, reg *Registry, ws *Workspace, name string, args string) []byte {
	t.Helper()
	res, err := reg.Call(context.Background(), ws, name, json.RawMessage(args))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Every generated signature has a handler and every handler a signature,
// and the day-one metadata is present on all three.
func TestSignaturesAndHandlersAgree(t *testing.T) {
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if unbound := reg.Unbound(); len(unbound) != 0 {
		t.Fatalf("signatures with no handler: %v", unbound)
	}
	sigs := reg.List()
	if len(sigs) != 3 {
		t.Fatalf("expected the three Phase-0 tools, got %d", len(sigs))
	}
	for _, sig := range sigs {
		if sig.Reversibility != "na" {
			t.Errorf("%s: the three read-only tools carry reversibility 'na', got %q", sig.Name, sig.Reversibility)
		}
		if sig.MinTier != "T0" {
			t.Errorf("%s: min_tier %q, want T0", sig.Name, sig.MinTier)
		}
		if len(sig.InputSchema) == 0 {
			t.Errorf("%s: no input schema", sig.Name)
		}
	}
}

// Criterion 12, second clause: all three tools through the registry, no
// adapter anywhere.
func TestRegistryDrivesAllThreeTools(t *testing.T) {
	ws := fixture(t)
	reg, err := DefaultRegistry()
	if err != nil {
		t.Fatal(err)
	}

	// Tool 1 — the workspace-wide view is reachable (F5).
	raw := call(t, reg, ws, "list_domain_classes", "{}")
	var listed struct {
		Classes []ClassSummary `json:"classes"`
		Count   int            `json:"count"`
	}
	if err := json.Unmarshal(raw, &listed); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range listed.Classes {
		names[c.Name] = true
	}
	for _, want := range []string{"jetsa:Incident", "jetsa:AgentRun", "jets:Entity", "lp:Person"} {
		if !names[want] {
			t.Errorf("list_domain_classes: %s missing from the workspace-wide view", want)
		}
	}

	// Tool 2, schema-first class — the sidecar and the compiled model agree,
	// and the vocabulary is inlined in full.
	raw = call(t, reg, ws, "describe_domain_class", `{"name":"jetsa:Incident"}`)
	var desc ClassDescription
	if err := json.Unmarshal(raw, &desc); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(desc.Documentation, "triaged cluster") {
		t.Errorf("describe jetsa:Incident: sidecar documentation missing, got %q", desc.Documentation)
	}
	if got := len(desc.Vocabularies["IncidentStatus"]); got != 11 {
		t.Errorf("describe jetsa:Incident: IncidentStatus inlined with %d values, want 11", got)
	}

	// Tool 2, PHI marking arrives through the triples channel.
	raw = call(t, reg, ws, "describe_domain_class", `{"name":"jetsa:Evidence"}`)
	if err := json.Unmarshal(raw, &desc); err != nil {
		t.Fatal(err)
	}
	foundPHI := false
	for _, tr := range desc.Triples {
		if tr.Subject == "jetsa:statement" && tr.Predicate == "jetsa:data_classification" && tr.Object == "PHI" {
			foundPHI = true
		}
	}
	if !foundPHI {
		t.Errorf("describe jetsa:Evidence: the PHI data_classification triple is not visible; got %+v", desc.Triples)
	}

	// Tool 2, .jr-authored class — the defined degrade, not a missing-key error.
	raw = call(t, reg, ws, "describe_domain_class", `{"name":"lp:Person"}`)
	if err := json.Unmarshal(raw, &desc); err != nil {
		t.Fatal(err)
	}
	if desc.Documentation != "declared, no metadata authored" && desc.Documentation != "documented by workspace triples" {
		t.Errorf("describe lp:Person: expected the defined degrade, got %q", desc.Documentation)
	}

	// Tool 3, a valid config is accepted. A minimal one, authored to the
	// current contract — test_ws's own pipes_config files are stale against
	// it (string delimiters, output_tables without channel_spec_name) and
	// are not part of the counted corpus, so they prove nothing here.
	validCfg := `{
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
	raw = call(t, reg, ws, "validate_cpipes_config", `{"config":`+validCfg+`}`)
	var report ValidationReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Valid || report.StepsValidated != 1 {
		t.Errorf("validate_cpipes_config rejects a valid config: %+v", report)
	}

	// Tool 3, a broken config is rejected with a diagnostic, not an error.
	raw = call(t, reg, ws, "validate_cpipes_config",
		`{"config":{"conditional_pipes_config":[{"pipes_config":[]}]}}`)
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Valid || len(report.Diagnostics) == 0 {
		t.Errorf("validate_cpipes_config accepted a config with an empty conditional step: %+v", report)
	}
}

// A3.1: the glossary narrows per entity, inlines vocabularies in full, and
// reports the undocumented rather than omitting them.
func TestGlossary(t *testing.T) {
	ws := fixture(t)
	text, err := ws.Glossary([]string{"jetsa:Incident", "jetsa:Evidence", "lp:Person", "jetsa:NoSuchThing"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"suppressed_as_benign",           // the 11th IncidentStatus value — the taxonomy is inlined in full
		"PHI",                            // Evidence.statement's classification is stated to the prompt
		"declared, no metadata authored", // lp:Person degrades honestly
		"not declared in this workspace", // and the unknown is reported, not omitted
	} {
		if !strings.Contains(text, want) {
			t.Errorf("glossary missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "jetsa:AgentRun:") {
		t.Error("glossary did not narrow: an unrequested entity appears")
	}
}
