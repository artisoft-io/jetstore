package shadow

// A feed incident carries its feed and its position, so a feed condition finds
// it after the delivery it is written on is deleted with its session: end to
// end against a real Postgres, on the feed fixture.
//
//	docker run -d --rm --name shadow-feed-pg -e POSTGRES_PASSWORD=pw -p 55892:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run Deleted ./jets/agentic/shadow/

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// deleteDelivery removes a delivery as the session purge removes its
// session: its registration and its anomalies.
func deleteDelivery(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range []string{`DELETE FROM jetsapi.anomaly WHERE anomaly_session_id = $1`,
		`DELETE FROM jetsapi.input_registry WHERE session_id = $1`} {
		if _, err := pool.Exec(ctx, stmt, session); err != nil {
			t.Fatal(err)
		}
	}
}

// feedIncidents names the incidents at a locus carrying a feed's key.
func feedIncidents(t *testing.T, pool *pgxpool.Pool, feed, locus string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT incident_id FROM jetsapi.incident
		WHERE incident_feed = $1 AND incident_locus = $2 ORDER BY 1`, feed, locus)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

// rerun triages each session again and fails on anything written.
func rerun(t *testing.T, pool *pgxpool.Pool, w *Writer, sessions []string, into string) {
	t.Helper()
	for _, s := range sessions {
		res, err := w.Run(context.Background(), pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 {
			t.Errorf("%s wrote %v, want nothing", s, res.Written)
		}
		if into != "" && !slices.ContainsFunc(append(res.Grouped, res.AlreadyRaised...), func(g string) bool {
			return strings.Contains(g, into)
		}) {
			t.Errorf("%s: grouped %v, already raised %v; want it in %s", s, res.Grouped, res.AlreadyRaised, into)
		}
	}
}

// heldIncidents names the incidents a condition holds.
func heldIncidents(fc *feedcondition.Feed, c *triage.Condition) []string {
	var out []string
	for _, inc := range fc.IncidentsOn(c) {
		out = append(out, inc.IncidentId)
	}
	return out
}

// A feed incident is written with its feed and its anchor delivery's arrival;
// when that delivery is deleted with its session, the condition keeps its
// incident, every later delivery joins it, and the next delivery joins it too:
// no second incident.
func TestADeletedFirstDeliveryKeepsItsFeedIncident(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_deleted_first")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	triageAll(t, pool, &w)

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	inc := IncidentIdFor(ws[0], departs)
	var feed string
	var anchor time.Time
	if err := pool.QueryRow(ctx, `SELECT incident_feed, incident_anchor_start_time FROM jetsapi.incident
		WHERE incident_id = $1`, inc).Scan(&feed, &anchor); err != nil {
		t.Fatal(err)
	}
	want, _ := triage.SessionArrival(ws[0])
	if feed != weekly.TableName || !anchor.Equal(want) {
		t.Errorf("the incident carries feed %q at %s, want %q at %s", feed, anchor, weekly.TableName, want)
	}

	deleteDelivery(t, pool, ws[0])
	fc, err := feedcondition.Read(ctx, pool, weekly.TableName)
	if err != nil {
		t.Fatal(err)
	}
	c := triage.ConditionAt(fc.Conditions, ws[1], departs)
	if c == nil || len(c.Members) != 7 || c.First().SessionId != ws[1] {
		t.Fatalf("the condition after its first delivery is deleted: %+v", c)
	}
	if got := heldIncidents(fc, c); !slices.Equal(got, []string{inc}) {
		t.Errorf("the condition holds %v, want %s", got, inc)
	}
	rerun(t, pool, &w, ws[1:], inc)

	last := ws[len(ws)-1]
	next := plantAfter(t, pool, departsAnomalyOn(t, pool, last), last, 1000, feedfixture.Clock, "next")
	rerun(t, pool, &w, []string{next}, inc)
	if got := feedIncidents(t, pool, weekly.TableName, departs); !slices.Equal(got, []string{inc}) {
		t.Errorf("the feed's incidents %v, want only %s", got, inc)
	}
}

// A delivery deleted inside a condition neither ends nor splits it; one
// deleted at the start of a condition that a clean delivery separates from
// the previous keeps its incident with its own condition.
func TestADeletedDeliveryInsideAFeedConditionEndsNothing(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_deleted_inside")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	triageAll(t, pool, &w)

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	inc := IncidentIdFor(ws[0], departs)
	deleteDelivery(t, pool, ws[3])
	fc, err := feedcondition.Read(ctx, pool, weekly.TableName)
	if err != nil {
		t.Fatal(err)
	}
	c := triage.ConditionAt(fc.Conditions, ws[4], departs)
	if c == nil || len(c.Members) != 7 || c.First().SessionId != ws[0] || !c.Open() {
		t.Fatalf("the condition across a deleted delivery: %+v", c)
	}
	live := slices.Delete(slices.Clone(ws), 3, 4)
	rerun(t, pool, &w, live, inc)
	if got := feedIncidents(t, pool, weekly.TableName, departs); !slices.Equal(got, []string{inc}) {
		t.Errorf("the feed's incidents %v, want only %s", got, inc)
	}

	// The wide-span feed's two conditions, a clean delivery between them: the
	// second's first delivery deleted, its incident stays the second's.
	wide, _ := feedfixture.FeedByTable(feedfixture.WideSpanIncremental)
	fc, err = feedcondition.Read(ctx, pool, wide.TableName)
	if err != nil {
		t.Fatal(err)
	}
	var runs []triage.Condition
	for _, c := range fc.Conditions {
		if c.Locus == departs {
			runs = append(runs, c)
		}
	}
	if len(runs) != 2 {
		t.Fatalf("the wide-span feed has %d conditions, want 2", len(runs))
	}
	first, second := IncidentIdFor(runs[0].First().SessionId, departs), IncidentIdFor(runs[1].First().SessionId, departs)
	deleteDelivery(t, pool, runs[1].First().SessionId)
	fc, err = feedcondition.Read(ctx, pool, wide.TableName)
	if err != nil {
		t.Fatal(err)
	}
	c1 := triage.ConditionAt(fc.Conditions, runs[0].First().SessionId, departs)
	c2 := triage.ConditionAt(fc.Conditions, runs[1].Members[1].SessionId, departs)
	if c1 == nil || c2 == nil || c1 == c2 || len(c2.Members) != 2 {
		t.Fatalf("the wide-span feed's conditions after the deletion: %+v and %+v", c1, c2)
	}
	if got := heldIncidents(fc, c1); !slices.Equal(got, []string{first}) {
		t.Errorf("the first condition holds %v, want %s", got, first)
	}
	if got := heldIncidents(fc, c2); !slices.Equal(got, []string{second}) {
		t.Errorf("the second condition holds %v, want %s", got, second)
	}
	rerun(t, pool, &w, c2.Sessions(), second)
	if n := len(feedIncidents(t, pool, wide.TableName, departs)); n != 2 {
		t.Errorf("the wide-span feed has %d incidents, want 2", n)
	}
}

// A feed incident written before feed incidents carried a key is found by its
// session, as before: the condition's later deliveries join it.
func TestAFeedIncidentWithNoKeyIsFoundBySession(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_no_key")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	legacy := legacyRaise(t, pool, &w, ws[0], departs)
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident WHERE incident_id = $1
		AND incident_feed IS NULL AND incident_anchor_start_time IS NULL`, legacy); n != 1 {
		t.Fatalf("the incident written as before carries a key")
	}
	rerun(t, pool, &w, ws[1:], legacy)
	fc, err := feedcondition.Read(ctx, pool, weekly.TableName)
	if err != nil {
		t.Fatal(err)
	}
	if got := heldIncidents(fc, triage.ConditionAt(fc.Conditions, ws[0], departs)); !slices.Equal(got,
		[]string{legacy}) {
		t.Errorf("the condition holds %v, want %s", got, legacy)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident WHERE incident_locus = $1`, departs); n != 1 {
		t.Errorf("%d incidents at the locus, want the one written before keys", n)
	}
}
