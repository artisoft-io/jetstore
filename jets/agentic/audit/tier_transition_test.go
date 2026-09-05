package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The autonomy tier transition (task AJ.2, gap 7b, criterion 51) — what raises a
// run's tier, who may, and what the audit record shows.
//
// **The refusals are in the committed suite rather than in a throwaway script**,
// which is §11.5's rule and P4 §12.6's practice: a negative control that lives in
// a script runs once, on the day it was written.

func startTierRun(t *testing.T, pool *pgxpool.Pool, tier string) string {
	t.Helper()
	runId := fmt.Sprintf("run_tt_%d", time.Now().UnixNano())
	err := StartRun(context.Background(), pool, &Run{
		RunId: runId, AgentId: "agent:test", AgentVersion: "0.1.0",
		ModelId: "test-model", PromptVersion: "p1", Tier: tier,
		StartedAt: time.Now().UTC(), DomainModelVersion: "0.1.0",
		IterationCap: 4, WallClockCapSeconds: 60,
	}, []byte(`{"intent":"tier transition test"}`))
	if err != nil {
		t.Fatalf("starting run %s: %v", runId, err)
	}
	return runId
}

func runTier(t *testing.T, pool *pgxpool.Pool, runId string) string {
	t.Helper()
	tier, err := RunTier(context.Background(), pool, runId)
	if err != nil {
		t.Fatalf("reading the tier of %s: %v", runId, err)
	}
	return tier
}

// A human raises a run's authority: the run moves, the chain records it, and
// the record carries both operands as fields.
func TestATierRaiseMovesTheRunAndIsChained(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTierRun(t, pool, string(TierT1))

	tt := &TierTransition{
		RunId: runId, FromTier: string(TierT1), ToTier: string(TierT3),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
		Rationale: "the remediation was reviewed and the run may act on it",
	}
	seq, err := RecordTierTransition(ctx, pool, tt)
	if err != nil {
		t.Fatalf("recording the raise: %v", err)
	}
	if seq <= 0 {
		t.Errorf("seq = %d; a tier transition's chain event is unconditional, its subject being a run", seq)
	}
	if got := runTier(t, pool, runId); got != string(TierT3) {
		t.Errorf("the run is at %q after a raise to T3", got)
	}
	if tt.TierBefore != string(TierT1) {
		t.Errorf("TierBefore read back as %q; it is an output, taken off the locked row", tt.TierBefore)
	}
	if tt.TransitionedAt.IsZero() {
		t.Error("TransitionedAt was not filled in")
	}

	// The event stamps the tier that was in force, not the one granted — a
	// grant stamped with what it grants is circular.
	var eventTier, actor string
	var payload []byte
	if err := pool.QueryRow(ctx,
		`SELECT coalesce(tier, ''), actor, payload FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
		runId, seq).Scan(&eventTier, &actor, &payload); err != nil {
		t.Fatalf("reading the chain event: %v", err)
	}
	if eventTier != string(TierT1) {
		t.Errorf("the grant event is stamped %q; it records the authority in force, which was T1", eventTier)
	}
	if actor != "michel@artisoft.io" {
		t.Errorf("the chain event's actor is %q", actor)
	}
	var p map[string]any
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	for k, want := range map[string]any{
		"state_change": TierStateChange,
		"tier_before":  string(TierT1),
		"tier_after":   string(TierT3),
		"actor_kind":   ActorHuman,
		"raise":        true,
	} {
		if p[k] != want {
			t.Errorf("payload[%q] = %v, want %v — both operands travel as fields rather than as a sentence", k, p[k], want)
		}
	}

	// And the chain is still verifiable with the grant in it.
	tr, err := ReadTranscript(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	if !tr.Verified() {
		t.Errorf("the chain does not verify with a tier transition in it: %v", tr.Defects)
	}
}

// **A gate the gated party can open is not a gate.** This is the one rule in
// this file that is a security property rather than bookkeeping.
func TestAnAgentMayNotRaiseItsOwnTier(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTierRun(t, pool, string(TierT1))

	_, err := RecordTierTransition(ctx, pool, &TierTransition{
		RunId: runId, FromTier: string(TierT1), ToTier: string(TierT3),
		Actor: "agent:triage", ActorKind: ActorAgent,
		Rationale: "I would like to act",
	})
	var refused *ErrTierRaiseNotHuman
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want an *ErrTierRaiseNotHuman", err)
	}
	if refused.From != string(TierT1) || refused.To != string(TierT3) {
		t.Errorf("the refusal carries %s -> %s", refused.From, refused.To)
	}
	// Refused before the transaction, so nothing moved.
	if got := runTier(t, pool, runId); got != string(TierT1) {
		t.Errorf("the run is at %q after a refused raise", got)
	}
}

// The asymmetry, which is the half a reader will want to check: an agent may
// reduce its own authority, and needs no reason to.
func TestAnAgentMayLowerItsOwnTier(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTierRun(t, pool, string(TierT3))

	seq, err := RecordTierTransition(ctx, pool, &TierTransition{
		RunId: runId, FromTier: string(TierT3), ToTier: string(TierT1),
		Actor: "agent:triage", ActorKind: ActorAgent,
	})
	if err != nil {
		t.Fatalf("an agent lowering its own tier must be permitted: %v", err)
	}
	if seq <= 0 {
		t.Errorf("seq = %d", seq)
	}
	if got := runTier(t, pool, runId); got != string(TierT1) {
		t.Errorf("the run is at %q after lowering to T1", got)
	}
}

// A grant with no stated reason is the record a supervisor cannot audit.
func TestARaiseWithNoReasonIsRefused(t *testing.T) {
	err := (&TierTransition{
		RunId: "run_x", FromTier: string(TierT1), ToTier: string(TierT2),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
	}).validate()
	if err == nil || !strings.Contains(err.Error(), "states no reason") {
		t.Fatalf("want a refusal naming the missing reason; got %v", err)
	}
	// And a lowering with none is fine, which is the same asymmetry stated as a
	// pair rather than as two tests a reader has to join up.
	if err := (&TierTransition{
		RunId: "run_x", FromTier: string(TierT2), ToTier: string(TierT1),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman,
	}).validate(); err != nil {
		t.Fatalf("a lowering needs no stated reason: %v", err)
	}
}

// The guard, restated from RecordIncidentTransition: a decision taken against a
// stale view of the run is a conflict rather than a silent overwrite.
func TestATierTransitionAgainstAStaleTierIsAConflict(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTierRun(t, pool, string(TierT2))

	_, err := RecordTierTransition(ctx, pool, &TierTransition{
		RunId: runId, FromTier: string(TierT1), ToTier: string(TierT3),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman, Rationale: "reviewed",
	})
	var conflict *ErrTierStateConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want an *ErrTierStateConflict", err)
	}
	if conflict.Found != string(TierT2) || conflict.Expected != string(TierT1) {
		t.Errorf("conflict says found %q expected %q", conflict.Found, conflict.Expected)
	}
	if got := runTier(t, pool, runId); got != string(TierT2) {
		t.Errorf("the run moved to %q on a refused transition", got)
	}
}

// A run that was never made durable has no authority to move — ErrNoRun rather
// than a default, which is RunTier's own argument passed through.
func TestATierTransitionOnAnUnrecordedRunIsRefused(t *testing.T) {
	pool := testPool(t)
	_, err := RecordTierTransition(context.Background(), pool, &TierTransition{
		RunId: "run_that_never_was", FromTier: string(TierT1), ToTier: string(TierT2),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman, Rationale: "reviewed",
	})
	var missing *ErrNoRun
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, want an *ErrNoRun", err)
	}
}

// The vocabulary is asserted at the boundary, which is Q-56's answer in
// practice: ParseTier is what catches an unknown value, and a named string type
// would not have.
func TestATierTransitionOutsideTheVocabularyIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, from, to string }{
		{"from", "assisted", string(TierT2)},
		{"to", string(TierT1), "autonomous"},
		{"from empty", "", string(TierT2)},
		{"to empty", string(TierT1), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (&TierTransition{
				RunId: "run_v", FromTier: tc.from, ToTier: tc.to,
				Actor: "michel@artisoft.io", ActorKind: ActorHuman, Rationale: "reviewed",
			}).validate()
			if err == nil || !strings.Contains(err.Error(), "is not an autonomy tier") {
				t.Fatalf("want a vocabulary refusal; got %v", err)
			}
		})
	}
}

// A transition that changes no authority would put a row in the chain asserting
// that somebody took responsibility for nothing.
func TestATierTransitionThatChangesNothingIsRefused(t *testing.T) {
	err := (&TierTransition{
		RunId: "run_n", FromTier: string(TierT2), ToTier: string(TierT2),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman, Rationale: "reviewed",
	}).validate()
	if err == nil || !strings.Contains(err.Error(), "changes no authority") {
		t.Fatalf("want a refusal of the no-op; got %v", err)
	}
}

// K.2's rule: a write path with no read path is a store nobody can check. There
// is no tier_event table — the chain is the record, its key being the subject —
// so the read path reads the chain.
func TestTierTransitionsForReadsTheChainBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startTierRun(t, pool, string(TierT1))

	if _, err := RecordTierTransition(ctx, pool, &TierTransition{
		RunId: runId, FromTier: string(TierT1), ToTier: string(TierT3),
		Actor: "michel@artisoft.io", ActorKind: ActorHuman, Rationale: "reviewed the diagnosis",
	}); err != nil {
		t.Fatalf("raising: %v", err)
	}
	if _, err := RecordTierTransition(ctx, pool, &TierTransition{
		RunId: runId, FromTier: string(TierT3), ToTier: string(TierT0),
		Actor: "agent:triage", ActorKind: ActorAgent,
	}); err != nil {
		t.Fatalf("lowering: %v", err)
	}

	got, err := TierTransitionsFor(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("read back %d transitions, want 2", len(got))
	}
	if got[0].FromTier != string(TierT1) || got[0].ToTier != string(TierT3) ||
		got[0].ActorKind != ActorHuman || got[0].Rationale != "reviewed the diagnosis" {
		t.Errorf("first transition read back as %+v", got[0])
	}
	if got[1].FromTier != string(TierT3) || got[1].ToTier != string(TierT0) ||
		got[1].ActorKind != ActorAgent {
		t.Errorf("second transition read back as %+v", got[1])
	}
	// The discriminator earns its place: the run's intent event is also in the
	// chain and is not a tier transition.
	if got[0].RunId != runId {
		t.Errorf("read back under run %q", got[0].RunId)
	}
}

// recordedActs is the two writers behind one shape. See state_change.go: Q-52
// was answered on the argument that a human verdict on an incident and a tier
// transition are the same shape of act, and this is that claim as a test rather
// than as a sentence.
//
// The tier case is a *lowering* so that both instances are valid for either
// actor kind; the who-may rule is the one thing the two acts do not share, and
// it has its own tests above.
var recordedActs = []struct {
	name  string
	build func(actor, kind string) (StateChange, func() error)
}{
	{"incident transition", func(actor, kind string) (StateChange, func() error) {
		t := &IncidentTransition{
			IncidentEventId: "ie_shape", IncidentRef: "inc_shape",
			FromStatus: IncidentTriaged, ToStatus: IncidentDiagnosed,
			Actor: actor, ActorKind: kind,
		}
		return t, t.validate
	}},
	{"autonomy tier transition", func(actor, kind string) (StateChange, func() error) {
		t := &TierTransition{
			RunId: "run_shape", FromTier: string(TierT3), ToTier: string(TierT1),
			Actor: actor, ActorKind: kind,
		}
		return t, t.validate
	}},
}

func TestBothRecordedActsAreTheSameShape(t *testing.T) {
	// The interface assertion is the compile-time half. It is here rather than
	// at package scope so that a reader meets it beside what it is for.
	var _ StateChange = (*IncidentTransition)(nil)
	var _ StateChange = (*TierTransition)(nil)

	for _, act := range recordedActs {
		t.Run(act.name+"/is well formed", func(t *testing.T) {
			sc, validate := act.build("someone", ActorAgent)
			if err := validate(); err != nil {
				t.Fatalf("the valid instance was refused: %v", err)
			}
			if sc.Subject() == "" {
				t.Error("Subject() is empty; a state change names the row it moves")
			}
			from, to := sc.From(), sc.To()
			if from == "" || to == "" || from == to {
				t.Errorf("From()/To() = %q/%q", from, to)
			}
			if actor, kind := sc.Attribution(); actor != "someone" || kind != ActorAgent {
				t.Errorf("Attribution() = %q/%q", actor, kind)
			}
		})
		t.Run(act.name+"/refuses an unattributable act", func(t *testing.T) {
			_, validate := act.build("", ActorAgent)
			if err := validate(); err == nil || !strings.Contains(err.Error(), "names no actor") {
				t.Fatalf("want a refusal naming the missing actor; got %v", err)
			}
		})
		t.Run(act.name+"/refuses an unknown actor kind", func(t *testing.T) {
			for _, kind := range []string{"", "detector"} {
				_, validate := act.build("someone", kind)
				if err := validate(); err == nil || !strings.Contains(err.Error(), "actor kind") {
					t.Fatalf("kind %q: want a refusal naming the actor kind; got %v", kind, err)
				}
			}
		})
	}
}
