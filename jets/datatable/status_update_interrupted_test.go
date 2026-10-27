// What an interrupted run reports and registers (I-51).
//
// CoordinateWork's "interrupted" arm used to record "interrupted" in
// pipeline_execution_status and leave ca.Status holding what the state machine
// passed in, which on the success path is "completed". Every reader after the
// switch read ca.Status, so input_loader_status recorded an interrupted load as
// completed, the loader registered its file, and a pipeline registered its
// partial outputs as inputs for the next process.
//
// These tests drive a run through each status CoordinateWork computes and read
// back what each reader wrote. Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5477:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5477/postgres go test ./jets/datatable/ -run Interrupted
package datatable

import (
	"context"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
)

// statusCases are the five statuses CoordinateWork computes, each as the pair
// (what the state machine passed in, what the workers recorded).
var statusCases = []struct {
	name           string
	inStatus       string
	workerStatuses []string
}{
	{"completed", "completed", []string{"completed", "completed"}},
	{"recovered", "completed", []string{"completed", "failed"}},
	{"errors", "completed", []string{"completed", "errors"}},
	{"interrupted", "completed", []string{"completed", "interrupted"}},
	{"failed", "failed", []string{"failed"}},
}

// withNoManifestUpload stubs the run manifest's s3 upload, which a completed
// run makes and which is not what these tests are about.
func withNoManifestUpload(t *testing.T) {
	t.Helper()
	restore := compute_pipes.RunManifestUploader
	compute_pipes.RunManifestUploader = func(bucket, objKey string, buf []byte) error { return nil }
	t.Cleanup(func() { compute_pipes.RunManifestUploader = restore })
}

// TestAnInterruptedLoadIsRecordedInterruptedAndRegistersNothing is acceptance
// criterion 30: the loader path, which records input_loader_status and
// registers the loaded file in input_registry.
func TestAnInterruptedLoadIsRecordedInterruptedAndRegistersNothing(t *testing.T) {
	pool := widenedTestPool(t, append(manifestRunTables, "input_loader_status")...)
	ctx := context.Background()
	withNoManifestUpload(t)

	// A failed load has always registered its file, and still does.
	wantRegistered := map[string]bool{
		"completed": true, "recovered": true, "errors": true, "interrupted": false, "failed": true,
	}
	for _, c := range statusCases {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "load-" + c.name
			originSessionId := "origin-" + c.name
			peKey := seedRun(t, pool, sessionId, c.workerStatuses)
			var ilKey int
			if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_loader_status
				(object_type, table_name, client, file_key, source_period_key, session_id, status, user_email)
				VALUES ('claim', 'acme_claims', 'acme', 'in/claims.csv', 1, $1, 'submitted', 'x@y.z')
				RETURNING key`, sessionId).Scan(&ilKey); err != nil {
				t.Fatalf("input_loader_status: %v", err)
			}
			ca := &StatusUpdate{
				CpipesMode: true,
				CpipesEnv: map[string]any{
					"$INPUT_LOADER_STATUS_KEY":  ilKey,
					"$CLIENT":                   "acme",
					"$ORG":                      "",
					"$OBJECT_TYPE":              "claim",
					"$ORIGIN_SESSIONID":         originSessionId,
					"$ORIGIN_SOURCE_PERIOD_KEY": 1,
					"${STAGING_TABLE_NAME}":     "acme_claims",
					"$SESSIONID":                sessionId,
				},
				Dbpool:  pool,
				PeKey:   peKey,
				Status:  c.inStatus,
				FileKey: "in/claims.csv",
			}
			if err := ca.CoordinateWork(); err != nil {
				t.Fatalf("CoordinateWork: %v", err)
			}
			var loaderStatus string
			if err := pool.QueryRow(ctx,
				`SELECT status FROM jetsapi.input_loader_status WHERE key = $1`, ilKey).
				Scan(&loaderStatus); err != nil {
				t.Fatalf("reading input_loader_status: %v", err)
			}
			if loaderStatus != c.name {
				t.Errorf("input_loader_status recorded %q, want %q", loaderStatus, c.name)
			}
			var registered int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.input_registry
				WHERE session_id = $1 AND source_type = 'file'`, originSessionId).
				Scan(&registered); err != nil {
				t.Fatalf("reading input_registry: %v", err)
			}
			if (registered > 0) != wantRegistered[c.name] {
				t.Errorf("the %s load registered %d files, want registered = %v",
					c.name, registered, wantRegistered[c.name])
			}
		})
	}
}

// TestAnInterruptedRunRegistersNoDbTable is the pipeline path's half: the
// ${REGISTER_DB_TABLE} registration, which shares its guard with the output
// tables' (outputsAreUsable).
func TestAnInterruptedRunRegistersNoDbTable(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()
	withNoManifestUpload(t)

	wantRegistered := map[string]bool{
		"completed": true, "recovered": true, "errors": true, "interrupted": false, "failed": false,
	}
	for _, c := range statusCases {
		t.Run(c.name, func(t *testing.T) {
			sessionId := "dbt-" + c.name
			peKey := seedRun(t, pool, sessionId, c.workerStatuses)
			ca := &StatusUpdate{
				CpipesMode: true,
				CpipesEnv: map[string]any{
					"${REGISTER_DB_TABLE}": 1,
					"$CLIENT":              "acme",
					"$ORG":                 "",
					"${DB_TABLE_OBJ_TYPE}": "claim",
					"$SESSIONID":           sessionId,
					"$YEAR":                2026,
					"$MONTH":               9,
					"$DAY":                 21,
					"${TABLE_NAME}":        "claims_report",
				},
				Dbpool:  pool,
				PeKey:   peKey,
				Status:  c.inStatus,
				FileKey: "in/claims.csv",
			}
			if err := ca.CoordinateWork(); err != nil {
				t.Fatalf("CoordinateWork: %v", err)
			}
			var registered int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.input_registry
				WHERE session_id = $1 AND source_type = 'db_table'`, sessionId).
				Scan(&registered); err != nil {
				t.Fatalf("reading input_registry: %v", err)
			}
			if (registered > 0) != wantRegistered[c.name] {
				t.Errorf("the %s run registered %d db tables, want registered = %v",
					c.name, registered, wantRegistered[c.name])
			}
		})
	}
}

// outputsAreUsable is total over the five statuses and refuses anything else,
// so a status added to CoordinateWork's switch registers nothing until it is
// named here.
func TestOutputsAreUsable(t *testing.T) {
	for status, want := range map[string]bool{
		"completed": true, "recovered": true, "errors": true,
		"interrupted": false, "failed": false, "": false, "submitted": false,
	} {
		if got := outputsAreUsable(status); got != want {
			t.Errorf("outputsAreUsable(%q) = %v, want %v", status, got, want)
		}
	}
}
