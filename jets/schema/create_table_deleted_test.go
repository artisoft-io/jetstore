// CreateTable honouring the deleted flag, run against a real Postgres.
//
// The defect this pins is a *fresh install* one, and that is why it cannot be
// tested any other way: on a database where a table has never existed, CreateTable
// ran and UpdateTable did not, and only CreateTable failed to test the flag. Every
// deployment that has been migrated twice is past it for ever, so a fixture that
// reuses a database cannot see it -- which is how it survived (I-305).
//
// Needs JETS_TEST_DSN (any throwaway database; the tests create and drop their own
// jetsapi schema); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/schema/
package schema

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaTestPool returns a pool on a database of this suite's own, created and
// dropped here.
//
// **These tests install the whole jetsapi schema and drop it first, so they cannot
// share a database with anything.** migrationTestPool hands out the DSN itself,
// which is right for the widening tests beside them -- those create and drop the
// two tables they name. A fresh install is not that: DROP SCHEMA jetsapi CASCADE
// on a shared database takes jets/agentic/observe's fixtures with it, and the
// symptom is that suite failing on a table it never installed. Observed while
// writing these, which is why the isolation is a database rather than a
// convention. Same shape as triage's freshDB
// (`freshDB`, `jets/agentic/triage/classify_test.go:94`).
func schemaTestPool(t *testing.T, dbName string) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("JETS_TEST_DSN")
	if base == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
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

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/" + dbName
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// loadAllTableDefinitions reads the shipped jets_schema.json whole, so the fresh
// install under test is the one that actually ships.
func loadAllTableDefinitions(t *testing.T) []TableDefinition {
	t.Helper()
	b, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading jets_schema.json: %v", err)
	}
	var defs []TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	return defs
}

// freshInstall drops the jetsapi schema and runs one migration pass over every
// table, which is what update_db -migrateDb does on a database that has never been
// migrated (`MigrateDb`, `jets/update_db/migrate_db.go:47`).
func freshInstall(t *testing.T, pool *pgxpool.Pool, defs []TableDefinition) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS jetsapi CASCADE"); err != nil {
		t.Fatalf("dropping jetsapi schema: %v", err)
	}
	for i := range defs {
		if defs[i].Deleted {
			continue
		}
		if err := defs[i].UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("first migration pass, table %s: %v", defs[i].TableName, err)
		}
	}
}

func columnExists(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_schema = 'jetsapi' AND table_name = $1 AND column_name = $2`,
		table, column).Scan(&n)
	if err != nil {
		t.Fatalf("checking column %s.%s: %v", table, column, err)
	}
	return n > 0
}

// TestFreshInstallOmitsDeletedColumns is the direct assertion: one migration pass
// on a virgin database produces the same columns the second pass used to produce.
func TestFreshInstallOmitsDeletedColumns(t *testing.T) {
	pool := schemaTestPool(t, "ah1_deleted_columns")
	defs := loadAllTableDefinitions(t)
	freshInstall(t, pool, defs)

	checked := 0
	for i := range defs {
		if defs[i].Deleted {
			continue
		}
		for _, col := range defs[i].Columns {
			if !col.Deleted {
				continue
			}
			checked++
			if columnExists(t, pool, defs[i].TableName, col.ColumnName) {
				t.Errorf("deleted column %s.%s exists after a fresh install",
					defs[i].TableName, col.ColumnName)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no column in jets_schema.json is marked deleted; this test asserts nothing")
	}
	t.Logf("checked %d deleted columns across the shipped schema", checked)
}

// TestFreshInstallOmitsDeletedIndexes is the other half, and it is not a
// restatement. Two deleted indexes are declared over deleted columns, so honouring
// the flag on columns alone would make the fresh install fail outright rather than
// leave it wrong -- which is why both are one change.
func TestFreshInstallOmitsDeletedIndexes(t *testing.T) {
	pool := schemaTestPool(t, "ah1_deleted_indexes")
	defs := loadAllTableDefinitions(t)
	freshInstall(t, pool, defs)

	checked := 0
	for i := range defs {
		if defs[i].Deleted {
			continue
		}
		for _, idx := range defs[i].Indexes {
			if !idx.Deleted {
				continue
			}
			checked++
			var n int
			err := pool.QueryRow(context.Background(),
				`SELECT count(*) FROM pg_indexes WHERE schemaname = 'jetsapi' AND indexname = $1`,
				idx.IndexName).Scan(&n)
			if err != nil {
				t.Fatalf("checking index %s: %v", idx.IndexName, err)
			}
			if n > 0 {
				t.Errorf("deleted index %s exists after a fresh install", idx.IndexName)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no index in jets_schema.json is marked deleted; this test asserts nothing")
	}
	t.Logf("checked %d deleted indexes across the shipped schema", checked)
}

// TestFreshInstallAcceptsTheRunConfigurationInsert is I-305 stated as the symptom
// rather than as the mechanism, and it is the reason the fix is worth making: the
// sharding start names six columns and neither of the two deleted ones, so a
// freshly created cpipes_execution_status used to reject its own writer's insert.
//
// The statement is copied column for column from StartShardingComputePipes
// (`jets/compute_pipes/actions_start_sharding_cp.go:385`).
func TestFreshInstallAcceptsTheRunConfigurationInsert(t *testing.T) {
	pool := schemaTestPool(t, "ah1_run_config_insert")
	freshInstall(t, pool, loadAllTableDefinitions(t))

	_, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.cpipes_execution_status
			(pipeline_execution_status_key, session_id, cpipes_config_json, input_parquet_schema_json,
			 cpipes_startup_json, input_row_columns_json)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		1, "AH1-session", "{}", "{}", "{}", "{}")
	if err != nil {
		t.Fatalf("the sharding start's own insert failed on a fresh install: %v", err)
	}
}

// TestFreshInstallIsIdempotentWithTheSecondPass pins what the fix must not change.
// Running the migration twice was the accidental workaround, so the columns after
// one pass must now equal the columns after two -- otherwise a fixed fresh install
// and an existing deployment would diverge, which is a worse defect than the one
// being fixed.
func TestFreshInstallIsIdempotentWithTheSecondPass(t *testing.T) {
	pool := schemaTestPool(t, "ah1_idempotent")
	defs := loadAllTableDefinitions(t)
	ctx := context.Background()

	freshInstall(t, pool, defs)
	afterOne := schemaFingerprint(t, pool)

	for i := range defs {
		if defs[i].Deleted {
			continue
		}
		if err := defs[i].UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("second migration pass, table %s: %v", defs[i].TableName, err)
		}
	}
	afterTwo := schemaFingerprint(t, pool)

	if afterOne != afterTwo {
		t.Errorf("one pass and two passes disagree about the schema\nafter one:\n%s\nafter two:\n%s",
			afterOne, afterTwo)
	}
	_ = ctx
}

// schemaFingerprint renders every column of every jetsapi table as one sorted
// string, which is coarse on purpose: the claim being tested is that the two
// passes agree, not what they agree on.
func schemaFingerprint(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT table_name || '.' || column_name || ':' || data_type || ':' || is_nullable
		 FROM information_schema.columns WHERE table_schema = 'jetsapi'
		 ORDER BY table_name, column_name`)
	if err != nil {
		t.Fatalf("fingerprinting schema: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scanning fingerprint: %v", err)
		}
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}

// TestCreateTableRefusesAnAllDeletedDefinition covers the one shape the fix
// introduces. A definition whose columns are all tombstones would emit
// "CREATE TABLE x()", and Postgres would name the parenthesis rather than the
// cause; the guard says what is actually wrong. No shipped table is in this state
// and the test constructs one rather than waiting for it.
func TestCreateTableRefusesAnAllDeletedDefinition(t *testing.T) {
	pool := schemaTestPool(t, "ah1_all_deleted")
	def := TableDefinition{
		SchemaName: "jetsapi",
		TableName:  "ah1_all_deleted_probe",
		Columns: []ColumnDefinition{
			{ColumnName: "gone", DataType: "text", Deleted: true},
		},
	}
	err := def.CreateTable(pool)
	if err == nil {
		t.Fatal("expected CreateTable to refuse a definition with no live column")
	}
	if !strings.Contains(err.Error(), "no column that is not marked deleted") {
		t.Errorf("error does not name the cause: %v", err)
	}
}
