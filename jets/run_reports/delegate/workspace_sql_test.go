package delegate

// Which login a report's SQL runs on (workspace_sql.go).
//
// The unit tests need nothing. TestReportSQLRunsAsIDELogin needs a scratch
// PostgreSQL named by JETS_TEST_DSN, as a superuser: it creates a login
// (rrl_apiserver) and a database it owns (rrl_run_reports), as a deployment's
// apiserver user is, and runs everything else as that login. The login holds
// CREATEROLE and is not a superuser, or is one with
// JETS_TEST_APISERVER_SUPERUSER=1, which turns the IDE code guard on (package
// jets/dblogin/dblogintest). The test passes either way, and whichever test
// created the cluster-wide restricted roles first. Skipped when JETS_TEST_DSN
// is not set. Locally:
//
//	docker run -d --rm --name rrl-pg -e POSTGRES_PASSWORD=pw -p 5491:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5491/postgres \
//	  go test -count=1 -run IDELogin ./jets/run_reports/delegate/

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/artisoft-io/jetstore/jets/dblogin/dblogintest"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWorkspaceSQLRefusesWithoutTheIDELogin(t *testing.T) {
	ca := &CommandArguments{}
	if _, err := ca.workspaceSQL(); !errors.Is(err, errNoWorkspaceSQLPool) {
		t.Fatalf("workspaceSQL with no SQLPool: got %v, want errNoWorkspaceSQLPool", err)
	}
	pool := &pgxpool.Pool{}
	ca.SQLPool = pool
	if got, err := ca.workspaceSQL(); err != nil || got != pool {
		t.Fatalf("workspaceSQL: got %p, %v; want SQLPool", got, err)
	}
}

// RunReports must refuse before any statement when the IDE login is not open,
// rather than fall back to the pool it was given, which is the apiserver's.
func TestRunReportsRefusesWithoutTheIDELogin(t *testing.T) {
	ca := &CommandArguments{
		CurrentReportDirectives: &ReportDirectives{ReportScripts: []string{"r.sql"}},
		ReportScriptPaths:       []string{"r.sql"},
	}
	// dbpool is nil: reaching a statement would panic, and RunReports recovers
	// a panic only into a log line, so the error is what says it stopped first.
	if err := ca.RunReports(nil); !errors.Is(err, errNoWorkspaceSQLPool) {
		t.Fatalf("RunReports with no SQLPool: got %v, want errNoWorkspaceSQLPool", err)
	}
}

func TestWithIDEHint(t *testing.T) {
	denied := &pgconn.PgError{Code: "42501", Message: "permission denied for table users"}
	got := withIDEHint(denied)
	if !errors.Is(got, denied) || !strings.Contains(got.Error(), dblogin.IDE) {
		t.Fatalf("withIDEHint(42501) = %v; want the error, wrapped, naming %s", got, dblogin.IDE)
	}
	other := &pgconn.PgError{Code: "42P01", Message: "relation does not exist"}
	if got := withIDEHint(other); got != error(other) {
		t.Fatalf("withIDEHint(42P01) = %v; want it unchanged", got)
	}
}

// rrlDB is a database owned by the apiserver's user, and a pool connected as
// that login: one that holds CREATEROLE and is not a superuser, as a
// deployment's is, or a superuser (see jets/dblogin/dblogintest).
func rrlDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return dblogintest.Apiserver(t, "rrl_apiserver", "rrl_run_reports")
}

// writeScript puts sql in the workspace's reports/ directory, where
// confineReportPath looks, and returns its path.
func writeScript(t *testing.T, name, sql string) string {
	t.Helper()
	path := filepath.Join(workspaceHome, wprefix, "reports", name)
	if err := os.WriteFile(path, []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReportSQLRunsAsIDELogin(t *testing.T) {
	db := rrlDB(t)
	ctx := context.Background()

	savedHome, savedPrefix := workspaceHome, wprefix
	t.Cleanup(func() { workspaceHome, wprefix = savedHome, savedPrefix })
	workspaceHome, wprefix = t.TempDir(), "rrl_ws"
	if err := os.MkdirAll(filepath.Join(workspaceHome, wprefix, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}

	// The apiserver's tables: a security table, and a report table an earlier
	// release's run_reports created as the apiserver's user.
	for _, s := range []string{
		"CREATE SCHEMA jetsapi",
		"CREATE TABLE jetsapi.users (user_email TEXT PRIMARY KEY, name TEXT, password TEXT, git_token TEXT, encrypted_roles TEXT[])",
		"INSERT INTO jetsapi.users VALUES ('admin@example.com', 'Admin', 'secret', NULL, '{admin}')",
		"CREATE TABLE jetsapi.session_registry (session_id TEXT, source_period_key INTEGER)",
		"INSERT INTO jetsapi.session_registry VALUES ('s1', 1)",
		"CREATE TABLE public.rrl_thresholds (report TEXT NOT NULL, value DOUBLE PRECISION)",
		"CREATE TABLE public.rrl_defaults (report TEXT NOT NULL, value DOUBLE PRECISION)",
	} {
		if _, err := db.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}

	// No dblogin.Ensure has run on this database: OpenWorkspaceSQLPool sets the
	// logins up, as on a database neither the apiserver nor update_db reached.
	sqlPool, err := OpenWorkspaceSQLPool(ctx, db)
	if err != nil {
		t.Fatalf("OpenWorkspaceSQLPool: %v", err)
	}
	t.Cleanup(sqlPool.Close)
	ca := &CommandArguments{SQLPool: sqlPool, SessionId: "s1"}
	login, err := dblogin.LoginName(ctx, db, dblogin.IDE)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a script runs as the IDE login", func(t *testing.T) {
		var who string
		if err := sqlPool.QueryRow(ctx, "SELECT session_user").Scan(&who); err != nil {
			t.Fatal(err)
		}
		if who != login {
			t.Fatalf("session_user = %q, want %q", who, login)
		}
	})

	t.Run("a script cannot write the security tables", func(t *testing.T) {
		path := writeScript(t, "promote.sql", "UPDATE jetsapi.users SET encrypted_roles = '{admin}' WHERE user_email = 'admin@example.com'")
		err := ca.runSqlScriptDelegate(sqlPool, path)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" || !strings.Contains(err.Error(), dblogin.IDE) {
			t.Fatalf("UPDATE jetsapi.users as the IDE login: got %v, want permission denied with the hint", err)
		}
	})

	t.Run("a report cannot read a secret column", func(t *testing.T) {
		// Read the way DoCsvReport and DoParquetReport do, whose upload needs
		// S3: the refusal arrives from rows.Err, not from Query.
		rows, err := sqlPool.Query(ctx, "SELECT password FROM jetsapi.users")
		if err == nil {
			for rows.Next() {
			}
			err = rows.Err()
			rows.Close()
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("SELECT password as the IDE login: got %v, want permission denied", err)
		}
	})

	t.Run("a script reads jetsapi and uses temp tables in its own transaction", func(t *testing.T) {
		path := writeScript(t, "temp.sql", `BEGIN;
CREATE TEMP TABLE tmp_keys ON COMMIT DROP AS
  SELECT source_period_key FROM jetsapi.session_registry WHERE session_id = '$SESSIONID';
INSERT INTO public.rrl_defaults (report, value) SELECT 'r', source_period_key FROM tmp_keys;
COMMIT;`)
		if err := ca.runSqlScriptDelegate(sqlPool, path); err != nil {
			t.Fatalf("temp-table script: %v", err)
		}
	})

	t.Run("a script may truncate and refill a table it did not create", func(t *testing.T) {
		path := writeScript(t, "defaults.sql", `BEGIN;
CREATE TABLE IF NOT EXISTS public.rrl_defaults (report TEXT NOT NULL, value DOUBLE PRECISION);
TRUNCATE public.rrl_defaults;
INSERT INTO public.rrl_defaults VALUES ('r', 1);
COMMIT;`)
		if err := ca.runSqlScriptDelegate(sqlPool, path); err != nil {
			t.Fatalf("truncate-and-refill script: %v", err)
		}
	})

	// The upgrade: a report that indexes a table the apiserver's user created
	// fails until the table is the IDE login's, and the IDE login cannot make
	// it so itself, so the transfer cannot sit in a script that runs as it
	// (update_db runs base__workspace_init_db.sql on this login).
	indexScript := `BEGIN;
CREATE TABLE IF NOT EXISTS public.rrl_thresholds (report TEXT NOT NULL, value DOUBLE PRECISION);
CREATE INDEX IF NOT EXISTS rrl_thresholds_idx ON public.rrl_thresholds (report);
COMMIT;`
	t.Run("indexing a table the apiserver created is refused", func(t *testing.T) {
		path := writeScript(t, "index.sql", indexScript)
		err := ca.runSqlScriptDelegate(sqlPool, path)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("CREATE INDEX IF NOT EXISTS on the apiserver's table: got %v, want must be owner", err)
		}
	})
	t.Run("the IDE login cannot take the table over", func(t *testing.T) {
		_, err := sqlPool.Exec(ctx, "ALTER TABLE public.rrl_thresholds OWNER TO "+login)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Fatalf("ALTER TABLE OWNER TO the IDE login, as it: got %v, want must be owner", err)
		}
	})
	t.Run("after the apiserver transfers it, the report runs", func(t *testing.T) {
		if _, err := db.Exec(ctx, "ALTER TABLE public.rrl_thresholds OWNER TO "+login); err != nil {
			t.Fatalf("ALTER TABLE OWNER TO the IDE login, as the apiserver: %v", err)
		}
		path := writeScript(t, "index.sql", indexScript)
		if err := ca.runSqlScriptDelegate(sqlPool, path); err != nil {
			t.Fatalf("index script after the transfer: %v", err)
		}
		// And the apiserver still reads and writes it, by membership.
		if _, err := db.Exec(ctx, "INSERT INTO public.rrl_thresholds VALUES ('r', 2)"); err != nil {
			t.Fatalf("apiserver insert after the transfer: %v", err)
		}
	})
}
