package stack

import (
	"testing"
)

// The two opt-outs, JETS_SKIP_REPORTS_SM and JETS_VPC_ENDPOINTS_EXCLUDE, must leave a deployment
// that sets neither with the stack it has today. These tests hold the readers; the synthesised
// template, byte-identical with both unset, is what held the claim when the toggles were added.
//
// unsetEnv is build_cpipes_lambdas_test.go's, in this same package.

const skipReportsSMVariable = "JETS_SKIP_REPORTS_SM"

func TestSkipReportsSMFromEnv(t *testing.T) {
	tests := []struct {
		what  string
		set   bool
		value string
		want  bool
	}{
		{what: "unset keeps reportsSM", want: false},
		{what: "empty keeps reportsSM", set: true, value: "", want: false},
		{what: "TRUE skips", set: true, value: "TRUE", want: true},
		{what: "true skips, the value being upper-cased", set: true, value: "true", want: true},
		{what: "1 skips", set: true, value: "1", want: true},
		{what: "FALSE keeps", set: true, value: "FALSE", want: false},
		{what: "0 keeps", set: true, value: "0", want: false},
		{what: "yes keeps", set: true, value: "yes", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			if tc.set {
				t.Setenv(skipReportsSMVariable, tc.value)
			} else {
				unsetEnv(t, skipReportsSMVariable)
			}
			if got := SkipReportsSMFromEnv(); got != tc.want {
				t.Errorf("SkipReportsSMFromEnv with %s=%q gave %v, want %v",
					skipReportsSMVariable, tc.value, got, tc.want)
			}
		})
	}
}

// The short names come from CDK, not from this package, so the list below is what a deployment
// writes in JETS_VPC_ENDPOINTS_EXCLUDE and what the README documents. A CDK upgrade that renamed
// one would fail here rather than at a client's synth.
func TestInterfaceVpcEndpointShortNames(t *testing.T) {
	want := []string{"ecr.dkr", "ecr.api", "config", "kms", "sns", "sqs", "ecs-agent",
		"ecs-telemetry", "ecs", "lambda", "secretsmanager", "git-codecommit", "sync-states",
		"states", "logs", "monitoring", "events"}
	eps := interfaceVpcEndpoints()
	if len(eps) != len(want) {
		t.Fatalf("interfaceVpcEndpoints has %d endpoints, want %d", len(eps), len(want))
	}
	for i, ep := range eps {
		if got := *ep.service.ShortName(); got != want[i] {
			t.Errorf("endpoint %d (%s) has short name %q, want %q", i, ep.id, got, want[i])
		}
	}
}

func TestParseVpcEndpointsExclude(t *testing.T) {
	tests := []struct {
		what    string
		value   string
		want    []string
		wantErr bool
	}{
		{what: "empty excludes nothing", value: "", want: nil},
		{what: "one name", value: "kms", want: []string{"kms"}},
		{what: "trimmed and lower-cased", value: " KMS , ecs-agent ", want: []string{"kms", "ecs-agent"}},
		{what: "empty items skipped", value: "kms,,sns,", want: []string{"kms", "sns"}},
		{what: "an unknown name fails", value: "kms,kmss", wantErr: true},
		{what: "execute-api cannot be excluded", value: "execute-api", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			got, err := parseVpcEndpointsExclude(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseVpcEndpointsExclude(%q) gave %v, want an error", tc.value, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseVpcEndpointsExclude(%q): %v", tc.value, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseVpcEndpointsExclude(%q) gave %v, want %v", tc.value, got, tc.want)
			}
			for _, name := range tc.want {
				if !got[name] {
					t.Errorf("parseVpcEndpointsExclude(%q) is missing %q", tc.value, name)
				}
			}
		})
	}
}
