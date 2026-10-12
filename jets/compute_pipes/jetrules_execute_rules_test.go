package compute_pipes_test

// A characterisation test of JrPoolWorker.executeRules, on the Go engine.
//
// rules_unit_test_harness Phase 1, task AA.1. Nothing tested the rule loop
// before this file (that project's F77), and AA.2 extracts it into an exported
// function that pipelines and the rules unit-test harness both call. So this
// test pins what the loop does **today**, before it is moved, and it must pass
// unchanged across the extraction: a refactor that edits the test along with the
// code proves nothing.
//
// # What it pins
//
//   - a loop that completes before its maximum;
//   - a loop that reaches its maximum, from the rule file's $max_looping and from
//     the jetrules_config max_looping override;
//   - I-7, fixed by AA.3: completion, or an ExecuteRules error, on the last
//     allowed iteration is not reported as MAX LOOP REACHED. AA.1 pinned the
//     defect; AA.3 changed these cases in the commit that fixed it;
//   - an ExecuteRules error, the max vertex visit limit;
//   - a jets:exception, with and without a saved rete session;
//   - each on_error policy's effect on output, for each of the three failures;
//   - max_error_count, which stops the reporting and not the counting;
//   - a rule sequence reports each jets:exception once, from the rule set that
//     raised it, though the exception stays in the rdf session and every later
//     rule set's scan finds it. Until I-8 was fixed it was reported again by
//     every later rule set; the case was changed in the commit that fixed it;
//   - two distinct exceptions raised by two rule sets of a sequence are reported
//     once each, and a later rule set raising the first one again does not
//     report it a second time;
//   - the order: the first rule set's rete session is created before the input
//     is asserted. On the Go engine the order changes no outcome, so it is pinned
//     by recording the calls; on C++ it is load-bearing, because creating a
//     resource through the meta graph after the first rdf session opens aborts
//     the process (rules_unit_test_harness I-15).
//
// # How it runs
//
// The test copies jets/jetrules/test_ws to a scratch WORKSPACES_HOME and re-runs
// this test binary as a child process with WORKSPACES_HOME and WORKSPACE set.
// The child compiles the copy in process, then runs the cases. **It has to be a
// child process**: package rete reads WORKSPACES_HOME and WORKSPACE in its init,
// and they are unexported there, so nothing in this process can point the Go
// engine at a scratch workspace after it has started.
//
// The rule files are jet_rules/test_looping_main.jr, whose loop completes on
// loop 3 under a $max_looping of 5, and jet_rules/test_execute_rules_main.jr,
// added for this test, whose input entity's xr:mode selects the behaviour (see
// its header). The sequence test_execute_rules_sequence runs the second and then
// the first; test_execute_rules_two_exceptions runs test_execute_rules_main.jr and
// then jet_rules/test_execute_rules_second.jr (see its header).
//
// Run it with -count=1: the rules it compiles are read from disk, outside the
// Go module's cache key, so a cached ok survives a change to them.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

// characterisationHomeEnv marks the child process and carries its scratch home.
const characterisationHomeEnv = "JETS_EXECUTE_RULES_CHARACTERISATION_HOME"

const (
	loopingRuleFile       = "jet_rules/test_looping_main.jr"
	executeRulesRuleFile  = "jet_rules/test_execute_rules_main.jr"
	executeRulesSequence  = "test_execute_rules_sequence"
	twoExceptionsSequence = "test_execute_rules_two_exceptions"
)

func TestExecuteRulesCharacterisation(t *testing.T) {
	if home := os.Getenv(characterisationHomeEnv); home != "" {
		runCharacterisation(t, home)
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
	// The compiler builds its paths from workspace_control.json's workspace_name,
	// which the committed file does not carry; patched in the copy only, as
	// rerunOverCompiledTestWs in jets/jetrules/rete/retraction_mode_factory_test.go does.
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

	cmd := exec.Command(os.Args[0], "-test.run=^TestExecuteRulesCharacterisation$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(),
		"WORKSPACES_HOME="+home,
		"WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+filepath.Join(root, "jets", "workspace_schema.sql"),
		characterisationHomeEnv+"="+home,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the characterisation cases failed in the child process: %v\n%s", err, childFailures(out))
	}
	if testing.Verbose() {
		t.Logf("child process:\n%s", childFailures(out))
	}
}

// childFailures keeps the child's test lines and drops the engine's logging,
// which is several thousand lines for the runaway case.
func childFailures(out []byte) string {
	var keep []string
	for line := range strings.SplitSeq(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "===") ||
			strings.Contains(line, "_test.go:") || strings.HasPrefix(trimmed, "FAIL") ||
			strings.HasPrefix(trimmed, "PASS") || strings.HasPrefix(trimmed, "ok") ||
			strings.HasPrefix(trimmed, "panic") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// errorColumns are the process_errors columns the error channel carries here.
var errorColumns = []string{"error_message", "rete_session_saved", "rete_session_triples"}

// run is one call to executeRules and everything it wrote.
type run struct {
	err        error
	errorRows  []compute_pipes.ProcessError
	cases      [][]any // xr:Case rows: jets:key, xr:mode, xr:result
	persons    [][]any // lp:Person rows: lp:name
	results    []compute_pipes.JetrulesWorkerResult
	errChErrs  []error
	doneClosed bool
}

func (r *run) messages() []string {
	m := make([]string, 0, len(r.errorRows))
	for _, pe := range r.errorRows {
		m = append(m, pe.ErrorMessage)
	}
	return m
}

// recordingEngine wraps the Go engine so that the rdf sessions it opens record,
// in order, each rete session created and each triple inserted.
type recordingEngine struct {
	compute_pipes.JetRuleEngine
	calls *[]string
}

func (e recordingEngine) NewRdfSession() (compute_pipes.JetRdfSession, error) {
	s, err := e.JetRuleEngine.NewRdfSession()
	if err != nil {
		return nil, err
	}
	return recordingSession{JetRdfSession: s, calls: e.calls}, nil
}

type recordingSession struct {
	compute_pipes.JetRdfSession
	calls *[]string
}

func (s recordingSession) NewReteSession(ruleset string) (compute_pipes.JetReteSession, error) {
	*s.calls = append(*s.calls, "NewReteSession "+ruleset)
	return s.JetRdfSession.NewReteSession(ruleset)
}

func (s recordingSession) Insert(subject, predicate, object compute_pipes.RdfNode) error {
	*s.calls = append(*s.calls, "Insert "+predicate.String())
	return s.JetRdfSession.Insert(subject, predicate, object)
}

// harness holds one engine and one worker, so that max_error_count can be seen
// across bundles.
type harness struct {
	t       *testing.T
	calls   []string
	worker  *compute_pipes.JrPoolWorker
	errCh   chan []any
	caseCh  chan []any
	persCh  chan []any
	resCh   chan compute_pipes.JetrulesWorkerResult
	errsCh  chan error
	done    chan struct{}
	errCols map[string]int
}

func newHarness(t *testing.T, process string, config *compute_pipes.JetrulesSpec) *harness {
	t.Helper()
	engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, process, false)
	if err != nil {
		t.Fatalf("building the Go engine for %s: %v", process, err)
	}
	t.Cleanup(func() { engine.Release() })

	// What NewJetrulesTransformationPipe defaults before building a worker.
	if config.OnError == "" {
		config.OnError = compute_pipes.OnErrorPassThrough
	}
	if config.MaxErrorCount < 1 {
		config.MaxErrorCount = 20
	}
	config.ProcessName = process
	config.CurrentSourcePeriod = 1

	h := &harness{
		t:      t,
		errCh:  make(chan []any, 100),
		caseCh: make(chan []any, 100),
		persCh: make(chan []any, 100),
		resCh:  make(chan compute_pipes.JetrulesWorkerResult, 10),
		errsCh: make(chan error, 10),
		done:   make(chan struct{}),
	}
	h.errCols = make(map[string]int)
	for i, c := range errorColumns {
		h.errCols[c] = i
	}
	errorOut := &compute_pipes.OutputChannel{
		Name:    "process_errors",
		Channel: h.errCh,
		Columns: &h.errCols,
		Config:  &compute_pipes.ChannelSpec{Name: "process_errors", Columns: errorColumns},
	}
	caseCols := []string{"jets:key", "xr:mode", "xr:result"}
	personCols := []string{"lp:name"}
	outputs := []*compute_pipes.JetrulesOutputChan{
		{
			ClassName: "xr:Case",
			OutputCh: &compute_pipes.OutputChannel{
				Name: "cases", Channel: h.caseCh,
				Config: &compute_pipes.ChannelSpec{Name: "cases", ClassName: "xr:Case", Columns: caseCols},
			},
		},
		{
			ClassName: "lp:Person",
			OutputCh: &compute_pipes.OutputChannel{
				Name: "persons", Channel: h.persCh,
				Config: &compute_pipes.ChannelSpec{Name: "persons", ClassName: "lp:Person", Columns: personCols},
			},
		},
	}
	source := &compute_pipes.InputChannel{Name: "input", HasGroupedRows: true}
	rdfType2Columns := map[string][]string{
		"xr:Case": {"jets:key", "rdf:type", "jets:source_period_sequence", "xr:mode"},
	}
	h.worker = (&compute_pipes.BuilderContext{}).NewJrPoolWorker(config, source, rdfType2Columns,
		recordingEngine{JetRuleEngine: engine, calls: &h.calls}, errorOut, outputs, h.done, h.errsCh)
	return h
}

// execute runs one bundle holding one xr:Case with the given mode.
func (h *harness) execute(mode string) *run {
	h.t.Helper()
	bundle := []any{[]any{"case-" + mode, []any{"xr:Case"}, nil, mode}}
	r := &run{}
	r.err = h.worker.ExecuteRulesForTest(&bundle, h.resCh)
	for len(h.errCh) > 0 {
		row := <-h.errCh
		pe := compute_pipes.ProcessError{}
		pe.ErrorMessage, _ = row[h.errCols["error_message"]].(string)
		pe.ReteSessionSaved, _ = row[h.errCols["rete_session_saved"]].(string)
		pe.ReteSessionTriples, _ = row[h.errCols["rete_session_triples"]].(sql.NullString)
		r.errorRows = append(r.errorRows, pe)
	}
	for len(h.caseCh) > 0 {
		r.cases = append(r.cases, <-h.caseCh)
	}
	for len(h.persCh) > 0 {
		r.persons = append(r.persons, <-h.persCh)
	}
	for len(h.resCh) > 0 {
		r.results = append(r.results, <-h.resCh)
	}
	for len(h.errsCh) > 0 {
		r.errChErrs = append(r.errChErrs, <-h.errsCh)
	}
	select {
	case <-h.done:
		r.doneClosed = true
	default:
	}
	return r
}

func runOnce(t *testing.T, process, mode string, config *compute_pipes.JetrulesSpec) *run {
	t.Helper()
	return newHarness(t, process, config).execute(mode)
}

// The messages executeRules writes, as it writes them.
const (
	msgExecuteRulesError = "ExecuteRules returned error: error: max vertex visit reached"
	msgException         = "jets:exception caught: xr exception raised"
	msgSecondException   = "jets:exception caught: xr second exception raised"
	msgOnErrorFail       = "jetrules: record bundle failed rule execution (on_error: fail)"
)

func msgMaxLoop(n string) string { return "MAX LOOP REACHED, maxLooping is " + n }

func assertMessages(t *testing.T, r *run, want ...string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got := r.messages(); !slices.Equal(got, want) {
		t.Errorf("error channel messages = %q, want %q", got, want)
	}
}

// assertBundleKept is the success path: no error returned, nothing on errCh, done open.
func assertBundleKept(t *testing.T, r *run) {
	t.Helper()
	if r.err != nil {
		t.Errorf("executeRules returned %v, want nil", r.err)
	}
	if len(r.errChErrs) != 0 || len(r.results) != 0 {
		t.Errorf("errCh got %v and resultCh got %v, want nothing on either", r.errChErrs, r.results)
	}
	if r.doneClosed {
		t.Errorf("done was closed, want it open")
	}
}

func caseResult(t *testing.T, r *run) any {
	t.Helper()
	if len(r.cases) != 1 {
		t.Fatalf("extracted %d xr:Case rows, want 1", len(r.cases))
	}
	return r.cases[0][2]
}

func runCharacterisation(t *testing.T, home string) {
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "aa1",
		workspace.SkipAssetValidation("characterisation test of executeRules: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws under %s: %v\n%s", home, err, out)
	}

	// test_looping_main.jr: $max_looping = 5, and LOOP03 sets jets:completed on loop 3.
	// One lp:Person is created on each of loops 1 to 3.
	t.Run("loop_completes_before_its_maximum", func(t *testing.T) {
		r := runOnce(t, loopingRuleFile, "ok", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		assertMessages(t, r)
		if len(r.persons) != 3 {
			t.Errorf("loop completion: extracted %d lp:Person, want 3, one per loop until loop 3", len(r.persons))
		}
	})

	t.Run("max_looping_override_reached", func(t *testing.T) {
		r := runOnce(t, loopingRuleFile, "ok", &compute_pipes.JetrulesSpec{MaxLooping: 2})
		assertBundleKept(t, r)
		assertMessages(t, r, msgMaxLoop("2"))
		if len(r.persons) != 2 {
			t.Errorf("max_looping 2: extracted %d lp:Person, want 2, the loop stopping after loop 2", len(r.persons))
		}
	})

	// I-7, fixed by AA.3. The loop completes on loop 3, the last one max_looping 3
	// allows, and is not reported as reaching its maximum. Until AA.3 it was.
	t.Run("I7_completion_on_last_allowed_loop_not_max_loop", func(t *testing.T) {
		r := runOnce(t, loopingRuleFile, "ok", &compute_pipes.JetrulesSpec{MaxLooping: 3})
		assertBundleKept(t, r)
		assertMessages(t, r)
		if len(r.persons) != 3 {
			t.Errorf("max_looping 3: extracted %d lp:Person, want 3, the loop having completed", len(r.persons))
		}
	})

	// test_execute_rules_main.jr: $max_looping = 2.
	t.Run("completes_under_the_rule_file_max_looping", func(t *testing.T) {
		r := runOnce(t, executeRulesRuleFile, "ok", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		assertMessages(t, r)
		if got := caseResult(t, r); got != "done" {
			t.Errorf("xr:result = %v, want done", got)
		}
	})

	t.Run("rule_file_max_looping_reached", func(t *testing.T) {
		r := runOnce(t, executeRulesRuleFile, "endless", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		assertMessages(t, r, msgMaxLoop("2"))
	})

	// I-7's second shape: an ExecuteRules error on loop 2, the last one the rule
	// file allows, is the error alone. Until AA.3 it was also MAX LOOP REACHED.
	t.Run("I7_execute_rules_error_on_last_allowed_loop_not_max_loop", func(t *testing.T) {
		r := runOnce(t, executeRulesRuleFile, "late_runaway", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		assertMessages(t, r, msgExecuteRulesError)
	})

	t.Run("execute_rules_error_max_vertex_visits", func(t *testing.T) {
		r := runOnce(t, executeRulesRuleFile, "runaway", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		// The error breaks the loop on loop 0, so the maximum is not reported as well.
		assertMessages(t, r, msgExecuteRulesError)
	})

	t.Run("jets_exception_session_not_saved_by_default", func(t *testing.T) {
		r := runOnce(t, executeRulesRuleFile, "exception", &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, r)
		assertMessages(t, r, msgException)
		if len(r.errorRows) != 1 {
			t.FailNow()
		}
		if r.errorRows[0].ReteSessionSaved != "N" || r.errorRows[0].ReteSessionTriples.Valid {
			t.Errorf("exception row saved the rete session (%q, valid %v), want N and no triples",
				r.errorRows[0].ReteSessionSaved, r.errorRows[0].ReteSessionTriples.Valid)
		}
	})

	t.Run("jets_exception_session_saved_up_to_max_rete_sessions_saved", func(t *testing.T) {
		h := newHarness(t, executeRulesRuleFile, &compute_pipes.JetrulesSpec{MaxReteSessionsSaved: 1})
		first, second := h.execute("exception"), h.execute("exception")
		assertMessages(t, first, msgException)
		assertMessages(t, second, msgException)
		if len(first.errorRows) != 1 || len(second.errorRows) != 1 {
			t.FailNow()
		}
		if first.errorRows[0].ReteSessionSaved != "Y" ||
			!strings.Contains(first.errorRows[0].ReteSessionTriples.String, "xr exception raised") {
			t.Errorf("first exception row: saved %q, triples %.80q; want Y and the session carrying the exception",
				first.errorRows[0].ReteSessionSaved, first.errorRows[0].ReteSessionTriples.String)
		}
		if second.errorRows[0].ReteSessionSaved != "N" {
			t.Errorf("second exception row: saved %q, want N, max_rete_sessions_saved being 1",
				second.errorRows[0].ReteSessionSaved)
		}
	})

	// Each on_error policy against each of the three failures, and a bundle that did not fail.
	failures := []struct {
		mode    string
		message string
		result  string // xr:result the rules leave on the case
	}{
		{"ok", "", "done"},
		{"endless", msgMaxLoop("2"), "looping"},
		{"runaway", msgExecuteRulesError, "runaway"},
		{"exception", msgException, "raised"},
	}
	for _, policy := range []string{compute_pipes.OnErrorPassThrough, compute_pipes.OnErrorDrop, compute_pipes.OnErrorFail} {
		for _, f := range failures {
			t.Run("on_error_"+policy+"_"+f.mode, func(t *testing.T) {
				r := runOnce(t, executeRulesRuleFile, f.mode, &compute_pipes.JetrulesSpec{OnError: policy})
				if f.message == "" {
					assertMessages(t, r)
				} else {
					assertMessages(t, r, f.message)
				}
				failed := f.message != ""
				switch {
				case !failed || policy == compute_pipes.OnErrorPassThrough:
					assertBundleKept(t, r)
					if got := caseResult(t, r); got != f.result {
						t.Errorf("on_error %s: xr:result = %v, want %s", policy, got, f.result)
					}
				case policy == compute_pipes.OnErrorDrop:
					assertBundleKept(t, r)
					if len(r.cases) != 0 {
						t.Errorf("on_error drop: extracted %d xr:Case rows, want none", len(r.cases))
					}
				case policy == compute_pipes.OnErrorFail:
					if r.err == nil || r.err.Error() != msgOnErrorFail {
						t.Errorf("on_error fail: executeRules returned %v, want %q", r.err, msgOnErrorFail)
					}
					if len(r.cases) != 0 {
						t.Errorf("on_error fail: extracted %d xr:Case rows, want none", len(r.cases))
					}
					if len(r.results) != 1 || r.results[0].Err == nil || r.results[0].Err.Error() != msgOnErrorFail {
						t.Errorf("on_error fail: resultCh got %v, want one result carrying the error", r.results)
					}
					if len(r.errChErrs) != 1 || len(r.results) != 1 || !errors.Is(r.errChErrs[0], r.results[0].Err) {
						t.Errorf("on_error fail: errCh got %v, want the same error once", r.errChErrs)
					}
					if !r.doneClosed {
						t.Errorf("on_error fail: done is open, want it closed")
					}
				}
			})
		}
	}

	// max_error_count stops the reporting, not the counting, and not on_error.
	t.Run("max_error_count_stops_reporting", func(t *testing.T) {
		h := newHarness(t, executeRulesRuleFile, &compute_pipes.JetrulesSpec{MaxErrorCount: 1, OnError: compute_pipes.OnErrorDrop})
		first, second := h.execute("exception"), h.execute("runaway")
		assertMessages(t, first, msgException)
		assertMessages(t, second)
		if len(second.cases) != 0 {
			t.Errorf("an unreported failure still drops: extracted %d xr:Case rows, want none", len(second.cases))
		}
		if got := h.worker.ErrorCountForTest(); got != 2 {
			t.Errorf("errorCount = %d, want 2, counting the unreported failure", got)
		}
	})

	// The first rule set's rete session is created before the input is asserted,
	// and the input is asserted once, before any rule set runs.
	t.Run("rete_session_created_before_input_asserted", func(t *testing.T) {
		h := newHarness(t, executeRulesSequence, &compute_pipes.JetrulesSpec{})
		assertBundleKept(t, h.execute("ok"))
		firstRete, firstKey, keys := -1, -1, 0
		for i, c := range h.calls {
			if strings.HasPrefix(c, "NewReteSession ") && firstRete < 0 {
				firstRete = i
			}
			if c == "Insert jets:key" {
				keys++
				if firstKey < 0 {
					firstKey = i
				}
			}
		}
		if len(h.calls) == 0 {
			t.Fatalf("the rdf session recorded no calls")
		}
		if firstRete != 0 || h.calls[0] != "NewReteSession "+executeRulesRuleFile {
			t.Errorf("first call = %q, want the first rule set's rete session before anything is inserted", h.calls[0])
		}
		if firstKey != firstRete+1 {
			t.Errorf("input asserted at call %d, want %d, straight after the first rete session (calls %q)",
				firstKey, firstRete+1, h.calls[:min(len(h.calls), 6)])
		}
		if keys != 1 {
			t.Errorf("input asserted %d times, want once for the whole sequence", keys)
		}
	})

	// A rule sequence: the exception asserted by test_execute_rules_main.jr stays in
	// the rdf session and is found again by test_looping_main.jr's scan, but it is
	// reported once, by the rule set that raised it (I-8, fixed 2026-10-11). Until
	// then it was reported once per rule set from the raiser to the last, and each
	// report counted toward max_error_count and could save a rete session. Each
	// rule set reads its own $max_looping: 2, then 5.
	t.Run("sequence_reports_an_exception_once_from_the_rule_set_that_raised_it", func(t *testing.T) {
		r := runOnce(t, executeRulesSequence, "exception", &compute_pipes.JetrulesSpec{MaxReteSessionsSaved: 2})
		assertBundleKept(t, r)
		assertMessages(t, r, msgException)
		if len(r.errorRows) == 1 && r.errorRows[0].ReteSessionSaved != "Y" {
			t.Errorf("exception row: saved %q, want Y, the rule set that raised it saving its session", r.errorRows[0].ReteSessionSaved)
		}
		if len(r.persons) != 3 {
			t.Errorf("sequence: extracted %d lp:Person, want 3 from the second rule set's loop", len(r.persons))
		}
		if got := caseResult(t, r); got != "raised" {
			t.Errorf("sequence: xr:result = %v, want raised", got)
		}
	})

	// Two rule sets each raise an exception: test_execute_rules_main.jr raises
	// "xr exception raised", and test_execute_rules_second.jr raises it again,
	// which is the same triple, and raises "xr second exception raised". Each is
	// reported once, by the rule set that raised it first, and each saves a session.
	t.Run("sequence_reports_two_exceptions_from_two_rule_sets_once_each", func(t *testing.T) {
		h := newHarness(t, twoExceptionsSequence, &compute_pipes.JetrulesSpec{MaxReteSessionsSaved: 5})
		r := h.execute("exception")
		assertBundleKept(t, r)
		assertMessages(t, r, msgException, msgSecondException)
		if len(r.errorRows) != 2 {
			t.FailNow()
		}
		// The first row's session is the first rule set's: the second exception is
		// not in it yet.
		if r.errorRows[0].ReteSessionSaved != "Y" || r.errorRows[1].ReteSessionSaved != "Y" {
			t.Errorf("exception rows saved %q and %q, want Y and Y",
				r.errorRows[0].ReteSessionSaved, r.errorRows[1].ReteSessionSaved)
		}
		if strings.Contains(r.errorRows[0].ReteSessionTriples.String, "xr second exception raised") ||
			!strings.Contains(r.errorRows[1].ReteSessionTriples.String, "xr second exception raised") {
			t.Errorf("saved sessions: want the second exception in the second rule set's session only")
		}
		if got := h.worker.ErrorCountForTest(); got != 2 {
			t.Errorf("errorCount = %d, want 2, one per exception", got)
		}
	})
}
