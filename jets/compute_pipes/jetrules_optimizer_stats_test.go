package compute_pipes_test

// The jetrules step's optimizer_statistics option (rules_benchmark Phase 9,
// CC.1 and CC.3), on the C++ engine over a compiled copy of test_ws, in a child
// process as TestExecuteRulesCharacterisation is. Run it with -count=1.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/awsi"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/optstats"
	"github.com/artisoft-io/jetstore/jets/workspace"
)

const optimizerStatsHomeEnv = "JETS_OPTIMIZER_STATS_TEST_HOME"

func TestOptimizerStatisticsOption(t *testing.T) {
	if os.Getenv(optimizerStatsHomeEnv) == "" {
		rerunOverScratchTestWs(t, optimizerStatsHomeEnv)
		return
	}
	if out, err := workspace.CompileWorkspace(nil, "test_ws", "cc3",
		workspace.SkipAssetValidation("optimizer statistics test: test_ws has no user flows")); err != nil {
		t.Fatalf("compiling the scratch test_ws: %v\n%s", err, out)
	}
	load := func(t *testing.T) *compute_pipes.OptimizerStats {
		t.Helper()
		st, err := compute_pipes.LoadOptimizerStats(workspace.WorkspaceDbPath())
		if err != nil {
			t.Fatal(err)
		}
		return st
	}
	// Partials are written through the uploader, here into a map.
	uploaded := map[string][]byte{}
	saved := compute_pipes.OptimizerStatsUploader
	compute_pipes.OptimizerStatsUploader = func(bucket, key string, buf []byte) error {
		uploaded[key] = append([]byte(nil), buf...)
		return nil
	}
	defer func() { compute_pipes.OptimizerStatsUploader = saved }()

	// Off, the step counts nothing and its output is unchanged; on, it extracts
	// the same rows and counts each bundle that did not fail once per rule set: a
	// bundle with a jets:exception contributes nothing.
	t.Run("option_off_changes_nothing", func(t *testing.T) {
		off := newHarness(t, executeRulesSequence, &compute_pipes.JetrulesSpec{})
		st := load(t)
		on := newHarness(t, executeRulesSequence, &compute_pipes.JetrulesSpec{OptimizerStatistics: true})
		on.worker.SetOptimizerStatsForTest(st)
		for _, mode := range []string{"ok", "exception"} {
			a, b := off.execute(mode), on.execute(mode)
			if a.err != nil || b.err != nil {
				t.Fatalf("%s: errors %v and %v", mode, a.err, b.err)
			}
			if !equalRows(a.cases, b.cases) || len(a.persons) != len(b.persons) ||
				strings.Join(a.messages(), "|") != strings.Join(b.messages(), "|") {
				t.Errorf("%s: off %v %d %q, on %v %d %q", mode, a.cases, len(a.persons), a.messages(),
					b.cases, len(b.persons), b.messages())
			}
		}
		f := st.File()
		if len(f.RuleSets) != 2 {
			t.Fatalf("on: rule sets %v, want the sequence's two", f.RuleSets)
		}
		for name, rs := range f.RuleSets {
			if rs.Sessions != 1 {
				t.Errorf("on: %s counted %d sessions, want the ok bundle only", name, rs.Sessions)
			}
		}
	})

	// Partials from k nodes, merged, equal one node's statistics over the same
	// bundles, and each node's partial goes to its own key under the session's
	// prefix.
	t.Run("partials_merge_exactly", func(t *testing.T) {
		modes := []string{"ok", "exception", "ok", "ok", "exception"}
		whole := load(t)
		hw := newHarness(t, executeRulesSequence, &compute_pipes.JetrulesSpec{})
		hw.worker.SetOptimizerStatsForTest(whole)
		for _, m := range modes {
			if r := hw.execute(m); r.err != nil {
				t.Fatal(r.err)
			}
		}
		const k = 3
		for node := range k {
			st := load(t)
			h := newHarness(t, executeRulesSequence, &compute_pipes.JetrulesSpec{})
			h.worker.SetOptimizerStatsForTest(st)
			for i, m := range modes {
				if i%k == node {
					if r := h.execute(m); r.err != nil {
						t.Fatal(r.err)
					}
				}
			}
			if err := compute_pipes.WriteOptimizerStatsPartialForTest("Optimizer_Statistics", "17", "", node, st); err != nil {
				t.Fatal(err)
			}
		}
		prefix := optstats.PartialsPrefix(awsi.JetStoreStagePrefix(), "Optimizer_Statistics", "17")
		var files []*optstats.File
		for node := range k {
			key := optstats.PartialKey(awsi.JetStoreStagePrefix(), "Optimizer_Statistics", "17", "", node)
			if !strings.HasPrefix(key, prefix) {
				t.Fatalf("partial key %s is not under %s", key, prefix)
			}
			f, err := optstats.Decode(bytes.NewReader(uploaded[key]))
			if err != nil {
				t.Fatalf("node %d's partial %s: %v", node, key, err)
			}
			files = append(files, f)
		}
		merged, err := optstats.Merge(files...)
		if err != nil {
			t.Fatal(err)
		}
		a, err := optstats.Marshal(merged)
		if err != nil {
			t.Fatal(err)
		}
		b, err := optstats.Marshal(whole.File())
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%d partials merged:\n%s\none node over the same bundles:\n%s", k, a, b)
		}
		// The three ok bundles; the two with a jets:exception contribute nothing.
		if rs := merged.ForRuleSet(executeRulesRuleFile); rs == nil || rs.Sessions != 3 {
			t.Errorf("%s: %+v, want 3 sessions", executeRulesRuleFile, rs)
		}
	})
}

func equalRows(a, b [][]any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
