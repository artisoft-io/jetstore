package rete

// The retraction mode's labels. That the mode reaches a compiled network through
// NewReteMetaStoreFactory is retraction_mode_factory_test.go, in package rete_test,
// because it compiles with package workspace, which imports this package.

import "testing"

func TestRetractionModeLabels(t *testing.T) {
	want := []string{"legacy", "fixed-1", "fixed-2", "fixed"}
	for i, mode := range AllRetractionModes() {
		if got := mode.Label(); got != want[i] {
			t.Errorf("mode %d: Label() = %q, want %q", i, got, want[i])
		}
		if got := mode.IsLegacy(); got != (i == 0) {
			t.Errorf("mode %s: IsLegacy() = %v", mode.Label(), got)
		}
	}
	if !LegacyRetraction.IsLegacy() || (RetractionMode{}) != LegacyRetraction {
		t.Error("the zero value must be legacy for both defects")
	}
}
