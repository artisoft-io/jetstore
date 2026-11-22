package observe

// A delivery is a file at its first registration: the delivery-grain detectors
// over the fixture's re-processing feeds, installed beside the base feeds into
// a database of its own on the server JETS_TEST_DSN names; skipped when it is
// unset. Run with -count=1.
//
//	docker run -d --rm --name feed-reprocessing-pg -e POSTGRES_PASSWORD=pw -p 55961:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55961/postgres \
//	  go test -count=1 -run Reprocess ./jets/agentic/observe/

import (
	"context"
	"maps"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const feedReprocessingTestDatabase = "feed_reprocessing_test"

// feedReprocessingDB drops and creates a database of its own and installs the
// base fixture, the re-processing feeds and jetsapi.anomaly into it.
func feedReprocessingDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := pgx.Identifier{feedReprocessingTestDatabase}.Sanitize()
	_, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	if err == nil {
		_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	}
	admin.Close()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + feedReprocessingTestDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = feedfixture.InstallReprocessing(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// feedReprocessingProfiles are both profile files, the base feeds' and the
// re-processing feeds'.
func feedReprocessingProfiles(t *testing.T) map[string]feedprofile.Profile {
	t.Helper()
	p, err := feedprofile.LoadFile(feedfixture.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	r, err := feedprofile.LoadFile(feedfixture.ReprocessingProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	maps.Copy(p, r)
	return p
}

// feedReprocessingFeeds reads every client's feeds with both profile files.
func feedReprocessingFeeds(t *testing.T, db DB) FeedProcessFeeds {
	t.Helper()
	feeds, err := ReadFeedProcessFeeds(context.Background(), db, nil, feedReprocessingProfiles(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds.Unmatched) > 0 {
		t.Fatalf("profiles that match no feed: %v", feeds.Unmatched)
	}
	return feeds
}

func feedReprocessingInput(t *testing.T, pool *pgxpool.Pool, asOf time.Time) *FeedDeliveryInput {
	t.Helper()
	feeds := feedReprocessingFeeds(t, pool)
	in, err := ReadFeedDeliveryInput(context.Background(), pool, feeds.Profiled, feeds.Unprofiled, asOf)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// reregisteredSessions are the sessions of a feed's re-registrations.
func reregisteredSessions(f feedfixture.Feed) []string {
	var out []string
	for _, r := range f.Reregistrations {
		out = append(out, r.SessionID)
	}
	return out
}

// Each file is one delivery, at its first registration, carrying its later
// registrations as re-processing; no verdict is on a re-registration; the
// written verdicts hold; and the coverage line counts the re-processing.
func TestFeedDeliveryReprocessingIsCountedAndNotJudged(t *testing.T) {
	pool := feedReprocessingDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	in := feedReprocessingInput(t, pool, feedfixture.Clock)
	again := map[string]bool{}
	for _, f := range feedfixture.ReprocessingFeeds() {
		for _, s := range reregisteredSessions(f) {
			again[s] = true
		}
		ds := in.Deliveries[f.TableName]
		if len(ds) != len(f.Deliveries) {
			t.Fatalf("%s: %d deliveries read, want %d files", f.TableName, len(ds), len(f.Deliveries))
		}
		for i, d := range ds {
			if d.SessionId != f.Deliveries[i].SessionID || d.Ordinal != i+1 {
				t.Errorf("%s delivery %d is session %s, ordinal %d; the fixture's first registration is %s",
					f.TableName, i+1, d.SessionId, d.Ordinal, f.Deliveries[i].SessionID)
			}
			var wantAgain, gotAgain []string
			for _, r := range f.Reregistrations {
				if r.Of == i+1 {
					wantAgain = append(wantAgain, r.SessionID)
				}
			}
			for _, r := range d.Reprocessed {
				gotAgain = append(gotAgain, r.SessionId)
			}
			slices.Sort(wantAgain)
			if !slices.Equal(gotAgain, wantAgain) {
				t.Errorf("%s delivery %d is re-processed by %v, the fixture says %v", f.TableName, i+1, gotAgain, wantAgain)
			}
		}
	}

	backfill, _ := feedfixture.FeedByTable(feedfixture.BackfilledIncremental)
	for det, d := range feedDeliveryDetectors() {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range res.Verdicts {
			if again[v.SessionId] {
				t.Errorf("%s judges re-registration %s of %s", det, v.SessionId, v.TableName)
			}
		}
		got := map[string]FeedDeliveryVerdict{}
		for _, v := range res.Verdicts {
			got[v.TableName+"#"+v.SessionId] = v
		}
		for _, f := range feedfixture.ReprocessingFeeds() {
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
			}
			n := res.Coverage.Reprocessed[f.TableName]
			if n.Registrations != len(f.Reregistrations) || n.Files != f.ReprocessedFiles() {
				t.Errorf("%s's coverage counts %+v for %s, want %d re-registrations of %d files", det, n,
					f.TableName, len(f.Reregistrations), f.ReprocessedFiles())
			}
		}
		if !strings.Contains(res.Coverage.Line(), feedfixture.RemappedIncremental+": 12 re-registration(s) of 12 file(s)") {
			t.Errorf("%s's coverage line does not count the re-map: %s", det, res.Coverage.Line())
		}
		if det == feedprofile.DeliveryBackdated {
			var fired []string
			for _, a := range res.Anomalies {
				if a.SubjectRef == feedfixture.BackfilledIncremental {
					fired = append(fired, a.SessionId)
				}
			}
			wantFired := []string{backfill.Deliveries[7].SessionID, backfill.Deliveries[8].SessionID,
				backfill.Deliveries[9].SessionID}
			if !slices.Equal(fired, wantFired) {
				t.Errorf("delivery_backdated fires on the backfill at %v, want once per new file, %v", fired, wantFired)
			}
		}
	}
}

// A run as of a past instant reads the re-processing registered by then and
// nothing after: between the backfill's second file and its re-registration,
// the first file has been registered again once and the second not yet.
func TestFeedDeliveryReprocessingAsOfAPastInstant(t *testing.T) {
	pool := feedReprocessingDB(t)
	f, _ := feedfixture.FeedByTable(feedfixture.BackfilledIncremental)
	asOf := f.Deliveries[8].Registered.Add(time.Second)
	in := feedReprocessingInput(t, pool, asOf)
	ds := in.Deliveries[f.TableName]
	if len(ds) != 9 {
		t.Fatalf("as of %s, %d deliveries read, want 9", asOf, len(ds))
	}
	if len(ds[7].Reprocessed) != 1 || len(ds[8].Reprocessed) != 0 {
		t.Errorf("as of %s, deliveries 8 and 9 carry %d and %d re-registrations, want 1 and 0", asOf,
			len(ds[7].Reprocessed), len(ds[8].Reprocessed))
	}
	res, err := DefaultDeliveryBackdated().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	if n := res.Coverage.Reprocessed[f.TableName]; n != (FeedReprocessed{Files: 1, Registrations: 1}) {
		t.Errorf("as of %s, the coverage counts %+v", asOf, n)
	}
	remap, _ := feedfixture.FeedByTable(feedfixture.RemappedIncremental)
	if n, ok := res.Coverage.Reprocessed[remap.TableName]; ok {
		t.Errorf("as of %s, before the re-map, the coverage counts %+v for it", asOf, n)
	}
}
