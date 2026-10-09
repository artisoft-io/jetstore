package main

// The two claims execScript rests on are claims about PostgreSQL and pgx rather
// than about this code, so they are worth running rather than reading: that a
// whole script submitted in one Exec is parsed by the server (and therefore
// that a `;` in a comment or a literal is not a statement boundary), and that
// the server wraps such a script in one implicit transaction (and therefore
// that a DELETE is undone when the INSERT after it fails).
//
// It skips unless JETS_TEST_DSN names a PostgreSQL to write to. It creates and
// drops its own tables in a schema it creates, so point it at a scratch
// database and not at a jetsapi you care about. On the machine that wrote this:
//
//	docker run -d --name jets-sqltest -e POSTGRES_PASSWORD=test \
//	  -e POSTGRES_DB=jetstest -p 55432:5432 postgres:16-alpine
//	JETS_TEST_DSN='postgres://postgres:test@localhost:55432/jetstest' \
//	  go test -count=1 -run TestExecScript ./jets/update_db/
//
// Measured against PostgreSQL 16.15 and pgx v5.10.0 on 2026-09-12.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(),
		"DROP SCHEMA IF EXISTS sqlscript_test CASCADE; CREATE SCHEMA sqlscript_test"); err != nil {
		t.Fatalf("preparing schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS sqlscript_test CASCADE")
	})
	return pool
}

// The failure that produced this change: a semicolon inside a `--` comment. The
// reader this replaced split the statement at it, so the fragment after the
// semicolon — bare prose continuing the comment's sentence — reached the server
// as SQL.
func TestExecScriptIgnoresSemicolonsThatAreNotTerminators(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	script := `-- Registering it does not schedule it; a pipeline runs when somebody
-- starts it.
CREATE TABLE sqlscript_test.t (k text PRIMARY KEY, v text);
-- A literal with a semicolon in it, which two walrus client scripts carry.
INSERT INTO sqlscript_test.t VALUES ('a', 'Drug on Formulary; Non-preferred');
INSERT INTO sqlscript_test.t VALUES ('b', 'x') ON CONFLICT DO NOTHING;
-- A quoted identifier with one, and a dollar-quoted body.
CREATE TABLE sqlscript_test."odd;name" ("c;1" int);
DO $fn$ BEGIN INSERT INTO sqlscript_test."odd;name" VALUES (1); END $fn$;
-- No terminator on the last statement, and a trailing comment after it.
INSERT INTO sqlscript_test.t VALUES ('c', 'y')
-- End of Export Client Script
`
	if err := execScript(ctx, pool, "fixture.sql", script); err != nil {
		t.Fatalf("execScript: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sqlscript_test.t`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("sqlscript_test.t has %d rows, want 3", n)
	}
	var v string
	if err := pool.QueryRow(ctx,
		`SELECT v FROM sqlscript_test.t WHERE k = 'a'`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != "Drug on Formulary; Non-preferred" {
		t.Errorf("literal = %q", v)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM sqlscript_test."odd;name"`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf(`sqlscript_test."odd;name" has %d rows, want 1`, n)
	}
}

// The init db scripts are written as DELETE followed by INSERT against the same
// table. Under the per-statement execution this replaced, a failing INSERT left
// the configuration deleted.
func TestExecScriptIsOneTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE TABLE sqlscript_test.cfg (k text PRIMARY KEY);
		INSERT INTO sqlscript_test.cfg VALUES ('keep-me')`); err != nil {
		t.Fatal(err)
	}
	script := `DELETE FROM sqlscript_test.cfg WHERE k = 'keep-me';
INSERT INTO sqlscript_test.cfg VALUES ('new');
INSERT INTO sqlscript_test.cfg VALUES ('new');
`
	err := execScript(ctx, pool, "fixture.sql", script)
	if err == nil {
		t.Fatal("execScript: want the duplicate key to fail")
	}
	if !strings.Contains(err.Error(), "fixture.sql") {
		t.Errorf("error does not name the file: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM sqlscript_test.cfg WHERE k = 'keep-me'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("the row deleted by the first statement was not restored: count = %d, want 1", n)
	}
}

// A script that opens its own transaction keeps its own semantics: an explicit
// BEGIN supersedes the implicit one rather than conflicting with it. Two report
// scripts in the corpus are written this way, and an explicit transaction
// opened by this code around the script would have broken them.
func TestExecScriptHonoursAnExplicitTransaction(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	script := `BEGIN;
CREATE TEMP TABLE tmp_cfg (k text) ON COMMIT DROP;
INSERT INTO tmp_cfg VALUES ('x');
CREATE TABLE sqlscript_test.after_commit AS SELECT count(*) AS n FROM tmp_cfg;
COMMIT;
`
	if err := execScript(ctx, pool, "fixture.sql", script); err != nil {
		t.Fatalf("execScript: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT n FROM sqlscript_test.after_commit`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("ON COMMIT DROP temp table held %d rows, want 1", n)
	}
}

// Where PostgreSQL reports a character position, it is turned back into a line
// and column of the file — counted over the whole script, so the semicolon in
// the comment on line 2 must not shift it. This is what replaces the statement
// text the per-statement reader could have printed and did not.
//
// Not every error carries a position: a constraint violation reports none, and
// the message then names the file alone. TestExecScriptIsOneTransaction
// exercises that path.
func TestExecScriptLocatesAnErrorByLineAndColumn(t *testing.T) {
	pool := testPool(t)
	script := "SELECT 1;\n-- a comment; with a semicolon\nSELECT oops FROM nowhere;\n"
	err := execScript(context.Background(), pool, "fixture.sql", script)
	if err == nil {
		t.Fatal("want an error")
	}
	// Line 3 column 18 is the `n` of `nowhere`.
	if !strings.Contains(err.Error(), "fixture.sql:3:18") {
		t.Errorf("error = %v, want it to name fixture.sql:3:18", err)
	}
}
