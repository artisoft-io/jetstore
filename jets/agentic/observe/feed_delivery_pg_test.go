package observe

// The delivery-grain feed detectors against the synthetic fixture, installed
// into a throwaway Postgres. Needs JETS_TEST_DSN, as extract_test.go does, and
// creates a database of its own on that server; skipped otherwise. Run with
// -count=1: the fixture reads jets_schema.json, which the test cache does not
// see.
//
//	docker run -d --rm --name feed-delivery-pg -e POSTGRES_PASSWORD=pw -p 55731:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55731/postgres \
//	  go test -count=1 -run FeedDelivery ./jets/agentic/observe/

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const feedDeliveryTestDatabase = "feed_delivery_detectors_test"

// feedDeliveryDB drops and creates a database of its own, installs the
// fixture and the anomaly table into it, and returns a pool on it.
func feedDeliveryDB(t *testing.T) *pgxpool.Pool {
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
	name := pgx.Identifier{feedDeliveryTestDatabase}.Sanitize()
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
	u.Path = "/" + feedDeliveryTestDatabase
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// feedDeliveryFixtureInput reads the fixture's feeds the way the
// retrospective does: from source_config, with the profiles a file declares
// attached on table_name.
func feedDeliveryFixtureInput(t *testing.T, pool *pgxpool.Pool, asOf time.Time) *FeedDeliveryInput {
	t.Helper()
	ctx := context.Background()
	var feeds []feedprofile.Feed
	clients := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if clients[f.Client] {
			continue
		}
		clients[f.Client] = true
		profiled, unprofiled, err := feedprofile.Load(ctx, pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		feeds = append(append(feeds, profiled...), unprofiled...)
	}
	profiles, err := feedprofile.LoadFile(feedfixture.ProfilesPath())
	if err != nil {
		t.Fatal(err)
	}
	profiled, unprofiled, unmatched := feedprofile.Attach(feeds, profiles)
	if len(unmatched) > 0 {
		t.Fatalf("the profile file names tables no feed has: %v", unmatched)
	}
	in, err := ReadFeedDeliveryInput(ctx, pool, profiled, unprofiled, asOf)
	if err != nil {
		t.Fatal(err)
	}
	return in
}

type feedDeliveryDetector interface {
	Detect(*FeedDeliveryInput) (FeedDeliveryResult, error)
}

func feedDeliveryDetectors() map[string]feedDeliveryDetector {
	return map[string]feedDeliveryDetector{
		feedprofile.FeedOverdue:       DefaultFeedOverdue(),
		feedprofile.DeliverySize:      DefaultDeliverySize(),
		feedprofile.DeliveryBackdated: DefaultDeliveryBackdated(),
	}
}

// Every verdict the written expected result gives the three detectors, by feed
// and delivery, session included, and the not-judged feeds by reason.
func TestFeedDeliveryDetectorsMeetTheExpectedResult(t *testing.T) {
	pool := feedDeliveryDB(t)
	want, err := feedfixture.LoadExpected()
	if err != nil {
		t.Fatal(err)
	}
	if !want.Clock.Equal(feedfixture.Clock) {
		t.Fatalf("expected.json is written for %s, the fixture's clock is %s", want.Clock, feedfixture.Clock)
	}
	in := feedDeliveryFixtureInput(t, pool, feedfixture.Clock)
	fired := map[string]int{}
	for name, det := range feedDeliveryDetectors() {
		res, err := det.Detect(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("%s", res.Coverage.Line())
		got := map[string]FeedDeliveryVerdict{}
		for _, v := range res.Verdicts {
			got[fmt.Sprintf("%s#%d", v.TableName, v.Ordinal)] = v
		}
		notJudged := map[string]FeedDeliveryNotJudgedFeed{}
		for _, n := range res.Coverage.NotJudged {
			notJudged[n.TableName] = n
		}
		anomalies := map[string]Anomaly{}
		for _, a := range res.Anomalies {
			anomalies[a.AnomalyId] = a
		}
		for _, f := range feedfixture.Feeds() {
			ev, ok := want.Of(name, f.TableName)
			if !ok {
				t.Fatalf("expected.json has no verdict of %s over %s", name, f.TableName)
			}
			if ev.NotJudged != "" {
				n, ok := notJudged[f.TableName]
				if !ok || n.Reason != ev.NotJudged {
					t.Errorf("%s over %s: expected not judged, %s; the coverage says %+v", name, f.TableName,
						ev.NotJudged, n)
				}
				continue
			}
			if n, ok := notJudged[f.TableName]; ok {
				t.Errorf("%s over %s: expected judged, the coverage says not judged, %s", name, f.TableName, n.Reason)
				continue
			}
			if name == feedprofile.FeedOverdue {
				last := f.Deliveries[len(f.Deliveries)-1]
				v, ok := got[fmt.Sprintf("%s#%d", f.TableName, len(f.Deliveries))]
				if !ok || v.SessionId != last.SessionID {
					t.Errorf("feed_overdue over %s: no verdict on the last delivery, session %s", f.TableName,
						last.SessionID)
					continue
				}
				if v.Verdict != ev.AtClock {
					t.Errorf("feed_overdue over %s: %s, expected %s", f.TableName, v.Verdict, ev.AtClock)
					continue
				}
				if v.Verdict == FeedDeliveryFires {
					fired[name]++
					a := anomalies[v.AnomalyId]
					if ev.OnDelivery != v.Ordinal {
						t.Errorf("feed_overdue over %s hangs on delivery %d, expected %d", f.TableName, v.Ordinal,
							ev.OnDelivery)
					}
					if a.ExpectedMax == nil || *a.ExpectedMax != ev.FirstFiresAt.UTC().Format(time.RFC3339) {
						t.Errorf("feed_overdue over %s is overdue from %v, expected %s", f.TableName,
							a.ExpectedMax, ev.FirstFiresAt)
					}
				}
				continue
			}
			for _, d := range f.Deliveries {
				wv, wr := ev.Delivery(d.Ordinal)
				v, ok := got[fmt.Sprintf("%s#%d", f.TableName, d.Ordinal)]
				switch {
				case !ok:
					t.Errorf("%s over %s, delivery %d: no verdict", name, f.TableName, d.Ordinal)
				case v.SessionId != d.SessionID:
					t.Errorf("%s over %s, delivery %d: session %s, expected %s", name, f.TableName, d.Ordinal,
						v.SessionId, d.SessionID)
				case v.Verdict != wv || v.Reason != wr:
					t.Errorf("%s over %s, delivery %d: %s %s, expected %s %s", name, f.TableName, d.Ordinal,
						v.Verdict, v.Reason, wv, wr)
				case v.Verdict == FeedDeliveryFires:
					fired[name]++
					if _, ok := anomalies[v.AnomalyId]; !ok {
						t.Errorf("%s over %s, delivery %d fires with no anomaly", name, f.TableName, d.Ordinal)
					}
				}
			}
		}
		if len(res.Anomalies) != fired[name] {
			t.Errorf("%s: %d anomalies for %d firing verdicts", name, len(res.Anomalies), fired[name])
		}
	}
	// The firing counts the fixture's own reference reading gives.
	for name, n := range map[string]int{feedprofile.FeedOverdue: 1, feedprofile.DeliverySize: 8,
		feedprofile.DeliveryBackdated: 1} {
		if fired[name] != n {
			t.Errorf("%s fires on %d, the fixture gives %d", name, fired[name], n)
		}
	}
}

// Every anomaly validates, names its declaration and its date, carries the
// identity the backfill relies on, and holds no member-level value; written
// twice, it is one row.
func TestFeedDeliveryAnomaliesAreWrittenOnce(t *testing.T) {
	pool := feedDeliveryDB(t)
	ctx := context.Background()
	in := feedDeliveryFixtureInput(t, pool, feedfixture.Clock)
	total := 0
	for name, det := range feedDeliveryDetectors() {
		res, err := det.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		for i := range res.Anomalies {
			a := &res.Anomalies[i]
			if err := a.Validate(); err != nil {
				t.Errorf("%s: %v", a.AnomalyId, err)
			}
			if a.SubjectType != SubjectFeed || !strings.HasPrefix(a.AnomalyId, name+"@1/"+a.SubjectRef+"/"+a.SessionId) {
				t.Errorf("%s: subject %s %s, session %s", a.AnomalyId, a.SubjectType, a.SubjectRef, a.SessionId)
			}
			if !strings.Contains(a.ExpectedBasis, "declared by fixture@example.com on 2026-03-01") {
				t.Errorf("%s: the basis does not name the declaration: %s", a.AnomalyId, a.ExpectedBasis)
			}
			if !a.DetectedAt.Equal(feedfixture.Clock) {
				t.Errorf("%s: detected at %s, not the as-of time", a.AnomalyId, a.DetectedAt)
			}
			for _, c := range feedfixture.Canaries() {
				if strings.Contains(fmt.Sprintf("%+v", *a), c) {
					t.Errorf("%s carries a member-level value", a.AnomalyId)
				}
			}
		}
		want := map[string]string{feedprofile.FeedOverdue: SignalArrival, feedprofile.DeliverySize: SignalVolume,
			feedprofile.DeliveryBackdated: SignalDistribution}[name]
		for _, a := range res.Anomalies {
			if a.SignalType != want {
				t.Errorf("%s: signal %s, want %s", a.AnomalyId, a.SignalType, want)
			}
		}
		for pass := 1; pass <= 2; pass++ {
			n, err := FeedDeliveryWrite(ctx, pool, res)
			if err != nil {
				t.Fatal(err)
			}
			if pass == 1 && n != len(res.Anomalies) || pass == 2 && n != 0 {
				t.Errorf("%s pass %d wrote %d new of %d", name, pass, n, len(res.Anomalies))
			}
		}
		total += len(res.Anomalies)
	}
	var rows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM jetsapi.anomaly").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != total || total != 10 {
		t.Errorf("%d rows for %d anomalies, expected 10", rows, total)
	}
}

// The backfill rule: as of a date between deliveries, nothing registered later
// is read, and each detector says what it would have said that day. The
// stopped feed is first overdue at the instant expected.json gives, and not a
// second before; the late delivery of the late-period feed is not yet there.
func TestFeedDeliveryBackfillReadsNothingAfter(t *testing.T) {
	pool := feedDeliveryDB(t)
	want, _ := feedfixture.LoadExpected()
	ev, _ := want.Of(feedprofile.FeedOverdue, feedfixture.StoppedIncremental)
	first := *ev.FirstFiresAt
	for _, c := range []struct {
		at    time.Time
		fires bool
	}{{first.Add(-time.Second), false}, {first, true}} {
		in := feedDeliveryFixtureInput(t, pool, c.at)
		res, err := DefaultFeedOverdue().Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		fired := slices.ContainsFunc(res.Verdicts, func(v FeedDeliveryVerdict) bool {
			return v.TableName == feedfixture.StoppedIncremental && v.Verdict == FeedDeliveryFires
		})
		if fired != c.fires {
			t.Errorf("as of %s the stopped feed fires %v, want %v", c.at, fired, c.fires)
		}
	}

	late, _ := feedfixture.FeedByTable(feedfixture.LatePeriodIncremental)
	lastAt := late.Deliveries[len(late.Deliveries)-1].Registered
	in := feedDeliveryFixtureInput(t, pool, lastAt.Add(-time.Millisecond))
	if n := len(in.Deliveries[feedfixture.LatePeriodIncremental]); n != len(late.Deliveries)-1 {
		t.Errorf("as of just before the late delivery, %d deliveries are read, want %d", n, len(late.Deliveries)-1)
	}
	res, err := DefaultDeliveryBackdated().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Anomalies) != 0 {
		t.Errorf("before the late delivery, delivery_backdated fires: %+v", res.Anomalies)
	}
}

// A delivery registered under another client is attributed to its feed by
// table name, judged, and reported; a delivery whose size was not rebuilt by
// the sync is not judged rather than silent.
func TestFeedDeliveryRegistrationsUnderAnotherClient(t *testing.T) {
	pool := feedDeliveryDB(t)
	ctx := context.Background()
	f, _ := feedfixture.FeedByTable(feedfixture.SiblingIncremental)
	last := f.Deliveries[len(f.Deliveries)-1]
	at := last.Registered.Add(7 * 24 * time.Hour)
	session := fmt.Sprint(at.UnixMilli())
	var period int
	if err := pool.QueryRow(ctx, `SELECT source_period_key FROM jetsapi.input_registry WHERE session_id = $1`,
		last.SessionID).Scan(&period); err != nil {
		t.Fatal(err)
	}
	// Registered under a placeholder client and an empty org, as some
	// registrations are, with no file_key_staging row.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
		VALUES ('Any', '', $1, 'elsewhere/key.csv', $2, $3, 'file', $4, 'system', $5)`,
		feedfixture.DomainKey, period, f.TableName, session, at); err != nil {
		t.Fatal(err)
	}
	in := feedDeliveryFixtureInput(t, pool, at)
	ds := in.Deliveries[f.TableName]
	if len(ds) != len(f.Deliveries)+1 || ds[len(ds)-1].SessionId != session {
		t.Fatalf("the registration under another client is not read as the feed's: %d deliveries", len(ds))
	}
	res, err := DefaultDeliverySize().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	var v FeedDeliveryVerdict
	for _, x := range res.Verdicts {
		if x.SessionId == session {
			v = x
		}
	}
	if v.Verdict != FeedDeliveryNotJudged || v.Reason != FeedDeliveryNoSize {
		t.Errorf("a delivery with no size: %+v", v)
	}
	var e *FeedDeliveryElsewhere
	for i := range res.Coverage.Elsewhere {
		if res.Coverage.Elsewhere[i].TableName == f.TableName {
			e = &res.Coverage.Elsewhere[i]
		}
	}
	if e == nil || e.Client != "Any" || e.Deliveries != 1 || e.LatestSession != session {
		t.Errorf("the coverage does not report the registration elsewhere: %+v", res.Coverage.Elsewhere)
	}
	if !strings.Contains(res.Coverage.Line(), `registered under client "Any"`) {
		t.Errorf("the coverage line does not say so: %s", res.Coverage.Line())
	}
	// It is the feed's last delivery now, so the overdue finding would hang on
	// it, and its basis would say where it was registered.
	ov, err := FeedOverdue{Generation: 1}.Detect(&FeedDeliveryInput{AsOf: at.AddDate(0, 3, 0),
		Profiled: in.Profiled, Deliveries: in.Deliveries})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range ov.Anomalies {
		if a.SubjectRef == f.TableName && (a.SessionId != session || !strings.Contains(a.ExpectedBasis, `"Any"`)) {
			t.Errorf("the overdue finding hangs on %s with basis %s", a.SessionId, a.ExpectedBasis)
		}
	}
}

// The coverage lists every feed not judged with its reason, sorted, and a
// feed is never both judged and not judged.
func TestFeedDeliveryCoverageIsAPartition(t *testing.T) {
	pool := feedDeliveryDB(t)
	in := feedDeliveryFixtureInput(t, pool, feedfixture.Clock)
	for name, det := range feedDeliveryDetectors() {
		res, err := det.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		var all []string
		all = append(all, res.Coverage.Judged...)
		for _, n := range res.Coverage.NotJudged {
			all = append(all, n.TableName)
		}
		sort.Strings(all)
		if len(all) != len(feedfixture.Feeds()) || len(slices.Compact(slices.Clone(all))) != len(all) {
			t.Errorf("%s: judged and not judged are not a partition of the feeds: %v", name, all)
		}
	}
}
