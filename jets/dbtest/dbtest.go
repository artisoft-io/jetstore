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
// The role in JETS_TEST_DSN needs CREATEDB (a superuser has it). A test that
// creates a database of its own beside the one JETS_TEST_DSN names still does,
// beside the package's database; those it keeps or drops as it always has.
//
// A test that runs its own test binary again in a child process, to watch it
// crash or to isolate it, hands the child its environment, and the child's Main
// finds JETS_TEST_DSN already naming the package's database, by OwnerVar. It runs
// its tests there and leaves the database to the parent, so a child sees the
// tables its parent made, as it did before each package had its own.
//
// A test binary that panics or times out does not return to Main, so its
// database stays behind. Main drops such leftovers, by the time in their name,
// once they are a day old.
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

// Main runs the package's tests on a database of their own, as the package
// documentation describes, and exits with their result.
func Main(m *testing.M) {
	dsn := os.Getenv(DSNVar)
	if dsn == "" || madeByAParent(dsn) {
		os.Exit(m.Run())
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

// drop drops the package's database, closing any connection a test left open.
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
}

// dropLeftovers drops the databases Main created more than staleAfter ago,
// which a test binary that panicked or timed out left behind. A younger one may
// belong to a package running now, and is left alone.
func dropLeftovers(ctx context.Context, admin *pgx.Conn) {
	rows, err := admin.Query(ctx, "SELECT datname FROM pg_database WHERE datname LIKE '"+prefix+"%'")
	if err != nil {
		return
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-staleAfter).Unix()
	for _, name := range names {
		m := leftover.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if created, err := strconv.ParseInt(m[1], 10, 64); err == nil && created < cutoff {
			_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		}
	}
}
