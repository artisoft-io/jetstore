package harness

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

func checkRow(session, check, outcome, found string) rulestest.CheckResult {
	return rulestest.CheckResult{SessionName: session, Kind: rulestest.KindExpected, Check: check, Outcome: outcome, Found: found}
}

// Criterion 11: compare_engines fails a test whose engines disagree, and
// lists every difference.
func TestCompareRunsDisagree(t *testing.T) {
	native := &engineRun{engine: rulestest.EngineNative, status: rulestest.StatusPass, loops: 3,
		checks: []rulestest.CheckResult{
			checkRow("s1", `{"s":"a:x","p":"a:sum","o":"3"}`, rulestest.OutcomePass, "found"),
			checkRow("s1", `{"s":"a:x","p":"a:n","o":"1"}`, rulestest.OutcomePass, "found"),
		}}
	goRun := &engineRun{engine: rulestest.EngineGo, status: rulestest.StatusFail, reason: "1 of 2 checks failed", loops: 4,
		checks: []rulestest.CheckResult{
			checkRow("s1", `{"s":"a:x","p":"a:sum","o":"3"}`, rulestest.OutcomeFail, "no match; (a:x a:sum) has 2^^int"),
			checkRow("s1", `{"s":"a:x","p":"a:n","o":"1"}`, rulestest.OutcomePass, "found"),
		}}
	r := compareRuns(&rulestest.Result{Engine: rulestest.EngineBoth}, []*engineRun{native, goRun})
	if r.Status != rulestest.StatusFail || !strings.Contains(r.Reason, "the engines disagree (2 difference(s)): status: native=pass") {
		t.Errorf("status %q reason %q", r.Status, r.Reason)
	}
	if len(r.EngineDifferences) != 2 || !strings.Contains(r.EngineDifferences[1], "native=pass (found), go=fail (no match; (a:x a:sum) has 2^^int)") {
		t.Errorf("differences: %q", r.EngineDifferences)
	}
	if len(r.Checks) != 4 || r.Checks[0].Engine != rulestest.EngineNative || r.Checks[3].Engine != rulestest.EngineGo {
		t.Errorf("checks: %+v", r.Checks)
	}
	if r.Loops != 3 {
		t.Errorf("loops %d, want the native run's 3", r.Loops)
	}
}

func TestCompareRunsAgree(t *testing.T) {
	mk := func(engine string) *engineRun {
		return &engineRun{engine: engine, status: rulestest.StatusFail, reason: "1 of 1 checks failed",
			checks: []rulestest.CheckResult{checkRow("s1", `{"s":"a:x","p":"a:n","o":"1"}`, rulestest.OutcomeFail, "no match")}}
	}
	r := compareRuns(&rulestest.Result{}, []*engineRun{mk(rulestest.EngineNative), mk(rulestest.EngineGo)})
	if r.Status != rulestest.StatusFail || r.Reason != "1 of 1 checks failed" || len(r.EngineDifferences) != 0 {
		t.Errorf("agreeing engines: %+v", r)
	}
	// A check evaluated on one engine only is a difference too.
	a, b := mk(rulestest.EngineNative), mk(rulestest.EngineGo)
	b.checks = nil
	b.status, b.reason = a.status, a.reason
	r = compareRuns(&rulestest.Result{}, []*engineRun{a, b})
	if r.Status != rulestest.StatusFail || len(r.EngineDifferences) != 1 || !strings.Contains(r.EngineDifferences[0], "evaluated on native only") {
		t.Errorf("one-sided check: %+v", r)
	}
}

// AB.7: expect, on its own.
func TestApplyExpect(t *testing.T) {
	cases := []struct {
		name           string
		expect         *rulestest.Expectation
		status, reason string
		errored        bool
		failed         int
		wantStatus     string
		wantReason     string
	}{
		{"no expect", nil, rulestest.StatusError, "x", true, 0, rulestest.StatusError, "x"},
		{"expected error", &rulestest.Expectation{Status: "error", Reason: "Max Vertex Visit"}, rulestest.StatusError,
			`session "s": ExecuteRules: error: max vertex visit reached`, true, 0, rulestest.StatusPass, "the expected error occurred"},
		{"any error", &rulestest.Expectation{Status: "error"}, rulestest.StatusError, "boom", true, 0, rulestest.StatusPass, "boom"},
		{"expected error, check failed", &rulestest.Expectation{Status: "error", Reason: "boom"}, rulestest.StatusError, "boom", true, 1,
			rulestest.StatusFail, "and 1 of 3 checks failed"},
		{"another error", &rulestest.Expectation{Status: "error", Reason: "max vertex"}, rulestest.StatusError, "jets:exception: e1", true, 0,
			rulestest.StatusError, `expected an error containing "max vertex"; got: jets:exception: e1`},
		{"terminated", &rulestest.Expectation{Status: "error", Reason: "max vertex"}, rulestest.StatusPass, "", false, 0,
			rulestest.StatusFail, "the rules ran to completion"},
		{"terminated, checks failed", &rulestest.Expectation{Status: "error"}, rulestest.StatusFail, "1 of 3 checks failed", false, 1,
			rulestest.StatusFail, "ran to completion and 1 of 3 checks failed"},
	}
	for _, c := range cases {
		run := &engineRun{status: c.status, reason: c.reason}
		applyExpect(c.expect, run, c.errored, c.failed, 3)
		if run.status != c.wantStatus || !strings.Contains(run.reason, c.wantReason) {
			t.Errorf("%s: %q %q, want %q containing %q", c.name, run.status, run.reason, c.wantStatus, c.wantReason)
		}
	}
}

func TestEnginesFor(t *testing.T) {
	td := &rulestest.TestDefinition{}
	got := func(override string) string {
		e, err := EnginesFor(td, override)
		if err != nil {
			return "error: " + err.Error()
		}
		return strings.Join(e, "+")
	}
	if g := got(""); g != "native" {
		t.Errorf("default %q, want native", g)
	}
	td.JetrulesConfig.UseJetRulesGo = true
	if g := got(""); g != "go" {
		t.Errorf("use_jet_rules_go %q", g)
	}
	if g := got("native"); g != "native" {
		t.Errorf("override %q", g)
	}
	td.JetrulesConfig.UseJetRulesGo = false
	td.CompareEngines = true
	if g := got(""); g != "native+go" {
		t.Errorf("compare %q", g)
	}
	td.CompareEngines = false
	td.JetrulesConfig.RuleConfig = []map[string]any{{rulestest.NoTruthMainOnExist: map[string]any{"value": "1", "type": "int"}}}
	if g := got(""); g != "native" {
		t.Errorf("C++ with the flag %q", g)
	}
	if g := got("go"); !strings.Contains(g, "exists only in the C++ engine") {
		t.Errorf("override to Go with the C++-only flag: %q", g)
	}
	if g := got("python"); !strings.Contains(g, "unknown engine") {
		t.Errorf("bad override %q", g)
	}
}
