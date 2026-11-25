package triage

// Locus rows_lost_silently judges each step against its own history, as the
// volume detector does: Gather reads the history and the locus passes it to the
// detector. Without it, every step that reduces or joins by design is present
// on every run. Needs JETS_TEST_DSN; skipped otherwise.

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRowsLostSilentlyJudgesAStepAgainstItsOwnHistory(t *testing.T) {
	pool := freshDB(t, "triage_volume_history", migratedTables)

	// Four earlier runs of a step that writes 2% of what it reads.
	for i := 4; i >= 1; i-- {
		session := "s-prior" + string(rune('0'+i))
		start := now.Add(-time.Duration(i) * 24 * time.Hour)
		k := header(t, pool, "client3", "review", "claims", session, "completed", start)
		workerRow(t, pool, k, "client3", "review", session, "reducing01", 0, "completed", 100000, 2000, "", start)
	}

	t.Run("steady", func(t *testing.T) {
		start := now.Add(-2 * time.Hour)
		k := header(t, pool, "client3", "review", "claims", "s-steady", "completed", start)
		workerRow(t, pool, k, "client3", "review", "s-steady", "reducing01", 0, "completed", 100000, 1900, "", start)
		f := wantVerdict(t, classify(t, pool, "s-steady", 0), LocusRowsLostSilently, Absent)
		if !strings.Contains(f.Basis, "volume_collapse@2") || !strings.Contains(f.Basis, "median ratio") {
			t.Errorf("the absent basis does not say what it compared against: %s", f.Basis)
		}
	})

	t.Run("collapsed", func(t *testing.T) {
		start := now.Add(-time.Hour)
		k := header(t, pool, "client3", "review", "claims", "s-collapsed", "completed", start)
		workerRow(t, pool, k, "client3", "review", "s-collapsed", "reducing01", 0, "completed", 100000, 200, "", start)
		f := wantVerdict(t, classify(t, pool, "s-collapsed", 0), LocusRowsLostSilently, Present)
		if !strings.Contains(f.Basis, "against the step's own history") {
			t.Errorf("the present basis is not the step's history: %s", f.Basis)
		}
	})
}

// The history is read with the bounds of the classifier that judges it. A
// classifier configured with a lower MinInput counts as history the runs its
// detector would count, and one configured with a shorter HistoryRuns reads
// fewer of them; Gather, with Default's bounds, reads neither.
func TestTheVolumeHistoryIsReadWithTheClassifiersBounds(t *testing.T) {
	pool := freshDB(t, "triage_volume_bounds", migratedTables)
	ctx := context.Background()

	// Four earlier runs of a step that writes 2% of what it reads, each reading
	// 500 rows: below Default's MinInput of 1000, so not history by default.
	for i := 4; i >= 1; i-- {
		session := "s-small" + string(rune('0'+i))
		start := now.Add(-time.Duration(i) * 24 * time.Hour)
		k := header(t, pool, "client4", "review", "claims", session, "completed", start)
		workerRow(t, pool, k, "client4", "review", session, "reducing02", 0, "completed", 500, 10, "", start)
	}
	start := now.Add(-time.Hour)
	k := header(t, pool, "client4", "review", "claims", "s-judged", "completed", start)
	workerRow(t, pool, k, "client4", "review", "s-judged", "reducing02", 0, "completed", 5000, 100, "", start)

	low := Default()
	low.Volume.MinInput = 100
	short := low
	short.Volume.HistoryRuns = 2

	for _, tc := range []struct {
		name string
		c    Classifier
		// runs is how many runs of the step the history holds: the prior runs
		// read, and the judged run itself.
		runs    int
		verdict Verdict
		basis   string
	}{
		{"default bounds", Default(), 1, Present, "against the absolute floor"},
		{"lower min input", low, 5, Absent, "median ratio"},
		{"shorter history", short, 3, Present, "against the absolute floor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ev, err := tc.c.Gather(ctx, pool, "s-judged", 0)
			if err != nil {
				t.Fatal(err)
			}
			if ev.Volume == nil || len(ev.Volume.Runs) != 1 {
				t.Fatalf("the history holds %+v, want one step", ev.Volume)
			}
			for _, runs := range ev.Volume.Runs {
				if len(runs) != tc.runs {
					t.Errorf("the history holds %d runs of the step, want %d", len(runs), tc.runs)
				}
			}
			r := tc.c.Classify(ev)
			assertAllValid(t, r)
			f := wantVerdict(t, r, LocusRowsLostSilently, tc.verdict)
			if !strings.Contains(f.Basis, tc.basis) {
				t.Errorf("the basis does not say %q: %s", tc.basis, f.Basis)
			}
		})
	}

	// The package's Gather is Default's.
	ev, err := Gather(ctx, pool, "s-judged", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, runs := range ev.Volume.Runs {
		if len(runs) != 1 {
			t.Errorf("Gather's history holds %d runs of the step, want Default's 1", len(runs))
		}
	}
}
