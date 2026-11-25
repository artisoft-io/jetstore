package main

// The incident list's page, client filter and spans, through the endpoint,
// against a real Postgres holding the synthetic feed fixture (conditionDB).
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run 'IncidentList' ./jets/apiserver/

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func listIncidentsPage(t *testing.T, pool *pgxpool.Pool, a AgenticAction) map[string]any {
	t.Helper()
	a.Action = "list_incidents"
	payload, code, err := agenticDispatch(context.Background(), pgOps{pool}, &a, "supervisor@example.com", audit.RedactPHI)
	if err != nil || code != 200 {
		t.Fatalf("list_incidents returned %d: %v", code, err)
	}
	return decode(t, payload)
}

func incidentIds(page map[string]any) []string {
	var out []string
	for _, r := range page["incidents"].([]any) {
		out = append(out, r.(map[string]any)["incidentId"].(string))
	}
	return out
}

// A page is a slice of the whole list in its order, the total is the whole
// list's, and the client filter keeps the incidents whose session the client
// registered.
func TestIncidentListPagesAndFiltersByClient(t *testing.T) {
	pool := conditionDB(t, "agentic_list_paging")
	all := listIncidentsPage(t, pool, AgenticAction{})
	ids := incidentIds(all)
	total := int(all["total"].(float64))
	if total != len(ids) || total < 6 {
		t.Fatalf("total %d over %d rows; the fixture raises more than six incidents", total, len(ids))
	}
	for off := 0; off < 6; off += 3 {
		page := listIncidentsPage(t, pool, AgenticAction{Limit: 3, Offset: off})
		if got := incidentIds(page); !slices.Equal(got, ids[off:off+3]) {
			t.Errorf("page at %d is %v, want %v", off, got, ids[off:off+3])
		}
		if int(page["total"].(float64)) != total {
			t.Errorf("a page's total is %v, want the list's %d", page["total"], total)
		}
	}

	byClient := map[string]int{}
	for _, r := range all["incidents"].([]any) {
		row := r.(map[string]any)
		if row["client"] == "" || row["objectType"] == "" {
			t.Errorf("incident %s has client %q and object type %q; every fixture session is registered",
				row["incidentId"], row["client"], row["objectType"])
		}
		byClient[row["client"].(string)]++
	}
	for client, n := range byClient {
		page := listIncidentsPage(t, pool, AgenticAction{Client: client})
		if int(page["total"].(float64)) != n {
			t.Errorf("client %s keeps %v incidents, want %d", client, page["total"], n)
		}
		for _, r := range page["incidents"].([]any) {
			if c := r.(map[string]any)["client"]; c != client {
				t.Errorf("the filter on %s kept an incident of %v", client, c)
			}
		}
	}
	if none := listIncidentsPage(t, pool, AgenticAction{Client: "no_such_client"}); none["total"].(float64) != 0 ||
		len(none["incidents"].([]any)) != 0 {
		t.Errorf("a client with no incident keeps %v", none["total"])
	}
}

// A feed incident's count and span are its condition's, as get_incident
// reads it: the deliveries placed in this incident, by arrival. A run
// incident's are its run condition's, as get_incident reads it: the runs still
// held placed in it, from its position to its latest member. The feed fixture
// writes no run incident, so three runs of one process are planted beside it.
func TestIncidentListSpansAreTheConditions(t *testing.T) {
	pool := conditionDB(t, "agentic_list_spans")
	_, starts := plantRunCondition(t, pool, "client_r", "loader")
	list := listIncidentsPage(t, pool, AgenticAction{})
	feedRows, runRows := 0, 0
	for _, r := range list["incidents"].([]any) {
		row := r.(map[string]any)
		id := row["incidentId"].(string)
		if !slices.Contains(triage.DeliveryLoci, row["locus"].(string)) {
			runRows++
			c, ok := readIncident(t, pool, id)["runCondition"].(map[string]any)
			if !ok {
				t.Errorf("run incident %s carries no run condition", id)
				continue
			}
			if row["count"] != c["runsHeld"] || row["firstSeen"] != c["firstSeen"] || row["lastSeen"] != c["lastSeen"] ||
				row["fromRunCondition"] != true {
				t.Errorf("run incident %s: count %v, %v to %v; its run condition holds %v, %v to %v", id,
					row["count"], row["firstSeen"], row["lastSeen"], c["runsHeld"], c["firstSeen"], c["lastSeen"])
			}
			if row["count"].(float64) != 3 || row["firstSeen"] != rfc(starts[0]) || row["lastSeen"] != rfc(starts[2]) {
				t.Errorf("run incident %s: count %v, %v to %v; want the three planted runs", id, row["count"],
					row["firstSeen"], row["lastSeen"])
			}
			continue
		}
		c, ok := readIncident(t, pool, id)["condition"].(map[string]any)
		if !ok || c == nil {
			continue
		}
		feedRows++
		var first, last string
		for _, d := range c["deliveries"].([]any) {
			dm := d.(map[string]any)
			if dm["incidentId"] != id {
				continue
			}
			at := dm["arrival"].(string)
			if at == "" {
				at = dm["firstDetectedAt"].(string)
			}
			if first == "" || at < first {
				first = at
			}
			if last == "" || at > last {
				last = at
			}
		}
		if row["count"].(float64) != c["standsFor"].(float64) {
			t.Errorf("incident %s counts %v, its condition stands for %v", id, row["count"], c["standsFor"])
		}
		if row["firstSeen"] != first || row["lastSeen"] != last {
			t.Errorf("incident %s spans %v to %v, its placed deliveries %s to %s", id, row["firstSeen"],
				row["lastSeen"], first, last)
		}
		if row["fromCondition"] != true {
			t.Errorf("incident %s: fromCondition %v", id, row["fromCondition"])
		}
	}
	if feedRows == 0 {
		t.Fatal("no feed incident with a condition on the fixture")
	}
	if runRows == 0 {
		t.Fatal("no run incident on the list, so its spans are not tested")
	}
	// The weekly repeat's condition is eight deliveries in one incident.
	weekly := listIncidentsPage(t, pool, AgenticAction{})
	found := false
	for _, r := range weekly["incidents"].([]any) {
		row := r.(map[string]any)
		if row["sessionId"] == fixtureSessions(feedfixture.CumulativeDeclaredIncremental)[0] &&
			row["locus"] == departsLocus {
			found = true
			if row["count"].(float64) < 2 || row["firstSeen"] == row["lastSeen"] {
				t.Errorf("the weekly repeat's incident counts %v over %v to %v", row["count"],
					row["firstSeen"], row["lastSeen"])
			}
		}
	}
	if !found {
		t.Error("the weekly repeat's incident is not on the list")
	}
}

// The cost of the spans on a page of 150 feed incidents: fifteen synthetic
// feeds of ten daily deliveries, each delivery with its own incident, added to
// the fixture. The spans read the page's anomalies once and each feed once.
func TestIncidentListSpansOnAPageOf150(t *testing.T) {
	pool := conditionDB(t, "agentic_list_cost")
	ctx := context.Background()
	base := feedfixture.Clock.Add(-400 * 24 * time.Hour)
	for f := 0; f < 15; f++ {
		table := fmt.Sprintf("synthetic_feed_%02d", f)
		for d := 0; d < 10; d++ {
			at := base.Add(time.Duration(f)*time.Minute + time.Duration(d)*24*time.Hour)
			session := fmt.Sprint(at.UnixMilli())
			if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.input_registry
				(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
				VALUES ('client_s', '', 'SyntheticClaim', $1, 1, $2, 'file', $3, 'system', $4)`,
				table+"/"+session, table, session, at); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.anomaly
				(anomaly_id, detected_at, anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
				 anomaly_signal_type, anomaly_observed_value, anomaly_expected_basis, anomaly_confounders,
				 anomaly_detector_ref)
				VALUES ($1, $2, $3, 'feed', $4, 'volume', '1', 'synthetic', '{}', 'delivery_size@1')`,
				"an_"+table+"_"+session, at, session, table); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.incident
				(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
				 incident_confounders, incident_model_version)
				VALUES ($1, $2, $3, $4, 'medium', 'diagnosed', '{}', 'synthetic')`,
				"inc_"+table+"_"+session, session, feedfixture.Clock.Add(time.Hour+time.Duration(f*10+d)*time.Second),
				departsLocus); err != nil {
				t.Fatal(err)
			}
		}
	}
	page, total, err := pgOps{pool}.ListIncidents(ctx, nil, audit.ListPage{Limit: 150})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 150 {
		t.Fatalf("the page has %d rows of %d, want 150", len(page), total)
	}
	var best time.Duration
	for i := 0; i < 5; i++ {
		start := time.Now()
		spans, err := readIncidentSpans(ctx, pool, page)
		el := time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		if len(spans) != 150 {
			t.Fatalf("%d spans for 150 rows", len(spans))
		}
		if i == 0 || el < best {
			best = el
		}
	}
	start := time.Now()
	listIncidentsPage(t, pool, AgenticAction{Limit: 150})
	t.Logf("spans for a page of 150 feed incidents over 15 feeds: best of 5 %v; the whole list_incidents %v",
		best, time.Since(start))
}

// A proposal an incident triggered has its incident's client and object type;
// one an authoring run produced has none, and a client filter does not keep it.
func TestProposalListCarriesItsIncidentsClient(t *testing.T) {
	pool := conditionDB(t, "agentic_list_proposals")
	ctx := context.Background()
	inc := listIncidentsPage(t, pool, AgenticAction{Limit: 1})["incidents"].([]any)[0].(map[string]any)
	for _, p := range []struct{ id, trigger, ref string }{
		{"prop_from_incident", audit.TriggerIncident, inc["incidentId"].(string)},
		{"prop_from_run", "authoring_run", "run_1"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.change_proposal
			(proposal_id, trigger, trigger_ref, affected_pipelines, generated_tests, impact_affected_assets,
			 impact_clinical_relevance_touched, approval_state, proposal_model_version)
			VALUES ($1, $2, $3, '{}', '{}', '{}', false, 'draft', 'test')`, p.id, p.trigger, p.ref); err != nil {
			t.Fatal(err)
		}
	}
	list := func(a AgenticAction) map[string]any {
		a.Action = "list_proposals"
		payload, code, err := agenticDispatch(ctx, pgOps{pool}, &a, "supervisor@example.com", audit.RedactPHI)
		if err != nil || code != 200 {
			t.Fatalf("list_proposals returned %d: %v", code, err)
		}
		return decode(t, payload)
	}
	all := list(AgenticAction{})
	if all["total"].(float64) != 2 {
		t.Fatalf("total %v, want 2", all["total"])
	}
	for _, r := range all["proposals"].([]any) {
		row := r.(map[string]any)
		switch row["proposalId"] {
		case "prop_from_incident":
			if row["client"] != inc["client"] || row["objectType"] != inc["objectType"] || row["client"] == "" {
				t.Errorf("the incident's proposal reads %v/%v, its incident %v/%v", row["client"],
					row["objectType"], inc["client"], inc["objectType"])
			}
		case "prop_from_run":
			if row["client"] != "" || row["objectType"] != "" {
				t.Errorf("the run's proposal reads %v/%v, want none", row["client"], row["objectType"])
			}
		}
	}
	kept := list(AgenticAction{Client: inc["client"].(string)})
	if kept["total"].(float64) != 1 || kept["proposals"].([]any)[0].(map[string]any)["proposalId"] != "prop_from_incident" {
		t.Errorf("the client filter keeps %v", kept["proposals"])
	}
	if p := list(AgenticAction{Limit: 1, Offset: 1}); len(p["proposals"].([]any)) != 1 || p["total"].(float64) != 2 {
		t.Errorf("the second page of one is %v of %v", p["proposals"], p["total"])
	}
}
