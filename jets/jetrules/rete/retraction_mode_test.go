package rete

// THE RETRACTION MODE REACHES A COMPILED NETWORK THROUGH THE FACTORY.
//
// retract_recorded_consequents_test.go and loop_through_negation_test.go build their
// networks by hand and set the mode on the meta store themselves, so neither shows that
// NewReteMetaStoreFactory passes WithRetractionMode to AddConsequentTerm and to the
// meta store it builds. This test does, on test_ws compiled into a scratch directory
// (compileTestWorkspace, join_values_rule_test.go): test_join_values_main.jr's
// consequents are join_values aggregates, which read the graph and mint nothing, so
// their vertexes record only with LEGACY defect 1 turned off.
//
// -count=1: it compiles files outside the Go module's cache key.

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

func TestFactoryAppliesRetractionMode(t *testing.T) {
	home := compileTestWorkspace(t)
	savedHome, savedPrefix := workspaceHome, wprefix
	t.Cleanup(func() { workspaceHome, wprefix = savedHome, savedPrefix })
	workspaceHome, wprefix = home, "test_ws"

	// recording counts the vertexes that record, and the ones whose consequents read
	// the graph and mint nothing -- the vertexes defect 1 is about.
	recording := func(ms *ReteMetaStore) (records, graphOnly int) {
		for _, nv := range ms.NodeVertices {
			if nv.RecordsConsequents {
				records++
			}
			reads, mints := false, false
			for _, an := range nv.ConsequentAlphaNodes {
				reads = reads || an.ReadsGraph()
				mints = mints || an.MintsResource()
			}
			if reads && !mints {
				graphOnly++
			}
		}
		return
	}

	for _, c := range []struct {
		name string
		opts []FactoryOption
		want RetractionMode
	}{
		{"no option", nil, LegacyRetraction},
		{"legacy", []FactoryOption{WithRetractionMode(LegacyRetraction)}, LegacyRetraction},
		{"fixed-1", []FactoryOption{WithRetractionMode(RetractionMode{ReplayGraphReadingConsequents: true})},
			RetractionMode{ReplayGraphReadingConsequents: true}},
		{"fixed", []FactoryOption{WithRetractionMode(RetractionMode{ReplayGraphReadingConsequents: true, EraseWithdrawnRows: true})},
			RetractionMode{ReplayGraphReadingConsequents: true, EraseWithdrawnRows: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			f, err := NewReteMetaStoreFactory(joinValuesMainRuleFile, c.opts...)
			if err != nil {
				t.Fatalf("loading the compiled workspace: %v", err)
			}
			if f.RetractionMode != c.want {
				t.Errorf("factory mode = %+v, want %+v", f.RetractionMode, c.want)
			}
			ms := f.MetaStoreLookup[joinValuesMainRuleFile]
			if ms == nil {
				t.Fatalf("no metastore for %s", joinValuesMainRuleFile)
			}
			if ms.RetractionMode != c.want {
				t.Errorf("meta store mode = %+v, want %+v", ms.RetractionMode, c.want)
			}
			records, graphOnly := recording(ms)
			if graphOnly == 0 {
				t.Fatal("the rule file has no consequent that reads the graph and mints nothing: this test proves nothing")
			}
			if c.want.ReplayGraphReadingConsequents {
				if records < graphOnly {
					t.Errorf("%d vertexes record, want at least the %d graph-reading ones", records, graphOnly)
				}
			} else if records != 0 {
				t.Errorf("%d vertexes record in a legacy mode for defect 1, want 0 (the file mints nothing)", records)
			}
		})
	}
}
