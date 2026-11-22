package shadow

// Feed findings become incidents in shadow mode, end to end against a real
// Postgres: the synthetic feed fixture is installed, the eight feed detectors
// run as library calls and write their anomalies, shadow triage classifies the
// sessions those anomalies hang on, and the record is read back.
//
//	docker run -d --rm --name shadow-feed-pg -e POSTGRES_PASSWORD=pw -p 55892:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run FeedFindings ./jets/agentic/shadow/
//
// Run it with -count=1: the fixture reads jets_schema.json, which the test cache
// does not see.

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// detectFeeds runs the eight feed detectors over the installed fixture, as of
// its clock, and writes their anomalies. It returns how many were written.
func detectFeeds(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	ctx := context.Background()
	var profiled, unprofiled []feedprofile.Feed
	clients := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if clients[f.Client] {
			continue
		}
		clients[f.Client] = true
		p, u, err := feedprofile.Load(ctx, pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		profiled, unprofiled = append(profiled, p...), append(unprofiled, u...)
	}
	if len(profiled) == 0 {
		t.Fatal("no feed was read with a profile: the fixture did not fill source_config.feed_profile_json")
	}
	in, err := observe.ReadFeedDeliveryInput(ctx, pool, profiled, unprofiled, feedfixture.Clock)
	if err != nil {
		t.Fatal(err)
	}
	written := 0
	for _, d := range []interface {
		Detect(*observe.FeedDeliveryInput) (observe.FeedDeliveryResult, error)
	}{observe.DefaultFeedOverdue(), observe.DefaultDeliverySize(), observe.DefaultDeliveryBackdated()} {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		n, err := observe.FeedDeliveryWrite(ctx, pool, res)
		if err != nil {
			t.Fatal(err)
		}
		written += n
	}
	o := observe.DefaultClaimSetOptions()
	o.Now = feedfixture.Clock
	verdicts, err := observe.DetectClaimSets(ctx, pool, append(slices.Clone(profiled), unprofiled...), o)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := observe.WriteClaimSetAnomalies(ctx, pool, verdicts)
	if err != nil {
		t.Fatal(err)
	}
	return written + n
}

// TestFeedFindingsBecomeIncidentsInShadowMode is the end-to-end case: every feed
// anomaly reaches an incident at the delivery locus LocusOf places it at, and
// no other incident is written; every incident stops at diagnosed with the
// floor's classes for its locus; every hypothesis carries the feed's
// declaration against it and the overdue one carries the declared cadence for
// it; the attestation holds; and no canary reaches any incident, hypothesis or
// transition.
func TestFeedFindingsBecomeIncidentsInShadowMode(t *testing.T) {
	pool := freshDB(t, "shadow_feed_incidents", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}

	// One delivery registered under client Any, as some deployments register
	// them: the stopped feed's last, which is where its overdue finding hangs.
	// It is attributed to the feed by table name, so no verdict moves.
	stopped, _ := feedfixture.FeedByTable(feedfixture.StoppedIncremental)
	lastStopped := stopped.Deliveries[len(stopped.Deliveries)-1].SessionID
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.input_registry SET client = 'Any'
		WHERE session_id = $1 AND table_name = $2`, lastStopped, feedfixture.StoppedIncremental); err != nil {
		t.Fatal(err)
	}

	if n := detectFeeds(t, pool); n != 54 {
		t.Errorf("the detectors wrote %d anomalies, want 54: 10 at delivery grain and 44 over the claim sets", n)
	}

	// The incidents the anomalies call for, by (session, locus).
	want := map[string]string{} // incident id -> locus
	rows, err := pool.Query(ctx, `SELECT anomaly_session_id, anomaly_subject_type, anomaly_signal_type
		FROM jetsapi.anomaly`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var session, subject, signal string
		if err := rows.Scan(&session, &subject, &signal); err != nil {
			t.Fatal(err)
		}
		l, ok := triage.LocusOf(subject, signal)
		if !ok {
			t.Fatalf("a feed detector wrote a (%s, %s) anomaly that no locus takes", subject, signal)
		}
		want[IncidentIdFor(session, l)] = l
	}
	rows.Close()

	// The sessions: none is a run, so the run selector finds nothing, and the
	// feed selector finds every session an anomaly hangs on.
	since := feedfixture.Clock.Add(-time.Hour)
	runs, err := RecentSessions(ctx, pool, since, "", "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("the run selector found %d sessions in a fixture with no run", len(runs))
	}
	sessions, err := RecentFeedSessions(ctx, pool, since, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	// The client filter is the feed's own: the stopped feed's last session is
	// registered under Any and is selected under the feed's client.
	ofClient, err := RecentFeedSessions(ctx, pool, since, stopped.Client, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(ofClient, lastStopped) {
		t.Errorf("the stopped feed's last session, registered under Any, is not selected under client %s",
			stopped.Client)
	}
	if anyOf, _ := RecentFeedSessions(ctx, pool, since, "Any", 1000); len(anyOf) != 0 {
		t.Errorf("%d sessions selected under client Any, which owns no feed", len(anyOf))
	}

	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	census := map[string]int{}
	for _, s := range MergeSessions(runs, sessions) {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
		for _, f := range res.Report.Findings {
			census[f.Locus+"/"+string(f.Verdict)]++
		}
		for _, wr := range res.Written {
			if wr.Status != audit.IncidentDiagnosed {
				t.Errorf("%s was left at %s, want diagnosed", wr.IncidentId, wr.Status)
			}
		}
	}
	for _, l := range triage.Loci {
		t.Logf("%-34s present %3d  absent %3d  not_evaluable %3d", l, census[l+"/present"],
			census[l+"/absent"], census[l+"/not_evaluable"])
		// A registration session has no run, so no run locus was asked: none
		// may be counted into a run locus's denominator.
		if !slices.Contains(triage.DeliveryLoci, l) && census[l+"/present"]+census[l+"/absent"] != 0 {
			t.Errorf("run locus %s was evaluated on a registration session with no run", l)
		}
	}

	// Exactly the incidents the anomalies call for: one per (session, locus),
	// and none at a run locus.
	got := map[string]string{}
	rows, err = pool.Query(ctx, `SELECT incident_id, incident_locus, status FROM jetsapi.incident`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, locus, status string
		if err := rows.Scan(&id, &locus, &status); err != nil {
			t.Fatal(err)
		}
		got[id] = locus
		if status != audit.IncidentDiagnosed {
			t.Errorf("%s is at %s, want diagnosed", id, status)
		}
	}
	rows.Close()
	if len(got) != len(want) {
		t.Errorf("%d incidents written, want %d", len(got), len(want))
	}
	for id, l := range want {
		if got[id] != l {
			t.Errorf("incident %s at %s is missing (got locus %q)", id, l, got[id])
		}
	}
	t.Logf("%d anomalies became %d incidents over %d sessions", 54, len(got), len(sessions))

	profiles, err := feedfixture.Profiles()
	if err != nil {
		t.Fatal(err)
	}
	wantClasses := map[string][]string{
		triage.LocusDeliveryOverdue: {rca.CauseSourceDeliveryFailure, rca.CauseBenignVariation},
		triage.LocusDeliveryDepartsProfile: {rca.CauseSourceContentChange, rca.CauseTransformationDefect,
			rca.CauseBenignVariation},
	}
	for id, locus := range got {
		inc, err := audit.ReadIncident(ctx, pool, id, audit.DisclosePHI)
		if err != nil {
			t.Fatal(err)
		}
		var classes []string
		for _, h := range inc.Hypotheses {
			classes = append(classes, h.CauseCategory)
			if h.Locus != locus {
				t.Errorf("%s: hypothesis %s is at %s under an incident at %s", id, h.HypothesisId, h.Locus, locus)
			}
			if h.Basis.Evidenceability != string(rca.EvidenceabilityAt(h.CauseCategory, h.Locus)) {
				t.Errorf("%s: %s's basis records %q", id, h.CauseCategory, h.Basis.Evidenceability)
			}
			// Every feed hypothesis carries its declaration's author and date
			// against it.
			against := statements(h.ContradictingEvidence)
			if !strings.Contains(against, "declared by fixture@example.com on 2026-") {
				t.Errorf("%s: %s does not carry the declaration against it:\n%s", id, h.CauseCategory, against)
			}
			if h.CauseCategory == rca.CauseSourceDeliveryFailure {
				if !strings.Contains(statements(h.SupportingEvidence), "the declared cadence is what was due") {
					t.Errorf("%s: source_delivery_failure does not carry the declared cadence for it", id)
				}
			}
		}
		slices.Sort(classes)
		wc := slices.Clone(wantClasses[locus])
		slices.Sort(wc)
		if !slices.Equal(classes, wc) {
			t.Errorf("%s at %s has classes %v, want %v", id, locus, classes, wc)
		}
	}

	// The stopped feed's overdue incident hangs on its last delivery and names
	// the feed's own client, not the client its last delivery was registered
	// under.
	overdue := IncidentIdFor(lastStopped, triage.LocusDeliveryOverdue)
	if got[overdue] != triage.LocusDeliveryOverdue {
		t.Fatalf("no overdue incident on the stopped feed's last delivery %s", lastStopped)
	}
	var rationale string
	if err := pool.QueryRow(ctx, `SELECT transition_rationale FROM jetsapi.incident_event
		WHERE event_incident_ref = $1 AND to_status = 'triaged'`, overdue).Scan(&rationale); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"client " + stopped.Client, "registered under client Any",
		"the one after which nothing came"} {
		if !strings.Contains(rationale, s) {
			t.Errorf("the overdue incident's triage rationale does not say %q:\n%s", s, rationale)
		}
	}
	if strings.Contains(rationale, "of client Any") {
		t.Errorf("the overdue incident names the registration's client as the feed's:\n%s", rationale)
	}
	if _, ok := profiles[feedfixture.StoppedIncremental]; !ok {
		t.Errorf("the stopped feed has no profile in the fixture")
	}

	// Shadow mode's ceiling holds over them.
	a, err := Attest(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Holds() {
		t.Errorf("the attestation does not hold over the feed incidents:\n%s", a.Report())
	}
	if a.TransitionsIntoActing != 0 || a.IncidentsAtActingStatus != 0 || a.Remediations != 0 {
		t.Errorf("something acted: %d transitions into acting, %d incidents acting, %d remediations",
			a.TransitionsIntoActing, a.IncidentsAtActingStatus, a.Remediations)
	}

	// No canary reaches an incident, a hypothesis or a transition.
	for _, table := range []string{"incident", "hypothesis", "incident_event"} {
		for _, c := range feedfixture.Canaries() {
			var n int
			q := fmt.Sprintf(`SELECT count(*) FROM jetsapi.%s r WHERE row_to_json(r)::text LIKE '%%' || $1 || '%%'`, table)
			if err := pool.QueryRow(ctx, q, c).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("%d rows of jetsapi.%s carry the canary %q", n, table, c)
			}
		}
	}

	// A second pass adds nothing: every locus is already raised.
	for _, s := range sessions {
		res, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Written) != 0 {
			t.Errorf("a second pass over %s wrote %d incidents", s, len(res.Written))
		}
	}
}

func statements(es []audit.Evidence) string {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(e.Statement)
		b.WriteString("\n")
	}
	return b.String()
}
