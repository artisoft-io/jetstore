// Package purgetest builds a scratch JetStore database for the tests of the
// session purge, and seeds sessions into it. It is used only by tests.
//
// The database is real PostgreSQL, named by JETS_TEST_DSN; a test calling DB is
// skipped when the variable is unset. Locally:
//
//	docker run -d --rm --name srg-pg -e POSTGRES_PASSWORD=pw -p 5483:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5483/postgres \
//	  go test -count=1 ./jets/sessionpurge/... ./jets/purge_database/...
//
// Run with -count=1: the tests read jets_schema.json and jets_init_db.sql, which
// the test cache does not see.
package purgetest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JetsDir is the jets/ directory of this checkout.
func JetsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// SchemaDefinitions reads jets_schema.json.
func SchemaDefinitions(t testing.TB) []schema.TableDefinition {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(JetsDir(), "jets_schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var defs []schema.TableDefinition
	if err = json.Unmarshal(buf, &defs); err != nil {
		t.Fatal(err)
	}
	return defs
}

var historyView = regexp.MustCompile(`(?s)CREATE OR REPLACE VIEW jetsapi\.pipeline_execution_history AS.*?\n;`)

// HistoryViewStatement is the statement creating pipeline_execution_history,
// read from jets_init_db.sql.
func HistoryViewStatement(t testing.TB) string {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join(JetsDir(), "jets_init_db.sql"))
	if err != nil {
		t.Fatal(err)
	}
	stmt := historyView.FindString(string(buf))
	if stmt == "" {
		t.Fatal("jets_init_db.sql has no CREATE OR REPLACE VIEW jetsapi.pipeline_execution_history")
	}
	return stmt
}

// DB returns a pool on a fresh database named name: the jetsapi tables of
// jets_schema.json, the view pipeline_execution_history, an empty public schema
// for domain tables, and one source_period row, key 1.
func DB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of jets/sessionpurge/purgetest")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	_ = admin.QueryRow(ctx, "SELECT count(*) FROM pg_database WHERE datname = $1", name).Scan(&n)
	if n == 0 {
		if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			admin.Close()
			t.Fatal(err)
		}
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err = pool.Exec(ctx, `DROP SCHEMA IF EXISTS jetsapi CASCADE;
		DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public`); err != nil {
		t.Fatal(err)
	}
	defs := SchemaDefinitions(t)
	for i := range defs {
		if defs[i].Deleted {
			continue
		}
		if err = defs[i].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("table %s: %v", defs[i].TableName, err)
		}
	}
	if _, err = pool.Exec(ctx, HistoryViewStatement(t)); err != nil {
		t.Fatalf("creating pipeline_execution_history: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO jetsapi.source_period
		(key, year, month, day, month_period, week_period, day_period) VALUES (1, 2026, 1, 1, 313, 1357, 9497)`); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Exec runs a statement and fails the test on error.
func Exec(t testing.TB, pool *pgxpool.Pool, stmt string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), stmt, args...); err != nil {
		t.Fatalf("%v\n%s", err, stmt)
	}
}

// Session is a session to seed.
type Session struct {
	ID, Client string
	// Registered writes the session_registry row; an orphan has none.
	Registered bool
	// LastUpdate is the last_update of its registry, input_registry and status
	// rows.
	LastUpdate time.Time
	// DomainTables are created in public if missing, with a session_id column,
	// registered in input_registry, and given two rows of the session each.
	DomainTables []string
	// Runs is the statuses of its pipeline_execution_status rows; each gets a
	// details row. Empty means one completed run.
	Runs []string
}

// Seed writes s's rows: in session_registry when Registered, in every domain
// table it names and in input_registry for each, in pipeline_execution_status
// for each run, and one row in every other table the purge deletes from,
// pipeline coordinator tables included. It returns the keys of the status rows.
func Seed(t testing.TB, pool *pgxpool.Pool, s Session) []int {
	t.Helper()
	ctx := context.Background()
	lu := s.LastUpdate
	if lu.IsZero() {
		lu = time.Now()
	}
	if s.Registered {
		Exec(t, pool, `INSERT INTO jetsapi.session_registry
			(session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
			VALUES ($1, 'file', $2, 1, 313, 1357, 9497, $3::timestamptz)`, s.ID, s.Client, lu)
	}
	for _, dt := range s.DomainTables {
		ident := pgx.Identifier{dt}.Sanitize()
		Exec(t, pool, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (session_id text, v text)`, ident))
		Exec(t, pool, fmt.Sprintf(`INSERT INTO %s (session_id, v) VALUES ($1, 'a'), ($1, 'b')`, ident), s.ID)
		Exec(t, pool, `INSERT INTO jetsapi.input_registry
			(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email, last_update)
			VALUES ($1, 'ObjectA', 'k', 1, $2, 'domain_table', $3, 'system', $4::timestamptz)`, s.Client, dt, s.ID, lu)
	}
	runs := s.Runs
	if len(runs) == 0 {
		runs = []string{"completed"}
	}
	var keys []int
	for _, status := range runs {
		var key int
		err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
			(pipeline_config_key, client, process_name, main_object_type, session_id, source_period_key,
			 status, user_email, start_time, last_update, workspace_name)
			VALUES (1, $1, 'proc', 'ObjectA', $2, 1, $3, 'system', $4::timestamptz, $4::timestamptz, 'jets_ws')
			RETURNING key`, s.Client, s.ID, status, lu).Scan(&key)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_details
			(pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
			 session_id, source_period_key, status, user_email)
			VALUES (1, $1, $2, 'proc', $3, $3, 1, $4, 'system')`, key, s.Client, s.ID, status)
	}
	for _, stmt := range []string{
		`INSERT INTO jetsapi.input_loader_status (object_type, table_name, client, file_key, source_period_key, session_id, status, user_email)
			VALUES ('ObjectA', 'staging', 'c', 'k', 1, $1, 'completed', 'u')`,
		`INSERT INTO jetsapi.process_errors (session_id, error_message) VALUES ($1, 'e')`,
		`INSERT INTO jetsapi.pipeline_execution_channel_details (pipeline_execution_details_key, session_id) VALUES (1, $1)`,
		`INSERT INTO jetsapi.report_execution_status (client, report_name, session_id, status) VALUES ('c', 'r', $1, 'completed')`,
		`INSERT INTO jetsapi.compute_pipes_shard_registry (session_id, file_key) VALUES ($1, 'k')`,
		`INSERT INTO jetsapi.compute_pipes_partitions_registry (session_id, jets_partition) VALUES ($1, 'p')`,
		`INSERT INTO jetsapi.cpipes_results (session_id, jets_partition, node_id, category, name, row_count, parts_count, err)
			VALUES ($1, 'p', 0, 'c', 'n', 1, 1, '')`,
		`INSERT INTO jetsapi.cpipes_metrics (session_id, jets_partition, node_id, category, name, value, units)
			VALUES ($1, 'p', 0, 'c', 'n', 1, 'u')`,
		`INSERT INTO jetsapi.session_reservation (session_id) VALUES ($1)`,
		`INSERT INTO jetsapi.pipeline_lock (state_machine_name, session_id) VALUES ('sm-' || $1, $1)`,
		`INSERT INTO jetsapi.pipeline_coordinator_map (request_id) VALUES ('req-' || $1)`,
		`INSERT INTO jetsapi.pipeline_coordinator_map_items (request_id, session_id) VALUES ('req-' || $1, $1)`,
		`INSERT INTO jetsapi.pipeline_coordinator_lock (request_id, session_id) VALUES ('req-' || $1, $1)`,
	} {
		Exec(t, pool, stmt, s.ID)
	}
	return keys
}

// SessionRows counts the rows carrying session id id in every table, in jetsapi
// and public, with a session_id column, the archive and session_deletion
// excepted; and the
// coordinator map rows of its requests, under jetsapi.pipeline_coordinator_map.
// Tables with no row of the session are left out.
func SessionRows(t testing.TB, pool *pgxpool.Pool, id string) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT c.table_schema, c.table_name FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.column_name = 'session_id' AND c.table_schema IN ('jetsapi', 'public')
			AND t.table_type = 'BASE TABLE'
			AND c.table_name NOT IN ('pipeline_execution_status_archive', 'session_deletion')
		ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	var tables [][2]string
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, [2]string{s, n})
	}
	rows.Close()
	out := map[string]int64{}
	for _, tb := range tables {
		var n int64
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s WHERE session_id = $1`,
			pgx.Identifier{tb[0], tb[1]}.Sanitize()), id).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			out[tb[0]+"."+tb[1]] = n
		}
	}
	var n int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_coordinator_map WHERE request_id = 'req-' || $1`,
		id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		out["jetsapi.pipeline_coordinator_map"] = n
	}
	return out
}

// RowCounts counts the rows of every table in jetsapi and public.
func RowCounts(t testing.TB, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT schemaname, tablename FROM pg_tables
		WHERE schemaname IN ('jetsapi', 'public') ORDER BY 1, 2`)
	if err != nil {
		t.Fatal(err)
	}
	var tables [][2]string
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, [2]string{s, n})
	}
	rows.Close()
	out := map[string]int64{}
	for _, tb := range tables {
		var n int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{tb[0], tb[1]}.Sanitize()).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out[tb[0]+"."+tb[1]] = n
	}
	return out
}
