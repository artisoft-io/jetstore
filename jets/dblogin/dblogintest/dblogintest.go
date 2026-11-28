// Package dblogintest gives a test a scratch database owned by an apiserver
// user set up the way a deployment's is, for the tests that run SQL on the
// restricted logins (package dblogin). It is used only by tests.
//
// The server is a throwaway PostgreSQL named by JETS_TEST_DSN, whose user is a
// superuser; a test calling Apiserver is skipped when the variable is unset.
// Locally:
//
//	docker run -d --rm --name dbl-pg -e POSTGRES_PASSWORD=pw -p 5491:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5491/postgres \
//	  go test -count=1 ./jets/datatable/ ./jets/run_reports/delegate/
//
// # The two apiservers
//
// By default the apiserver user holds CREATEROLE and is not a superuser. It may
// not create event triggers, so dblogin.Ensure cannot install the IDE code
// guard, and the guard is off. With SuperuserVar set to 1 (or true) it is a
// superuser, which may create them, as the master user of an RDS or Aurora
// database may, and the guard is on. Run the tests both ways: a test whose
// expectation depends on the guard reads dblogin.CodeGuard rather than assume
// either.
//
// # Running in any order
//
// The restricted group roles and their logins (dblogin.Reader, dblogin.IDE)
// are cluster-wide, and whichever test reaches dblogin.Ensure first creates
// them, as its own apiserver user. A user that holds CREATEROLE and is not a
// superuser may grant, alter or drop only a role it has ADMIN OPTION on, so an
// apiserver that did not create them would fail with "permission denied to
// grant role". Apiserver gives it ADMIN OPTION on every one that exists: the
// standing a deployment's master user has over the roles it created.
package dblogintest

import (
	"context"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/artisoft-io/jetstore/jets/dbtest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SuperuserVar is the environment variable that makes the apiserver user a
// superuser, so that the IDE code guard is on.
const SuperuserVar = "JETS_TEST_APISERVER_SUPERUSER"

// password is every test apiserver's password.
const password = "pw"

// Superuser is true when SuperuserVar asks for a superuser apiserver.
func Superuser() bool {
	on, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(SuperuserVar)))
	return on
}

// Apiserver creates a database owned by a login role, which it creates if need
// be and gives the attributes SuperuserVar asks for, and returns a pool on the
// database connected as the role. The role also gets ADMIN OPTION on every
// restricted role and login that exists.
//
// role and database are suffixes: the role and the database are named under
// the package's test database, dbtest.Name(t, role) and dbtest.Name(t,
// database), and are dropped when the test ends, with the database's restricted
// logins. The restricted group roles, which every database shares, are marked
// for the last test package to drop (dbtest.SharedRoles).
func Apiserver(t *testing.T, role, database string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of jets/dblogin/dblogintest")
	}
	attrs := "NOSUPERUSER CREATEROLE"
	if Superuser() {
		attrs = "SUPERUSER"
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	role, dbName := dbtest.Name(t, role), dbtest.Name(t, database)
	// A role made by an earlier call in this test owns that call's database,
	// and is dropped after it.
	var made int
	if err = admin.QueryRow(ctx, "SELECT count(*) FROM pg_roles WHERE rolname = $1", role).Scan(&made); err != nil {
		t.Fatal(err)
	}
	if made == 0 {
		dbtest.DropRole(t, role)
	}
	dbtest.SharedRoles(t, dblogin.Reader, dblogin.IDE)
	for _, login := range Logins(dbName) {
		dbtest.DropRole(t, login)
	}
	r := pgx.Identifier{role}.Sanitize()
	for _, s := range []string{
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + strings.ReplaceAll(role, "'", "''") + `') THEN
				CREATE ROLE ` + r + ` LOGIN;
			END IF;
		END $$`,
		"ALTER ROLE " + r + " LOGIN PASSWORD '" + password + "' " + attrs,
	} {
		if _, err = admin.Exec(ctx, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
	dbtest.CreateDatabase(t, database, "OWNER "+r)
	adoptRestrictedRoles(t, admin, role)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, password)
	u.Path = "/" + dbName
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Logins is the restricted logins dblogin.Ensure makes for database. A package
// whose tests run Ensure passes it to its TestMain, so that the logins of every
// database its tests made go with them:
//
//	func TestMain(m *testing.M) { dbtest.Main(m, dbtest.RolesOf(dblogintest.Logins)) }
func Logins(database string) []string {
	var logins []string
	for _, group := range []string{dblogin.Reader, dblogin.IDE} {
		login, _ := dblogin.LoginName(context.Background(), databaseName(database), group)
		logins = append(logins, login)
	}
	return logins
}

// databaseName answers dblogin.LoginName's query for the current database with
// its own name, so the logins of a database are known before it exists.
type databaseName string

func (d databaseName) QueryRow(context.Context, string, ...any) pgx.Row { return d }

func (d databaseName) Scan(dest ...any) error {
	*dest[0].(*string) = string(d)
	return nil
}

// restrictedRoles matches the names of the restricted group roles and of their
// logins, which are the group's name and a suffix.
var restrictedRoles = "^(" + regexp.QuoteMeta(dblogin.Reader) + "|" + regexp.QuoteMeta(dblogin.IDE) + ")(_|$)"

// adoptRestrictedRoles gives owner ADMIN OPTION on every restricted role and
// login that exists, as the superuser admin is connected.
func adoptRestrictedRoles(t *testing.T, admin *pgxpool.Pool, owner string) {
	t.Helper()
	_, err := admin.Exec(context.Background(), `DO $$ DECLARE r text; BEGIN
		FOR r IN SELECT rolname FROM pg_roles WHERE rolname ~ '`+restrictedRoles+`' LOOP
			EXECUTE format('GRANT %I TO %I WITH ADMIN OPTION', r, '`+strings.ReplaceAll(owner, "'", "''")+`');
		END LOOP;
	END $$`)
	if err != nil {
		t.Fatalf("giving %s ADMIN OPTION on the restricted roles: %v", owner, err)
	}
}
