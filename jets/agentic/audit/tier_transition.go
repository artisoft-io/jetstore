package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// The autonomy tier transition (task AJ.2, gap 7b, criterion 51) — what raises
// a run's tier, who may, and what the audit record shows.
//
// `AJ.1` built the comparison: a tool declares a tier, the run has one, and
// `TierGate` refuses below it. **A comparison whose losing operand can never
// change is a ceiling, not a gate** — so this is the other half: the recorded,
// attributable act that moves `jetsapi.agent_run.tier`.
//
// # It is RecordIncidentTransition a second time, and state_change.go says so
//
// Read-and-lock the subject, guard it against the state the caller expected,
// refuse a move the policy does not permit, require an actor and an actor kind,
// write the new state, append the chain event, one transaction. That is the
// same seven steps, and both writers satisfy `StateChange` so the claim is a
// compile error rather than a paragraph.
//
// **The one structural difference is in this act's favour.** An incident's
// chain event is conditional — `agent_audit.run_id` is an AgentRun's key
// (P4 F254) and an incident may name no run, so a person's correction of an
// incident nothing agentic raised is durable, attributable and **not**
// tamper-evident (R-44). A tier transition's subject *is* a run, so the chain
// event is unconditional and there is no such case. The act this project was
// least able to make tamper-evident and the act it was most able to are the same
// shape, and the difference is a property of the subject rather than of either
// writer.
//
// # Who may: an agent may lower its own authority and may not raise it
//
// **A gate the gated party can open is not a gate.** Every other rule here is
// mechanical; this one is the security property the whole tier concept rests
// on, and it is enforced in the primitive rather than left to a handler on
// RecordIncidentTransition's precedent — for Incident "the policy and the
// primitive land together and there is no handler at all", and that is true
// here too.
//
// It is asymmetric on purpose. A **raise** grants authority the run did not
// have, so it requires `ActorHuman` and a rationale: A§4 defines the scale by
// how much human involvement an action needs, and A.5's preamble says an agent
// "may propose a transition; it cannot effect an illegal one". A **lowering** is
// self-limitation — an agent that concludes it should not be acting at T3 may
// write that down, and refusing it would mean the only way to reduce authority
// is to ask a person for permission to do less.
//
// # What is deliberately not invented here
//
// **No per-step graph.** `lifecycle.go` and `incident_transition_lifecycle.go`
// both carry a transition table because Appendix A.5 draws one. A.5 draws no
// machine over AutonomyTier, and the vocabulary is an ordered scale rather than
// a graph — `Reaches` is the whole of its structure. So T1 -> T4 in one act is
// permitted, and whether it should be is asked as **Q-71** rather than answered
// by this file. Inventing a one-step-at-a-time rule here would be gap 2b's
// unreviewed extraction with an audit trail attached.
//
// **No ceiling.** Nothing in the model says what a given agent, identity or
// deployment may be raised *to*; `guard.go` already records that which identity
// runs at which tier is a deployment rule the code cannot enforce, and Q-57 is
// where that is asked. A ceiling invented here would be a policy with no
// author.
//
// **No executor, and nothing acts.** This raises an authority; it does not use
// one. `Remediation` remains tabled with no executor (criterion 47), and a
// transition is an authority change rather than an execution.
//
// # No endpoint, and that is K.2's argument rather than an omission
//
// `api_agentic.go` opens by recording why K.2 built `audit.ReadTranscript` and
// registered no route: "a route registered before its consumer exists fixes its
// shape by guess". There is no run supervision screen, and — F505 — nothing
// outside the `agent` package constructs a gated `Loop` at all, so an HTTP
// action raising a tier would be authority over nothing. The verdict half of
// this task *does* get an endpoint, because its consumer is the screen the same
// task ships. That asymmetry is the argument being applied, not an inconsistency.

// TierTransition is one change of the authority a run is operating under.
//
// TierBefore is an **output**: RecordTierTransition reads it off the locked row
// and writes it back, on ClassificationBefore's argument one file over — the
// party being measured does not get to state what it is being measured against.
type TierTransition struct {
	// RunId is the run whose authority moves. It is also the chain key, which
	// is why this act is always tamper-evident.
	RunId string
	// FromTier is the tier the caller expects the run to be at. The transition
	// is refused if it is not, so a decision taken against a stale view of the
	// run is a conflict rather than a silent overwrite.
	FromTier string
	// ToTier is the authority the run moves to.
	ToTier string
	// Actor is a user_email or an agent identity.
	Actor string
	// ActorKind is ActorHuman or ActorAgent. A raise requires ActorHuman.
	ActorKind string
	// Rationale is why, in the actor's words. Required for a raise and optional
	// for a lowering: a grant of authority with no stated reason is the record
	// a supervisor cannot audit, and a renunciation of it is not.
	Rationale string
	// TransitionedAt is when. Zero means now, resolved here rather than in SQL
	// so that the same value reaches the chain event and its payload.
	TransitionedAt time.Time
	// TierBefore is filled in from the locked row. It equals FromTier on
	// success — the guard sees to that — and is what the record shows.
	TierBefore string
}

// Subject, From, To, Attribution and Reason put *TierTransition into
// StateChange's shape. See state_change.go.
func (t *TierTransition) Subject() string { return t.RunId }

// From is the tier the run is expected to be at.
func (t *TierTransition) From() string { return t.FromTier }

// To is the tier it moves to.
func (t *TierTransition) To() string { return t.ToTier }

// Attribution is who moved it and of which kind.
func (t *TierTransition) Attribution() (string, string) { return t.Actor, t.ActorKind }

// Reason is the stated reason for the move.
func (t *TierTransition) Reason() string { return t.Rationale }

// IsRaise reports whether this transition grants authority the run did not
// have. It is the whole of the "who may" rule's input, and it is computed from
// the vocabulary's order rather than from a flag the caller sets.
func (t *TierTransition) IsRaise() (bool, error) {
	from, err := ParseTier(t.FromTier)
	if err != nil {
		return false, err
	}
	to, err := ParseTier(t.ToTier)
	if err != nil {
		return false, err
	}
	reaches, err := Reaches(from, to)
	if err != nil {
		return false, err
	}
	// from reaches to means to is at or below from, so a raise is exactly the
	// case where it does not.
	return !reaches, nil
}

// ErrTierStateConflict is returned when the run is not at the expected tier.
// Distinct because it is the one a caller can act on — re-read the run and show
// the transition that got there first — and it names what was actually found.
type ErrTierStateConflict struct {
	RunId    string
	Expected string
	Found    string
}

func (e *ErrTierStateConflict) Error() string {
	return fmt.Sprintf("run %s is at autonomy tier %q, not %q; another transition reached it first",
		e.RunId, e.Found, e.Expected)
}

// ErrTierRaiseNotHuman is the refusal of an agent raising its own authority.
//
// It is a distinct type so a handler can answer 403 rather than 400: the
// request is well formed and the actor is not entitled to make it, which are
// different things and read differently in a governance record.
type ErrTierRaiseNotHuman struct {
	RunId     string
	From      string
	To        string
	Actor     string
	ActorKind string
}

func (e *ErrTierRaiseNotHuman) Error() string {
	return fmt.Sprintf(
		"run %s cannot be raised from %s to %s by %s (%s): only a human may grant authority, "+
			"because a gate the gated party can open is not a gate",
		e.RunId, e.From, e.To, e.Actor, e.ActorKind)
}

func (t *TierTransition) validate() error {
	if t == nil {
		return errors.New("no tier transition to record")
	}
	if t.RunId == "" {
		return errors.New("a tier transition must name the run whose authority moves")
	}
	what := fmt.Sprintf("the tier transition of run %s", t.RunId)
	if err := validateAttribution(what, t.Actor, t.ActorKind); err != nil {
		return err
	}
	// Both operands are parsed before either is compared, which is AJ.1's rule
	// at the gate applied here at the writer: an unreadable tier is reported as
	// one rather than as an authority failure. ParseTier refuses the empty
	// string too, so a caller that did not set FromTier is told it did not
	// rather than being guarded against "".
	if _, err := ParseTier(t.FromTier); err != nil {
		return fmt.Errorf("%s names a from_tier this build cannot read: %w", what, err)
	}
	if _, err := ParseTier(t.ToTier); err != nil {
		return fmt.Errorf("%s names a to_tier this build cannot read: %w", what, err)
	}
	if t.FromTier == t.ToTier {
		// Not a no-op to be tolerated: it would put a row in the chain
		// asserting that somebody took responsibility for a change that did not
		// happen, which is worse than nothing in a record whose whole value is
		// that every entry means something.
		return fmt.Errorf("%s moves it from %s to %s, which changes no authority", what, t.FromTier, t.ToTier)
	}
	raise, err := t.IsRaise()
	if err != nil {
		return err
	}
	if raise {
		if t.ActorKind != ActorHuman {
			return &ErrTierRaiseNotHuman{
				RunId: t.RunId, From: t.FromTier, To: t.ToTier,
				Actor: t.Actor, ActorKind: t.ActorKind,
			}
		}
		if t.Rationale == "" {
			return fmt.Errorf("%s raises it from %s to %s and states no reason; "+
				"a grant of authority with no recorded reason is the one a supervisor cannot audit",
				what, t.FromTier, t.ToTier)
		}
	}
	return nil
}

// RecordTierTransition moves the run's tier, appends the chain event, and
// returns the seq the chain trigger assigned — in one transaction.
//
// **The chain event is unconditional**, unlike RecordIncidentTransition's: the
// subject is a run, and a run is what agent_audit is keyed on.
//
// It fills in t.TierBefore and t.TransitionedAt.
func RecordTierTransition(ctx context.Context, db Beginner, t *TierTransition) (int, error) {
	if err := t.validate(); err != nil {
		return 0, err
	}
	if t.TransitionedAt.IsZero() {
		t.TransitionedAt = time.Now().UTC()
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("while opening the transaction for the tier transition of run %s: %w", t.RunId, err)
	}
	defer tx.Rollback(ctx)

	// Read and lock before deciding anything, on RecordIncidentTransition's
	// argument: the guard is checked against the row rather than trusted as it,
	// and the lock makes the read and the write atomic so a conflict names the
	// tier actually found.
	var current string
	err = tx.QueryRow(ctx,
		`SELECT tier FROM jetsapi.agent_run WHERE run_id = $1 FOR UPDATE`, t.RunId).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		// The same refusal RunTier makes, and for the same reason: a run that
		// was never made durable has no authority to be moved, and defaulting
		// one would invent the thing the record exists to establish.
		return 0, &ErrNoRun{RunId: t.RunId}
	}
	if err != nil {
		return 0, fmt.Errorf("while reading the tier of run %s: %w", t.RunId, err)
	}
	if current != t.FromTier {
		return 0, &ErrTierStateConflict{RunId: t.RunId, Expected: t.FromTier, Found: current}
	}
	t.TierBefore = current

	if _, err := tx.Exec(ctx,
		`UPDATE jetsapi.agent_run SET tier = $2 WHERE run_id = $1`,
		t.RunId, t.ToTier); err != nil {
		return 0, fmt.Errorf("while moving run %s to tier %s: %w", t.RunId, t.ToTier, err)
	}

	raise, err := t.IsRaise()
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(map[string]any{
		// The discriminator. agent_audit has one taxonomy column and it says
		// `decision` for both of this package's state changes, so what kind of
		// decision this is has to travel in the payload — recorded as I-449.
		"state_change":    TierStateChange,
		"run_ref":         t.RunId,
		"tier_before":     t.TierBefore,
		"tier_after":      t.ToTier,
		"raise":           raise,
		"actor_kind":      t.ActorKind,
		"transitioned_at": t.TransitionedAt.UTC().Format(time.RFC3339Nano),
		"rationale":       t.Rationale,
	})
	if err != nil {
		return 0, fmt.Errorf("while encoding the tier transition of run %s for the chain: %w", t.RunId, err)
	}

	// **The event is stamped with the tier the run was at, not the one it moves
	// to.** Event.Tier records the authority in force when an event happened,
	// and an act that grants T3 was not itself performed at T3 — a grant
	// stamped with what it grants would be circular, and a reader scanning the
	// chain for "what authority was in force here" would see the new value on
	// the very event that created it. Both operands are in the payload, where a
	// reader who wants the pair finds it as fields rather than as a sentence —
	// which is ErrTierTooLow's rule for a refusal, applied to a grant.
	seq, err := Append(ctx, tx, &Event{
		RunId:     t.RunId,
		EventType: EventDecision,
		Actor:     t.Actor,
		Tier:      t.TierBefore,
		Payload:   payload,
	})
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("while committing the tier transition of run %s: %w", t.RunId, err)
	}
	return seq, nil
}

// TierStateChange is the payload discriminator that distinguishes a tier
// transition from every other `decision` event in the chain. See I-449: the
// taxonomy column has six members and two of this package's writers share one
// of them.
const TierStateChange = "autonomy_tier"

// TierTransitionsFor returns one run's tier transitions, oldest first, read back
// out of the chain.
//
// **It exists because a write path with no read path is a store nobody can
// check** — K.2's rule, which is also why HumanVerdicts exists with nothing to
// count. There is no `jetsapi.tier_event` table to read instead: the chain is
// the record here, deliberately, because its key is the subject.
func TierTransitionsFor(ctx context.Context, db Querier, runId string) ([]TierTransition, error) {
	rows, err := db.Query(ctx,
		`SELECT actor, payload
		   FROM jetsapi.agent_audit
		  WHERE run_id = $1
		    AND event_type = $2
		    AND payload->>'state_change' = $3
		  ORDER BY seq`, runId, EventDecision, TierStateChange)
	if err != nil {
		return nil, fmt.Errorf("while reading the tier transitions of run %s: %w", runId, err)
	}
	defer rows.Close()
	var out []TierTransition
	for rows.Next() {
		var actor string
		var payload []byte
		if err := rows.Scan(&actor, &payload); err != nil {
			return nil, fmt.Errorf("while scanning a tier transition of run %s: %w", runId, err)
		}
		var p struct {
			TierBefore     string `json:"tier_before"`
			TierAfter      string `json:"tier_after"`
			ActorKind      string `json:"actor_kind"`
			Rationale      string `json:"rationale"`
			TransitionedAt string `json:"transitioned_at"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, fmt.Errorf("while decoding a tier transition of run %s: %w", runId, err)
		}
		t := TierTransition{
			RunId: runId, FromTier: p.TierBefore, ToTier: p.TierAfter,
			TierBefore: p.TierBefore, Actor: actor, ActorKind: p.ActorKind,
			Rationale: p.Rationale,
		}
		if at, err := time.Parse(time.RFC3339Nano, p.TransitionedAt); err == nil {
			t.TransitionedAt = at
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
