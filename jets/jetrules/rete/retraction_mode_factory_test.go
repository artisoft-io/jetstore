package rete_test

// THE RETRACTION MODE REACHES A COMPILED NETWORK THROUGH THE FACTORY.
//
// retract_recorded_consequents_test.go and loop_through_negation_test.go build their
// networks by hand and set the mode on the meta store themselves, so neither shows that
// NewReteMetaStoreFactory passes WithRetractionMode to AddConsequentTerm and to the
// meta store it builds. This test does, on test_ws compiled into a scratch directory:
// test_join_values_main.jr's consequents are join_values aggregates, which read the
// graph and mint nothing, so their vertexes record only with LEGACY defect 1 off.
//
// WHY A CHILD PROCESS AND package rete_test. The workspace is compiled in-process by
// workspace.CompileWorkspace, with the asset validator skipped (test_ws has no user
// flows), so the test needs neither the compile_workspace binary nor
// JETS_WORKSPACE_VALIDATOR. Package workspace imports package rete, so only an external
// test package can import it; and both packages fix their workspace paths from the
// environment at init, so the test re-runs itself in a child whose WORKSPACES_HOME is
// the scratch copy. Run it with -count=1: it compiles files outside the Go module's
// cache key.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const (
	// compiledTestWsChildEnv marks the child process rerunOverCompiledTestWs starts;
	// join_values_rule_test.go uses it too.
	compiledTestWsChildEnv = "JETS_RETE_COMPILED_TEST_WS_CHILD"
	factoryModeRuleFile    = "jet_rules/test_join_values_main.jr"
)

func TestFactoryAppliesRetractionMode(t *testing.T) {
	if os.Getenv(compiledTestWsChildEnv) == "" {
		rerunOverCompiledTestWs(t)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "af_factory_mode",
		workspace.SkipAssetValidation("rete factory retraction mode test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}

	// recording counts the vertexes that record, and the ones whose consequents read
	// the graph and mint nothing -- the vertexes defect 1 is about.
	recording := func(ms *rete.ReteMetaStore) (records, graphOnly int) {
		for _, nv := range ms.NodeVertices {
			if nv.RecordsConsequents {
				records++
			}
			reads, mints := false, false
			for _, an := range nv.ConsequentAlphaNodes {
				reads = reads || an.ReadsGraph()
				mints = mints || an.MintsResource()
			}
			if reads && !mints {
				graphOnly++
			}
		}
		return
	}

	replay := rete.RetractionMode{ReplayGraphReadingConsequents: true}
	fixed := rete.RetractionMode{ReplayGraphReadingConsequents: true, EraseWithdrawnRows: true}
	for _, c := range []struct {
		name string
		opts []rete.FactoryOption
		want rete.RetractionMode
	}{
		{"no option", nil, rete.LegacyRetraction},
		{"legacy", []rete.FactoryOption{rete.WithRetractionMode(rete.LegacyRetraction)}, rete.LegacyRetraction},
		{"fixed-1", []rete.FactoryOption{rete.WithRetractionMode(replay)}, replay},
		{"fixed", []rete.FactoryOption{rete.WithRetractionMode(fixed)}, fixed},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := rete.NewReteMetaStoreFactory(factoryModeRuleFile, c.opts...)
			if err != nil {
				t.Fatalf("loading the compiled workspace: %v", err)
			}
			if f.RetractionMode != c.want {
				t.Errorf("factory mode = %+v, want %+v", f.RetractionMode, c.want)
			}
			ms := f.MetaStoreLookup[factoryModeRuleFile]
			if ms == nil {
				t.Fatalf("no metastore for %s", factoryModeRuleFile)
			}
			if ms.RetractionMode != c.want {
				t.Errorf("meta store mode = %+v, want %+v", ms.RetractionMode, c.want)
			}
			records, graphOnly := recording(ms)
			if graphOnly == 0 {
				t.Fatal("the rule file has no consequent that reads the graph and mints nothing: this test proves nothing")
			}
			if c.want.ReplayGraphReadingConsequents {
				if records < graphOnly {
					t.Errorf("%d vertexes record, want at least the %d graph-reading ones", records, graphOnly)
				}
			} else if records != 0 {
				t.Errorf("%d vertexes record in a legacy mode for defect 1, want 0 (the file mints nothing)", records)
			}
		})
	}
}

// rerunOverCompiledTestWs copies jets/jetrules/test_ws to a scratch WORKSPACES_HOME and
// re-runs the calling test in a child process pointed at it.
func rerunOverCompiledTestWs(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd() // jets/jetrules/rete
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	home := t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	// The compiler builds every path from workspace_control.json's workspace_name, which
	// the checked-in file does not carry; patched in the copy only.
	ctlPath := filepath.Join(wsDir, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["workspace_name"] = "test_ws"
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
		compiledTestWsChildEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed in the child process: %v\n%s", t.Name(), err, out)
	}
	if testing.Verbose() {
		var keep []string
		for line := range strings.SplitSeq(string(out), "\n") {
			if strings.Contains(line, "--- ") || strings.Contains(line, "_test.go:") {
				keep = append(keep, line)
			}
		}
		t.Logf("child process:\n%s", strings.Join(keep, "\n"))
	}
}
