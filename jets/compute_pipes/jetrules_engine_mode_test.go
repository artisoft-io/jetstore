package compute_pipes_test

// THE RETRACTION MODE REACHES THE GO ENGINE THROUGH THE FACTORY INTERFACE
// (rules_unit_test_harness Phase 1, task AF.5).
//
// compute_pipes.WithRetractionMode is the only way a JetRulesFactory builds an
// engine with a LEGACY defect turned off, and only the rules unit test harness
// passes it. A pipeline calls NewJetRuleEngine with no option
// (pipe_transformation_jetrules.go) and must get the legacy engine. What the modes
// do to a network is tested in jets/jetrules/rete; this pins the plumbing. The
// native adaptor's half is jetrules_native_adaptor/engine_mode_test.go, which
// needs libjets.so. Runs in a child process over a compiled copy of test_ws
// (inCompiledTestWs); run it with -count=1.

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/compute_pipes/jetrules_go_adaptor"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

const engineModeProcess = "jet_rules/test_join_values_main.jr"

func TestGoEngineRetractionMode(t *testing.T) {
	inCompiledTestWs(t, func(t *testing.T) {
		build := func(opts ...compute_pipes.EngineOption) compute_pipes.RetractionMode {
			t.Helper()
			engine, err := jetrules_go_adaptor.NewJetRulesFactory().NewJetRuleEngine(nil, engineModeProcess, false, opts...)
			if err != nil {
				t.Fatalf("building the Go engine: %v", err)
			}
			defer engine.Release()
			moded, ok := engine.(interface {
				RetractionMode() compute_pipes.RetractionMode
			})
			if !ok {
				t.Fatal("the Go engine does not report its retraction mode")
			}
			return moded.RetractionMode()
		}
		if got := build(); !got.IsLegacy() {
			t.Errorf("no option (a pipeline): mode %s, want legacy", got.Label())
		}
		for _, mode := range rete.AllRetractionModes() {
			if got := build(compute_pipes.WithRetractionMode(mode)); got != mode {
				t.Errorf("WithRetractionMode(%s): engine built in %s", mode.Label(), got.Label())
			}
		}
	})
}

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
