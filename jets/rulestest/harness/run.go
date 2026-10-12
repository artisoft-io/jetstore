package harness

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Factories are the engines the harness can build. The command installs both,
// as local_test_driver does; a harness without the C++ engine leaves Native nil
// and refuses a test that needs it.
type Factories struct {
	Native compute_pipes.JetRulesFactory
	Go     compute_pipes.JetRulesFactory
}

// Options says which test to run and how.
type Options struct {
	// WorkspaceRoot is $WORKSPACES_HOME/$WORKSPACE. The engines read the
	// workspace from those variables themselves (definition F16); this is for
	// resolving TestPath and reporting it relative to the workspace.
	WorkspaceRoot string
	// TestPath is the .tst.json, absolute or relative to WorkspaceRoot.
	TestPath string
	// Engine, when not empty, overrides the test's choice: rulestest.EngineNative
	// or rulestest.EngineGo. It turns compare_engines off.
	Engine    string
	Factories Factories
}

// Run runs one test and returns its result. It never returns nil: whatever
// stops the test is its status error, with the reason.
func Run(opts Options) *rulestest.Result {
	start := time.Now()
	path := opts.TestPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(opts.WorkspaceRoot, path)
	}
	r := &rulestest.Result{
		TestPath:   relativeTo(opts.WorkspaceRoot, path),
		LegacyMode: rulestest.LegacyModeLegacy,
		Checks:     []rulestest.CheckResult{},
	}
	defer func() { r.DurationMs = time.Since(start).Milliseconds() }()

	td, findings, err := rulestest.LoadTestDefinition(path)
	switch {
	case err != nil:
		return failed(r, rulestest.StatusError, "cannot read the test definition: %v", err)
	case td == nil || rulestest.HasErrors(findings):
		r.Findings = findings
		if td != nil {
			r.TestName = td.TestName
		}
		return failed(r, rulestest.StatusError, "invalid test definition: %s", findingSummary(findings))
	}
	r.TestName = td.TestName
	// The mode the test asks for; replaced below by the mode each engine reports it
	// was built in, so the result records what ran rather than what was written.
	r.LegacyMode = td.LegacyMode()
	r.Sessions = len(td.InputSessions)

	engines, err := EnginesFor(td, opts.Engine)
	r.Engine = engineLabel(engines)
	if err != nil {
		return failed(r, rulestest.StatusError, "%v", err)
	}
	for _, e := range engines {
		if factoryFor(opts.Factories, e) == nil {
			return failed(r, rulestest.StatusError, "the %s engine is not available in this harness", e)
		}
	}
	// The CSVs are read once, before any engine is built: a misspelt column is
	// the author's error and is reported without paying for a load.
	dir := filepath.Dir(path)
	tables := make([][]*InputTable, len(td.InputSessions))
	for i, s := range td.InputSessions {
		for _, in := range s.Input {
			t, err := ReadInputTable(filepath.Join(dir, in), in)
			if err != nil {
				return failed(r, rulestest.StatusError, "session %q: %v", s.Name, err)
			}
			tables[i] = append(tables[i], t)
		}
	}

	runs := make([]*engineRun, len(engines))
	for i, e := range engines {
		runs[i] = runOnEngine(td, e, factoryFor(opts.Factories, e), tables)
		r.LoadMs += runs[i].loadMs
		r.RunMs += runs[i].runMs
		if runs[i].legacyMode != "" {
			r.LegacyMode = runs[i].legacyMode
		}
	}
	if len(runs) == 1 {
		run := runs[0]
		r.Status, r.Reason = run.status, run.reason
		r.Loops = run.loops
		r.Checks = append(r.Checks, run.checks...)
		r.SessionResults = run.sessions
		return r
	}
	return compareRuns(r, runs)
}

// RetractionModeOf is the engine retraction mode a test definition's legacy object
// asks for: a switch that is absent or true keeps its defect, false turns it off.
// Labels agree: RetractionModeOf(td).Label() == td.LegacyMode().
func RetractionModeOf(td *rulestest.TestDefinition) compute_pipes.RetractionMode {
	return compute_pipes.RetractionMode{
		ReplayGraphReadingConsequents: !td.Legacy.RecomputeOnRetractionOn(),
		EraseWithdrawnRows:            !td.Legacy.WithdrawnRowStaysOn(),
	}
}

func failed(r *rulestest.Result, status, format string, args ...any) *rulestest.Result {
	r.Status = status
	r.Reason = fmt.Sprintf(format, args...)
	return r
}

// EnginesFor is the engine choice (definition R10): C++ unless the test sets
// use_jet_rules_go, both with compare_engines, and override when not empty.
// _0:no_truth_main_on_exist exists only in C++ (definition F14), so a run that
// includes Go refuses it; validation refuses it already when the test itself
// asks for Go, and this covers an override.
func EnginesFor(td *rulestest.TestDefinition, override string) ([]string, error) {
	var engines []string
	switch {
	case override == rulestest.EngineNative || override == rulestest.EngineGo:
		engines = []string{override}
	case override != "":
		return nil, fmt.Errorf("unknown engine %q: %s or %s", override, rulestest.EngineNative, rulestest.EngineGo)
	case td.CompareEngines:
		engines = []string{rulestest.EngineNative, rulestest.EngineGo}
	case td.JetrulesConfig.UseJetRulesGo:
		engines = []string{rulestest.EngineGo}
	default:
		engines = []string{rulestest.EngineNative}
	}
	for _, e := range engines {
		if e != rulestest.EngineGo {
			continue
		}
		for _, rc := range td.JetrulesConfig.RuleConfig {
			if _, ok := rc[rulestest.NoTruthMainOnExist]; ok {
				return engines, fmt.Errorf("%s is set in rule_config and exists only in the C++ engine; the Go engine would ignore it",
					rulestest.NoTruthMainOnExist)
			}
		}
	}
	return engines, nil
}

func engineLabel(engines []string) string {
	if len(engines) == 2 {
		return rulestest.EngineBoth
	}
	if len(engines) == 1 {
		return engines[0]
	}
	return ""
}

func factoryFor(f Factories, engine string) compute_pipes.JetRulesFactory {
	if engine == rulestest.EngineGo {
		return f.Go
	}
	return f.Native
}

// engineRun is the test run on one engine.
type engineRun struct {
	engine         string
	legacyMode     string // as the engine reports it was built; empty when no engine was built
	status, reason string
	loadMs, runMs  int64
	loops          int
	checks         []rulestest.CheckResult
	sessions       []rulestest.SessionResult
}

// runOnEngine builds one engine and runs every input session on it. The order
// is the pipeline's (NewJetrulesTransformationPipe, then executeRules): the
// engine, its JetResources, the source period and rule_config into the meta
// graph, and only then the first RDF session — the meta graph locks when it
// opens (definition F31), and a new meta resource is refused after that: a nil
// node on Go, an error on C++, where it aborted the process until 2026-10-11
// (I-15).
func runOnEngine(td *rulestest.TestDefinition, engine string, factory compute_pipes.JetRulesFactory,
	tables [][]*InputTable) (run *engineRun) {
	run = &engineRun{engine: engine}
	defer func() {
		if p := recover(); p != nil {
			log.Printf("rules_unit_test: recovered on the %s engine: %v\n%s", engine, p, debug.Stack())
			run.status, run.reason = rulestest.StatusError, fmt.Sprintf("the %s engine panicked: %v", engine, p)
		}
	}()
	cfg := &td.JetrulesConfig
	spec, err := JetrulesSpecOf(cfg)
	if err != nil {
		run.status, run.reason = rulestest.StatusError, err.Error()
		return
	}
	loadStart := time.Now()
	// The LEGACY switches are applied here, when the rules load (definition §8.5.3):
	// the only caller that passes a mode other than legacy (§8.5.5).
	mode := RetractionModeOf(td)
	re, err := factory.NewJetRuleEngine(nil, cfg.ProcessName, cfg.IsDebug, compute_pipes.WithRetractionMode(mode))
	if err != nil {
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("while building the %s engine for %s: %v", engine, cfg.ProcessName, err)
		return
	}
	defer re.Release()
	// legacy_mode is what the engine says it was built in, and a test never runs in a
	// mode other than the one it declares.
	built, ok := re.(interface {
		RetractionMode() compute_pipes.RetractionMode
	})
	switch {
	case ok:
		run.legacyMode = built.RetractionMode().Label()
	case mode.IsLegacy():
		run.legacyMode = rulestest.LegacyModeLegacy
	default:
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("the %s engine cannot report its retraction mode, and the test asks for %s", engine, td.LegacyMode())
		return
	}
	if run.legacyMode != td.LegacyMode() {
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("the %s engine was built in legacy mode %s, and the test asks for %s", engine, run.legacyMode, td.LegacyMode())
		return
	}
	re.JetResources()
	env := map[string]any{}
	if err := compute_pipes.AssertSourcePeriodInfo(re, spec, env); err != nil {
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("while asserting the source period: %v", err)
		return
	}
	if err := compute_pipes.AssertRuleConfiguration(re, spec, env); err != nil {
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("while asserting rule_config: %v", err)
		return
	}
	run.loadMs = time.Since(loadStart).Milliseconds()
	runStart := time.Now()
	defer func() { run.runMs = time.Since(runStart).Milliseconds() }()

	exceptionExpected := expectsException(td)
	var firstError string
	failedChecks, totalChecks := 0, 0
	for i := range td.InputSessions {
		s := &td.InputSessions[i]
		sr, checks := runSession(td, s, spec, re, tables[i], exceptionExpected)
		sr.Engine = engine
		run.sessions = append(run.sessions, sr)
		run.loops += sr.Loops
		for j := range checks {
			checks[j].SessionName = s.Name
			totalChecks++
			if checks[j].Outcome != rulestest.OutcomePass {
				failedChecks++
			}
		}
		run.checks = append(run.checks, checks...)
		if sr.Status == rulestest.StatusError && firstError == "" {
			firstError = sr.Reason
		}
	}
	switch {
	case firstError != "":
		run.status, run.reason = rulestest.StatusError, firstError
	case failedChecks > 0:
		run.status, run.reason = rulestest.StatusFail, fmt.Sprintf("%d of %d checks failed", failedChecks, totalChecks)
	default:
		run.status, run.reason = rulestest.StatusPass, ""
	}
	applyExpect(td.Expect, run, firstError != "", failedChecks, totalChecks)
	return
}

// runSession runs one input session: an RDF session, the rule sets through
// the pipeline's own loop (RunRuleSets) with the CSVs as its input, then the
// checks against a snapshot of the session.
func runSession(td *rulestest.TestDefinition, s *rulestest.InputSession, spec *compute_pipes.JetrulesSpec,
	re compute_pipes.JetRuleEngine, tables []*InputTable, exceptionExpected bool) (sr rulestest.SessionResult, checks []rulestest.CheckResult) {
	sr = rulestest.SessionResult{Name: s.Name}
	setError := func(format string, args ...any) {
		if sr.Status != rulestest.StatusError {
			sr.Status, sr.Reason = rulestest.StatusError, fmt.Sprintf("session %q: ", s.Name)+fmt.Sprintf(format, args...)
		}
	}
	rdfSession, err := re.NewRdfSession()
	if err != nil {
		setError("while creating the rdf session: %v", err)
		return
	}
	defer rdfSession.Release()
	outcome := compute_pipes.RunRuleSets(rdfSession, re, &compute_pipes.RuleSetsConfig{
		MaxLooping: td.JetrulesConfig.MaxLooping,
		IsDebug:    td.JetrulesConfig.IsDebug,
	}, assertTables(spec, tables, td.JetrulesConfig.IsDebug))
	for _, rs := range outcome.RuleSets {
		sr.Loops += rs.Loops
	}
	sr.MaxLoopReached = outcome.MaxLoopReached
	sr.Exceptions = outcome.Exceptions
	if outcome.Err != nil {
		// The rules did not run to the end; there is nothing to check.
		setError("%v", outcome.Err)
		return
	}
	if outcome.ExecuteErr != nil {
		setError("ExecuteRules: %v", outcome.ExecuteErr)
	}
	if outcome.MaxLoopReached {
		for _, rs := range outcome.RuleSets {
			if rs.MaxLoopReached {
				setError("maximum loop count reached (%d) in rule set %s", rs.MaxLooping, rs.RuleSet)
				break
			}
		}
	}
	if len(outcome.Exceptions) > 0 && !exceptionExpected {
		setError("jets:exception: %s", strings.Join(outcome.Exceptions, "; "))
	}

	snap, err := TakeSnapshot(rdfSession)
	if err != nil {
		setError("%v", err)
		return
	}
	evaluate := func(list []rulestest.Check, kind string) {
		for i := range list {
			cr, err := snap.EvaluateCheck(&list[i], kind)
			if err != nil {
				cr.Outcome, cr.Found = rulestest.OutcomeFail, "cannot evaluate: "+err.Error()
				setError("check %s cannot be evaluated: %v", cr.Check, err)
			}
			checks = append(checks, cr)
		}
	}
	evaluate(s.ExpectedTriples, rulestest.KindExpected)
	evaluate(s.ExcludedTriples, rulestest.KindExcluded)
	evaluate(td.ExpectedTriples, rulestest.KindExpected)
	evaluate(td.ExcludedTriples, rulestest.KindExcluded)
	if sr.Status == "" {
		sr.Status = rulestest.StatusPass
		for _, c := range checks {
			if c.Outcome != rulestest.OutcomePass {
				sr.Status = rulestest.StatusFail
				break
			}
		}
	}
	return
}

// expectsException: an expected check about (jets:iState jets:exception ?)
// makes an exception an ordinary check instead of an error (definition §6).
func expectsException(td *rulestest.TestDefinition) bool {
	about := func(list []rulestest.Check) bool {
		for i := range list {
			c := &list[i]
			if c.Count != nil && c.S == "jets:iState" && c.P == "jets:exception" {
				return true
			}
			for _, t := range c.Triples() {
				if t.S == "jets:iState" && t.P == "jets:exception" {
					return true
				}
			}
		}
		return false
	}
	if about(td.ExpectedTriples) {
		return true
	}
	for i := range td.InputSessions {
		if about(td.InputSessions[i].ExpectedTriples) {
			return true
		}
	}
	return false
}

// applyExpect is the expect field (definition §6, AB.7): a run ending in the
// expected error passes when its checks do, a run ending in another error stays
// an error, and a run whose rules terminate fails. Only an error from running a
// session is matched; a test that cannot be loaded is not what expect is about.
func applyExpect(expect *rulestest.Expectation, run *engineRun, sessionErrored bool, failedChecks, totalChecks int) {
	if expect == nil || run.status == "" {
		return
	}
	want := expect.Reason
	label := "an error"
	if want != "" {
		label = fmt.Sprintf("an error containing %q", want)
	}
	switch {
	case !sessionErrored:
		reason := fmt.Sprintf("expected %s; the rules ran to completion", label)
		if failedChecks > 0 {
			reason += fmt.Sprintf(" and %d of %d checks failed", failedChecks, totalChecks)
		}
		run.status, run.reason = rulestest.StatusFail, reason
	case want == "" || strings.Contains(strings.ToLower(run.reason), strings.ToLower(want)):
		if failedChecks > 0 {
			run.status = rulestest.StatusFail
			run.reason = fmt.Sprintf("the expected error occurred (%s) and %d of %d checks failed", run.reason, failedChecks, totalChecks)
		} else {
			run.status, run.reason = rulestest.StatusPass, "the expected error occurred: "+run.reason
		}
	default:
		run.reason = fmt.Sprintf("expected %s; got: %s", label, run.reason)
	}
}

// compareRuns is compare_engines (definition Q-11): each check appears once
// per engine, and the test fails when the engines' statuses or any check's
// outcome differ, listing every difference.
func compareRuns(r *rulestest.Result, runs []*engineRun) *rulestest.Result {
	a, b := runs[0], runs[1]
	r.Loops = a.loops
	for _, run := range runs {
		for _, c := range run.checks {
			c.Engine = run.engine
			r.Checks = append(r.Checks, c)
		}
		r.SessionResults = append(r.SessionResults, run.sessions...)
	}
	var diffs []string
	key := func(c rulestest.CheckResult) string { return c.SessionName + "\x00" + c.Kind + "\x00" + c.Check }
	bChecks := map[string]rulestest.CheckResult{}
	for _, c := range b.checks {
		bChecks[key(c)] = c
	}
	for _, ca := range a.checks {
		cb, ok := bChecks[key(ca)]
		switch {
		case !ok:
			diffs = append(diffs, fmt.Sprintf("session %q check %s: evaluated on %s only", ca.SessionName, ca.Check, a.engine))
		case ca.Outcome != cb.Outcome:
			diffs = append(diffs, fmt.Sprintf("session %q check %s: %s=%s (%s), %s=%s (%s)",
				ca.SessionName, ca.Check, a.engine, ca.Outcome, ca.Found, b.engine, cb.Outcome, cb.Found))
		}
		delete(bChecks, key(ca))
	}
	for _, cb := range bChecks {
		diffs = append(diffs, fmt.Sprintf("session %q check %s: evaluated on %s only", cb.SessionName, cb.Check, b.engine))
	}
	sort.Strings(diffs)
	if a.status != b.status {
		diffs = append([]string{fmt.Sprintf("status: %s=%s (%s), %s=%s (%s)",
			a.engine, a.status, a.reason, b.engine, b.status, b.reason)}, diffs...)
	}
	if len(diffs) > 0 {
		r.EngineDifferences = diffs
		r.Status = rulestest.StatusFail
		r.Reason = fmt.Sprintf("the engines disagree (%d difference(s)): %s", len(diffs), diffs[0])
		return r
	}
	r.Status, r.Reason = a.status, a.reason
	return r
}

func findingSummary(findings []wsvalidate.Finding) string {
	errs := 0
	first := ""
	for _, f := range findings {
		if f.Severity != wsvalidate.Error {
			continue
		}
		errs++
		if first == "" {
			first = f.Message
			if f.Path != "" {
				first = f.Path + ": " + first
			}
		}
	}
	if errs > 1 {
		return fmt.Sprintf("%s (and %d more)", first, errs-1)
	}
	return first
}

func relativeTo(root, path string) string {
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && filepath.IsLocal(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return path
}
