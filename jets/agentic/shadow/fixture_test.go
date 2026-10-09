package shadow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The execution-record fixture, which is a second copy of the one in
// jets/agentic/triage's test file.
//
// **It is duplicated rather than shared, and that is recorded rather than
// defended.** Those helpers are in `package triage`, so nothing outside that
// package can call them; extracting them into a test-support package would edit
// another task's suite on this task's authority, which is the extraction gap 2b
// exists to prevent. The property that matters is preserved: **worker rows are
// written by the cpipes node's own two statements** rather than by SQL a test
// composed, which is F104's point and the reason a query written here does not
// simply agree with the hand that wrote the rows. Recorded as I-378.

const schemaPath = "../../jets_schema.json"

var migratedTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
	"pipeline_execution_channel_details", "process_errors", "cpipes_execution_status",
}

// unmigratedTables is the execution record and nothing else — the state of a
// deployment that has never run `update_db -migrateDb` since AB.1, which is
// every deployment measured on 2026-08-25 (I-132).
var unmigratedTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
}

var fixtureNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

func dsnFor(t *testing.T, dbName string) string {
	t.Helper()
	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/" + dbName
	return u.String()
}

// freshDB drops and recreates a database and installs the named execution tables.
// installAgentic decides whether audit.InstallSchema runs, which is what puts
// jetsapi.incident, hypothesis, incident_event and remediation there.
func freshDB(t *testing.T, dbName string, tables []string, installAgentic bool) *pgxpool.Pool {
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
	if installAgentic {
		if err := audit.InstallSchema(ctx, pool); err != nil {
			t.Fatalf("installing the agentic DDL: %v", err)
		}
	}
	return pool
}

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
// that never came back looks like.
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

func processErrors(t *testing.T, pool *pgxpool.Pool, execKey int, session string, n, withColumn int) {
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

func storeConfig(t *testing.T, pool *pgxpool.Pool, execKey int, session, config string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_config_json) VALUES ($1, $2, $3)`,
		execKey, session, config)
	if err != nil {
		t.Fatalf("inserting cpipes_execution_status row: %v", err)
	}
}

const configNoErrorChannel = `[{"type":"fan_out","input_channel":{"name":"input"},
  "apply":[{"type":"map_record","map_record_config":{},"output_channel":{"name":"mapped"}},
           {"type":"sort","sort_config":{},"output_channel":{"name":"sorted"}}]}]`

// multiLocusSession is triage's own census case: a failed worker, a stalled
// worker, a collapsed worker, per-record errors and an operator configured with
// no error channel, all in one run. Five loci fire (F280), which is what makes it
// the right subject for a writer that emits one incident per locus.
func multiLocusSession(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	k := header(t, pool, "cgt", "loader", "claims", session, "failed", fixtureNow.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", session, "reducing01", 0, "failed", 5000, 0, "boom,bang", fixtureNow.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", session, "reducing01", 1, "", 0, 0, "", fixtureNow.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", session, "reducing01", 2, "completed", 8000, 40, "", fixtureNow.Add(-time.Hour))
	processErrors(t, pool, k, session, 20, 0)
	storeConfig(t, pool, k, session, configNoErrorChannel)
}

// failedWorkerSession uses only the two tables of the execution record itself,
// so it can be written on a deployment that has never been migrated.
func failedWorkerSession(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	k := header(t, pool, "cgt", "loader", "claims", session, "failed", fixtureNow.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", session, "reducing01", 0, "failed", 5000, 0, "boom", fixtureNow.Add(-time.Hour))
}

func testWriter() Writer {
	w := DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return fixtureNow }
	return w
}
