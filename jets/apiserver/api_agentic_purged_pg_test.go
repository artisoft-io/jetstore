package main

// The incident screens after the purge, against a real Postgres, on the feed
// fixture conditionDB installs: an incident whose own delivery is deleted is
// still shown with its condition, found by its feed key and position, and an
// incident the retention sweep deleted is said to be purged, on its own page
// and on the page of an incident whose account names it.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55443/postgres \
//	  go test -count=1 -run Purged ./jets/apiserver/

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// installTables installs the named jetsapi tables from jets_schema.json.
func installTables(t *testing.T, pool *pgxpool.Pool, names ...string) {
	t.Helper()
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	for i := range defs {
		if want[defs[i].TableName] {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("installing %s: %v", defs[i].TableName, err)
			}
		}
	}
}

// deleteFixtureDelivery removes a delivery as the session purge removes its
// session: its registration and its anomalies; its incident stays.
func deleteFixtureDelivery(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	for _, stmt := range []string{`DELETE FROM jetsapi.anomaly WHERE anomaly_session_id = $1`,
		`DELETE FROM jetsapi.input_registry WHERE session_id = $1`} {
		if _, err := pool.Exec(context.Background(), stmt, session); err != nil {
			t.Fatal(err)
		}
	}
}

// The weekly repeat's first delivery deleted: its incident's page still shows
// the condition, the seven deliveries it now stands for, and a summary read
// from the earliest of them; the incident list counts the seven.
func TestAKeyedIncidentWhoseDeliveryIsPurgedShowsItsCondition(t *testing.T) {
	pool := conditionDB(t, "apiserver_feed_condition_purged")
	ws := fixtureSessions(feedfixture.CumulativeDeclaredIncremental)
	inc := shadow.IncidentIdFor(ws[0], departsLocus)
	before := readIncident(t, pool, inc)
	if before["condition"] == nil {
		t.Fatalf("before the deletion the incident shows no condition: %v", before["conditionNote"])
	}

	deleteFixtureDelivery(t, pool, ws[0])
	row := readIncident(t, pool, inc)
	c, ok := row["condition"].(map[string]any)
	if !ok {
		t.Fatalf("after its delivery is deleted the incident shows no condition: note %v, error %v",
			row["conditionNote"], row["conditionError"])
	}
	ds := c["deliveries"].([]any)
	if len(ds) != 7 || c["standsFor"] != float64(7) || ds[0].(map[string]any)["sessionId"] != ws[1] {
		t.Errorf("the condition: %d deliveries, stands for %v, first %v", len(ds), c["standsFor"], ds[0])
	}
	for _, d := range ds {
		if d.(map[string]any)["incidentId"] != inc {
			t.Errorf("delivery %v is placed elsewhere than %s", d, inc)
		}
	}
	if row["summary"] == nil {
		t.Errorf("no summary: %v", row["summaryError"])
	}

	payload, code, err := agenticDispatch(context.Background(), pgOps{pool}, &AgenticAction{Action: "list_incidents",
		Limit: 200}, "supervisor@example.com", audit.RedactPHI)
	if err != nil || code != http.StatusOK {
		t.Fatalf("list_incidents returned %d: %v", code, err)
	}
	found := false
	for _, r := range decode(t, payload)["incidents"].([]any) {
		r := r.(map[string]any)
		if r["incidentId"] == inc {
			found = true
			if r["count"] != float64(7) || r["fromCondition"] != true {
				t.Errorf("the list row: count %v, from its condition %v; want 7, true", r["count"], r["fromCondition"])
			}
		}
	}
	if !found {
		t.Errorf("%s is not on the list", inc)
	}
}

// An incident the database no longer holds, nor its session, is purged: its
// page answers 410 with the sentence the screen shows, and an incident whose
// account names it lists it as purged.
func TestAPurgedIncidentIsSaidToBePurged(t *testing.T) {
	pool := conditionDB(t, "apiserver_feed_condition_purged_named")
	installTables(t, pool, "session_registry", "session_deletion")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.session_deletion
		(session_id, client, reason, requested_by, status, completed_at)
		VALUES ('1700000000000', 'c1', 'retention', 'purge', 'done', now())`); err != nil {
		t.Fatal(err)
	}
	purged := "inc_1700000000000_" + departsLocus

	_, code, err := agenticDispatch(ctx, pgOps{pool}, &AgenticAction{Action: "get_incident", IncidentId: purged},
		"supervisor@example.com", audit.RedactPHI)
	if code != http.StatusGone || err == nil || !strings.Contains(err.Error(), "was purged") {
		t.Errorf("get_incident on a purged incident: %d, %v", code, err)
	}
	// A mistyped id of no incident's shape is still a 404.
	if _, code, _ := agenticDispatch(ctx, pgOps{pool}, &AgenticAction{Action: "get_incident", IncidentId: "nope"},
		"supervisor@example.com", audit.RedactPHI); code != http.StatusNotFound {
		t.Errorf("get_incident on an id of no incident's shape: %d, want 404", code)
	}

	// An incident raised after a person's verdict names the judged one in its
	// account; the judged one purged, the page says so.
	ws := fixtureSessions(feedfixture.CumulativeDeclaredIncremental)
	inc := shadow.IncidentIdFor(ws[0], departsLocus)
	if row := readIncident(t, pool, inc); row["purgedIncidents"] != nil {
		t.Fatalf("an incident naming no purged incident lists %v", row["purgedIncidents"])
	}
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.incident_event
		SET transition_rationale = transition_rationale || '. The previous incident at this locus is ' || $2::text ||
		    ', on session 1700000000000, with a person''s verdict'
		WHERE event_incident_ref = $1 AND to_status = 'triaged'`, inc, purged); err != nil {
		t.Fatal(err)
	}
	list, ok := readIncident(t, pool, inc)["purgedIncidents"].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("purgedIncidents %v, want %s", list, purged)
	}
	p := list[0].(map[string]any)
	if p["incidentId"] != purged || p["sessionId"] != "1700000000000" || p["deleted"] != true ||
		!strings.Contains(p["sentence"].(string), "was purged") {
		t.Errorf("the purged incident named: %v", p)
	}
}
