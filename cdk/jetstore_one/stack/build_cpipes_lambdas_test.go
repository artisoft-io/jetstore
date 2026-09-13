package stack

import (
	"os"
	"testing"
)

// The cpipes node Lambda's entry is resolved from the environment as of 2026-09-12, and
// these tests exist to hold one claim: with JETS_CPIPES_NODE_LAMBDA_ENTRY unset, the
// resolved path is byte for byte the literal the property carried before the variable
// existed, so a deployment that has never heard of it synthesises the Lambda it synthesises
// today.
//
// The literal is written out here rather than compared against cpipesNodeLambdaDefaultEntry,
// deliberately: comparing a constant with itself would pass after somebody changed it, which
// is the change these tests exist to catch.
const entryDeployedToday = "lambdas/compute_pipes/cp_node"

// unsetEnv makes name genuinely unset for the duration of the test. t.Setenv records the
// variable's prior state and restores it at cleanup -- including restoring it to unset --
// so unsetting after it is safe; t.Setenv(name, "") on its own would leave the variable
// present and empty, which is a different input.
func unsetEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "placeholder")
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("while unsetting %s: %v", name, err)
	}
}

func TestCpipesNodeLambdaEntryUnsetIsTheEntryDeployedToday(t *testing.T) {
	unsetEnv(t, "JETS_CPIPES_NODE_LAMBDA_ENTRY")
	got := lambdaEntryOrDefault("JETS_CPIPES_NODE_LAMBDA_ENTRY", cpipesNodeLambdaDefaultEntry)
	if got != entryDeployedToday {
		t.Errorf("unset JETS_CPIPES_NODE_LAMBDA_ENTRY resolved to %q, want %q", got, entryDeployedToday)
	}
}

func TestCpipesNodeLambdaDefaultEntryIsTheEntryDeployedToday(t *testing.T) {
	if cpipesNodeLambdaDefaultEntry != entryDeployedToday {
		t.Errorf("cpipesNodeLambdaDefaultEntry is %q, want %q -- changing it changes the stack that"+
			" every deployment not setting JETS_CPIPES_NODE_LAMBDA_ENTRY gets",
			cpipesNodeLambdaDefaultEntry, entryDeployedToday)
	}
}

func TestLambdaEntryOrDefault(t *testing.T) {
	const name = "JETS_CPIPES_NODE_LAMBDA_ENTRY"
	const siteEntry = "/home/ws/cedargate_ws/go/lambdas/cp_node"
	tests := []struct {
		what  string
		set   bool
		value string
		want  string
	}{
		{what: "unset falls back to the stock entry", want: entryDeployedToday},
		{what: "empty is treated as unset", set: true, value: "", want: entryDeployedToday},
		{what: "a site entry wins", set: true, value: siteEntry, want: siteEntry},
		{what: "the value is not trimmed", set: true, value: " lambdas/x ", want: " lambdas/x "},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			if tc.set {
				t.Setenv(name, tc.value)
			} else {
				unsetEnv(t, name)
			}
			if got := lambdaEntryOrDefault(name, cpipesNodeLambdaDefaultEntry); got != tc.want {
				t.Errorf("lambdaEntryOrDefault = %q, want %q", got, tc.want)
			}
		})
	}
}
