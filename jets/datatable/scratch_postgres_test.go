package datatable

// The scratch PostgreSQL this package's database tests share.
//
// A test that needs a database is skipped unless JETS_TEST_DSN names a
// PostgreSQL it may write to. The tests create their tables in schema jetsapi
// from the shipped jets/jets_schema.json, never from hand-written DDL, so a
// test sees the columns and constraints a deployment migrates; and they drop
// and recreate what they use, so point JETS_TEST_DSN at a throwaway database
// and not at a jetsapi you care about. Locally:
//
//	docker run -d --rm --name jets-test-pg -e POSTGRES_PASSWORD=pw \
//	  -p 5471:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5471/postgres \
//	  go test -count=1 ./jets/datatable/
//
// Two shapes, and a suite picks the one it needs:
//
//   - createTablesFromSchema: the named tables, in the database JETS_TEST_DSN
//     names, dropped again when the test ends;
//   - scratchDatabase: a database of the suite's own beside it, for a suite
//     that sets up logins or roles, or more tables than it wants to share.
//
// Each suite wrote its own copy of this until 2026-11-14, eleven or so of
// them; the copies differed in what they skipped with and in whether they
// dropped what they made. A suite with a further need (one connection, a
// database recreated per test, the restricted logins) builds on these.

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// scratchDSN is JETS_TEST_DSN, or skips the test when it is not set.
func scratchDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of scratch_postgres_test.go")
	}
	return dsn
}

// jetsSchemaTables is the shipped jets_schema.json's table definitions, by
// table name, so the tables a test creates are the ones a deployment migrates.
func jetsSchemaTables(t *testing.T) map[string]*schema.TableDefinition {
	t.Helper()
	buf, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading ../jets_schema.json: %v", err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatalf("decoding ../jets_schema.json: %v", err)
	}
	out := make(map[string]*schema.TableDefinition, len(defs))
	for i := range defs {
		out[defs[i].TableName] = &defs[i]
	}
	return out
}

// createTablesFromSchema connects to JETS_TEST_DSN and creates the named jetsapi
// tables from jets_schema.json, dropping any left by an earlier run, and drops
// them again when the test ends.
func createTablesFromSchema(t *testing.T, names ...string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), scratchDSN(t))
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err = pool.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatalf("creating schema jetsapi: %v", err)
	}
	defs := jetsSchemaTables(t)
	for _, name := range names {
		def, ok := defs[name]
		if !ok {
			t.Fatalf("jets_schema.json carries no definition for jetsapi.%s", name)
		}
		// dropExisting, so the test owns the table whatever was there before
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			if _, err := pool.Exec(context.Background(), "DROP TABLE IF EXISTS jetsapi."+name+" CASCADE"); err != nil {
				t.Logf("warning, while dropping jetsapi.%s: %v", name, err)
			}
		}
	})
	return pool
}

// scratchDatabase returns a pool on the database dbName beside the one
// JETS_TEST_DSN names, creating it when it does not exist. It is kept between
// runs, so a suite recreates the tables it uses in it.
func scratchDatabase(t *testing.T, dbName string) *pgxpool.Pool {
	t.Helper()
	dsn := scratchDSN(t)
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	var n int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname = $1", dbName).Scan(&n); err != nil {
		admin.Close()
		t.Fatalf("looking for the test database %s: %v", dbName, err)
	}
	if n == 0 {
		if _, err = admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
			admin.Close()
			t.Fatalf("creating the test database %s: %v", dbName, err)
		}
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/" + dbName
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to the test database %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)
	return pool
}
