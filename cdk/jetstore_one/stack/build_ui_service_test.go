package stack

import (
	"testing"
)

// JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC unset must leave the property to CDK (60 seconds behind a
// load balancer), so a deployment that never sets it synthesises the template it does today. The
// synth A/B in the pull request that added it held that claim; this test holds the parser.
func TestParseUiHealthCheckGracePeriod(t *testing.T) {
	tests := []struct {
		what    string
		value   string
		want    int
		wantNil bool
		wantErr bool
	}{
		{what: "empty leaves CDK's default", value: "", wantNil: true},
		{what: "seconds", value: "600", want: 600},
		{what: "zero is a value, not unset", value: "0", want: 0},
		{what: "negative fails", value: "-1", wantErr: true},
		{what: "a unit fails", value: "600s", wantErr: true},
		{what: "a fraction fails", value: "1.5", wantErr: true},
		{what: "a space fails, nothing being trimmed", value: " 600", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			got, err := parseUiHealthCheckGracePeriod(tc.value)
			switch {
			case tc.wantErr:
				if err == nil {
					t.Fatalf("parseUiHealthCheckGracePeriod(%q) gave %v, want an error", tc.value, got)
				}
			case err != nil:
				t.Fatalf("parseUiHealthCheckGracePeriod(%q): %v", tc.value, err)
			case tc.wantNil:
				if got != nil {
					t.Errorf("parseUiHealthCheckGracePeriod(%q) gave %d, want nil", tc.value, *got)
				}
			case got == nil || *got != tc.want:
				t.Errorf("parseUiHealthCheckGracePeriod(%q) gave %v, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestUiHealthCheckGracePeriodUnsetIsNil(t *testing.T) {
	unsetEnv(t, "JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC")
	if got := UiHealthCheckGracePeriodSec(); got != nil {
		t.Errorf("unset JETS_UI_HEALTH_CHECK_GRACE_PERIOD_SEC gave %d, want nil (CDK's default)", *got)
	}
}
