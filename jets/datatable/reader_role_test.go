package datatable

// A user's reads run on the reader login (ReaderRole, readOnlyTx). This test pins what that role
// may read, through the three read paths a jetstore_read user can send SQL on:
// raw_query, raw_query_map and the data-table read.
//
// It connects as a database user that is not a superuser and holds CREATEROLE,
// and that owns its database, as the master user of a deployment's Aurora
// cluster does. Skipped unless JETS_TEST_DSN names a throwaway PostgreSQL whose
// user may create roles and databases:
//
//	docker run -d --rm --name jm095-pg -e POSTGRES_PASSWORD=pw -p 5472:5432 postgres:15
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5472/postgres go test ./datatable -run ReaderRole -count=1

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	jm095Owner = "jm095_owner"
	jm095DB    = "jm095_reader"
)

// jm095Pool returns a pool on a fresh database, connected as its owner, a
// CREATEROLE user that is not a superuser.
func jm095Pool(t *testing.T) *pgxpool.Pool {
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
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + jm095Owner + `') THEN
				CREATE ROLE ` + jm095Owner + ` LOGIN CREATEROLE PASSWORD 'pw';
			END IF;
		END $$`,
		`DROP DATABASE IF EXISTS ` + jm095DB + ` WITH (FORCE)`,
		`CREATE DATABASE ` + jm095DB + ` OWNER ` + jm095Owner,
	} {
		if _, err = admin.Exec(bg, stmt); err != nil {
			t.Fatalf("%v\n%s", err, stmt)
		}
	}
	// Another test in this run may have created the roles as the superuser. On a
	// deployment the master user creates them and administers them; give the
	// owner the same standing here.
	adoptRestrictedRoles(t, admin, jm095Owner)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(jm095Owner, "pw")
	u.Path = "/" + jm095DB
	pool, err := pgxpool.New(bg, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// adoptRestrictedRoles gives owner ADMIN OPTION on every restricted role and
// login that exists, the standing a deployment's master user has over the
// roles it created.
func adoptRestrictedRoles(t *testing.T, admin *pgxpool.Pool, owner string) {
	t.Helper()
	_, err := admin.Exec(context.Background(), `DO $$ DECLARE r text; BEGIN
		FOR r IN SELECT rolname FROM pg_roles WHERE rolname ~ '^jets_(reader|ide)' LOOP
			EXECUTE format('GRANT %I TO %I WITH ADMIN OPTION', r, '`+owner+`');
		END LOOP;
	END $$`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestReaderRoleWithholdsSecretsAndExtensions(t *testing.T) {
	pool := jm095Pool(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"users", "roles", "role_capability", "secret_rotation"} {
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	for _, s := range []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles, git_token)
			VALUES ('reader@test', 'Reader', 'hash', 1, '{reader}', 'token')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('reader', 'jetstore_read')`,
		`INSERT INTO jetsapi.secret_rotation (secret, last_update) VALUES ('s3cret', now())`,
		`CREATE TABLE public.domain_probe (key integer, name text)`,
		`INSERT INTO public.domain_probe VALUES (1, 'a')`,
		// An extension in its own schema, as aws_s3 is on Aurora.
		`CREATE SCHEMA ext_probe`,
		`CREATE EXTENSION pgcrypto SCHEMA ext_probe`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("setting up: %v\n%s", err, s)
		}
	}
	// Twice: it runs at every apiserver start.
	for i := 0; i < 2; i++ {
		if err := EnsureRestrictedLogins(bg, pool); err != nil {
			t.Fatalf("EnsureRestrictedLogins, pass %d: %v", i+1, err)
		}
	}
	// A table created after the start is readable too (default privileges).
	if _, err := pool.Exec(bg, `CREATE TABLE public.later_probe (key integer); INSERT INTO public.later_probe VALUES (1)`); err != nil {
		t.Fatal(err)
	}

	// The control: the apiserver's own user reads the secrets, so what refuses
	// them below is the role.
	var password string
	if err := pool.QueryRow(bg, `SELECT password FROM jetsapi.users`).Scan(&password); err != nil || password != "hash" {
		t.Fatalf("the owner reading jetsapi.users.password: %q, %v", password, err)
	}

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "jm095-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	token, err := user.CreateToken("reader@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &DataTableContext{Dbpool: pool}
	rawQuery := func(sql string) error {
		_, _, err := ctx.ExecRawQuery(&DataTableAction{Action: "raw_query", RawQuery: sql}, token, CapabilityReadData)
		return err
	}
	rawQueryMap := func(sql string) error {
		_, _, err := ctx.ExecRawQueryMap(&DataTableAction{Action: "raw_query_map",
			RawQueryMap: map[string]string{"q": sql}}, token)
		return err
	}
	read := func(schema, table string, columns ...string) error {
		cols := make([]Column, 0, len(columns))
		for _, c := range columns {
			cols = append(cols, Column{Table: table, Column: c})
		}
		_, _, err := ctx.DoReadAction(&DataTableAction{Action: "read",
			FromClauses: []FromClause{{Schema: schema, Table: table}},
			Columns:     cols, Limit: 10}, token)
		return err
	}
	refused := func(what string, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err %v, want %q", what, err, want)
		}
	}
	allowed := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}

	// The secret columns, by every path.
	refused("raw_query of users.password", rawQuery(`SELECT password FROM jetsapi.users`), "permission denied")
	refused("raw_query of users.git_token", rawQuery(`SELECT git_token FROM jetsapi.users`), "permission denied")
	refused("raw_query of users.*", rawQuery(`SELECT * FROM jetsapi.users`), "permission denied")
	refused("raw_query of secret_rotation.secret", rawQuery(`SELECT secret FROM jetsapi.secret_rotation`), "permission denied")
	refused("raw_query_map of users.password", rawQueryMap(`SELECT password FROM jetsapi.users`), "permission denied")
	refused("read of users.password", read("jetsapi", "users", "user_email", "password"), "permission denied")
	refused("a WITH clause reading users.git_token", func() error {
		_, _, err := ctx.DoReadAction(&DataTableAction{Action: "read",
			FromClauses: []FromClause{{Table: "t"}},
			Columns:     []Column{{Table: "t", Column: "git_token"}},
			WithClauses: []WithClause{{Name: "t", Stmt: "SELECT git_token FROM jetsapi.users"}},
			Limit:       10}, token)
		return err
	}(), "permission denied")

	// An extension's functions.
	refused("raw_query calling an extension", rawQuery(`SELECT ext_probe.digest('x', 'sha256')`), "permission denied for schema ext_probe")

	// What the screens read still reads.
	allowed("raw_query of users' other columns", rawQuery(`SELECT user_email, name, encrypted_roles, is_active, git_name FROM jetsapi.users`))
	allowed("raw_query of secret_rotation.last_update", rawQuery(`SELECT max(last_update) FROM jetsapi.secret_rotation`))
	allowed("raw_query_map of role_capability", rawQueryMap(`SELECT role, capability FROM jetsapi.role_capability`))
	allowed("read of users' other columns", read("jetsapi", "users", "user_email", "name"))
	allowed("read of a domain table", read("public", "domain_probe", "key", "name"))
	allowed("raw_query of a table created after the start", rawQuery(`SELECT key FROM public.later_probe`))
	allowed("raw_query of the catalog", rawQuery(`SELECT column_name FROM information_schema.columns WHERE table_name = 'domain_probe'`))
}
