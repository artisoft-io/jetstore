package feedsummary

// The summary of a condition, end to end against a real Postgres: the feed
// fixture's conditions as shadow triage raises them, read back and summarised
// the way the incident read does it. Run as fixture_pg_test.go says.

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

const departsLocus = triage.LocusDeliveryDepartsProfile

func sumOf(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

// wordsOf spells a count as the template does, up to twenty.
func wordsOf(n int) string { return countWords[n] }

// findingPositions counts the (session, locus) pairs carrying a feed finding at
// a delivery locus, from the anomalies alone.
func findingPositions(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT DISTINCT anomaly_session_id, anomaly_subject_type,
		anomaly_signal_type FROM jetsapi.anomaly WHERE anomaly_subject_type = 'feed'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var s, subject, signal string
		if err := rows.Scan(&s, &subject, &signal); err != nil {
			t.Fatal(err)
		}
		if l, ok := triage.LocusOf(subject, signal); ok && slices.Contains(triage.DeliveryLoci, l) {
			seen[s+"/"+l] = true
		}
	}
	return len(seen)
}

func sessionsOf(f feedfixture.Feed) []string {
	out := make([]string, len(f.Deliveries))
	for i, d := range f.Deliveries {
		out[i] = d.SessionID
	}
	return out
}

func arrival(t *testing.T, session string) time.Time {
	t.Helper()
	ms, err := strconv.ParseInt(session, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return time.UnixMilli(ms).UTC()
}

// summaryOf reads an incident and renders its summary, and fails on any
// statement the counter finds untrue of it.
func summaryOf(t *testing.T, pool *pgxpool.Pool, id string) (*Input, string) {
	t.Helper()
	ctx := context.Background()
	inc, err := audit.ReadIncident(ctx, pool, id, audit.RedactPHI)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	in, err := Read(ctx, pool, inc)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	s, err := Render(in)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	if c := CountUntrue(s.Text, in); c.Untrue() != 0 {
		t.Errorf("%s: the counter finds untrue statements in the render:\n%s\n%s", id, c, s.Text)
	}
	return in, strings.Join(strings.Fields(s.Text), " ")
}

func says(t *testing.T, id, flat string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(flat, want) {
			t.Errorf("%s: the summary does not say %q:\n%s", id, want, flat)
		}
	}
}

func dmy(t time.Time) string { return t.Format("2 January 2006") }

// The weekly repeat is one open condition of eight deliveries, and its
// incident's summary says so; the interrupted condition is two, the first
// ended by a delivery with no finding and the second naming the first's
// incident with its verdict, read now. Triaged newest first, as the daily run
// does, the second incident is raised before the first, so its basis says the
// first has no incident yet; the summary, read afterwards, names it.
func TestTheSummaryOfAStandingCondition(t *testing.T) {
	pool, _ := fixtureIncidents(t, "feedsummary_condition")
	ctx := context.Background()

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	id := shadow.IncidentIdFor(ws[0], departsLocus)
	in, flat := summaryOf(t, pool, id)
	c := in.Condition
	if c == nil || c.Held != 8 || c.Total != 8 || c.End != EndOpen || c.Previous.Kind != PrevNone {
		t.Fatalf("the weekly repeat's condition: %+v", c)
	}
	says(t, id, flat,
		"This incident stands for eight deliveries of the feed with a finding at this locus, registered from "+
			dmy(arrival(t, ws[0]))+" to "+dmy(arrival(t, ws[7]))+".",
		"It is still open: the feed's latest delivery carries the finding.",
		"No earlier delivery of the feed carries a finding at this locus.",
		"give it one verdict")

	wide, _ := feedfixture.FeedByTable(feedfixture.WideSpanIncremental)
	fc := wideConditions(t, pool, wide.TableName)
	first := shadow.IncidentIdFor(fc[0].First().SessionId, departsLocus)
	second := shadow.IncidentIdFor(fc[1].First().SessionId, departsLocus)
	var basis string
	if err := pool.QueryRow(ctx, `SELECT transition_rationale FROM jetsapi.incident_event
		WHERE event_incident_ref = $1 AND to_status = 'triaged'`, second).Scan(&basis); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(basis, "has no incident yet") {
		t.Fatalf("the second incident was not raised before the first, so the live read is not tested:\n%s", basis)
	}
	in, flat = summaryOf(t, pool, first)
	if c := in.Condition; c == nil || c.Held != 3 || c.End != EndNoFinding || c.EndedAt.IsZero() {
		t.Fatalf("the first wide-span condition: %+v", c)
	}
	says(t, first, flat,
		"This incident stands for three deliveries",
		"It ended with the feed's next delivery, registered on "+dmy(arrival(t, fc[0].EndedBy))+
			", which carries no finding at this locus")
	in, flat = summaryOf(t, pool, second)
	if c := in.Condition; c == nil || c.Previous.Kind != PrevCondition || c.Previous.IncidentID != first {
		t.Fatalf("the second wide-span condition: %+v", c)
	}
	says(t, second, flat,
		"The feed's previous condition at this locus, three deliveries registered from "+
			dmy(fc[0].First().Arrival)+" to "+dmy(fc[0].Last().Arrival)+", has incident "+first+", with no verdict.")
}

func wideConditions(t *testing.T, pool *pgxpool.Pool, feed string) []*triage.Condition {
	t.Helper()
	h, err := triage.ReadFeedHistory(context.Background(), pool, feed, true)
	if err != nil {
		t.Fatal(err)
	}
	var out []*triage.Condition
	cs := h.Conditions()
	for i := range cs {
		if cs[i].Locus == departsLocus {
			out = append(out, &cs[i])
		}
	}
	if len(out) != 2 {
		t.Fatalf("the wide-span feed has %d conditions at %s, want 2", len(out), departsLocus)
	}
	return out
}

// plant writes a copy of an anomaly on a new session with no registration, ms
// after another, detected at detected.
func plant(t *testing.T, pool *pgxpool.Pool, base observe.Anomaly, after string, ms int64, detected time.Time,
	suffix string) string {
	t.Helper()
	a := base
	a.SessionId = strconv.FormatInt(arrival(t, after).UnixMilli()+ms, 10)
	a.AnomalyId = base.AnomalyId + "/" + suffix
	a.DetectedAt = detected
	if err := observe.InsertAnomaly(context.Background(), pool, &a); err != nil {
		t.Fatal(err)
	}
	return a.SessionId
}

// A person's verdict closes the condition's incident to later evidence: the
// judged incident's summary says the verdict closed it and names the incident
// its next delivery belongs to, and the new incident's summary says it was
// opened after that verdict, naming the judged incident and the verdict. Before
// the new incident is raised, the judged one's summary counts the new evidence
// as having no incident yet.
func TestTheSummaryOfAConditionOpenedAfterAVerdict(t *testing.T) {
	pool, _ := fixtureIncidents(t, "feedsummary_condition_verdict")
	ctx := context.Background()
	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	last := ws[len(ws)-1]
	judged := shadow.IncidentIdFor(ws[0], departsLocus)
	verdictAt := feedfixture.Clock.Add(time.Hour)
	if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
		IncidentEventId: "ie_person_benign", IncidentRef: judged,
		FromStatus: audit.IncidentDiagnosed, ToStatus: audit.IncidentSuppressedAsBenign,
		Actor: "reviewer@example.com", ActorKind: audit.ActorHuman, TransitionedAt: verdictAt,
		Rationale: "the repeat is expected",
	}); err != nil {
		t.Fatal(err)
	}

	var base observe.Anomaly
	as, err := observe.ReadAnomalies(ctx, pool, last)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if l, ok := triage.LocusOf(a.SubjectType, a.SignalType); ok && l == departsLocus {
			base = a
			break
		}
	}
	before := plant(t, pool, base, last, 1000, verdictAt.Add(-time.Minute), "before")
	after := plant(t, pool, base, last, 2000, verdictAt.Add(time.Minute), "after")
	later := plant(t, pool, base, last, 3000, verdictAt.Add(2*time.Minute), "later")

	// Not triaged yet: the two found after the verdict have no incident.
	in, flat := summaryOf(t, pool, judged)
	if c := in.Condition; c == nil || c.Held != 9 || c.Total != 11 || c.Unraised != 2 || c.End != EndVerdict ||
		c.EndedInto != "" || c.Verdict != audit.IncidentSuppressedAsBenign {
		t.Fatalf("the judged condition before triage: %+v", c)
	}
	says(t, judged, flat,
		"This incident stands for nine deliveries of the feed with a finding at this locus, of the eleven in its condition",
		"It was closed by a person's verdict, suppressed_as_benign on "+dmy(verdictAt)+
			": the condition's next delivery, registered on "+dmy(arrival(t, after))+
			", was found after the verdict, so it is new evidence and not part of what was judged; it has no incident yet.",
		"Two deliveries of its condition have no incident yet")

	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return verdictAt.Add(time.Hour) }
	for _, s := range []string{before, after, later} {
		if _, err := w.Run(ctx, pool, s); err != nil {
			t.Fatal(err)
		}
	}
	fresh := shadow.IncidentIdFor(after, departsLocus)
	in, flat = summaryOf(t, pool, judged)
	if c := in.Condition; c == nil || c.Held != 9 || c.End != EndVerdict || c.EndedInto != fresh ||
		!slices.Equal(c.Others, []string{fresh}) || c.Unraised != 0 {
		t.Fatalf("the judged condition after triage: %+v", c)
	}
	says(t, judged, flat, "it belongs to incident "+fresh+".",
		"Other deliveries of its condition belong to other incidents: "+fresh+".")

	in, flat = summaryOf(t, pool, fresh)
	c := in.Condition
	if c == nil || c.Held != 2 || c.Total != 11 || c.End != EndOpen || c.Previous.Kind != PrevAfterVerdict ||
		c.Previous.IncidentID != judged || c.Previous.Verdict != audit.IncidentSuppressedAsBenign {
		t.Fatalf("the condition opened after the verdict: %+v", c)
	}
	says(t, fresh, flat,
		"This incident stands for two deliveries of the feed with a finding at this locus, of the eleven in its "+
			"condition, registered on "+dmy(arrival(t, after)),
		"It is still open",
		"Other deliveries of its condition belong to other incidents: "+judged+".",
		"It was opened after a person's verdict on incident "+judged+", earlier in the same condition, "+
			"suppressed_as_benign on "+dmy(verdictAt)+": this incident's first delivery was found after that verdict")
}
