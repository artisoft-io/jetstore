package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Who approved what (task K.1, gap 7a) — the supervision seam of plan §7.2,
// and criterion 28's "durable, joined to the audit chain, survives restart".
//
// **The schema answered "event type or table" with both, and neither half was
// a new idea.** `agent_audit` has carried an `approval` event type since Phase
// 0 — declared, CHECK-constrained, exercised by TestEventTypes, and written by
// nothing. `ApprovalEvent` has been a modelled entity just as long, with nine
// typed properties and no table, because `jr_as_table` is a JetRules property
// and says nothing about Postgres (I-24). So K.1 wrote no new concepts; it
// connected two that had been waiting for each other.
//
// **Why both rather than the event alone.** The event is what makes a decision
// tamper-evident: hash-chained, append-only by trigger, ordered within its run.
// What it cannot do is carry structure — its columns are actor and tier, so the
// subject, the two states and the rationale would live in `payload` jsonb,
// where `from_state` is unconstrained while `change_proposal.approval_state` is
// CHECKed against the same vocabulary. **Recording a transition less strictly
// than the state it produces is an asymmetry with no argument behind it**, and
// the modelled entity already types both ends.
//
// **The three writes are one transaction, which is StartRun's argument reused.**
// The run row and its intent event ride one transaction so that "its record and
// the thing it records cannot disagree"; an approval is the same shape with
// three participants instead of two. A committed transition whose event was
// lost would be an approval nobody can attribute.
//
// **The state guard is not decoration.** The UPDATE matches on the expected
// `from_state`, so two approvers acting on one proposal cannot both transition
// it — the second finds no row and is told so, rather than silently overwriting
// a decision that is already recorded in the chain. That is I-25's lesson
// (idempotent is not concurrency-safe) applied where the cost of losing is a
// governance record rather than a schema.

// Approval is one decision on one subject.
type Approval struct {
	// ApprovalEventId is the event's own identity.
	ApprovalEventId string
	// RunRef is the run the decision belongs to — **the run that produced the
	// proposal, not the approver's session.** A proposal is decided after that
	// run has ended, so both this table and the chain grow past
	// agent_run.ended_at. Nothing seals a chain, so there is no invariant to
	// violate; the approval is part of that run's story.
	RunRef string
	// SubjectRef is the ChangeProposal (or Remediation) being decided.
	SubjectRef string
	// FromState is the state the subject is expected to be in. The transition
	// is refused if it is not.
	FromState string
	// ToState is the state it moves to.
	ToState string
	// Actor is a user_email or an agent identity.
	Actor string
	// TierAtEvent is the AutonomyTier at the time of the decision.
	TierAtEvent string
	// DecidedAt is when. Zero means now, resolved here rather than in SQL so
	// the same value reaches the table, the chain and the payload.
	DecidedAt time.Time
	// DecisionRationale is why, in the actor's words. Optional.
	DecisionRationale string
}

func (a *Approval) validate() error {
	switch {
	case a == nil:
		return fmt.Errorf("no approval to record")
	case a.ApprovalEventId == "":
		return fmt.Errorf("an approval must carry an event id")
	case a.RunRef == "":
		return fmt.Errorf("approval %s names no run; an approval outside a run's chain is not joined to anything", a.ApprovalEventId)
	case a.SubjectRef == "":
		return fmt.Errorf("approval %s names no subject; there is nothing to approve", a.ApprovalEventId)
	case a.FromState == "":
		return fmt.Errorf("approval %s names no from_state, so the transition cannot be guarded against a concurrent decision", a.ApprovalEventId)
	case a.ToState == "":
		return fmt.Errorf("approval %s names no to_state", a.ApprovalEventId)
	case a.Actor == "":
		return fmt.Errorf("approval %s names no actor; an unattributable approval is the one thing this record exists to prevent", a.ApprovalEventId)
	case a.TierAtEvent == "":
		return fmt.Errorf("approval %s carries no tier; tier_at_event is NOT NULL because a decision's authority is part of the decision", a.ApprovalEventId)
	}
	return nil
}

// ErrStateConflict is returned when the subject is not in the expected
// from_state. It is a distinct error because it is the one a caller can act on
// — re-read the proposal and show the decision that got there first — while
// every other failure here is a bug or an outage.
type ErrStateConflict struct {
	SubjectRef string
	Expected   string
}

func (e *ErrStateConflict) Error() string {
	return fmt.Sprintf("proposal %s is not in state %q; another decision reached it first",
		e.SubjectRef, e.Expected)
}

// Beginner is satisfied by *pgxpool.Pool and *pgx.Conn; RecordApproval opens
// its own transaction because all three writes must land together and none of
// them is the caller's.
//
// RecordApproval writes the typed row, appends the chain event and moves the
// proposal's state, in one transaction. It returns the seq the trigger assigned
// the audit row, so a caller can cite it.
func RecordApproval(ctx context.Context, db Beginner, a *Approval) (int, error) {
	if err := a.validate(); err != nil {
		return 0, err
	}
	if a.DecidedAt.IsZero() {
		a.DecidedAt = time.Now().UTC()
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("while opening the transaction for approval %s: %w", a.ApprovalEventId, err)
	}
	defer tx.Rollback(ctx)

	// The subject moves first, because its guard is what decides whether this
	// approval happens at all. Writing the audit event before knowing that
	// would append a decision that did not take effect.
	tag, err := tx.Exec(ctx,
		`UPDATE jetsapi.change_proposal
		    SET approval_state = $3
		  WHERE proposal_id = $1 AND approval_state = $2`,
		a.SubjectRef, a.FromState, a.ToState)
	if err != nil {
		return 0, fmt.Errorf("while moving proposal %s to %s: %w", a.SubjectRef, a.ToState, err)
	}
	if tag.RowsAffected() == 0 {
		return 0, &ErrStateConflict{SubjectRef: a.SubjectRef, Expected: a.FromState}
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO jetsapi.approval_event (
		   approval_event_id, run_ref, subject_ref, from_state, to_state,
		   actor, tier_at_event, decided_at, decision_rationale)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''))`,
		a.ApprovalEventId, a.RunRef, a.SubjectRef, a.FromState, a.ToState,
		a.Actor, a.TierAtEvent, a.DecidedAt, a.DecisionRationale); err != nil {
		return 0, fmt.Errorf("while recording approval %s: %w", a.ApprovalEventId, err)
	}

	payload, err := json.Marshal(map[string]any{
		"approval_event_id":  a.ApprovalEventId,
		"subject_ref":        a.SubjectRef,
		"from_state":         a.FromState,
		"to_state":           a.ToState,
		"decided_at":         a.DecidedAt.UTC().Format(time.RFC3339Nano),
		"decision_rationale": a.DecisionRationale,
	})
	if err != nil {
		return 0, fmt.Errorf("while encoding approval %s for the chain: %w", a.ApprovalEventId, err)
	}
	seq, err := Append(ctx, tx, &Event{
		RunId:     a.RunRef,
		EventType: EventApproval,
		Actor:     a.Actor,
		Tier:      a.TierAtEvent,
		Payload:   payload,
	})
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("while committing approval %s: %w", a.ApprovalEventId, err)
	}
	return seq, nil
}

// ApprovalsFor returns one subject's decisions, oldest first. This is the query
// the typed table exists for: over the chain alone it would be a jsonb scan of
// every run's events with no index to help.
func ApprovalsFor(ctx context.Context, db Querier, subjectRef string) ([]Approval, error) {
	rows, err := db.Query(ctx,
		`SELECT approval_event_id, run_ref, subject_ref, from_state, to_state,
		        actor, tier_at_event, decided_at, coalesce(decision_rationale, '')
		   FROM jetsapi.approval_event
		  WHERE subject_ref = $1
		  ORDER BY decided_at, approval_event_id`, subjectRef)
	if err != nil {
		return nil, fmt.Errorf("while reading approvals for %s: %w", subjectRef, err)
	}
	defer rows.Close()
	var out []Approval
	for rows.Next() {
		var a Approval
		if err := rows.Scan(&a.ApprovalEventId, &a.RunRef, &a.SubjectRef, &a.FromState,
			&a.ToState, &a.Actor, &a.TierAtEvent, &a.DecidedAt, &a.DecisionRationale); err != nil {
			return nil, fmt.Errorf("while scanning an approval for %s: %w", subjectRef, err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Querier is the read slice of pgx, satisfied by the pool, a conn and a tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}
