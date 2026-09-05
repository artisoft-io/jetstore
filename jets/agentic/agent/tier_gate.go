package agent

import (
	"context"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
)

// The autonomy gate (task AJ.1, gap 7b, criterion 51).
//
// **This is the gate, not the thing it gates.** Nothing here executes a
// remediation and nothing here decides what a run may be raised to; Phase 5's
// §2 says the comparison track builds the gate, and `AJ.2` carries the
// transition. What this file adds is the missing `if`: the tier an action
// requires, compared against the authority the run is operating at, refused
// below it, and the refusal on the transcript.
//
// # Two required tiers exist and only one is reachable from here
//
// Gap 7b's row is about `Remediation.autonomy_tier_required`
// (`autonomy_tier_required`, tools/jets_agentic/jets_agentic/model.py:1009).
// There is no remediation executor — criterion 47 records that as deliberate —
// so there is no call site for it. What *is* reachable at the loop's enforcement
// point is the other one: `Signature.MinTier`
// (`MinTier`, jets/agentic/tools/registry.go:30), carried since Phase 1 and
// surfaced to MCP clients as `jetstore.min_tier`
// (`jetstore.min_tier`, jets/agentic/mcp/adapter.go:56), and likewise never
// compared — its only reader outside this file asserts that all three tools are
// T0.
//
// **So gap 7b's comparison was missing twice, and this closes the half that has
// a caller.** The primitive underneath — audit.Tier, audit.ParseTier,
// audit.Reaches — and the refusal below take a required tier as a string and
// care nothing about where it came from, so an executor arriving later uses them
// unchanged with `autonomy_tier_required` in place of `MinTier`. That is the
// whole of the claim: the remediation path is **not** gated by this change, and
// nothing here should be read as saying it is.
//
// # Nil means ungated, on Guard's precedent
//
// The gate is a separate field on Loop rather than a widening of Guard, because
// the two answer different questions. Guard asks *may this identity still act*
// — a kill switch over an identity, with no action in the question. This asks
// *does this run's authority reach this action*, which is meaningless without
// one. Nil means ungated, which is what the existing tests are and what
// production must not be, and it is why adding this breaks nothing.

// ErrTierTooLow is the refusal. It carries both operands and the action so the
// audit payload can be structured rather than a sentence somebody later parses.
type ErrTierTooLow struct {
	// Action is what was refused — a tool name at the loop's call site.
	Action string
	// Required is the tier the action declares.
	Required string
	// Current is the authority the run is operating at.
	Current string
}

func (e *ErrTierTooLow) Error() string {
	return fmt.Sprintf("agent: %q requires autonomy tier %s and this run is operating at %s; refusing to act",
		e.Action, e.Required, e.Current)
}

// TierGate answers whether this run's authority reaches what an action
// requires. It is consulted before every tool call, so an implementation must
// be cheap; it may re-read its source, and PgTierGate does.
type TierGate interface {
	// Permit returns nil when the run may take an action requiring
	// requiredTier, and an error otherwise. The error is *ErrTierTooLow when
	// the comparison was made and refused, and something else when it could
	// not be made — a caller that flattens the two loses the distinction
	// between "not authorised" and "could not tell".
	Permit(ctx context.Context, action, requiredTier string) error
}

// TierGateFunc adapts a function to TierGate.
type TierGateFunc func(ctx context.Context, action, requiredTier string) error

func (f TierGateFunc) Permit(ctx context.Context, action, requiredTier string) error {
	return f(ctx, action, requiredTier)
}

// TierSource is the optional interface a Verifier may satisfy to report the
// tier a tool requires. *tools.Registry satisfies it.
//
// **It is an assertion at the call site rather than a widening of Verifier**,
// because Verifier is deliberately the narrowest thing the loop needs — "the
// registry, narrowed the same way" — and every stub in the suite implements it.
// Widening the interface to carry a tier would make a tier gate a compile-time
// obligation of everything that can answer a tool call, which is the wrong
// trade for a field that is nil by default.
type TierSource interface {
	MinTierOf(name string) (string, error)
}

// FixedTierGate compares against a tier fixed for the run — Loop.Tier, which is
// what the caller declared and what PgRecorder writes into agent_run.tier.
//
// **It is the weaker of the two implementations and the docstring says so
// rather than leaving it to be discovered.** The authority it checks against is
// the caller's own claim, so it bounds what the loop will do and says nothing
// about what the caller was entitled to claim. That is still worth having — it
// is the difference between a tool call that happens and one that does not —
// but a governance record wants PgTierGate, on exactly the argument RunTier's
// docstring already makes for approvals.
type FixedTierGate struct {
	// Current is the authority the run is operating at.
	Current string
}

func (g *FixedTierGate) Permit(_ context.Context, action, requiredTier string) error {
	return compareTiers(action, g.Current, requiredTier)
}

// PgTierGate resolves the run's authority from jetsapi.agent_run, freshly,
// every time it is asked.
//
// **The re-read is the same property PgGuard has and for the same reason.** A
// gate that caches would answer from whatever the process was told at start-up;
// this one answers from the committed record, so a run that was never made
// durable has no tier to be checked against and is refused rather than
// defaulted. RunTier returns *ErrNoRun rather than a default precisely because
// there is no honest tier to fall back on, and that refusal is passed through
// here rather than softened.
type PgTierGate struct {
	DB audit.Querier
	// RunId is the run whose authority is being checked. It is the loop's own
	// RunId; the gate reads the tier rather than being handed it, which is the
	// whole of what this implementation adds over FixedTierGate.
	RunId string
}

func (g *PgTierGate) Permit(ctx context.Context, action, requiredTier string) error {
	if g.RunId == "" {
		return fmt.Errorf("agent: no run id to resolve an autonomy tier from, refusing to act; "+
			"a gate that cannot reach its source must not fail open (action %q)", action)
	}
	current, err := audit.RunTier(ctx, g.DB, g.RunId)
	if err != nil {
		return fmt.Errorf("agent: cannot resolve the autonomy tier of run %s, refusing to act: %w", g.RunId, err)
	}
	return compareTiers(action, current, requiredTier)
}

// compareTiers is the comparison both implementations share. Both operands are
// parsed before either is compared, so an unreadable value is reported as one
// rather than as an authority failure.
func compareTiers(action, current, required string) error {
	req, err := audit.ParseTier(required)
	if err != nil {
		return fmt.Errorf("agent: %q declares an autonomy tier this build cannot read, refusing to act: %w", action, err)
	}
	cur, err := audit.ParseTier(current)
	if err != nil {
		return fmt.Errorf("agent: this run's autonomy tier cannot be read, refusing to act: %w", err)
	}
	ok, err := audit.Reaches(cur, req)
	if err != nil {
		return fmt.Errorf("agent: the autonomy tiers of %q cannot be compared, refusing to act: %w", action, err)
	}
	if !ok {
		return &ErrTierTooLow{Action: action, Required: string(req), Current: string(cur)}
	}
	return nil
}

// permitTool is the loop's side of the gate: find what the tool requires, then
// ask.
//
// **It fails closed at both steps.** A registry that cannot report a tier and a
// tool whose signature carries none are both refusals, not exemptions — the
// alternative is a gate that quietly passes everything it could not read, which
// is worse than no gate because it looks like one. guard.go sets this precedent
// in as many words: a check that cannot reach its source must not fail open.
func (l *Loop) permitTool(ctx context.Context, name string) error {
	src, ok := l.Registry.(TierSource)
	if !ok {
		return fmt.Errorf("agent: a tier gate is configured and this registry cannot say what tier %q requires; "+
			"refusing to act rather than assuming one", name)
	}
	required, err := src.MinTierOf(name)
	if err != nil {
		return fmt.Errorf("agent: cannot determine the tier %q requires, refusing to act: %w", name, err)
	}
	return l.TierGate.Permit(ctx, name, required)
}
