package main

import (
	"flag"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// testFlags is a FlagSet holding the flags these tests set, so that the command
// line's own flags are never set and no test leaves one visited for another.
func testFlags(t *testing.T, set ...string) map[string]bool {
	t.Helper()
	fs := flag.NewFlagSet("rules_bench", flag.ContinueOnError)
	fs.String("legacy", "", "")
	fs.String("observe", "", "")
	fs.String("stats", "", "")
	fs.Bool("per_loop", false, "")
	fs.Int("sample_ms", 10, "")
	if err := fs.Parse(set); err != nil {
		t.Fatal(err)
	}
	return visitedFlags(fs)
}

// -legacy chose a retraction mode until the LEGACY retraction mode was deleted
// (rules_benchmark Q-95): given, whatever its value, it is refused naming the
// deletion; not given, nothing is refused.
func TestLegacyFlagIsRefused(t *testing.T) {
	if err := refuseLegacyFlag(testFlags(t)); err != nil {
		t.Errorf("-legacy not given: %v", err)
	}
	for _, v := range []string{"legacy", "fixed", ""} {
		err := refuseLegacyFlag(testFlags(t, "-legacy="+v))
		if err == nil || !strings.Contains(err.Error(), rulestest.LegacyRetractionDeleted) || !strings.Contains(err.Error(), "-legacy") {
			t.Errorf("-legacy %q: %v, want a refusal naming the deletion", v, err)
		}
	}
}

// -observe is untimed: with it, the timing outputs are refused, naming -observe
// (rules_benchmark BU.2).
func TestObserveRefusesTheTimingFlags(t *testing.T) {
	if err := refuseTimingFlags(testFlags(t, "-observe", t.TempDir()), "-observe"); err != nil {
		t.Errorf("-observe alone: %v", err)
	}
	err := refuseTimingFlags(testFlags(t, "-observe", t.TempDir(), "-per_loop", "-sample_ms", "5"), "-observe")
	if err == nil || !strings.Contains(err.Error(), "-observe is an untimed run and refuses -per_loop, -sample_ms") {
		t.Errorf("-observe -per_loop -sample_ms: %v", err)
	}
	for name := range visitedFlags(flag.CommandLine) {
		if !strings.HasPrefix(name, "test.") { // go test's own
			t.Errorf("the command line's -%s was set", name)
		}
	}
}

// -stats is untimed: with it, the timing outputs are refused, naming -stats
// (rules_benchmark BX.2).
func TestStatsRefusesTheTimingFlags(t *testing.T) {
	if err := refuseTimingFlags(testFlags(t, "-stats", "s.json"), "-stats"); err != nil {
		t.Errorf("-stats alone: %v", err)
	}
	err := refuseTimingFlags(testFlags(t, "-stats", "s.json", "-sample_ms", "5"), "-stats")
	if err == nil || !strings.Contains(err.Error(), "-stats is an untimed run and refuses -sample_ms") {
		t.Errorf("-stats -sample_ms: %v", err)
	}
	if f := flag.Lookup("stats"); f == nil || !strings.Contains(f.Usage, "I-231") || !strings.Contains(f.Usage, "when the rule set ends") {
		t.Errorf("-stats must say when statistics are taken (I-231): %v", f)
	}
}
