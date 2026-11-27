// A run judged against its own main input source's runs, a step that stopped
// running leaving what is expected, and a run whose main input held no records.
//
// Needs JETS_TEST_DSN; run with -count=1.
package triage

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/jackc/pgx/v5/pgxpool"
)

var sourceTables = append(slices.Clone(migratedTables), "input_registry", "source_config")

// registerMainInput registers a run's main input under a source and points
// the run's header at it, as a run started from a registered file is.
func registerMainInput(t *testing.T, pool *pgxpool.Pool, execKey int, src observe.Source, session string) {
	t.Helper()
	ctx := context.Background()
	var irKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ($1, $2, $3, $4, 1, $5, 'file', $6, 'test@test') RETURNING key`,
		src.Client, src.Org, src.ObjectType, "in/"+session, src.Org+"_"+src.ObjectType, "reg-"+session).
		Scan(&irKey); err != nil {
		t.Fatalf("registering the main input of %s: %v", session, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_status SET main_input_registry_key = $1
		WHERE key = $2`, irKey, execKey); err != nil {
		t.Fatalf("pointing %s at its main input: %v", session, err)
	}
}

// sourceRun writes a completed run of process loader for client1 from src,
// hours after now, with a worker row for each step: the sharding step read
// records records, and every other step 100.
func sourceRun(t *testing.T, pool *pgxpool.Pool, src observe.Source, session string, hours int, records int,
	steps ...string) {
	t.Helper()
	start := now.Add(time.Duration(hours) * time.Hour)
	k := header(t, pool, "client1", "loader", "claims", session, "completed", start)
	registerMainInput(t, pool, k, src, session)
	workerRow(t, pool, k, "client1", "loader", session, ShardingStepId, 0, "completed", records, records, "", start)
	for i, s := range steps {
		workerRow(t, pool, k, "client1", "loader", session, s, i+1, "completed", 100, 100, "",
			start.Add(time.Minute))
	}
}

var (
	vendorA = observe.Source{Client: "client1", Org: "vendor_a", ObjectType: "claims"}
	vendorB = observe.Source{Client: "client1", Org: "vendor_b", ObjectType: "claims"}
)

// A run is compared with its own source's runs. Vendor A's files reach the
// sharding step and one more; vendor B's reach two more. A vendor A run with
// both of its steps is absent at step_never_started, though vendor B's runs,
// interleaved with it, all have a step it does not; judged against the process
// as a whole, it would be present.
func TestAStepIsExpectedOnlyOfItsOwnSourcesRuns(t *testing.T) {
	pool := freshDB(t, "triage_source_baseline", sourceTables)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		sourceRun(t, pool, vendorA, "a"+string(rune('1'+i)), -48+2*i, 50, "reducing01")
		sourceRun(t, pool, vendorB, "b"+string(rune('1'+i)), -47+2*i, 80, "reducing01", "reducing02")
	}
	sourceRun(t, pool, vendorA, "a-judged", 0, 50, "reducing01")

	ev, err := Gather(ctx, pool, "a-judged", 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Header.Source != vendorA {
		t.Fatalf("the header's source is %+v, want %+v", ev.Header.Source, vendorA)
	}
	for _, b := range ev.Prior.Baselines {
		if b.Source != vendorA || b.StepId == "reducing02" {
			t.Errorf("the baseline of a vendor A run holds %+v", b)
		}
	}
	f := wantVerdict(t, Default().Classify(ev), LocusStepNeverStarted, Absent)
	if !strings.Contains(f.Basis, vendorA.Describe()) {
		t.Errorf("the basis does not name the run's source: %s", f.Basis)
	}

	// A vendor B run missing reducing02 is present: the step is its source's.
	sourceRun(t, pool, vendorB, "b-judged", 1, 80, "reducing01")
	f = wantVerdict(t, classify(t, pool, "b-judged", 7*24*time.Hour), LocusStepNeverStarted, Present)
	if f.StepRef != "reducing02" {
		t.Errorf("the vendor B run is missing %q, want reducing02: %s", f.StepRef, f.Basis)
	}
	// The basis says what the header records of the workspace version, rather
	// than that no run is bound to one.
	if strings.Contains(f.Basis, "no run is bound to a workspace version") ||
		!strings.Contains(f.Basis, "records the workspace version it ran under") {
		t.Errorf("the basis's workspace-version sentence is not the corrected one: %s", f.Basis)
	}
}

// A step that stops running is expected of at most MinPriorRuns later runs of
// its source, and of none after: the fourth run after it stopped no longer has
// it among the source's last three.
func TestAStepThatStoppedIsExpectedOfAtMostThreeRuns(t *testing.T) {
	pool := freshDB(t, "triage_retired_step", sourceTables)
	c := Default()
	if c.MinPriorRuns != 3 {
		t.Fatalf("MinPriorRuns is %d; this fixture is written for 3", c.MinPriorRuns)
	}
	for i := 1; i <= 4; i++ {
		sourceRun(t, pool, vendorA, "with-"+string(rune('0'+i)), -40+i, 50, "reducing01", "reducing02")
	}
	var after []string
	for i := 1; i <= 5; i++ {
		s := "after-" + string(rune('0'+i))
		sourceRun(t, pool, vendorA, s, -20+i, 50, "reducing01")
		after = append(after, s)
	}
	var expected []string
	for _, s := range after {
		f := verdictOf(t, classify(t, pool, s, 7*24*time.Hour), LocusStepNeverStarted)
		switch f.Verdict {
		case Present:
			expected = append(expected, s)
			if f.StepRef != "reducing02" {
				t.Errorf("%s is missing %q, want reducing02", s, f.StepRef)
			}
		case Absent:
			if !strings.Contains(f.Basis, "the source no longer runs it") {
				t.Errorf("%s is absent without saying reducing02 was retired: %s", s, f.Basis)
			}
		default:
			t.Errorf("%s is %s at step_never_started: %s", s, f.Verdict, f.Basis)
		}
	}
	if want := after[:3]; !slices.Equal(expected, want) {
		t.Errorf("the stopped step is expected of %v, want the three runs after it stopped, %v", expected, want)
	}
}

// A run whose sharding step read 0 records is present at main_input_empty and
// not evaluable at step_never_started, though its source's earlier runs all
// had later steps; when its feed's profile declares empty deliveries expected,
// it is absent at main_input_empty, naming the declaration.
func TestAnEmptyMainInputIsItsOwnLocus(t *testing.T) {
	pool := freshDB(t, "triage_main_input_empty", sourceTables)
	ctx := context.Background()
	for i := 1; i <= 4; i++ {
		sourceRun(t, pool, vendorA, "full-"+string(rune('0'+i)), -40+i, 50, "reducing01", "reducing02")
	}
	sourceRun(t, pool, vendorA, "empty-1", 0, 0)

	r := classify(t, pool, "empty-1", 7*24*time.Hour)
	f := wantVerdict(t, r, LocusMainInputEmpty, Present)
	if !strings.Contains(f.Basis, "read 0 records") || !strings.Contains(f.Basis, vendorA.Describe()) {
		t.Errorf("the empty input's basis: %s", f.Basis)
	}
	if !strings.Contains(f.Basis, "no source configuration has client") {
		t.Errorf("the basis does not say no feed declaration was read: %s", f.Basis)
	}
	f = wantVerdict(t, r, LocusStepNeverStarted, NotEvaluable)
	if !strings.Contains(f.Basis, "read 0 records") || !strings.Contains(f.Basis, LocusMainInputEmpty) {
		t.Errorf("step_never_started does not say why it is set aside: %s", f.Basis)
	}
	// A run with records is absent at the locus.
	wantVerdict(t, classify(t, pool, "full-4", 7*24*time.Hour), LocusMainInputEmpty, Absent)

	// The feed of the source, profiled without the declaration: still present.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.source_config
		(client, org, object_type, table_name, feed_profile_json, user_email)
		VALUES ($1, $2, $3, 'vendor_a_claims',
		        '{"declared_by":"someone@example.com","declared_at":"2026-11-01"}'::jsonb, 'test@test')`,
		vendorA.Client, vendorA.Org, vendorA.ObjectType); err != nil {
		t.Fatal(err)
	}
	f = wantVerdict(t, classify(t, pool, "empty-1", 7*24*time.Hour), LocusMainInputEmpty, Present)
	if !strings.Contains(f.Basis, "does not declare empty deliveries expected") {
		t.Errorf("the basis does not say the profile makes no declaration: %s", f.Basis)
	}

	// Declared: absent, naming the declaration, and step_never_started still
	// set aside, so the run raises nothing.
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.source_config SET feed_profile_json =
		'{"empty_deliveries_expected":true,"declared_by":"someone@example.com","declared_at":"2026-11-02"}'::jsonb
		WHERE org = $1`, vendorA.Org); err != nil {
		t.Fatal(err)
	}
	r = classify(t, pool, "empty-1", 7*24*time.Hour)
	f = wantVerdict(t, r, LocusMainInputEmpty, Absent)
	if !strings.Contains(f.Basis, "empty_deliveries_expected") || !strings.Contains(f.Basis, "2026-11-02") {
		t.Errorf("the declared absence does not name the declaration: %s", f.Basis)
	}
	wantVerdict(t, r, LocusStepNeverStarted, NotEvaluable)
	if fired := r.Loci(); len(fired) != 0 {
		t.Errorf("a declared empty run fires %v", fired)
	}

	// A run whose sharding step has not completed is not evaluable.
	k := header(t, pool, "client1", "loader", "claims", "running", "completed", now.Add(time.Hour))
	registerMainInput(t, pool, k, vendorA, "running")
	workerRow(t, pool, k, "client1", "loader", "running", ShardingStepId, 0, "", 0, 0, "", now.Add(time.Hour))
	wantVerdict(t, classify(t, pool, "running", 0), LocusMainInputEmpty, NotEvaluable)
}
