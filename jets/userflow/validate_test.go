package userflow

import (
	"os"
	"path/filepath"
	"testing"
)

func readFlow(t *testing.T, name string) *Flow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(flowsDir, name+".uf.json"))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	flow, err := ParseFlow(data)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	return flow
}

// TestShippingFlowsHaveNoReferenceErrors is the other half of the rule the
// schema test states: a real configuration that fails means the check is wrong.
// Under the default policy the eleven flows are clean.
func TestShippingFlowsHaveNoReferenceErrors(t *testing.T) {
	for _, path := range flowFiles(t) {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			flow := readFlow(t, name[:len(name)-len(".uf.json")])
			if errs := ErrorsOnly(ValidateFlow(flow, DefaultPolicy())); len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
		})
	}
}

// TestStrictReachabilityChangesTheOutcome is the switch doing its job on the one
// flow in the corpus that exercises it — and on the same flow, which is what
// makes it a severity change rather than a different check.
func TestStrictReachabilityChangesTheOutcome(t *testing.T) {
	flow := readFlow(t, "pipelineConfigUF")

	lenient := ValidateFlow(flow, DefaultPolicy())
	if got := len(ErrorsOnly(lenient)); got != 0 {
		t.Errorf("default policy: expected no errors, got %d", got)
	}
	if len(lenient) != 2 {
		t.Fatalf("default policy: expected 2 findings, got %d: %v", len(lenient), lenient)
	}

	strict := ValidateFlow(flow, StrictPolicy())
	if got := len(ErrorsOnly(strict)); got != 2 {
		t.Errorf("strict policy: expected 2 errors, got %d: %v", got, strict)
	}
	for i := range strict {
		if strict[i].Code != lenient[i].Code || strict[i].Message != lenient[i].Message {
			t.Errorf("finding %d differs beyond severity:\n %v\n %v", i, lenient[i], strict[i])
		}
	}
	if strict[0].Message != `state "add_injected_process_inputs" is not reachable from "select_add_or_edit"` {
		t.Errorf("unexpected first finding: %q", strict[0].Message)
	}
}

// TestStrictReachabilityRefusesTheShippingFlow states the trade the environment
// variable buys, as a test rather than as a sentence in a comment. A deployment
// that sets it cannot save pipelineConfigUF unmodified.
func TestStrictReachabilityRefusesTheShippingFlow(t *testing.T) {
	t.Setenv(StrictReachabilityEnvVar, "true")
	policy := PolicyFromEnv()

	refused := 0
	for _, path := range flowFiles(t) {
		name := filepath.Base(path)
		flow := readFlow(t, name[:len(name)-len(".uf.json")])
		if len(ErrorsOnly(ValidateFlow(flow, policy))) > 0 {
			refused++
			if name != "pipelineConfigUF.uf.json" {
				t.Errorf("%s is refused under strict reachability and should not be", name)
			}
		}
	}
	if refused != 1 {
		t.Errorf("expected exactly pipelineConfigUF to be refused, got %d flows", refused)
	}
}

func TestPolicyFromEnv(t *testing.T) {
	// The value table is duplicated in isTruthy's test in
	// jetsclient_ide/src/userflow/validate.test.ts, and must stay identical:
	// the two implementations agreeing is the point of keeping the helper
	// separate in both.
	truthy := []string{"1", "true", "TRUE", "True", "yes", "YES", "on", "ON", " true ", "\ttrue\n"}
	falsy := []string{"", "0", "false", "no", "off", "2", "y", "t", "tru", "enabled", " "}

	for _, value := range truthy {
		t.Setenv(StrictReachabilityEnvVar, value)
		if PolicyFromEnv().UnreachableState != Error {
			t.Errorf("%q should enable strict reachability", value)
		}
	}
	for _, value := range falsy {
		t.Setenv(StrictReachabilityEnvVar, value)
		if PolicyFromEnv().UnreachableState != Warning {
			t.Errorf("%q should leave the warning a warning", value)
		}
	}

	os.Unsetenv(StrictReachabilityEnvVar)
	if PolicyFromEnv().UnreachableState != Warning {
		t.Error("an unset variable should leave the warning a warning")
	}
}

// TestReferenceErrorsAreFoundRegardlessOfPolicy keeps the switch honest in the
// other direction: it moves one finding's severity and nothing else's.
func TestReferenceErrorsAreFoundRegardlessOfPolicy(t *testing.T) {
	cases := map[string]struct {
		json string
		code string
	}{
		"a start state that is not a state": {
			json: `{"startAtKey":"nope","states":{"a":{"isEnd":true}}}`,
			code: CodeUnknownStartState,
		},
		"a transition to a state that does not exist": {
			json: `{"startAtKey":"a","states":{"a":{"defaultNextState":"typo"}}}`,
			code: CodeUnknownTarget,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			flow, err := ParseFlow([]byte(tc.json))
			if err != nil {
				t.Fatalf("parsing: %v", err)
			}
			for _, policy := range []Policy{DefaultPolicy(), StrictPolicy()} {
				errs := ErrorsOnly(ValidateFlow(flow, policy))
				if len(errs) != 1 || errs[0].Code != tc.code {
					t.Errorf("policy %v: expected one %s, got %v", policy, tc.code, errs)
				}
			}
		})
	}
}

// TestUnknownStartStateIsNotBuriedUnderUnreachables holds under either policy,
// and matters more under the strict one: every state would otherwise become an
// error and the actual fault would be one line in fifty.
func TestUnknownStartStateIsNotBuriedUnderUnreachables(t *testing.T) {
	flow, err := ParseFlow([]byte(
		`{"startAtKey":"nope","states":{"a":{"defaultNextState":"b"},"b":{"isEnd":true}}}`))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	findings := ValidateFlow(flow, StrictPolicy())
	if len(findings) != 1 || findings[0].Code != CodeUnknownStartState {
		t.Errorf("expected one unknownStartState, got %v", findings)
	}
}
