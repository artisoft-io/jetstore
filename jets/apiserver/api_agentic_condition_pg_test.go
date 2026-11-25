package main

// A grouped delivery's link to its condition's incident, through the endpoint,
// against a real Postgres: the synthetic feed fixture is installed, the feed
// detectors run, shadow triage raises one incident per condition, and
// find_session_incidents and get_incident are read back through
// agenticDispatch over pgOps.
//
//	docker run -d --rm --name apiserver-pg -e POSTGRES_PASSWORD=pw -p 55892:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run Condition ./jets/apiserver/
//
// Run it with -count=1: the fixture reads jets_schema.json, which the test
// cache does not see.

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const departsLocus = triage.LocusDeliveryDepartsProfile

// conditionDB is a fresh database holding the execution tables shadow triage
// reads, the agentic DDL, and the feed fixture, with the feed detectors run
// over it and shadow triage run over every session they fired on.
func conditionDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	for _, q := range []string{"DROP DATABASE IF EXISTS " + name + " WITH (FORCE)", "CREATE DATABASE " + name} {
		if _, err := admin.Exec(ctx, q); err != nil {
			admin.Close()
			t.Fatalf("%s: %v", q, err)
		}
	}
	admin.Close()
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)

	f, err := os.Open("../jets_schema.json")
	if err != nil {
		t.Fatalf("opening jets_schema.json: %v", err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	want := map[string]bool{"pipeline_execution_status": true, "pipeline_execution_details": true,
		"pipeline_execution_channel_details": true, "process_errors": true, "cpipes_execution_status": true}
	for i := range defs {
		if want[defs[i].TableName] {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("installing %s: %v", defs[i].TableName, err)
			}
		}
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if n := detectFixtureFeeds(t, pool); n != 54 {
		t.Fatalf("the detectors wrote %d anomalies, want 54", n)
	}

	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return feedfixture.Clock }
	sessions, err := shadow.RecentFeedSessions(ctx, pool, feedfixture.Clock.Add(-time.Hour), "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if _, err := w.Run(ctx, pool, s); err != nil {
			t.Fatalf("session %s: %v", s, err)
		}
	}
	return pool
}

// detectFixtureFeeds runs the feed detectors over the installed fixture, as of
// its clock, and writes their anomalies, as shadow's own feed tests do.
func detectFixtureFeeds(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	ctx := context.Background()
	var profiled, unprofiled []feedprofile.Feed
	clients := map[string]bool{}
	for _, f := range feedfixture.Feeds() {
		if clients[f.Client] {
			continue
		}
		clients[f.Client] = true
		p, u, err := feedprofile.Load(ctx, pool, f.Client)
		if err != nil {
			t.Fatal(err)
		}
		profiled, unprofiled = append(profiled, p...), append(unprofiled, u...)
	}
	in, err := observe.ReadFeedDeliveryInput(ctx, pool, profiled, unprofiled, feedfixture.Clock)
	if err != nil {
		t.Fatal(err)
	}
	written := 0
	for _, d := range []interface {
		Detect(*observe.FeedDeliveryInput) (observe.FeedDeliveryResult, error)
	}{observe.DefaultFeedOverdue(), observe.DefaultDeliverySize(), observe.DefaultDeliveryBackdated()} {
		res, err := d.Detect(in)
		if err != nil {
			t.Fatal(err)
		}
		n, err := observe.FeedDeliveryWrite(ctx, pool, res)
		if err != nil {
			t.Fatal(err)
		}
		written += n
	}
	o := observe.DefaultClaimSetOptions()
	o.Now = feedfixture.Clock
	verdicts, err := observe.DetectClaimSets(ctx, pool, append(slices.Clone(profiled), unprofiled...), o)
	if err != nil {
		t.Fatal(err)
	}
	n, _, err := observe.WriteClaimSetAnomalies(ctx, pool, verdicts)
	if err != nil {
		t.Fatal(err)
	}
	return written + n
}

func fixtureSessions(table string) []string {
	f, _ := feedfixture.FeedByTable(table)
	out := make([]string, len(f.Deliveries))
	for i, d := range f.Deliveries {
		out[i] = d.SessionID
	}
	return out
}

// decode round-trips a payload through JSON, so a test reads what the browser
// reads rather than the Go values.
func decode(t *testing.T, payload *map[string]any) map[string]any {
	t.Helper()
	b, err := json.Marshal(*payload)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func findIncidents(t *testing.T, pool *pgxpool.Pool, session string) map[string]any {
	t.Helper()
	payload, code, err := agenticDispatch(context.Background(), pgOps{pool},
		&AgenticAction{Action: "find_session_incidents", SessionId: session}, "supervisor@example.com", audit.RedactPHI)
	if err != nil || code != 200 {
		t.Fatalf("find_session_incidents %s returned %d: %v", session, code, err)
	}
	return decode(t, payload)
}

func readIncident(t *testing.T, pool *pgxpool.Pool, id string) map[string]any {
	t.Helper()
	payload, code, err := agenticDispatch(context.Background(), pgOps{pool},
		&AgenticAction{Action: "get_incident", IncidentId: id}, "supervisor@example.com", audit.RedactPHI)
	if err != nil || code != 200 {
		t.Fatalf("get_incident %s returned %d: %v", id, code, err)
	}
	return decode(t, payload)["incident"].(map[string]any)
}

// The weekly repeat: a delivery in the middle of the condition carries no
// incident of its own, and the endpoint links it, and a pipeline run that
// read it, to the incident on the condition's first delivery. The incident's
// page lists the condition's eight deliveries, with their file keys.
func TestAGroupedDeliveryLinksToItsConditionsIncident(t *testing.T) {
	pool := conditionDB(t, "apiserver_feed_condition")
	ctx := context.Background()

	ws := fixtureSessions(feedfixture.CumulativeDeclaredIncremental)
	if len(ws) != 8 {
		t.Fatalf("the weekly repeat has %d deliveries, want 8", len(ws))
	}
	inc := shadow.IncidentIdFor(ws[0], departsLocus)
	middle := ws[3]

	// The middle delivery has no incident of its own.
	var own int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident WHERE incident_session_id = $1`,
		middle).Scan(&own); err != nil || own != 0 {
		t.Fatalf("the middle delivery carries %d incidents of its own (%v), want none", own, err)
	}

	got := findIncidents(t, pool, middle)
	matches := got["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("the middle delivery matches %d incidents, want 1: %v", len(matches), matches)
	}
	m := matches[0].(map[string]any)
	if m["how"] != SessionIncidentCondition || m["incidentId"] != inc || m["incidentSessionId"] != ws[0] ||
		m["locus"] != departsLocus || m["feed"] != feedfixture.CumulativeDeclaredIncremental ||
		m["sessionId"] != middle {
		t.Errorf("the middle delivery's match: %v, want the condition's incident %s on %s", m, inc, ws[0])
	}
	if !strings.Contains(m["why"].(string), inc) {
		t.Errorf("the match does not say why: %q", m["why"])
	}

	// The first delivery's incident is its own.
	first := findIncidents(t, pool, ws[0])["matches"].([]any)
	if len(first) != 1 || first[0].(map[string]any)["how"] != SessionIncidentOwn ||
		first[0].(map[string]any)["incidentId"] != inc {
		t.Errorf("the first delivery's matches: %v, want its own incident %s", first, inc)
	}

	// A pipeline run that read the middle delivery finds the condition's
	// incident through its input session.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id, input_session_id,
		 source_period_key, status, user_email, start_time, last_update)
		VALUES (1, 'client_a', 'map', 'PharmacyClaim', 'run-of-middle', $1, 1, 'completed', 'test@test', now(), now())`,
		middle); err != nil {
		t.Fatal(err)
	}
	run := findIncidents(t, pool, "run-of-middle")
	if in := run["inputSessions"].([]any); len(in) != 1 || in[0] != middle {
		t.Errorf("the run's input sessions: %v, want [%s]", in, middle)
	}
	rm := run["matches"].([]any)
	if len(rm) != 1 || rm[0].(map[string]any)["incidentId"] != inc || rm[0].(map[string]any)["sessionId"] != middle {
		t.Errorf("the run's matches: %v, want the condition's incident through %s", rm, middle)
	}

	// A session nothing was found on matches nothing.
	if none := findIncidents(t, pool, "no-such-session")["matches"].([]any); len(none) != 0 {
		t.Errorf("an unknown session matches %v", none)
	}

	// The incident's page carries the condition, read live.
	c, ok := readIncident(t, pool, inc)["condition"].(map[string]any)
	if !ok {
		t.Fatalf("the weekly incident carries no condition")
	}
	ds := c["deliveries"].([]any)
	if len(ds) != 8 || c["standsFor"] != float64(8) || c["open"] != true || c["previous"] != nil ||
		c["feed"] != feedfixture.CumulativeDeclaredIncremental {
		t.Errorf("the weekly condition: %d deliveries, stands for %v, open %v, previous %v",
			len(ds), c["standsFor"], c["open"], c["previous"])
	}
	for i, d := range ds {
		d := d.(map[string]any)
		if d["sessionId"] != ws[i] || d["incidentId"] != inc || d["fileKey"] == "" || d["registered"] != true {
			t.Errorf("delivery %d of the condition: %v", i+1, d)
		}
	}
	if ds[0].(map[string]any)["placement"] != "already_raised" || ds[3].(map[string]any)["placement"] != "join" {
		t.Errorf("the placements: first %v, middle %v", ds[0].(map[string]any)["placement"],
			ds[3].(map[string]any)["placement"])
	}
}

// The interrupted condition: the second incident's page names the first
// condition's incident, read live. The daily run triages newest first, so the
// second incident's stored basis says the first is not written yet; the page
// says what is true now, and shows a person's verdict given since.
func TestAnIncidentsConditionNamesThePreviousIncidentLive(t *testing.T) {
	pool := conditionDB(t, "apiserver_feed_condition_previous")
	ctx := context.Background()

	wide := fixtureSessions(feedfixture.WideSpanIncremental)
	var withIncident []string
	for _, s := range wide {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident WHERE incident_session_id = $1
			AND incident_locus = $2`, s, departsLocus).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			withIncident = append(withIncident, s)
		}
	}
	if len(withIncident) != 2 {
		t.Fatalf("the wide-span feed has incidents on %v, want two", withIncident)
	}
	first, second := shadow.IncidentIdFor(withIncident[0], departsLocus), shadow.IncidentIdFor(withIncident[1], departsLocus)

	var basis string
	if err := pool.QueryRow(ctx, `SELECT transition_rationale FROM jetsapi.incident_event
		WHERE event_incident_ref = $1 AND to_status = 'triaged'`, second).Scan(&basis); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(basis, "has no incident yet") {
		t.Fatalf("the second incident's basis was expected to say the first is not written yet:\n%s", basis)
	}

	c := readIncident(t, pool, second)["condition"].(map[string]any)
	prev, ok := c["previous"].(map[string]any)
	if !ok {
		t.Fatalf("the second condition names no previous incident: %v", c)
	}
	pi := prev["incident"].(map[string]any)
	if pi["incidentId"] != first || pi["verdict"] != nil || prev["inSameCondition"] != false {
		t.Errorf("the previous incident: %v, want %s with no verdict, in the previous condition", prev, first)
	}
	if pc := prev["condition"].(map[string]any); pc["deliveries"] != float64(3) || pc["firstSessionId"] != withIncident[0] {
		t.Errorf("the previous condition: %v", pc)
	}
	fc := readIncident(t, pool, first)["condition"].(map[string]any)
	if fc["open"] != false || fc["end"] != triage.EndNoFinding || fc["endedBy"] == "" || fc["standsFor"] != float64(3) {
		t.Errorf("the first condition: open %v, end %v by %v, stands for %v", fc["open"], fc["end"], fc["endedBy"],
			fc["standsFor"])
	}

	// A person's verdict on the first, and the second's page shows it.
	if _, code, err := agenticDispatch(ctx, pgOps{pool}, &AgenticAction{Action: "record_incident_transition",
		IncidentId: first, FromStatus: audit.IncidentDiagnosed, ToStatus: "suppressed_as_benign",
		Rationale: "the profile is out of date"}, "supervisor@example.com", audit.RedactPHI); err != nil || code != 200 {
		t.Fatalf("recording the verdict returned %d: %v", code, err)
	}
	pi = readIncident(t, pool, second)["condition"].(map[string]any)["previous"].(map[string]any)["incident"].(map[string]any)
	v, ok := pi["verdict"].(map[string]any)
	if !ok || v["toStatus"] != "suppressed_as_benign" || v["actor"] != "supervisor@example.com" {
		t.Errorf("the previous incident's verdict: %v", pi["verdict"])
	}
}
