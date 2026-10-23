package compute_pipes_test

// THE ENGINE OPTIONS' PLUMBING (rules_unit_test_harness Phase 1, task AF.5).
//
// compute_pipes.WithRetractionMode is the only way a JetRulesFactory builds an
// engine with a LEGACY defect turned off, and only the rules unit test harness
// passes it. A pipeline calls NewJetRuleEngine with no option
// (pipe_transformation_jetrules.go) and must get the legacy engine. That the mode
// reaches the C++ engine is jetrules_native_adaptor/engine_mode_test.go; the Go
// engine's half was here until that engine's retirement (October 2026).

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

func TestApplyEngineOptions(t *testing.T) {
	if o := compute_pipes.ApplyEngineOptions(); !o.RetractionMode.IsLegacy() {
		t.Errorf("no options: %s, want legacy", o.RetractionMode.Label())
	}
	if o := compute_pipes.ApplyEngineOptions(nil); !o.RetractionMode.IsLegacy() {
		t.Errorf("a nil option: %s, want legacy", o.RetractionMode.Label())
	}
	fixed := compute_pipes.RetractionMode{ReplayGraphReadingConsequents: true, EraseWithdrawnRows: true}
	if o := compute_pipes.ApplyEngineOptions(compute_pipes.WithRetractionMode(fixed)); o.RetractionMode != fixed {
		t.Errorf("WithRetractionMode(fixed): %s", o.RetractionMode.Label())
	}
}
