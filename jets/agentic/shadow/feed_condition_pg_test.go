package shadow

// A feed's standing condition is one incident, end to end against a real
// Postgres: the feed fixture is installed, the feed detectors run, shadow
// triage classifies the sessions they fired on, and the record is read back.
//
//	docker run -d --rm --name shadow-feed-pg -e POSTGRES_PASSWORD=pw -p 55892:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run Condition ./jets/agentic/shadow/

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

const departs = triage.LocusDeliveryDepartsProfile

// detectedFixture installs the fixture in a fresh database and runs the
// detectors over it.
func detectedFixture(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	pool := freshDB(t, name, migratedTables, true)
	if _, err := feedfixture.Install(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	if n := detectFeeds(t, pool); n != 54 {
		t.Fatalf("the detectors wrote %d anomalies, want 54", n)
	}
	return pool
}

// triageAll runs shadow triage over every session carrying a feed anomaly, as
// the daily run selects them, and returns what it wrote.
func triageAll(t *testing.T, pool *pgxpool.Pool, w *Writer) (sessions, written []string) {
	t.Helper()
	ctx := context.Background()
	sessions, err := RecentFeedSessions(ctx, pool, feedfixture.Clock.Add(-time.Hour), "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
		for _, wr := range res.Written {
			written = append(written, wr.IncidentId)
		}
	}
	return sessions, written
}

// basisOf is the rationale of an incident's move to triaged: its basis.
func basisOf(t *testing.T, pool *pgxpool.Pool, incident string) string {
	t.Helper()
	var r string
	if err := pool.QueryRow(context.Background(), `SELECT transition_rationale FROM jetsapi.incident_event
		WHERE event_incident_ref = $1 AND to_status = 'triaged'`, incident).Scan(&r); err != nil {
		t.Fatalf("the basis of %s: %v", incident, err)
	}
	return r
}

func sessionsOf(f feedfixture.Feed) []string {
	out := make([]string, len(f.Deliveries))
	for i, d := range f.Deliveries {
		out[i] = d.SessionID
	}
	return out
}

// The weekly repeat is one incident over eight deliveries; the interrupted
// condition is two, the second naming the first; and a second pass writes
// nothing.
func TestAStandingConditionIsOneIncident(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_condition")
	ctx := context.Background()
	// The read-only reader sees the eight conditions before anything is
	// triaged, and writes nothing.
	all, err := feedcondition.ReadAll(ctx, pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range all {
		n += len(f.Conditions)
	}
	if n != 8 || count(t, pool, `SELECT count(*) FROM jetsapi.incident`) != 0 {
		t.Errorf("the reader read %d conditions, want 8, and must write nothing", n)
	}

	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	sessions, _ := triageAll(t, pool, &w)

	// The misdeclared cumulative feed sends a repeat every week: one condition
	// of its eight deliveries, one incident, on the first.
	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	inc := IncidentIdFor(ws[0], departs)
	if got := incidentsOn(t, pool, ws, departs); !slices.Equal(got, []string{inc}) {
		t.Errorf("incidents on the weekly repeat %v, want only %s", got, inc)
	}
	fc, err := feedcondition.Read(ctx, pool, weekly.TableName)
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range ws {
		p := fc.Resolve(s, departs)
		if (i == 0 && p.Kind != feedcondition.AlreadyRaised) || (i > 0 && (p.Kind != feedcondition.Join ||
			p.Target.IncidentId != inc)) {
			t.Errorf("delivery %d resolves to %s %v, want it in %s", i+1, p.Kind, p.Target, inc)
		}
	}
	c := triage.ConditionAt(fc.Conditions, ws[0], departs)
	if c == nil || len(c.Members) != 8 || !c.Open() {
		t.Fatalf("the weekly condition: %+v", c)
	}
	for _, want := range []string{"8 deliveries", ws[7], "it is open", "It is written on the condition's first delivery",
		"No earlier delivery of the feed has a finding at this locus"} {
		if !strings.Contains(basisOf(t, pool, inc), want) {
			t.Errorf("the weekly incident's basis does not say %q:\n%s", want, basisOf(t, pool, inc))
		}
	}

	// The wide-span feed's condition is interrupted by one delivery with no
	// finding: two conditions of three, two incidents, the second naming the
	// first.
	wide, _ := feedfixture.FeedByTable(feedfixture.WideSpanIncremental)
	fc, err = feedcondition.Read(ctx, pool, wide.TableName)
	if err != nil {
		t.Fatal(err)
	}
	var runs []*triage.Condition
	for i := range fc.Conditions {
		if fc.Conditions[i].Locus == departs {
			runs = append(runs, &fc.Conditions[i])
		}
	}
	if len(runs) != 2 || len(runs[0].Members) != 3 || len(runs[1].Members) != 3 || runs[0].End != triage.EndNoFinding {
		t.Fatalf("the wide-span feed's conditions: %d, want two of three, the first ended by a delivery with no finding",
			len(runs))
	}
	first, second := IncidentIdFor(runs[0].First().SessionId, departs), IncidentIdFor(runs[1].First().SessionId, departs)
	if got := incidentsOn(t, pool, sessionsOf(wide), departs); !slices.Equal(got, []string{first, second}) &&
		!slices.Equal(got, []string{second, first}) {
		t.Errorf("incidents on the wide-span feed %v, want %s and %s", got, first, second)
	}
	// The daily run triages newest first, so the second condition's incident
	// is raised before the first's, and names where the first's will be.
	if b := basisOf(t, pool, second); !strings.Contains(b, "The previous condition of the feed at this locus, 3 "+
		"deliveries from session "+runs[0].First().SessionId) || !strings.Contains(b, "has no incident yet") {
		t.Errorf("the second incident does not name the first condition:\n%s", b)
	}
	// Raised oldest first, it names the first's incident and its verdict.
	for _, id := range []string{first, second} {
		if _, err := pool.Exec(ctx, `DELETE FROM jetsapi.hypothesis WHERE hypothesis_incident_ref = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM jetsapi.incident_event WHERE event_incident_ref = $1`, id); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM jetsapi.incident WHERE incident_id = $1`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range sessionsOf(wide) {
		if _, err := w.Run(ctx, pool, s); err != nil {
			t.Fatal(err)
		}
	}
	if b := basisOf(t, pool, second); !strings.Contains(b, "has incident "+first) || !strings.Contains(b, "with no verdict") {
		t.Errorf("raised oldest first, the second incident does not name the first and its verdict:\n%s", b)
	}

	// A second pass writes nothing.
	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 {
			t.Errorf("a second pass over %s wrote %v", s, res.Written)
		}
	}
	if after := count(t, pool, `SELECT count(*) FROM jetsapi.incident`); after != before || after != 8 {
		t.Errorf("%d incidents after the second pass, %d before; want 8", after, before)
	}
	if a, err := Attest(ctx, pool); err != nil || !a.Holds() {
		t.Errorf("the attestation: %v\n%s", err, a.Report())
	}
}

// plantAfter writes a copy of an anomaly on a new session with no
// registration, ms after the feed's last delivery, detected at detected.
func plantAfter(t *testing.T, pool *pgxpool.Pool, base observe.Anomaly, after string, ms int64,
	detected time.Time, suffix string) string {
	t.Helper()
	last, err := strconv.ParseInt(after, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	a := base
	a.SessionId = strconv.FormatInt(last+ms, 10)
	a.AnomalyId = base.AnomalyId + "/" + suffix
	a.DetectedAt = detected
	if err := observe.InsertAnomaly(context.Background(), pool, &a); err != nil {
		t.Fatal(err)
	}
	return a.SessionId
}

// departsAnomalyOn returns a feed anomaly at the departs-profile locus on a
// session.
func departsAnomalyOn(t *testing.T, pool *pgxpool.Pool, session string) observe.Anomaly {
	t.Helper()
	as, err := observe.ReadAnomalies(context.Background(), pool, session)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range as {
		if l, ok := triage.LocusOf(a.SubjectType, a.SignalType); ok && l == departs {
			return a
		}
	}
	t.Fatalf("no departs-profile anomaly on %s", session)
	return observe.Anomaly{}
}

// A person's verdict closes a condition for later evidence: a finding detected
// before the verdict joins the judged incident, one detected after it opens a
// new incident naming the judged one and its verdict, and a finding after that
// joins the new incident. Nothing is suppressed.
func TestAVerdictMidConditionOpensANewIncident(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_condition_verdict")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	triageAll(t, pool, &w)

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	judged := IncidentIdFor(ws[0], departs)
	verdictAt := feedfixture.Clock.Add(time.Hour)
	if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
		IncidentEventId: "ie_person_benign", IncidentRef: judged,
		FromStatus: audit.IncidentDiagnosed, ToStatus: audit.IncidentSuppressedAsBenign,
		Actor: "reviewer@example.com", ActorKind: audit.ActorHuman, TransitionedAt: verdictAt,
		Rationale: "the repeat is expected",
	}); err != nil {
		t.Fatal(err)
	}

	// The condition's own deliveries were detected before the verdict: a pass
	// over them after it writes nothing.
	for _, s := range ws {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 {
			t.Errorf("delivery %s, detected before the verdict, wrote %v after it", s, res.Written)
		}
	}

	base := departsAnomalyOn(t, pool, ws[len(ws)-1])
	last := ws[len(ws)-1]
	before := plantAfter(t, pool, base, last, 1000, verdictAt.Add(-time.Minute), "before")
	after := plantAfter(t, pool, base, last, 2000, verdictAt.Add(time.Minute), "after")
	later := plantAfter(t, pool, base, last, 3000, verdictAt.Add(2*time.Minute), "later")

	w.Now = func() time.Time { return verdictAt.Add(time.Hour) }
	res, err := w.Run(ctx, pool, before)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], judged) {
		t.Errorf("a finding detected before the verdict: written %v, grouped %v; want joined to %s",
			res.Written, res.Grouped, judged)
	}
	res, err = w.Run(ctx, pool, after)
	if err != nil {
		t.Fatal(err)
	}
	fresh := IncidentIdFor(after, departs)
	if len(res.Written) != 1 || res.Written[0].IncidentId != fresh || len(res.Grouped) != 0 {
		t.Fatalf("a finding detected after the verdict: written %v, grouped %v; want %s written",
			res.Written, res.Grouped, fresh)
	}
	b := basisOf(t, pool, fresh)
	for _, want := range []string{"is new evidence", judged, "suppressed_as_benign by reviewer@example.com",
		"The previous incident at this locus is " + judged} {
		if !strings.Contains(b, want) {
			t.Errorf("the new incident's basis does not say %q:\n%s", want, b)
		}
	}
	res, err = w.Run(ctx, pool, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], fresh) {
		t.Errorf("a later finding: written %v, grouped %v; want joined to %s", res.Written, res.Grouped, fresh)
	}

	// Nothing was suppressed: the judged incident keeps its verdict, the new
	// one is at diagnosed, and the attestation holds.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, fresh).
		Scan(&status); err != nil || status != audit.IncidentDiagnosed {
		t.Errorf("the new incident is at %s (%v), want diagnosed", status, err)
	}
	if a, err := Attest(ctx, pool); err != nil || !a.Holds() {
		t.Errorf("the attestation: %v\n%s", err, a.Report())
	}
}

// Two loci on one delivery are two conditions: a planted delivery after the
// weekly repeat carries a departure, which joins the repeat's incident, and an
// arrival finding, which is a condition of one with an incident of its own.
func TestTwoLociOnOneDeliveryAreTwoConditions(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_condition_loci")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	triageAll(t, pool, &w)

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	last := ws[len(ws)-1]
	base := departsAnomalyOn(t, pool, last)
	s := plantAfter(t, pool, base, last, 1000, feedfixture.Clock, "loci")
	overdue := base
	overdue.SessionId, overdue.AnomalyId = s, base.AnomalyId+"/overdue"
	overdue.SignalType, overdue.DetectorRef = observe.SignalArrival, "feed_overdue@1"
	overdue.DetectedAt = feedfixture.Clock
	if err := observe.InsertAnomaly(ctx, pool, &overdue); err != nil {
		t.Fatal(err)
	}

	res, err := w.Run(ctx, pool, s)
	if err != nil {
		t.Fatal(err)
	}
	own := IncidentIdFor(s, triage.LocusDeliveryOverdue)
	if len(res.Written) != 1 || res.Written[0].IncidentId != own {
		t.Errorf("written %v, want the overdue incident %s", res.Written, own)
	}
	if len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], IncidentIdFor(ws[0], departs)) {
		t.Errorf("grouped %v, want the departure joined to the weekly repeat's incident", res.Grouped)
	}
}

// A run that selects only a condition's latest delivery raises
// the condition's incident on its first delivery, and joins it; a later run
// over the first delivery finds it raised. A dry run reports the same and
// writes nothing.
func TestOnlyTheLastDeliveryTriagedRaisesTheFirst(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_condition_anchor")
	ctx := context.Background()
	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	last := ws[len(ws)-1]
	anchor := IncidentIdFor(ws[0], departs)

	dry := testWriter()
	dry.DryRun = true
	res, err := dry.Run(ctx, pool, last)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 1 || res.Written[0].IncidentId != anchor || res.Written[0].SessionId != ws[0] {
		t.Errorf("the dry run reports %v, want %s on %s", res.Written, anchor, ws[0])
	}
	// A dry run over another delivery of the condition reports joining it.
	if res, err := dry.Run(ctx, pool, ws[3]); err != nil || len(res.Written) != 0 || len(res.Grouped) != 1 {
		t.Errorf("the dry run's next delivery: %v, written %v, grouped %v", err, res.Written, res.Grouped)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident`); n != 0 {
		t.Fatalf("a dry run wrote %d incidents", n)
	}

	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	res, err = w.Run(ctx, pool, last)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 1 || res.Written[0].IncidentId != anchor || res.Written[0].Status != audit.IncidentDiagnosed {
		t.Fatalf("written %v, want %s raised at diagnosed", res.Written, anchor)
	}
	if len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], anchor) {
		t.Errorf("grouped %v, want into %s", res.Grouped, anchor)
	}
	if b := basisOf(t, pool, anchor); !strings.Contains(b, "It was raised when session "+last) {
		t.Errorf("the anchor's basis does not say what raised it:\n%s", b)
	}
	if got := incidentsOn(t, pool, ws, departs); !slices.Equal(got, []string{anchor}) {
		t.Errorf("incidents %v, want only %s", got, anchor)
	}
	res, err = w.Run(ctx, pool, ws[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.AlreadyRaised) == 0 {
		t.Errorf("the first delivery after its incident was raised: written %v, already raised %v",
			res.Written, res.AlreadyRaised)
	}
}

// legacyRaise writes a session's own incident at a locus, as shadow triage did
// for every delivery before conditions.
func legacyRaise(t *testing.T, pool *pgxpool.Pool, w *Writer, session, locus string) string {
	t.Helper()
	ctx := context.Background()
	_, report, ranking, _, err := w.classify(ctx, pool, session)
	if err != nil {
		t.Fatal(err)
	}
	var hyps []rca.Hypothesis
	for _, h := range ranking.Hypotheses {
		if h.Locus == locus {
			hyps = append(hyps, h)
		}
	}
	for i := range report.Findings {
		f := &report.Findings[i]
		if f.Locus != locus || !f.Fired() {
			continue
		}
		wr := &IncidentWritten{IncidentId: IncidentIdFor(session, locus), SessionId: session, Locus: locus,
			Status: audit.IncidentDetected, Hypotheses: len(hyps)}
		if err := w.writeOne(ctx, pool, f, hyps, ranking, SeverityInfo, ModelVersion, wr); err != nil {
			t.Fatal(err)
		}
		return wr.IncidentId
	}
	t.Fatalf("%s does not hold on %s", locus, session)
	return ""
}

// Incidents written one per delivery before conditions are never retracted: a
// pass under conditions over a database holding them writes nothing, the
// read-only reader shows them as one condition with eight incidents, and a
// later delivery of the condition joins the latest of them.
func TestIncidentsWrittenPerDeliveryAreNeverRetracted(t *testing.T) {
	pool := detectedFixture(t, "shadow_feed_condition_legacy")
	ctx := context.Background()
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	var legacy []string
	for _, s := range ws {
		legacy = append(legacy, legacyRaise(t, pool, &w, s, departs))
	}

	for _, s := range ws {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 || len(res.AlreadyRaised) != 1 {
			t.Errorf("%s: written %v, already raised %v; want its own incident left alone", s, res.Written,
				res.AlreadyRaised)
		}
	}
	if got := incidentsOn(t, pool, ws, departs); len(got) != len(ws) {
		t.Errorf("%d incidents on the weekly repeat after a pass under conditions, want the %d written before",
			len(got), len(ws))
	}
	fc, err := feedcondition.Read(ctx, pool, weekly.TableName)
	if err != nil {
		t.Fatal(err)
	}
	c := triage.ConditionAt(fc.Conditions, ws[0], departs)
	if c == nil || len(c.Members) != 8 || len(fc.IncidentsOn(c)) != 8 {
		t.Fatalf("the reader shows %+v", c)
	}

	last := ws[len(ws)-1]
	next := plantAfter(t, pool, departsAnomalyOn(t, pool, last), last, 1000, feedfixture.Clock, "next")
	res, err := w.Run(ctx, pool, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 0 || len(res.Grouped) != 1 || !strings.Contains(res.Grouped[0], legacy[len(legacy)-1]) {
		t.Errorf("the next delivery: written %v, grouped %v; want it joined to %s", res.Written, res.Grouped,
			legacy[len(legacy)-1])
	}
}
