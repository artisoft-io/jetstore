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
	// AC.3's incident transition history; nil is a legitimate answer.
	transitions    []audit.IncidentTransition
	transitionsErr error

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

	// phiSeen captures the PHI decision ReadIncident was handed (task AE.2).
	// **The redaction itself is not testable here**: this fake stands where the
	// audit package's read would be, and that read is where the withholding
	// happens — which is the design rather than a limitation of the fake. What
	// this endpoint owes is that the capability decision reaches the read
	// unaltered, and that is what phiSeen is for.
	phiSeen  audit.PHIAccess
	phiAsked bool

	// The verdict write (task AJ.2, Q-52).
	recordedVerdict             *audit.IncidentTransition
	verdictErr                  error
	verdictClassificationBefore string
	verdictRunRef               string
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
func (f *fakeOps) ReadIncident(_ context.Context, id string, phi audit.PHIAccess) (*audit.Incident, error) {
	f.phiSeen = phi
	f.phiAsked = true
	if f.incidentErr != nil {
		return nil, f.incidentErr
	}
	if f.incident == nil || f.incident.IncidentId != id {
		return nil, &audit.ErrNoIncident{IncidentId: id}
	}
	return f.incident, nil
}

// dispatch drives one action. **The PHI access defaults to RedactPHI and is
// passed explicitly by the tests that are about it** — deliberately the safe
// value, on the same argument the production signature makes: a helper that
// disclosed by default would let a future test inherit disclosure by writing
// nothing, which is the failure AE.2 exists to close one layer down.
func dispatch(t *testing.T, ops agenticOps, a *AgenticAction, actor string, phi ...audit.PHIAccess) (map[string]any, int, error) {
	t.Helper()
	access := audit.RedactPHI
	if len(phi) > 0 {
		access = phi[0]
	}
	res, code, err := agenticDispatch(context.Background(), ops, a, actor, access)
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

// AB.4, Q-32. The run reference is what decides whether a transition on this
// incident reaches the hash chain, so a supervision surface that dropped it on
// the way to the browser would leave the screen unable to say so.
func TestIncidentRowCarriesTheRunReference(t *testing.T) {
	sample := sampleIncident()
	sample.RunRef = "run_abc"
	ops := &fakeOps{incidents: []audit.IncidentSummary{sample}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "list_incidents"}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	rows := res["incidents"].([]map[string]any)
	if rows[0]["runRef"] != "run_abc" {
		t.Errorf("runRef reached the wire as %v", rows[0]["runRef"])
	}
	// "" rather than absent for an incident nothing agentic raised, which is
	// the ordinary case out of AC.1 and is a value the screen renders words for.
	ops2 := &fakeOps{incidents: []audit.IncidentSummary{sampleIncident()}}
	res2, _, err := dispatch(t, ops2, &AgenticAction{Action: "list_incidents"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	rows2 := res2["incidents"].([]map[string]any)
	if got, ok := rows2[0]["runRef"]; !ok || got != "" {
		t.Errorf("an incident with no run reached the wire as %v (present: %v)", got, ok)
	}
}

// AE.2, I-311. Two claims, and they are separate: the capability decision has to
// reach the read unchanged, and the caller has to be told what was withheld
// rather than being left to infer it from a blank statement.
func TestPHIAccessReachesTheReadAndIsReportedToTheCaller(t *testing.T) {
	for _, tc := range []struct {
		name     string
		phi      audit.PHIAccess
		redacted bool
	}{
		{"without the capability", audit.RedactPHI, true},
		{"with the capability", audit.DisclosePHI, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
			res, code, err := dispatch(t, ops,
				&AgenticAction{Action: "get_incident", IncidentId: "inc_1"}, "a@b", tc.phi)
			if err != nil || code != http.StatusOK {
				t.Fatalf("code %d err %v", code, err)
			}
			if !ops.phiAsked {
				t.Fatal("the read was never asked for a PHI decision")
			}
			if ops.phiSeen != tc.phi {
				t.Errorf("the read was handed %v, want %v", ops.phiSeen, tc.phi)
			}
			if res["phiRedacted"] != tc.redacted {
				t.Errorf("phiRedacted %v, want %v", res["phiRedacted"], tc.redacted)
			}
			if res["phiCapability"] != AgentPHIAccessCapability {
				t.Errorf("phiCapability %v", res["phiCapability"])
			}
			// The classified properties come from the generated manifest, so a
			// second marked property reaches the screen with no edit here.
			props, ok := res["phiProperties"].([]string)
			if !ok || len(props) != 1 || props[0] != "statement (PHI)" {
				t.Errorf("phiProperties %v", res["phiProperties"])
			}
		})
	}
}

// The grant was taken by the user on 2026-09-04 (Q-42): knowledge_engineer, the
// role jets_init_db.sql calls a super user and the only seeded role holding
// agent_supervision. This test is the one the earlier version said would have to
// be edited, and it is edited rather than deleted, because what it asserts is
// still worth asserting: the capability is documented, and exactly one role holds
// it.
//
// **What the grant costs is worth stating where somebody will read it.** In the
// seeded deployment the set of principals who can supervise agents and the set
// who can read PHI are now the same set, because knowledge_engineer is the only
// member of the first. So I-311's floor — a PHI-marked field is not covered by
// agent_supervision alone — is true in the code and has no effect in the default
// seed. It is not vacuous: it is what a *second* role granted agent_supervision
// and not agent_phi_access will meet, and a deployment that wants supervision
// without disclosure now has a mechanism rather than a request.
func TestAgentPHIAccessCapabilityIsGrantedToExactlyKnowledgeEngineer(t *testing.T) {
	sql, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading jets_init_db.sql: %v", err)
	}
	if !strings.Contains(string(sql), fmt.Sprintf("- %s:", AgentPHIAccessCapability)) {
		t.Errorf("jets_init_db.sql does not document the %q capability", AgentPHIAccessCapability)
	}
	grant := fmt.Sprintf("('knowledge_engineer', '%s')", AgentPHIAccessCapability)
	if !strings.Contains(string(sql), grant) {
		t.Errorf("jets_init_db.sql does not grant %q to knowledge_engineer; the user took that "+
			"decision on 2026-09-04 (Q-42)", AgentPHIAccessCapability)
	}
	if n := strings.Count(string(sql), fmt.Sprintf("'%s')", AgentPHIAccessCapability)); n != 1 {
		t.Errorf("jets_init_db.sql grants %q to %d roles, expected exactly 1 (knowledge_engineer). "+
			"Widening it is a policy decision (Q-42): take it deliberately and update this test",
			AgentPHIAccessCapability, n)
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

// IncidentTransitionsFor completes the agenticOps interface for the fake store
// (task AC.3). The scripted value is `transitions`; nil is a legitimate answer
// and is what a database migrated between AB.1 and AB.2 would give.
func (f *fakeOps) IncidentTransitionsFor(context.Context, string) ([]audit.IncidentTransition, error) {
	return f.transitions, f.transitionsErr
}

// RecordIncidentTransition is the verdict write (task AJ.2). `recordedVerdict`
// captures what the handler composed, which is how the two things the request
// may not carry — the actor and the actor **kind** — are asserted to have been
// derived server-side.
func (f *fakeOps) RecordIncidentTransition(_ context.Context, t *audit.IncidentTransition) (int, error) {
	f.recordedVerdict = t
	if f.verdictErr != nil {
		return 0, f.verdictErr
	}
	// The classification the row carried before the write is an output of the
	// real function, read off the locked row. The fake supplies it so a test
	// can assert the pair reaches the response rather than being dropped.
	t.ClassificationBefore = f.verdictClassificationBefore
	t.RunRef = f.verdictRunRef
	if t.RunRef == "" {
		// 0 seq is what an incident naming no run gets, and it is not an error.
		return 0, nil
	}
	return 11, nil
}

// The verdict control's server half (task AJ.2, Q-52) — the single change that
// gives `HumanVerdicts` a writer.
//
// **Every test below is about something the request may not decide.** The
// endpoint's own argument for `record_approval` is that a caller who could name
// its own actor could sign a decision as somebody else; a caller who could name
// its own actor *kind* could write itself into the labelled population, which is
// worse, because the population is the instrument.

func TestAVerdictIsRecordedAsAHumanAndTheRequestCannotSayOtherwise(t *testing.T) {
	ops := &fakeOps{
		incident:                    &audit.Incident{IncidentSummary: sampleIncident()},
		verdictClassificationBefore: "",
		verdictRunRef:               "run_1",
	}
	res, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1",
		FromStatus: "triaged", ToStatus: audit.IncidentReclassified,
		Classification: "transformation_defect",
		Rationale:      "the worker message names a mapping error, not a source failure",
	}, "michel@artisoft.io")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	got := ops.recordedVerdict
	if got == nil {
		t.Fatal("nothing reached RecordIncidentTransition")
	}
	if got.ActorKind != audit.ActorHuman {
		t.Errorf("actor kind recorded as %q; a verdict written through this endpoint is a person's", got.ActorKind)
	}
	if got.Actor != "michel@artisoft.io" {
		t.Errorf("actor recorded as %q, want the authenticated user", got.Actor)
	}
	if got.FromStatus != "triaged" {
		t.Errorf("from_status recorded as %q; it is read off the row", got.FromStatus)
	}
	if got.IncidentEventId == "" {
		t.Error("the event id was not minted server-side")
	}
	if res["actorKind"] != audit.ActorHuman {
		t.Errorf("the response says actorKind %v", res["actorKind"])
	}
	if res["auditSeq"] != 11 {
		t.Errorf("auditSeq = %v; an incident naming a run is chained", res["auditSeq"])
	}
	if res["classificationAfter"] != "transformation_defect" {
		t.Errorf("classificationAfter = %v", res["classificationAfter"])
	}
	// The two ends of the label come back together — a pairing recovered later
	// from a mutable column is one the next correction destroys (P4 §12.4).
	if _, ok := res["classificationBefore"]; !ok {
		t.Error("the response drops classificationBefore, which is half the label")
	}
}

// A verdict on an incident nothing agentic raised is durable, attributable and
// not hash-chained (AB.4, R-44). Seq 0 is that case and is not an error.
func TestAVerdictOnAnIncidentWithNoRunIsRecordedAndNotChained(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
	res, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1",
		FromStatus: "triaged", ToStatus: audit.IncidentSuppressedAsBenign,
		Rationale: "a known seasonal dip",
	}, "michel@artisoft.io")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	if res["auditSeq"] != 0 {
		t.Errorf("auditSeq = %v, want 0", res["auditSeq"])
	}
	if res["runRef"] != "" {
		t.Errorf("runRef = %v, want empty", res["runRef"])
	}
}

func TestAVerdictNeedsAnAuthenticatedActor(t *testing.T) {
	_, code, err := dispatch(t, &fakeOps{}, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1", ToStatus: "verified",
	}, "")
	if err == nil || code != http.StatusUnauthorized {
		t.Fatalf("code %d err %v, want 401", code, err)
	}
}

// A.5's graph is enforced at the endpoint as well as in the primitive, so the
// message can name the permitted set rather than the constraint that fired.
func TestAnIllegalVerdictEdgeIsRefused(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1",
		FromStatus: "triaged", ToStatus: "closed",
	}, "michel@artisoft.io")
	if err == nil || code != http.StatusBadRequest {
		t.Fatalf("code %d err %v, want 400", code, err)
	}
	if !strings.Contains(err.Error(), "not a permitted transition") {
		t.Errorf("message = %v", err)
	}
	if ops.recordedVerdict != nil {
		t.Error("the write was attempted after the edge was refused")
	}
}

// A screen that has gone stale gets a 409 rather than overwriting somebody
// else's verdict — recordApproval's rule, applied to the second act of the same
// shape.
func TestAVerdictAgainstAStaleStatusIsAConflict(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1",
		FromStatus: "detected", ToStatus: "triaged",
	}, "michel@artisoft.io")
	if code != http.StatusConflict {
		t.Fatalf("code %d err %v, want 409", code, err)
	}
	var conflict *audit.ErrIncidentStateConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("error = %v, want an *ErrIncidentStateConflict", err)
	}
	if conflict.Found != "triaged" {
		t.Errorf("the conflict says found %q", conflict.Found)
	}
}

func TestAVerdictNamingAnUnknownClassificationIsRefused(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_1",
		FromStatus: "triaged", ToStatus: audit.IncidentReclassified,
		Classification: "operator_error", Rationale: "because",
	}, "michel@artisoft.io")
	if err == nil || code != http.StatusBadRequest {
		t.Fatalf("code %d err %v, want 400", code, err)
	}
	if !strings.Contains(err.Error(), "not an incident classification") {
		t.Errorf("message = %v", err)
	}
}

func TestAVerdictOnAMissingIncidentIs404(t *testing.T) {
	_, code, _ := dispatch(t, &fakeOps{}, &AgenticAction{
		Action: "record_incident_transition", IncidentId: "inc_nope", ToStatus: "verified",
	}, "michel@artisoft.io")
	if code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", code)
	}
}

// The detail response carries what may be done next and what a reclassification
// may claim, both from the server (AJ.2).
func TestGetIncidentCarriesThePermittedTransitionsAndTheCauses(t *testing.T) {
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: sampleIncident()}}
	res, code, err := dispatch(t, ops,
		&AgenticAction{Action: "get_incident", IncidentId: "inc_1"}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	permitted, ok := res["permittedTransitions"].([]string)
	if !ok {
		t.Fatalf("permittedTransitions = %#v", res["permittedTransitions"])
	}
	// `triaged` is the only status with three successors and all three are
	// adjudications — which is why plan §10.7 calls this screen the labelling
	// instrument.
	if len(permitted) != 3 {
		t.Errorf("permittedTransitions = %v, want the three adjudications of `triaged`", permitted)
	}
	causes, ok := res["classifications"].([]string)
	if !ok || len(causes) != len(audit.IncidentClassifications) {
		t.Errorf("classifications = %#v", res["classifications"])
	}
}

// A terminal status offers nothing, as an empty array rather than null — the
// argument transitionsOrEmpty already makes for proposals.
func TestGetIncidentOffersNothingOnATerminalStatus(t *testing.T) {
	inc := sampleIncident()
	inc.Status = audit.IncidentClosed
	ops := &fakeOps{incident: &audit.Incident{IncidentSummary: inc}}
	res, _, err := dispatch(t, ops,
		&AgenticAction{Action: "get_incident", IncidentId: "inc_1"}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	if got := res["permittedTransitions"].([]string); len(got) != 0 {
		t.Errorf("a closed incident offers %v", got)
	}
}
