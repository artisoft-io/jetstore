package shadow

// A feed whose deliveries look cumulative under an incremental declaration gets
// a change proposal in draft, end to end against a real Postgres: the feed
// fixture is installed, the feed detectors and shadow triage run as library
// calls, the advisory step drafts, and the record is read back.
//
//	docker run -d --rm --name shadow-proposal-pg -e POSTGRES_PASSWORD=pw -p 55893:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55893/postgres \
//	  go test -count=1 -run FeedProposal ./jets/agentic/shadow/
//
// Run it with -count=1: the fixture reads jets_schema.json, which the test cache
// does not see.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedproposal"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// restate writes one delivery_overlap anomaly at a share of 1 on a delivery of
// an incremental feed, as the detector would on a delivery that restates its
// predecessor.
func restate(t *testing.T, pool *pgxpool.Pool, table, session string) {
	t.Helper()
	max := "0.0500"
	if err := observe.InsertAnomaly(context.Background(), pool, &observe.Anomaly{
		AnomalyId:     fmt.Sprintf("%s@1/%s/%s", observe.ClaimSetDeliveryOverlap, table, session),
		DetectedAt:    feedfixture.Clock,
		SessionId:     session,
		SubjectType:   observe.SubjectFeed,
		SubjectRef:    table,
		SignalType:    observe.SignalCardinality,
		ObservedValue: "1.0000",
		ExpectedMax:   &max,
		ExpectedBasis: "a restated delivery, planted by the test",
		Confounders:   []string{},
		DetectorRef:   observe.ClaimSetDeliveryOverlap + "@1",
	}); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, q string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// TestFeedProposalIsDraftedForCumulativeUnderIncremental asserts exactly one
// draft, for the feed whose deliveries look cumulative while
// it is declared incremental; none for the stale cumulative feed, for a feed
// with one restated delivery, or for any other; the incident stays at
// diagnosed with no remediation and the attestation holding; a second pass
// drafts nothing; and no canary reaches the proposal.
func TestFeedProposalIsDraftedForCumulativeUnderIncremental(t *testing.T) {
	pool := freshDB(t, "shadow_feed_proposals", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if n := detectFeeds(t, pool); n != 54 {
		t.Fatalf("the detectors wrote %d anomalies, want 54", n)
	}

	// The healthy incremental control, given what a feed that restates now
	// and then looks like: a run of three restated deliveries in the middle of
	// its history, and one restated delivery at its end. Neither is the case:
	// the first is not where the feed is now, and the second is one delivery.
	sibling, _ := feedfixture.FeedByTable(feedfixture.SiblingIncremental)
	n := len(sibling.Deliveries)
	for _, i := range []int{4, 5, 6, n - 1} {
		restate(t, pool, sibling.TableName, sibling.Deliveries[i].SessionID)
	}

	since := feedfixture.Clock.Add(-time.Hour)
	sessions, err := RecentFeedSessions(ctx, pool, since, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	for _, s := range sessions {
		if _, err := w.Run(ctx, pool, s); err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
	}
	eventsBefore := count(t, pool, `SELECT count(*) FROM jetsapi.incident_event`)
	incidentsBefore := count(t, pool, `SELECT count(*) FROM jetsapi.incident`)

	res, err := w.DraftFeedProposals(ctx, pool, sessions)
	if err != nil {
		t.Fatal(err)
	}

	// The verdict on every feed that carries an overlap or unchanged anomaly:
	// the denominator beside the one draft.
	want := map[string]string{
		feedfixture.CumulativeDeclaredIncremental: "",
		feedfixture.StaleCumulative:               feedproposal.ReasonDeclaredCumulative,
		feedfixture.ShrinkingCumulative:           feedproposal.ReasonDeclaredCumulative,
		feedfixture.SiblingIncremental:            feedproposal.ReasonShortStreak,
	}
	if len(res.Assessed) != len(want) {
		t.Errorf("%d feeds assessed, want %d", len(res.Assessed), len(want))
	}
	var drafted *feedproposal.Assessment
	for i := range res.Assessed {
		a := &res.Assessed[i]
		t.Logf("%-34s deliveries %2d  run of evidence %d  qualifies %-5v %s",
			a.TableName, len(a.Deliveries), len(a.Streak), a.Qualifies, a.Reason)
		reason, ok := want[a.TableName]
		switch {
		case !ok:
			t.Errorf("feed %s was assessed and carries no overlap or unchanged anomaly", a.TableName)
		case reason == "" && !a.Qualifies:
			t.Errorf("feed %s does not qualify (%s)", a.TableName, a.Reason)
		case reason != "" && (a.Qualifies || a.Reason != reason):
			t.Errorf("feed %s: qualifies %v, reason %q, want reason %q", a.TableName, a.Qualifies, a.Reason, reason)
		}
		if a.Qualifies {
			drafted = a
		}
		if a.TableName == feedfixture.SiblingIncremental && len(a.Streak) != 1 {
			t.Errorf("the restating feed's run of evidence is %d, want 1: its latest delivery alone", len(a.Streak))
		}
	}
	if drafted == nil {
		t.Fatal("no feed qualified")
	}

	// Exactly one draft, for the misdeclared feed.
	if len(res.Drafts) != 1 || res.Written() != 1 {
		t.Fatalf("%d drafts, %d written; want exactly one", len(res.Drafts), res.Written())
	}
	d := res.Drafts[0]
	if d.Assessment.TableName != feedfixture.CumulativeDeclaredIncremental {
		t.Fatalf("the draft is for %s", d.Assessment.TableName)
	}
	// Its deliveries 2 to 8 are each mostly what their predecessor held; the
	// first has no predecessor and is not judged.
	feed, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	if got, want := len(d.Assessment.Streak), len(feed.Deliveries)-1; got != want {
		t.Errorf("the run of evidence is %d deliveries, want %d", got, want)
	}
	onset := IncidentIdFor(feed.Deliveries[1].SessionID, triage.LocusDeliveryDepartsProfile)
	if d.Proposal.TriggerRef != onset {
		t.Errorf("the draft is triggered by %s, want the onset's incident %s", d.Proposal.TriggerRef, onset)
	}

	if n := count(t, pool, `SELECT count(*) FROM jetsapi.change_proposal`); n != 1 {
		t.Errorf("%d change proposals recorded, want 1", n)
	}
	p, err := audit.ReadProposal(ctx, pool, d.Proposal.ProposalId)
	if err != nil {
		t.Fatal(err)
	}
	if p.ApprovalState != audit.StateDraft || p.Trigger != feedproposal.Trigger || p.TriggerRef != onset {
		t.Errorf("recorded as state %s, trigger %s %s", p.ApprovalState, p.Trigger, p.TriggerRef)
	}
	for _, s := range []string{
		"source_config.is_cumulative to 1", "feed " + feedfixture.CumulativeDeclaredIncremental,
		"only the latest delivery across the whole lookback", "Nothing has been applied",
		onset,
	} {
		if !strings.Contains(p.Rationale, s) {
			t.Errorf("the rationale does not say %q:\n%s", s, p.Rationale)
		}
	}
	t.Logf("proposal %s:\n%s", p.ProposalId, p.Rationale)

	// Nothing else moved: the incident is at diagnosed, drafting wrote no
	// incident and no transition, there is no remediation, and the
	// attestation holds and counts the draft.
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, onset).
		Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != audit.IncidentDiagnosed {
		t.Errorf("the triggering incident is at %s, want diagnosed", status)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident_event`); n != eventsBefore {
		t.Errorf("drafting wrote %d incident transitions", n-eventsBefore)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident`); n != incidentsBefore {
		t.Errorf("drafting wrote %d incidents", n-incidentsBefore)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.remediation`); n != 0 {
		t.Errorf("%d remediations", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.approval_event`); n != 0 {
		t.Errorf("%d approval events: nothing decided the draft", n)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident WHERE status <> 'diagnosed'`); n != 0 {
		t.Errorf("%d incidents are not at diagnosed", n)
	}
	a, err := Attest(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Holds() {
		t.Errorf("the attestation does not hold:\n%s", a.Report())
	}
	if !a.ProposalsRead || a.IncidentProposals != 1 || a.IncidentProposalsInDraft != 1 {
		t.Errorf("the attestation counts %d incident proposals, %d in draft (read %v)",
			a.IncidentProposals, a.IncidentProposalsInDraft, a.ProposalsRead)
	}
	t.Logf("%s", a.Report())

	// No canary reaches the proposal.
	for _, c := range feedfixture.Canaries() {
		if n := count(t, pool, `SELECT count(*) FROM jetsapi.change_proposal r
			WHERE row_to_json(r)::text LIKE '%' || $1 || '%'`, c); n != 0 {
			t.Errorf("%d change proposals carry the canary %q", n, c)
		}
	}

	// A second pass, triage and drafting both, writes nothing.
	for _, s := range sessions {
		r, err := w.Run(ctx, pool, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Written) != 0 {
			t.Errorf("a second pass over %s wrote %d incidents", s, len(r.Written))
		}
	}
	again, err := w.DraftFeedProposals(ctx, pool, sessions)
	if err != nil {
		t.Fatal(err)
	}
	if again.Written() != 0 || len(again.Drafts) != 1 || again.Drafts[0].Existing != audit.StateDraft {
		t.Errorf("a second pass drafted %d (drafts %d)", again.Written(), len(again.Drafts))
	}
	// An unscoped pass, over every feed, finds the same one.
	all, err := w.DraftFeedProposals(ctx, pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	if all.Written() != 0 {
		t.Errorf("an unscoped pass drafted %d", all.Written())
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.change_proposal`); n != 1 {
		t.Errorf("%d change proposals after the second pass, want 1", n)
	}

	// The attestation notices a draft that left draft with nobody deciding it.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.change_proposal SET approval_state = 'approved'
		WHERE proposal_id = $1`, p.ProposalId); err != nil {
		t.Fatal(err)
	}
	if a, err := Attest(ctx, pool); err != nil || a.Holds() || a.UnexplainedProposalStates != 1 {
		t.Errorf("an out-of-band approval of the draft is not reported: %v\n%s", err, a.Report())
	}
}
