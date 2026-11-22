// A decision on a proposal no run produced: recorded and not chained, against
// a real Postgres. Needs JETS_TEST_DSN; see the note at the head of
// audit_test.go.
package audit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedIncidentProposal records a draft whose trigger is an incident, as a feed
// incident's drafted proposal is.
func seedIncidentProposal(t *testing.T, pool *pgxpool.Pool, suffix string) (incidentId, proposalId string) {
	t.Helper()
	uniq := fmt.Sprintf("%s%d", suffix, time.Now().UnixNano())
	incidentId = "inc_runless_" + uniq
	proposalId = "chg_runless" + uniq
	if err := RecordProposal(context.Background(), pool, &Proposal{
		ProposalId: proposalId, Trigger: TriggerIncident, TriggerRef: incidentId,
		Rationale: "declare the feed cumulative", ApprovalState: StateDraft, ModelVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("seeding proposal: %v", err)
	}
	return incidentId, proposalId
}

// walk records each step of path as one decision by actor, unchained.
func walk(t *testing.T, pool *pgxpool.Pool, incidentId, proposalId, actor string, path ...string) {
	t.Helper()
	for i := 1; i < len(path); i++ {
		seq, err := RecordApproval(context.Background(), pool, &Approval{
			ApprovalEventId: fmt.Sprintf("apr_%s_%d", proposalId, i), RunRef: incidentId,
			SubjectRef: proposalId, FromState: path[i-1], ToState: path[i],
			Actor: actor, TierAtEvent: RunlessDecisionTier, DecisionRationale: "read the evidence",
			Unchained: true,
		})
		if err != nil {
			t.Fatalf("%s -> %s: %v", path[i-1], path[i], err)
		}
		if seq != 0 {
			t.Errorf("%s -> %s returned chain seq %d for a decision that joins no chain", path[i-1], path[i], seq)
		}
	}
}

func TestARunlessProposalIsDecidedAndNotChained(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	for _, path := range [][]string{
		{StateDraft, StateValidated, StateAgentReviewed, StateAwaitingHumanApproval, StateApproved},
		{StateDraft, StateValidated, StateAgentReviewed, StateAwaitingHumanApproval, StateRejected},
	} {
		end := path[len(path)-1]
		incidentId, proposalId := seedIncidentProposal(t, pool, end)
		walk(t, pool, incidentId, proposalId, "person@example.com", path...)

		p, err := ReadProposal(ctx, pool, proposalId)
		if err != nil {
			t.Fatal(err)
		}
		if p.ApprovalState != end {
			t.Errorf("the proposal is in %s, want %s", p.ApprovalState, end)
		}
		got, err := ApprovalsFor(ctx, pool, proposalId)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(path)-1 {
			t.Fatalf("%d decisions recorded, want %d", len(got), len(path)-1)
		}
		for _, a := range got {
			if a.RunRef != incidentId || a.TierAtEvent != RunlessDecisionTier || a.Actor != "person@example.com" {
				t.Errorf("recorded with run_ref %s, tier %s, actor %s", a.RunRef, a.TierAtEvent, a.Actor)
			}
		}
		// No chain event, under the incident id or anywhere else.
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.agent_audit
			WHERE run_id = $1 OR payload ->> 'subject_ref' = $2`, incidentId, proposalId).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%d chain events for a proposal no run produced", n)
		}
	}
}

// Whether a decision joins a chain is the proposal's property: a run-backed
// proposal cannot be decided unchained, and a runless one cannot claim a chain.
func TestTheChainIsTheProposalsChoiceNotTheCallers(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	runId, runBacked := seedProposal(t, pool, "mismatch")
	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apr_mismatch_run_" + runBacked, RunRef: runId, SubjectRef: runBacked,
		FromState: StateDraft, ToState: StateValidated, Actor: "a@b", TierAtEvent: RunlessDecisionTier,
		Unchained: true,
	}); err == nil {
		t.Error("a run-backed proposal was decided outside its run's chain")
	}

	incidentId, runless := seedIncidentProposal(t, pool, "mismatch")
	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apr_mismatch_inc_" + runless, RunRef: incidentId, SubjectRef: runless,
		FromState: StateDraft, ToState: StateValidated, Actor: "a@b", TierAtEvent: "T2",
	}); err == nil {
		t.Error("a runless proposal was decided into a chain named after its incident")
	}
	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apr_mismatch_tier_" + runless, RunRef: incidentId, SubjectRef: runless,
		FromState: StateDraft, ToState: StateValidated, Actor: "a@b", TierAtEvent: "T2", Unchained: true,
	}); err == nil {
		t.Error("a runless decision was recorded at a tier other than the runless tier")
	}
	for _, id := range []string{runBacked, runless} {
		p, err := ReadProposal(ctx, pool, id)
		if err != nil {
			t.Fatal(err)
		}
		if p.ApprovalState != StateDraft {
			t.Errorf("%s moved to %s on a refused decision", id, p.ApprovalState)
		}
	}
}
