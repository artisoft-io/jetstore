// The nine predicates against a real Postgres, over worker rows and DAG edges
// written by the cpipes worker's own writers.
//
// # What is production here and what is not
//
//   - The execution tables are installed from jets/jets_schema.json by the same
//     jets/schema code `update_db -migrateDb` runs, so every query is checked
//     against the deployed shape rather than a hand-copied DDL.
//   - jetsapi.incident, whose incident_locus_ck is the locus vocabulary, is
//     installed by audit.InstallSchema — the same call update_db makes
//     (jets/update_db/main.go:71).
//   - Worker rows are written by ComputePipesContext.InsertPipelineExecutionStatus
//     and UpdatePipelineExecutionStatus (actions_process_file.go:302, :320).
//   - DAG edges are written by AggregateChannelResults and
//     InsertChannelExecutionDetails (compute_pipes_results.go:193, :262), which
//     is what makes the folded-sink case in locus 5 a real fold rather than a
//     blank string this file typed.
//   - Run headers, process_errors rows and cpipes_execution_status rows are
//     composed here. Their production writers are the apiserver at submission,
//     a channel-fed table writer and the sharding/reducing starts, none of which
//     is reachable without S3, a compiled workspace and a state machine. So the
//     write path is real for the two tables the classifier reasons hardest over
//     and is not for the three it reads flat.
//
// # Two deployments rather than one
//
// The suite builds two databases: one with every table the classifier reads and
// one with only the two the execution record needs. The second is not a
// convenience — it is the state all four production environments measured on
// 2026-08-25 were in (I-132), and it is the only way to test that a locus whose
// table is absent reports not_evaluable rather than absent.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5457:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5457/postgres go test ./jets/agentic/triage/
//
// Use `go test -p 1` when running this beside jets/agentic/observe or
// jets/agentic/audit against one DSN: all three call audit.InstallSchema and two
// sessions issuing the same CREATE TABLE / CREATE TRIGGER sequence deadlock on
// catalogue locks (I-175). The separate databases below remove that for this
// package's own tests and not across packages.
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaPath = "../../jets_schema.json"

// The tables a fully migrated deployment has.
var migratedTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
	"pipeline_execution_channel_details", "process_errors", "cpipes_execution_status",
}

// The tables a deployment that has never run the migration has. It is the two
// the execution record itself needs, and it is deliberately not a subset chosen
// for convenience: process_errors and cpipes_execution_status are older tables
// that a lagging deployment does have, so the interesting shortfall is the
// channel-detail table alone. The set below is narrower than any real
// deployment on purpose, so that four loci report not_evaluable in one run.
var unmigratedTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
}

func dsnFor(t *testing.T, dbName string) string {
	t.Helper()
	base := os.Getenv("JETS_TEST_DSN")
	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}

// freshDB drops and recreates a database, installs the named execution tables
// and the agentic DDL, and returns a pool on it. A database per scenario group
// is what lets one run hold two deployments in two migration states.
func freshDB(t *testing.T, dbName string, tables []string) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); err != nil {
		admin.Close()
		t.Fatalf("dropping %s: %v", dbName, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		admin.Close()
		t.Fatalf("creating %s: %v", dbName, err)
	}
	admin.Close()

	pool, err := pgxpool.New(ctx, dsnFor(t, dbName))
	if err != nil {
		t.Fatalf("connecting to %s: %v", dbName, err)
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
	for _, n := range tables {
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
	if found != len(tables) {
		t.Fatalf("found %d of the %d requested tables in %s", found, len(tables), schemaPath)
	}
	// jetsapi.incident rides the audit store's generated DDL, from the same
	// call update_db makes. The classifier does not write it; the tests read
	// its CHECK constraint as the authority for the locus vocabulary.
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// --- fixture writers ---------------------------------------------------------

func header(t *testing.T, pool *pgxpool.Pool, client, process, objectType, session, status string,
	start time.Time) int {
	t.Helper()
	var key int
	err := pool.QueryRow(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id,
		 source_period_key, status, failure_details, user_email, start_time, last_update)
		VALUES (1, $1, $2, $3, $4, 1, $5, '', 'test@test', $6, $6) RETURNING key`,
		client, process, objectType, session, status, start).Scan(&key)
	if err != nil {
		t.Fatalf("inserting run header: %v", err)
	}
	return key
}

// workerRow writes one worker row the way a cpipes node does: the 'in progress'
// insert, then the update that carries the counts and the terminal status. Pass
// terminal == "" to leave the row as the insert left it, which is what a worker
// that never came back looks like and is where the six NULL counts come from.
func workerRow(t *testing.T, pool *pgxpool.Pool, execKey int, client, process, session, stepId string,
	shard int, terminal string, in, out int, errMsg string, start time.Time) int64 {
	t.Helper()
	cpCtx := &compute_pipes.ComputePipesContext{
		ComputePipesArgs: compute_pipes.ComputePipesArgs{
			ComputePipesNodeArgs: compute_pipes.ComputePipesNodeArgs{
				NodeId:          shard,
				PipelineExecKey: execKey,
			},
			ComputePipesCommonArgs: compute_pipes.ComputePipesCommonArgs{
				Client:            client,
				ProcessName:       process,
				SessionId:         session,
				InputSessionId:    session,
				SourcePeriodKey:   1,
				PipelineConfigKey: 1,
				UserEmail:         "test@test",
				MainInputStepId:   stepId,
			},
		},
	}
	key, err := cpCtx.InsertPipelineExecutionStatus(pool)
	if err != nil {
		t.Fatalf("InsertPipelineExecutionStatus: %v", err)
	}
	if terminal != "" {
		if err := cpCtx.UpdatePipelineExecutionStatus(pool, key,
			in, 0, 1, 1, 0, out, stepId, terminal, errMsg); err != nil {
			t.Fatalf("UpdatePipelineExecutionStatus: %v", err)
		}
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE jetsapi.pipeline_execution_details SET start_time = $1, last_update = $1 WHERE key = $2`,
		start, key); err != nil {
		t.Fatalf("backdating worker row: %v", err)
	}
	return int64(key)
}

// edges writes a worker's per-channel detail rows through the production
// aggregation and insert, so the SinksCount > 1 fold that blanks output_entity
// happens in AggregateChannelResults rather than here.
func edges(t *testing.T, pool *pgxpool.Pool, parentKey int64, session string,
	results []compute_pipes.ComputePipesResult) {
	t.Helper()
	err := compute_pipes.InsertChannelExecutionDetails(pool, int(parentKey), session,
		compute_pipes.AggregateChannelResults(results))
	if err != nil {
		t.Fatalf("InsertChannelExecutionDetails: %v", err)
	}
}

func processErrors(t *testing.T, pool *pgxpool.Pool, execKey int, session string, n int,
	withColumn int) {
	t.Helper()
	for i := 0; i < n; i++ {
		col := ""
		if i < withColumn {
			col = "member_dob"
		}
		_, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, input_column, error_message, shard_id)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			execKey, session, fmt.Sprintf("k%d", i), col, "value is not a valid date", i%2)
		if err != nil {
			t.Fatalf("inserting process_errors row: %v", err)
		}
	}
}

// storeConfig puts a configuration where a run's configuration lives.
//
// **It names sharding_config_json and reducing_config_json, and the production
// writer does not** — which is I-305 and is a defect in jets/schema rather than
// a quirk of this fixture. Both columns are marked "deleted": true in
// jets_schema.json and both are declared NOT NULL with no default.
// TableDefinition.UpdateTable drops a deleted column (schema.go:305) and
// TableDefinition.CreateTable does not check the flag at all (schema.go:240),
// so an existing deployment loses them at the next migration and a **freshly
// created** database gets them. On such a database the sharding start's own
// insert (actions_start_sharding_cp.go:385) names six columns, not these two,
// and fails with a not-null violation — reproduced verbatim on 2026-09-04.
//
// **Running the migration twice clears it**, because the second pass finds the
// table and takes UpdateTable. That is the remedy and it is also why the defect
// is invisible: every environment that has been migrated more than once is fine
// for ever after, and only a brand-new deployment's first run meets it.
//
// observe's own suite does not see it either, and by luck rather than by
// design: its testPool calls UpdateTableSchema at every test, and the first
// test that calls storeConfig is the fourth, by which point the second pass has
// already dropped the columns. Verified 2026-09-04 against a virgin schema —
// the suite is green. The databases below are created per run and per test
// group, so the first pass is the only pass, which is why this surfaced here.
func storeConfig(t *testing.T, pool *pgxpool.Pool, execKey int, session, config string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_config_json,
		 sharding_config_json, reducing_config_json) VALUES ($1, $2, $3, '', '')`,
		execKey, session, config)
	if err != nil {
		t.Fatalf("inserting cpipes_execution_status row: %v", err)
	}
}

// TestFreshInstallCannotWriteTheRunConfiguration is I-305 as an assertion
// rather than a comment. It runs the sharding start's insert statement verbatim
// against a freshly created schema and requires it to fail, so that the day
// somebody fixes CreateTable this test fails and says which defect went away.
func TestFreshInstallCannotWriteTheRunConfiguration(t *testing.T) {
	pool := freshDB(t, "triage_freshinstall", migratedTables)
	// The statement from actions_start_sharding_cp.go:385, column for column.
	_, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_config_json, input_parquet_schema_json,
		 cpipes_startup_json, input_row_columns_json)
		VALUES (1, 'probe', '{}', '{}', '{}', '{}')`)
	if err == nil {
		t.Skip("the sharding start's insert now succeeds on a fresh schema: I-305 appears to be fixed, " +
			"and storeConfig above can drop its two extra columns")
	}
	if !strings.Contains(err.Error(), "sharding_config_json") {
		t.Fatalf("the insert failed for some other reason than I-305: %v", err)
	}
	t.Logf("I-305 reproduced: %v", err)
}

// Two configurations in the shape cpipes_config_json holds: a list of PipeSpec.
// The first is the corpus's ordinary state — a map_record with no error channel,
// which is 243 of 243 instances across the four workspaces (F188). The second is
// the 9-of-12 jetrules case that does declare one.
const configNoErrorChannel = `[{"type":"fan_out","input_channel":{"name":"input"},
  "apply":[{"type":"map_record","map_record_config":{},"output_channel":{"name":"mapped"}},
           {"type":"sort","sort_config":{},"output_channel":{"name":"sorted"}}]}]`

const configWithErrorChannel = `[{"type":"fan_out","input_channel":{"name":"input"},
  "apply":[{"type":"jetrules","jetrules_config":{"error_channel":{"name":"process_errors.out"}},
            "output_channel":{"name":"ruled"}}]}]`

// --- helpers -----------------------------------------------------------------

func verdictOf(t *testing.T, r *Report, locus string) Finding {
	t.Helper()
	for i := range r.Findings {
		if r.Findings[i].Locus == locus {
			return r.Findings[i]
		}
	}
	t.Fatalf("no finding for locus %s", locus)
	return Finding{}
}

func wantVerdict(t *testing.T, r *Report, locus string, want Verdict) Finding {
	t.Helper()
	f := verdictOf(t, r, locus)
	if f.Verdict != want {
		t.Errorf("locus %s: verdict %q, want %q\n  basis: %s", locus, f.Verdict, want, f.Basis)
	}
	return f
}

func classify(t *testing.T, pool *pgxpool.Pool, session string, baseline time.Duration) *Report {
	t.Helper()
	ev, err := Gather(context.Background(), pool, session, baseline)
	if err != nil {
		t.Fatalf("Gather(%s): %v", session, err)
	}
	r := Default().Classify(ev)
	assertAllValid(t, r)
	return r
}

var now = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// --- the nine, on a migrated deployment --------------------------------------

func TestPredicatesOnAMigratedDeployment(t *testing.T) {
	pool := freshDB(t, "triage_migrated", migratedTables)

	// An old, ordinary run so the database has a worker record reaching back
	// before every session below. Without it, locus run_not_started reports
	// not_evaluable for the purge reason rather than firing — which is a real
	// case and is tested separately.
	oldKey := header(t, pool, "cgt", "loader", "claims", "old", "completed", now.Add(-72*time.Hour))
	workerRow(t, pool, oldKey, "cgt", "loader", "old", "reducing00", 0, "completed", 5000, 5000, "", now.Add(-72*time.Hour))

	t.Run("run_not_started", func(t *testing.T) {
		header(t, pool, "cgt", "loader", "claims", "s-nostart", "failed", now.Add(-time.Hour))
		r := classify(t, pool, "s-nostart", 0)
		f := wantVerdict(t, r, LocusRunNotStarted, Present)
		if !strings.Contains(f.Basis, "no pipeline_execution_details row") {
			t.Errorf("basis does not say what it looked at: %s", f.Basis)
		}
		// Every other locus that needs a worker row must be not_evaluable
		// rather than absent: there is nothing to have been clean.
		for _, l := range []string{LocusWorkerNotTerminated, LocusWorkerFailed, LocusRowsLostSilently} {
			wantVerdict(t, r, l, NotEvaluable)
		}
	})

	t.Run("run_not_started is not evaluable when the run predates the worker record", func(t *testing.T) {
		// A header older than the oldest worker row anywhere: the purge takes
		// the two on independent clocks (F54), so a run whose workers have been
		// purged is indistinguishable from one that never started.
		header(t, pool, "cgt", "loader", "claims", "s-purged", "completed", now.Add(-200*24*time.Hour))
		r := classify(t, pool, "s-purged", 0)
		f := wantVerdict(t, r, LocusRunNotStarted, NotEvaluable)
		if !strings.Contains(f.Basis, "RETENTION_DAYS") {
			t.Errorf("basis does not name the purge: %s", f.Basis)
		}
	})

	t.Run("worker_not_terminated", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-stall", "failed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-stall", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-stall", "reducing01", 1, "", 0, 0, "", now.Add(-time.Hour))
		r := classify(t, pool, "s-stall", 0)
		f := wantVerdict(t, r, LocusWorkerNotTerminated, Present)
		if !strings.Contains(f.Basis, "cpipes_execution_status_details") {
			t.Errorf("basis does not carry F193's finding that the step aggregate hides the row: %s", f.Basis)
		}
		// One stalled worker of two, so the finding localises to its shard.
		if f.ShardRef == nil || *f.ShardRef != 1 {
			t.Errorf("shard_ref is %v, want 1", f.ShardRef)
		}
		// And to no step, ever. cpipes_step_id is written only by
		// UpdatePipelineExecutionStatus, and a worker that never terminated is
		// exactly one the update never reached — so this locus cannot localise
		// to a step by construction, whatever the configuration says (I-302).
		// The worker was created with MainInputStepId "reducing01" and the row
		// still carries an empty label.
		if f.StepRef != "" {
			t.Errorf("step_ref is %q; a stalled worker has no cpipes_step_id to name", f.StepRef)
		}
		if !strings.Contains(f.Basis, "none of them can say which step it is") {
			t.Errorf("basis does not say the step is unknowable here: %s", f.Basis)
		}
		wantVerdict(t, r, LocusWorkerFailed, Absent)
	})

	t.Run("worker_failed", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-fail", "failed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-fail", "reducing01", 0, "failed", 100, 0,
			"while reading file s3://b/k, unexpected EOF,while writing to jetsapi.claims, deadlock detected",
			now.Add(-time.Hour))
		r := classify(t, pool, "s-fail", 0)
		f := wantVerdict(t, r, LocusWorkerFailed, Present)
		// The comma count is reported as the upper bound it is: the message
		// holds three commas and two errors, which is the whole of F191.
		if !strings.Contains(f.Basis, "4 comma-separated segments is an upper bound") {
			t.Errorf("basis does not report the comma count as an upper bound: %s", f.Basis)
		}
		wantVerdict(t, r, LocusWorkerNotTerminated, Absent)
	})

	t.Run("rows_lost_silently", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-collapse", "completed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-collapse", "reducing01", 0, "completed", 10000, 12, "", now.Add(-time.Hour))
		storeConfig(t, pool, k, "s-collapse", configNoErrorChannel)
		r := classify(t, pool, "s-collapse", 0)
		f := wantVerdict(t, r, LocusRowsLostSilently, Present)
		if !strings.Contains(f.Basis, "volume_collapse@1") {
			t.Errorf("basis does not name the detector it ran: %s", f.Basis)
		}
		if !strings.Contains(f.Basis, "unsourced") {
			t.Errorf("the one locus carrying a threshold does not say so: %s", f.Basis)
		}
	})

	t.Run("rows_lost_silently is absent on a healthy ratio", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-clean", "completed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-clean", "reducing01", 0, "completed", 10000, 9800, "", now.Add(-time.Hour))
		storeConfig(t, pool, k, "s-clean", configWithErrorChannel)
		r := classify(t, pool, "s-clean", 0)
		wantVerdict(t, r, LocusRowsLostSilently, Absent)
		wantVerdict(t, r, LocusWorkerFailed, Absent)
		wantVerdict(t, r, LocusWorkerNotTerminated, Absent)
		wantVerdict(t, r, LocusRunNotStarted, Absent)
		wantVerdict(t, r, LocusPerRecordFailuresReported, Absent)
		wantVerdict(t, r, LocusPerRecordFailuresUnreportable, Absent)
	})

	t.Run("per_record_failures_reported", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-perr", "completed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-perr", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
		processErrors(t, pool, k, "s-perr", 20, 7)
		r := classify(t, pool, "s-perr", 0)
		f := wantVerdict(t, r, LocusPerRecordFailuresReported, Present)
		if !strings.Contains(f.Basis, "lower bound") {
			t.Errorf("basis does not report the cap: %s", f.Basis)
		}
		if !strings.Contains(f.Basis, "7 rows name an input_column") {
			t.Errorf("basis does not report how many rows can say which column failed: %s", f.Basis)
		}
	})

	t.Run("per_record_failures_unreportable", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-unrep", "completed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-unrep", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
		storeConfig(t, pool, k, "s-unrep", configNoErrorChannel)
		r := classify(t, pool, "s-unrep", 0)
		f := wantVerdict(t, r, LocusPerRecordFailuresUnreportable, Present)
		if !strings.Contains(f.Basis, "1 map_record") {
			t.Errorf("basis does not count the instances: %s", f.Basis)
		}
		if !strings.Contains(f.Basis, "at most one step's config") {
			t.Errorf("basis does not carry the configuration's own limit: %s", f.Basis)
		}
		// The sort operator is not one of the five and must not be counted as
		// unreportable: it cannot report a per-record error in any configuration.
		if strings.Contains(f.Basis, "sort") {
			t.Errorf("basis counts an operator that has no error channel to declare: %s", f.Basis)
		}
	})

	t.Run("per_record_failures_unreportable is not evaluable with no stored config", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-noconf", "failed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-noconf", "reducing01", 0, "failed", 10, 0, "boom", now.Add(-time.Hour))
		r := classify(t, pool, "s-noconf", 0)
		f := wantVerdict(t, r, LocusPerRecordFailuresUnreportable, NotEvaluable)
		if !strings.Contains(f.Basis, "sharding validation") {
			t.Errorf("basis does not name what an absent row means: %s", f.Basis)
		}
	})

	t.Run("sink_failed_under_completed_worker", func(t *testing.T) {
		k := header(t, pool, "cgt", "loader", "claims", "s-sink", "completed", now.Add(-time.Hour))
		w := workerRow(t, pool, k, "cgt", "loader", "s-sink", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))
		// Two sinks on one edge, one of which failed: AggregateChannelResults
		// folds them and blanks output_entity, which is the thing the locus
		// cannot see and which this writes through the production code.
		edges(t, pool, w, "s-sink", []compute_pipes.ComputePipesResult{
			{Type: "db_table", InputChannel: "mapped", OutputChannel: "out", OutputChannelSpec: "claims_spec",
				EntityName: "jetsapi.claims_a", OutputLocation: "sql://jetsapi.claims_a", CopyRowCount: 600},
			{Type: "db_table", InputChannel: "mapped", OutputChannel: "out", OutputChannelSpec: "claims_spec",
				EntityName: "jetsapi.claims_b", OutputLocation: "sql://jetsapi.claims_b", CopyRowCount: 0,
				Err: fmt.Errorf("deadlock detected")},
		})
		r := classify(t, pool, "s-sink", 0)
		f := wantVerdict(t, r, LocusSinkFailedUnderCompletedWorker, Present)
		if !strings.Contains(f.Basis, "1 of the 1 name no sink at all") {
			t.Errorf("basis does not report the fold: %s", f.Basis)
		}
		// The parent completed, so nothing else fires on this run.
		wantVerdict(t, r, LocusWorkerFailed, Absent)
		// And written_not_arrived is not evaluable even with the table there.
		g := wantVerdict(t, r, LocusWrittenNotArrived, NotEvaluable)
		if !strings.Contains(g.Basis, "check against S3") {
			t.Errorf("basis does not say why it cannot be answered: %s", g.Basis)
		}
	})

	t.Run("step_never_started", func(t *testing.T) {
		// Four prior runs with two steps each, then a run with only one.
		for i := 4; i >= 1; i-- {
			s := fmt.Sprintf("s-hist-%d", i)
			k := header(t, pool, "usi", "qc", "members", s, "completed", now.Add(-time.Duration(i*24)*time.Hour))
			workerRow(t, pool, k, "usi", "qc", s, "reducing00", 0, "completed", 100, 100, "", now.Add(-time.Duration(i*24)*time.Hour))
			workerRow(t, pool, k, "usi", "qc", s, "reducing01", 0, "completed", 100, 100, "", now.Add(-time.Duration(i*24)*time.Hour))
		}
		k := header(t, pool, "usi", "qc", "members", "s-missing", "completed", now)
		workerRow(t, pool, k, "usi", "qc", "s-missing", "reducing00", 0, "completed", 100, 100, "", now)
		r := classify(t, pool, "s-missing", 30*24*time.Hour)
		f := wantVerdict(t, r, LocusStepNeverStarted, Present)
		if f.StepRef != "reducing01" {
			t.Errorf("step_ref is %q, want reducing01", f.StepRef)
		}
		if !strings.Contains(f.Basis, "workspace version") {
			t.Errorf("basis does not carry F196: %s", f.Basis)
		}
		// The invariant: a finding naming a step carries step_label_ambiguous.
		// assertAllValid has already refused it otherwise; this says why.
		if err := f.Validate(); err != nil {
			t.Errorf("a finding naming a step did not validate: %v", err)
		}
	})

	t.Run("step_never_started is absent when every step ran", func(t *testing.T) {
		k := header(t, pool, "usi", "qc", "members", "s-complete", "completed", now.Add(time.Hour))
		workerRow(t, pool, k, "usi", "qc", "s-complete", "reducing00", 0, "completed", 100, 100, "", now.Add(time.Hour))
		workerRow(t, pool, k, "usi", "qc", "s-complete", "reducing01", 0, "completed", 100, 100, "", now.Add(time.Hour))
		r := classify(t, pool, "s-complete", 30*24*time.Hour)
		wantVerdict(t, r, LocusStepNeverStarted, Absent)
	})

	t.Run("step_never_started is not evaluable with no baseline", func(t *testing.T) {
		r := classify(t, pool, "s-missing", 0)
		f := wantVerdict(t, r, LocusStepNeverStarted, NotEvaluable)
		if !strings.Contains(f.Basis, "no step history") {
			t.Errorf("basis does not say why: %s", f.Basis)
		}
	})

	t.Run("a headerless run leaves four loci unanswerable", func(t *testing.T) {
		// Worker rows whose header has been purged. The header is deleted at a
		// hard-coded six months while the worker record follows RETENTION_DAYS,
		// and in three of four measured environments the worker rows outlive
		// their headers.
		k := header(t, pool, "cgt", "loader", "claims", "s-orphan", "completed", now.Add(-time.Hour))
		workerRow(t, pool, k, "cgt", "loader", "s-orphan", "reducing01", 0, "", 0, 0, "", now.Add(-time.Hour))
		if _, err := pool.Exec(context.Background(),
			`DELETE FROM jetsapi.pipeline_execution_status WHERE session_id = 's-orphan'`); err != nil {
			t.Fatalf("purging the header: %v", err)
		}
		r := classify(t, pool, "s-orphan", 30*24*time.Hour)
		for _, l := range []string{LocusRunNotStarted, LocusStepNeverStarted, LocusWorkerNotTerminated} {
			f := wantVerdict(t, r, l, NotEvaluable)
			if f.Basis == "" {
				t.Errorf("locus %s is not evaluable and says nothing about why", l)
			}
		}
		// But the loci that need no header still answer, which is the point of
		// the worker row being self-keying (F98).
		wantVerdict(t, r, LocusWorkerFailed, Absent)
	})
}

// --- the same nine, on a deployment that has not been migrated ---------------

// This is the test the three-valued verdict exists for. All four production
// environments measured on 2026-08-25 lacked pipeline_execution_channel_details
// (I-132), and a boolean classifier would report "no sink failed" and "nothing
// was written and lost" on every one of them.
func TestUnmigratedDeploymentReportsNotEvaluableRatherThanAbsent(t *testing.T) {
	pool := freshDB(t, "triage_unmigrated", unmigratedTables)

	k := header(t, pool, "cgt", "loader", "claims", "s-1", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-1", "reducing01", 0, "completed", 1000, 1000, "", now.Add(-time.Hour))

	ev, err := Gather(context.Background(), pool, "s-1", 0)
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if ev.Extent.ChannelDetails {
		t.Fatal("the fixture installed the channel-detail table; the test is meaningless")
	}
	r := Default().Classify(ev)
	assertAllValid(t, r)

	for _, l := range []string{
		LocusSinkFailedUnderCompletedWorker,
		LocusWrittenNotArrived,
		LocusPerRecordFailuresReported,
		LocusPerRecordFailuresUnreportable,
	} {
		f := wantVerdict(t, r, l, NotEvaluable)
		if !strings.Contains(f.Basis, "not deployed") {
			t.Errorf("locus %s does not name the missing table: %s", l, f.Basis)
		}
	}
	// Five of nine unanswerable, and the denominator says so. Four for want of
	// a table and one — step_never_started — for want of a baseline, which is a
	// caller's choice rather than a deployment's and is why the two reasons are
	// in the basis rather than in the count.
	if r.Evaluable != 4 {
		t.Errorf("Evaluable is %d, want 4", r.Evaluable)
	}
	if f := verdictOf(t, r, LocusStepNeverStarted); f.Verdict != NotEvaluable ||
		!strings.Contains(f.Basis, "no step history") {
		t.Errorf("step_never_started should be unanswerable for want of a baseline: %q / %s",
			f.Verdict, f.Basis)
	}
	// The five that do not need those tables still answer.
	for _, l := range []string{LocusRunNotStarted, LocusWorkerNotTerminated, LocusWorkerFailed,
		LocusRowsLostSilently} {
		wantVerdict(t, r, l, Absent)
	}
}

// --- the census, which is what criterion 44 asks for -------------------------

// A per-locus census over every session the migrated fixture built, with the
// denominators separated. It is a report rather than an assertion about the
// world: what it asserts is that the classifier produces one and that no locus
// is missing from it, which is the property a per-class metric with no aggregate
// needs and which a classifier returning only its hits cannot supply.
func TestCensusHasADenominatorForEveryLocus(t *testing.T) {
	pool := freshDB(t, "triage_census", migratedTables)

	// One run per shape, deliberately overlapping: s-multi exhibits three loci
	// at once, which is the case Q-30 asks about and which a single-valued
	// incident_locus cannot represent in one row.
	k1 := header(t, pool, "cgt", "loader", "claims", "s-multi", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k1, "cgt", "loader", "s-multi", "reducing01", 0, "failed", 5000, 0, "boom,bang", now.Add(-time.Hour))
	workerRow(t, pool, k1, "cgt", "loader", "s-multi", "reducing01", 1, "", 0, 0, "", now.Add(-time.Hour))
	workerRow(t, pool, k1, "cgt", "loader", "s-multi", "reducing01", 2, "completed", 8000, 40, "", now.Add(-time.Hour))
	processErrors(t, pool, k1, "s-multi", 20, 0)
	storeConfig(t, pool, k1, "s-multi", configNoErrorChannel)

	k2 := header(t, pool, "cgt", "loader", "claims", "s-ok", "completed", now.Add(-time.Hour))
	workerRow(t, pool, k2, "cgt", "loader", "s-ok", "reducing01", 0, "completed", 5000, 4900, "", now.Add(-time.Hour))
	storeConfig(t, pool, k2, "s-ok", configWithErrorChannel)

	sessions := []string{"s-multi", "s-ok"}
	census := map[string]map[Verdict]int{}
	for _, l := range Loci {
		census[l] = map[Verdict]int{}
	}
	multi := 0
	for _, s := range sessions {
		r := classify(t, pool, s, 0)
		for i := range r.Findings {
			census[r.Findings[i].Locus][r.Findings[i].Verdict]++
		}
		if n := len(r.Fired()); n > 1 {
			multi++
			t.Logf("session %s fires %d loci: %v", s, n, r.Loci())
		}
	}

	t.Logf("%-38s %8s %8s %14s", "locus", "present", "absent", "not_evaluable")
	for _, l := range Loci {
		c := census[l]
		t.Logf("%-38s %8d %8d %14d", l, c[Present], c[Absent], c[NotEvaluable])
		if c[Present]+c[Absent]+c[NotEvaluable] != len(sessions) {
			t.Errorf("locus %s has %d verdicts over %d sessions: a per-class denominator needs one each",
				l, c[Present]+c[Absent]+c[NotEvaluable], len(sessions))
		}
	}
	if multi == 0 {
		t.Error("no session fired more than one locus; the fixture was meant to build one that does")
	}
	// s-multi is the concrete case for Q-30: a failed worker, a stalled worker
	// and a volume collapse in one session, which incident_locus cannot hold in
	// one row.
	for _, l := range []string{LocusWorkerFailed, LocusWorkerNotTerminated, LocusRowsLostSilently,
		LocusPerRecordFailuresReported, LocusPerRecordFailuresUnreportable} {
		if census[l][Present] == 0 {
			t.Errorf("locus %s never fired in the census; the fixture no longer covers it", l)
		}
	}
}

// The classifier must not depend on which order Gather happened to read rows
// in, and the cheapest way that breaks is a map iteration reaching a basis
// string. Two classifications of one run must be identical.
func TestClassificationIsStable(t *testing.T) {
	pool := freshDB(t, "triage_stable", migratedTables)
	k := header(t, pool, "cgt", "loader", "claims", "s-1", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-1", "reducing01", 0, "failed", 100, 0, "a,b", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-1", "reducing02", 1, "completed", 9000, 3, "", now.Add(-time.Hour))
	storeConfig(t, pool, k, "s-1", configNoErrorChannel)

	first := classify(t, pool, "s-1", 0)
	for i := 0; i < 5; i++ {
		next := classify(t, pool, "s-1", 0)
		for j := range first.Findings {
			a, b := first.Findings[j], next.Findings[j]
			if a.Locus != b.Locus || a.Verdict != b.Verdict || a.Basis != b.Basis ||
				strings.Join(a.Confounders, ",") != strings.Join(b.Confounders, ",") {
				t.Fatalf("run %d disagrees with run 0 on locus %s:\n  %q %v\n  %q %v",
					i+1, a.Locus, a.Verdict, a.Confounders, b.Verdict, b.Confounders)
			}
		}
	}
}

// The confounders a finding carries must be writable into jetsapi.incident.
// This is the assertion that the Go vocabulary and the database's agree in the
// direction that matters — an insert rather than a string comparison — and it
// is the F68 lesson applied to a column rather than to a table.
func TestFindingConfoundersAreAcceptedByTheIncidentTable(t *testing.T) {
	pool := freshDB(t, "triage_confounders", migratedTables)
	ctx := context.Background()

	k := header(t, pool, "cgt", "loader", "claims", "s-1", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-1", "reducing01", 0, "", 0, 0, "", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-1", "", 1, "failed", 10, 0, "x", now.Add(-time.Hour))
	r := classify(t, pool, "s-1", 0)

	written := 0
	for i := range r.Findings {
		f := &r.Findings[i]
		conf := f.Confounders
		if conf == nil {
			conf = []string{}
		}
		// The classifier does not write incidents — that is AC.3 — so this
		// insert exists only to prove the vocabularies agree. It uses the
		// finding's own locus and confounders and nothing else of its own.
		_, err := pool.Exec(ctx, `INSERT INTO jetsapi.incident
			(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
			 incident_step_ref, incident_confounders, incident_model_version)
			VALUES ($1, $2, now(), $3, 'low', 'triaged', $4, $5, 'test')`,
			fmt.Sprintf("i-%d", i), f.SessionId, f.Locus, nullable(f.StepRef), conf)
		if err != nil {
			t.Errorf("locus %s: a finding's own locus and confounders were refused by jetsapi.incident: %v\n  confounders: %v",
				f.Locus, err, conf)
			continue
		}
		written++
	}
	if written != len(Loci) {
		t.Errorf("%d of %d findings were writable", written, len(Loci))
	}
	// And the invariant the CHECK enforces really is enforced, so the Go check
	// above is a better message rather than the only guard.
	_, err := pool.Exec(ctx, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_step_ref, incident_confounders, incident_model_version)
		VALUES ('i-bad', 's-1', now(), 'worker_failed', 'low', 'triaged', 'reducing01', '{}', 'test')`)
	if err == nil {
		t.Error("jetsapi.incident accepted a step_ref with no step_label_ambiguous confounder")
	}
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// observe.IsConfounder is the only thing this package adds to that one, and it
// must agree with the vocabulary it reads from.
func TestConfoundersWrittenAreAllInTheVocabulary(t *testing.T) {
	for _, c := range observe.RecordConfounders {
		if !observe.IsConfounder(c) {
			t.Errorf("%q is in RecordConfounders and IsConfounder does not know it", c)
		}
	}
	if observe.IsConfounder("probably_fine") {
		t.Error("IsConfounder accepted a member that is not in the vocabulary")
	}
}
