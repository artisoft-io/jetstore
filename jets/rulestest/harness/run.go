package harness

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Options says which test to run and how.
type Options struct {
	// WorkspaceRoot is $WORKSPACES_HOME/$WORKSPACE. The engines read the
	// workspace from those variables themselves (definition F16); this is for
	// resolving TestPath and reporting it relative to the workspace.
	WorkspaceRoot string
	// TestPath is the .tst.json, absolute or relative to WorkspaceRoot.
	TestPath string
	// Factory builds the engine, the C++ one (jetrules_native_adaptor). A harness
	// without it refuses every test. There was a second, the Go engine, until its
	// retirement in October 2026; a test asking for it fails validation
	// (rulestest.GoEngineRetired).
	Factory compute_pipes.JetRulesFactory
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
		LegacyMode: rulestest.LegacyModeFixed,
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
	r.Sessions = len(td.InputSessions)

	r.Engine = rulestest.EngineNative
	if opts.Factory == nil {
		return failed(r, rulestest.StatusError, "the %s engine is not available in this harness", rulestest.EngineNative)
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

	run := runOnEngine(td, rulestest.EngineNative, opts.Factory, tables)
	r.LoadMs, r.RunMs = run.loadMs, run.runMs
	r.Status, r.Reason = run.status, run.reason
	r.Loops = run.loops
	r.Checks = append(r.Checks, run.checks...)
	r.SessionResults = run.sessions
	return r
}

func failed(r *rulestest.Result, status, format string, args ...any) *rulestest.Result {
	r.Status = status
	r.Reason = fmt.Sprintf(format, args...)
	return r
}

// engineRun is the test run on one engine.
type engineRun struct {
	engine         string
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
	re, err := factory.NewJetRuleEngine(nil, cfg.ProcessName, cfg.IsDebug)
	if err != nil {
		run.status, run.reason = rulestest.StatusError, fmt.Sprintf("while building the %s engine for %s: %v", engine, cfg.ProcessName, err)
		return
	}
	defer re.Release()
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
