// The proposal read paths of task K.3 — the staging half of criterion 31 —
// tested against a real Postgres. Needs JETS_TEST_DSN; see the note at the head
// of audit_test.go.
//
// **The SQL is why these exist rather than a fake.** ListProposals leans on
// three things a hand-rolled stub would not exercise: an aggregate LEFT JOIN
// that must not multiply a proposal by its decisions, `cardinality($1::text[])`
// as the "no filter" case, and `array_length` returning NULL rather than 0 for
// an empty array — which is a Postgres behaviour, not a Go one, and the reason
// the coalesce is there.
package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedProposalIn is seedProposal with a chosen approval state, for the list
// tests. Ids are time-based per I-72: JETS_TEST_DSN names a database that
// persists between runs, and fixed ids pass in isolation and collide in a
// package run.
func seedProposalIn(t *testing.T, pool *pgxpool.Pool, suffix, state string) (runId, proposalId string) {
	t.Helper()
	ctx := context.Background()
	uniq := fmt.Sprintf("%s_%d", suffix, time.Now().UnixNano())
	runId = "run_k3_" + uniq
	proposalId = "chg_k3_" + uniq
	if err := StartRun(ctx, pool, &Run{
		RunId: runId, AgentId: "authoring", AgentVersion: "0.1.0",
		ModelId: "granite4.1:3b", PromptVersion: "p1", Tier: "T3",
		StartedAt: time.Now().UTC(), DomainModelVersion: "0.1.0",
		IterationCap: 3, WallClockCapSeconds: 60,
	}, []byte(`{"intent":"author a transformation"}`)); err != nil {
		t.Fatalf("seeding run: %v", err)
	}
	if err := RecordProposal(ctx, pool, &Proposal{
		ProposalId: proposalId, Trigger: "authoring_run", TriggerRef: runId,
		Rationale: "because", AffectedPipelines: []string{}, GeneratedTests: []string{},
		ImpactAffectedAssets: []string{}, ApprovalState: state, ModelVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("seeding proposal: %v", err)
	}
	return runId, proposalId
}

func findSummary(rows []ProposalSummary, id string) *ProposalSummary {
	for i := range rows {
		if rows[i].ProposalId == id {
			return &rows[i]
		}
	}
	return nil
}

// The staging list finds a freshly written proposal, and reports the counts
// rather than the arrays. A draft carries empty arrays honestly, and
// `array_length` on an empty array is NULL in Postgres — so "0 tests" only
// reaches a reviewer because of the coalesce.
func TestListProposalsFindsADraftAndCountsItsEmptyArrays(t *testing.T) {
	pool := testPool(t)
	_, proposalId := seedProposalIn(t, pool, "list", StateDraft)

	rows, err := ListProposals(context.Background(), pool, nil, 0)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	got := findSummary(rows, proposalId)
	if got == nil {
		t.Fatalf("the proposal just written is not in the unfiltered list of %d", len(rows))
	}
	if got.ApprovalState != StateDraft {
		t.Errorf("state %q, want draft", got.ApprovalState)
	}
	if got.GeneratedTestCount != 0 || got.AffectedPipelineCount != 0 {
		t.Errorf("empty arrays counted as %d tests and %d pipelines",
			got.GeneratedTestCount, got.AffectedPipelineCount)
	}
	if !got.LastDecisionAt.IsZero() {
		t.Errorf("a proposal nobody has decided reports a decision at %v", got.LastDecisionAt)
	}
}

// The filter is a set. An empty one means every state — the "no filter" arm of
// the query, which is a cardinality check rather than a nil comparison.
func TestListProposalsFiltersByStateSet(t *testing.T) {
	pool := testPool(t)
	_, draftId := seedProposalIn(t, pool, "filter_d", StateDraft)
	_, rejectedId := seedProposalIn(t, pool, "filter_r", StateRejected)
	ctx := context.Background()

	rows, err := ListProposals(ctx, pool, []string{StateRejected}, 0)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if findSummary(rows, rejectedId) == nil {
		t.Error("the rejected proposal is missing from a rejected-only list")
	}
	if findSummary(rows, draftId) != nil {
		t.Error("a draft came back from a rejected-only list")
	}

	both, err := ListProposals(ctx, pool, []string{StateDraft, StateRejected}, 0)
	if err != nil {
		t.Fatalf("listing two states: %v", err)
	}
	if findSummary(both, draftId) == nil || findSummary(both, rejectedId) == nil {
		t.Error("a two-state filter did not return both")
	}
}

// A state outside the vocabulary is a refusal rather than an empty list. An
// empty list would read as "nothing is in that state", which is a different and
// wrong answer to a typo.
func TestListProposalsRefusesAStateThatDoesNotExist(t *testing.T) {
	pool := testPool(t)
	if _, err := ListProposals(context.Background(), pool, []string{"drafted"}, 0); err == nil {
		t.Error("a state outside the vocabulary was accepted")
	}
}

// **The join must not multiply.** A proposal with two decisions is still one
// row, and its LastDecisionAt is the later of them. This is the whole reason
// the join is over an aggregate rather than over approval_event directly.
func TestATwiceDecidedProposalIsStillOneRow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposalIn(t, pool, "twice", StateDraft)

	first := time.Now().UTC().Add(-time.Hour)
	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apr_k3_a_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: StateDraft, ToState: StateValidated,
		Actor: "a@x", TierAtEvent: "T3", DecidedAt: first,
	}); err != nil {
		t.Fatalf("first decision: %v", err)
	}
	second := time.Now().UTC()
	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apr_k3_b_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: StateValidated, ToState: StateAgentReviewed,
		Actor: "b@x", TierAtEvent: "T3", DecidedAt: second,
	}); err != nil {
		t.Fatalf("second decision: %v", err)
	}

	rows, err := ListProposals(ctx, pool, nil, 0)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	seen := 0
	for _, r := range rows {
		if r.ProposalId == proposalId {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("a proposal with two decisions produced %d rows", seen)
	}
	got := findSummary(rows, proposalId)
	if got == nil {
		t.Fatal("the proposal vanished from the list")
	}
	if got.LastDecisionAt.Sub(second).Abs() > time.Second {
		t.Errorf("last decision reported as %v, want the later of the two (%v)", got.LastDecisionAt, second)
	}
	if got.ApprovalState != StateAgentReviewed {
		t.Errorf("state %q; the second decision should have moved it", got.ApprovalState)
	}
}

func TestReadProposalRoundTripsWhatWasWritten(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposalIn(t, pool, "read", StateDraft)

	p, err := ReadProposal(ctx, pool, proposalId)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if p.TriggerRef != runId || p.Trigger != "authoring_run" {
		t.Errorf("trigger %q/%q, want authoring_run/%s", p.Trigger, p.TriggerRef, runId)
	}
	if p.Rationale != "because" || p.ApprovalState != StateDraft {
		t.Errorf("round trip lost the rationale or the state: %+v", p)
	}
}

// A stale link is a distinct error, not a nil proposal and not a generic
// failure: it is the one the screen turns into "no such proposal" rather than
// "the server is unwell".
func TestReadProposalOnAnUnknownIdIsTyped(t *testing.T) {
	pool := testPool(t)
	_, err := ReadProposal(context.Background(), pool, fmt.Sprintf("chg_nope_%d", time.Now().UnixNano()))
	var missing *ErrNoProposal
	if !errors.As(err, &missing) {
		t.Errorf("got %v (%T), want an ErrNoProposal", err, err)
	}
}

// The tier a decision is recorded at comes from the run, and the run is the
// only place it can honestly come from.
func TestRunTierReadsTheRunsOwnTier(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, _ := seedProposalIn(t, pool, "tier", StateDraft)

	tier, err := RunTier(ctx, pool, runId)
	if err != nil {
		t.Fatalf("reading tier: %v", err)
	}
	if tier != "T3" {
		t.Errorf("tier %q, want the run's T3", tier)
	}

	_, err = RunTier(ctx, pool, fmt.Sprintf("run_nope_%d", time.Now().UnixNano()))
	var missing *ErrNoRun
	if !errors.As(err, &missing) {
		t.Errorf("got %v (%T), want an ErrNoRun — a decision must not fall back to a guessed tier", err, err)
	}
}
