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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
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
	// The trigger is the incident the run's latest delivery belongs to: its
	// feed condition's, written on the condition's first delivery, which is the
	// feed's first, one before the run of evidence (its finding there is not an
	// overlap: it has no predecessor to overlap).
	onset := IncidentIdFor(feed.Deliveries[0].SessionID, triage.LocusDeliveryDepartsProfile)
	if d.Proposal.TriggerRef != onset {
		t.Errorf("the draft is triggered by %s, want the condition's incident %s", d.Proposal.TriggerRef, onset)
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

	// A person moves it, as the proposals screen records a decision: each step
	// under the person's account, at the runless tier, with run_ref the
	// incident, and no chain event. The incident does not move, and the
	// attestation holds with every state explained.
	path := []string{audit.StateDraft, audit.StateValidated, audit.StateAgentReviewed,
		audit.StateAwaitingHumanApproval, audit.StateApproved}
	for i := 1; i < len(path); i++ {
		seq, err := audit.RecordApproval(ctx, pool, &audit.Approval{
			ApprovalEventId: fmt.Sprintf("apr_person_%d", i), RunRef: p.TriggerRef,
			SubjectRef: p.ProposalId, FromState: path[i-1], ToState: path[i],
			Actor: "person@example.com", TierAtEvent: audit.RunlessDecisionTier,
			DecisionRationale: "the feed is cumulative", Unchained: true,
		})
		if err != nil || seq != 0 {
			t.Fatalf("%s -> %s: seq %d, %v", path[i-1], path[i], seq, err)
		}
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.agent_audit`); n != 0 {
		t.Errorf("%d chain events after a person decided a proposal no run produced", n)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, onset).
		Scan(&status); err != nil || status != audit.IncidentDiagnosed {
		t.Errorf("the incident is at %s after the proposal was approved (%v)", status, err)
	}
	if a, err := Attest(ctx, pool); err != nil || !a.Holds() || a.UnexplainedProposalStates != 0 ||
		a.IncidentProposalsInDraft != 0 {
		t.Errorf("after a person's approval: %v\n%s", err, a.Report())
	}
	if again, err := w.DraftFeedProposals(ctx, pool, sessions); err != nil || again.Written() != 0 ||
		again.Drafts[0].Existing != audit.StateApproved {
		t.Errorf("a pass after the approval: %v, %+v", err, again)
	}

	// The attestation notices a proposal that moved with nobody deciding it.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.change_proposal SET approval_state = 'deployed'
		WHERE proposal_id = $1`, p.ProposalId); err != nil {
		t.Fatal(err)
	}
	if a, err := Attest(ctx, pool); err != nil || a.Holds() || a.UnexplainedProposalStates != 1 {
		t.Errorf("an out-of-band deployment of the proposal is not reported: %v\n%s", err, a.Report())
	}
}

// reregister registers a delivered file of a feed again under a new session,
// as JetStore records a re-processing: the same file key and period.
func reregister(t *testing.T, pool *pgxpool.Pool, table, ofSession string, at time.Time) string {
	t.Helper()
	session := fmt.Sprintf("%d", at.UnixMilli())
	tag, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
		SELECT client, org, object_type, file_key, source_period_key, table_name, source_type, $3, user_email, $4
		  FROM jetsapi.input_registry WHERE table_name = $1 AND session_id = $2`, table, ofSession, session, at)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("re-registering %s's session %s: %v (%d rows)", table, ofSession, err, tag.RowsAffected())
	}
	return session
}

// detectFeedsOf runs the eight feed detectors over the clients of feeds, as of
// the fixture's clock, and writes their anomalies.
func detectFeedsOf(t *testing.T, pool *pgxpool.Pool, feeds []feedfixture.Feed) {
	t.Helper()
	ctx := context.Background()
	var profiled, unprofiled []feedprofile.Feed
	clients := map[string]bool{}
	for _, f := range feeds {
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
	in, err := observe.ReadFeedDeliveryInput(ctx, pool, profiled, unprofiled, feedfixture.Clock)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []interface {
		Detect(*observe.FeedDeliveryInput) (observe.FeedDeliveryResult, error)
	}{observe.DefaultFeedOverdue(), observe.DefaultDeliverySize(), observe.DefaultDeliveryBackdated()} {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := observe.FeedDeliveryWrite(ctx, pool, res); err != nil {
			t.Fatal(err)
		}
	}
	o := observe.DefaultClaimSetOptions()
	o.Now = feedfixture.Clock
	verdicts, err := observe.DetectClaimSets(ctx, pool, append(slices.Clone(profiled), unprofiled...), o)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := observe.WriteClaimSetAnomalies(ctx, pool, verdicts); err != nil {
		t.Fatal(err)
	}
}

// TestFeedProposalReadsReprocessingAsNotADelivery: the misdeclared feed's files
// are registered again, once in the middle of its history as a backfill does
// and all of them after its last delivery as a re-map does, newest first. A
// re-registration is re-processing and carries no finding, so a reading that
// took every registration for a delivery would end the run of evidence at the
// re-map and draft nothing. The draft is still written, over the same eight
// deliveries; and neither re-processing feed of the fixture is drafted.
func TestFeedProposalReadsReprocessingAsNotADelivery(t *testing.T) {
	pool := freshDB(t, "shadow_feed_proposals_reprocessing", migratedTables, true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := feedfixture.InstallReprocessing(ctx, pool); err != nil {
		t.Fatal(err)
	}

	feed, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ds := feed.Deliveries
	reregistered := []string{reregister(t, pool, feed.TableName, ds[3].SessionID, ds[3].Registered.Add(2*time.Second))}
	remapAt := ds[len(ds)-1].Registered.Add(48 * time.Hour)
	if !remapAt.Before(feedfixture.Clock) {
		t.Fatalf("the re-map at %s is not before the fixture's clock", remapAt)
	}
	for i := range ds {
		reregistered = append(reregistered, reregister(t, pool, feed.TableName, ds[len(ds)-1-i].SessionID,
			remapAt.Add(time.Duration(i)*60*time.Millisecond)))
	}

	detectFeedsOf(t, pool, append(feedfixture.Feeds(), feedfixture.ReprocessingFeeds()...))

	// No finding hangs on a re-registration, and the feed's latest
	// registration is one: under a registration reading the run would be 0.
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.anomaly WHERE anomaly_session_id = ANY($1)`,
		reregistered); n != 0 {
		t.Errorf("%d anomalies on re-registrations", n)
	}
	var latest string
	if err := pool.QueryRow(ctx, `SELECT session_id FROM jetsapi.input_registry WHERE table_name = $1
		ORDER BY length(session_id) DESC, session_id DESC LIMIT 1`, feed.TableName).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(reregistered, latest) {
		t.Fatalf("the feed's latest registration %s is not a re-registration", latest)
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
	res, err := w.DraftFeedProposals(ctx, pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range res.Assessed {
		t.Logf("%-34s deliveries %2d  re-registrations %2d  run of evidence %d  qualifies %-5v %s",
			a.TableName, len(a.Deliveries), a.Reregistrations, len(a.Streak), a.Qualifies, a.Reason)
		if a.Qualifies && a.TableName != feed.TableName {
			t.Errorf("feed %s qualifies", a.TableName)
		}
	}
	if len(res.Drafts) != 1 || res.Written() != 1 || res.Drafts[0].Assessment.TableName != feed.TableName {
		t.Fatalf("%d drafts, %d written; want one, for %s", len(res.Drafts), res.Written(), feed.TableName)
	}
	a := res.Drafts[0].Assessment
	if len(a.Deliveries) != len(ds) || a.Reregistrations != len(reregistered) || len(a.Streak) != len(ds)-1 {
		t.Errorf("read %d deliveries, %d re-registrations and a run of %d; want %d, %d and %d",
			len(a.Deliveries), a.Reregistrations, len(a.Streak), len(ds), len(reregistered), len(ds)-1)
	}
	if !strings.Contains(res.Drafts[0].Proposal.Rationale, "are re-processing and are not counted") {
		t.Errorf("the rationale does not say the re-registrations were not counted:\n%s",
			res.Drafts[0].Proposal.Rationale)
	}
	if a, err := Attest(ctx, pool); err != nil || !a.Holds() || a.IncidentProposalsInDraft != 1 {
		t.Errorf("the attestation: %v\n%s", err, a.Report())
	}
}
