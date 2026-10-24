package jetrules_native_adaptor

// A RULE CONFIGURATION SETTING _0:no_truth_main_on_exist IS REFUSED, AND THE REFUSAL
// REACHES GO WITH ITS REASON.
//
// The switch was deleted in October 2026 with the LEGACY retraction mode, which this
// file tested until then (rules_unit_test_harness AF.5: the mode reached the C++ engine
// through the factory). ReteSession::initialize refuses a configuration that still turns
// it on, and bridge.NewReteSession returns the session's message rather than a bare
// return code. The rule_config triples are in the meta graph, inserted before the first
// rdf session, as a pipeline inserts them.
//
// Needs libjets.so with get_rete_session_err_msg: set CGO_CFLAGS, CGO_LDFLAGS and
// LD_LIBRARY_PATH to a build of this commit. Runs in a child process over a compiled
// copy of jets/jetrules/test_ws, because package workspace fixes its paths at init. Run
// it with -count=1.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const engineModeChildEnv = "JETS_NATIVE_ENGINE_MODE_TEST_HOME"

func TestNativeEngineRefusesNoTruthMainOnExist(t *testing.T) {
	if os.Getenv(engineModeChildEnv) == "" {
		runInCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "af5",
		workspace.SkipAssetValidation("native engine refusal test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	const process = "jet_rules/test_join_values_main.jr"
	for _, tc := range []struct {
		name    string
		value   func(compute_pipes.JetResourceManager) compute_pipes.RdfNode
		refused bool
	}{
		{"absent", nil, false},
		{"zero", func(rm compute_pipes.JetResourceManager) compute_pipes.RdfNode { return rm.NewIntLiteral(0) }, false},
		{"one", func(rm compute_pipes.JetResourceManager) compute_pipes.RdfNode { return rm.NewIntLiteral(1) }, true},
		{"text", func(rm compute_pipes.JetResourceManager) compute_pipes.RdfNode { return rm.NewTextLiteral("1") }, true},
	} {
		engine, err := NewJetRulesFactory().NewJetRuleEngine(nil, process, false)
		if err != nil {
			t.Fatalf("%s: building the native engine: %v", tc.name, err)
		}
		withVertexCap(t, engine)
		rm := engine.GetMetaResourceManager()
		if tc.value != nil {
			if err := engine.Insert(engine.JetResources().Jets__istate,
				rm.NewResource("_0:no_truth_main_on_exist"), tc.value(rm)); err != nil {
				t.Fatalf("%s: inserting the rule config: %v", tc.name, err)
			}
		}
		ses, err := engine.NewRdfSession()
		if err != nil {
			t.Fatalf("%s: NewRdfSession: %v", tc.name, err)
		}
		rete, err := ses.NewReteSession(process)
		switch {
		case tc.refused && err == nil:
			t.Errorf("%s: the session started", tc.name)
		case tc.refused:
			for _, want := range []string{"_0:no_truth_main_on_exist", "deleted with the LEGACY retraction mode", "remove it"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("%s: the error does not say %q: %v", tc.name, want, err)
				}
			}
		case err != nil:
			t.Errorf("%s: refused: %v", tc.name, err)
		default:
			rete.Release()
		}
		ses.Release()
		engine.Release()
	}
}

// testVertexCap is the vertex visit cap withVertexCap sets: 10,000 visits, the Go
// engine's default, which the tests moved here from the Go adaptor ran under, and
// C++'s own default since rules_benchmark Q-83.
const testVertexCap = 10000

// withVertexCap asserts (jets:iState jets:max_vertex_visits testVertexCap) into the
// engine's meta graph, before its first rdf session, as a pipeline's rule_config
// is. C++ had no cap without it until Q-83, when a rule that stopped terminating
// hung the test to its timeout; the cap is kept explicit, so the tests do not
// depend on the default.
func withVertexCap(t *testing.T, engine compute_pipes.JetRuleEngine) {
	t.Helper()
	jr := engine.JetResources()
	if err := engine.Insert(jr.Jets__istate, jr.Jets__max_vertex_visits,
		engine.GetMetaResourceManager().NewIntLiteral(testVertexCap)); err != nil {
		t.Fatalf("asserting the vertex visit cap: %v", err)
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
