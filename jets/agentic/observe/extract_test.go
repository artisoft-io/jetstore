// The extraction against a real Postgres. The execution tables are built from
// jets/jets_schema.json by the same jets/schema code `update_db -migrateDb`
// runs, so these queries are checked against the deployed shape rather than
// against a hand-copied DDL that would drift from it.
//
// Needs JETS_TEST_DSN (any throwaway database; the suite installs the schema);
// skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/agentic/observe/
package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaPath = "../../jets_schema.json"

// The three tables this package reads and writes. pipeline_execution_channel_details
// is deliberately not installed by default: ReadExtent's report that it is
// absent is a case worth testing, and it is the state every production
// environment measured on 2026-08-25 was in.
var executionTables = []string{"pipeline_execution_status", "pipeline_execution_details"}

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to %s: %v", dsn, err)
	}
	t.Cleanup(pool.Close)

	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("opening %s: %v", schemaPath, err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatalf("decoding %s: %v", schemaPath, err)
	}
	want := map[string]bool{}
	for _, n := range executionTables {
		want[n] = true
	}
	found := 0
	for i := range defs {
		if !want[defs[i].TableName] {
			continue
		}
		found++
		if err := defs[i].UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("installing %s: %v", defs[i].TableName, err)
		}
	}
	if found != len(executionTables) {
		t.Fatalf("found %d of the %d execution tables in %s", found, len(executionTables), schemaPath)
	}
	// jetsapi.anomaly rides the audit store's generated DDL, which is where
	// update_db installs it from too (jets/update_db/main.go:71).
	if err := audit.InstallSchema(context.Background(), pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// fixture builds one run: a header (unless headless) and its worker rows. It
// returns the header key, 0 when there is none.
type worker struct {
	stepId  string
	status  string
	in, out *int64
}

func i64(v int64) *int64 { return &v }

func insertRun(t *testing.T, pool *pgxpool.Pool, client, process, objectType, session string,
	runStatus string, headless bool, start time.Time, workers []worker) int64 {
	t.Helper()
	ctx := context.Background()

	var headerKey int64
	if !headless {
		err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, client, process_name, main_object_type, session_id,
			 source_period_key, status, user_email, start_time, last_update)
			VALUES (1, $1, $2, $3, $4, 1, $5, 'test@test', $6, $6) RETURNING key`,
			client, process, objectType, session, runStatus, start).Scan(&headerKey)
		if err != nil {
			t.Fatalf("inserting run header: %v", err)
		}
	} else {
		// A worker row whose header has been purged still points at a key; the
		// row it named is gone. 0 is not a valid key, so use a value no header
		// has.
		headerKey = 900000 + int64(len(session))
	}

	for i, w := range workers {
		var key int64
		err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_details
			(pipeline_config_key, pipeline_execution_status_key, client, process_name,
			 main_input_session_id, session_id, source_period_key, shard_id, jets_partition,
			 cpipes_step_id, status, user_email, start_time, last_update)
			VALUES (1, $1, $2, $3, $4, $4, 1, $5, '', $6, $7, 'test@test', $8, $8) RETURNING key`,
			headerKey, client, process, session, i, w.stepId, w.status, start).Scan(&key)
		if err != nil {
			t.Fatalf("inserting worker row: %v", err)
		}
		// The counts are set by the update and are NULL until it runs, which
		// is what a worker still in progress looks like.
		if w.in != nil || w.out != nil {
			if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_details
				SET input_records_count = $1, output_records_count = $2 WHERE key = $3`,
				w.in, w.out, key); err != nil {
				t.Fatalf("updating worker counts: %v", err)
			}
		}
	}
	return headerKey
}

func uniq(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("t%d", time.Now().UnixNano())
}

// The purge asymmetry, which is the reason every join to the header in this
// package is an outer one: in three of the four production environments
// measured on 2026-08-25 the worker rows outlive their headers, so an inner
// join drops the oldest part of the record without saying so.
func TestWorkersKeepsRowsWhoseHeaderIsPurged(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-time.Hour)

	insertRun(t, pool, client, "p", "obj", client+"_kept", "completed", false, start,
		[]worker{{stepId: "reducing00", status: StatusCompleted, in: i64(100), out: i64(100)}})
	insertRun(t, pool, client, "p", "obj", client+"_orphan", "", true, start,
		[]worker{{stepId: "reducing00", status: StatusCompleted, in: i64(100), out: i64(100)}})

	set, err := Workers(context.Background(), pool, Window{
		Since: start.Add(-time.Minute), Client: client})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Rows) != 2 {
		t.Fatalf("got %d worker rows, want 2 — an inner join would give 1", len(set.Rows))
	}
	if set.Headerless != 1 {
		t.Errorf("Headerless = %d, want 1", set.Headerless)
	}
	for _, r := range set.Rows {
		if r.HasHeader && r.MainObjectType != "obj" {
			t.Errorf("kept row lost its object type: %q", r.MainObjectType)
		}
		if !r.HasHeader && r.MainObjectType != "" {
			t.Errorf("orphan row invented an object type: %q", r.MainObjectType)
		}
		// The self-keying columns are on the worker row either way, which is
		// what makes a within-run predicate join-free.
		if r.Client != client || r.ProcessName != "p" || r.SessionId == "" {
			t.Errorf("worker row is not self-keying: %+v", r)
		}
	}
}

// Row 4's predicate, and the reason it needs the header: 'in progress' is only
// an anomaly under a run that finished.
func TestStalledNeedsATerminalHeader(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-time.Hour)

	insertRun(t, pool, client, "p", "obj", client+"_done", "completed", false, start,
		[]worker{{stepId: "s1", status: StatusInProgress}})
	insertRun(t, pool, client, "p", "obj", client+"_running", "submitted", false, start,
		[]worker{{stepId: "s1", status: StatusInProgress}})
	insertRun(t, pool, client, "p", "obj", client+"_orphan", "", true, start,
		[]worker{{stepId: "s1", status: StatusInProgress}})

	set, err := Workers(context.Background(), pool, Window{
		Since: start.Add(-time.Minute), Client: client})
	if err != nil {
		t.Fatal(err)
	}
	stalled := 0
	for i := range set.Rows {
		r := &set.Rows[i]
		if r.Stalled() {
			stalled++
		}
		// The counts are NULL for exactly as long as a worker is running.
		if r.Status == StatusInProgress && r.InputRecords != nil {
			t.Errorf("an in-progress worker carries a count: %v", *r.InputRecords)
		}
	}
	if stalled != 1 {
		t.Fatalf("Stalled() fired %d times, want 1 (the run that finished; not the one still going, not the orphan)", stalled)
	}
}

// Row 3's substrate: one GROUP BY, and the whole of the "SQL" the decision
// turns on.
func TestStepBaselinesAggregatesPerSourceAndStep(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	base := time.Now().Add(-48 * time.Hour)

	for i := range 3 {
		insertRun(t, pool, client, "p", "obj", fmt.Sprintf("%s_ok%d", client, i), "completed", false,
			base.Add(time.Duration(i)*time.Hour),
			[]worker{{stepId: "reducing01", status: StatusCompleted, in: i64(100), out: i64(100)}})
	}
	insertRun(t, pool, client, "p", "obj", client+"_bad", "failed", false, base.Add(4*time.Hour),
		[]worker{{stepId: "reducing01", status: StatusFailed, in: i64(100), out: i64(0)}})
	// A second object type is a second source, and must not be folded in.
	insertRun(t, pool, client, "p", "other", client+"_other", "completed", false, base,
		[]worker{{stepId: "reducing01", status: StatusCompleted, in: i64(1), out: i64(1)}})
	// An empty step label is one of F52's ten reducing steps, and the baseline
	// has to say it cannot tell them apart.
	insertRun(t, pool, client, "p", "obj", client+"_nostep", "completed", false, base,
		[]worker{{stepId: "", status: StatusCompleted, in: i64(1), out: i64(1)}})
	// And an orphan, which is excluded from the aggregate and counted.
	insertRun(t, pool, client, "p", "obj", client+"_orphan", "", true, base,
		[]worker{{stepId: "reducing01", status: StatusCompleted, in: i64(1), out: i64(1)}})

	set, err := StepBaselines(context.Background(), pool, Window{
		Since: base.Add(-time.Hour), Client: client})
	if err != nil {
		t.Fatal(err)
	}
	if set.Headerless != 1 {
		t.Errorf("Headerless = %d, want 1", set.Headerless)
	}

	var main, other, nostep *StepBaseline
	for i := range set.Baselines {
		b := &set.Baselines[i]
		switch {
		case b.MainObjectType == "obj" && b.StepId == "reducing01":
			main = b
		case b.MainObjectType == "other":
			other = b
		case b.StepId == "":
			nostep = b
		}
	}
	if main == nil || other == nil || nostep == nil {
		t.Fatalf("expected three baselines, got %d: %+v", len(set.Baselines), set.Baselines)
	}
	if main.Runs != 4 || main.Completed != 3 || main.Failed != 1 {
		t.Errorf("main baseline = %d runs / %d completed / %d failed, want 4/3/1", main.Runs, main.Completed, main.Failed)
	}
	if !main.EverSucceeded() {
		t.Error("main baseline has three successes and says it has none")
	}
	if main.LastFailure == nil || main.LastSuccess == nil {
		t.Fatal("main baseline is missing a last success or a last failure")
	}
	if !main.LastFailure.After(*main.LastSuccess) {
		t.Error("row 3's predicate is not visible: the failure should be the most recent event")
	}
	if other.Runs != 1 {
		t.Errorf("a second object type was folded into the first: %+v", other)
	}

	// The confounders the record can establish, and only those.
	if !contains(nostep.Confounders, ConfounderStepLabelAmbiguous) {
		t.Errorf("an empty step label is not qualified: %v", nostep.Confounders)
	}
	if contains(main.Confounders, ConfounderStepLabelAmbiguous) {
		t.Errorf("a named step label is qualified as ambiguous: %v", main.Confounders)
	}
	if !contains(main.Confounders, ConfounderHistoryTruncated) {
		t.Errorf("a window holding an orphaned worker row is not qualified as truncated: %v", main.Confounders)
	}
	for _, c := range main.Confounders {
		if !contains(RecordConfounders, c) {
			t.Errorf("the extraction set %q, which is not readable off the record", c)
		}
	}
	if d := set.Describe(main); d == "" {
		t.Error("Describe produced no expected_basis")
	}
}

// ReadExtent is how a detector learns which of RETENTION_DAYS' three regimes
// the deployment is in, and whether the two tables update_db creates are there.
func TestReadExtentReportsTheDeploymentsOwnLimits(t *testing.T) {
	pool := testPool(t)
	e, err := ReadExtent(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Anomalies {
		t.Error("jetsapi.anomaly is absent after the audit DDL was installed")
	}
	if e.ChannelDetails {
		t.Error("pipeline_execution_channel_details reports present; this suite does not install it")
	}
	if e.Regime() == "" {
		t.Error("Regime() named nothing")
	}
}

// The emitter half of "SQL plus a Go emitter", against the generated table.
func TestInsertAnomaly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := uniq(t)

	mag := 0.94
	a := &Anomaly{
		AnomalyId:          id,
		SessionId:          id + "_s",
		SubjectType:        SubjectWorker,
		SubjectRef:         "1234",
		SignalType:         SignalStepRegression,
		ObservedValue:      "failed",
		ExpectedBasis:      "3 prior runs of this step, all completed",
		DeviationMagnitude: &mag,
		Confounders:        []string{ConfounderStepLabelAmbiguous, ConfounderHistoryTruncated},
		DetectorRef:        "step_regression@1",
	}
	if err := InsertAnomaly(ctx, pool, a); err != nil {
		t.Fatalf("inserting: %v", err)
	}

	var signal string
	var conf []string
	var min, max *string
	if err := pool.QueryRow(ctx, `SELECT anomaly_signal_type, anomaly_confounders,
		anomaly_expected_min, anomaly_expected_max FROM jetsapi.anomaly WHERE anomaly_id = $1`,
		id).Scan(&signal, &conf, &min, &max); err != nil {
		t.Fatal(err)
	}
	if signal != SignalStepRegression {
		t.Errorf("signal round-tripped as %q", signal)
	}
	if len(conf) != 2 {
		t.Errorf("confounders round-tripped as %v", conf)
	}
	// The three nullable properties are nullable because four of the six
	// derivable rows are within-run predicates with no range (I-126).
	if min != nil || max != nil {
		t.Errorf("an unset expected range came back non-null: %v %v", min, max)
	}

	// An empty confounder list is a claim and must be storable as one.
	b := *a
	b.AnomalyId = id + "_b"
	b.Confounders = nil
	b.DeviationMagnitude = nil
	if err := InsertAnomaly(ctx, pool, &b); err != nil {
		t.Fatalf("inserting an anomaly with no confounders: %v", err)
	}

	// And the vocabulary is enforced before the insert reaches Postgres.
	c := *a
	c.AnomalyId = id + "_c"
	c.SignalType = "vollume"
	if err := InsertAnomaly(ctx, pool, &c); err == nil {
		t.Error("a signal type outside the vocabulary was accepted")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
