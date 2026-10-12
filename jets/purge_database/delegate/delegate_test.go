package delegate

// The rules unit test purge against a real PostgreSQL. Skipped unless
// JETS_TEST_DSN names one; it recreates the two tables in a database of its own,
// rut_ac_purge. Locally:
//
//	docker run -d --rm --name rut-ac-pg -e POSTGRES_PASSWORD=pw -p 5471:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5471/postgres \
//	  go test -count=1 ./jets/purge_database/delegate/

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func purgeTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	_ = admin.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname = 'rut_ac_purge'").Scan(&n)
	if n == 0 {
		if _, err = admin.Exec(ctx, "CREATE DATABASE rut_ac_purge"); err != nil {
			admin.Close()
			t.Fatal(err)
		}
	}
	admin.Close()
	u, _ := url.Parse(dsn)
	u.Path = "/rut_ac_purge"
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	buf, err := os.ReadFile("../../jets_schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	for i := range defs {
		switch defs[i].TableName {
		case "rules_unit_test_execution", "rules_unit_test_check":
			if err = defs[i].UpdateTableSchema(pool, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	return pool
}

// Finished runs older than the retention go with their checks; a run still
// queued or executing stays whatever its age, and so does a recent one.
func TestPurgeRulesUnitTestRuns(t *testing.T) {
	pool := purgeTestDB(t)
	ctx := context.Background()
	old := "now() - interval '40 days'"
	rows := []struct{ name, status, at string }{
		{"old pass", "pass", old},
		{"old error", "error", old},
		{"old executing", "executing", old},
		{"recent fail", "fail", "now() - interval '1 day'"},
	}
	keys := map[string]int{}
	for _, r := range rows {
		var k int
		err := pool.QueryRow(ctx, `INSERT INTO jetsapi.rules_unit_test_execution
			(workspace_name, test_path, test_name, status, user_email, last_update)
			VALUES ('w', 'p', $1, $2, 'u', `+r.at+`) RETURNING key`, r.name, r.status).Scan(&k)
		if err != nil {
			t.Fatal(err)
		}
		keys[r.name] = k
		if _, err = pool.Exec(ctx, `INSERT INTO jetsapi.rules_unit_test_check
			(execution_key, session_name, kind, "check", outcome) VALUES ($1, 's', 'expected', '[]', 'pass')`, k); err != nil {
			t.Fatal(err)
		}
	}
	runs, checks := purgeRulesUnitTestRuns(pool, time.Now().Add(-30*24*time.Hour))
	if runs != 2 || checks != 2 {
		t.Errorf("purged %d runs and %d checks, want 2 and 2", runs, checks)
	}
	for name, want := range map[string]int{"old pass": 0, "old error": 0, "old executing": 1, "recent fail": 1} {
		var n, c int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.rules_unit_test_execution WHERE key = $1`, keys[name]).Scan(&n)
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.rules_unit_test_check WHERE execution_key = $1`, keys[name]).Scan(&c)
		if n != want || c != want {
			t.Errorf("%s: %d run rows and %d check rows left, want %d of each", name, n, c, want)
		}
	}
}
