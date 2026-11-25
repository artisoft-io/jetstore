package observe

// The feed detectors over the fixture's not-judged feeds, installed alone into
// a database of their own on the server JETS_TEST_DSN names; skipped when it is
// unset. Run with -count=1: the fixture reads jets_schema.json, which the test
// cache does not see.
//
//	docker run -d --rm --name feed-not-judged-pg -e POSTGRES_PASSWORD=pw -p 55962:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55962/postgres \
//	  go test -count=1 -run NotJudged ./jets/agentic/observe/

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const feedNotJudgedTestDatabase = "feed_not_judged_test"

func feedNotJudgedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return claimSetTestPoolWith(t, feedNotJudgedTestDatabase, feedfixture.InstallNotJudged)
}

// feedNotJudgedFeeds reads the not-judged feeds with their profile file.
func feedNotJudgedFeeds(t *testing.T, db DB) FeedProcessFeeds {
	t.Helper()
	profiles, err := feedprofile.LoadFile(feedfixture.NotJudgedProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	feeds, err := ReadFeedProcessFeeds(context.Background(), db, nil, profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds.Unmatched) > 0 {
		t.Fatalf("profiles that match no feed: %v", feeds.Unmatched)
	}
	return feeds
}

// The content checks reach every written verdict on the not-judged feeds, and
// do not judge exactly as many deliveries, for exactly the reasons, as the
// feeds were built to give: each reason once per delivery it names.
func TestClaimSetNotJudgedReasonsAreTheWrittenOnes(t *testing.T) {
	pool := feedNotJudgedDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	feeds := feedNotJudgedFeeds(t, pool)
	vs, err := DetectClaimSets(context.Background(), pool, append(slices.Clone(feeds.Profiled), feeds.Unprofiled...),
		claimSetTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ClaimSetVerdict{}
	for _, v := range vs {
		got[v.Detector+"/"+v.TableName+"/"+v.SessionID] = v
	}
	reasons := map[string]int{}
	checked := 0
	for _, row := range want.NotJudgedRows() {
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
		if v.Verdict == ClaimSetNotJudged {
			reasons[v.Reason]++
		}
		// A reason that names what is missing names it.
		switch v.Reason {
		case ClaimSetColumnAbsent:
			absent := feedfixture.ColAbsentMember
			if v.TableName == feedfixture.AbsentRequiredField {
				absent = feedfixture.ColAbsentRequired
			}
			if !strings.Contains(v.Detail, pgx.Identifier{absent}.Sanitize()) {
				t.Errorf("%s on %s #%d: column_absent does not name %s: %s", row.Detector, row.TableName,
					row.Ordinal, absent, v.Detail)
			}
		case ClaimSetPredecessorNotLoaded:
			if !strings.Contains(v.Detail, v.PredecessorSessionID) || v.PredecessorSessionID == "" {
				t.Errorf("%s on %s #%d: predecessor_no_claim_rows does not name the predecessor %q: %s",
					row.Detector, row.TableName, row.Ordinal, v.PredecessorSessionID, v.Detail)
			}
		}
	}
	if checked != 5*28 || len(got) != checked {
		t.Errorf("checked %d verdicts of %d given, want five detectors over 28 deliveries", checked, len(got))
	}
	wantReasons := map[string]int{
		ClaimSetNoClaimRows:          5,
		ClaimSetPredecessorNotLoaded: 2,
		ClaimSetNoServiceDates:       4,
		ClaimSetColumnAbsent:         5*7 + 7,
		ClaimSetNoPredecessor:        2 * 3,
	}
	if !maps.Equal(reasons, wantReasons) {
		t.Errorf("the content checks do not judge %v, want %v", reasons, wantReasons)
	}
}

// The delivery-grain detectors reach every written verdict on the not-judged
// feeds: delivery_size judges the feed that declares no size range against its
// trailing band, not judging its first four deliveries and firing on its small
// last file.
func TestFeedDeliveryNotJudgedReasonsAreTheWrittenOnes(t *testing.T) {
	pool := feedNotJudgedDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	feeds := feedNotJudgedFeeds(t, pool)
	in, err := ReadFeedDeliveryInput(context.Background(), pool, feeds.Profiled, feeds.Unprofiled, feedfixture.Clock)
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]int{}
	fired := map[string]int{}
	for det, d := range feedDeliveryDetectors() {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]FeedDeliveryVerdict{}
		for _, v := range res.Verdicts {
			got[v.TableName+"#"+v.SessionId] = v
		}
		for _, f := range feedfixture.NotJudgedFeeds() {
			ev, _ := want.Of(det, f.TableName)
			if det == feedprofile.FeedOverdue {
				last := f.Deliveries[len(f.Deliveries)-1]
				if v := got[f.TableName+"#"+last.SessionID]; v.Verdict != ev.AtClock {
					t.Errorf("feed_overdue over %s at the clock: %s on %s, expected %s on the last delivery",
						f.TableName, v.Verdict, v.SessionId, ev.AtClock)
				}
				continue
			}
			for _, fd := range f.Deliveries {
				wv, wr := ev.Delivery(fd.Ordinal)
				v, ok := got[f.TableName+"#"+fd.SessionID]
				if !ok || v.Verdict != wv || v.Reason != wr {
					t.Errorf("%s over %s delivery %d: %s %q, expected %s %q", det, f.TableName, fd.Ordinal,
						v.Verdict, v.Reason, wv, wr)
				}
				switch v.Verdict {
				case FeedDeliveryNotJudged:
					reasons[v.Reason]++
				case FeedDeliveryFires:
					fired[det]++
				}
				if v.Reason == FeedDeliveryShortHistory && !slices.Contains(v.Confounders, ConfounderHistoryTruncated) {
					t.Errorf("%s over %s delivery %d is %s without %s", det, f.TableName, fd.Ordinal, v.Reason,
						ConfounderHistoryTruncated)
				}
			}
		}
	}
	wantReasons := map[string]int{FeedDeliveryShortHistory: 4, FeedDeliveryNoPredecessor: 4}
	if !maps.Equal(reasons, wantReasons) {
		t.Errorf("the delivery-grain detectors do not judge %v, want %v", reasons, wantReasons)
	}
	if !maps.Equal(fired, map[string]int{feedprofile.DeliverySize: 1}) {
		t.Errorf("the delivery-grain detectors fire %v, want delivery_size once", fired)
	}
}
