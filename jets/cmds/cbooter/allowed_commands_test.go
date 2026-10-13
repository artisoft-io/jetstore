package main

// Tests for the launcher's allow-list as the scheduled agentic detection task uses it
// (agentic_ai I-861). The task runs `cbooter anomaly_detect` and `cbooter shadow_triage`
// (cdk/jetstore_one/stack/build_agentic_detect.go), so both names must be allowed, and
// neither may be made to wait on a workspace it never opens.

import (
	"strings"
	"testing"
)

func TestDetectionCommandsAreAllowed(t *testing.T) {
	for _, cmd := range []string{"anomaly_detect", "shadow_triage"} {
		if !allowedCommands[cmd] {
			t.Errorf("%s is not in allowedCommands; the scheduled detection task would be refused", cmd)
		}
		if !strings.Contains(allowedCommandNames(), cmd) {
			t.Errorf("allowedCommandNames() = %q does not name %s", allowedCommandNames(), cmd)
		}
	}
}

// The commands that were allowed before I-861 keep their behaviour: every one but
// infer_server stages a workspace, as it did when the test was `cmd != "infer_server"`.
func TestNeedsWorkspace(t *testing.T) {
	want := map[string]bool{
		"apiserver":            true,
		"run_reports":          true,
		"cpipes_server":        true,
		"cpipes_native_server": true,
		"infer_server":         false,
		"anomaly_detect":       false,
		"shadow_triage":        false,
	}
	for cmd := range allowedCommands {
		if _, ok := want[cmd]; !ok {
			t.Errorf("%s is allowed and this test does not say whether it needs a workspace", cmd)
		}
	}
	for cmd, w := range want {
		if got := needsWorkspace(cmd); got != w {
			t.Errorf("needsWorkspace(%q) = %v, want %v", cmd, got, w)
		}
	}
}

func TestAllowedCommandNamesIsSorted(t *testing.T) {
	names := strings.Split(allowedCommandNames(), ", ")
	if len(names) != len(allowedCommands) {
		t.Fatalf("allowedCommandNames() lists %d names, allowedCommands has %d", len(names), len(allowedCommands))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("allowedCommandNames() is not sorted: %q before %q", names[i-1], names[i])
		}
	}
}
