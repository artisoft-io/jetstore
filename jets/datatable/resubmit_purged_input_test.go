// A run whose input was purged. The purge deletes input_registry rows after
// RETENTION_DAYS and keeps pipeline_execution_status rows for six months, so
// the execution screen still lists a run whose input is gone. Resubmitting it
// started a run that failed at its first step, and the status update, joining
// the same missing row, could not record the failure: the run stayed
// "submitted".
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5466:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5466/postgres go test ./jets/datatable/ -run Purged
package datatable

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// purgeInput deletes the input_registry row of the run peKey, as the purge does.
func purgeInput(t *testing.T, pool *pgxpool.Pool, peKey int) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM jetsapi.input_registry ir
		USING jetsapi.pipeline_execution_status pe
		WHERE pe.key = $1 AND ir.key = pe.main_input_registry_key`, peKey); err != nil {
		t.Fatalf("purging the input: %v", err)
	}
}

func TestResubmitRefusesARunWhoseInputWasPurged(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()

	seedRun(t, pool, "sess-kept", []string{"completed"})
	if err := CheckResubmittable(ctx, pool, "sess-kept"); err != nil {
		t.Fatalf("a run whose input is registered: %v", err)
	}

	peKey := seedRun(t, pool, "sess-purged", []string{"completed"})
	purgeInput(t, pool, peKey)
	err := CheckResubmittable(ctx, pool, "sess-purged")
	if err == nil || !strings.Contains(err.Error(), "no longer registered") {
		t.Fatalf("a run whose input was purged: got %v, want the purged-input refusal", err)
	}

	err = CheckResubmittable(ctx, pool, "sess-none")
	if err == nil || !strings.Contains(err.Error(), "no pipeline execution") {
		t.Fatalf("an unknown session: got %v, want the no-execution refusal", err)
	}
}

func TestAFailedRunWhoseInputWasPurgedIsRecordedFailed(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()

	peKey := seedRun(t, pool, "sess-purged-failed", []string{"failed"})
	purgeInput(t, pool, peKey)

	schemaProviderJson, sessionId, err := GetSchemaProviderJsonFromPipelineKey(pool, peKey)
	if err != nil || schemaProviderJson != "" || sessionId != "sess-purged-failed" {
		t.Fatalf("GetSchemaProviderJsonFromPipelineKey = (%q, %q, %v), want (\"\", \"sess-purged-failed\", nil)",
			schemaProviderJson, sessionId, err)
	}

	ca := &StatusUpdate{
		CpipesMode: true,
		CpipesEnv:  map[string]any{},
		Dbpool:     pool,
		PeKey:      peKey,
		Status:     "failed",
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
	if recorded != "failed" {
		t.Fatalf("the run was recorded %q, want \"failed\"", recorded)
	}
}

// The two reviewer cases of jetstore#2366: a registered input still reads its
// schema provider through both functions, and the LEFT JOIN leaves no rows
// only for an unknown execution, which is an error rather than an empty answer.
func TestSchemaProviderStillReadWhenRegistered(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()
	peKey := seedRun(t, pool, "sess-kept-sp", []string{"completed"})
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.input_registry ir SET schema_provider_json = '{"request_id":"r1"}'
		FROM jetsapi.pipeline_execution_status pe WHERE pe.key = $1 AND ir.key = pe.main_input_registry_key`,
		peKey); err != nil {
		t.Fatal(err)
	}
	sp, sid, err := GetSchemaProviderJsonFromPipelineKey(pool, peKey)
	if err != nil || sp != `{"request_id":"r1"}` || sid != "sess-kept-sp" {
		t.Fatalf("by key: got (%q, %q, %v)", sp, sid, err)
	}
	if sp, err = GetSchemaProviderJsonFromPipelineSession(pool, "sess-kept-sp"); err != nil || sp != `{"request_id":"r1"}` {
		t.Fatalf("by session: got (%q, %v)", sp, err)
	}
}

func TestUnknownPipelineExecutionKeyIsAnError(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	if sp, sid, err := GetSchemaProviderJsonFromPipelineKey(pool, 987654); err == nil {
		t.Fatalf("unknown key: got (%q, %q, nil), want an error", sp, sid)
	}
}

func TestResubmitRefusesARunWithNoInputKey(t *testing.T) {
	pool := widenedTestPool(t, manifestRunTables...)
	ctx := context.Background()
	peKey := seedRun(t, pool, "sess-no-key", []string{"completed"})
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_status
		SET main_input_registry_key = NULL WHERE key = $1`, peKey); err != nil {
		t.Fatal(err)
	}
	err := CheckResubmittable(ctx, pool, "sess-no-key")
	if err == nil || !strings.Contains(err.Error(), "has no main input") {
		t.Fatalf("a run with no input key: got %v, want the no-input refusal", err)
	}
}
