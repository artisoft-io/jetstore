package rulesbench

// Run on the C++ engine, end to end, over a compiled copy of
// jets/jetrules/test_ws in a child process: the workspace packages fix their
// paths at init, as the RunRuleSets tests in jets/compute_pipes note. These ran on
// the Go engine until its retirement (October 2026); they now need the cgo
// environment of jets/cmds/rules_unit_test/README.md; a C++ engine that does not
// build fails the test. Run with -count=1.

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_native_adaptor"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/rulesbench/graphdump"
	"github.com/artisoft-io/jetstore/jets/rulestest"
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
	// The optimizer statistics tests' rule set and sequence, in the copy only.
	if err := os.WriteFile(filepath.Join(wsDir, optimizerStatsRuleFile), []byte(optimizerStatsRules), 0o644); err != nil {
		t.Fatal(err)
	}
	ctl["rule_sets"] = append(ctl["rule_sets"].([]any), optimizerStatsRuleFile)
	ctl["rule_sequences"] = append(ctl["rule_sequences"].([]any), map[string]any{
		"name": optimizerStatsSequence, "rule_sets": []any{"jet_rules/test_execute_rules_main.jr", optimizerStatsRuleFile}})
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
	if strings.Contains(string(out), "--- SKIP: "+t.Name()) {
		t.Skipf("skipped in the child process:\n%s", strings.Join(keep, "\n"))
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
	  "engines": ["native"],
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
	opts.Factory = jetrules_native_adaptor.NewJetRulesFactory()
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
			if r.Status != "ok" || r.Engine != "native" || r.LegacyMode != "fixed" || r.PoolSize != 2 || r.MaxVertexVisits != 1000 {
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
				if s.TriplesTotal == nil || *s.TriplesTotal == 0 {
					t.Errorf("session %s: %v triples by iteration", s.Name, s.TriplesTotal)
				}
				if len(s.RuleSets) != 1 {
					t.Fatalf("session %s: %d rule sets", s.Name, len(s.RuleSets))
				}
				rs := s.RuleSets[0]
				if s.ReteReleaseNs != rs.ReteReleaseNs || rs.ReteReleaseNs < 0 {
					t.Errorf("session %s: rete_release_ns %d, its one rule set's %d", s.Name, s.ReteReleaseNs, rs.ReteReleaseNs)
				}
				if rs.LoopsRun != 4 || len(rs.LoopNs) != 4 || rs.LastLoop != 3 || !rs.Completed {
					t.Errorf("rule set %+v", rs)
				}
			}
			var reteRelease int64
			for _, s := range r.Sessions {
				reteRelease += s.ReteReleaseNs
			}
			if r.Totals.ReteReleaseNs != reteRelease {
				t.Errorf("totals rete_release_ns %d, sessions' sum %d", r.Totals.ReteReleaseNs, reteRelease)
			}
			for w := range workers {
				if w < 0 || w > 1 {
					t.Errorf("worker %d of a pool of 2", w)
				}
			}
		})
		t.Run("rows_dir_writes_each_sessions_rows", func(t *testing.T) {
			path := writeBench(t, 4, `, "max_vertex_visits": 1000, "pool_size": 2, "warmup": 1`)
			dir := t.TempDir()
			rows := NewRowWriter(dir)
			if _, err := runBench(t, path, Options{OnExtractedRow: rows.Write, AfterExtraction: rows.SessionDone}); err != nil {
				t.Fatal(err)
			}
			if len(rows.open) != 0 {
				t.Errorf("%d sessions' files still open after the run", len(rows.open))
			}
			if err := rows.Close(); err != nil {
				t.Fatal(err)
			}
			// Three persons per session, each a row of lp:Person's domain properties,
			// rdf:type among them as the class alone.
			for _, run := range []string{"w01", "r01"} {
				for i := 1; i <= 4; i++ {
					file := filepath.Join(dir, run, "native", fmt.Sprintf("case%d", i), "lp:Person.rows")
					raw, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
					if len(lines) != 3 || !strings.Contains(lines[0], "\t[string:\"lp:Person\"]\t") {
						t.Errorf("%s: %q; want three lp:Person rows", file, raw)
					}
				}
			}
		})
		// BU.2 (Q-98): an observing run writes one file per session and repetition,
		// each rule set's vertices with their rows, visits and rules, keyed to
		// workspace.db by rule set file and vertex; it is untimed.
		t.Run("observe_writes_each_session", func(t *testing.T) {
			path := writeBench(t, 2, `, "max_vertex_visits": 1000, "warmup": 1`)
			dir := t.TempDir()
			results, err := runBench(t, path, Options{ObserveDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range results {
				if r.Status != "ok" || !r.Observed {
					t.Errorf("result %d: status %s observed %v", r.Repetition, r.Status, r.Observed)
				}
			}
			for _, run := range []string{"w01", "r01"} {
				for i := 1; i <= 2; i++ {
					file := filepath.Join(dir, run, "native", fmt.Sprintf("case%d.json", i))
					raw, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					var so SessionObservation
					if err := json.Unmarshal(raw, &so); err != nil {
						t.Fatal(err)
					}
					if so.Schema != ObserveSchema || so.Session != fmt.Sprintf("case%d", i) || so.Warmup != (run == "w01") || len(so.RuleSets) == 0 {
						t.Fatalf("%s: %+v", file, so)
					}
					for _, ro := range so.RuleSets {
						if ro.Err != "" || ro.SourceFileKey < 0 || len(ro.Vertices) < 2 {
							t.Errorf("%s %s: error %q, file key %d, %d vertices", file, ro.RuleSet, ro.Err, ro.SourceFileKey, len(ro.Vertices))
						}
						var now, peak int64
						named, visited := 0, false
						for j, v := range ro.Vertices {
							now, peak = now+v.RowsNow, peak+v.RowsPeak
							if v.Vertex != j || v.RowsPeak < v.RowsNow {
								t.Errorf("%s %s: vertex %d at %d, now %d peak %d", file, ro.RuleSet, v.Vertex, j, v.RowsNow, v.RowsPeak)
							}
							if len(v.Rules) > 0 {
								named++
							}
							visited = visited || v.Inferred > 0
						}
						if now != ro.RowsNow || peak != ro.RowsPeak || ro.Vertices[0].ParentVertex != -1 || named < len(ro.Vertices)-1 || !visited {
							t.Errorf("%s %s: sums %d/%d against %d/%d, head parent %d, %d of %d named, visited %v",
								file, ro.RuleSet, now, peak, ro.RowsNow, ro.RowsPeak, ro.Vertices[0].ParentVertex, named, len(ro.Vertices), visited)
						}
					}
				}
			}
			for _, o := range []Options{{ObserveDir: dir, PerLoop: true}, {ObserveDir: dir, SampleMs: 10}} {
				if _, err := runBench(t, path, o); err == nil || !strings.Contains(err.Error(), "observing run is untimed") {
					t.Errorf("observing with timing (%+v): %v", o, err)
				}
			}
		})
		// BX.2: a statistics run counts the first measured repetition of each
		// session, after each rule set of a sequence, for the rule set's antecedents
		// only; its counts are the dump's, its keys are the rules', its bytes do not
		// depend on the pool's order, and it is untimed. The second rule set,
		// optimizerStatsRuleFile, has a text constant, a subject constant, a negated
		// antecedent and a volatile resource declared "_0:dbl".
		t.Run("stats_counts_each_session_once", func(t *testing.T) {
			path := writeStatsBench(t, optimizerStatsSequence, []string{"ok", "ok", "ok"},
				`, "max_vertex_visits": 1000, "warmup": 1, "repetitions": 2, "pool_size": 2`)
			dir := t.TempDir()
			dumps := filepath.Join(dir, "dumps")
			stats := filepath.Join(dir, "stats.json")
			results, err := runBench(t, path, Options{
				StatsFile: stats,
				// The graph the statistics count, meta graph included.
				AfterRuleSet: NewDumper(dumps, graphdump.Path, func(w io.Writer, s compute_pipes.JetRdfSession) error {
					return graphdump.WriteSession(w, s, graphdump.Options{IncludeMeta: true})
				}),
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range results {
				counted := !r.Warmup && r.Repetition == 1
				if r.Status != "ok" || r.Stats != counted {
					t.Errorf("result warmup %v repetition %d: status %s stats %v, want stats on r01 only", r.Warmup, r.Repetition, r.Status, r.Stats)
				}
			}
			f, err := optstats.Read(stats)
			if err != nil {
				t.Fatal(err)
			}
			ruleSets := []string{"jet_rules/test_execute_rules_main.jr", optimizerStatsRuleFile}
			if len(f.RuleSets) != len(ruleSets) {
				t.Fatalf("rule sets %v; want the sequence's %v", f.RuleSets, ruleSets)
			}
			for i, ruleSet := range ruleSets {
				rs := f.ForRuleSet(ruleSet)
				if rs == nil || rs.Sessions != 3 {
					t.Fatalf("%s: %+v; want 3 sessions (r01 only)", ruleSet, rs)
				}
				// The keys: every predicate and constant pair of the rule set's
				// antecedents in workspace.db, and nothing else.
				wanted := wantedFromWorkspaceDb(t, ruleSet)
				if !reflect.DeepEqual(keySet(rs.Predicates), wanted.Predicates) {
					t.Errorf("%s: predicates %v, want the antecedents' %v", ruleSet, keySet(rs.Predicates), wanted.Predicates)
				}
				for _, side := range []struct {
					name string
					got  map[string]map[string]int64
					want map[string]map[string]bool
				}{{"subject", rs.SubjectConstants, wanted.SubjectPairs}, {"object", rs.ObjectConstants, wanted.ObjectPairs}} {
					got := map[string]map[string]bool{}
					for p, m := range side.got {
						got[p] = keySet(m)
					}
					if !reflect.DeepEqual(got, side.want) {
						t.Errorf("%s: %s constants %v, want the antecedents' %v", ruleSet, side.name, got, side.want)
					}
				}
				// The counts, against the r01 dumps of this rule set read independently.
				want := countDumps(t, filepath.Join(dumps, "r01", "native"), fmt.Sprintf("%02d_*.tsv", i+1), wanted)
				if !reflect.DeepEqual(rs, want) {
					a, _ := json.Marshal(rs)
					b, _ := json.Marshal(want)
					t.Errorf("%s: statistics\n %s\nfrom the dumps\n %s", ruleSet, a, b)
				}
			}
			rs := f.ForRuleSet(optimizerStatsRuleFile)
			for _, c := range []struct {
				what string
				got  int64
				ok   bool
				want int64
			}{
				{"(jets:iState, rdf:type, *), a subject constant, two per session (the dumps agree)", rs.SubjectConstants["rdf:type"]["jets:iState"], rs.SubjectConstants["rdf:type"] != nil, 6},
				{`(*, xr:result, "done"), a text constant`, rs.ObjectConstants["xr:result"][`"done"`], rs.ObjectConstants["xr:result"] != nil, 3},
				{"xr:count, the negated antecedent's predicate, which no session holds", rs.Predicates["xr:count"].Triples, rs.Predicates["xr:count"] != nil, 0},
				{"(*, xr:count, 0), the negated antecedent's constant pair, a zero written", rs.ObjectConstants["xr:count"]["0^^int"], rs.ObjectConstants["xr:count"] != nil, 0},
				{"_0:_0:dbl, the volatile resource declared \"_0:dbl\", as the engine names it", rs.Predicates["_0:_0:dbl"].Triples, rs.Predicates["_0:_0:dbl"] != nil, 3},
			} {
				if !c.ok || c.got != c.want {
					t.Errorf("%s: %d (present %v), want %d", c.what, c.got, c.ok, c.want)
				}
			}
			if rs.Predicates["_0:dbl"] != nil {
				t.Errorf("_0:dbl is counted: the engine holds the declared volatile resource as _0:_0:dbl")
			}

			// And one count by hand: the dumps' lines whose predicate is rdf:type, in
			// the first rule set.
			var typeLines int64
			files, _ := filepath.Glob(filepath.Join(dumps, "r01", "native", "*", "01_*.tsv"))
			for _, file := range files {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				typeLines += int64(strings.Count(string(raw), "\trdf:type\t"))
			}
			first := f.ForRuleSet(ruleSets[0])
			if typeLines == 0 || first.Predicates["rdf:type"].Triples != typeLines {
				t.Errorf("rdf:type: %d triples counted, %d lines in the dumps", first.Predicates["rdf:type"].Triples, typeLines)
			}

			// Deterministic: a second run, its pool in another order, gives the same bytes.
			stats2 := filepath.Join(dir, "stats2.json")
			if _, err := runBench(t, path, Options{StatsFile: stats2}); err != nil {
				t.Fatal(err)
			}
			b1, _ := os.ReadFile(stats)
			b2, _ := os.ReadFile(stats2)
			if string(b1) != string(b2) {
				t.Errorf("two runs wrote different statistics:\n%s\n%s", b1, b2)
			}
			for _, o := range []Options{{StatsFile: stats, PerLoop: true}, {StatsFile: stats, SampleMs: 10}} {
				if _, err := runBench(t, path, o); err == nil || !strings.Contains(err.Error(), "statistics run is untimed") {
					t.Errorf("statistics with timing (%+v): %v", o, err)
				}
			}
		})
		// A session that fails contributes nothing, and a statistics run with a
		// failed counted session writes no file and returns an error: here the
		// runaway session stops at the vertex cap.
		t.Run("stats_refuses_a_failed_session", func(t *testing.T) {
			path := writeStatsBench(t, optimizerStatsSequence, []string{"ok", "runaway", "ok"}, `, "max_vertex_visits": 1000`)
			stats := filepath.Join(t.TempDir(), "stats.json")
			results, err := runBench(t, path, Options{StatsFile: stats})
			if err == nil || !strings.Contains(err.Error(), "1 counted session(s) failed") {
				t.Errorf("a failed session: %v", err)
			}
			if _, serr := os.Stat(stats); !os.IsNotExist(serr) {
				t.Errorf("the statistics file was written: %v", serr)
			}
			if len(results) != 1 || results[0].Status != "error" {
				t.Errorf("results %d, want the one run, its status error", len(results))
			}
		})
		t.Run("an_after_extraction_error_is_the_sessions", func(t *testing.T) {
			path := writeBench(t, 2, `, "max_vertex_visits": 1000`)
			results, err := runBench(t, path, Options{AfterExtraction: func(p DumpPoint) error {
				if p.Session == "case2" {
					return fmt.Errorf("disk full")
				}
				return nil
			}})
			if err != nil || len(results) != 1 {
				t.Fatalf("%d results, %v", len(results), err)
			}
			for _, s := range results[0].Sessions {
				if want := s.Name == "case2"; strings.Contains(s.Err, "disk full") != want {
					t.Errorf("session %s: error %q", s.Name, s.Err)
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
		// One retraction behaviour (rules_benchmark Q-95, Q-102): a spec's legacy
		// object with its switches false is accepted, timed or not, and selects
		// nothing; a switch set true is refused naming the deletion. Every result
		// records legacy_mode fixed, and an untimed run refuses the timing options.
		t.Run("one_retraction_behaviour", func(t *testing.T) {
			for _, legacy := range []string{``, `, "legacy": {}`, `, "legacy": {"recompute_on_retraction": false}`,
				`, "legacy": {"recompute_on_retraction": false, "withdrawn_row_stays": false}`} {
				path := writeBench(t, 1, `, "max_vertex_visits": 1000`+legacy)
				for _, o := range []Options{{}, {EquivalenceOnly: true}} {
					results, err := runBench(t, path, o)
					if err != nil {
						t.Fatalf("%q %+v: %v", legacy, o, err)
					}
					r := results[0]
					if r.Status != "ok" || r.LegacyMode != rulestest.LegacyModeFixed || r.EquivalenceOnly != o.EquivalenceOnly || r.Totals.Loops != 3 {
						t.Errorf("%q %+v: status %s mode %s equivalence_only %v loops %d", legacy, o, r.Status, r.LegacyMode, r.EquivalenceOnly, r.Totals.Loops)
					}
				}
				for _, o := range []Options{{EquivalenceOnly: true, PerLoop: true}, {EquivalenceOnly: true, SampleMs: 10}} {
					if _, err := runBench(t, path, o); err == nil || !strings.Contains(err.Error(), "untimed") {
						t.Errorf("%q untimed with timing (%+v): %v", legacy, o, err)
					}
				}
			}
			for _, legacy := range []string{`{"recompute_on_retraction": true}`, `{"withdrawn_row_stays": true}`} {
				path := writeBench(t, 1, `, "max_vertex_visits": 1000, "legacy": `+legacy)
				if _, err := LoadSpec(path); err == nil || !strings.Contains(err.Error(), rulestest.LegacyRetractionDeleted) {
					t.Errorf("%s: %v, want the spec refused naming the deletion", legacy, err)
				}
			}
		})
		// I-20: the unit-test harness reads _0: columns, and a benchmark refuses one.
		t.Run("local_column_refused", func(t *testing.T) {
			path := writeBench(t, 1, `, "max_vertex_visits": 1000`)
			csv := filepath.Join(filepath.Dir(path), "case1.csv")
			if err := os.WriteFile(csv, []byte("jets:key,rdf:type,xr:mode,_0:n as int\ncase1,xr:Case,ok,3\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := runBench(t, path, Options{}); err == nil || !strings.Contains(err.Error(), "rule-local (_0:) property") {
				t.Errorf("a _0: column: %v", err)
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
			// other phase; the timing callbacks' own time is counters_ns, apart too.
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
				"r01 native case1 0 jet_rules/test_looping_main.jr", "r01 native case2 0 jet_rules/test_looping_main.jr",
				"r02 native case1 0 jet_rules/test_looping_main.jr", "r02 native case2 0 jet_rules/test_looping_main.jr",
			}
			if strings.Join(calls, "|") != strings.Join(want, "|") {
				t.Errorf("hook calls %v, want %v", calls, want)
			}
			s := results[0].Sessions[0]
			if s.DumpNs < delay.Nanoseconds() || s.RuleSetsNs >= delay.Nanoseconds() || s.TotalNs >= delay.Nanoseconds() {
				t.Errorf("dump_ns %d, rule_sets_ns %d, total_ns %d: the hook's %v must be in dump_ns only", s.DumpNs, s.RuleSetsNs, s.TotalNs, delay)
			}
			if s.CountersNs < 0 || s.CountersNs >= delay.Nanoseconds() {
				t.Errorf("counters_ns %d, want the callbacks' own time, without the hook's %v", s.CountersNs, delay)
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

func keySet[V any](m map[string]V) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// wantedFromWorkspaceDb is what the statistics of ruleSet must hold, read from
// workspace.db's antecedents with the rules' own kinds and values, not from the
// engine.
func wantedFromWorkspaceDb(t *testing.T, ruleSet string) *optstats.Wanted {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+workspace.WorkspaceDbPath()+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT coalesce(s.type, ''), coalesce(s.value, ''), coalesce(p.type, ''), coalesce(p.value, ''),
	    coalesce(o.type, ''), coalesce(o.value, '')
	  FROM rete_nodes rn JOIN workspace_control wc ON wc.key = rn.source_file_key
	  LEFT JOIN resources s ON s.key = rn.subject_key
	  LEFT JOIN resources p ON p.key = rn.predicate_key
	  LEFT JOIN resources o ON o.key = rn.object_key
	  WHERE rn.type = 'antecedent' AND wc.source_file_name = ?`, ruleSet)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	w := optstats.NewWanted()
	// workspace.db holds a volatile resource with one _0: trimmed and the engine
	// always prefixes it, so it is rendered "_0:"+value, never by TermKey's
	// add-when-absent form, which is for the compiler's value.
	key := func(kind, value string) string {
		if kind == optstats.KindVolatileResource {
			kind, value = optstats.KindResource, optstats.VolatilePrefix+value
		}
		k, _ := optstats.TermKey(kind, value)
		return k
	}
	for rows.Next() {
		var st, sv, pt, pv, ot, ov string
		if err := rows.Scan(&st, &sv, &pt, &pv, &ot, &ov); err != nil {
			t.Fatal(err)
		}
		w.AddAntecedent(key(st, sv), key(pt, pv), key(ot, ov))
	}
	if len(w.Predicates) == 0 {
		t.Fatalf("no antecedent of %s in workspace.db", ruleSet)
	}
	return w
}

// countDumps counts, for w, the dumps of one rule set under dir, the files
// pattern names in each session's folder, from their lines: the dump writes resources bare, text quoted and an
// int as N^^int, which are the statistics' keys.
func countDumps(t *testing.T, dir, pattern string, w *optstats.Wanted) *optstats.RuleSet {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*", pattern))
	if err != nil || len(files) == 0 {
		t.Fatalf("no dump under %s: %v", dir, err)
	}
	acc := optstats.NewAccumulator()
	for _, file := range files {
		fd, err := os.Open(file)
		if err != nil {
			t.Fatal(err)
		}
		var g [][3]string
		sc := bufio.NewScanner(fd)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			parts := strings.Split(sc.Text(), "\t")
			if len(parts) != 3 {
				t.Fatalf("%s: %q", file, sc.Text())
			}
			g = append(g, [3]string(parts))
		}
		fd.Close()
		present := map[string]bool{}
		for _, tr := range g {
			present[tr[0]], present[tr[1]], present[tr[2]] = true, true, true
		}
		c, err := optstats.NewCounter(w, func(k string) (any, bool) { return k, present[k] })
		if err != nil {
			t.Fatal(err)
		}
		for _, tr := range g {
			c.Add(tr[0], tr[1], tr[2])
		}
		if err := acc.Add("x", c.Session()); err != nil {
			t.Fatal(err)
		}
	}
	return acc.File().RuleSets["x"]
}

// The optimizer statistics tests' rule set, run after test_execute_rules_main.jr
// in optimizerStatsSequence (both added to the workspace copy only). Its
// antecedents hold a text constant ("done", which test_execute_rules_main.jr
// asserts for mode ok), a subject constant (jets:iState), a negated antecedent on
// a predicate no ok session holds (xr:count, a negated term's variables being
// bound earlier, so with a constant object), and a volatile resource declared
// with its prefix, which the engine names _0:_0:dbl.
const (
	optimizerStatsRuleFile = "jet_rules/test_optimizer_stats_main.jr"
	optimizerStatsSequence = "test_optimizer_stats_sequence"
	optimizerStatsRules    = `import "data_model/jets_model.jr"
import "data_model/test_execute_rules_model.jr"

volatile_resource dbl = "_0:dbl";
volatile_resource seen = "seen";

[OS_First]:
  (?c rdf:type xr:Case).
  (?c xr:result "done").
  not(?c xr:count 0)
  ->
  (?c dbl "first")
;

[OS_State]:
  (jets:iState rdf:type ?t).
  (?c dbl ?v)
  ->
  (?c seen ?v)
;
`
)

// writeStatsBench writes a spec running process over one xr:Case session per
// mode, as test_execute_rules_main.jr reads xr:mode.
func writeStatsBench(t *testing.T, process string, modes []string, extra string) string {
	t.Helper()
	dir := t.TempDir()
	var sessions []string
	for i, mode := range modes {
		name := fmt.Sprintf("case%d", i+1)
		csv := "jets:key,rdf:type,xr:mode\n" + name + ",xr:Case," + mode + "\n"
		if err := os.WriteFile(filepath.Join(dir, name+".csv"), []byte(csv), 0o644); err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, `{"name": "`+name+`", "input": ["`+name+`.csv"]}`)
	}
	spec := `{
	  "bench_name": "stats",
	  "jetrules_config": {"process_name": "` + process + `", "current_source_period": 1},
	  "input_sessions": [` + strings.Join(sessions, ",") + `],
	  "engines": ["native"],
	  "output_classes": [{"class_name": "xr:Case", "columns": []}]` + extra + `
	}`
	path := filepath.Join(dir, "bench.json")
	if err := os.WriteFile(path, []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
