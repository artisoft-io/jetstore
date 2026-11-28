// Package dbtest gives each Go test package that needs a PostgreSQL a database
// of its own. It is used only by tests.
//
// The database tests read the server's address from JETS_TEST_DSN and create
// and drop what they need in the database it names. Until every package got its
// own database they all shared that one, so a package could fail on state
// another left there: a view over a table that a later package drops and
// recreates, or rows a later package counts. Run one package at a time on a
// fresh server and they passed; run several, in one go test or one after
// another, and some did not.
//
// A package that reads JETS_TEST_DSN calls Main from its TestMain:
//
//	func TestMain(m *testing.M) { dbtest.Main(m) }
//
// Main creates a database with a unique name on the server JETS_TEST_DSN names,
// points JETS_TEST_DSN at it for the package's tests, runs them, and drops the
// database afterwards, whether they pass or fail. When JETS_TEST_DSN is not set
// it only runs the tests, which skip as they always have.
//
// The role in JETS_TEST_DSN needs CREATEDB (a superuser has it), and CREATEROLE
// for the tests that make roles.
//
// A test that needs a database or a role of its own beside the package's makes
// it with CreateDatabase or CreateRole, which name it under the package's
// database (Name) and drop it when the test ends; DropRole drops a role the code
// under test names. Main drops whatever still carries the package database's
// name when the package ends. A cluster-wide role the code under test makes
// under a fixed name, shared by several packages, is marked by SharedRoles, or
// by Main for a package whose TestMain names it with Shared, and dropped by the
// last package to finish.
//
// A test that runs its own test binary again in a child process, to watch it
// crash or to isolate it, hands the child its environment, and the child's Main
// finds JETS_TEST_DSN already naming the package's database, by OwnerVar. It runs
// its tests there and leaves the database to the parent, so a child sees the
// tables its parent made, as it did before each package had its own.
//
// A test binary that panics or times out does not return to Main, so its
// database, and the databases and roles its tests made under its name, stay
// behind. Main drops such leftovers, by the time in their name, once they are a
// day old.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DSNVar is the environment variable naming the test server.
const DSNVar = "JETS_TEST_DSN"

// OwnerVar names the database Main made, in the environment of the package's
// tests and of any process they start.
const OwnerVar = "JETS_TEST_DSN_DBTEST"

// prefix starts the name of every database Main creates; the Unix time of its
// creation follows, so a leftover can be recognised and its age read.
const prefix = "jetstest_"

// staleAfter is the age at which a leftover database is dropped.
const staleAfter = 24 * time.Hour

var leftover = regexp.MustCompile(`^` + prefix + `([0-9]+)_`)

// An Option changes what Main does.
type Option func()

// RolesOf names the roles the code under test makes for a database under names
// derived from the database's, which no test chooses: a login per database,
// say. Main drops them for every database it drops, after the database: its
// own, those its tests made with CreateDatabase, and the leftovers it sweeps.
func RolesOf(f func(database string) []string) Option {
	return func() { rolesOf = f }
}

// rolesOf is the function RolesOf gave Main, if any.
var rolesOf func(database string) []string

// Shared names the cluster-wide roles the code under test makes under fixed
// names, which several packages share: Main marks those that exist when the
// package ends, as SharedRoles does when a test ends, so that whichever test
// made them, the last package to finish drops them. It is for a package with
// more than one way to make the roles, where a test that makes them without
// calling SharedRoles would otherwise leave them on the server.
func Shared(roles ...string) Option {
	return func() { sharedRoles = append(sharedRoles, roles...) }
}

// sharedRoles is the roles Shared gave Main.
var sharedRoles []string

// created lists the databases CreateDatabase made in this process, whose roles
// Main drops when the package ends.
var created struct {
	sync.Mutex
	names []string
}

// Main runs the package's tests on a database of their own, as the package
// documentation describes, and exits with their result.
func Main(m *testing.M, opts ...Option) {
	for _, opt := range opts {
		opt()
	}
	dsn := os.Getenv(DSNVar)
	if dsn == "" {
		os.Exit(m.Run())
	}
	if madeByAParent(dsn) {
		code := m.Run()
		dropRolesOfCreated(dsn)
		os.Exit(code)
	}
	ctx := context.Background()
	admin, err := connectAdmin(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbtest: connecting to the server %s names: %v\n", DSNVar, err)
		os.Exit(1)
	}
	dropLeftovers(ctx, admin)
	name := databaseName(time.Now())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close(ctx)
		fmt.Fprintf(os.Stderr, "dbtest: creating the test database %s (the role in %s needs CREATEDB): %v\n",
			name, DSNVar, err)
		os.Exit(1)
	}
	admin.Close(ctx)
	packageDSN, err := withDatabase(dsn, name)
	if err == nil {
		err = os.Setenv(DSNVar, packageDSN)
	}
	if err == nil {
		err = os.Setenv(OwnerVar, name)
	}
	code := 1
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbtest: pointing %s at %s: %v\n", DSNVar, name, err)
	} else {
		code = m.Run()
	}
	os.Setenv(DSNVar, dsn)
	os.Unsetenv(OwnerVar)
	drop(ctx, dsn, name)
	os.Exit(code)
}

// madeByAParent is true when dsn names the database OwnerVar names: this test
// binary is a child started by a test of a binary whose Main made it.
func madeByAParent(dsn string) bool {
	owner := os.Getenv(OwnerVar)
	if owner == "" {
		return false
	}
	cfg, err := pgconn.ParseConfig(dsn)
	return err == nil && cfg.Database == owner
}

// connectAdmin connects to the database dsn names, or to the server's
// maintenance database, postgres, when that one cannot be reached. Main
// creates and drops the package's database on this connection, which is never
// a connection to the package's database itself.
func connectAdmin(ctx context.Context, dsn string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err == nil {
		return conn, nil
	}
	maintenance, err2 := withDatabase(dsn, "postgres")
	if err2 != nil {
		return nil, err
	}
	if conn, err2 = pgx.Connect(ctx, maintenance); err2 != nil {
		return nil, err
	}
	return conn, nil
}

// databaseName is prefix, the Unix time, eight random hex digits and the test
// binary's package name, within PostgreSQL's 63 bytes.
func databaseName(now time.Time) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	pkg := strings.TrimSuffix(filepath.Base(os.Args[0]), ".test")
	pkg = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r - 'A' + 'a'
		}
		return '_'
	}, pkg)
	name := prefix + strconv.FormatInt(now.Unix(), 10) + "_" + hex.EncodeToString(b[:]) + "_" + pkg
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

// withDatabase is dsn with its database replaced by name. dsn is a URL
// (postgres://...) or keyword/value settings (host=... dbname=...); in the
// second a later keyword overrides an earlier one.
func withDatabase(dsn, name string) (string, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		u.Path = "/" + name
		u.RawPath = ""
		return u.String(), nil
	}
	return dsn + " dbname=" + name, nil
}

// drop drops the package's database, closing any connection a test left open,
// then the databases and roles its tests made under its name and did not drop,
// then marks the roles Shared names and, when no other test database is left,
// drops the shared roles.
func drop(ctx context.Context, dsn, name string) {
	admin, err := connectAdmin(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dbtest: warning, the test database %s was not dropped: %v\n", name, err)
		return
	}
	defer admin.Close(ctx)
	if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
		fmt.Fprintf(os.Stderr, "dbtest: warning, the test database %s was not dropped: %v\n", name, err)
	}
	under := func(n string) bool { return strings.HasPrefix(n, name+"_") }
	dropped := []string{name}
	for _, db := range list(ctx, admin, databases, under) {
		if _, err = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)"); err != nil {
			fmt.Fprintf(os.Stderr, "dbtest: warning, the test database %s was not dropped: %v\n", db, err)
		}
		dropped = append(dropped, db)
	}
	created.Lock()
	dropped = append(dropped, created.names...)
	created.Unlock()
	for _, role := range append(roleNamesOf(dropped), list(ctx, admin, roles, under)...) {
		if err = dropRoleOn(ctx, admin, role); err != nil {
			fmt.Fprintf(os.Stderr, "dbtest: warning, the test role %s was not dropped: %v\n", role, err)
		}
	}
	markShared(ctx, admin, sharedRoles)
	dropSharedRoles(ctx, admin)
}

// markShared marks those of roles that exist as made by a test, for
// dropSharedRoles.
func markShared(ctx context.Context, conn *pgx.Conn, roles []string) {
	for _, role := range roles {
		if exists(conn, role) {
			_, _ = conn.Exec(ctx, "COMMENT ON ROLE "+pgx.Identifier{role}.Sanitize()+" IS '"+sharedMarker+"'")
		}
	}
}

// roleNamesOf is the roles RolesOf names for databases, each once.
func roleNamesOf(dbs []string) []string {
	if rolesOf == nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	for _, db := range dbs {
		for _, role := range rolesOf(db) {
			if !seen[role] {
				seen[role] = true
				names = append(names, role)
			}
		}
	}
	return names
}

// dropRolesOfCreated drops the roles RolesOf names for the databases this
// process's tests made, on the database dsn names: what a child process, which
// leaves the package's database to its parent, drops itself.
func dropRolesOfCreated(dsn string) {
	created.Lock()
	names := roleNamesOf(created.names)
	created.Unlock()
	if len(names) == 0 {
		return
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return
	}
	defer conn.Close(ctx)
	for _, role := range names {
		if err = dropRoleOn(ctx, conn, role); err != nil {
			fmt.Fprintf(os.Stderr, "dbtest: warning, the test role %s was not dropped: %v\n", role, err)
		}
	}
}

// The queries list reads: every database, and every role, whose name starts
// with prefix.
const (
	databases = "SELECT datname FROM pg_database WHERE starts_with(datname, '" + prefix + "')"
	roles     = "SELECT rolname FROM pg_roles WHERE starts_with(rolname, '" + prefix + "')"
)

// list runs query, one of databases and roles, and returns the names keep is
// true of; nothing when the query fails.
func list(ctx context.Context, admin *pgx.Conn, query string, keep func(string) bool) []string {
	rows, err := admin.Query(ctx, query)
	if err != nil {
		return nil
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil
	}
	kept := names[:0]
	for _, n := range names {
		if keep(n) {
			kept = append(kept, n)
		}
	}
	return kept
}

// dropSharedRoles drops the roles SharedRoles marked, when no test database is
// left: a package still running has its database, and may be using them. A
// role something still depends on, in a database that is not a test's, stays.
func dropSharedRoles(ctx context.Context, admin *pgx.Conn) {
	if len(list(ctx, admin, databases, func(string) bool { return true })) > 0 {
		return
	}
	rows, err := admin.Query(ctx, `SELECT r.rolname FROM pg_roles r
		JOIN pg_shdescription d ON d.objoid = r.oid AND d.classoid = 'pg_authid'::regclass
		WHERE d.description = $1`, sharedMarker)
	if err != nil {
		return
	}
	shared, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return
	}
	for _, role := range shared {
		_, _ = admin.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize())
	}
}

// dropLeftovers drops the databases Main created more than staleAfter ago,
// which a test binary that panicked or timed out left behind, with the
// databases and then the roles its tests made under their names. A younger one
// may belong to a package running now, and is left alone.
func dropLeftovers(ctx context.Context, admin *pgx.Conn) {
	cutoff := time.Now().Add(-staleAfter).Unix()
	stale := func(name string) bool {
		m := leftover.FindStringSubmatch(name)
		if m == nil {
			return false
		}
		created, err := strconv.ParseInt(m[1], 10, 64)
		return err == nil && created < cutoff
	}
	dropped := list(ctx, admin, databases, stale)
	for _, name := range dropped {
		_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	}
	for _, name := range append(roleNamesOf(dropped), list(ctx, admin, roles, stale)...) {
		_ = dropRoleOn(ctx, admin, name)
	}
}
