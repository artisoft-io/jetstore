package main

import (
	"net/http"
	"slices"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
)

// An incident-triggered proposal is decided by the session's user at the
// runless tier, with run_ref holding the incident, and joins no chain. The run
// is never asked for its tier: tierErr would refuse the decision if it were.
func TestAnIncidentTriggeredProposalIsDecidedWithoutARun(t *testing.T) {
	for _, to := range []string{audit.StateApproved, audit.StateRejected} {
		ops := &fakeOps{
			proposal: &audit.Proposal{ProposalId: "chg_1", Trigger: audit.TriggerIncident,
				TriggerRef: "inc_1_delivery_departs_profile", ApprovalState: audit.StateAwaitingHumanApproval},
			tierErr: &audit.ErrNoRun{RunId: "inc_1_delivery_departs_profile"},
		}
		res, code, err := dispatch(t, ops, &AgenticAction{
			Action: "record_approval", ProposalId: "chg_1",
			FromState: audit.StateAwaitingHumanApproval, ToState: to, Rationale: "read the evidence",
		}, "person@example.com")
		if err != nil || code != http.StatusOK {
			t.Fatalf("to %s: code %d err %v", to, code, err)
		}
		r := ops.recorded
		if r.Actor != "person@example.com" || r.TierAtEvent != audit.RunlessDecisionTier ||
			r.RunRef != "inc_1_delivery_departs_profile" || !r.Unchained || r.ToState != to {
			t.Errorf("recorded %+v", r)
		}
		if res["chained"] != false {
			t.Errorf("chained = %v, want false", res["chained"])
		}
	}
}

// Nothing but an authenticated person decides one.
func TestAnIncidentTriggeredProposalNeedsAnAuthenticatedActor(t *testing.T) {
	ops := &fakeOps{proposal: &audit.Proposal{ProposalId: "chg_1", Trigger: audit.TriggerIncident,
		TriggerRef: "inc_1", ApprovalState: audit.StateDraft}}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateValidated,
	}, "")
	if err == nil || code != http.StatusUnauthorized || ops.recorded != nil {
		t.Errorf("code %d err %v recorded %v", code, err, ops.recorded)
	}
}

// A run-triggered proposal takes the path it always took: the tier from the
// run, run_ref the run, and a chain.
func TestARunTriggeredProposalIsStillTieredByItsRun(t *testing.T) {
	p := awaitingProposal()
	p.Trigger = "authoring_run"
	ops := &fakeOps{proposal: p, tier: "T2"}
	res, _, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "a@b")
	if err != nil {
		t.Fatal(err)
	}
	if ops.recorded.TierAtEvent != "T2" || ops.recorded.RunRef != "run_1" || ops.recorded.Unchained {
		t.Errorf("recorded %+v", ops.recorded)
	}
	if res["chained"] != true {
		t.Errorf("chained = %v, want true", res["chained"])
	}
	// And one with no run is refused as before.
	ops = &fakeOps{proposal: p, tierErr: &audit.ErrNoRun{RunId: "run_1"}}
	if _, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateApproved,
	}, "a@b"); err == nil || code != http.StatusConflict {
		t.Errorf("code %d err %v", code, err)
	}
}

// A person rejects an incident-triggered draft where it stands, and is offered
// the edge; a run-triggered draft is neither offered it nor allowed it.
func TestAnIncidentTriggeredDraftMayBeRejected(t *testing.T) {
	ops := &fakeOps{
		proposal: &audit.Proposal{ProposalId: "chg_1", Trigger: audit.TriggerIncident,
			TriggerRef: "inc_1", ApprovalState: audit.StateDraft},
		tierErr: &audit.ErrNoRun{RunId: "inc_1"},
	}
	if got := transitionsOrEmpty(audit.StateDraft, true); !slices.Contains(got, audit.StateRejected) {
		t.Errorf("runless draft offered %v", got)
	}
	_, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1",
		FromState: audit.StateDraft, ToState: audit.StateRejected, Rationale: "not a real departure",
	}, "person@example.com")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d err %v", code, err)
	}
	if r := ops.recorded; r.FromState != audit.StateDraft || r.ToState != audit.StateRejected || !r.Unchained {
		t.Errorf("recorded %+v", r)
	}

	p := awaitingProposal()
	p.Trigger = "authoring_run"
	p.ApprovalState = audit.StateDraft
	ops = &fakeOps{proposal: p, tier: "T2"}
	if got := transitionsOrEmpty(audit.StateDraft, false); slices.Contains(got, audit.StateRejected) {
		t.Errorf("run-produced draft offered %v", got)
	}
	if _, code, err := dispatch(t, ops, &AgenticAction{
		Action: "record_approval", ProposalId: "chg_1", ToState: audit.StateRejected,
	}, "a@b"); err == nil || code != http.StatusBadRequest || ops.recorded != nil {
		t.Errorf("code %d err %v recorded %v", code, err, ops.recorded)
	}
}
