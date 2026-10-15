package stack

import "testing"

// The variable is written out rather than compared against agenticDetectHourVariable, for
// build_cpipes_lambdas_test.go's stated reason: comparing a constant with itself passes the
// day somebody renames it, and a rename is what turns every deployment's schedule off.
const agenticDetectVariable = "JETS_AGENTIC_DETECT_HOUR_UTC"

func TestAgenticDetectUnsetIsOff(t *testing.T) {
	unsetEnv(t, agenticDetectVariable)
	_, ok, err := AgenticDetectHourFromEnv()
	if ok || err != nil {
		t.Errorf("unset %s gave ok=%v err=%v; unset must build nothing and fail nothing, which is what"+
			" keeps a deployment that has never heard of it on the stack it synthesises today",
			agenticDetectVariable, ok, err)
	}
}

func TestAgenticDetectHourFromEnv(t *testing.T) {
	tests := []struct {
		what    string
		value   string
		want    string
		wantOk  bool
		wantErr bool
	}{
		{what: "empty is off", value: ""},
		{what: "0 is midnight", value: "0", want: "0", wantOk: true},
		{what: "7 is the purge lambda's default hour", value: "7", want: "7", wantOk: true},
		{what: "23 is the last hour", value: "23", want: "23", wantOk: true},
		{what: "a leading zero is normalised", value: "07", want: "7", wantOk: true},
		{what: "24 is refused", value: "24", wantErr: true},
		{what: "a negative hour is refused", value: "-1", wantErr: true},
		{what: "a cron expression is refused", value: "0 7 * * ? *", wantErr: true},
		{what: "a space is a typo, and refused rather than trimmed", value: " 7", wantErr: true},
		{what: "a word is refused", value: "daily", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.what, func(t *testing.T) {
			t.Setenv(agenticDetectVariable, tc.value)
			got, ok, err := AgenticDetectHourFromEnv()
			if (err != nil) != tc.wantErr {
				t.Fatalf("%s=%q: err = %v, want an error: %v", agenticDetectVariable, tc.value, err, tc.wantErr)
			}
			if ok != tc.wantOk || got != tc.want {
				t.Errorf("%s=%q: got (%q, %v), want (%q, %v)", agenticDetectVariable, tc.value, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}
