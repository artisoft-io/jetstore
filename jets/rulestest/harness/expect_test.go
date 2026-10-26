package harness

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

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
