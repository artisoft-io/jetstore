package shadow

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run conditions keyed by the main input source, against a real Postgres: the
// runs of one process reading two vendors' files are two sources, each its
// own condition.

var sourceConditionTables = append(slices.Clone(runConditionTables), "input_registry", "source_config")

var (
	srcA = observe.Source{Client: "client2", Org: "vendor_a", ObjectType: "claims"}
	srcB = observe.Source{Client: "client2", Org: "vendor_b", ObjectType: "claims"}
)

// plantSourceRun writes one run of process loader for client2 from src, hours
// after runStart: its sharding step read records records, and, unless the
// input is empty, a reducing step that failed or completed.
func plantSourceRun(t *testing.T, pool *pgxpool.Pool, src observe.Source, session string, hours float64,
	failed bool, records int) {
	t.Helper()
	ctx := context.Background()
	start := runStart.Add(time.Duration(hours * float64(time.Hour)))
	status := "completed"
	if failed {
		status = "failed"
	}
	k := header(t, pool, "client2", "loader", "claims", session, status, start)
	var irKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ($1, $2, $3, $4, 1, $5, 'file', $6, 'test@test') RETURNING key`,
		src.Client, src.Org, src.ObjectType, "in/"+session, src.Org+"_claims", "reg-"+session).Scan(&irKey); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_status SET main_input_registry_key = $1
		WHERE key = $2`, irKey, k); err != nil {
		t.Fatal(err)
	}
	workerRow(t, pool, k, "client2", "loader", session, triage.ShardingStepId, 0, "completed", records, records, "",
		start)
	if records == 0 {
		return
	}
	worker, out, msg := "completed", records, ""
	if failed {
		worker, out, msg = "failed", 0, "boom"
	}
	workerRow(t, pool, k, "client2", "loader", session, "reducing01", 1, worker, records, out, msg,
		start.Add(time.Minute))
}

func sourceOf(t *testing.T, pool *pgxpool.Pool, incident string) (src observe.Source, null bool) {
	t.Helper()
	var c, o, ot *string
	if err := pool.QueryRow(context.Background(), `SELECT incident_source_client, incident_source_org,
		incident_source_object_type FROM jetsapi.incident WHERE incident_id = $1`, incident).Scan(&c, &o, &ot); err != nil {
		t.Fatalf("reading the source of %s: %v", incident, err)
	}
	if c == nil || o == nil || ot == nil {
		return src, true
	}
	return observe.Source{Client: *c, Org: *o, ObjectType: *ot}, false
}

// Two vendors' runs of one process alternate. Vendor A's runs all fail at a
// worker; vendor B's are clean. Vendor A's runs are one condition and one
// incident, on its first run, which records its source; vendor B's runs
// neither end that condition nor raise anything. Under a key without the
// source, each clean vendor B run would end the condition, and each vendor A
// run would raise an incident of its own.
func TestTwoSourcesOfOneProcessAreTwoConditions(t *testing.T) {
	pool := freshDB(t, "shadow_source_conditions", sourceConditionTables, true)
	ctx := context.Background()
	w := testWriter()
	wf := triage.LocusWorkerFailed
	order := []struct {
		session string
		src     observe.Source
		failed  bool
	}{
		{"a-01", srcA, true}, {"b-01", srcB, false}, {"a-02", srcA, true}, {"b-02", srcB, false},
		{"a-03", srcA, true}, {"b-03", srcB, false},
	}
	for i, r := range order {
		plantSourceRun(t, pool, r.src, r.session, float64(i+1), r.failed, 100)
	}
	ia := IncidentIdFor("a-01", wf)
	for _, r := range order {
		res := run(t, pool, &w, r.session)
		switch {
		case r.session == "a-01":
			if got := atKeys(res.Written); !slices.Equal(got, []string{ia}) {
				t.Fatalf("a-01 wrote %v, want %s", got, ia)
			}
		case r.src == srcA:
			if len(atKeys(res.Written)) != 0 || !grouped(res, ia) {
				t.Errorf("%s: wrote %v, grouped %v; want joined to %s", r.session, res.Written, res.Grouped, ia)
			}
		default:
			if len(atKeys(res.Written)) != 0 || len(res.Grouped) != 0 {
				t.Errorf("%s, vendor B, clean: wrote %v, grouped %v", r.session, res.Written, res.Grouped)
			}
		}
	}
	if got := incidentsAt(t, pool, wf, ""); !slices.Equal(got, []string{ia}) {
		t.Errorf("worker_failed incidents %v, want %s alone", got, ia)
	}
	if src, null := sourceOf(t, pool, ia); null || src != srcA {
		t.Errorf("incident %s records source %+v (null %v), want %+v", ia, src, null, srcA)
	}
	h, err := runcondition.Read(ctx, pool, runcondition.Key{Client: "client2", Process: "loader", Source: srcA,
		Locus: wf})
	if err != nil {
		t.Fatal(err)
	}
	c, members := h.Members(ia)
	if c == nil || !slices.Equal(c.Sessions(), []string{"a-01", "a-02", "a-03"}) || !c.Open() || len(members) != 3 {
		t.Errorf("vendor A's condition: %+v, %d members", c, len(members))
	}
	if len(h.Runs) != 3 {
		t.Errorf("vendor A's key has %d positions, want its own three runs", len(h.Runs))
	}
	// Vendor B's key holds its own runs and no condition.
	hb, err := runcondition.Read(ctx, pool, runcondition.Key{Client: "client2", Process: "loader", Source: srcB,
		Locus: wf})
	if err != nil {
		t.Fatal(err)
	}
	if len(hb.Runs) != 3 || len(hb.Conditions()) != 0 || len(hb.Incidents) != 0 {
		t.Errorf("vendor B's key: %d runs, %d conditions, %d incidents", len(hb.Runs), len(hb.Conditions()),
			len(hb.Incidents))
	}
}

// A run condition open when the key gained the source: the incident written
// before carries no source, the new key does not find it, and the condition's
// first run after the change raises one new incident for each source; the
// later runs of each source join it. The condition's first run holds the old
// incident's id, so the new incident is written on the run being triaged.
func TestAConditionOpenAcrossTheSourceKeyRaisesOneIncidentPerSource(t *testing.T) {
	pool := freshDB(t, "shadow_source_across_the_change", sourceConditionTables, true)
	ctx := context.Background()
	w := testWriter()
	wf := triage.LocusWorkerFailed
	plantSourceRun(t, pool, srcA, "a-01", 1, true, 100)
	plantSourceRun(t, pool, srcB, "b-01", 2, true, 100)
	// Before the change, one condition held both runs and its incident was on
	// a-01; b-01 joined it. a-01 is triaged, and b-01's verdicts are stored as
	// a run that joined would have stored them.
	run(t, pool, &w, "a-01")
	ev, err := w.Classifier.Gather(ctx, pool, "b-01", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storeRunFindings(ctx, pool, runcondition.RowsOf(w.Classifier.Classify(ev), fixtureNow)); err != nil {
		t.Fatal(err)
	}
	old := IncidentIdFor("a-01", wf)
	// As written before the key carried the source: the three columns NULL.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.incident SET incident_source_client = NULL,
		incident_source_org = NULL, incident_source_object_type = NULL`); err != nil {
		t.Fatal(err)
	}
	// The run with no source still finds it; a run with one does not.
	h, err := runcondition.Read(ctx, pool, runcondition.Key{Client: "client2", Process: "loader", Locus: wf})
	if err != nil || len(h.Incidents) != 1 {
		t.Errorf("the key with no source finds %d incidents (%v), want the old one", len(h.Incidents), err)
	}

	plantSourceRun(t, pool, srcA, "a-02", 3, true, 100)
	plantSourceRun(t, pool, srcB, "b-02", 4, true, 100)
	plantSourceRun(t, pool, srcA, "a-03", 5, true, 100)
	plantSourceRun(t, pool, srcB, "b-03", 6, true, 100)
	newA := IncidentIdFor("a-02", wf)
	res := run(t, pool, &w, "a-02")
	if got := atKeys(res.Written); !slices.Equal(got, []string{newA}) {
		t.Fatalf("a-02, vendor A's first run after the change, wrote %v, want %s", got, newA)
	}
	if b := basisOf(t, pool, newA); !strings.Contains(b, "carries incident "+old) {
		t.Errorf("the new incident does not say why it is not on the condition's first run:\n%s", b)
	}
	// Vendor B's condition is b-01, b-02: its first run carries no incident
	// at this locus, so its incident is raised there.
	newBAnchor := IncidentIdFor("b-01", wf)
	res = run(t, pool, &w, "b-02")
	if got := atKeys(res.Written); len(got) != 1 {
		t.Fatalf("b-02, vendor B's first run after the change, wrote %v, want one incident", got)
	}
	gotB := atKeys(res.Written)[0]
	if gotB != newBAnchor {
		t.Errorf("vendor B's new incident is %s, want %s on its condition's first run", gotB, newBAnchor)
	}
	if b := basisOf(t, pool, gotB); strings.Contains(b, "carries incident") {
		t.Errorf("vendor B's incident says its first run carries one:\n%s", b)
	}
	for _, s := range []string{"a-03", "b-03"} {
		if res = run(t, pool, &w, s); len(atKeys(res.Written)) != 0 {
			t.Errorf("%s wrote %v; want it joined to its source's new incident", s, res.Written)
		}
	}
	if !grouped(res, gotB) {
		t.Errorf("b-03 grouped %v, want into %s", res.Grouped, gotB)
	}
	if got := incidentsAt(t, pool, wf, ""); len(got) != 3 {
		t.Errorf("worker_failed incidents %v: want the old one and one new per source", got)
	}
	if src, null := sourceOf(t, pool, newA); null || src != srcA {
		t.Errorf("%s records source %+v (null %v)", newA, src, null)
	}
}

// A run whose main input held no records, against a real Postgres: present at
// main_input_empty and raised there, and not evaluable at step_never_started,
// so no step incident; vendor A sending empty files day after day is one
// incident. When the feed's profile declares empty deliveries expected, the
// next empty run raises nothing.
func TestAnEmptyMainInputIsOneIncidentUntilDeclared(t *testing.T) {
	pool := freshDB(t, "shadow_main_input_empty", sourceConditionTables, true)
	ctx := context.Background()
	w := testWriter()
	w.Baseline = 7 * 24 * time.Hour
	me := triage.LocusMainInputEmpty
	// Vendor A's runs with records, so it has a step baseline, then two empty.
	for i, s := range []string{"a-01", "a-02", "a-03"} {
		plantSourceRun(t, pool, srcA, s, float64(i+1), false, 100)
	}
	plantSourceRun(t, pool, srcA, "a-04", 4, false, 0)
	plantSourceRun(t, pool, srcA, "a-05", 5, false, 0)

	ie := IncidentIdFor("a-04", me)
	res := run(t, pool, &w, "a-04")
	if got := res.Written; len(got) != 1 || got[0].IncidentId != ie {
		t.Fatalf("a-04, empty: wrote %v, want %s alone", got, ie)
	}
	for _, f := range res.Report.Findings {
		switch f.Locus {
		case me:
			if f.Verdict != triage.Present {
				t.Errorf("main_input_empty is %s: %s", f.Verdict, f.Basis)
			}
		case triage.LocusStepNeverStarted:
			if f.Verdict != triage.NotEvaluable {
				t.Errorf("step_never_started on an empty run is %s: %s", f.Verdict, f.Basis)
			}
		}
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, ie).
		Scan(&status); err != nil || status != "triaged" {
		t.Errorf("incident %s is at %q (%v); no cause class maps to the locus, so it stays triaged", ie, status, err)
	}
	if res = run(t, pool, &w, "a-05"); len(res.Written) != 0 || !grouped(res, ie) {
		t.Errorf("a-05, empty again: wrote %v, grouped %v; want joined to %s", res.Written, res.Grouped, ie)
	}

	// Declared: the next empty run is absent at the locus, and that ends the
	// condition, raising nothing.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.source_config
		(client, org, object_type, table_name, feed_profile_json, user_email)
		VALUES ($1, $2, $3, 'vendor_a_claims',
		        '{"empty_deliveries_expected":true,"declared_by":"someone@example.com","declared_at":"2026-09-01"}'::jsonb,
		        'test@test')`, srcA.Client, srcA.Org, srcA.ObjectType); err != nil {
		t.Fatal(err)
	}
	plantSourceRun(t, pool, srcA, "a-06", 6, false, 0)
	res = run(t, pool, &w, "a-06")
	if len(res.Written) != 0 || len(res.Grouped) != 0 {
		t.Errorf("a-06, empty and declared: wrote %v, grouped %v; want nothing", res.Written, res.Grouped)
	}
	if got := incidentsAt(t, pool, triage.LocusStepNeverStarted, ""); len(got) != 0 {
		t.Errorf("an empty run raised step_never_started incidents %v", got)
	}
}
