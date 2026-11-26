package shadow

// The session purge and the agentic retention sweep, then shadow triage, end to
// end against a real Postgres: a condition whose first delivery or run is
// deleted by the purge keeps its incident while a later delivery or run of it
// is held, the next one joins it rather than raising a second, and the sweep
// deletes the incident, with its rows, once none is held.
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 55443:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55443/postgres \
//	  go test -count=1 -run Purge ./jets/agentic/shadow/

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/agentic/retention"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/jackc/pgx/v5/pgxpool"
)

// purgeTables are the tables the session purge writes beside the execution
// record: the registry, the deletion queue, the coordinator tables and the
// archive.
var purgeTables = []string{"session_registry", "session_deletion", "pipeline_coordinator_lock",
	"pipeline_coordinator_map", "pipeline_coordinator_map_items", "pipeline_execution_status_archive"}

// purgeSession deletes a session by the session purge's executor, by the
// purge's retention deletion or by a user's, as each runs it, and returns the
// report.
func purgeSession(t *testing.T, pool *pgxpool.Pool, session, client string, reason sessionpurge.Reason) sessionpurge.Report {
	t.Helper()
	ctx := context.Background()
	req := sessionpurge.Request{SessionID: session, Client: client, Reason: reason, RequestedBy: "someone@example.com"}
	if _, err := sessionpurge.Queue(ctx, pool, []sessionpurge.Request{req}); err != nil {
		t.Fatal(err)
	}
	opts := sessionpurge.Options{}
	if reason == sessionpurge.ReasonUser {
		opts.Reasons = []sessionpurge.Reason{sessionpurge.ReasonUser}
	}
	r, err := sessionpurge.Execute(ctx, pool, opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.Done != 1 {
		t.Fatalf("purging %s: %+v", session, r)
	}
	return r
}

func sweep(t *testing.T, pool *pgxpool.Pool) retention.Report {
	t.Helper()
	r, err := retention.Sweep(context.Background(), pool, retention.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func incidentRows(t *testing.T, pool *pgxpool.Pool, id string) (incident, hypotheses, events int) {
	t.Helper()
	ctx := context.Background()
	for _, q := range []struct {
		stmt string
		n    *int
	}{{`SELECT count(*) FROM jetsapi.incident WHERE incident_id = $1`, &incident},
		{`SELECT count(*) FROM jetsapi.hypothesis WHERE hypothesis_incident_ref = $1`, &hypotheses},
		{`SELECT count(*) FROM jetsapi.incident_event WHERE event_incident_ref = $1`, &events}} {
		if err := pool.QueryRow(ctx, q.stmt, id).Scan(q.n); err != nil {
			t.Fatal(err)
		}
	}
	return
}

// The weekly repeat: its first delivery purged with its session (its anomalies
// with it), the sweep keeps the condition's incident, every later delivery and
// the next one join it, and no second incident is raised. Once every delivery
// of the condition is purged, the sweep deletes it with its hypotheses and
// events, and a dry run says so first.
func TestAPurgedFirstDeliveryKeepsItsIncidentUntilNoneIsHeld(t *testing.T) {
	pool := freshDB(t, "shadow_retention_feed", slices.Concat(migratedTables, purgeTables), true)
	ctx := context.Background()
	if _, err := feedfixture.Install(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if n := detectFeeds(t, pool); n != 54 {
		t.Fatalf("the detectors wrote %d anomalies, want 54", n)
	}
	w := testWriter()
	w.Now = func() time.Time { return feedfixture.Clock }
	triageAll(t, pool, &w)

	weekly, _ := feedfixture.FeedByTable(feedfixture.CumulativeDeclaredIncremental)
	ws := sessionsOf(weekly)
	inc := IncidentIdFor(ws[0], departs)
	if n, h, _ := incidentRows(t, pool, inc); n != 1 || h == 0 {
		t.Fatalf("the condition's incident %s: %d rows, %d hypotheses", inc, n, h)
	}
	anchor := departsAnomalyOn(t, pool, ws[0])

	r := purgeSession(t, pool, ws[0], weekly.Client, sessionpurge.ReasonRetention)
	if r.Rows["jetsapi.anomaly"] == 0 {
		t.Errorf("the purge deleted no anomaly of %s: %v", ws[0], r.Rows)
	}
	if s := sweep(t, pool); s.Candidates != 1 || s.Held != 1 || s.Deleted != 0 {
		t.Errorf("the sweep with seven deliveries of the condition held: %+v, want %s kept", s, inc)
	}
	rerun(t, pool, &w, ws[1:], inc)
	last := ws[len(ws)-1]
	next := plantAfter(t, pool, anchor, last, 1000, feedfixture.Clock, "next")
	rerun(t, pool, &w, []string{next}, inc)
	if got := feedIncidents(t, pool, weekly.TableName, departs); !slices.Equal(got, []string{inc}) {
		t.Errorf("the feed's incidents %v, want only %s", got, inc)
	}

	// Every other delivery of the condition, by a user's deletion.
	for _, s := range append(slices.Clone(ws[1:]), next) {
		purgeSession(t, pool, s, weekly.Client, sessionpurge.ReasonUser)
	}
	dry, err := retention.Sweep(ctx, pool, retention.Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(dry.DeletedIncidents, inc) {
		t.Errorf("the dry run would delete %v, want %s among them", dry.DeletedIncidents, inc)
	}
	if n, _, _ := incidentRows(t, pool, inc); n != 1 {
		t.Error("the dry run deleted the incident")
	}
	real := sweep(t, pool)
	if !slices.Equal(real.DeletedIncidents, dry.DeletedIncidents) || !slices.Contains(real.DeletedIncidents, inc) {
		t.Errorf("the sweep deleted %v, the dry run predicted %v", real.DeletedIncidents, dry.DeletedIncidents)
	}
	if n, h, e := incidentRows(t, pool, inc); n+h+e != 0 {
		t.Errorf("incident %s left %d rows, %d hypotheses, %d events", inc, n, h, e)
	}
	// The other feeds' incidents, on deliveries still held, are untouched.
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident i WHERE NOT `+
		retention.SessionHeldSQL("i.incident_session_id")).Scan(&left); err != nil || left != 0 {
		t.Errorf("%d incidents on sessions no longer held survived the sweep (%v)", left, err)
	}
}

// A run condition: its first run purged, the sweep keeps its incident while the
// later runs of the condition are held, the next failing run joins it, and
// once every run of the condition is purged the incident goes with its rows.
func TestAPurgedFirstRunKeepsItsIncidentUntilNoneIsHeld(t *testing.T) {
	pool := freshDB(t, "shadow_retention_run",
		slices.Concat(runConditionTables, purgeTables, []string{"input_registry"}), true)
	w := testWriter()
	wf := triage.LocusWorkerFailed
	for i, s := range []string{"run-01", "run-02", "run-03"} {
		plantRun(t, pool, s, float64(i+1), true, 0, 0)
		run(t, pool, &w, s)
	}
	inc := IncidentIdFor("run-01", wf)
	if got := incidentsAt(t, pool, wf, ""); !slices.Equal(got, []string{inc}) {
		t.Fatalf("the worker incidents %v, want %s", got, inc)
	}

	r := purgeSession(t, pool, "run-01", "client2", sessionpurge.ReasonRetention)
	if r.Rows["jetsapi.run_finding"] == 0 {
		t.Errorf("the purge deleted no run verdict of run-01: %v", r.Rows)
	}
	if s := sweep(t, pool); s.Candidates == 0 || s.Held != s.Candidates || s.Deleted != 0 {
		t.Errorf("the sweep with two runs of the condition held: %+v, want every incident on run-01 kept", s)
	}
	plantRun(t, pool, "run-04", 4, true, 0, 0)
	if res := run(t, pool, &w, "run-04"); len(atKeys(res.Written)) != 0 || !grouped(res, inc) {
		t.Errorf("run-04: written %v, grouped %v; want it joined to %s", res.Written, res.Grouped, inc)
	}
	if got := incidentsAt(t, pool, wf, ""); !slices.Equal(got, []string{inc}) {
		t.Errorf("the worker incidents %v, want only %s", got, inc)
	}

	for _, s := range []string{"run-02", "run-03", "run-04"} {
		purgeSession(t, pool, s, "client2", sessionpurge.ReasonUser)
	}
	s := sweep(t, pool)
	if !slices.Contains(s.DeletedIncidents, inc) || s.Held != 0 {
		t.Errorf("the sweep with no run of the condition held: %+v", s)
	}
	if n, h, e := incidentRows(t, pool, inc); n+h+e != 0 {
		t.Errorf("incident %s left %d rows, %d hypotheses, %d events", inc, n, h, e)
	}
}
