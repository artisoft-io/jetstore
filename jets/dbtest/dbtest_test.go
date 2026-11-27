package dbtest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// The package runs its own tests through Main, so the tests below that connect see the
// database Main made when JETS_TEST_DSN is set.
func TestMain(m *testing.M) { Main(m) }

func TestWithDatabase(t *testing.T) {
	for _, c := range []struct{ dsn, want string }{
		{"postgres://u:p@localhost:5432/postgres?sslmode=disable", "postgres://u:p@localhost:5432/jetstest_x?sslmode=disable"},
		{"postgresql://u@h/db", "postgresql://u@h/jetstest_x"},
		{"postgres://u@h:1", "postgres://u@h:1/jetstest_x"},
		{"host=h user=u dbname=db", "host=h user=u dbname=db dbname=jetstest_x"},
	} {
		got, err := withDatabase(c.dsn, "jetstest_x")
		if err != nil || got != c.want {
			t.Errorf("withDatabase(%q) = %q, %v; want %q", c.dsn, got, err, c.want)
		}
	}
}

func TestDatabaseName(t *testing.T) {
	now := time.Unix(1764201600, 0)
	a, b := databaseName(now), databaseName(now)
	if a == b {
		t.Errorf("two names made in the same second are both %q", a)
	}
	if len(a) > 63 {
		t.Errorf("%q is longer than 63 bytes", a)
	}
	m := leftover.FindStringSubmatch(a)
	if m == nil || m[1] != "1764201600" {
		t.Errorf("%q does not carry its creation time where dropLeftovers reads it", a)
	}
}

// currentDatabase is the database JETS_TEST_DSN connects to, or skips the test.
func currentDatabase(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv(DSNVar)
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set")
	}
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	var name string
	if err = conn.QueryRow(context.Background(), "SELECT current_database()").Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestTheTestsRunOnTheirOwnDatabase(t *testing.T) {
	name := currentDatabase(t)
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, "_dbtest") {
		t.Errorf("the tests run on database %q, not on one Main made for this package", name)
	}
}

// childVar makes TestChildDatabase print its database; it is set only for the
// child TestAChildRunsOnItsParentsDatabase starts.
const childVar = "JETS_DBTEST_CHILD"

func TestChildDatabase(t *testing.T) {
	if os.Getenv(childVar) == "" {
		t.Skip("run only as the child of TestAChildRunsOnItsParentsDatabase")
	}
	fmt.Println("database=" + currentDatabase(t))
}

// A test that starts its own test binary again hands the child its environment,
// and the child's Main must run it on the parent's database rather than make a
// new, empty one.
func TestAChildRunsOnItsParentsDatabase(t *testing.T) {
	parent := currentDatabase(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestChildDatabase$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), childVar+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "database="+parent+"\n") {
		t.Errorf("the child did not run on its parent's database %s:\n%s", parent, out)
	}
}
