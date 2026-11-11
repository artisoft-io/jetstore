package datatable

// The query button (raw_query_tool) runs under a statement timeout shorter than
// the apiserver's write timeout, so a statement the HTTP response has given up
// on does not keep running on the database. Submit DDL (exec_ddl) has none.
//
// Skipped unless JETS_TEST_DSN names a throwaway PostgreSQL whose user may
// create databases:
//
//	docker run -d --rm --name jm067-pg -e POSTGRES_PASSWORD=pw -p 127.0.0.1::5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@127.0.0.1:<port>/postgres go test ./datatable -run QueryTool -count=1

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

const jm067DB = "jm067_query_tool"

// jm067Pool returns a one-connection pool on a fresh database, so every
// statement in a test runs on the same backend unless the pool destroyed it.
func jm067Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	bg := context.Background()
	admin, err := pgxpool.New(bg, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	for _, stmt := range []string{
		`DROP DATABASE IF EXISTS ` + jm067DB + ` WITH (FORCE)`,
		`CREATE DATABASE ` + jm067DB,
	} {
		if _, err = admin.Exec(bg, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + jm067DB
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(bg, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestQueryToolStatementTimeoutIsInsideTheWriteTimeout(t *testing.T) {
	if QueryToolStatementTimeout >= ApiServerWriteTimeout {
		t.Fatalf("the query tool's statement timeout (%v) must end before the write timeout (%v)",
			QueryToolStatementTimeout, ApiServerWriteTimeout)
	}
}

func TestQueryToolCancelsAStatementPastItsTimeout(t *testing.T) {
	pool := jm067Pool(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"users", "roles", "role_capability"} {
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	for _, s := range []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles, git_token)
			VALUES ('ide@test', 'Ide', 'hash', 1, '{ide}', 'token')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('ide', 'workspace_ide')`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("setting up: %v\n%s", err, s)
		}
	}
	// The query tool runs on the IDE login's pool; one connection, so every
	// statement below runs on the same backend unless the pool destroyed it.
	t.Setenv("JETS_IDE_POOL_SIZE", "1")
	if err := EnsureRestrictedLogins(bg, pool); err != nil {
		t.Fatalf("EnsureRestrictedLogins: %v", err)
	}
	t.Cleanup(CloseRestrictedLogins)
	ide, err := ideDB()
	if err != nil {
		t.Fatal(err)
	}

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "jm067-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	token, err := user.CreateToken("ide@test")
	if err != nil {
		t.Fatal(err)
	}

	// One second rather than 110, so the test does not wait.
	previousTimeout := queryToolStatementTimeout
	queryToolStatementTimeout = time.Second
	t.Cleanup(func() { queryToolStatementTimeout = previousTimeout })

	ctx := &DataTableContext{Dbpool: pool}
	queryTool := func(sql string) (*map[string]any, error) {
		results, _, err := ctx.ExecRawQuery(&DataTableAction{Action: "raw_query_tool", RawQuery: sql}, token, CapabilityQueryTool)
		return results, err
	}
	firstCell := func(results *map[string]any) string {
		t.Helper()
		rows := *(*results)["rows"].(*[][]any)
		if len(rows) != 1 || len(rows[0]) != 1 {
			t.Fatalf("expected one cell, got %v", rows)
		}
		return rows[0][0].(string)
	}
	// Created through the tool, so the IDE login owns it and may index it.
	if _, err = queryTool("CREATE TABLE public.probe (key integer)"); err != nil {
		t.Fatalf("creating the probe through the query tool: %v", err)
	}
	var backend, defaultTimeout string
	if err = ide.QueryRow(bg, "SELECT pg_backend_pid()::text, current_setting('statement_timeout')").Scan(&backend, &defaultTimeout); err != nil {
		t.Fatal(err)
	}

	// The limit is in force while the button's statement runs.
	results, err := queryTool("SHOW statement_timeout")
	if err != nil {
		t.Fatalf("SHOW through the query tool: %v", err)
	}
	if got := firstCell(results); got != "1s" {
		t.Errorf("statement_timeout while the query tool runs is %q, want 1s", got)
	}

	// A statement past it is cancelled, with an error that says so and says
	// where a long statement belongs.
	start := time.Now()
	_, err = queryTool("SELECT pg_sleep(4)")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("pg_sleep(4) through the query tool returned no error after %v", elapsed)
	}
	if !strings.Contains(err.Error(), "was cancelled") || !strings.Contains(err.Error(), "Submit DDL") {
		t.Errorf("the timeout's error does not explain itself: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("pg_sleep(4) ran %v; the timeout did not stop it", elapsed)
	}

	// The connection went back to the pool without the limit, and was reused
	// rather than destroyed.
	var after, afterTimeout string
	if err = ide.QueryRow(bg, "SELECT pg_backend_pid()::text, current_setting('statement_timeout')").Scan(&after, &afterTimeout); err != nil {
		t.Fatal(err)
	}
	if after != backend {
		t.Errorf("the pool's connection changed from backend %s to %s; it should have been reset and reused", backend, after)
	}
	if afterTimeout != defaultTimeout {
		t.Errorf("statement_timeout on the pooled connection is %q after the query tool, want the default %q", afterTimeout, defaultTimeout)
	}
	if _, err = ide.Exec(bg, "SELECT pg_sleep(1.5)"); err != nil {
		t.Errorf("a 1.5 s statement on the pool after the query tool: %v", err)
	}

	// The button is not a transaction and not read only: statements that refuse
	// a transaction block still run through it.
	for _, sql := range []string{
		"INSERT INTO public.probe VALUES (1)",
		"VACUUM public.probe",
		"CREATE INDEX CONCURRENTLY probe_key ON public.probe (key)",
	} {
		if _, err = queryTool(sql); err != nil {
			t.Errorf("%s through the query tool: %v", sql, err)
		}
	}

	// Submit DDL has no limit.
	if _, _, err = ctx.ExecDataManagementStatement(&DataTableAction{Action: "exec_ddl", RawQuery: "SELECT pg_sleep(1.5)"}, token); err != nil {
		t.Errorf("a 1.5 s statement through Submit DDL: %v", err)
	}
	if err = ide.QueryRow(bg, "SELECT current_setting('statement_timeout')").Scan(&afterTimeout); err != nil {
		t.Fatal(err)
	}
	if afterTimeout != defaultTimeout {
		t.Errorf("statement_timeout on the pooled connection is %q at the end, want %q", afterTimeout, defaultTimeout)
	}
}
