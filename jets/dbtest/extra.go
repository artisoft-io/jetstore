package dbtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// sharedMarker is the comment SharedRoles puts on a cluster-wide role a test
// cannot name: the last package to finish drops the roles that carry it.
const sharedMarker = "dbtest: made by a test; dropped with the last test database"

// Name is the name of an extra database or role a test makes: the package's
// database, an underscore and suffix, lower-cased, within PostgreSQL's 63
// bytes. Main drops what carries its database's name when the package ends,
// and the leftover sweep drops it once it is a day old, so a test that makes
// its databases and roles under such names leaves nothing on the server.
//
// The test is skipped when JETS_TEST_DSN is not set, and fails when its
// package's TestMain does not call Main.
func Name(t testing.TB, suffix string) string {
	t.Helper()
	if os.Getenv(DSNVar) == "" {
		t.Skip(DSNVar + " is not set")
	}
	owner := os.Getenv(OwnerVar)
	if owner == "" {
		t.Fatalf("%s is not set: the package's TestMain must call dbtest.Main", OwnerVar)
	}
	return extraName(owner, suffix)
}

// extraName is owner_suffix, suffix lower-cased and anything but a letter, a
// digit or an underscore made an underscore. A name over 63 bytes keeps its
// first 54 and ends with an underscore and eight hex digits of the suffix's
// hash, so two long suffixes still make two names.
func extraName(owner, suffix string) string {
	suffix = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r - 'A' + 'a'
		}
		return '_'
	}, suffix)
	name := owner + "_" + suffix
	if len(name) > 63 {
		sum := sha256.Sum256([]byte(suffix))
		name = name[:54] + "_" + hex.EncodeToString(sum[:4])
	}
	return name
}

// DSN is JETS_TEST_DSN with its database replaced by database.
func DSN(t testing.TB, database string) string {
	t.Helper()
	dsn, err := withDatabase(os.Getenv(DSNVar), database)
	if err != nil {
		t.Fatalf("pointing %s at %s: %v", DSNVar, database, err)
	}
	return dsn
}

// CreateDatabase creates the database Name(t, suffix), with the CREATE
// DATABASE options given (OWNER r, TEMPLATE template0, ...), and drops it WITH
// (FORCE) when the test ends. One an earlier test left under that name is
// dropped first. It returns the database's name.
func CreateDatabase(t testing.TB, suffix string, options ...string) string {
	t.Helper()
	name := Name(t, suffix)
	quoted := pgx.Identifier{name}.Sanitize()
	execOrFail(t, "DROP DATABASE IF EXISTS "+quoted+" WITH (FORCE)")
	execOrFail(t, strings.TrimSpace("CREATE DATABASE "+quoted+" "+strings.Join(options, " ")))
	created.Lock()
	created.names = append(created.names, name)
	created.Unlock()
	dsn := os.Getenv(DSNVar)
	t.Cleanup(func() { dropDatabase(t, dsn, name) })
	return name
}

// CreateRole creates the role Name(t, suffix) with the attributes given (LOGIN
// PASSWORD 'pw', NOSUPERUSER CREATEROLE, ...), and drops it when the test ends.
// Cleanups run last in, first out, so a database created after the role, owned
// by it, is dropped before it. One an earlier test left is dropped first. It
// returns the role's name.
func CreateRole(t testing.TB, suffix, attributes string) string {
	t.Helper()
	name := Name(t, suffix)
	dsn := os.Getenv(DSNVar)
	dropRole(t, dsn, name)
	execOrFail(t, strings.TrimSpace("CREATE ROLE "+pgx.Identifier{name}.Sanitize()+" "+attributes))
	DropRole(t, name)
	return name
}

// DropRole drops role when the test ends, with whatever it owns and every
// privilege granted to it or by it in the package's database. It is for a role
// a test does not name itself, such as a login the code under test makes for a
// database the test created: call it before creating that database, so that the
// database is dropped first.
func DropRole(t testing.TB, role string) {
	t.Helper()
	dsn := os.Getenv(DSNVar)
	t.Cleanup(func() { dropRole(t, dsn, role) })
}

// SharedRoles marks, when the test ends, those of roles that exist as made by
// a test. It is for the cluster-wide roles the code under test makes under
// fixed names, which a test cannot name under its package's database and which
// several packages share: the package whose Main finds no other test database
// left drops every role so marked that nothing depends on.
func SharedRoles(t testing.TB, roles ...string) {
	t.Helper()
	dsn := os.Getenv(DSNVar)
	t.Cleanup(func() {
		conn, err := pgx.Connect(context.Background(), dsn)
		if err != nil {
			t.Logf("dbtest: marking the shared roles: %v", err)
			return
		}
		defer conn.Close(context.Background())
		markShared(context.Background(), conn, roles)
	})
}

// execOrFail runs stmt on the package's database and fails the test on error.
func execOrFail(t testing.TB, stmt string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv(DSNVar))
	if err != nil {
		t.Fatalf("dbtest: connecting to the package's database: %v", err)
	}
	defer conn.Close(ctx)
	if _, err = conn.Exec(ctx, stmt); err != nil {
		t.Fatalf("dbtest: %v\n%s", err, stmt)
	}
}

// exists is true when role exists.
func exists(conn *pgx.Conn, role string) bool {
	var n int
	err := conn.QueryRow(context.Background(), "SELECT count(*) FROM pg_roles WHERE rolname = $1", role).Scan(&n)
	return err == nil && n > 0
}

// dropDatabase drops database on the server dsn names, connected to dsn's
// database, and reports a failure without failing the test.
func dropDatabase(t testing.TB, dsn, database string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err == nil {
		defer conn.Close(ctx)
		_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)")
	}
	if err != nil {
		t.Errorf("dbtest: dropping the database %s: %v", database, err)
	}
}

// dropRole drops role, as dropRoleOn does, on the database dsn names, and
// reports a failure without failing the test.
func dropRole(t testing.TB, dsn, role string) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err == nil {
		defer conn.Close(ctx)
		err = dropRoleOn(ctx, conn, role)
	}
	if err != nil {
		t.Errorf("dbtest: dropping the role %s: %v", role, err)
	}
}

// dropRoleOn drops role if it exists, after DROP OWNED BY, which drops what it
// owns in conn's database and revokes its privileges there and on the
// cluster's shared objects, the grants of role memberships it made included.
// What it owns in another database stops the drop; dropping that database
// first is the caller's to do.
func dropRoleOn(ctx context.Context, conn *pgx.Conn, role string) error {
	if !exists(conn, role) {
		return nil
	}
	quoted := pgx.Identifier{role}.Sanitize()
	if _, err := conn.Exec(ctx, "DROP OWNED BY "+quoted); err != nil {
		return err
	}
	_, err := conn.Exec(ctx, "DROP ROLE "+quoted)
	return err
}
