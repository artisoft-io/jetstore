package retention

// Tests against a real PostgreSQL, skipped unless JETS_TEST_DSN names one; each
// rebuilds the database agentic_retention. Run with -count=1, and with -p 1
// beside other packages that install the schema in the same server:
//
//	docker run -d --rm -p 55443:5432 -e POSTGRES_PASSWORD=pw postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55443/postgres \
//	  go test -count=1 -p 1 ./jets/agentic/retention/

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDB = "agentic_retention"

var ctx = context.Background()

func db(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := purgetest.DB(t, testDB)
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func count(t *testing.T, pool *pgxpool.Pool, stmt string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(ctx, stmt, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, stmt)
	}
	return n
}

// incident writes an incident on a session at a locus, with nHyp hypotheses,
// nEv events and nRem remediations, and returns its id.
func incident(t *testing.T, pool *pgxpool.Pool, session, locus string, nHyp, nEv, nRem int) string {
	t.Helper()
	id := "inc_" + session + "_" + locus
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version)
		VALUES ($1, $2, now(), $3, 'high', 'diagnosed', '{}', '0.1.0')`, id, session, locus)
	children(t, pool, id, locus, nHyp, nEv, nRem)
	return id
}

func children(t *testing.T, pool *pgxpool.Pool, id, locus string, nHyp, nEv, nRem int) {
	t.Helper()
	for i := range nHyp {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.hypothesis
			(hypothesis_id, hypothesis_incident_ref, cause, confidence, rank, supporting_evidence,
			 contradicting_evidence, hypothesis_locus, basis)
			VALUES ($1, $2, 'c', 0.5, $3, '[]', '[]', $4, '{}')`, fmt.Sprintf("hyp_%s_%d", id, i), id, i+1, locus)
	}
	for i := range nEv {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident_event
			(incident_event_id, event_incident_ref, from_status, to_status, event_actor, event_actor_kind,
			 transitioned_at)
			VALUES ($1, $2, 'detected', 'triaged', 'triage@1', 'agent', now())`, fmt.Sprintf("ev_%s_%d", id, i), id)
	}
	for i := range nRem {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.remediation
			(remediation_id, incident_ref, autonomy_tier_required, reversibility, remediation_approval_state)
			VALUES ($1, $2, 'T2', 'fully_reversible', 'draft')`, fmt.Sprintf("rem_%s_%d", id, i), id)
	}
}

func rowsOf(t *testing.T, pool *pgxpool.Pool, id string) map[string]int64 {
	t.Helper()
	return map[string]int64{
		IncidentTable:      count(t, pool, `SELECT count(*) FROM jetsapi.incident WHERE incident_id = $1`, id),
		HypothesisTable:    count(t, pool, `SELECT count(*) FROM jetsapi.hypothesis WHERE hypothesis_incident_ref = $1`, id),
		RemediationTable:   count(t, pool, `SELECT count(*) FROM jetsapi.remediation WHERE incident_ref = $1`, id),
		IncidentEventTable: count(t, pool, `SELECT count(*) FROM jetsapi.incident_event WHERE event_incident_ref = $1`, id),
	}
}

func anomaly(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.anomaly
		(anomaly_id, detected_at, anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
		 anomaly_signal_type, anomaly_observed_value, anomaly_expected_basis, anomaly_confounders,
		 anomaly_detector_ref)
		VALUES ($1, now(), $2, 'run', 'proc', 'duration', 'x', 'history', '{}', 'duration@1')`, "a-"+session, session)
}

// An incident with no key goes, with every hypothesis, event and remediation,
// exactly when its own session is held no more: not while the session is
// registered, has a run, an input or an anomaly. A dry run reports the same
// deletion and deletes nothing.
func TestAnIncidentWithNoKeyGoesWithItsSession(t *testing.T) {
	pool := db(t)
	gone := incident(t, pool, "gone", "worker_failed", 3, 2, 1)
	// Held four ways.
	purgetest.Seed(t, pool, purgetest.Session{ID: "registered", Client: "c1", Registered: true})
	registered := incident(t, pool, "registered", "worker_failed", 2, 1, 0)
	purgetest.Seed(t, pool, purgetest.Session{ID: "failed", Client: "c1", Runs: []string{"failed"}})
	purgetest.Exec(t, pool, `DELETE FROM jetsapi.input_registry WHERE session_id = 'failed'`)
	failed := incident(t, pool, "failed", "worker_failed", 1, 1, 0)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('c1', 'o', 'k', 1, 'feed_a', 'file', 'input', 'system')`)
	input := incident(t, pool, "input", "delivery_overdue", 1, 1, 0)
	anomaly(t, pool, "anomaly")
	withAnomaly := incident(t, pool, "anomaly", "worker_failed", 1, 1, 0)
	held := []string{registered, failed, input, withAnomaly}

	before := rowsOf(t, pool, gone)
	want := map[string]int64{IncidentTable: 1, HypothesisTable: 3, RemediationTable: 1, IncidentEventTable: 2}
	if !reflect.DeepEqual(before, want) {
		t.Fatalf("seeded %v, want %v", before, want)
	}

	dry, err := Sweep(ctx, pool, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Candidates != 1 || dry.Deleted != 1 || dry.Held != 0 || !reflect.DeepEqual(dry.Rows, want) ||
		!slices.Equal(dry.DeletedIncidents, []string{gone}) {
		t.Errorf("dry run report %+v, want %s and its rows %v", dry, gone, want)
	}
	if got := rowsOf(t, pool, gone); !reflect.DeepEqual(got, want) {
		t.Errorf("the dry run deleted rows: %v left, want %v", got, want)
	}

	real, err := Sweep(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dry, real) {
		t.Errorf("the dry run predicted\n%+v\nthe real run did\n%+v", dry, real)
	}
	for table, n := range rowsOf(t, pool, gone) {
		if n != 0 {
			t.Errorf("%d rows of %s left in %s", n, gone, table)
		}
	}
	for _, id := range held {
		if n := rowsOf(t, pool, id)[IncidentTable]; n != 1 {
			t.Errorf("incident %s, on a session still held, was deleted", id)
		}
	}
	// A second sweep finds nothing.
	if again, err := Sweep(ctx, pool, Options{}); err != nil || again.Candidates != 0 || again.Deleted != 0 {
		t.Errorf("the second sweep: %+v, %v", again, err)
	}
}

// A sweep past its deadline starts nothing and says how many it left.
func TestSweepDeadline(t *testing.T) {
	pool := db(t)
	incident(t, pool, "a", "worker_failed", 1, 0, 0)
	incident(t, pool, "b", "worker_failed", 1, 0, 0)
	r, err := Sweep(ctx, pool, Options{Deadline: time.Now().Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates != 2 || r.NotReached != 2 || r.Deleted != 0 {
		t.Errorf("report %+v, want 2 not reached", r)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident`); n != 2 {
		t.Errorf("%d incidents left, want 2", n)
	}
}

// A database without the agentic tables has nothing to sweep.
func TestSweepWithoutAgenticTables(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	r, err := Sweep(ctx, pool, Options{})
	if err != nil || r.Installed || r.Candidates != 0 {
		t.Errorf("report %+v, %v", r, err)
	}
}

// runAt writes a live run of process proc for client c1 starting at start,
// with its verdict at worker_failed; "" writes none.
func runAt(t *testing.T, pool *pgxpool.Pool, session string, start time.Time, verdict string) {
	t.Helper()
	purgetest.Seed(t, pool, purgetest.Session{ID: session, Client: "c1", Registered: true, LastUpdate: start})
	if verdict != "" {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.run_finding
			(session_id, locus, error_code, verdict, classifier_ref, classified_at)
			VALUES ($1, 'worker_failed', '', $2, 'triage@1', $3)`, session, verdict, start)
	}
}

// keyedRunIncident writes a run incident carrying its key and position.
func keyedRunIncident(t *testing.T, pool *pgxpool.Pool, session string, start time.Time) string {
	t.Helper()
	id := "inc_" + session + "_worker_failed"
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version, incident_client, incident_process_name,
		 incident_anchor_start_time)
		VALUES ($1, $2, now(), 'worker_failed', 'high', 'diagnosed', '{}', '0.1.0', 'c1', 'proc', $3)`,
		id, session, start)
	children(t, pool, id, "worker_failed", 3, 2, 1)
	return id
}

func deleteRun(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	for _, stmt := range []string{
		`DELETE FROM jetsapi.run_finding WHERE session_id = $1`,
		`DELETE FROM jetsapi.pipeline_execution_status WHERE session_id = $1`,
		`DELETE FROM jetsapi.input_registry WHERE session_id = $1`,
		`DELETE FROM jetsapi.session_registry WHERE session_id = $1`,
	} {
		purgetest.Exec(t, pool, stmt, session)
	}
}

// A run incident whose own run is deleted is kept while a later live run of its
// condition is placed in it, and goes, with its rows, once none is; an incident
// of another condition at the key, ended by a run whose finding is absent, goes
// when its own run does.
func TestARunIncidentGoesWhenNoRunOfItsConditionIsHeld(t *testing.T) {
	pool := db(t)
	t0 := time.Now().Add(-48 * time.Hour).UTC().Truncate(time.Second)
	at := func(h int) time.Time { return t0.Add(time.Duration(h) * time.Hour) }
	// r1 r2 r3 present (one condition, its incident on r1), r4 absent, r5
	// present (a second condition, its incident on r5).
	for i, v := range []string{"present", "present", "present", "absent", "present"} {
		runAt(t, pool, fmt.Sprintf("r%d", i+1), at(i), v)
	}
	first := keyedRunIncident(t, pool, "r1", at(0))
	second := keyedRunIncident(t, pool, "r5", at(4))

	deleteRun(t, pool, "r1")
	r, err := Sweep(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates != 1 || r.Held != 1 || r.Deleted != 0 {
		t.Errorf("with r2 and r3 placed in it: %+v, want it kept", r)
	}
	deleteRun(t, pool, "r2")
	deleteRun(t, pool, "r3")
	deleteRun(t, pool, "r5")
	// r4 is absent: it ends the first condition and holds neither incident.
	r, err = Sweep(ctx, pool, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates != 2 || r.Deleted != 2 || r.Rows[HypothesisTable] != 6 {
		t.Errorf("dry run with no run of either condition held: %+v", r)
	}
	if rowsOf(t, pool, first)[IncidentTable] != 1 || rowsOf(t, pool, second)[IncidentTable] != 1 {
		t.Error("the dry run deleted an incident")
	}
	if _, err = Sweep(ctx, pool, Options{}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first, second} {
		for table, n := range rowsOf(t, pool, id) {
			if n != 0 {
				t.Errorf("%d rows of %s left in %s", n, id, table)
			}
		}
	}
}

func TestSessionOfIncidentId(t *testing.T) {
	for _, c := range []struct{ id, session, locus string }{
		{"inc_1764000000000_delivery_overdue", "1764000000000", "delivery_overdue"},
		{"inc_s-golden_worker_failed", "s-golden", "worker_failed"},
		{"inc_17_per_record_failures_reported_map_record_error", "17", "per_record_failures_reported"},
		{"inc_a_b_run_not_started", "a_b", "run_not_started"},
	} {
		s, l, ok := SessionOfIncidentId(c.id)
		if !ok || s != c.session || l != c.locus {
			t.Errorf("%s: %q %q %v, want %q %q", c.id, s, l, ok, c.session, c.locus)
		}
	}
	for _, id := range []string{"run_1", "inc_", "inc_worker_failed", "inc_1_no_such_locus", "hyp_inc_1_worker_failed_x"} {
		if _, _, ok := SessionOfIncidentId(id); ok {
			t.Errorf("%s read as an incident id", id)
		}
	}
	text := "The previous incident at this locus is inc_17_delivery_overdue, on session 17. See inc_18_worker_failed."
	if got := IncidentIdsIn(text); !slices.Equal(got, []string{"inc_17_delivery_overdue", "inc_18_worker_failed"}) {
		t.Errorf("IncidentIdsIn: %v", got)
	}
}

// An id the database holds no incident under, whose session it does not hold
// either, is purged; with the session's deletion recorded, it is said to be.
func TestReadPurged(t *testing.T) {
	pool := db(t)
	purgetest.Seed(t, pool, purgetest.Session{ID: "live", Client: "c1", Registered: true})
	held := incident(t, pool, "live", "worker_failed", 0, 0, 0)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion
		(session_id, client, reason, requested_by, status, completed_at)
		VALUES ('deleted', 'c1', 'retention', 'purge', 'done', now())`)
	got, err := ReadPurged(ctx, pool, []string{held, "inc_live_delivery_overdue", "inc_deleted_worker_failed",
		"inc_never_worker_failed", "not-an-incident"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Purged{{IncidentId: "inc_deleted_worker_failed", SessionId: "deleted", Deleted: true},
		{IncidentId: "inc_never_worker_failed", SessionId: "never"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ReadPurged %+v, want %+v", got, want)
	}
}
