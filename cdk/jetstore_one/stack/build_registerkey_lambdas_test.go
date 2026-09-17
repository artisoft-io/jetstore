package stack

import "testing"

// The register-key Lambda's entry is resolved from the environment as of 2026-09-16, and these
// tests hold one claim: with JETS_REGISTER_KEY_LAMBDA_ENTRY unset, the resolved path is byte for
// byte the literal the property carried before the variable existed, so a deployment that has
// never heard of it synthesises the Lambda it synthesises today.
//
// They exist because the resolution changed shape after the claim was measured. The variable
// arrived on main with the default written inline, where an A/B cdk synth established that an
// unset variable leaves the template and assets manifest byte-identical; the inline default was
// then collapsed onto lambdaEntryOrDefault when this work reached jets_ai, which already carried
// that helper. A synth is a minute and a set of plausible account values, so what guards the
// collapse is this: the only input to the construct that either form can move is the resolved
// string.
//
// The literal is written out here rather than compared against registerKeyLambdaDefaultEntry,
// deliberately -- comparing a constant with itself would pass after somebody changed it, which is
// the change these tests exist to catch. It is spelled registerKeyEntryDeployedToday rather than
// reusing entryDeployedToday (build_cpipes_lambdas_test.go) because that name is already spent in
// this package on the cpipes node Lambda's literal, and they are different paths.
const registerKeyEntryDeployedToday = "lambdas/register_keys/register_keys_v2"

func TestRegisterKeyLambdaEntryUnsetIsTheEntryDeployedToday(t *testing.T) {
	unsetEnv(t, "JETS_REGISTER_KEY_LAMBDA_ENTRY")
	got := lambdaEntryOrDefault("JETS_REGISTER_KEY_LAMBDA_ENTRY", registerKeyLambdaDefaultEntry)
	if got != registerKeyEntryDeployedToday {
		t.Errorf("unset JETS_REGISTER_KEY_LAMBDA_ENTRY resolved to %q, want %q",
			got, registerKeyEntryDeployedToday)
	}
}

func TestRegisterKeyLambdaDefaultEntryIsTheEntryDeployedToday(t *testing.T) {
	if registerKeyLambdaDefaultEntry != registerKeyEntryDeployedToday {
		t.Errorf("registerKeyLambdaDefaultEntry is %q, want %q -- changing it changes the stack that"+
			" every deployment not setting JETS_REGISTER_KEY_LAMBDA_ENTRY gets, and that is the main"+
			" ingest path rather than an optional component",
			registerKeyLambdaDefaultEntry, registerKeyEntryDeployedToday)
	}
}

// TestRegisterKeyLambdaEntryEmptyIsUnset is the one behaviour of the collapse worth asserting
// separately from TestLambdaEntryOrDefault, which already covers it for the cpipes variable:
// `export JETS_REGISTER_KEY_LAMBDA_ENTRY=` in a deploy script must mean the same thing as never
// having written the line, because for this Lambda the alternative reading is a stack with no
// ingest.
func TestRegisterKeyLambdaEntryEmptyIsUnset(t *testing.T) {
	t.Setenv("JETS_REGISTER_KEY_LAMBDA_ENTRY", "")
	got := lambdaEntryOrDefault("JETS_REGISTER_KEY_LAMBDA_ENTRY", registerKeyLambdaDefaultEntry)
	if got != registerKeyEntryDeployedToday {
		t.Errorf("empty JETS_REGISTER_KEY_LAMBDA_ENTRY resolved to %q, want %q",
			got, registerKeyEntryDeployedToday)
	}
}
