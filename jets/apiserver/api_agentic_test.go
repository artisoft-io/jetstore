package main

// The supervision endpoint's behaviour, driven through a fake store.
//
// **Two of these cannot be tested any other way, which is why the seam exists.**
// The state conflict needs two approvers racing on one row; the damaged
// transcript needs a chain the append-only trigger makes impossible to produce.
// Both are the cases the screens have to render correctly and both are the
// cases a live database will never hand a test on demand.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
)

// fakeOps is a scripted agenticOps. Every field is optional; an action that
// reaches an unset one gets a zero value, which is what an "unused in this
// test" arm should look like.
type fakeOps struct {
	proposals   []audit.ProposalSummary
	proposal    *audit.Proposal
	proposalErr error
	approvals   []audit.Approval
	transcript  *audit.Transcript
	tier        string
	tierErr     error
	recordErr   error
	// recorded captures what RecordApproval was handed, which is how the
	// server-side derivation of actor and tier is asserted.
	recorded *audit.Approval

	// The incident half (task AE.1). `incidentErr` is what makes the
	// unmigrated-database path testable here at all: the alternative is a
	// database with no schema, which the audit package tests and this one
	// cannot.
	incidents   []audit.IncidentSummary
	incident    *audit.Incident
	incidentErr error
	// statusFilter captures what ListIncidents was asked for, so a test can
	// assert the screen's filter reached the store rather than the wrong field.
	statusFilter []string
}

func (f *fakeOps) ListProposals(context.Context, []string, int) ([]audit.ProposalSummary, error) {
	return f.proposals, nil
}
func (f *fakeOps) ReadProposal(_ context.Context, id string) (*audit.Proposal, error) {
	if f.proposalErr != nil {
		return nil, f.proposalErr
	}
	if f.proposal == nil || f.proposal.ProposalId != id {
		return nil, &audit.ErrNoProposal{ProposalId: id}
	}
	return f.proposal, nil
}
func (f *fakeOps) ApprovalsFor(context.Context, string) ([]audit.Approval, error) {
	return f.approvals, nil
}
func (f *fakeOps) ReadTranscript(_ context.Context, runId string) (*audit.Transcript, error) {
	if f.transcript == nil {
		return nil, &audit.ErrNoTranscript{RunId: runId}
	}
	return f.transcript, nil
}
func (f *fakeOps) RunTier(context.Context, string) (string, error) {
	if f.tierErr != nil {
		return "", f.tierErr
	}
	return f.tier, nil
}
func (f *fakeOps) RecordApproval(_ context.Context, a *audit.Approval) (int, error) {
	f.recorded = a
	if f.recordErr != nil {
		return 0, f.recordErr
	}
	return 7, nil
}

func (f *fakeOps) ListIncidents(_ context.Context, statuses []string, _ int) ([]audit.IncidentSummary, error) {
	f.statusFilter = statuses
	if f.incidentErr != nil {
		return nil, f.incidentErr
	}
	return f.incidents, nil
}
func (f *fakeOps) ReadIncident(_ context.Context, id string) (*audit.Incident, error) {
	if f.incidentErr != nil {
		return nil, f.incidentErr
	}
	if f.incident == nil || f.incident.IncidentId != id {
		return nil, &audit.ErrNoIncident{IncidentId: id}
	}
	return f.incident, nil
}

func dispatch(t *testing.T, ops agenticOps, a *AgenticAction, actor string) (map[string]any, int, error) {
	t.Helper()
	res, code, err := agenticDispatch(context.Background(), ops, a, actor)
	if res == nil {
		return nil, code, err
	}
	return *res, code, err
}

// The capability constant and the seed file must name the same string, and the
// seed file must actually grant it to somebody. A capability nobody holds
// refuses everyone, which looks exactly like a bug in the screen.
func TestAgentSupervisionCapabilityIsSeeded(t *testing.T) {
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading jets_init_db.sql: %v", err)
	}
	grant := fmt.Sprintf("'%s')", AgentSupervisionCapability)
	if !strings.Contains(string(sql), grant) {
		t.Errorf("jets_init_db.sql grants no role the %q capability", AgentSupervisionCapability)
	}
}

func TestUnknownActionIsRefused(t *testing.T) {
	_, code, err := dispatch(t, &fakeOps{}, &AgenticAction{Action: "delete_everything"}, "a@b")
	if err == nil {
		t.Fatal("an unknown action was accepted")
	}
	if code != http.StatusBadRequest {
		t.Errorf("code %d, want 400", code)
	}
}

func TestListProposalsCarriesTheTransitionsPerRow(t *testing.T) {
	ops := &fakeOps{proposals: []audit.ProposalSummary{
		{ProposalId: "chg_1", ApprovalState: audit.StateDraft},
		{ProposalId: "chg_2", ApprovalState: audit.StateRejected},
	}}
	res, _, err := dispatch(t, ops, &AgenticAction{Action: "list_proposals"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	rows := res["proposals"].([]map[string]any)
	if len(rows) != 2 {
		t.Fatalf("got %d rows", len(rows))
	}
	if got := rows[0]["transitions"].([]string); len(got) != 2 {
		t.Errorf("a draft should offer validated and superseded, got %v", got)
	}
	// A terminal state must serialise as [] rather than null, so the screen has
	// no branch between "no buttons" and "unknown".
	if got := rows[1]["transitions"].([]string); got == nil || len(got) != 0 {
		t.Errorf("a rejected proposal should offer an empty list, got %v", got)
	}
}

func TestGetProposalOnAMissingIdIs404(t *testing.T) {
	_, code, err := dispatch(t, &fakeOps{}, &AgenticAction{Action: "get_proposal", ProposalId: "chg_nope"}, "a@b")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code != http.StatusNotFound {
		t.Errorf("code %d, want 404", code)
	}
}

func TestGetProposalReturnsItsDecisionsAndItsNextStates(t *testing.T) {
	ops := &fakeOps{
		proposal: &audit.Proposal{
			ProposalId: "chg_1", Trigger: "authoring_run", TriggerRef: "run_1",
			ApprovalState: audit.StateAwaitingHumanApproval,
		},
		approvals: []audit.Approval{{
			ApprovalEventId: "apr_1", FromState: audit.StateDraft, ToState: audit.StateValidated,
			Actor: "reviewer@x", TierAtEvent: "T1", DecidedAt: time.Now().UTC(),
		}},
	}
	res, _, err := dispatch(t, ops, &AgenticAction{Action: "get_proposal", ProposalId: "chg_1"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res["approvals"].([]map[string]any)); got != 1 {
		t.Errorf("got %d approvals, want 1", got)
	}
	if got := res["transitions"].([]string); len(got) != 4 {
		t.Errorf("awaiting_human_approval offers four, got %v", got)
	}
	if res["terminal"].(bool) {
		t.Error("awaiting_human_approval is not terminal")
	}
}

// K.2's decision, enforced at the wire: there is no way to obtain the events
// without the verdict on them. A viewer that renders a tamper-evident log
// without checking the evidence offers the appearance of the property.
func TestTranscriptCarriesItsVerdictAndItsDefects(t *testing.T) {
	ops := &fakeOps{transcript: &audit.Transcript{
		RunId: "run_1",
		Events: []audit.TranscriptEvent{
			{Seq: 1, EventType: "intent", Actor: "agent", Payload: json.RawMessage(`{"a":1}`)},
			{Seq: 2, EventType: "tool_call", Actor: "agent", Payload: json.RawMessage(`{}`)},
		},
		Defects: []audit.ChainDefect{{Seq: 2, Kind: audit.DefectHash, Detail: "row_hash does not recompute"}},
	}}
	res, _, err := dispatch(t, ops, &AgenticAction{Action: "read_transcript", RunId: "run_1"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	if res["verified"].(bool) {
		t.Error("a transcript with a defect reported itself verified")
	}
	if got := len(res["defects"].([]map[string]any)); got != 1 {
		t.Errorf("got %d defects, want 1", got)
	}
	// And the events still come back: a damaged transcript is still the thing
	// you want to look at, so refusing to render it would destroy the evidence.
	if got := len(res["events"].([]map[string]any)); got != 2 {
		t.Errorf("got %d events, want 2", got)
	}
}

func TestTranscriptOfAnUnknownRunIs404(t *testing.T) {
	_, code, err := dispatch(t, &fakeOps{}, &AgenticAction{Action: "read_transcript", RunId: "run_nope"}, "a@b")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code != http.StatusNotFound {
		t.Errorf("code %d, want 404", code)
	}
}

func awaitingProposal() *audit.Proposal {
	return &audit.Proposal{
		ProposalId: "chg_1", TriggerRef: "run_1",
		ApprovalState: audit.StateAwaitingHumanApproval,
	}
}

// The actor is the authenticated user and the tier comes from the run. Neither
// is readable off the request, and this is the assertion that keeps it that
// way: the action below carries no actor and no tier and could not.
func TestApprovalIsSignedByTheSessionAndTieredByTheRun(t *testing.T) {
	ops := &fakeOps{proposal: awaitingProposal(), tier: "T2"}
	res, _, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1",
		FromState: audit.StateAwaitingHumanApproval, ToState: audit.StateApproved,
		Rationale: "checked the diff",
	}, "supervisor@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if ops.recorded.Actor != "supervisor@example.com" {
		t.Errorf("actor %q, want the session's user", ops.recorded.Actor)
	}
	if ops.recorded.TierAtEvent != "T2" {
		t.Errorf("tier %q, want the run's T2", ops.recorded.TierAtEvent)
	}
	if ops.recorded.RunRef != "run_1" {
		t.Errorf("run_ref %q, want the originating run", ops.recorded.RunRef)
	}
	if res["auditSeq"].(int) != 7 {
		t.Errorf("the chain seq was not returned: %v", res["auditSeq"])
	}
}

func TestApprovalWithNoAuthenticatedActorIsRefused(t *testing.T) {
	ops := &fakeOps{proposal: awaitingProposal(), tier: "T2"}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "")
	if err == nil {
		t.Fatal("an unattributable approval was accepted")
	}
	if code != http.StatusUnauthorized {
		t.Errorf("code %d, want 401", code)
	}
	if ops.recorded != nil {
		t.Error("it reached the store anyway")
	}
}

// The lifecycle is enforced on the server. A client that offers `approved` from
// `draft` — a stale screen, or a hand-written request — is refused, and told
// what is permitted rather than only that it was wrong.
func TestAnIllegalTransitionIsRefusedWithWhatIsPermitted(t *testing.T) {
	ops := &fakeOps{
		proposal: &audit.Proposal{ProposalId: "chg_1", TriggerRef: "run_1", ApprovalState: audit.StateDraft},
		tier:     "T2",
	}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "a@b")
	if err == nil {
		t.Fatal("draft -> approved was accepted")
	}
	if code != http.StatusBadRequest {
		t.Errorf("code %d, want 400", code)
	}
	if !strings.Contains(err.Error(), audit.StateValidated) {
		t.Errorf("the refusal does not name what is permitted: %v", err)
	}
	if ops.recorded != nil {
		t.Error("it reached the store anyway")
	}
}

// A screen that has been open while somebody else decided sends a stale
// fromState. It is caught before the write, against the row, and reported as a
// conflict — the one failure the screen recovers from by re-reading.
func TestAStaleFromStateIsAConflictBeforeTheWrite(t *testing.T) {
	ops := &fakeOps{proposal: awaitingProposal(), tier: "T2"}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1",
		FromState: audit.StateDraft, ToState: audit.StateValidated,
	}, "a@b")
	if err == nil {
		t.Fatal("a stale fromState was accepted")
	}
	if code != http.StatusConflict {
		t.Errorf("code %d, want 409", code)
	}
	var conflict *audit.ErrStateConflict
	if !errors.As(err, &conflict) {
		t.Errorf("the error is not an ErrStateConflict: %T", err)
	}
	if ops.recorded != nil {
		t.Error("it reached the store anyway")
	}
}

// And the race the read cannot close: two approvers pass the check and one of
// them loses at the UPDATE. K.1 made that a distinct error so a caller could
// act on it; this is where it becomes a status code the browser can branch on.
func TestTheLostRaceReachesTheClientAs409(t *testing.T) {
	ops := &fakeOps{
		proposal:  awaitingProposal(),
		tier:      "T2",
		recordErr: &audit.ErrStateConflict{SubjectRef: "chg_1", Expected: audit.StateAwaitingHumanApproval},
	}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "a@b")
	if err == nil {
		t.Fatal("expected the conflict to surface")
	}
	if code != http.StatusConflict {
		t.Errorf("code %d, want 409 — a 500 would tell the screen to give up rather than re-read", code)
	}
}

// A proposal whose originating run is not in agent_run has no honest tier, so
// the decision is refused rather than recorded at a guessed authority.
func TestAProposalWithNoRunCannotBeDecided(t *testing.T) {
	ops := &fakeOps{proposal: awaitingProposal(), tierErr: &audit.ErrNoRun{RunId: "run_1"}}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "a@b")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code != http.StatusConflict {
		t.Errorf("code %d, want 409", code)
	}
	if ops.recorded != nil {
		t.Error("it reached the store anyway")
	}
}

// The incident half (task AE.1) --------------------------------------------
//
// **What is worth asserting here is what the endpoint refuses to lose**, on the
// same argument as the proposal tests above: both taxonomies travel together,
// an unclaimed cause survives as an empty string rather than becoming one, a
// shard that is absent stays absent, and an unmigrated database is a different
// answer from an outage.

func sampleIncident() audit.IncidentSummary {
	shard := int64(0)
	return audit.IncidentSummary{
		IncidentId: "inc_1", SessionId: "sess_1", DetectedAt: time.Unix(1757000000, 0).UTC(),
		Locus: audit.LocusWorkerFailed, Classification: "", Severity: "high", Status: "triaged",
		StepRef: "reducing00", ShardRef: &shard,
		Confounders: []string{"step_label_ambiguous"}, ModelVersion: "0.1.0", HypothesisCount: 1,
	}
}

func TestListIncidentsCarriesBothTaxonomiesAndTheVocabulary(t *testing.T) {
	ops := &fakeOps{incidents: []audit.IncidentSummary{sampleIncident()}}
	res, code, err := dispatch(t, ops, &AgenticAction{
		Action: "list_incidents", Statuses: []string{"triaged"},
	}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	if got := ops.statusFilter; len(got) != 1 || got[0] != "triaged" {
		t.Errorf("the status filter reached the store as %v", got)
	}
	rows := res["incidents"].([]map[string]any)
	if len(rows) != 1 {
		t.Fatalf("got %d rows", len(rows))
	}
	row := rows[0]
	// The row carries both columns. A response with only one of them is R-27
	// arriving on the wire: a locus rendered under a heading that says cause.
	if row["locus"] != audit.LocusWorkerFailed {
		t.Errorf("locus %v", row["locus"])
	}
	if _, ok := row["classification"]; !ok {
		t.Error("the row has no classification key at all; an unclaimed cause must be visibly empty")
	}
	if row["classification"] != "" {
		t.Errorf("classification %v, want empty", row["classification"])
	}
	if shard, ok := row["shardRef"].(*int64); !ok || shard == nil || *shard != 0 {
		t.Errorf("shardRef %v, want a pointer to 0 — shard 0 is a shard", row["shardRef"])
	}
	// The nine loci come from the server so the screen's legend cannot drift
	// from the CHECK the rows are written against.
	if loci, ok := res["loci"].([]string); !ok || len(loci) != 9 {
		t.Errorf("loci %v, want the nine", res["loci"])
	}
	if st, ok := res["incidentStatuses"].([]string); !ok || len(st) != 11 {
		t.Errorf("incidentStatuses %v, want the eleven", res["incidentStatuses"])
	}
}

func TestGetIncidentReturnsBothEvidenceSides(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{
		IncidentSummary: sampleIncident(),
		Hypotheses: []audit.Hypothesis{{
			HypothesisId: "hyp_1", IncidentRef: "inc_1", Cause: "a step regressed",
			CauseCategory: "", Confidence: 0.6, Rank: 1,
			SupportingEvidence:    []audit.Evidence{{Statement: "slower", Source: "run_telemetry"}},
			ContradictingEvidence: []audit.Evidence{},
		}},
	}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: "inc_1"}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	inc := res["incident"].(map[string]any)
	hs := inc["hypotheses"].([]map[string]any)
	if len(hs) != 1 {
		t.Fatalf("got %d hypotheses", len(hs))
	}
	// An empty contradicting array must arrive as an array. A hypothesis that
	// asserts nothing against itself and one that was never asked are different
	// claims, and A.2.8 calls the difference a calibration control.
	got, ok := hs[0]["contradictingEvidence"].([]map[string]any)
	if !ok || got == nil {
		t.Fatalf("contradictingEvidence is %T, want an array", hs[0]["contradictingEvidence"])
	}
	if len(got) != 0 {
		t.Errorf("contradictingEvidence %v, want empty", got)
	}
	if len(hs[0]["supportingEvidence"].([]map[string]any)) != 1 {
		t.Error("the supporting side was lost")
	}
}

func TestAnUnknownIncidentIs404(t *testing.T) {
	_, code, err := dispatch(t, &fakeOps{}, &AgenticAction{Action: "get_incident", IncidentId: "nope"}, "a@b")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if code != http.StatusNotFound {
		t.Errorf("code %d, want 404", code)
	}
}

// **The one a live database will not hand a test on demand**, and the reason
// ErrTablesNotDeployed is a type rather than a message: on every deployment
// older than AB.1 these tables are absent, and a 500 would put a missing
// migration in the same bucket as an outage. 503 says the deployment is not
// ready, which is both true and actionable.
func TestAnUnmigratedDatabaseIs503RatherThan500(t *testing.T) {
	ops := &fakeOps{incidentErr: &audit.ErrTablesNotDeployed{Detail: `relation "jetsapi.incident" does not exist`}}
	for _, action := range []*AgenticAction{
		{Action: "list_incidents"},
		{Action: "get_incident", IncidentId: "inc_1"},
	} {
		_, code, err := dispatch(t, ops, action, "a@b")
		if err == nil {
			t.Fatalf("%s: expected the failure to surface", action.Action)
		}
		if code != http.StatusServiceUnavailable {
			t.Errorf("%s: code %d, want 503", action.Action, code)
		}
		if !strings.Contains(err.Error(), "migrateDb") {
			t.Errorf("%s: the error should name the remedy; got %v", action.Action, err)
		}
	}
}
