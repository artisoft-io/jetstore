package triage

// Locus rows_lost_silently judges each step against its own history, as the
// volume detector does: Gather reads the history and the locus passes it to the
// detector. Without it, every step that reduces or joins by design is present
// on every run. Needs JETS_TEST_DSN; skipped otherwise.

import (
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
