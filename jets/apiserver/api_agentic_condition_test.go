package main

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
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

	run    *RunIncidentDetail
	runErr error
	runOf  []string
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

func (f *fakeOps) RunIncident(_ context.Context, inc *audit.Incident) (*RunIncidentDetail, error) {
	f.condition.runOf = append(f.condition.runOf, inc.IncidentId)
	return f.condition.run, f.condition.runErr
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
// incident asks for its run condition instead (TestGetIncidentCarriesARunIncidentsCondition).
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

	if len(ops.condition.runOf) != 0 {
		t.Errorf("a feed incident's run condition was read: %v", ops.condition.runOf)
	}
}

// A run incident reads its run condition, as a feed incident reads its feed
// condition: the runs still held that are placed in it, from its position to
// its latest member, with its error code and its run's codes where it has
// them. It reads no feed condition. A condition holding the incident's own run
// alone adds nothing, and one that cannot be read leaves the incident readable.
func TestGetIncidentCarriesARunIncidentsCondition(t *testing.T) {
	run := &audit.Incident{IncidentSummary: audit.IncidentSummary{
		IncidentId: "inc_2_per_record_failures_reported_amount_not_numeric",
		SessionId:  "2", Locus: "per_record_failures_reported", Status: audit.IncidentDiagnosed}}
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	rows := func(n int64) *int64 { return &n }
	key := runcondition.Key{Client: "client_a", Process: "loader", Locus: run.Locus, Code: "amount not numeric"}
	detail := &RunIncidentDetail{
		Key:   RunIncidentKey{ErrorCode: key.Code, Client: key.Client, Process: key.Process, Anchor: t0},
		Codes: []RunCode{{Code: key.Code, Verdict: "present", StoredRows: 7}, {Code: "other", Verdict: "quiet", StoredRows: 1}},
		Condition: &IncidentRunCondition{Key: key, FirstSeen: t0, LastSeen: t0.Add(48 * time.Hour),
			Runs: []RunConditionRun{{SessionId: "2", StartTime: t0, StoredRows: rows(7)},
				{SessionId: "4", StartTime: t0.Add(48 * time.Hour), StoredRows: rows(3)}},
			ConditionRuns: 2, OwnRunHeld: true, Open: true},
	}
	ops := &fakeOps{incident: run, condition: conditionFake{run: detail}}
	res, code, err := dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: run.IncidentId}, "a@b")
	if err != nil || code != http.StatusOK {
		t.Fatalf("code %d, err %v", code, err)
	}
	inc := res["incident"].(map[string]any)
	if len(ops.condition.conditionOf) != 0 || len(ops.condition.runOf) != 1 {
		t.Errorf("a run incident read feed conditions %v and run conditions %v", ops.condition.conditionOf,
			ops.condition.runOf)
	}
	if inc["errorCode"] != key.Code {
		t.Errorf("errorCode %v, want %q", inc["errorCode"], key.Code)
	}
	if cs := inc["codes"].([]map[string]any); len(cs) != 2 || cs[0]["storedRows"] != int64(7) ||
		cs[1]["verdict"] != "quiet" {
		t.Errorf("codes %v", cs)
	}
	c, ok := inc["runCondition"].(map[string]any)
	if !ok {
		t.Fatalf("no run condition: %v", inc)
	}
	if c["runsHeld"] != 2 || c["process"] != "loader" || c["client"] != "client_a" || c["errorCode"] != key.Code ||
		c["firstSeen"] != "2026-09-01T00:00:00Z" || c["lastSeen"] != "2026-09-03T00:00:00Z" || c["open"] != true {
		t.Errorf("run condition %v", c)
	}
	if rs := c["runs"].([]map[string]any); len(rs) != 2 || rs[1]["sessionId"] != "4" || rs[1]["storedRows"] != int64(3) {
		t.Errorf("runs %v", rs)
	}

	// Alone on its run: nothing is added, as before run conditions.
	alone := *detail
	alone.Codes = nil
	alone.Key.ErrorCode = ""
	alone.Condition = &IncidentRunCondition{Key: key, FirstSeen: t0, LastSeen: t0,
		Runs: []RunConditionRun{{SessionId: "2", StartTime: t0}}, ConditionRuns: 1, OwnRunHeld: true, Open: true}
	ops = &fakeOps{incident: run, condition: conditionFake{run: &alone}}
	res, _, _ = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: run.IncidentId}, "a@b")
	for _, k := range []string{"errorCode", "codes", "runCondition", "runConditionNote", "runConditionError"} {
		if v, ok := res["incident"].(map[string]any)[k]; ok {
			t.Errorf("an incident alone on its run carries %s: %v", k, v)
		}
	}

	// No condition holds it: the note says why.
	ops = &fakeOps{incident: run, condition: conditionFake{run: &RunIncidentDetail{Note: "written before"}}}
	res, _, _ = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: run.IncidentId}, "a@b")
	if inc := res["incident"].(map[string]any); inc["runConditionNote"] != "written before" || inc["runCondition"] != nil {
		t.Errorf("no run condition: %v", inc)
	}

	ops = &fakeOps{incident: run, condition: conditionFake{runErr: errors.New("no run_finding table")}}
	res, code, err = dispatch(t, ops, &AgenticAction{Action: "get_incident", IncidentId: run.IncidentId}, "a@b")
	if err != nil || code != http.StatusOK || res["incident"].(map[string]any)["runConditionError"] != "no run_finding table" {
		t.Errorf("a run condition that cannot be read: code %d, err %v, %v", code, err, res["incident"])
	}
}
