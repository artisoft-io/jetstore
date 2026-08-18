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

// withOrphan returns a flow with one state nothing reaches. The corpus no longer
// has one, which is the correction of I-18; the check still has to work.
func withOrphan(t *testing.T) *Flow {
	t.Helper()
	flow := readFlow(t, "loadFilesUF")
	flow.States["orphan"] = State{IsEnd: true}
	return flow
}

// TestStrictReachabilityChangesTheOutcome is the switch doing its job, and doing
// only its job: the same finding, one field apart.
func TestStrictReachabilityChangesTheOutcome(t *testing.T) {
	lenient := ValidateFlow(withOrphan(t), DefaultPolicy())
	if got := len(ErrorsOnly(lenient)); got != 0 {
		t.Errorf("default policy: expected no errors, got %d", got)
	}
	if len(lenient) != 1 {
		t.Fatalf("default policy: expected 1 finding, got %d: %v", len(lenient), lenient)
	}

	strict := ValidateFlow(withOrphan(t), StrictPolicy())
	if got := len(ErrorsOnly(strict)); got != 1 {
		t.Fatalf("strict policy: expected 1 error, got %d: %v", got, strict)
	}
	if strict[0].Code != lenient[0].Code || strict[0].Message != lenient[0].Message {
		t.Errorf("finding differs beyond severity:\n %v\n %v", lenient[0], strict[0])
	}
	if strict[0].Message != `state "orphan" is not reachable from "select_source_config"` {
		t.Errorf("unexpected finding: %q", strict[0].Message)
	}
}

// TestStrictReachabilityRefusesNothingShipping is the reverse of the assertion
// S.1 shipped, and the reversal is the finding.
//
// The switch was introduced believing a strict deployment could not save
// pipelineConfigUF. That was true of the check rather than of the flow: two of
// its states are reached by a button, and the document did not declare the edge.
// It does now, so the switch costs a deployment nothing today — see I-18.
func TestStrictReachabilityRefusesNothingShipping(t *testing.T) {
	t.Setenv(StrictReachabilityEnvVar, "true")
	policy := PolicyFromEnv()

	for _, path := range flowFiles(t) {
		name := filepath.Base(path)
		flow := readFlow(t, name[:len(name)-len(".uf.json")])
		if refusals := ErrorsOnly(ValidateFlow(flow, policy)); len(refusals) > 0 {
			t.Errorf("%s is refused under strict reachability: %v", name, refusals)
		}
	}
}

// TestGoToStatesCountAsReaching pins the edge kind that was missing, in both
// directions: it reaches what it names and nothing else, and a target that does
// not exist is still an error.
func TestGoToStatesCountAsReaching(t *testing.T) {
	flow := readFlow(t, "loadFilesUF")
	flow.States["reached"] = State{IsEnd: true}
	flow.States["orphan"] = State{IsEnd: true}
	start := flow.States[flow.StartAtKey]
	start.GoToStates = []string{"reached"}
	flow.States[flow.StartAtKey] = start

	findings := ValidateFlow(flow, DefaultPolicy())
	if len(findings) != 1 || findings[0].Code != CodeUnreachableState {
		t.Fatalf("expected only orphan to be unreached, got %v", findings)
	}
	if findings[0].Message != `state "orphan" is not reachable from "select_source_config"` {
		t.Errorf("unexpected finding: %q", findings[0].Message)
	}

	start.GoToStates = []string{"typo"}
	flow.States[flow.StartAtKey] = start
	errs := ErrorsOnly(ValidateFlow(flow, DefaultPolicy()))
	if len(errs) != 1 || errs[0].Code != CodeUnknownTarget {
		t.Errorf("a declared edge to nowhere should be an unknownTarget, got %v", errs)
	}
}

// TestFindingsCarryAPath pins the JSON Pointer the agentic_ai stream asked for.
// The three transition shapes produce three different paths, and a document-wide
// finding still points at the field that caused it rather than at nothing.
func TestFindingsCarryAPath(t *testing.T) {
	flow := readFlow(t, "clientRegistryUF")

	start := flow.States[flow.StartAtKey]
	start.Choices[1].NextState = "typoInChoice"
	flow.States[flow.StartAtKey] = start

	create := flow.States["create_client"]
	create.DefaultNextState = "typoInDefault"
	create.GoToStates = []string{"typoInGoTo"}
	flow.States["create_client"] = create

	want := []Finding{
		{Severity: Error, Code: CodeUnknownTarget, Message: `state "create_client" transitions to "typoInDefault", which is not a state`, Path: "/states/create_client/defaultNextState"},
		{Severity: Error, Code: CodeUnknownTarget, Message: `state "create_client" transitions to "typoInGoTo", which is not a state`, Path: "/states/create_client/goToStates/0"},
		{Severity: Error, Code: CodeUnknownTarget, Message: `state "select_client_vendor" transitions to "typoInChoice", which is not a state`, Path: "/states/select_client_vendor/choices/1/nextState"},
		// One typo really does strand two states; that is the walk working.
		{Severity: Warning, Code: CodeUnreachableState, Message: `state "select_client" is not reachable from "select_client_vendor"`, Path: "/states/select_client"},
		{Severity: Warning, Code: CodeUnreachableState, Message: `state "show_org" is not reachable from "select_client_vendor"`, Path: "/states/show_org"},
	}
	got := ValidateFlow(flow, DefaultPolicy())
	if len(got) != len(want) {
		t.Fatalf("expected %d findings, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d:\n got  %+v\n want %+v", i, got[i], want[i])
		}
	}
}

func TestUnknownStartStatePointsAtTheField(t *testing.T) {
	flow := readFlow(t, "loadFilesUF")
	flow.StartAtKey = "nope"
	findings := ValidateFlow(flow, DefaultPolicy())
	if len(findings) != 1 || findings[0].Path != "/startAtKey" {
		t.Errorf("expected one finding at /startAtKey, got %+v", findings)
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
