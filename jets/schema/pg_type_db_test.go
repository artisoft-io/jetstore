package schema

// Against a real PostgreSQL. Skipped unless JETS_TEST_DSN names a throwaway
// server whose user is a superuser, for example:
//
//	docker run -d --rm --name schema-pg -e POSTGRES_PASSWORD=pw -p 5497:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5497/postgres go test -count=1 ./jets/schema/
//
// The test creates its own apiserver user (CREATEROLE, not a superuser, as a
// deployment's is) and database.

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5/pgxpool"
)

const injectedType = "text); UPDATE public.victim SET v = 'injected'; CREATE TABLE public.z (y text"

// testDB creates the database name, owned by the login role name, and returns a
// pool connected to it as that role.
func testDB(t *testing.T, name string) *pgxpool.Pool {
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
	defer admin.Close()
	for _, s := range []string{
		"DROP DATABASE IF EXISTS " + name + " WITH (FORCE)",
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + name + `') THEN
				CREATE ROLE ` + name + ` LOGIN CREATEROLE PASSWORD 'pw';
			END IF;
		END $$`,
		"CREATE DATABASE " + name + " OWNER " + name,
		// Another test may have created the cluster-wide roles: give this
		// apiserver the standing a deployment's master user has over them.
		`DO $$ DECLARE r text; BEGIN
			FOR r IN SELECT rolname FROM pg_roles WHERE rolname ~ '^jets_(reader|ide)' LOOP
				EXECUTE format('GRANT %I TO %I WITH ADMIN OPTION', r, '` + name + `');
			END LOOP;
		END $$`,
	} {
		if _, err = admin.Exec(ctx, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	u.User = url.UserPassword(name, "pw")
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func exec(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
}

// A column type that is not one of PgTypeNames is refused before any statement
// runs: the table it would have replaced is still there, and the statements
// written into the type did not run.
func TestInjectedColumnTypeIsRefused(t *testing.T) {
	pool := testDB(t, "schema_pgtype_inject")
	ctx := context.Background()
	exec(t, pool,
		"CREATE TABLE public.victim (v text)",
		"INSERT INTO public.victim VALUES ('original')",
		"CREATE TABLE public.domain_t (a text)")

	create := TableDefinition{SchemaName: "public", TableName: "domain_t",
		Columns: []ColumnDefinition{{ColumnName: "a", DataType: injectedType}}}
	if err := create.CreateTable(pool); err == nil || !strings.Contains(err.Error(), "unknown column type") {
		t.Fatalf("CreateTable: %v; want the type refused", err)
	}
	existing, err := GetTableSchema(pool, "public", "domain_t")
	if err != nil {
		t.Fatal(err)
	}
	if err = create.UpdateTable(pool, existing); err == nil || !strings.Contains(err.Error(), "unknown column type") {
		t.Fatalf("UpdateTable: %v; want the type refused", err)
	}

	var v string
	var tables int
	if err = pool.QueryRow(ctx, "SELECT v FROM public.victim").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('domain_t', 'z')").Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if v != "original" || tables != 1 {
		t.Errorf("victim.v = %q, tables domain_t and z = %d; want original, and domain_t alone", v, tables)
	}
}

// A schema UpdateTableSchema creates is reachable by the restricted logins at
// once, not at the apiserver's next start; and before the logins exist, the
// table update still succeeds.
func TestNewSchemaIsGrantedToTheLogins(t *testing.T) {
	pool := testDB(t, "schema_pgtype_grant")
	ctx := context.Background()

	def := func(schema string) *TableDefinition {
		return &TableDefinition{SchemaName: schema, TableName: "t", Columns: []ColumnDefinition{
			{ColumnName: "k", DataType: "int", IsPK: true},
			{ColumnName: "v", DataType: "text"},
		}}
	}
	var groups int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_roles WHERE rolname IN ($1, $2)", dblogin.Reader, dblogin.IDE).Scan(&groups); err != nil {
		t.Fatal(err)
	}
	if groups == 0 {
		// The logins are cluster-wide, so this half runs only on a server no
		// earlier test has set them up on.
		if err := def("before_logins").UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("UpdateTableSchema before the logins exist: %v", err)
		}
	}

	if err := dblogin.Ensure(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := def("added_later").UpdateTableSchema(pool, false); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, "INSERT INTO added_later.t (v) VALUES ('row')")

	reader, err := dblogin.Open(ctx, pool, dblogin.Reader, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var v string
	if err = reader.QueryRow(ctx, "SELECT v FROM added_later.t").Scan(&v); err != nil || v != "row" {
		t.Fatalf("reader on the new schema: %q, %v", v, err)
	}

	ide, err := dblogin.Open(ctx, pool, dblogin.IDE, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ide.Close()
	if _, err = ide.Exec(ctx, "INSERT INTO added_later.t (v) VALUES ('from the ide')"); err != nil {
		t.Fatalf("ide login writing the new schema: %v", err)
	}
	if _, err = ide.Exec(ctx, "CREATE TABLE added_later.scratch (x int)"); err != nil {
		t.Fatalf("ide login creating a table in the new schema: %v", err)
	}
	// And the reader still cannot write there.
	if _, err = reader.Exec(ctx, "INSERT INTO added_later.t (v) VALUES ('no')"); err == nil {
		t.Fatal("the reader login wrote to the new schema")
	}
}
