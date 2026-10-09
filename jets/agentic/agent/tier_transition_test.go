package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// What a tier transition does to a gate (task AJ.2, gap 7b, criterion 51).
//
// **`AJ.1` built the comparison and this is what makes it more than a
// ceiling.** These tests are in `agent` rather than in `audit` because the
// claim is about the two packages together: `audit.RecordTierTransition` moves
// the committed record, and `PgTierGate` re-reads it per check, so a grant
// recorded by a person while a run is in flight is honoured at the run's very
// next tool call with nothing restarted and no gate rebuilt.
//
// Needs JETS_TEST_DSN, the same throwaway database the rest of this suite uses.

func startGatedRun(t *testing.T, pool *pgxpool.Pool, tier string) string {
	t.Helper()
	runId := fmt.Sprintf("run_ttg_%d", time.Now().UnixNano())
	err := audit.StartRun(context.Background(), pool, &audit.Run{
		RunId: runId, AgentId: "agent:test", AgentVersion: "0.1.0",
		ModelId: "test-model", PromptVersion: "p1", Tier: tier,
		StartedAt: time.Now().UTC(), DomainModelVersion: "0.1.0",
		IterationCap: 4, WallClockCapSeconds: 60,
	}, []byte(`{"intent":"tier gate test"}`))
	if err != nil {
		t.Fatalf("starting run %s: %v", runId, err)
	}
	return runId
}

// The demonstration: refused, granted, permitted — one gate, one run, no
// restart.
func TestATierRaiseUnblocksAGatedRunWithoutRestartingIt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startGatedRun(t, pool, string(audit.TierT1))

	gate := &PgTierGate{DB: pool, RunId: runId}

	// Before: a T1 run does not reach a T3 action.
	err := gate.Permit(ctx, "validate_cpipes_config", string(audit.TierT3))
	var low *ErrTierTooLow
	if !errors.As(err, &low) {
		t.Fatalf("before the grant: error = %v, want an *ErrTierTooLow", err)
	}
	if low.Current != string(audit.TierT1) || low.Required != string(audit.TierT3) {
		t.Errorf("refusal carries current %q required %q", low.Current, low.Required)
	}

	// A person takes responsibility for the change.
	if _, err := audit.RecordTierTransition(ctx, pool, &audit.TierTransition{
		RunId: runId, FromTier: string(audit.TierT1), ToTier: string(audit.TierT3),
		Actor: "michel@artisoft.io", ActorKind: audit.ActorHuman,
		Rationale: "the proposed change was reviewed at the supervision screen",
	}); err != nil {
		t.Fatalf("recording the raise: %v", err)
	}

	// After: **the same gate object**, which is the whole claim. PgTierGate
	// re-reads jetsapi.agent_run on every check, so the grant takes effect at
	// the next tool call rather than at the next process start.
	if err := gate.Permit(ctx, "validate_cpipes_config", string(audit.TierT3)); err != nil {
		t.Fatalf("after the grant: %v", err)
	}

	// And the record shows both halves in one chain: the refusal's operands are
	// on the transcript at the loop's call site (AJ.1, F501), and the grant is
	// here with its own.
	ts, err := audit.TierTransitionsFor(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the transitions back: %v", err)
	}
	if len(ts) != 1 || ts[0].ToTier != string(audit.TierT3) || ts[0].ActorKind != audit.ActorHuman {
		t.Fatalf("the chain reads back as %+v", ts)
	}
	tr, err := audit.ReadTranscript(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the transcript: %v", err)
	}
	if !tr.Verified() {
		t.Errorf("the chain does not verify: %v", tr.Defects)
	}
}

// **A FixedTierGate does not notice, and that is a finding rather than a
// caveat.** I-406 records that `Loop.Tier` and `PgRecorder.Run.Tier` are two
// fields nothing compares; a transition moves the second and cannot move the
// first, so the weaker gate keeps refusing after a person has granted the
// authority. That is the concrete cost of the divergence the issue describes,
// and it is why `PgTierGate`'s docstring calls FixedTierGate the weaker of the
// two rather than leaving it to be discovered.
func TestAFixedTierGateDoesNotSeeAGrant(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId := startGatedRun(t, pool, string(audit.TierT1))

	fixed := &FixedTierGate{Current: string(audit.TierT1)}
	if err := fixed.Permit(ctx, "validate_cpipes_config", string(audit.TierT3)); err == nil {
		t.Fatal("a T1 run must not reach a T3 action")
	}

	if _, err := audit.RecordTierTransition(ctx, pool, &audit.TierTransition{
		RunId: runId, FromTier: string(audit.TierT1), ToTier: string(audit.TierT3),
		Actor: "michel@artisoft.io", ActorKind: audit.ActorHuman,
		Rationale: "granted at the screen",
	}); err != nil {
		t.Fatalf("recording the raise: %v", err)
	}

	if err := fixed.Permit(ctx, "validate_cpipes_config", string(audit.TierT3)); err == nil {
		t.Fatal("a FixedTierGate compares against the caller's own claim, which a grant does not touch; " +
			"if this now passes, the two tiers have been reconciled and I-406 is closed")
	}
	// The committed record did move, which is what separates "the grant did not
	// happen" from "this gate cannot see it".
	tier, err := audit.RunTier(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading the run's tier: %v", err)
	}
	if tier != string(audit.TierT3) {
		t.Errorf("the committed tier is %q; the grant was recorded", tier)
	}
}
