package observe

// The feed process over the base fixture and its re-processing feeds together.
// Needs JETS_TEST_DSN; see feed_delivery_reprocessing_pg_test.go.

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
)

func feedReprocessingRun(t *testing.T, db DB, o FeedProcessOptions) FeedProcessResult {
	t.Helper()
	res, err := RunFeedProcess(context.Background(), db, feedReprocessingFeeds(t, db), o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Every written verdict holds over all thirteen feeds, as a window and as a
// backfill; the re-processing feeds add three findings, the backfill's three
// new files back-dated; and every detector's coverage counts the
// re-processing of the feeds it judges.
func TestFeedProcessCountsReprocessingAndJudgesFiles(t *testing.T) {
	pool := feedReprocessingDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	window := feedReprocessingRun(t, pool, DefaultFeedProcessOptions(feedfixture.Clock))
	backfill := feedReprocessingRun(t, pool, feedProcessBackfill())
	for name, res := range map[string]FeedProcessResult{"window": window, "backfill": backfill} {
		got := feedProcessVerdicts(res)
		for _, r := range append(want.Rows(), want.ReprocessingRows()...) {
			if ev, _ := want.Of(r.Detector, r.TableName); ev.NotJudged != "" &&
				(r.Detector == feedprofile.DeliverySize || r.Detector == feedprofile.DeliveryBackdated) {
				continue // a feed not judged at all is the coverage's, as the base test checks
			}
			if g := got[feedProcessKey(r.Detector, r.TableName, r.Ordinal)]; g != [2]string{r.Verdict, r.Reason} {
				t.Errorf("%s: %s over %s delivery %d: %v, expected %s %s", name, r.Detector, r.TableName,
					r.Ordinal, g, r.Verdict, r.Reason)
			}
		}
		byDetector := map[string]int{}
		for _, a := range res.Anomalies {
			byDetector[strings.Split(a.DetectorRef, "@")[0]]++
		}
		wantCounts := map[string]int{"feed_overdue": 1, "delivery_size": 8, "delivery_backdated": 1 + 3,
			"service_date_stale": 9, "service_date_span": 14, "field_completeness": 7, "delivery_overlap": 8,
			"delivery_unchanged": 6}
		if !reflect.DeepEqual(byDetector, wantCounts) {
			t.Errorf("%s: anomalies by detector %v, want %v", name, byDetector, wantCounts)
		}
		for _, c := range res.Coverage {
			for _, f := range feedfixture.ReprocessingFeeds() {
				n := c.Reprocessed[f.TableName]
				if n.Registrations != len(f.Reregistrations) || n.Files != f.ReprocessedFiles() {
					t.Errorf("%s: %s counts %+v for %s, want %d re-registrations of %d files", name, c.Detector, n,
						f.TableName, len(f.Reregistrations), f.ReprocessedFiles())
				}
			}
			for _, f := range feedfixture.Feeds() {
				if n, ok := c.Reprocessed[f.TableName]; ok {
					t.Errorf("%s: %s counts re-processing %+v for %s, which registers each file once", name,
						c.Detector, n, f.TableName)
				}
			}
			if !strings.Contains(c.Line(), "backfilled_incremental: 4 re-registration(s) of 3 file(s)") {
				t.Errorf("%s: %s's line does not count the backfill: %s", name, c.Detector, c.Line())
			}
		}
	}
	if len(backfill.Anomalies) != len(window.Anomalies) {
		t.Errorf("the backfill finds %d, the window %d", len(backfill.Anomalies), len(window.Anomalies))
	}
}

// The coverage counts the re-registrations whose own arrival is in the run's
// window: a window opening after the backfill and before the re-map counts the
// re-map alone, and one opening after the re-map counts neither.
func TestFeedProcessCountsTheReprocessingInItsWindow(t *testing.T) {
	pool := feedReprocessingDB(t)
	for _, c := range []struct {
		since          time.Time
		remap, backfil bool
	}{
		{time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC), true, false},
		{time.Date(2026, time.May, 22, 0, 0, 0, 0, time.UTC), false, false},
		{time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC), true, true},
	} {
		o := DefaultFeedProcessOptions(feedfixture.Clock)
		o.Since = c.since
		res := feedReprocessingRun(t, pool, o)
		for _, cov := range res.Coverage {
			_, remap := cov.Reprocessed[feedfixture.RemappedIncremental]
			_, back := cov.Reprocessed[feedfixture.BackfilledIncremental]
			if remap != c.remap || back != c.backfil {
				t.Errorf("since %s, %s counts the re-map %v and the backfill %v, want %v and %v",
					c.since.Format(time.DateOnly), cov.Detector, remap, back, c.remap, c.backfil)
			}
		}
	}
}

// The export prints the one registration statement both families read, and
// judging what it returns is the in-process run, re-processing included.
func TestFeedProcessExportJudgesReprocessingAsItRuns(t *testing.T) {
	pool := feedReprocessingDB(t)
	ctx := context.Background()
	o := feedProcessBackfill()
	profiles := feedReprocessingProfiles(t)
	results, rounds := feedProcessRunExport(t, pool, nil, profiles, o)
	if rounds != 4 {
		t.Errorf("%d rounds, want four", rounds)
	}
	parsed, err := ReadFeedProcessResults(strings.NewReader(results))
	if err != nil {
		t.Fatal(err)
	}
	judged, err := JudgeFeedProcess(ctx, parsed, nil, profiles, o)
	if err != nil {
		t.Fatal(err)
	}
	live := feedReprocessingRun(t, pool, o)
	if a, b := FeedProcessVerdictLines(judged), FeedProcessVerdictLines(live); !reflect.DeepEqual(a, b) {
		t.Errorf("the judge gives %d verdict lines, the in-process run %d, and they differ", len(a), len(b))
	}
	if !reflect.DeepEqual(judged.Anomalies, live.Anomalies) {
		t.Errorf("the judge finds %d anomalies, the in-process run %d", len(judged.Anomalies), len(live.Anomalies))
	}
	if !reflect.DeepEqual(judged.Coverage, live.Coverage) {
		t.Errorf("the judge's coverage differs from the in-process run's")
	}
	registrations := 0
	for _, line := range strings.Split(strings.TrimSpace(results), "\n") {
		if strings.Contains(line, `"rows"`) && strings.Contains(line, feedfixture.RemappedIncremental) &&
			strings.Contains(line, "fixture/input/") {
			registrations++
		}
	}
	if registrations != 1 {
		t.Errorf("%d results carry the registrations, want the one statement both families read", registrations)
	}
}
