package rulestest

import "testing"

// LegacyConfigOf is ModeLabel's inverse, for the four labels and nothing else.
func TestLegacyConfigOfRoundTrips(t *testing.T) {
	for _, label := range []string{LegacyModeLegacy, LegacyModeFixed1, LegacyModeFixed2, LegacyModeFixed} {
		l, err := LegacyConfigOf(label)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if got := l.ModeLabel(); got != label {
			t.Errorf("LegacyConfigOf(%q).ModeLabel() = %q", label, got)
		}
	}
	if _, err := LegacyConfigOf("fixed-3"); err == nil {
		t.Error("an unknown label is accepted")
	}
}
