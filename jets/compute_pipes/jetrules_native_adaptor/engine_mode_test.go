package jetrules_native_adaptor

// THE RETRACTION MODE REACHES THE C++ ENGINE THROUGH THE FACTORY INTERFACE
// (rules_unit_test_harness Phase 1, task AF.5).
//
// With no option -- every pipeline -- NewJetRuleEngine loads the workspace through
// bridge.LoadJetRules, create_jetstore_hdl, as before the mode existed. With
// compute_pipes.WithRetractionMode and a defect turned off it loads through
// bridge.LoadJetRulesWithMode, create_jetstore_hdl_with_mode. What the modes do to
// a network is tested in jets/rete (retract_recorded_consequents_test.cc,
// loop_through_negation_test.cc); this pins that every mode builds an engine and
// the engine reports the mode it was built in.
//
// Needs libjets.so with create_jetstore_hdl_with_mode: set CGO_CFLAGS, CGO_LDFLAGS
// and LD_LIBRARY_PATH to a build of this commit. Runs in a child process over a
// compiled copy of jets/jetrules/test_ws, because package workspace fixes its
// paths at init. Run it with -count=1.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const engineModeChildEnv = "JETS_NATIVE_ENGINE_MODE_TEST_HOME"

func TestNativeEngineRetractionMode(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "af5",
		workspace.SkipAssetValidation("AF.5 native engine mode test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	const process = "jet_rules/test_join_values_main.jr"
	build := func(opts ...compute_pipes.EngineOption) compute_pipes.RetractionMode {
		t.Helper()
		engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, process, false, opts...)
		if err != nil {
			t.Fatalf("building the native engine: %v", err)
		}
		defer engine.Release()
		// the engine is usable: a meta resource round-trips
		if r := engine.GetMetaResourceManager().NewIntLiteral(5); r == nil || r.Value() != 5 {
			t.Errorf("NewIntLiteral(5) = %v", r)
		}
		return engine.(*JetRuleEngineNative).RetractionMode()
	}
	if got := build(); !got.IsLegacy() {
		t.Errorf("no option (a pipeline): mode %s, want legacy", got.Label())
	}
	for _, mode := range rete.AllRetractionModes() {
		if got := build(compute_pipes.WithRetractionMode(mode)); got != mode {
			t.Errorf("WithRetractionMode(%s): engine built in %s", mode.Label(), got.Label())
		}
	}
}

// runInCompiledTestWs copies test_ws to a scratch WORKSPACES_HOME and re-runs the
// test in a child process pointed at it.
func runInCompiledTestWs(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd() // jets/compute_pipes/jetrules_native_adaptor
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	home := t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws" // the compiler builds its paths from it
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(),
		"WORKSPACES_HOME="+home,
		"WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
		engineModeChildEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed in the child process: %v\n%s", t.Name(), err, out)
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", out)
	}
}
