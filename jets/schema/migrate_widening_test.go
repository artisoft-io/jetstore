// The migration the record widening owes, run against a real Postgres.
//
// Adding a column to jets_schema.json is the cheap half; the expensive half is the
// ALTER on a table that already holds rows, and that is what this exercises. It
// creates process_errors and pipeline_execution_status **as they were before the
// widening**, puts a row in each, then applies the current definition through the
// same UpdateTableSchema call that update_db -migrateDb makes, and asserts that the
// new columns and index arrive, that the existing row survives, and that the new
// columns are null on it rather than defaulted to something a reader would mistake
// for a measurement.
//
// Needs JETS_TEST_DSN (any throwaway database; the test creates and drops its own
// tables); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/schema/
package schema

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func migrationTestPool(t *testing.T) *pgxpool.Pool {
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
	return pool
}

// loadTableDefinition reads one table out of the shipped jets_schema.json, so the
// test migrates to the definition that actually ships rather than to a copy of it.
func loadTableDefinition(t *testing.T, tableName string) TableDefinition {
	t.Helper()
	b, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading jets_schema.json: %v", err)
	}
	var defs []TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	for i := range defs {
		if defs[i].TableName == tableName {
			return defs[i]
		}
	}
	t.Fatalf("table %s not found in jets_schema.json", tableName)
	return TableDefinition{}
}

// withoutColumns returns the definition with the named columns removed, which is
// the shape the table had before the widening.
func withoutColumns(def TableDefinition, drop ...string) TableDefinition {
	dropped := make(map[string]bool, len(drop))
	for _, c := range drop {
		dropped[c] = true
	}
	before := def
	before.Columns = nil
	for _, c := range def.Columns {
		if !dropped[c.ColumnName] {
			before.Columns = append(before.Columns, c)
		}
	}
	// The index over a column that does not exist yet cannot be created either.
	before.Indexes = nil
	for _, idx := range def.Indexes {
		keep := true
		for c := range dropped {
			if strings.Contains(idx.IndexDef, c) {
				keep = false
			}
		}
		if keep {
			before.Indexes = append(before.Indexes, idx)
		}
	}
	return before
}

func columnNames(t *testing.T, pool *pgxpool.Pool, schema, table string) map[string]bool {
	t.Helper()
	got, err := GetTableSchema(pool, schema, table)
	if err != nil {
		t.Fatalf("reading back %s.%s: %v", schema, table, err)
	}
	names := make(map[string]bool, len(got.Columns))
	for _, c := range got.Columns {
		names[c.ColumnName] = true
	}
	return names
}

// migrateAndCheck is the body both cases share: create the pre-widening table,
// seed it, migrate, and assert.
func migrateAndCheck(t *testing.T, tableName string, added []string, seed string, survives string) {
	t.Helper()
	pool := migrationTestPool(t)
	ctx := context.Background()

	def := loadTableDefinition(t, tableName)
	before := withoutColumns(def, added...)

	// Start from the pre-widening table. dropExisting takes the CreateTable arm,
	// which drops first, so a leftover from an earlier run does not decide the result.
	if err := before.UpdateTableSchema(pool, true); err != nil {
		t.Fatalf("creating the pre-widening %s: %v", tableName, err)
	}
	t.Cleanup(func() { _ = def.DropTable(pool) })

	names := columnNames(t, pool, def.SchemaName, tableName)
	for _, c := range added {
		if names[c] {
			t.Fatalf("%s should not exist before the migration", c)
		}
	}
	if _, err := pool.Exec(ctx, seed); err != nil {
		t.Fatalf("seeding %s: %v", tableName, err)
	}

	// The migration, through the call update_db -migrateDb makes.
	if err := def.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating %s: %v", tableName, err)
	}

	names = columnNames(t, pool, def.SchemaName, tableName)
	for _, c := range added {
		if !names[c] {
			t.Errorf("%s is missing after the migration", c)
		}
	}

	// The seeded row survives, and the new columns are null on it.
	var nulls int
	if err := pool.QueryRow(ctx, survives).Scan(&nulls); err != nil {
		t.Fatalf("reading back the seeded row: %v", err)
	}
	if nulls != 1 {
		t.Errorf("expected exactly one pre-migration row with null discriminators, got %d", nulls)
	}

	// Idempotent: update_db is run on every deployment, not once.
	if err := def.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("re-running the migration: %v", err)
	}
}

func TestMigrateProcessErrors(t *testing.T) {
	migrateAndCheck(t, "process_errors",
		[]string{"cpipes_step_id", "error_channel", "operator_type"},
		`INSERT INTO jetsapi.process_errors (pipeline_execution_status_key, session_id, error_message, shard_id)
		 VALUES (1, 'pre_migration_session', 'an error from before the widening', 0)`,
		`SELECT count(*) FROM jetsapi.process_errors
		 WHERE session_id = 'pre_migration_session'
		   AND cpipes_step_id IS NULL AND error_channel IS NULL AND operator_type IS NULL`)
}

func TestMigratePipelineExecutionStatus(t *testing.T) {
	migrateAndCheck(t, "pipeline_execution_status",
		[]string{"failure_class", "failure_source", "workspace_name", "workspace_version"},
		`INSERT INTO jetsapi.pipeline_execution_status
		   (pipeline_config_key, client, process_name, main_object_type, session_id,
		    source_period_key, status, failure_details, user_email)
		 VALUES (1, 'c', 'p', 'ot', 'pre_migration_session', 1, 'failed', 'some prose', 'system')`,
		`SELECT count(*) FROM jetsapi.pipeline_execution_status
		 WHERE session_id = 'pre_migration_session'
		   AND failure_details = 'some prose'
		   AND failure_class IS NULL AND failure_source IS NULL
		   AND workspace_name IS NULL AND workspace_version IS NULL`)
}

// The index the join needs is created by the migration, not only by CreateTable.
// Worth its own assertion: UpdateTable and CreateTable emit their indexes by
// different routes, and a migrated deployment is the one that would silently not
// have it.
func TestMigrationCreatesTheProcessErrorsIndex(t *testing.T) {
	pool := migrationTestPool(t)
	ctx := context.Background()

	def := loadTableDefinition(t, "process_errors")
	before := withoutColumns(def, "cpipes_step_id", "error_channel", "operator_type")
	if err := before.UpdateTableSchema(pool, true); err != nil {
		t.Fatalf("creating the pre-widening process_errors: %v", err)
	}
	t.Cleanup(func() { _ = def.DropTable(pool) })
	if err := def.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating process_errors: %v", err)
	}

	var count int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FROM pg_indexes
		 WHERE schemaname = 'jetsapi' AND tablename = 'process_errors'
		   AND indexname = 'process_errors_session_step_idx'`).Scan(&count)
	if err != nil {
		t.Fatalf("querying pg_indexes: %v", err)
	}
	if count != 1 {
		t.Errorf("process_errors_session_step_idx: got %d, want 1", count)
	}
}
