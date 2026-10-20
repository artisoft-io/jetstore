package graphdump

// Dumps of real engine sessions.
//
// WHY A CHILD PROCESS. compute_pipes, the rete package and jets/workspace read
// WORKSPACES_HOME and WORKSPACE in init, so a test can point them at a scratch
// workspace only by running the test binary again with the environment set (the
// same arrangement as jets/rulestest/harness/go_engine_test.go). -count=1: the
// workspaces read are outside the module's cache key.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const (
	childEnv        = "GRAPHDUMP_TEST_CHILD"
	loopingRuleFile = "jet_rules/test_looping_main.jr"
)

// factories are the engines a test can build. native_test.go adds the C++ one
// when the package is built with -tags jets_native.
var factories = map[string]func() compute_pipes.JetRulesFactory{
	rulestest.EngineGo: jetrules_go_adaptor.NewJetRulesFactory,
}

// rerunOverTestWs copies jets/jetrules/test_ws to a scratch WORKSPACES_HOME and
// runs the calling test again in a child process pointed at it.
func rerunOverTestWs(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("compiles a workspace")
	}
	wd, err := os.Getwd() // jets/rulesbench/graphdump
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	home := t.TempDir()
	wsDir := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", filepath.Join(root, "jets", "jetrules", "test_ws"), wsDir).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	// The compiler builds its paths from workspace_name, which the checked-in
	// workspace_control.json lacks.
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
	if err := os.WriteFile(ctlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"))
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: "+t.Name()) {
		t.Fatalf("child run failed: %v\n%s", err, out)
	}
	if testing.Verbose() {
		t.Logf("child:\n%s", out)
	}
}

func compileTestWs(t *testing.T) {
	t.Helper()
	if _, err := workspace.CompileWorkspace(nil, "test_ws", "graphdump",
		workspace.SkipAssetValidation("a scratch copy of test_ws compiled for its rules only")); err != nil {
		t.Fatalf("compiling test_ws: %v", err)
	}
}

// loopingDumps runs test_looping_main.jr on a new session of engine and returns
// the dump after its rule set, with and without the meta graph.
func loopingDumps(t *testing.T, engine compute_pipes.JetRuleEngine) (dump, withMeta []string) {
	t.Helper()
	ses, err := engine.NewRdfSession()
	if err != nil {
		t.Fatal(err)
	}
	defer ses.Release()
	outcome := compute_pipes.RunRuleSets(ses, engine, &compute_pipes.RuleSetsConfig{
		AfterRuleSet: func(s compute_pipes.JetRdfSession, rs *compute_pipes.RuleSetOutcome) {
			if dump, err = Lines(s, Options{}); err != nil {
				t.Fatalf("dump: %v", err)
			}
			if withMeta, err = Lines(s, Options{IncludeMeta: true}); err != nil {
				t.Fatalf("dump with meta: %v", err)
			}
		},
	}, func(compute_pipes.JetRdfSession) error { return nil })
	if outcome.Err != nil || !outcome.Completed {
		t.Fatalf("the rules did not complete: %+v", outcome)
	}
	return dump, withMeta
}

// checkLoopingDump checks what test_looping_main.jr leaves, as the harness's
// TestGoEngineCreateEntitySubjects reads it: three persons made by create_entity,
// each named three times, and completion at loop 3.
func checkLoopingDump(t *testing.T, dump, withMeta []string) {
	t.Helper()
	t.Logf("dump:\n%s", strings.Join(dump, "\n"))
	persons, names := map[string]bool{}, 0
	for _, l := range dump {
		s, p, o, ok := ParseLine(l)
		if !ok {
			t.Errorf("line does not parse: %q", l)
		}
		if uuidV4.MatchString(l) {
			t.Errorf("a minted UUID survived: %s", l)
		}
		if p.Value == "rdf:type" && o.Value == "lp:Person" {
			if s.Kind != "minted" {
				t.Errorf("a person is not renamed: %s", l)
			}
			persons[s.Value] = true
		}
		if p.Value == "lp:name" {
			names++
		}
	}
	if len(persons) != 3 || names != 9 {
		t.Errorf("want 3 persons with 9 names, got %d and %d", len(persons), names)
	}
	for _, want := range []string{"jets:iState\tjets:completed\t1^^int", "jets:iState\tjets:loop\t3^^int"} {
		if !contains(dump, want) {
			t.Errorf("missing %q", want)
		}
	}
	// (jets:iState rdf:type jets:State) is a triple(...) of jets_model.jr: meta.
	const meta = "jets:iState\trdf:type\tjets:State"
	if contains(dump, meta) || !contains(withMeta, meta) {
		t.Errorf("meta triple %q: in dump %v, in dump with meta %v; want false, true", meta, contains(dump, meta), contains(withMeta, meta))
	}
	if len(withMeta) <= len(dump) {
		t.Errorf("the meta graph added no line: %d and %d", len(withMeta), len(dump))
	}
}

func contains(lines []string, l string) bool {
	for _, x := range lines {
		if x == l {
			return true
		}
	}
	return false
}

// Two sessions of the Go engine mint different UUIDs and give the same dump.
func TestGoSessionDump(t *testing.T) {
	if os.Getenv(childEnv) == "" {
		rerunOverTestWs(t)
		return
	}
	compileTestWs(t)
	engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, loopingRuleFile, false)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Release()
	engine.JetResources()
	first, withMeta := loopingDumps(t, engine)
	checkLoopingDump(t, first, withMeta)
	second, _ := loopingDumps(t, engine)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two sessions differ:\n%s\n---\n%s", strings.Join(first, "\n"), strings.Join(second, "\n"))
	}
	excluded, err := dumpExcluding(t, engine, "lp:name")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range excluded {
		if strings.Contains(l, "\tlp:name\t") {
			t.Errorf("an excluded predicate is written: %s", l)
		}
	}
	if len(excluded) != len(first)-9 {
		t.Errorf("excluding lp:name left %d lines of %d, want 9 fewer", len(excluded), len(first))
	}
}

func dumpExcluding(t *testing.T, engine compute_pipes.JetRuleEngine, p string) (lines []string, err error) {
	ses, err := engine.NewRdfSession()
	if err != nil {
		return nil, err
	}
	defer ses.Release()
	compute_pipes.RunRuleSets(ses, engine, &compute_pipes.RuleSetsConfig{
		AfterRuleSet: func(s compute_pipes.JetRdfSession, _ *compute_pipes.RuleSetOutcome) {
			lines, err = Lines(s, Options{ExcludePredicates: []string{p}})
		},
	}, func(compute_pipes.JetRdfSession) error { return nil })
	return lines, err
}

// A session that cannot name its engine is refused unless the meta graph is wanted.
func TestSessionWithoutEngine(t *testing.T) {
	var s compute_pipes.JetRdfSession = noEngineSession{}
	if _, err := Lines(s, Options{}); err == nil || !strings.Contains(err.Error(), "IncludeMeta") {
		t.Errorf("want a refusal naming IncludeMeta, got %v", err)
	}
}

type noEngineSession struct{ compute_pipes.JetRdfSession }

func (noEngineSession) Find() compute_pipes.TripleIterator { return emptyIterator{} }

type emptyIterator struct{ compute_pipes.TripleIterator }

func (emptyIterator) IsEnd() bool    { return true }
func (emptyIterator) Release() error { return nil }

// TestDumpRulesUnitTest is the driver of the equivalence runs, not a check: it
// runs every input session of a rules unit test through RunRuleSets, the
// pipeline's loop, and writes a dump after each rule set, as
// <GRAPHDUMP_OUT>/<engine>/<session>/<NN>_<ruleset>.tsv. It skips unless
// GRAPHDUMP_TST names the .tst.json and GRAPHDUMP_OUT the output tree, and
// WORKSPACES_HOME and WORKSPACE name a compiled workspace. GRAPHDUMP_ENGINE is go
// (the default) or native, which needs -tags jets_native.
func TestDumpRulesUnitTest(t *testing.T) {
	tst, out := os.Getenv("GRAPHDUMP_TST"), os.Getenv("GRAPHDUMP_OUT")
	if tst == "" || out == "" {
		t.Skip("set GRAPHDUMP_TST and GRAPHDUMP_OUT to run")
	}
	engineName := os.Getenv("GRAPHDUMP_ENGINE")
	if engineName == "" {
		engineName = rulestest.EngineGo
	}
	factory := factories[engineName]
	if factory == nil {
		t.Skipf("no %s engine in this build: build with -tags jets_native and the cgo override", engineName)
	}
	td, findings, err := rulestest.LoadTestDefinition(tst)
	if err != nil || td == nil || rulestest.HasErrors(findings) {
		t.Fatalf("test definition %s: %v %v", tst, err, findings)
	}
	cfg := &td.JetrulesConfig
	spec, err := harness.JetrulesSpecOf(cfg)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := factory().NewJetRuleEngine(nil, cfg.ProcessName, false,
		compute_pipes.WithRetractionMode(harness.RetractionModeOf(td)))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Release()
	engine.JetResources()
	env := map[string]any{}
	if err := compute_pipes.AssertSourcePeriodInfo(engine, spec, env); err != nil {
		t.Fatal(err)
	}
	if err := compute_pipes.AssertRuleConfiguration(engine, spec, env); err != nil {
		t.Fatal(err)
	}
	for _, is := range td.InputSessions {
		var tables []*harness.InputTable
		for _, in := range is.Input {
			tbl, err := harness.ReadInputTable(filepath.Join(filepath.Dir(tst), in), in)
			if err != nil {
				t.Fatal(err)
			}
			tables = append(tables, tbl)
		}
		ses, err := engine.NewRdfSession()
		if err != nil {
			t.Fatal(err)
		}
		index := 0
		outcome := compute_pipes.RunRuleSets(ses, engine, &compute_pipes.RuleSetsConfig{
			MaxLooping: cfg.MaxLooping,
			AfterRuleSet: func(s compute_pipes.JetRdfSession, rs *compute_pipes.RuleSetOutcome) {
				path := Path(out, engineName, is.Name, index, rs.RuleSet)
				index++
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := WriteSession(f, s, Options{}); err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Logf("%s: loops %d, completed %v", path, rs.Loops, rs.Completed)
			},
		}, func(s compute_pipes.JetRdfSession) error {
			for _, tbl := range tables {
				for _, row := range tbl.Rows {
					if err := compute_pipes.AssertTextRow(spec, s, tbl.Columns, row); err != nil {
						return err
					}
				}
			}
			return nil
		})
		ses.Release()
		if outcome.Err != nil || outcome.ExecuteErr != nil {
			t.Errorf("session %s: %v %v", is.Name, outcome.Err, outcome.ExecuteErr)
		}
	}
}
