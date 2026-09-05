package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Who moved an incident, and from what to what (task AB.2, I-276) — the
// labelling instrument plan §10.7 argues for, built now so that Phase 5 can
// accumulate a population it cannot accumulate retrospectively.
//
// **This is RecordApproval a second time and the differences are the
// interesting part.** K.1's function writes the typed row, appends the chain
// event and moves the subject's state in one transaction, and this one does the
// same three things. What differs:
//
//   - **The prior classification is read, not accepted.** RecordApproval takes
//     `FromState` from its caller and uses it as the UPDATE's guard. Here the
//     row is read `FOR UPDATE` first, so the function can return what the
//     incident was classified as *before* the transition — which the guarded
//     UPDATE cannot, and which is the half of a corrected label that would
//     otherwise be lost the next time somebody reclassifies. The lock makes the
//     read and the write atomic, so the guard is no weaker; what it buys is a
//     conflict error that names the status actually found.
//
//   - **The lifecycle is enforced here rather than in a handler.** For
//     ChangeProposal the policy arrived two tasks after the primitive, and by
//     then `jets/apiserver/api_agentic.go` existed to hold it — K.1
//     deliberately left it out on the argument that a record of decisions
//     should not encode a workflow it was not asked about. **For Incident the
//     policy and the primitive land together and there is no handler at all**,
//     so leaving the edge unchecked would mean nothing enforces A.5's "an agent
//     may propose a transition; it cannot effect an illegal one". A handler may
//     still pre-check with IncidentTransitionAllowed to tell a bad request
//     apart from a lost race; it will not be enforcing something this function
//     leaves open. Recorded as **I-299**.
//
//   - **The chain event is conditional, and AB.4 changed what it is conditional
//     on.** `agent_audit.run_id` is an AgentRun's key. A ChangeProposal knows
//     its run — `trigger_ref` — and an Incident did not, so the chain event was
//     appended only when the *caller* named a run: an agent's own
//     reclassification was tamper-evident and a person's correction of it was
//     not, which inverts the property a governance record is for (**R-34**,
//     **I-297**). **Q-32 was settled by the user on 2026-09-04 in favour of
//     giving `Incident` a run reference**, and this function now reads
//     `incident_run_ref` off the row and uses it when the caller supplies none.
//     What remains conditional is the *incident* naming a run rather than the
//     transition — so an incident nothing agentic raised still chains nothing,
//     for every actor alike rather than for humans alone (**R-44**).
//
// **The event type is `decision` rather than `approval`.** An approval is a
// verdict on a proposed *action*; a reclassification is a verdict on a *claim*,
// and nothing is authorised by it. `approval` is also what ApprovalsFor and the
// supervision screens read the chain for, and widening its meaning would make
// that query answer a second question quietly.

// Actor kinds, mirroring the model's ActorKind and the
// incident_event_actor_kind_ck CHECK. Two members: what a label-bearing metric
// partitions on is whether the transition came from outside the system under
// test, and a deterministic classifier is not outside it.
const (
	ActorHuman = "human"
	ActorAgent = "agent"
)

// ActorKinds is the vocabulary, for a caller that needs to validate one.
var ActorKinds = []string{ActorHuman, ActorAgent}

// KnownActorKind reports whether a string is one of the two.
func KnownActorKind(kind string) bool {
	for _, k := range ActorKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// IncidentTransition is one movement of one incident.
//
// ClassificationBefore is an **output**: RecordIncidentTransition reads it off
// the row and writes it back into the struct, and a value supplied by the
// caller is overwritten. That is deliberate — the party being measured does not
// get to state what it is being measured against.
type IncidentTransition struct {
	// IncidentEventId is the event's own identity.
	IncidentEventId string
	// IncidentRef is the incident being moved.
	IncidentRef string
	// FromStatus is the status the incident is expected to be in. The
	// transition is refused if it is not.
	FromStatus string
	// ToStatus is the status it moves to. Must be an A.5 edge from FromStatus.
	ToStatus string
	// Actor is a user_email or an agent identity.
	Actor string
	// ActorKind is ActorHuman or ActorAgent — the column I-276 asks for.
	ActorKind string
	// RunRef is the AgentRun this transition belongs to, or "" to take the
	// incident's own `incident_run_ref` (AB.4). It is therefore an input *and*
	// an output: RecordIncidentTransition writes back whichever run the chain
	// event was appended to, and "" on return means the incident named none
	// either. The chain event is appended iff it is non-empty afterwards.
	RunRef string
	// ClassificationBefore is filled in by RecordIncidentTransition from the
	// row; "" means the incident carried none, which is the ordinary case out
	// of a deterministic triage step.
	ClassificationBefore string
	// ClassificationAfter is the classification this transition sets, or "" to
	// leave the incident's classification alone. Required for a
	// reclassification, by CHECK.
	ClassificationAfter string
	// TransitionedAt is when. Zero means now, resolved here rather than in SQL
	// so the same value reaches the table, the chain and the payload.
	TransitionedAt time.Time
	// Rationale is why, in the actor's words. Required for a reclassification,
	// by the same CHECK.
	Rationale string
}

func (t *IncidentTransition) validate() error {
	switch {
	case t == nil:
		return errors.New("no incident transition to record")
	case t.IncidentEventId == "":
		return errors.New("an incident transition must carry an event id")
	case t.IncidentRef == "":
		return fmt.Errorf("transition %s names no incident", t.IncidentEventId)
	case t.FromStatus == "":
		return fmt.Errorf("transition %s names no from_status, so it cannot be guarded against a concurrent decision", t.IncidentEventId)
	case t.ToStatus == "":
		return fmt.Errorf("transition %s names no to_status", t.IncidentEventId)
	case !KnownIncidentStatus(t.FromStatus):
		return fmt.Errorf("transition %s names from_status %q, which is not an IncidentStatus", t.IncidentEventId, t.FromStatus)
	case !KnownIncidentStatus(t.ToStatus):
		return fmt.Errorf("transition %s names to_status %q, which is not an IncidentStatus", t.IncidentEventId, t.ToStatus)
	case !IncidentTransitionAllowed(t.FromStatus, t.ToStatus):
		return fmt.Errorf("incident %s is in %s, from which %q is not a permitted transition; permitted: %v (Appendix A.5)",
			t.IncidentRef, t.FromStatus, t.ToStatus, IncidentTransitions(t.FromStatus))
	}
	// The attribution rule is shared with RecordTierTransition rather than
	// spelled twice (task AJ.2). It was this function's, moved into
	// state_change.go unchanged — including its messages, which two tests match
	// on — when a second act of the same shape arrived and needed exactly it.
	return validateAttribution(fmt.Sprintf("transition %s", t.IncidentEventId), t.Actor, t.ActorKind)
}

// `ErrNoIncident` is `incident.go`'s, landed hours earlier by `AE.1` for the
// read path, and it is reused here rather than shadowed: *no such incident* is
// one answer whether the caller was reading or writing, and two error types
// meaning it would make `errors.As` depend on which function was called.

// ErrIncidentStateConflict is returned when the incident is not in the expected
// from_status. Like ErrStateConflict it is distinct because it is the one a
// caller can act on — re-read the incident and show the transition that got
// there first — and it names what was actually found, which the approval
// equivalent cannot.
type ErrIncidentStateConflict struct {
	IncidentRef string
	Expected    string
	Found       string
}

func (e *ErrIncidentStateConflict) Error() string {
	return fmt.Sprintf("incident %s is in %q, not %q; another transition reached it first",
		e.IncidentRef, e.Found, e.Expected)
}

// RecordIncidentTransition moves the incident, writes the typed event and — when
// a run is in hand — appends the chain event, in one transaction. It returns the
// seq the chain trigger assigned, or **0 when no chain event was appended**,
// which is the case documented above and not an error.
//
// It fills in t.ClassificationBefore, t.TransitionedAt and, when the caller left
// it empty, t.RunRef.
func RecordIncidentTransition(ctx context.Context, db Beginner, t *IncidentTransition) (int, error) {
	if err := t.validate(); err != nil {
		return 0, err
	}
	if t.TransitionedAt.IsZero() {
		t.TransitionedAt = time.Now().UTC()
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("while opening the transaction for transition %s: %w", t.IncidentEventId, err)
	}
	defer tx.Rollback(ctx)

	// Read and lock the row before deciding anything. This is the approval
	// handler's "checked against the row, not trusted as it" rule moved down
	// into the write, so every caller gets it rather than every caller
	// remembering it — and it is the only way to learn the classification the
	// transition is about to replace.
	var status, before, incidentRun string
	err = tx.QueryRow(ctx,
		`SELECT status, coalesce(classification, ''), coalesce(incident_run_ref, '')
		   FROM jetsapi.incident
		  WHERE incident_id = $1
		    FOR UPDATE`, t.IncidentRef).Scan(&status, &before, &incidentRun)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, &ErrNoIncident{IncidentId: t.IncidentRef}
	}
	if err != nil {
		return 0, fmt.Errorf("while reading incident %s: %w", t.IncidentRef, err)
	}
	if status != t.FromStatus {
		return 0, &ErrIncidentStateConflict{
			IncidentRef: t.IncidentRef, Expected: t.FromStatus, Found: status,
		}
	}
	t.ClassificationBefore = before

	// AB.4, Q-32. The caller's run wins where it has one — an agent
	// transitioning an incident inside its own run belongs in *its* transcript,
	// not in the transcript of whatever raised the incident. Where it has none,
	// the incident's run is used, which is the whole of what the new column
	// buys: a person correcting a classification at a screen has no run of their
	// own and now chains onto the run that raised the thing they are correcting.
	//
	// **Read from the row rather than accepted as a second caller field**, on
	// ClassificationBefore's argument one line up: the party being measured does
	// not get to state what its verdict is attached to.
	if t.RunRef == "" {
		t.RunRef = incidentRun
	}

	if _, err := tx.Exec(ctx,
		`UPDATE jetsapi.incident
		    SET status = $2,
		        classification = CASE WHEN $3::text = '' THEN classification ELSE $3::text END
		  WHERE incident_id = $1`,
		t.IncidentRef, t.ToStatus, t.ClassificationAfter); err != nil {
		return 0, fmt.Errorf("while moving incident %s to %s: %w", t.IncidentRef, t.ToStatus, err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO jetsapi.incident_event (
		   incident_event_id, event_incident_ref, from_status, to_status,
		   event_actor, event_actor_kind, transitioned_at, event_run_ref,
		   classification_before, classification_after, transition_rationale)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''),
		         NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''))`,
		t.IncidentEventId, t.IncidentRef, t.FromStatus, t.ToStatus,
		t.Actor, t.ActorKind, t.TransitionedAt, t.RunRef,
		t.ClassificationBefore, t.ClassificationAfter, t.Rationale); err != nil {
		return 0, fmt.Errorf("while recording transition %s: %w", t.IncidentEventId, err)
	}

	var seq int
	if t.RunRef != "" {
		payload, err := json.Marshal(map[string]any{
			"incident_event_id":     t.IncidentEventId,
			"incident_ref":          t.IncidentRef,
			"from_status":           t.FromStatus,
			"to_status":             t.ToStatus,
			"actor_kind":            t.ActorKind,
			"classification_before": t.ClassificationBefore,
			"classification_after":  t.ClassificationAfter,
			"transitioned_at":       t.TransitionedAt.UTC().Format(time.RFC3339Nano),
			"transition_rationale":  t.Rationale,
		})
		if err != nil {
			return 0, fmt.Errorf("while encoding transition %s for the chain: %w", t.IncidentEventId, err)
		}
		seq, err = Append(ctx, tx, &Event{
			RunId:     t.RunRef,
			EventType: EventDecision,
			Actor:     t.Actor,
			Payload:   payload,
		})
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("while committing transition %s: %w", t.IncidentEventId, err)
	}
	return seq, nil
}

const incidentEventColumns = `incident_event_id, event_incident_ref, from_status, to_status,
	        event_actor, event_actor_kind, transitioned_at, coalesce(event_run_ref, ''),
	        coalesce(classification_before, ''), coalesce(classification_after, ''),
	        coalesce(transition_rationale, '')`

func scanIncidentTransitions(rows pgx.Rows, what string) ([]IncidentTransition, error) {
	defer rows.Close()
	var out []IncidentTransition
	for rows.Next() {
		var t IncidentTransition
		if err := rows.Scan(&t.IncidentEventId, &t.IncidentRef, &t.FromStatus, &t.ToStatus,
			&t.Actor, &t.ActorKind, &t.TransitionedAt, &t.RunRef,
			&t.ClassificationBefore, &t.ClassificationAfter, &t.Rationale); err != nil {
			return nil, fmt.Errorf("while scanning %s: %w", what, err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// IncidentTransitionsFor returns one incident's transitions, oldest first —
// how it got where it is, which is the whole point of the table and is what a
// supervision screen renders beside its current status.
func IncidentTransitionsFor(ctx context.Context, db Querier, incidentRef string) ([]IncidentTransition, error) {
	rows, err := db.Query(ctx,
		`SELECT `+incidentEventColumns+`
		   FROM jetsapi.incident_event
		  WHERE event_incident_ref = $1
		  ORDER BY transitioned_at, incident_event_id`, incidentRef)
	if err != nil {
		return nil, fmt.Errorf("while reading transitions for %s: %w", incidentRef, err)
	}
	return scanIncidentTransitions(rows, "a transition for "+incidentRef)
}

// HumanVerdicts returns the adjudications a person made since `since`, oldest
// first — **the labelled population, as a query**.
//
// Plan §10.7's argument is that the supervision screen is the only instrument
// that produces labels and that three of its four parts were already built. This
// is the fourth read back: `reclassified`, `verified` and `suppressed_as_benign`
// are the three IncidentStatus values that are adjudications rather than
// progress, and `event_actor_kind = 'human'` is what makes a row of them a
// label rather than the system agreeing with itself.
//
// It exists now, with a partial index behind it and nothing to count, for the
// reason K.2 recorded: a write path with no read path is a store nobody can
// check. What it will return in this phase is zero rows, and plan §10.7 says
// why that is a schedule rather than a defect.
func HumanVerdicts(ctx context.Context, db Querier, since time.Time) ([]IncidentTransition, error) {
	rows, err := db.Query(ctx,
		`SELECT `+incidentEventColumns+`
		   FROM jetsapi.incident_event
		  WHERE event_actor_kind = $1
		    AND to_status = ANY ($2)
		    AND transitioned_at >= $3
		  ORDER BY transitioned_at, incident_event_id`,
		ActorHuman,
		[]string{IncidentReclassified, IncidentVerified, IncidentSuppressedAsBenign},
		since)
	if err != nil {
		return nil, fmt.Errorf("while reading human verdicts since %s: %w", since, err)
	}
	return scanIncidentTransitions(rows, "a human verdict")
}
