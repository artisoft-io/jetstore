// AB.2's half of I-276, tested against a real Postgres: an incident's
// classification transitions carry an actor, an actor *kind* and a timestamp,
// the prior classification is read from the row rather than accepted from the
// caller, and the two CHECK constraints AB.2 adds refuse what the appendix
// says they must.
//
// Needs JETS_TEST_DSN, like the rest of this package; skipped otherwise.
package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedIncident writes one incident in `status` with `classification` ("" for
// none) and returns its id. It is deliberately a raw INSERT: nothing in
// JetStore writes jetsapi.incident yet — AC.1's is the first writer — so a
// helper that pretended otherwise would be a fixture agreeing with itself.
//
// **It names no run**, which is the ordinary case out of a deterministic triage
// step and is therefore the right default for a fixture (AB.4). Tests about the
// run reference use seedIncidentRaisedBy.
func seedIncident(t *testing.T, pool *pgxpool.Pool, status, classification string) string {
	t.Helper()
	return seedIncidentRaisedBy(t, pool, status, classification, "")
}

// seedIncidentRaisedBy is seedIncident with an incident_run_ref (AB.4, Q-32).
func seedIncidentRaisedBy(t *testing.T, pool *pgxpool.Pool, status, classification, runRef string) string {
	t.Helper()
	id := fmt.Sprintf("inc_%d", time.Now().UnixNano())
	var class, run any
	if classification != "" {
		class = classification
	}
	if runRef != "" {
		run = runRef
	}
	_, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.incident (
		   incident_id, incident_session_id, incident_run_ref, incident_detected_at,
		   incident_locus, classification, severity, status, incident_confounders,
		   incident_model_version)
		 VALUES ($1, $2, $3, now(), 'worker_failed', $4, 'high', $5, ARRAY[]::text[], '0.1.0')`,
		id, "sess_"+id, run, class, status)
	if err != nil {
		t.Fatalf("seeding incident: %v", err)
	}
	return id
}

// startTestRun writes an AgentRun so that a chain event's run_id names something
// that ran. No constraint enforces it — StartRun is how one arrives.
func startTestRun(t *testing.T, pool *pgxpool.Pool, prefix string) string {
	t.Helper()
	runId := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if err := StartRun(context.Background(), pool, &Run{
		RunId: runId, AgentId: "triage", AgentVersion: "0.1.0", ModelId: "none",
		PromptVersion: "0.1.0", Tier: "T1", IterationCap: 1, WallClockCapSeconds: 60,
		DomainModelVersion: "0.1.0",
	}, []byte(`{"intent":"triage"}`)); err != nil {
		t.Fatalf("starting the run: %v", err)
	}
	return runId
}

// The whole of I-276 in one test: a human corrects a classification, and the
// record afterwards says who, when, from what and to what — none of which the
// mutable status column could say.
func TestRecordIncidentTransitionCarriesActorKindAndTimestamp(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := seedIncident(t, pool, IncidentTriaged, "transformation_defect")

	tr := &IncidentTransition{
		IncidentEventId:     "ie_" + id,
		IncidentRef:         id,
		FromStatus:          IncidentTriaged,
		ToStatus:            IncidentReclassified,
		Actor:               "michel@artisoft.io",
		ActorKind:           ActorHuman,
		ClassificationAfter: "source_content_change",
		Rationale:           "the upstream feed changed its header row",
	}
	// The caller says nothing about the prior classification, and a caller that
	// did would be overruled: this field is an output.
	if _, err := RecordIncidentTransition(ctx, pool, tr); err != nil {
		t.Fatalf("recording the transition: %v", err)
	}
	if tr.ClassificationBefore != "transformation_defect" {
		t.Errorf("classification_before was read from the row as %q, want %q",
			tr.ClassificationBefore, "transformation_defect")
	}
	if tr.TransitionedAt.IsZero() {
		t.Error("a zero TransitionedAt should have been resolved to now")
	}

	got, err := IncidentTransitionsFor(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("read back %d transitions, want 1", len(got))
	}
	g := got[0]
	if g.Actor != "michel@artisoft.io" || g.ActorKind != ActorHuman {
		t.Errorf("actor/kind read back as %q/%q", g.Actor, g.ActorKind)
	}
	if g.ClassificationBefore != "transformation_defect" || g.ClassificationAfter != "source_content_change" {
		t.Errorf("the corrected label read back as %q -> %q", g.ClassificationBefore, g.ClassificationAfter)
	}
	if g.FromStatus != IncidentTriaged || g.ToStatus != IncidentReclassified {
		t.Errorf("statuses read back as %s -> %s", g.FromStatus, g.ToStatus)
	}
	if g.TransitionedAt.IsZero() {
		t.Error("transitioned_at came back zero")
	}

	// And the incident itself carries the new classification, written by the
	// same transaction — so the row and the event cannot disagree.
	var status, classification string
	if err := pool.QueryRow(ctx,
		`SELECT status, coalesce(classification, '') FROM jetsapi.incident WHERE incident_id = $1`,
		id).Scan(&status, &classification); err != nil {
		t.Fatal(err)
	}
	if status != IncidentReclassified || classification != "source_content_change" {
		t.Errorf("the incident is %s/%s after the transition", status, classification)
	}
}

// The pairing §11.3.1 scores is (what the system said, what the person said).
// The incident row cannot hold it, because a second correction overwrites the
// first — which is why the classifications travel on the event.
func TestASecondCorrectionDoesNotEraseTheFirst(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := seedIncident(t, pool, IncidentTriaged, "transformation_defect")

	backdated := time.Now().UTC().Add(-time.Hour)
	steps := []struct{ from, to, after, why string }{
		{IncidentTriaged, IncidentReclassified, "source_content_change", "header row changed"},
		{IncidentReclassified, IncidentTriaged, "", ""},
		{IncidentTriaged, IncidentReclassified, "parse_failure", "it was the decoder after all"},
	}
	for i, s := range steps {
		tr := &IncidentTransition{
			IncidentEventId:     fmt.Sprintf("ie_%s_%d", id, i),
			IncidentRef:         id,
			FromStatus:          s.from,
			ToStatus:            s.to,
			Actor:               "michel@artisoft.io",
			ActorKind:           ActorHuman,
			ClassificationAfter: s.after,
			Rationale:           s.why,
			// Ordered explicitly: two transitions inside one test can land in
			// the same microsecond, and `ORDER BY transitioned_at` would then
			// fall back to the event id, which is not the property under test.
			// **Backdated rather than spread forward**, which cost a debugging
			// pass: these are human reclassifications, so timestamps in the
			// future land inside the window a later test opens for
			// HumanVerdicts and are counted as its labels. A test that writes
			// into another test's future is a test that shares state with it.
			TransitionedAt: backdated.Add(time.Duration(i) * time.Second),
		}
		if _, err := RecordIncidentTransition(ctx, pool, tr); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	got, err := IncidentTransitionsFor(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("read back %d transitions, want 3", len(got))
	}
	if got[0].ClassificationBefore != "transformation_defect" || got[0].ClassificationAfter != "source_content_change" {
		t.Errorf("first correction: %q -> %q", got[0].ClassificationBefore, got[0].ClassificationAfter)
	}
	if got[2].ClassificationBefore != "source_content_change" || got[2].ClassificationAfter != "parse_failure" {
		t.Errorf("second correction: %q -> %q", got[2].ClassificationBefore, got[2].ClassificationAfter)
	}
}

// RecordApproval's guard, restated: two actors cannot both move one incident.
// The difference is that this one can say what it found.
func TestIncidentTransitionGuardsTheFromStatus(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := seedIncident(t, pool, IncidentTriaged, "")

	first := &IncidentTransition{
		IncidentEventId: "ie_a_" + id, IncidentRef: id,
		FromStatus: IncidentTriaged, ToStatus: IncidentSuppressedAsBenign,
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
	}
	if _, err := RecordIncidentTransition(ctx, pool, first); err != nil {
		t.Fatal(err)
	}

	second := &IncidentTransition{
		IncidentEventId: "ie_b_" + id, IncidentRef: id,
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "triage@jetstore", ActorKind: ActorAgent,
	}
	_, err := RecordIncidentTransition(ctx, pool, second)
	conflict, ok := err.(*ErrIncidentStateConflict)
	if !ok {
		t.Fatalf("a stale from_status must be a state conflict; got %v", err)
	}
	if conflict.Found != IncidentSuppressedAsBenign {
		t.Errorf("the conflict names %q as the status found, want %q",
			conflict.Found, IncidentSuppressedAsBenign)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.incident_event WHERE event_incident_ref = $1`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the refused transition left %d rows, want 1", count-0)
	}
}

func TestUnknownIncidentIsNotAConflict(t *testing.T) {
	pool := testPool(t)
	_, err := RecordIncidentTransition(context.Background(), pool, &IncidentTransition{
		IncidentEventId: "ie_missing", IncidentRef: "inc_does_not_exist",
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "triage@jetstore", ActorKind: ActorAgent,
	})
	if _, ok := err.(*ErrNoIncident); !ok {
		t.Fatalf("a missing incident must be its own error, not a conflict; got %v", err)
	}
}

// The chain event is conditional, and **AB.4 changed what it is conditional
// on** — this test's name says "the transition names a run" and that is now the
// *first* of two ways one is found. It is kept under its original name because
// the branch it exercises is unchanged: a caller that names a run gets a chain
// event, and one on an incident that names neither gets none. What AB.4 added is
// the middle case, which is TestATransitionInheritsTheIncidentsRun below.
//
// A test that only ever passed a run would leave the *other* branch — the one
// every human transition takes — unexercised, which is why R-34's residue is
// still asserted rather than assumed gone.
func TestTheChainEventIsAppendedOnlyWhenTheTransitionNamesARun(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	withoutRun := seedIncident(t, pool, IncidentTriaged, "")
	seq, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_norun_" + withoutRun, IncidentRef: withoutRun,
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq != 0 {
		t.Errorf("a transition with no run reported chain seq %d; 0 means no chain event", seq)
	}

	runId := startTestRun(t, pool, "run_ie")
	withRun := seedIncident(t, pool, IncidentTriaged, "")
	seq, err = RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_run_" + withRun, IncidentRef: withRun,
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "triage@jetstore", ActorKind: ActorAgent, RunRef: runId,
	})
	if err != nil {
		t.Fatal(err)
	}
	if seq == 0 {
		t.Fatal("a transition naming a run must append a chain event")
	}
	var eventType, actor string
	if err := pool.QueryRow(ctx,
		`SELECT event_type, actor FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
		runId, seq).Scan(&eventType, &actor); err != nil {
		t.Fatal(err)
	}
	// `decision`, not `approval`: an approval is a verdict on a proposed
	// action, and nothing is authorised by a reclassification.
	if eventType != EventDecision {
		t.Errorf("the chain event is %q, want %q", eventType, EventDecision)
	}
	if actor != "triage@jetstore" {
		t.Errorf("the chain event's actor is %q", actor)
	}
}

// A.5: "reclassified returns to triaged with a new classification and a
// recorded reason." The CHECK is where that sentence is enforced, so both
// halves of it are exercised here rather than trusted.
func TestReclassifiedRequiresAClassificationAndAReason(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		after string
		why   string
	}{
		{"no classification", "", "because"},
		{"no reason", "parse_failure", ""},
		{"neither", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := seedIncident(t, pool, IncidentTriaged, "")
			_, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
				IncidentEventId: "ie_rc_" + id, IncidentRef: id,
				FromStatus: IncidentTriaged, ToStatus: IncidentReclassified,
				Actor: "michel@artisoft.io", ActorKind: ActorHuman,
				ClassificationAfter: tc.after, Rationale: tc.why,
			})
			if err == nil || !strings.Contains(err.Error(), "incident_event_reclassified_ck") {
				t.Fatalf("want the reclassified CHECK to refuse it; got %v", err)
			}
			// The transaction rolled back, so the incident did not move either.
			var status string
			if err := pool.QueryRow(ctx,
				`SELECT status FROM jetsapi.incident WHERE incident_id = $1`, id).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if status != IncidentTriaged {
				t.Errorf("the refused transition still moved the incident to %s", status)
			}
		})
	}
}

// The vocabularies, asserted against the generated CHECKs the way TestEventTypes
// asserts the event taxonomy: eleven statuses and two actor kinds are accepted,
// and a plausible twelfth and third are not.
func TestIncidentEventVocabulariesMatchTheGeneratedChecks(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	insert := func(id, from, to, kind string) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO jetsapi.incident_event (
			   incident_event_id, event_incident_ref, from_status, to_status,
			   event_actor, event_actor_kind, transitioned_at, classification_after,
			   transition_rationale)
			 VALUES ($1, 'inc_vocab', $2, $3, 'test', $4, now(), 'parse_failure', 'vocabulary check')`,
			id, from, to, kind)
		return err
	}
	stamp := time.Now().UnixNano()
	for i, s := range IncidentStatuses {
		if err := insert(fmt.Sprintf("ie_v_%d_%d", stamp, i), s, s, ActorAgent); err != nil {
			t.Errorf("%s is an IncidentStatus and the CHECK refused it: %v", s, err)
		}
	}
	for i, k := range ActorKinds {
		if err := insert(fmt.Sprintf("ie_k_%d_%d", stamp, i), IncidentTriaged, IncidentDiagnosed, k); err != nil {
			t.Errorf("%s is an ActorKind and the CHECK refused it: %v", k, err)
		}
	}
	err := insert(fmt.Sprintf("ie_bad_status_%d", stamp), "escalated", IncidentDiagnosed, ActorAgent)
	if err == nil || !strings.Contains(err.Error(), "incident_event_from_status_ck") {
		// `escalated` is a state A.5's *Remediation* machine draws and no
		// vocabulary here carries — F252's divergence, arriving as a value.
		t.Errorf("an out-of-vocabulary status must fail the CHECK; got %v", err)
	}
	err = insert(fmt.Sprintf("ie_bad_kind_%d", stamp), IncidentTriaged, IncidentDiagnosed, "detector")
	if err == nil || !strings.Contains(err.Error(), "incident_event_actor_kind_ck") {
		t.Errorf("an out-of-vocabulary actor kind must fail the CHECK; got %v", err)
	}
}

// An unattributable transition is refused before it reaches the database, and
// the message says why rather than naming a column.
func TestATransitionWithoutAnActorKindIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, kind string }{
		{"empty", ""},
		{"invented", "detector"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (&IncidentTransition{
				IncidentEventId: "ie_x", IncidentRef: "inc_x",
				FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
				Actor: "someone", ActorKind: tc.kind,
			}).validate()
			if err == nil || !strings.Contains(err.Error(), "actor kind") {
				t.Fatalf("want a refusal naming the actor kind; got %v", err)
			}
		})
	}
}

// An illegal edge is refused by the primitive, not only by a handler — because
// for Incident there is no handler, and A.5's "an agent may propose a
// transition; it cannot effect an illegal one" would otherwise be enforced by
// nothing (I-299).
func TestAnIllegalEdgeIsRefusedByThePrimitive(t *testing.T) {
	err := (&IncidentTransition{
		IncidentEventId: "ie_y", IncidentRef: "inc_y",
		FromStatus: IncidentDetected, ToStatus: IncidentResolved,
		Actor: "someone", ActorKind: ActorAgent,
	}).validate()
	if err == nil || !strings.Contains(err.Error(), "not a permitted transition") {
		t.Fatalf("A.5's own illegal example must be refused; got %v", err)
	}
}

// §A.2.9: "reversibility: irreversible is rejected at schema level. No
// irreversible remediation may be persisted at any tier (Section 4.3)." The
// vocabulary carries the value; the table refuses it.
func TestRemediationRefusesAnIrreversibleAction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	insert := func(id, reversibility string) error {
		_, err := pool.Exec(ctx,
			`INSERT INTO jetsapi.remediation (
			   remediation_id, incident_ref, autonomy_tier_required, reversibility,
			   remediation_approval_state)
			 VALUES ($1, 'inc_rem', 'T1', $2, 'draft')`, id, reversibility)
		return err
	}
	stamp := time.Now().UnixNano()
	for i, r := range []string{"fully_reversible", "reversible_with_backfill"} {
		if err := insert(fmt.Sprintf("rem_%d_%d", stamp, i), r); err != nil {
			t.Errorf("%s is persistable and the CHECK refused it: %v", r, err)
		}
	}
	err := insert(fmt.Sprintf("rem_irr_%d", stamp), "irreversible")
	if err == nil || !strings.Contains(err.Error(), "remediation_reversibility_ck") {
		t.Fatalf("A.2.9's validator rule must refuse an irreversible remediation; got %v", err)
	}
	// And the tier CHECK, since autonomy_tier_required having a home is half of
	// what this table is for.
	err = insert(fmt.Sprintf("rem_tier_%d", stamp), "fully_reversible")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO jetsapi.remediation (
		   remediation_id, incident_ref, autonomy_tier_required, reversibility,
		   remediation_approval_state)
		 VALUES ($1, 'inc_rem', 'T9', 'fully_reversible', 'draft')`,
		fmt.Sprintf("rem_t9_%d", stamp))
	if err == nil || !strings.Contains(err.Error(), "remediation_tier_ck") {
		t.Errorf("T9 is not an AutonomyTier and the CHECK must refuse it; got %v", err)
	}
}

// The labelled population as a query. It returns nothing in this phase and the
// test says so: what is being asserted is that the instrument reads back what
// it wrote, not that anyone has used it.
func TestHumanVerdictsReadsAdjudicationsAndNotProgress(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	since := time.Now().UTC()

	// An agent moving an incident forward is progress, not a verdict.
	agentSide := seedIncident(t, pool, IncidentTriaged, "")
	if _, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_hv_agent_" + agentSide, IncidentRef: agentSide,
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "triage@jetstore", ActorKind: ActorAgent,
	}); err != nil {
		t.Fatal(err)
	}
	// An agent reclassifying its own incident is the case I-276 exists for: an
	// adjudication the measured system made, which must not be counted.
	selfCorrection := seedIncident(t, pool, IncidentTriaged, "parse_failure")
	if _, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_hv_self_" + selfCorrection, IncidentRef: selfCorrection,
		FromStatus: IncidentTriaged, ToStatus: IncidentReclassified,
		Actor: "triage@jetstore", ActorKind: ActorAgent,
		ClassificationAfter: "benign_variation", Rationale: "second pass",
	}); err != nil {
		t.Fatal(err)
	}
	// A person suppressing one is.
	humanSide := seedIncident(t, pool, IncidentTriaged, "")
	if _, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_hv_human_" + humanSide, IncidentRef: humanSide,
		FromStatus: IncidentTriaged, ToStatus: IncidentSuppressedAsBenign,
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := HumanVerdicts(ctx, pool, since)
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, v := range got {
		seen = append(seen, v.IncidentRef)
	}
	if len(seen) != 1 || seen[0] != humanSide {
		t.Fatalf("human verdicts since the test began: %v, want exactly [%s]", seen, humanSide)
	}
}

// AB.4, Q-32 — the case the change exists for.
//
// **A person at a supervision screen has no run of their own**, so before this
// their correction of a classification was written to jetsapi.incident_event and
// nowhere else: durable, attributable, and not tamper-evident, while the agent's
// own reclassification of the same incident was chained. That inversion is R-34
// and it is what this asserts is gone — the human transition names no run, and
// the chain event is appended to the run that raised the incident.
func TestATransitionInheritsTheIncidentsRun(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTestRun(t, pool, "run_ab4")
	id := seedIncidentRaisedBy(t, pool, IncidentTriaged, "transformation_defect", runId)

	tr := &IncidentTransition{
		IncidentEventId: "ie_inherit_" + id, IncidentRef: id,
		FromStatus: IncidentTriaged, ToStatus: IncidentReclassified,
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
		ClassificationAfter: "source_content_change",
		Rationale:           "the upstream feed changed its header row",
		// RunRef deliberately unset: this is the shape a screen writes.
	}
	seq, err := RecordIncidentTransition(ctx, pool, tr)
	if err != nil {
		t.Fatalf("recording the transition: %v", err)
	}
	if seq == 0 {
		t.Fatal("a human transition on an incident that names a run must reach the chain")
	}
	// The struct is filled in, so a caller can report which run its verdict was
	// chained to without re-reading the incident.
	if tr.RunRef != runId {
		t.Errorf("RunRef was filled in as %q, want %q", tr.RunRef, runId)
	}

	got, err := IncidentTransitionsFor(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RunRef != runId {
		t.Fatalf("the stored event names run %q, want %q", got[0].RunRef, runId)
	}
	// And the chain row is the one the incident's run carries, with the human as
	// its actor — which is the whole of what makes the correction tamper-evident.
	var actor, eventType string
	if err := pool.QueryRow(ctx,
		`SELECT actor, event_type FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
		runId, seq).Scan(&actor, &eventType); err != nil {
		t.Fatal(err)
	}
	if actor != "michel@artisoft.io" || eventType != EventDecision {
		t.Errorf("the chain event is %s by %q", eventType, actor)
	}
}

// The caller's run wins where it has one, and the reason is not precedence for
// its own sake: an agent transitioning an incident inside its own run belongs in
// *its* transcript, not in the transcript of whatever raised the incident.
func TestACallersRunWinsOverTheIncidentsRun(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	raiser := startTestRun(t, pool, "run_raiser")
	mover := startTestRun(t, pool, "run_mover")
	id := seedIncidentRaisedBy(t, pool, IncidentTriaged, "", raiser)

	seq, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
		IncidentEventId: "ie_own_" + id, IncidentRef: id,
		FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
		Actor: "rca@jetstore", ActorKind: ActorAgent, RunRef: mover,
	})
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
		mover, seq).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("the chain event did not land on the caller's run")
	}
	got, err := IncidentTransitionsFor(ctx, pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].RunRef != mover {
		t.Errorf("the stored event names %q, want the caller's run %q", got[0].RunRef, mover)
	}
}

// **R-44: the residue, asserted rather than assumed.** An incident nothing
// agentic raised chains nothing, and that is now true for every actor alike
// rather than for humans alone — which is the property a governance record
// needs and is a weaker claim than "every transition is tamper-evident". A test
// that only covered the two cases above would let this read as closed.
func TestAnIncidentWithNoRunChainsNothingForEitherActorKind(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, kind := range ActorKinds {
		id := seedIncident(t, pool, IncidentTriaged, "")
		seq, err := RecordIncidentTransition(ctx, pool, &IncidentTransition{
			IncidentEventId: fmt.Sprintf("ie_norun_%s_%s", kind, id), IncidentRef: id,
			FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
			Actor: "somebody@example.com", ActorKind: kind,
		})
		if err != nil {
			t.Fatal(err)
		}
		if seq != 0 {
			t.Errorf("actor kind %s: chain seq %d on an incident with no run", kind, seq)
		}
	}
}
