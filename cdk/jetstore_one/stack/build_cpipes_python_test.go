package stack

import (
	"testing"
)

// The Python compute pipes node is gated on DEPLOY_CPIPES_PYTHON as of 2026-09-18, and these
// tests hold one claim: **with the variable unset the gate is off**, so neither the
// CpipesPythonNodeLambda construct nor the third arm of ecsOrLambdaChoice is created, and a
// deployment that has never heard of the variable synthesises the stack it synthesises today.
//
// **This file is the weaker of the two instruments and knows it.** What decides the stack is
// the synthesised template, and reaching it means running the whole CDK app. The byte
// comparison of two synth runs is what establishes criterion 96, and it was run on §13.3's
// instrument -- measured 2026-09-18 on placeholder account 111111111111 in us-east-1, sixteen
// seconds a run:
//
//	JetstoreOneStack.template.json   with the variable unset   md5 fe3b1c485063e21a169aa291159bcec9
//	                                 at the base commit        md5 fe3b1c485063e21a169aa291159bcec9
//	with DEPLOY_CPIPES_NATIVE=1      unset / base              md5 474b40978dc7328e059705dae487cf3b, both
//
// identical in both branches, along with assets.json, manifest.json, cdk.out,
// validation-report.json and all ten locally bundled Lambda zips. metadata.json and tree.json
// differ, and a control run of the base commit against itself reproduces the same two files
// and the same 118-line difference -- jsii temp-directory paths inside stack traces -- which is
// what separates noise from consequence. With DEPLOY_CPIPES_PYTHON=1 the template md5 moves and
// the delta is exactly two added resources (the function and its log group), the two state
// machines, and their two role policies; EcsOrLambdaChoice goes from two Choices to three with
// the Python arm first, in **both** machines, off **one** function.
//
// **The A/B must be run from one directory**, which cost this task a false positive: run from
// two worktrees at different paths, six of the ten Lambda zips differ, because `go build`
// embeds absolute source paths and neither `-buildvcs=false` nor `-ldflags "-s -w"` removes
// them. Same tree, two checkouts, is the only form of this comparison that means anything
// (P9-I50).
//
// These tests are kept for the reason §13.4 keeps their sibling: they run in milliseconds, and
// the regression they catch -- somebody changing the predicate or the flag literal -- is what a
// string test catches and what nobody will re-run a synth to notice. The division is: the tests
// hold the constants, the synth held the claim.
//
// unsetEnv is build_cpipes_lambdas_test.go's, in this same package.

// The literals are written out rather than compared against the source's own constants,
// deliberately and for build_cpipes_lambdas_test.go's stated reason: comparing a constant with
// itself passes the day somebody changes it, which is the change the test exists to catch.
const (
	pythonDeployVariable = "DEPLOY_CPIPES_PYTHON"
	pythonReducingFlag   = "$.usePythonReducingTask"
	ecsReducingFlag      = "$.useECSReducingTask"
	noMoreTaskFlag       = "$.noMoreTask"
)

func TestDeployCpipesPythonUnsetIsOff(t *testing.T) {
	unsetEnv(t, pythonDeployVariable)
	if DeployCpipesPythonFromEnv() {
		t.Errorf("unset %s reported on -- with it unset the Python node must not be built and"+
			" the third arm of ecsOrLambdaChoice must not exist, which is what makes an"+
			" unconfigured deployment synthesise the stack it synthesises today",
			pythonDeployVariable)
	}
}

func TestDeployCpipesPythonFromEnv(t *testing.T) {
	tests := []struct {
		what  string
		set   bool
		value string
		want  bool
	}{
		{what: "unset is off", want: false},
		{what: "empty is off", set: true, value: "", want: false},
		{what: "TRUE is on", set: true, value: "TRUE", want: true},
		{what: "true is on, the value being upper-cased", set: true, value: "true", want: true},
		{what: "True is on", set: true, value: "True", want: true},
		{what: "1 is on", set: true, value: "1", want: true},
		{what: "0 is off", set: true, value: "0", want: false},
		{what: "FALSE is off", set: true, value: "FALSE", want: false},
		{what: "yes is off -- this is not a general truthiness test", set: true, value: "yes", want: false},
		// Nothing is trimmed, which is DEPLOY_CPIPES_NATIVE's behaviour rather than a choice
		// made here: two gates a deployment sets together must not disagree about what "on"
		// looks like. A space is a typo and silently repairing one hides it.
		{what: "a leading space is off, nothing being trimmed", set: true, value: " 1", want: false},
		{what: "a trailing space is off", set: true, value: "TRUE ", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			if tc.set {
				t.Setenv(pythonDeployVariable, tc.value)
			} else {
				unsetEnv(t, pythonDeployVariable)
			}
			if got := DeployCpipesPythonFromEnv(); got != tc.want {
				t.Errorf("DeployCpipesPythonFromEnv with %s=%q gave %v, want %v",
					pythonDeployVariable, tc.value, got, tc.want)
			}
		})
	}
}

// The selector the third arm branches on is its own, and neither of the two the Choice already
// reads.
//
// **This is not a tautology test.** ecsOrLambdaChoice is a first-match-wins array and the
// Python arm is added ahead of the ECS one, so a flag literal that collided with
// $.useECSReducingTask would silently send every ECS-selected reducing step to the Python
// worker -- on a deployment that had set the gate, with nothing in the template looking wrong.
// A collision with $.noMoreTask would divert the loop's own exit.
func TestCpipesPythonReducingFlagIsItsOwn(t *testing.T) {
	if cpipesPythonReducingFlag != pythonReducingFlag {
		t.Errorf("cpipesPythonReducingFlag is %q, want %q -- changing it changes which field of"+
			" ComputePipesRun selects the Python worker, and the field has no producer yet"+
			" (P9-I49), so nothing else would go red",
			cpipesPythonReducingFlag, pythonReducingFlag)
	}
	for _, taken := range []string{ecsReducingFlag, noMoreTaskFlag} {
		if cpipesPythonReducingFlag == taken {
			t.Errorf("cpipesPythonReducingFlag is %q, which ecsOrLambdaChoice already branches"+
				" on; the Python arm is evaluated first, so it would shadow that arm", taken)
		}
	}
}
