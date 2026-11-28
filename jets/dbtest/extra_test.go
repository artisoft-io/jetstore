package dbtest

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestExtraName(t *testing.T) {
	owner := "jetstest_1764201600_abcdef01_dbtest"
	if got := extraName(owner, "My-DB"); got != owner+"_my_db" {
		t.Errorf("extraName = %q", got)
	}
	long := strings.Repeat("x", 40)
	a, b := extraName(owner, long+"a"), extraName(owner, long+"b")
	if len(a) > 63 || len(b) > 63 {
		t.Errorf("%q or %q is longer than 63 bytes", a, b)
	}
	if a == b {
		t.Errorf("two long suffixes make one name, %q", a)
	}
	if !strings.HasPrefix(a, owner+"_") {
		t.Errorf("%q is not under %q", a, owner)
	}
	if m := leftover.FindStringSubmatch(a); m == nil || m[1] != "1764201600" {
		t.Errorf("%q does not carry its package's creation time where dropLeftovers reads it", a)
	}
}

// admin is a connection to the package's database, or skips the test.
func admin(t *testing.T) *pgx.Conn {
	t.Helper()
	dsn := os.Getenv(DSNVar)
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set")
	}
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

func count(t *testing.T, conn *pgx.Conn, query, name string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(context.Background(), query, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

const (
	databaseExists = "SELECT count(*) FROM pg_database WHERE datname = $1"
	roleExists     = "SELECT count(*) FROM pg_roles WHERE rolname = $1"
)

// A database and a role made by a test, the database owned by the role, are
// named under the package's database and gone when the test ends.
func TestCreateDatabaseAndRoleAreDroppedWithTheTest(t *testing.T) {
	conn := admin(t)
	var db, role string
	var other *pgx.Conn
	t.Run("make", func(t *testing.T) {
		role = CreateRole(t, "extra_owner", "LOGIN")
		db = CreateDatabase(t, "extra", "OWNER "+pgx.Identifier{role}.Sanitize())
		owner := os.Getenv(OwnerVar)
		if db != owner+"_extra" || role != owner+"_extra_owner" {
			t.Errorf("made %q and %q, not under %q", db, role, owner)
		}
		if count(t, conn, databaseExists, db) != 1 || count(t, conn, roleExists, role) != 1 {
			t.Fatal("the database or the role was not made")
		}
		// A connection left open does not keep the database.
		var err error
		if other, err = pgx.Connect(context.Background(), DSN(t, db)); err != nil {
			t.Fatal(err)
		}
	})
	if other != nil {
		defer other.Close(context.Background())
	}
	if n := count(t, conn, databaseExists, db); n != 0 {
		t.Errorf("the database %s is still there", db)
	}
	if n := count(t, conn, roleExists, role); n != 0 {
		t.Errorf("the role %s is still there", role)
	}
}

// A crashed run's databases and roles, its own and those its tests made under
// its name, go with the next run once they are a day old, with the roles RolesOf
// names for them; a younger run's stay.
func TestDropLeftoversDropsAStaleRunsExtras(t *testing.T) {
	conn := admin(t)
	ctx := context.Background()
	old := prefix + strconv.FormatInt(time.Now().Add(-2*staleAfter).Unix(), 10) + "_00000000_crashed"
	young := prefix + strconv.FormatInt(time.Now().Unix(), 10) + "_00000000_running"
	dbs := []string{old, extraName(old, "extra"), young}
	roles := []string{extraName(old, "role"), "dbtest_stale_login", young + "_role"}
	for _, db := range dbs {
		if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range roles {
		if _, err := conn.Exec(ctx, "CREATE ROLE "+pgx.Identifier{r}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, db := range dbs {
			_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)")
		}
		for _, r := range roles {
			_, _ = conn.Exec(ctx, "DROP ROLE IF EXISTS "+pgx.Identifier{r}.Sanitize())
		}
	})
	saved := rolesOf
	rolesOf = func(db string) []string {
		if db == old {
			return []string{"dbtest_stale_login"}
		}
		return nil
	}
	t.Cleanup(func() { rolesOf = saved })

	dropLeftovers(ctx, conn)
	for _, db := range dbs[:2] {
		if count(t, conn, databaseExists, db) != 0 {
			t.Errorf("the stale database %s was not dropped", db)
		}
	}
	for _, r := range roles[:2] {
		if count(t, conn, roleExists, r) != 0 {
			t.Errorf("the stale role %s was not dropped", r)
		}
	}
	if count(t, conn, databaseExists, young) != 1 || count(t, conn, roleExists, roles[2]) != 1 {
		t.Error("a young run's database or role was dropped")
	}
}
