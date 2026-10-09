// The run manifest's negative case, which is the whole design in one test.
//
// StatusUpdate is invoked on the error path as well as the success path -- the
// state machine's runErrorStatusLambdaTask and runSuccessStatusLambdaTask are
// the same Lambda object -- so the question "does a manifest exist?" is decided
// here and nowhere else. A producer that wrote before branching on the computed
// status would write a manifest for a run that failed, which is the one outcome
// a manifest exists to make impossible.
//
// So this drives CoordinateWork through all five statuses it can compute and
// asserts one manifest and four absences. It drives "errors" too, although
// nothing on the cpipes path produces that status today: the only caller of
// UpdatePipelineExecutionStatus writes completed, interrupted or failed
// (actions_process_file.go), so the arm is offered and unselected. A test of an
// unreachable branch costs one table row and is the only thing standing between
// "unreachable today" and "reachable and wrong tomorrow".
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5466:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5466/postgres go test ./jets/datatable/ -run Manifest
package datatable

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v5/pgxpool"
)

// manifestRunTables is every table CoordinateWork touches on the path this test
// drives. Naming them is cheaper than discovering them one failure at a time,
// and the list is the honest statement of how much of the function is exercised.
var manifestRunTables = []string{
	"pipeline_execution_status",
	"pipeline_execution_details",
	"pipeline_execution_channel_details",
	"cpipes_execution_status",
	"cpipes_execution_status_details",
	"input_registry",
	"process_config",
	"session_registry",
	"source_period",
}

// seedRun writes one cpipes run whose workers ended with the given statuses,
// and returns its pipeline execution key.
func seedRun(t *testing.T, pool *pgxpool.Pool, sessionId string, workerStatuses []string) int {
	t.Helper()
	ctx := context.Background()

	// One source period and one process config serve every case: both carry a
	// unique constraint, and the run is what varies here rather than the
	// reference data.
	var periodKey int
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.source_period
		(year, month, day, month_period, week_period, day_period)
		VALUES (2026, 9, 21, 1, 1, 1) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("source_period: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT key FROM jetsapi.source_period
		WHERE year = 2026 AND month = 9 AND day = 21`).Scan(&periodKey); err != nil {
		t.Fatalf("source_period: %v", err)
	}
	var registryKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
		(client, object_type, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('acme', 'claim', $1, 'claims', 'file', $2, 'x@y.z') RETURNING key`,
		periodKey, sessionId).Scan(&registryKey); err != nil {
		t.Fatalf("input_registry: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.process_config
		(process_name, main_rules, is_rule_set, output_tables, user_email)
		VALUES ('myprocess', 'main.jr', 0, '{}', 'x@y.z')
		ON CONFLICT DO NOTHING`); err != nil {
		t.Fatalf("process_config: %v", err)
	}
	var peKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, main_input_registry_key, merged_input_registry_keys, client, process_name,
		 main_object_type, session_id, source_period_key, status, user_email)
		VALUES (1, $1, '{}', 'acme', 'myprocess', 'claim', $2, $3, 'submitted', 'x@y.z')
		RETURNING key`, registryKey, sessionId, periodKey).Scan(&peKey); err != nil {
		t.Fatalf("pipeline_execution_status: %v", err)
	}
	for i, st := range workerStatuses {
		// cpipes_step_id is not null in cpipes_execution_status_details, which
		// CoordinateWork fills from these rows.
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_details
			(pipeline_config_key, pipeline_execution_status_key, client, process_name,
			 main_input_session_id, session_id, source_period_key, shard_id, status, user_email,
			 cpipes_step_id, input_records_count, output_records_count, input_files_size_mb,
			 input_bad_records_count)
			VALUES (1, $1, 'acme', 'myprocess', $2, $2, $3, $4, $5, 'x@y.z', 'step1', 10, 10, 1, 0)`,
			peKey, sessionId, periodKey, i, st); err != nil {
			t.Fatalf("pipeline_execution_details: %v", err)
		}
	}
	startup := compute_pipes.CpipesStartup{
		ProcessName: "myprocess",
		CpConfig: compute_pipes.ComputePipesConfig{
			OutputTables: []*compute_pipes.TableSpec{{Key: "claims_out", Name: "jetsapi.claims"}},
		},
	}
	startupJson, err := json.Marshal(startup)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_startup_json) VALUES ($1, $2, $3)`,
		peKey, sessionId, string(startupJson)); err != nil {
		t.Fatalf("cpipes_execution_status: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_channel_details
		(pipeline_execution_details_key, session_id, input_channel, output_channel,
		 output_channel_spec, output_type, output_entity, output_location,
		 output_sinks_count, output_records_count, parts_count, error_message)
		VALUES (1, $1, 'in', 'claims_out', 'claims_spec', 'db_table', 'jetsapi.claims',
		        'sql://jetsapi.claims', 1, 10, 0, '')`, sessionId); err != nil {
		t.Fatalf("pipeline_execution_channel_details: %v", err)
	}
	return peKey
}

// TestOnlyACompletedRunLeavesAManifest is acceptance criterion 11.
func TestOnlyACompletedRunLeavesAManifest(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()

	uploads := map[string][]byte{}
	restore := compute_pipes.RunManifestUploader
	compute_pipes.RunManifestUploader = func(bucket, objKey string, buf []byte) error {
		uploads[objKey] = buf
		return nil
	}
	t.Cleanup(func() { compute_pipes.RunManifestUploader = restore })

	// Each case is the pair (what the state machine passed in, what the workers
	// recorded), which is what the switch in CoordinateWork reads.
	cases := []struct {
		name           string
		inStatus       string
		workerStatuses []string
		wantStatus     string
		wantManifest   bool
	}{
		{"completed", "completed", []string{"completed", "completed"}, "completed", true},
		{"recovered", "completed", []string{"completed", "failed"}, "recovered", false},
		{"errors", "completed", []string{"completed", "errors"}, "errors", false},
		{"interrupted", "completed", []string{"completed", "interrupted"}, "interrupted", false},
		{"failed", "failed", []string{"failed"}, "failed", false},
	}
	manifests := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "sess-" + c.name
			peKey := seedRun(t, pool, sessionId, c.workerStatuses)
			ca := &StatusUpdate{
				CpipesMode: true,
				CpipesEnv:  map[string]any{},
				Dbpool:     pool,
				PeKey:      peKey,
				Status:     c.inStatus,
				FileKey:    "in/claims.csv",
			}
			if err := ca.CoordinateWork(); err != nil {
				t.Fatalf("CoordinateWork: %v", err)
			}
			var recorded string
			if err := pool.QueryRow(ctx,
				`SELECT status FROM jetsapi.pipeline_execution_status WHERE key = $1`, peKey).
				Scan(&recorded); err != nil {
				t.Fatalf("reading the recorded status: %v", err)
			}
			if recorded != c.wantStatus {
				t.Fatalf("the run was recorded %q, want %q -- the case does not drive the arm it names",
					recorded, c.wantStatus)
			}
			var stored string
			if err := pool.QueryRow(ctx,
				`SELECT run_manifest_json FROM jetsapi.cpipes_execution_status WHERE session_id = $1`,
				sessionId).Scan(&stored); err != nil {
				t.Fatalf("reading run_manifest_json: %v", err)
			}
			gotDb := stored != "" && stored != "{}"
			key := compute_pipes.RunManifestFileKey("", "myprocess", sessionId)
			_, gotS3 := uploads[key]
			if gotDb != c.wantManifest || gotS3 != c.wantManifest {
				t.Fatalf("status %q: manifest in database = %v, in s3 = %v, want %v for both",
					recorded, gotDb, gotS3, c.wantManifest)
			}
			if c.wantManifest {
				manifests++
				var m compute_pipes.CpipesRunManifest
				if err := json.Unmarshal([]byte(stored), &m); err != nil {
					t.Fatalf("the stored manifest does not parse: %v", err)
				}
				if m.Status != "completed" {
					t.Errorf("the manifest reports status %q", m.Status)
				}
				if len(m.Entries) != 1 || m.Entries[0].Channel != "claims_out" ||
					!m.Entries[0].Written {
					t.Errorf("the manifest does not describe the run's one declared table: %+v", m.Entries)
				}
			}
		})
	}
	if manifests != 1 {
		t.Fatalf("%d of the five statuses left a manifest, want exactly 1", manifests)
	}
}

// TestManifestFailureDoesNotFailTheRun is the additive posture, demonstrated
// rather than asserted: the manifest write is observability, and a pipeline
// that ran correctly must not be reported failed because an observability write
// did not land. The difference from InsertChannelExecutionDetails, which this
// copies, is that a missing detail row is detectable by arithmetic downstream
// and a missing manifest is indistinguishable from a run that never completed
// -- so the log line is the whole of the signal.
func TestManifestFailureDoesNotFailTheRun(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()

	restore := compute_pipes.RunManifestUploader
	compute_pipes.RunManifestUploader = func(bucket, objKey string, buf []byte) error {
		return errNoBucket
	}
	t.Cleanup(func() { compute_pipes.RunManifestUploader = restore })

	sessionId := "sess-additive"
	peKey := seedRun(t, pool, sessionId, []string{"completed"})
	ca := &StatusUpdate{
		CpipesMode: true, CpipesEnv: map[string]any{}, Dbpool: pool, PeKey: peKey,
		Status: "completed", FileKey: "in/claims.csv",
	}
	if err := ca.CoordinateWork(); err != nil {
		t.Fatalf("a failed manifest upload failed the run: %v", err)
	}
	var recorded string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM jetsapi.pipeline_execution_status WHERE key = $1`, peKey).
		Scan(&recorded); err != nil {
		t.Fatalf("%v", err)
	}
	if recorded != "completed" {
		t.Errorf("the run was recorded %q after a failed manifest upload", recorded)
	}
	// The database half still landed: one store failing must not take the other
	// with it.
	var stored string
	if err := pool.QueryRow(ctx,
		`SELECT run_manifest_json FROM jetsapi.cpipes_execution_status WHERE session_id = $1`,
		sessionId).Scan(&stored); err != nil {
		t.Fatalf("%v", err)
	}
	if !strings.Contains(stored, "claims_out") {
		t.Errorf("the database half did not land when the s3 half failed: %q", stored)
	}
}

type constErr string

func (e constErr) Error() string { return string(e) }

const errNoBucket = constErr("no bucket configured")
