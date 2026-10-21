package rulesbench

// Run on the Go engine, end to end, over a compiled copy of
// jets/jetrules/test_ws in a child process: the workspace packages fix their
// paths at init, as the RunRuleSets tests in jets/compute_pipes note. The C++
// engine runs the same code through the same interface; the command's smoke run
// on usi_ws cm_loop covers it (README.md). Run with -count=1.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/rulestest/harness"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const runChildEnv = "JETS_RULESBENCH_RUN_TEST_HOME"

func inCompiledTestWs(t *testing.T, test func(t *testing.T)) {
	t.Helper()
	if os.Getenv(runChildEnv) != "" {
		if _, err := os.Stat(filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws", "build")); err != nil {
			if out, err := workspace.CompileWorkspace(nil, "test_ws", "rbm",
				workspace.SkipAssetValidation("rulesbench tests: test_ws has no user flows")); err != nil {
				t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
			}
		}
		test(t)
		return
	}
	wd, err := os.Getwd() // jets/rulesbench
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
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
	ctl["workspace_name"] = "test_ws"
	if raw, err = json.MarshalIndent(ctl, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(),
		"WORKSPACES_HOME="+home,
		"WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
		runChildEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	var keep []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "---") || strings.Contains(line, "_test.go:") ||
			strings.HasPrefix(l, "FAIL") || strings.HasPrefix(l, "panic") {
			keep = append(keep, line)
		}
	}
	if err != nil {
		t.Fatalf("%s failed in the child process: %v\n%s", t.Name(), err, strings.Join(keep, "\n"))
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", strings.Join(keep, "\n"))
	}
}

// writeBench writes a spec running test_looping_main.jr ($max_looping 5,
// jets:completed on loop 3, one lp:Person per loop) over n one-case sessions.
func writeBench(t *testing.T, n int, extra string) string {
	t.Helper()
	dir := t.TempDir()
	var sessions []string
	for i := 1; i <= n; i++ {
		name := "case" + string(rune('0'+i))
		csv := "jets:key,rdf:type,xr:mode\n" + name + ",xr:Case,ok\n"
		if err := os.WriteFile(filepath.Join(dir, name+".csv"), []byte(csv), 0o644); err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, `{"name": "`+name+`", "input": ["`+name+`.csv"]}`)
	}
	spec := `{
	  "bench_name": "looping",
	  "jetrules_config": {"process_name": "jet_rules/test_looping_main.jr", "current_source_period": 1},
	  "input_sessions": [` + strings.Join(sessions, ",") + `],
	  "engines": ["go"],
	  "output_classes": [{"class_name": "lp:Person", "columns": []}]` + extra + `
	}`
	path := filepath.Join(dir, "bench.json")
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func runBench(t *testing.T, path string, opts Options) ([]*RunResult, error) {
	t.Helper()
	spec, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	opts.Spec, opts.SpecPath = spec, path
	opts.Factories = harness.Factories{Go: jetrules_go_adaptor.NewJetRulesFactory()}
	return Run(opts)
}

func TestRun(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		t.Run("pool_of_two_with_warmup", func(t *testing.T) {
			path := writeBench(t, 4, `, "max_vertex_visits": 1000, "pool_size": 2, "repetitions": 2, "warmup": 1`)
			var seen int
			before, after := 0, 0
			results, err := runBench(t, path, Options{
				SampleMs:     1,
				CountTriples: true,
				PerLoop:      true,
				OnResult:     func(*RunResult) { seen++ },
				BeforeRuns:   func() { before++ },
				AfterRuns:    func() { after++ },
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 3 || seen != 3 || before != 1 || after != 1 {
				t.Fatalf("%d results, %d seen, hooks %d/%d; want 3 (1 warmup, 2 measured), each once", len(results), seen, before, after)
			}
			if !results[0].Warmup || results[1].Warmup || results[2].Repetition != 2 || results[2].Order != 3 {
				t.Errorf("warmup and repetitions: %+v %+v %+v", results[0].Warmup, results[1].Warmup, results[2].Repetition)
			}
			r := results[1]
			if r.Status != "ok" || r.Engine != "go" || r.LegacyMode != "legacy" || r.PoolSize != 2 || r.MaxVertexVisits != 1000 {
				t.Errorf("run %+v", r)
			}
			if !r.Load.MaxVertexVisitsSeen || r.Load.MaxVertexVisitsFrom != "spec" || r.Load.EngineNs <= 0 {
				t.Errorf("load %+v", r.Load)
			}
			if r.Totals.Sessions != 4 || r.Totals.InputRows != 4 || r.Totals.Loops != 12 || r.Totals.Errors != 0 || r.Totals.Extracted != 12 {
				t.Errorf("totals %+v, want 4 sessions of 1 row, 3 loops and 3 persons each", r.Totals)
			}
			if r.Memory.Go.Mallocs == 0 || r.Memory.MaxRSSKB == 0 || r.Memory.PeakGoroutines == 0 {
				t.Errorf("memory %+v", r.Memory)
			}
			workers := map[int]bool{}
			for _, s := range r.Sessions {
				workers[s.Worker] = true
				if !s.Completed || s.Loops != 3 || s.Extracted["lp:Person"] != 3 || s.TotalNs <= 0 || s.InputAssertNs <= 0 {
					t.Errorf("session %+v", s)
				}
				if s.Memory != nil {
					t.Errorf("session %s has per-session memory with a pool of 2", s.Name)
				}
				if s.TriplesTotal == nil || s.Graph == nil || *s.TriplesTotal != s.Graph.Meta+s.Graph.Asserted+s.Graph.Inferred {
					t.Errorf("session %s: %v triples by iteration, graph %+v", s.Name, s.TriplesTotal, s.Graph)
				}
				if len(s.RuleSets) != 1 {
					t.Fatalf("session %s: %d rule sets", s.Name, len(s.RuleSets))
				}
				rs := s.RuleSets[0]
				if rs.LoopsRun != 4 || len(rs.LoopNs) != 4 || rs.LastLoop != 3 || !rs.Completed || rs.Rete == nil || rs.Rete.VertexVisits == 0 {
					t.Errorf("rule set %+v", rs)
				}
			}
			for w := range workers {
				if w < 0 || w > 1 {
					t.Errorf("worker %d of a pool of 2", w)
				}
			}
		})
		t.Run("warmup_zero_overrides_the_spec", func(t *testing.T) {
			path := writeBench(t, 1, `, "max_vertex_visits": 1000, "warmup": 2`)
			zero := 0
			results, err := runBench(t, path, Options{Warmup: &zero})
			if err != nil || len(results) != 1 || results[0].Warmup {
				t.Errorf("warmup 0 over a spec's 2: %d results, %v", len(results), err)
			}
		})
		t.Run("pool_of_one_records_session_memory", func(t *testing.T) {
			path := writeBench(t, 2, `, "max_vertex_visits": 1000`)
			results, err := runBench(t, path, Options{})
			if err != nil || len(results) != 1 {
				t.Fatalf("%v, %d results", err, len(results))
			}
			for _, s := range results[0].Sessions {
				if s.Memory == nil || s.Memory.Mallocs == 0 || s.Worker != 0 {
					t.Errorf("session %s: memory %+v, worker %d", s.Name, s.Memory, s.Worker)
				}
			}
		})
		t.Run("cap_required", func(t *testing.T) {
			path := writeBench(t, 1, ``)
			if _, err := runBench(t, path, Options{}); err == nil || !strings.Contains(err.Error(), "max_vertex_visits is required") {
				t.Errorf("no cap: %v", err)
			}
			if r, err := runBench(t, path, Options{MaxVertexVisits: 50}); err != nil || r[0].Load.MaxVertexVisitsFrom != "flag" {
				t.Errorf("a cap from the flag: %v", err)
			}
		})
		t.Run("cap_binds", func(t *testing.T) {
			// One visit per vertex cannot complete three loops: the cap binds and
			// the run reports it.
			path := writeBench(t, 1, `, "max_vertex_visits": 1`)
			results, err := runBench(t, path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			s := results[0].Sessions[0]
			if results[0].Totals.VertexCapReached != 1 || !s.RuleSets[0].VertexCapReached || s.ExecuteErr == "" {
				t.Errorf("session %+v, want the vertex cap reached", s)
			}
		})
		t.Run("after_rule_set_hook", func(t *testing.T) {
			// The hook's time, slept here, is the session's dump_ns and is in no
			// other phase; the counter walk is counters_ns, apart too.
			const delay = 150 * time.Millisecond
			path := writeBench(t, 2, `, "max_vertex_visits": 1000, "repetitions": 2`)
			var calls []string
			results, err := runBench(t, path, Options{
				AfterRuleSet: func(p DumpPoint, s compute_pipes.JetRdfSession) error {
					calls = append(calls, fmt.Sprintf("%s %s %s %d %s", p.RunDir(), p.Engine, p.Session, p.Index, p.RuleSet))
					time.Sleep(delay)
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				"r01 go case1 0 jet_rules/test_looping_main.jr", "r01 go case2 0 jet_rules/test_looping_main.jr",
				"r02 go case1 0 jet_rules/test_looping_main.jr", "r02 go case2 0 jet_rules/test_looping_main.jr",
			}
			if strings.Join(calls, "|") != strings.Join(want, "|") {
				t.Errorf("hook calls %v, want %v", calls, want)
			}
			s := results[0].Sessions[0]
			if s.DumpNs < delay.Nanoseconds() || s.RuleSetsNs >= delay.Nanoseconds() || s.TotalNs >= delay.Nanoseconds() {
				t.Errorf("dump_ns %d, rule_sets_ns %d, total_ns %d: the hook's %v must be in dump_ns only", s.DumpNs, s.RuleSetsNs, s.TotalNs, delay)
			}
			if s.CountersNs <= 0 {
				t.Errorf("counters_ns %d, want the Go counter walk timed", s.CountersNs)
			}
		})
	})
}

// TestSessionFailureRaisesTheRunStatus is rules_benchmark I-34's regression test: a
// session's err, execute_err or exceptions each fail the run, as the README says; a
// session that only reached max_looping does not.
func TestSessionFailureRaisesTheRunStatus(t *testing.T) {
	cases := []struct {
		s    SessionResult
		fail bool
	}{
		{SessionResult{Name: "ok", Completed: true}, false},
		{SessionResult{Name: "capped", MaxLoopReached: true}, false},
		{SessionResult{Name: "err", Err: "input does not read"}, true},
		{SessionResult{Name: "execute", ExecuteErr: "std::bad_alloc"}, true},
		{SessionResult{Name: "exception", Exceptions: []string{"exception one", "exception two"}}, true},
	}
	for _, c := range cases {
		reason := sessionFailure(&c.s)
		if (reason != "") != c.fail {
			t.Errorf("session %s: sessionFailure = %q, want failure %v", c.s.Name, reason, c.fail)
		}
		if c.fail && !strings.Contains(reason, c.s.Name) {
			t.Errorf("session %s: reason %q does not name the session", c.s.Name, reason)
		}
	}
}
