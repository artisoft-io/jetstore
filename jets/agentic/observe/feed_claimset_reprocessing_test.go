package observe

// The content checks over the fixture's re-processing feeds: a delivery is a
// file at its first registration, read as it first arrived. Needs
// JETS_TEST_DSN; see feed_delivery_reprocessing_pg_test.go.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
)

func claimSetReprocessingRun(t *testing.T, db DB) []ClaimSetVerdict {
	t.Helper()
	feeds := feedReprocessingFeeds(t, db)
	vs, err := DetectClaimSets(context.Background(), db, append(slices.Clone(feeds.Profiled), feeds.Unprofiled...),
		claimSetTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	return vs
}

// The written verdicts hold over the re-processing feeds; no verdict is on a
// re-registration and no predecessor is one; and nothing fires on them.
func TestClaimSetReprocessingIsNeverJudged(t *testing.T) {
	pool := feedReprocessingDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	again := map[string]bool{}
	for _, f := range feedfixture.ReprocessingFeeds() {
		for _, s := range reregisteredSessions(f) {
			again[s] = true
		}
	}
	got := map[string]ClaimSetVerdict{}
	for _, v := range claimSetReprocessingRun(t, pool) {
		if again[v.SessionID] || again[v.PredecessorSessionID] {
			t.Errorf("%s on %s: session %s against %s, a re-registration", v.Detector, v.TableName, v.SessionID,
				v.PredecessorSessionID)
		}
		got[v.Detector+"/"+v.TableName+"/"+v.SessionID] = v
	}
	checked := 0
	for _, row := range want.ReprocessingRows() {
		if !slices.Contains(ClaimSetDetectors, row.Detector) {
			continue
		}
		checked++
		v, ok := got[row.Detector+"/"+row.TableName+"/"+row.SessionID]
		if !ok || v.Ordinal != row.Ordinal || v.Verdict != row.Verdict || v.Reason != row.Reason {
			t.Errorf("%s on %s #%d: got #%d %s %q (%s), expected %s %q", row.Detector, row.TableName, row.Ordinal,
				v.Ordinal, v.Verdict, v.Reason, v.Detail, row.Verdict, row.Reason)
		}
		if len(v.Anomalies) > 0 {
			t.Errorf("%s on %s #%d writes %d anomalies", row.Detector, row.TableName, row.Ordinal, len(v.Anomalies))
		}
	}
	if checked != 5*26 {
		t.Errorf("checked %d verdicts, want five detectors over 26 deliveries", checked)
	}
	// The thirteenth delivery after the re-map is compared with the twelfth
	// as it first arrived, not with the re-map's registration of it.
	remap, _ := feedfixture.FeedByTable(feedfixture.RemappedIncremental)
	v := got[ClaimSetDeliveryOverlap+"/"+remap.TableName+"/"+remap.Deliveries[12].SessionID]
	if v.PredecessorSessionID != remap.Deliveries[11].SessionID {
		t.Errorf("after the re-map, the predecessor is %s, want the twelfth delivery's first registration %s",
			v.PredecessorSessionID, remap.Deliveries[11].SessionID)
	}
}

// The two families read one definition: over every feed, the content checks'
// deliveries, ordinals and predecessors are the delivery-grain reading's.
func TestClaimSetAndDeliveryGrainReadTheSameDeliveries(t *testing.T) {
	pool := feedReprocessingDB(t)
	in := feedReprocessingInput(t, pool, feedfixture.Clock)
	want := map[string]string{} // table/session -> ordinal and predecessor
	for table, ds := range in.Deliveries {
		for i, d := range ds {
			prev := ""
			if i > 0 {
				prev = ds[i-1].SessionId
			}
			want[table+"/"+d.SessionId] = fmt.Sprintf("#%d after %q", d.Ordinal, prev)
		}
	}
	got := map[string]string{}
	profiled := map[string]bool{}
	for _, f := range in.Profiled {
		profiled[f.TableName] = true
	}
	for _, v := range claimSetReprocessingRun(t, pool) {
		if profiled[v.TableName] {
			got[v.TableName+"/"+v.SessionID] = fmt.Sprintf("#%d after %q", v.Ordinal, v.PredecessorSessionID)
		}
	}
	if len(got) != len(want) {
		t.Errorf("the content checks read %d deliveries of the profiled feeds, the delivery-grain detectors %d",
			len(got), len(want))
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: content checks %s, delivery grain %s", k, got[k], w)
		}
	}
}

// When a delivery's first session has no claim rows, the check does not read a
// re-processing's rows instead: the delivery is not evaluable, its successor's
// comparison is not made, and both say which sessions re-processed the file.
func TestClaimSetDoesNotReadAReprocessingInsteadOfTheDelivery(t *testing.T) {
	pool := feedReprocessingDB(t)
	ctx := context.Background()
	f, _ := feedfixture.FeedByTable(feedfixture.BackfilledIncremental)
	first := f.Deliveries[7]
	table := pgx.Identifier{"public", f.ClaimTable}.Sanitize()
	if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE session_id = $1", first.SessionID); err != nil {
		t.Fatal(err)
	}
	var again string
	for _, r := range f.Reregistrations {
		if r.Of == first.Ordinal {
			again = r.SessionID
		}
	}
	for _, v := range claimSetReprocessingRun(t, pool) {
		if v.TableName != f.TableName {
			continue
		}
		switch {
		case v.SessionID == first.SessionID:
			if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoClaimRows || !strings.Contains(v.Detail, again) {
				t.Errorf("%s on the purged delivery: %s %q (%s)", v.Detector, v.Verdict, v.Reason, v.Detail)
			}
		case v.SessionID == f.Deliveries[8].SessionID && (v.Detector == ClaimSetDeliveryOverlap ||
			v.Detector == ClaimSetDeliveryUnchanged):
			if v.Reason != ClaimSetPredecessorNotLoaded || !strings.Contains(v.Detail, again) {
				t.Errorf("%s on the purged delivery's successor: %s %q (%s)", v.Detector, v.Verdict, v.Reason, v.Detail)
			}
		}
	}
}
