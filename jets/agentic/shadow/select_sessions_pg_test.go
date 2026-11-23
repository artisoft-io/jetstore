package shadow

// Which sessions a shadow triage run classifies, against a real Postgres: the
// synthetic feed fixture's anomalies and one run in the same window.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run SelectSessions ./jets/agentic/shadow/

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
)

// By default a window selects its runs and the sessions carrying a feed
// anomaly; feeds-only selects the feed sessions and no run, which is what a
// backfill needs; and the two combinations that would select nothing are
// refused.
func TestSelectSessionsFeedsOnly(t *testing.T) {
	pool := freshDB(t, "shadow_select_sessions", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if n := detectFeeds(t, pool); n == 0 {
		t.Fatal("the detectors wrote no anomaly, so this test would prove nothing")
	}
	const run = "run-in-the-window"
	header(t, pool, "a_client", "a_process", "a_type", run, "completed", feedfixture.Clock)
	from := feedfixture.Clock.Add(-time.Hour)

	all, err := SelectSessions(ctx, pool, from, "", "", 1000, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(all.Runs, []string{run}) {
		t.Fatalf("runs %v, want the one run in the window", all.Runs)
	}
	if len(all.Feeds) == 0 {
		t.Fatal("no feed session selected")
	}
	if len(all.Sessions) != 1+len(all.Feeds) || !slices.Contains(all.Sessions, run) {
		t.Errorf("sessions %v: want the run and the %d feed sessions", all.Sessions, len(all.Feeds))
	}

	feeds, err := SelectSessions(ctx, pool, from, "", "", 1000, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds.Runs) != 0 || slices.Contains(feeds.Sessions, run) {
		t.Errorf("feeds only selected the run: runs %v", feeds.Runs)
	}
	if !slices.Equal(feeds.Feeds, all.Feeds) || !slices.Equal(feeds.Sessions, all.Feeds) {
		t.Errorf("feeds only selected %v, want the feed sessions %v", feeds.Sessions, all.Feeds)
	}

	if _, err := SelectSessions(ctx, pool, from, "", "a_process", 1000, true, true); !errors.Is(err, ErrFeedsOnlyWithProcess) {
		t.Errorf("feeds only with a process: %v, want ErrFeedsOnlyWithProcess", err)
	}
	if _, err := SelectSessions(ctx, pool, from, "", "", 1000, true, false); !errors.Is(err, ErrFeedsOnlyWithoutAnomalies) {
		t.Errorf("feeds only without the anomaly table: %v, want ErrFeedsOnlyWithoutAnomalies", err)
	}

	// The default with a process keeps its runs and says why it has no feed session.
	proc, err := SelectSessions(ctx, pool, from, "", "a_process", 1000, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(proc.Sessions, []string{run}) || proc.FeedsSkipped == "" {
		t.Errorf("with a process: sessions %v, skipped %q", proc.Sessions, proc.FeedsSkipped)
	}
}
