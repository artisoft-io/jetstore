package compute_pipes_test

// Tests of the two entry points the rules unit-test harness calls:
// RunRuleSets, the rule loop extracted from executeRules, and AssertTextRow,
// typed assertion of a row of text cells (rules_unit_test_harness Phase 1,
// task AA.2).
//
// jetrules_execute_rules_test.go pins what executeRules does and is unchanged
// by the extraction; this file states what the harness may rely on in the
// outcome RunRuleSets returns. It runs the same way, in a child process over a
// compiled copy of jets/jetrules/test_ws, for the same reason: package rete
// fixes its workspace at init. Run it with -count=1.

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const ruleSetsChildEnv = "JETS_RUN_RULE_SETS_TEST_HOME"

// inCompiledTestWs runs test in a child process whose WORKSPACES_HOME holds a
// compiled copy of test_ws.
func inCompiledTestWs(t *testing.T, test func(t *testing.T)) {
	t.Helper()
	if os.Getenv(ruleSetsChildEnv) != "" {
		if _, err := os.Stat(filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws", "build")); err != nil {
			if out, err := workspace.CompileWorkspace(nil, "test_ws", "aa2",
				workspace.SkipAssetValidation("RunRuleSets tests: test_ws has no user flows")); err != nil {
				t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
			}
		}
		test(t)
		return
	}
	wd, err := os.Getwd() // jets/compute_pipes
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
		ruleSetsChildEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed in the child process: %v\n%s", t.Name(), err, testLines(out))
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", testLines(out))
	}
}

func testLines(out []byte) string {
	var keep []string
	for line := range strings.SplitSeq(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "===") ||
			strings.Contains(line, "_test.go:") || strings.HasPrefix(trimmed, "FAIL") ||
			strings.HasPrefix(trimmed, "PASS") || strings.HasPrefix(trimmed, "panic") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// goSession builds the Go engine for process and opens one rdf session on it.
func goSession(t *testing.T, process string) (compute_pipes.JetRuleEngine, compute_pipes.JetRdfSession) {
	t.Helper()
	engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, process, false)
	if err != nil {
		t.Fatalf("building the Go engine for %s: %v", process, err)
	}
	t.Cleanup(func() { engine.Release() })
	s, err := engine.NewRdfSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Release() })
	return engine, s
}

var caseColumns = []string{"jets:key", "rdf:type", "xr:mode"}

func assertCase(mode string) func(compute_pipes.JetRdfSession) error {
	spec := &compute_pipes.JetrulesSpec{CurrentSourcePeriod: 1}
	return func(s compute_pipes.JetRdfSession) error {
		return compute_pipes.AssertTextRow(spec, s, caseColumns, []string{"case-" + mode, "xr:Case", mode})
	}
}

func runRuleSets(t *testing.T, process, mode string, config *compute_pipes.RuleSetsConfig) compute_pipes.RuleSetsOutcome {
	t.Helper()
	engine, s := goSession(t, process)
	return compute_pipes.RunRuleSets(s, engine, config, assertCase(mode))
}

// processEngine is the Go engine under another process name.
type processEngine struct {
	compute_pipes.JetRuleEngine
	process string
}

func (e processEngine) MainRuleFile() string { return e.process }

// resourcesOrderEngine records the engine calls that create JetResources and
// open an rdf session.
type resourcesOrderEngine struct {
	compute_pipes.JetRuleEngine
	calls *[]string
}

func (e resourcesOrderEngine) JetResources() *compute_pipes.JetResources {
	*e.calls = append(*e.calls, "JetResources")
	return e.JetRuleEngine.JetResources()
}

func (e resourcesOrderEngine) NewRdfSession() (compute_pipes.JetRdfSession, error) {
	*e.calls = append(*e.calls, "NewRdfSession")
	return e.JetRuleEngine.NewRdfSession()
}

// orderSession records each rete session created.
type orderSession struct {
	compute_pipes.JetRdfSession
	calls *[]string
}

func (s orderSession) NewReteSession(ruleset string) (compute_pipes.JetReteSession, error) {
	*s.calls = append(*s.calls, "NewReteSession "+ruleset)
	return s.JetRdfSession.NewReteSession(ruleset)
}

// releaseCountingSession counts the rete sessions created through it and the
// ones released. ruleset, when not empty, is created in place of the rule set
// asked for, so that a process name can lead RunRuleSets to a rule set's
// $max_looping while the engine builds a rete session it has.
type releaseCountingSession struct {
	compute_pipes.JetRdfSession
	ruleset           string
	created, released *int
}

func (s releaseCountingSession) NewReteSession(ruleset string) (compute_pipes.JetReteSession, error) {
	if s.ruleset != "" {
		ruleset = s.ruleset
	}
	rs, err := s.JetRdfSession.NewReteSession(ruleset)
	if err != nil {
		return nil, err
	}
	*s.created++
	return releaseCountingRete{JetReteSession: rs, released: s.released}, nil
}

type releaseCountingRete struct {
	compute_pipes.JetReteSession
	released *int
}

func (r releaseCountingRete) Release() error {
	*r.released++
	return r.JetReteSession.Release()
}

// replaceRuleEngineConfig writes content as the compiled .config.json of
// ruleFile in the scratch workspace, and puts the original back afterwards.
func replaceRuleEngineConfig(t *testing.T, ruleFile, content string) {
	t.Helper()
	path := filepath.Join(os.Getenv("WORKSPACES_HOME"), "test_ws", "build",
		strings.TrimSuffix(ruleFile, ".jr")+".config.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the compiled config of %s: %v", ruleFile, err)
	}
	t.Cleanup(func() { os.WriteFile(path, original, 0644) })
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRunRuleSets(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		// test_looping_main.jr: $max_looping 5, jets:completed on loop 3.
		t.Run("completes", func(t *testing.T) {
			o := runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{})
			if o.Err != nil || !o.Completed || o.Loops != 3 || o.MaxLoopReached || o.ExecuteErr != nil || o.Failed() {
				t.Errorf("outcome = %+v, want completed on loop 3 with nothing failed", o)
			}
			if len(o.RuleSets) != 1 || o.RuleSets[0].RuleSet != loopingRuleFile || o.RuleSets[0].MaxLooping != 5 {
				t.Errorf("rule sets = %+v, want test_looping_main.jr with its $max_looping of 5", o.RuleSets)
			}
		})
		t.Run("max_looping_reached", func(t *testing.T) {
			o := runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{MaxLooping: 2})
			if o.Completed || o.Loops != 2 || !o.MaxLoopReached || !o.Failed() {
				t.Errorf("outcome = %+v, want not completed, loop 2 the last run, the maximum reached", o)
			}
		})
		// I-7, fixed by AA.3: completion or an error on the last allowed loop is not
		// the maximum reached.
		t.Run("I7_completion_on_last_allowed_loop_not_max_loop", func(t *testing.T) {
			o := runRuleSets(t, loopingRuleFile, "ok", &compute_pipes.RuleSetsConfig{MaxLooping: 3})
			if !o.Completed || o.Loops != 3 || o.MaxLoopReached || o.Failed() {
				t.Errorf("outcome = %+v, want completed on loop 3 and nothing failed (I-7)", o)
			}
		})
		t.Run("I7_execute_rules_error_on_last_allowed_loop_not_max_loop", func(t *testing.T) {
			o := runRuleSets(t, executeRulesRuleFile, "late_runaway", &compute_pipes.RuleSetsConfig{})
			if o.ExecuteErr == nil || o.Loops != 2 || o.MaxLoopReached {
				t.Errorf("outcome = %+v, want the ExecuteRules error on loop 2 and not the maximum (I-7)", o)
			}
		})
		t.Run("execute_rules_error", func(t *testing.T) {
			o := runRuleSets(t, executeRulesRuleFile, "runaway", &compute_pipes.RuleSetsConfig{})
			if o.ExecuteErr == nil || o.ExecuteErr.Error() != "error: max vertex visit reached" {
				t.Errorf("ExecuteErr = %v, want the max vertex visit error", o.ExecuteErr)
			}
			if o.Err != nil || o.Completed || o.Loops != 0 || o.MaxLoopReached || !o.Failed() {
				t.Errorf("outcome = %+v, want stopped on loop 0, not completed, no maximum", o)
			}
		})
		t.Run("exception", func(t *testing.T) {
			o := runRuleSets(t, executeRulesRuleFile, "exception", &compute_pipes.RuleSetsConfig{})
			if !slices.Equal(o.Exceptions, []string{"xr exception raised"}) || !o.Completed || !o.Failed() {
				t.Errorf("outcome = %+v, want completed with one exception", o)
			}
		})
		t.Run("sequence", func(t *testing.T) {
			// AfterRuleSet sees the session as each rule set left it, which is what a
			// saved rete session records: no lp:Person after the first rule set, the
			// three test_looping_main.jr creates after the second.
			var after []string
			o := runRuleSets(t, executeRulesSequence, "exception", &compute_pipes.RuleSetsConfig{
				AfterRuleSet: func(s compute_pipes.JetRdfSession, rs *compute_pipes.RuleSetOutcome) {
					rm := s.GetResourceManager()
					persons := 0
					it := s.FindSPO(nil, rm.NewResource("rdf:type"), rm.NewResource("lp:Person"))
					for ; !it.IsEnd(); it.Next() {
						persons++
					}
					it.Release()
					after = append(after, rs.RuleSet+" persons="+strconv.Itoa(persons))
				},
			})
			want := []string{executeRulesRuleFile + " persons=0", loopingRuleFile + " persons=3"}
			if !slices.Equal(after, want) {
				t.Errorf("AfterRuleSet called with %q, want %q", after, want)
			}
			if len(o.RuleSets) != 2 || o.RuleSets[0].MaxLooping != 2 || o.RuleSets[1].MaxLooping != 5 {
				t.Errorf("rule sets = %+v, want two, each with its own $max_looping", o.RuleSets)
			}
			// I-8: the exception stays in the session for the second rule set, and only
			// the first, which raised it, reports it.
			if !o.Completed || o.Loops != 3 || !slices.Equal(o.Exceptions, []string{"xr exception raised"}) {
				t.Errorf("outcome = %+v, want completed, the last rule set on loop 3, the exception once", o)
			}
			if len(o.RuleSets) == 2 && (len(o.RuleSets[0].Exceptions) != 1 || len(o.RuleSets[1].Exceptions) != 0) {
				t.Errorf("rule set exceptions = %q and %q, want the exception in the first only",
					o.RuleSets[0].Exceptions, o.RuleSets[1].Exceptions)
			}
		})
		// I-8: two distinct exceptions from two rule sets, once each; the second rule
		// set raising the first exception again is the same triple and reports nothing.
		t.Run("two_exceptions_once_each", func(t *testing.T) {
			o := runRuleSets(t, twoExceptionsSequence, "exception", &compute_pipes.RuleSetsConfig{})
			want := []string{"xr exception raised", "xr second exception raised"}
			if o.Err != nil || !slices.Equal(o.Exceptions, want) || !o.Failed() {
				t.Errorf("outcome = %+v, want the exceptions %q", o, want)
			}
			if len(o.RuleSets) != 2 ||
				!slices.Equal(o.RuleSets[0].Exceptions, want[:1]) || !slices.Equal(o.RuleSets[1].Exceptions, want[1:]) {
				t.Errorf("rule sets = %+v, want each exception in the rule set that raised it", o.RuleSets)
			}
		})
		t.Run("rete_session_created_before_input_asserted", func(t *testing.T) {
			engine, s := goSession(t, executeRulesSequence)
			var calls []string
			o := compute_pipes.RunRuleSets(orderSession{JetRdfSession: s, calls: &calls}, engine,
				&compute_pipes.RuleSetsConfig{}, func(s compute_pipes.JetRdfSession) error {
					calls = append(calls, "assertInput")
					return assertCase("ok")(s)
				})
			want := []string{"NewReteSession " + executeRulesRuleFile, "assertInput", "NewReteSession " + loopingRuleFile}
			if o.Err != nil || !slices.Equal(calls, want) {
				t.Errorf("calls = %q (err %v), want %q", calls, o.Err, want)
			}
		})
		// executeRules creates the engine's JetResources before it opens the rdf
		// session, because on C++ the first creation after a session exists aborts
		// the process (I-15). Recorded through a wrapping engine.
		t.Run("executeRules_creates_jet_resources_before_the_rdf_session", func(t *testing.T) {
			engine, _ := goSession(t, loopingRuleFile)
			var calls []string
			wrapped := resourcesOrderEngine{JetRuleEngine: engine, calls: &calls}
			config := &compute_pipes.JetrulesSpec{OnError: compute_pipes.OnErrorPassThrough, MaxErrorCount: 20, CurrentSourcePeriod: 1}
			done, errCh := make(chan struct{}), make(chan error, 1)
			worker := (&compute_pipes.BuilderContext{}).NewJrPoolWorker(config,
				&compute_pipes.InputChannel{Name: "input", HasGroupedRows: true},
				map[string][]string{"xr:Case": {"jets:key", "rdf:type", "jets:source_period_sequence", "xr:mode"}},
				wrapped, nil, nil, done, errCh)
			bundle := []any{[]any{"case-ok", []any{"xr:Case"}, nil, "ok"}}
			if err := worker.ExecuteRulesForTest(&bundle, make(chan compute_pipes.JetrulesWorkerResult, 1)); err != nil {
				t.Fatal(err)
			}
			if len(calls) < 2 || calls[0] != "JetResources" || calls[1] != "NewRdfSession" {
				t.Errorf("engine calls = %q, want JetResources before NewRdfSession", calls)
			}
		})
		// I-10: every rete session RunRuleSets creates is released, on the paths that
		// end the rule sets early as on the one that runs them all.
		t.Run("rete_session_released_on_every_path", func(t *testing.T) {
			for _, c := range []struct {
				name, process, ruleset string
				setup                  func(t *testing.T)
				assertInput            func(compute_pipes.JetRdfSession) error
				wantErr                string
			}{
				{name: "sequence_run_to_the_end", process: executeRulesSequence, assertInput: assertCase("ok")},
				{name: "input_not_asserted", process: loopingRuleFile,
					assertInput: func(compute_pipes.JetRdfSession) error { return errors.New("bad row") },
					wantErr:     "while asserting input records to rdf session: bad row"},
				// The rete session is test_looping_main.jr's; the $max_looping read is
				// that of the rule set named by the process, whose config is broken.
				{name: "max_looping_unreadable", process: "jet_rules/test_join_values_main.jr", ruleset: loopingRuleFile,
					setup: func(t *testing.T) {
						replaceRuleEngineConfig(t, "jet_rules/test_join_values_main.jr", "{not json")
					},
					assertInput: assertCase("ok"),
					wantErr:     "error: while getting '$max_looping' property from workspace jet_rules/test_join_values_main.jr"},
				{name: "max_looping_invalid", process: "jet_rules/test_lookup_main.jr", ruleset: loopingRuleFile,
					setup: func(t *testing.T) {
						replaceRuleEngineConfig(t, "jet_rules/test_lookup_main.jr", `{"jetstore_config": {"$max_looping": "five"}}`)
					},
					assertInput: assertCase("ok"),
					wantErr:     "error: invalid '$max_looping' property in workspace jet_rules/test_lookup_main.jr"},
			} {
				t.Run(c.name, func(t *testing.T) {
					if c.setup != nil {
						c.setup(t)
					}
					engineProcess := c.process
					if c.ruleset != "" {
						engineProcess = c.ruleset
					}
					engine, s := goSession(t, engineProcess)
					var created, released int
					o := compute_pipes.RunRuleSets(
						releaseCountingSession{JetRdfSession: s, ruleset: c.ruleset, created: &created, released: &released},
						processEngine{engine, c.process}, &compute_pipes.RuleSetsConfig{}, c.assertInput)
					switch {
					case c.wantErr == "" && o.Err != nil:
						t.Fatalf("RunRuleSets: %v", o.Err)
					case c.wantErr != "" && (o.Err == nil || !strings.HasPrefix(o.Err.Error(), c.wantErr)):
						t.Fatalf("RunRuleSets error = %v, want one starting %q", o.Err, c.wantErr)
					}
					if created == 0 || released != created {
						t.Errorf("rete sessions created %d, released %d; want every one created released", created, released)
					}
				})
			}
		})
		t.Run("no_rule_set", func(t *testing.T) {
			engine, s := goSession(t, loopingRuleFile)
			o := compute_pipes.RunRuleSets(s, processEngine{engine, "no_such_process"},
				&compute_pipes.RuleSetsConfig{}, assertCase("ok"))
			if o.Err == nil || o.Err.Error() != "error: no rulesets found for main rule file name no_such_process" || o.Completed {
				t.Errorf("outcome = %+v, want the no-ruleset error and not completed", o)
			}
		})
		t.Run("input_not_asserted", func(t *testing.T) {
			engine, s := goSession(t, loopingRuleFile)
			o := compute_pipes.RunRuleSets(s, engine, &compute_pipes.RuleSetsConfig{},
				func(compute_pipes.JetRdfSession) error { return errors.New("bad row") })
			if o.Err == nil || o.Err.Error() != "while asserting input records to rdf session: bad row" ||
				len(o.RuleSets) != 0 || o.Completed {
				t.Errorf("outcome = %+v, want the assertion error and no rule set run", o)
			}
		})
	})
}

func TestAssertTextRow(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		spec := &compute_pipes.JetrulesSpec{CurrentSourcePeriod: 1}
		value := func(s compute_pipes.JetRdfSession, subject, predicate string) any {
			rm := s.GetResourceManager()
			n := s.GetObject(rm.NewResource(subject), rm.NewResource(predicate))
			if n == nil {
				return nil
			}
			return compute_pipes.GetRdfNodeValue(n)
		}
		has := func(s compute_pipes.JetRdfSession, subject, predicate, object string) bool {
			rm := s.GetResourceManager()
			return s.Contains(rm.NewResource(subject), rm.NewResource(predicate), rm.NewResource(object))
		}

		t.Run("columns_by_name_cast_by_property", func(t *testing.T) {
			_, s := goSession(t, loopingRuleFile)
			// No jets:source_period_sequence, and the columns out of assertInputRow's order.
			err := compute_pipes.AssertTextRow(spec, s,
				[]string{"xr:mode", "rdf:type", "xr:count", "jets:key", "jets:currentSourcePeriodDate"},
				[]string{"ok", "xr:Case", "5", "k1", "2024-01-01T00:00:00"})
			if err != nil {
				t.Fatal(err)
			}
			if !has(s, "k1", "rdf:type", "xr:Case") || !has(s, "k1", "rdf:type", "jets:InputRecord") {
				t.Errorf("k1 is not typed xr:Case and jets:InputRecord")
			}
			if got := value(s, "k1", "xr:mode"); got != "ok" {
				t.Errorf("xr:mode = %#v, want text ok", got)
			}
			if got := value(s, "k1", "xr:count"); got != 5 {
				t.Errorf("xr:count = %#v, want int 5, cast by properties.json", got)
			}
			if got := value(s, "k1", "jets:source_period_sequence"); got != 0 {
				t.Errorf("jets:source_period_sequence = %#v, want 0, the current period", got)
			}
			rm := s.GetResourceManager()
			n := s.GetObject(rm.NewResource("k1"), rm.NewResource("jets:currentSourcePeriodDate"))
			if n == nil || n.Type() != "date" {
				t.Errorf("a date property at midnight = %v, want a date literal", n)
			} else if got := compute_pipes.GetRdfNodeValue(n); got != time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) {
				t.Errorf("date value = %#v, want 2024-01-01", got)
			}
		})
		// assertInputRow reads position 2 only as a Go int; anything else counts as
		// -1, the current period, and adds the jets:InputRecord marker. A period of 1
		// in a row of current period 1 is therefore 0 and unmarked only if the cell
		// cast to int.
		t.Run("source_period_sequence_casts_to_int", func(t *testing.T) {
			_, s := goSession(t, loopingRuleFile)
			if err := compute_pipes.AssertTextRow(spec, s, []string{"jets:key", "rdf:type", "jets:source_period_sequence"},
				[]string{"k4", "xr:Case", "1"}); err != nil {
				t.Fatal(err)
			}
			if got := value(s, "k4", "jets:source_period_sequence"); got != 0 {
				t.Errorf("jets:source_period_sequence = %#v, want 0, the row's period 1 read as an int", got)
			}
			if has(s, "k4", "rdf:type", "jets:InputRecord") {
				t.Errorf("k4 is marked jets:InputRecord, which assertInputRow does when the period is not an int")
			}
		})
		t.Run("blank_cell_asserts_nothing", func(t *testing.T) {
			_, s := goSession(t, loopingRuleFile)
			if err := compute_pipes.AssertTextRow(spec, s, []string{"jets:key", "rdf:type", "xr:count"},
				[]string{"k2", "xr:Case", " "}); err != nil {
				t.Fatal(err)
			}
			if got := value(s, "k2", "xr:count"); got != nil {
				t.Errorf("xr:count = %#v, want nothing for a blank cell", got)
			}
		})
		t.Run("key_absent_is_a_hash_and_type_from_config", func(t *testing.T) {
			_, s := goSession(t, loopingRuleFile)
			typed := &compute_pipes.JetrulesSpec{CurrentSourcePeriod: 1, InputRdfType: "xr:Case"}
			if err := compute_pipes.AssertTextRow(typed, s, []string{"xr:mode"}, []string{"ok"}); err != nil {
				t.Fatal(err)
			}
			rm := s.GetResourceManager()
			it := s.FindSPO(nil, rm.NewResource("rdf:type"), rm.NewResource("xr:Case"))
			defer it.Release()
			if it.IsEnd() {
				t.Fatalf("no xr:Case asserted")
			}
			subject := it.GetSubject()
			if key := s.GetObject(subject, rm.NewResource("jets:key")); key == nil || key.String() != subject.String() || key.String() == "" {
				t.Errorf("jets:key = %v for subject %v, want the generated key", key, subject)
			}
		})
		t.Run("refusals", func(t *testing.T) {
			_, s := goSession(t, loopingRuleFile)
			for _, c := range []struct {
				columns, cells []string
				want           string
			}{
				{[]string{"jets:key", "rdf:type", "xr:mdoe"}, []string{"k3", "xr:Case", "ok"}, "column 'xr:mdoe' is not a data property"},
				{[]string{"jets:key", "rdf:type", "xr:count"}, []string{"k3", "xr:Case", "five"}, "column 'xr:count' value 'five' is not a int"},
				{[]string{"jets:key", "rdf:type"}, []string{"k3"}, "row has 1 cells for 2 columns"},
				{[]string{"jets:key", "xr:mode"}, []string{"k3", "ok"}, "input rdf:Type not provided"},
			} {
				err := compute_pipes.AssertTextRow(spec, s, c.columns, c.cells)
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Errorf("AssertTextRow(%q, %q) = %v, want an error containing %q", c.columns, c.cells, err, c.want)
				}
			}
		})
	})
}
