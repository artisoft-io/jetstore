package main

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
)

// conditionFake scripts SessionIncidents and IncidentCondition.
type conditionFake struct {
	sessions    *SessionIncidents
	sessionsErr error
	askedFor    string

	condition    *IncidentCondition
	conditionWhy string
	conditionErr error
	conditionOf  []string
}

func (f *fakeOps) SessionIncidents(_ context.Context, session string) (*SessionIncidents, error) {
	f.condition.askedFor = session
	if f.condition.sessionsErr != nil {
		return nil, f.condition.sessionsErr
	}
	if f.condition.sessions == nil {
		return &SessionIncidents{SessionId: session}, nil
	}
	return f.condition.sessions, nil
}

func (f *fakeOps) IncidentCondition(_ context.Context, inc *audit.Incident) (*IncidentCondition, string, error) {
	f.condition.conditionOf = append(f.condition.conditionOf, inc.IncidentId)
	return f.condition.condition, f.condition.conditionWhy, f.condition.conditionErr
}

// find_session_incidents needs a session, maps an undeployed table to 503,
// and carries every match with how the session belongs to it.
func TestFindSessionIncidents(t *testing.T) {
	if _, code, err := dispatch(t, &fakeOps{}, &AgenticAction{Action: "find_session_incidents"}, "a@b"); err == nil ||
		code != http.StatusBadRequest {
		t.Errorf("no session: code %d, err %v; want 400", code, err)
	}

	ops := &fakeOps{condition: conditionFake{sessionsErr: &audit.ErrTablesNotDeployed{Detail: "x"}}}
	if _, code, _ := dispatch(t, ops, &AgenticAction{Action: "find_session_incidents", SessionId: "5"}, "a@b"); code != http.StatusServiceUnavailable {
		t.Errorf("tables not deployed: code %d, want 503", code)
	}

	ops = &fakeOps{condition: conditionFake{sessions: &SessionIncidents{SessionId: "run-5", InputSessions: []string{"5"},
		Matches: []SessionIncident{{SessionId: "5", Locus: "delivery_departs_profile", How: SessionIncidentCondition,
			IncidentId: "inc_1_delivery_departs_profile", IncidentSessionId: "1", Status: "diagnosed", Feed: "feed_a",
			Why: "it joins incident inc_1_delivery_departs_profile"}}}}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "find_session_incidents", SessionId: "run-5"}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d, err %v", code, err)
	}
	if ops.condition.askedFor != "run-5" {
		t.Errorf("the store was asked for %q", ops.condition.askedFor)
	}
	ms := res["matches"].([]map[string]any)
	if len(ms) != 1 || ms[0]["how"] != "condition" || ms[0]["incidentId"] != "inc_1_delivery_departs_profile" ||
		ms[0]["incidentSessionId"] != "1" || ms[0]["sessionId"] != "5" {
		t.Errorf("matches %v", ms)
	}
	if in := res["inputSessions"].([]string); len(in) != 1 || in[0] != "5" {
		t.Errorf("input sessions %v", in)
	}
}

// A feed incident carries its condition, or the reason it has none, or the
// error that kept it from being read, and stays readable in every case; a run
// incident asks for none.
func TestGetIncidentCarriesAFeedIncidentsCondition(t *testing.T) {
	feed := &audit.Incident{IncidentSummary: audit.IncidentSummary{IncidentId: "inc_1_delivery_departs_profile",
		SessionId: "1", Locus: "delivery_departs_profile", Status: audit.IncidentDiagnosed}}
	cond := &IncidentCondition{Feed: "feed_a", Locus: "delivery_departs_profile", StandsFor: 2, Open: true,
		Deliveries: []ConditionDelivery{
			{SessionId: "1", FileKey: "a.csv", Registered: true, Placement: string(feedcondition.AlreadyRaised),
				IncidentId: feed.IncidentId},
			{SessionId: "2", FileKey: "b.csv", Registered: true, Placement: string(feedcondition.Join),
				IncidentId: feed.IncidentId},
		},
		Previous: &feedcondition.Incident{IncidentId: "inc_0_delivery_departs_profile", SessionId: "0",
			Status: "suppressed_as_benign", Verdict: &audit.IncidentTransition{ToStatus: "suppressed_as_benign",
				Actor: "p@q"}}}
	ops := &fakeOps{incident: feed, condition: conditionFake{condition: cond}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: feed.IncidentId}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d, err %v", code, err)
	}
	c, ok := res["incident"].(map[string]any)["condition"].(map[string]any)
	if !ok {
		t.Fatalf("no condition: %v", res["incident"])
	}
	if len(c["deliveries"].([]map[string]any)) != 2 || c["standsFor"] != 2 || c["open"] != true {
		t.Errorf("condition %v", c)
	}
	prev := c["previous"].(map[string]any)["incident"].(map[string]any)
	if prev["incidentId"] != "inc_0_delivery_departs_profile" ||
		prev["verdict"].(map[string]any)["toStatus"] != "suppressed_as_benign" {
		t.Errorf("previous %v", prev)
	}

	ops = &fakeOps{incident: feed, condition: conditionFake{conditionWhy: "two feeds"}}
	res, _, _ = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: feed.IncidentId}, "a@b")
	if inc := res["incident"].(map[string]any); inc["conditionNote"] != "two feeds" || inc["condition"] != nil {
		t.Errorf("no condition: %v", inc)
	}

	ops = &fakeOps{incident: feed, condition: conditionFake{conditionErr: errors.New("no anomaly table")}}
	res, code, err = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: feed.IncidentId}, "a@b")
	if err != nil || code != http.StatusOK || res["incident"].(map[string]any)["conditionError"] != "no anomaly table" {
		t.Errorf("a condition that cannot be read: code %d, err %v, %v", code, err, res["incident"])
	}

	run := &audit.Incident{IncidentSummary: audit.IncidentSummary{IncidentId: "inc_2_worker_failed",
		SessionId: "2", Locus: "worker_failed", Status: audit.IncidentDiagnosed}}
	ops = &fakeOps{incident: run}
	res, _, _ = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: run.IncidentId}, "a@b")
	if _, ok := res["incident"].(map[string]any)["condition"]; ok || len(ops.condition.conditionOf) != 0 {
		t.Errorf("a run incident's condition was read: %v", res["incident"])
	}
}
