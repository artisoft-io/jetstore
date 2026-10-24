package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// -legacy chose a retraction mode until the LEGACY retraction mode was deleted
// (rules_benchmark Q-95): given, whatever its value, it is refused naming the
// deletion; not given, nothing is refused.
func TestLegacyFlagIsRefused(t *testing.T) {
	if err := refuseLegacyFlag(); err != nil {
		t.Errorf("-legacy not given: %v", err)
	}
	for _, v := range []string{"legacy", "fixed", ""} {
		if err := flag.Set("legacy", v); err != nil {
			t.Fatal(err)
		}
		err := refuseLegacyFlag()
		if err == nil || !strings.Contains(err.Error(), rulestest.LegacyRetractionDeleted) || !strings.Contains(err.Error(), "-legacy") {
			t.Errorf("-legacy %q: %v, want a refusal naming the deletion", v, err)
		}
	}
}

// -observe is untimed: with it, the timing outputs are refused, naming -observe
// (rules_benchmark BU.2).
func TestObserveRefusesTheTimingFlags(t *testing.T) {
	defer func() {
		flag.Set("observe", "")
		flag.Set("per_loop", "false")
	}()
	if err := flag.Set("observe", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := refuseTimingFlags(); err != nil {
		t.Errorf("-observe alone: %v", err)
	}
	if err := flag.Set("per_loop", "true"); err != nil {
		t.Fatal(err)
	}
	if err := refuseTimingFlags(); err == nil || !strings.Contains(err.Error(), "-observe is an untimed run and refuses -per_loop") {
		t.Errorf("-observe -per_loop: %v", err)
	}
}
