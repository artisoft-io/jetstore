// Criterion 28 — an approval is durable, joined to the audit chain, and
// survives restart — tested against a real Postgres. Needs JETS_TEST_DSN; see
// the note at the head of audit_test.go.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedProposal makes a run and a draft proposal for an approval to act on, and
// returns the run id. The run is finished deliberately: an approval arrives
// after the run that produced the proposal has ended, and the point of the test
// is that the chain still accepts it.
func seedProposal(t *testing.T, pool *pgxpool.Pool, suffix string) (runId, proposalId string) {
	t.Helper()
	ctx := context.Background()
	uniq := fmt.Sprintf("%s_%d", suffix, time.Now().UnixNano())
	runId = "run_k1_" + uniq
	proposalId = "chg_k1_" + uniq
	err := StartRun(ctx, pool, &Run{
		RunId: runId, AgentId: "authoring", AgentVersion: "0.1.0",
		ModelId: "granite4.1:3b", PromptVersion: "p1", Tier: "T1",
		StartedAt: time.Now().UTC(), DomainModelVersion: "0.1.0",
		IterationCap: 3, WallClockCapSeconds: 60,
	}, []byte(`{"intent":"author a transformation"}`))
	if err != nil {
		t.Fatalf("seeding run: %v", err)
	}
	if err := FinishRun(ctx, pool, runId, "succeeded", 100); err != nil {
		t.Fatalf("finishing run: %v", err)
	}
	if err := RecordProposal(ctx, pool, &Proposal{
		ProposalId: proposalId, Trigger: "authoring_run", TriggerRef: runId,
		Rationale: "because", AffectedPipelines: []string{}, GeneratedTests: []string{},
		ImpactAffectedAssets: []string{}, ApprovalState: "draft", ModelVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("seeding proposal: %v", err)
	}
	return runId, proposalId
}

func TestApprovalIsDurableAndJoinedToTheChain(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposal(t, pool, "durable")

	seq, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apv_k1_durable_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: "draft", ToState: "approved",
		Actor: "michel@artisoft.io", TierAtEvent: "T2",
		DecisionRationale: "read it, it is fine",
	})
	if err != nil {
		t.Fatalf("recording approval: %v", err)
	}

	// Joined to the chain: the approval event is in agent_audit, on this run,
	// and it is not the first event — it follows the intent the run opened
	// with, which is what "joined" has to mean.
	if seq < 2 {
		t.Errorf("approval landed at seq %d; it should follow the run's intent event", seq)
	}
	var evType, actor string
	var payload []byte
	if err := pool.QueryRow(ctx,
		`SELECT event_type, actor, payload FROM jetsapi.agent_audit WHERE run_id = $1 AND seq = $2`,
		runId, seq).Scan(&evType, &actor, &payload); err != nil {
		t.Fatalf("reading the chain event: %v", err)
	}
	if evType != EventApproval {
		t.Errorf("chain event is %q, want %q", evType, EventApproval)
	}
	if actor != "michel@artisoft.io" {
		t.Errorf("chain event actor is %q", actor)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("chain payload is not JSON: %v", err)
	}
	if decoded["subject_ref"] != proposalId || decoded["to_state"] != "approved" {
		t.Errorf("chain payload does not identify the decision: %v", decoded)
	}

	// The chain is still a chain: this row's prev_hash is the previous row's
	// row_hash. Appending after the run ended must not break that.
	var linked bool
	if err := pool.QueryRow(ctx,
		`SELECT a.prev_hash = b.row_hash
		   FROM jetsapi.agent_audit a JOIN jetsapi.agent_audit b
		     ON b.run_id = a.run_id AND b.seq = a.seq - 1
		  WHERE a.run_id = $1 AND a.seq = $2`, runId, seq).Scan(&linked); err != nil {
		t.Fatalf("checking the chain link: %v", err)
	}
	if !linked {
		t.Error("the approval event does not link to its predecessor's hash")
	}

	// The proposal moved, in the same transaction.
	var state string
	if err := pool.QueryRow(ctx,
		`SELECT approval_state FROM jetsapi.change_proposal WHERE proposal_id = $1`,
		proposalId).Scan(&state); err != nil {
		t.Fatalf("reading the proposal: %v", err)
	}
	if state != "approved" {
		t.Errorf("proposal is in %q, want approved", state)
	}

	// Survives restart: a new pool is a new set of connections and a new
	// session. Nothing of this record lived in the process that wrote it.
	fresh, err := pgxpool.New(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer fresh.Close()
	got, err := ApprovalsFor(ctx, fresh, proposalId)
	if err != nil {
		t.Fatalf("reading approvals after reopening: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("after reopening, %d approvals; want 1", len(got))
	}
	if got[0].Actor != "michel@artisoft.io" || got[0].FromState != "draft" ||
		got[0].ToState != "approved" || got[0].DecisionRationale != "read it, it is fine" {
		t.Errorf("the approval did not survive intact: %+v", got[0])
	}
}

func TestSecondDecisionOnOneProposalIsRefusedWhole(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposal(t, pool, "conflict")

	if _, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apv_k1_first_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: "draft", ToState: "approved", Actor: "first@artisoft.io", TierAtEvent: "T2",
	}); err != nil {
		t.Fatalf("first approval: %v", err)
	}

	before := countRows(t, pool, runId, proposalId)

	// A second actor who read the proposal as `draft` and decided on that
	// basis. The guard must refuse it.
	_, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apv_k1_second_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: "draft", ToState: "rejected", Actor: "second@artisoft.io", TierAtEvent: "T2",
	})
	var conflict *ErrStateConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("second decision returned %v; want an ErrStateConflict", err)
	}

	// And refused *whole*: no half-written approval, no orphan chain event.
	// This is the property the single transaction buys, and the one that would
	// fail silently if the three writes were three calls.
	after := countRows(t, pool, runId, proposalId)
	if after != before {
		t.Errorf("a refused decision left rows behind: %+v then %+v", before, after)
	}
	var state string
	if err := pool.QueryRow(ctx,
		`SELECT approval_state FROM jetsapi.change_proposal WHERE proposal_id = $1`,
		proposalId).Scan(&state); err != nil {
		t.Fatalf("reading the proposal: %v", err)
	}
	if state != "approved" {
		t.Errorf("the refused decision changed the state to %q", state)
	}
}

type rowCounts struct{ Audit, Approvals int }

func countRows(t *testing.T, pool *pgxpool.Pool, runId, proposalId string) rowCounts {
	t.Helper()
	ctx := context.Background()
	var c rowCounts
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = $1`, runId).Scan(&c.Audit); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.approval_event WHERE subject_ref = $1`, proposalId).Scan(&c.Approvals); err != nil {
		t.Fatalf("counting approvals: %v", err)
	}
	return c
}

func TestApprovalRefusesWhatItCannotAttribute(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	base := func() *Approval {
		return &Approval{
			ApprovalEventId: "apv_x", RunRef: "run_x", SubjectRef: "chg_x",
			FromState: "draft", ToState: "approved", Actor: "a@b.c", TierAtEvent: "T2",
		}
	}
	for _, tc := range []struct {
		name   string
		break_ func(*Approval)
	}{
		{"no actor", func(a *Approval) { a.Actor = "" }},
		{"no tier", func(a *Approval) { a.TierAtEvent = "" }},
		{"no run", func(a *Approval) { a.RunRef = "" }},
		{"no subject", func(a *Approval) { a.SubjectRef = "" }},
		{"no from_state", func(a *Approval) { a.FromState = "" }},
		{"no event id", func(a *Approval) { a.ApprovalEventId = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := base()
			tc.break_(a)
			if _, err := RecordApproval(ctx, pool, a); err == nil {
				t.Fatal("recorded an approval that cannot be attributed")
			}
		})
	}
}

// TestApprovalStateVocabulary asserts the states this package uses against the
// generated CHECK, the way TestEventTypes does for event types: a vocabulary
// change in the model should fail here rather than in a production insert.
func TestApprovalStateVocabulary(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	runId, proposalId := seedProposal(t, pool, "vocab")
	_, err := RecordApproval(ctx, pool, &Approval{
		ApprovalEventId: "apv_k1_vocab_" + proposalId, RunRef: runId, SubjectRef: proposalId,
		FromState: "draft", ToState: "not_a_state", Actor: "a@b.c", TierAtEvent: "T2",
	})
	if err == nil {
		t.Fatal("the CHECK constraint accepted a state outside the vocabulary")
	}
	if _, ok := err.(*ErrStateConflict); ok {
		t.Fatalf("expected the CHECK to reject it, got a state conflict: %v", err)
	}
}
