package dblogin

// The code guard against a real PostgreSQL. Skipped unless JETS_TEST_DSN names
// a throwaway server whose user is a superuser, for example:
//
//	docker run -d --name cg-pg -e POSTGRES_PASSWORD=pw -p 5491:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5491/postgres go test ./dblogin -count=1
//
// Each test creates its own apiserver user and database: a superuser for the
// guard (the master user of an RDS or Aurora database may create event
// triggers), and a CREATEROLE user for the fallback.

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// cgDB creates the database name owned by the login role name with attrs, and
// returns a pool connected as that role, and one connected as the superuser
// of JETS_TEST_DSN to the same database.
func cgDB(t *testing.T, name, attrs string) (apiserver, super *pgxpool.Pool) {
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
				CREATE ROLE ` + name + ` LOGIN PASSWORD 'pw';
			END IF;
		END $$`,
		"ALTER ROLE " + name + " " + attrs,
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
	if super, err = pgxpool.New(ctx, u.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(super.Close)
	u.User = url.UserPassword(name, "pw")
	if apiserver, err = pgxpool.New(ctx, u.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(apiserver.Close)
	return apiserver, super
}

// cgSetup creates the apiserver's tables: a security table with one user, and
// a stand-in for aws_s3.query_export_to_s3 that, like the real one, runs the
// SQL text it is given as the caller.
func cgSetup(t *testing.T, apiserver *pgxpool.Pool) {
	t.Helper()
	cgExec(t, apiserver,
		"CREATE SCHEMA jetsapi",
		"CREATE TABLE jetsapi.users (user_email TEXT PRIMARY KEY, password TEXT, git_token TEXT, encrypted_roles TEXT)",
		"INSERT INTO jetsapi.users VALUES ('ke@test', 'hash', NULL, '{knowledge_engineer}')",
		"CREATE SCHEMA aws_s3",
		`CREATE FUNCTION aws_s3.query_export_to_s3(query text) RETURNS bigint LANGUAGE plpgsql AS $$
			BEGIN EXECUTE query; RETURN 1; END $$`,
	)
}

func cgSetupGrants(t *testing.T, apiserver *pgxpool.Pool) {
	t.Helper()
	cgExec(t, apiserver,
		"GRANT USAGE ON SCHEMA aws_s3 TO "+IDE,
		"GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA aws_s3 TO "+IDE)
}

func cgExec(t *testing.T, pool *pgxpool.Pool, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}
}

func rolesOf(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var roles string
	if err := pool.QueryRow(context.Background(), "SELECT encrypted_roles FROM jetsapi.users WHERE user_email = 'ke@test'").Scan(&roles); err != nil {
		t.Fatal(err)
	}
	return roles
}

// The statements the guard refuses, each on a table the IDE login owns.
var cgRefused = []struct{ name, sql string }{
	{"CREATE FUNCTION", "CREATE FUNCTION public.cg_f() RETURNS int LANGUAGE sql AS 'SELECT 1'"},
	{"CREATE OR REPLACE FUNCTION", "CREATE OR REPLACE FUNCTION public.cg_f() RETURNS int LANGUAGE sql AS 'SELECT 1'"},
	{"CREATE PROCEDURE", "CREATE PROCEDURE public.cg_p() LANGUAGE sql AS 'SELECT 1'"},
	{"CREATE FUNCTION in a DO block", `DO $$ BEGIN EXECUTE 'CREATE FUNCTION public.cg_g() RETURNS int LANGUAGE sql AS ''SELECT 1'''; END $$`},
	{"CREATE TRIGGER with a built-in function",
		"CREATE TRIGGER cg_t BEFORE UPDATE ON public.cg_own FOR EACH ROW EXECUTE FUNCTION suppress_redundant_updates_trigger()"},
	{"CREATE POLICY", "CREATE POLICY cg_pol ON public.cg_own USING (true)"},
	{"ENABLE ROW LEVEL SECURITY", "ALTER TABLE public.cg_own ENABLE ROW LEVEL SECURITY"},
	{"FORCE ROW LEVEL SECURITY", "ALTER TABLE public.cg_own FORCE ROW LEVEL SECURITY"},
	{"CREATE RULE", "CREATE RULE cg_r AS ON INSERT TO public.cg_own DO ALSO NOTHING"},
	{"CREATE OPERATOR", "CREATE OPERATOR public.=== (LEFTARG = int, RIGHTARG = int, FUNCTION = int4eq)"},
	{"CREATE AGGREGATE", "CREATE AGGREGATE public.cg_sum(int) (SFUNC = int4pl, STYPE = int)"},
	{"a default calling aws_s3", "ALTER TABLE public.cg_own ALTER COLUMN b SET DEFAULT aws_s3.query_export_to_s3('SELECT 1')::text"},
	{"a default calling query_to_xml", "CREATE TABLE public.cg_x1 (a text DEFAULT query_to_xml('SELECT password FROM jetsapi.users', true, false, '')::text)"},
	{"a default calling set_config", "CREATE TABLE public.cg_x2 (a text DEFAULT set_config('search_path', 'public', false))"},
	{"a check calling pg_terminate_backend", "CREATE TABLE public.cg_x3 (a int CHECK (pg_terminate_backend(a)))"},
	{"a view calling table_to_xml", "CREATE VIEW public.cg_v1 AS SELECT table_to_xml('jetsapi.users', true, false, '') AS x"},
	{"a view calling aws_s3", "CREATE VIEW public.cg_v2 AS SELECT aws_s3.query_export_to_s3('SELECT 1') AS n"},
	{"a domain check calling aws_s3", "CREATE DOMAIN public.cg_d AS text CHECK (aws_s3.query_export_to_s3(VALUE) = 1)"},
}

// The IDE login's ordinary work, which the guard leaves alone.
var cgAllowed = []string{
	"CREATE TABLE public.cg_ok (a serial, b text DEFAULT now()::text, c uuid DEFAULT gen_random_uuid(), d int CHECK (d > 0), e text GENERATED ALWAYS AS (lower(b)) STORED)",
	"INSERT INTO public.cg_ok (b, d) VALUES ('X', 1)",
	"CREATE INDEX cg_ok_lower ON public.cg_ok (lower(b)) WHERE d > 0",
	"ALTER TABLE public.cg_ok ADD COLUMN f int DEFAULT 1",
	"CREATE VIEW public.cg_ok_v AS SELECT a, upper(b) AS b, count(*) OVER () AS n FROM public.cg_ok",
	"CREATE TABLE public.cg_ok_copy AS SELECT * FROM public.cg_ok",
	"CREATE TEMP TABLE cg_tmp (a int)",
	"SELECT aws_s3.query_export_to_s3('SELECT 1')",
	"TRUNCATE public.cg_ok",
	"DROP TABLE public.cg_ok_copy",
	`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'cg_ok_b') THEN
		CREATE INDEX cg_ok_b ON public.cg_ok (b); END IF; END $$`,
}

// The escalation the guard exists for: code on the IDE login's own table that
// changes the security tables when the apiserver's user reads or writes it.
var cgEscalation = []string{
	`CREATE TABLE public.cg_bait (a int)`,
	`CREATE FUNCTION public.cg_promote() RETURNS boolean LANGUAGE plpgsql AS $$
		BEGIN UPDATE jetsapi.users SET encrypted_roles = '{admin}' WHERE user_email = 'ke@test'; RETURN true;
		EXCEPTION WHEN insufficient_privilege THEN RETURN true; END $$`,
	`CREATE POLICY cg_bait_read ON public.cg_bait USING (public.cg_promote())`,
	`ALTER TABLE public.cg_bait ENABLE ROW LEVEL SECURITY`,
	`ALTER TABLE public.cg_bait FORCE ROW LEVEL SECURITY`,
	`CREATE FUNCTION public.cg_promote_t() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN PERFORM public.cg_promote(); RETURN NEW; END $$`,
	`CREATE TRIGGER cg_bait_ins BEFORE INSERT ON public.cg_bait FOR EACH ROW EXECUTE FUNCTION public.cg_promote_t()`,
	// A row, so the policy has something to filter. As the IDE login, the
	// trigger's UPDATE is refused and the function swallows the refusal.
	`INSERT INTO public.cg_bait VALUES (0)`,
}

func isRefusal(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" && strings.Contains(pgErr.Message, "not allowed")
}

func TestCodeGuardRefusesTheIDELoginsCode(t *testing.T) {
	apiserver, _ := cgDB(t, "cg_guard", "SUPERUSER")
	ctx := context.Background()
	cgSetup(t, apiserver)
	st, err := EnsureStatus(ctx, apiserver)
	if err != nil {
		t.Fatalf("EnsureStatus: %v", err)
	}
	if !st.CodeGuard || st.CodeGuardProblem != "" || len(st.IDECode) > 0 {
		t.Fatalf("status = %+v; want the guard on, no problem, no code", st)
	}
	if on, err := CodeGuard(ctx, apiserver); err != nil || !on {
		t.Fatalf("CodeGuard = %v, %v; want true", on, err)
	}
	cgSetupGrants(t, apiserver)
	ide, err := Open(ctx, apiserver, IDE, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer ide.Close()
	cgExec(t, ide, "CREATE TABLE public.cg_own (a int, b text)")

	for _, c := range cgRefused {
		if _, err := ide.Exec(ctx, c.sql); !isRefusal(err) {
			t.Errorf("%s as the IDE login: got %v, want refused by the guard", c.name, err)
		}
	}
	for _, s := range cgAllowed {
		if _, err := ide.Exec(ctx, s); err != nil {
			t.Errorf("as the IDE login: %v\n%s", err, s)
		}
	}

	t.Run("the escalation is refused at its first step", func(t *testing.T) {
		for _, s := range cgEscalation {
			_, _ = ide.Exec(ctx, s)
		}
		// Whatever was created, the apiserver now reads and writes the table.
		_, _ = apiserver.Exec(ctx, "SELECT * FROM public.cg_bait")
		_, _ = apiserver.Exec(ctx, "INSERT INTO public.cg_bait VALUES (1)")
		if got := rolesOf(t, apiserver); got != "{knowledge_engineer}" {
			t.Fatalf("the user's roles are %s after the apiserver read and wrote the IDE login's table", got)
		}
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil {
			t.Fatal(err)
		}
		if len(st.IDECode) > 0 {
			t.Fatalf("the IDE login owns code after the guard: %v", st.IDECode)
		}
	})

	t.Run("the apiserver's user may still create code", func(t *testing.T) {
		cgExec(t, apiserver,
			"CREATE TABLE jetsapi.cg_api (a int, b text DEFAULT aws_s3.query_export_to_s3('SELECT 1')::text)",
			"CREATE FUNCTION jetsapi.cg_api_t() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$",
			"CREATE TRIGGER cg_api_t BEFORE INSERT ON jetsapi.cg_api FOR EACH ROW EXECUTE FUNCTION jetsapi.cg_api_t()",
			"CREATE POLICY cg_api_p ON jetsapi.cg_api USING (true)",
			"ALTER TABLE jetsapi.cg_api ENABLE ROW LEVEL SECURITY",
			"CREATE RULE cg_api_r AS ON UPDATE TO jetsapi.cg_api DO ALSO NOTHING",
			"CREATE VIEW jetsapi.cg_api_v AS SELECT query_to_xml('SELECT 1', true, false, '') AS x",
			"INSERT INTO jetsapi.cg_api (a) VALUES (1)",
		)
	})

	t.Run("Ensure is idempotent and keeps the guard", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			st, err := EnsureStatus(ctx, apiserver)
			if err != nil || !st.CodeGuard || st.CodeGuardProblem != "" {
				t.Fatalf("pass %d: %+v, %v", i+1, st, err)
			}
		}
	})

	t.Run("the switch turns the guard off, and any other value back on", func(t *testing.T) {
		t.Setenv(CodeGuardSwitch, " OFF ")
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil || st.CodeGuard || !st.SwitchedOff || st.Removal != "dropped" || st.CodeGuardProblem != "" {
			t.Fatalf("with %s=off: %+v, %v; want the trigger dropped", CodeGuardSwitch, st, err)
		}
		if on, err := CodeGuard(ctx, apiserver); err != nil || on {
			t.Fatalf("CodeGuard with the switch off = %v, %v", on, err)
		}
		cgExec(t, ide, "CREATE FUNCTION public.cg_switched() RETURNS int LANGUAGE sql AS 'SELECT 1'")
		cgExec(t, apiserver, "DROP FUNCTION public.cg_switched()")
		// Again, with nothing left to remove.
		if st, err = EnsureStatus(ctx, apiserver); err != nil || st.CodeGuard || st.Removal != "" {
			t.Fatalf("with %s=off a second time: %+v, %v", CodeGuardSwitch, st, err)
		}

		t.Setenv(CodeGuardSwitch, "yes")
		if st, err = EnsureStatus(ctx, apiserver); err != nil || !st.CodeGuard || st.SwitchedOff {
			t.Fatalf("with %s=yes: %+v, %v; want the guard back on", CodeGuardSwitch, st, err)
		}
		if _, err := ide.Exec(ctx, "CREATE FUNCTION public.cg_switched() RETURNS int LANGUAGE sql AS 'SELECT 1'"); !isRefusal(err) {
			t.Fatalf("CREATE FUNCTION as the IDE login with the guard back on: %v", err)
		}
	})
}

func TestCodeGuardFallsBackWhenItCannotBeInstalled(t *testing.T) {
	apiserver, super := cgDB(t, "cg_plain", "NOSUPERUSER CREATEROLE")
	ctx := context.Background()
	cgSetup(t, apiserver)
	st, err := EnsureStatus(ctx, apiserver)
	if err != nil {
		t.Fatalf("EnsureStatus must not fail without the guard: %v", err)
	}
	if st.CodeGuard || !strings.Contains(st.CodeGuardProblem, "event trigger") {
		t.Fatalf("status = %+v; want the guard off, saying why", st)
	}
	if on, err := CodeGuard(ctx, apiserver); err != nil || on {
		t.Fatalf("CodeGuard = %v, %v; want false", on, err)
	}
	cgSetupGrants(t, apiserver)
	ide, err := Open(ctx, apiserver, IDE, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer ide.Close()

	// Without the guard the escalation works: this is the risk the warning names.
	cgExec(t, ide, cgEscalation...)
	if got := rolesOf(t, apiserver); got != "{knowledge_engineer}" {
		t.Fatalf("the IDE login changed the user's roles itself: %s", got)
	}
	cgExec(t, apiserver, "SELECT count(*) FROM public.cg_bait")
	if got := rolesOf(t, apiserver); got != "{admin}" {
		t.Fatalf("without the guard the policy should have run as the apiserver; roles = %s", got)
	}
	cgExec(t, apiserver, "UPDATE jetsapi.users SET encrypted_roles = '{knowledge_engineer}'")
	cgExec(t, apiserver, "ALTER TABLE public.cg_bait NO FORCE ROW LEVEL SECURITY", "INSERT INTO public.cg_bait VALUES (1)")
	if got := rolesOf(t, apiserver); got != "{admin}" {
		t.Fatalf("without the guard the trigger should have run as the apiserver; roles = %s", got)
	}
	cgExec(t, apiserver, "UPDATE jetsapi.users SET encrypted_roles = '{knowledge_engineer}'",
		"ALTER TABLE public.cg_bait FORCE ROW LEVEL SECURITY")

	t.Run("Ensure reports the code the IDE login owns, and drops nothing", func(t *testing.T) {
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(st.IDECode, "\n")
		for _, want := range []string{
			"function cg_promote()", "function cg_promote_t()",
			"policy cg_bait_read on cg_bait", "row-level security on cg_bait", "trigger cg_bait_ins on cg_bait",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("IDECode lacks %q:\n%s", want, got)
			}
		}
		var n int
		if err := apiserver.QueryRow(ctx, "SELECT count(*) FROM pg_proc WHERE proname IN ('cg_promote', 'cg_promote_t')").Scan(&n); err != nil || n != 2 {
			t.Fatalf("the IDE login's functions: %d, %v; want both kept", n, err)
		}
	})

	t.Run("a superuser installs the trigger, and Ensure keeps it", func(t *testing.T) {
		// What the Developer's Guide asks a superuser to run: the function is
		// already there, created by Ensure.
		cgExec(t, super,
			"CREATE EVENT TRIGGER "+CodeGuardTrigger+" ON ddl_command_end EXECUTE FUNCTION "+CodeGuardFunction+"()",
			"ALTER FUNCTION "+CodeGuardFunction+"() OWNER TO CURRENT_USER")
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil {
			t.Fatal(err)
		}
		if !st.CodeGuard || st.CodeGuardProblem != "" {
			t.Fatalf("status = %+v; want the guard on with no problem", st)
		}
		if _, err := ide.Exec(ctx, "CREATE FUNCTION public.cg_after() RETURNS int LANGUAGE sql AS 'SELECT 1'"); !isRefusal(err) {
			t.Fatalf("CREATE FUNCTION as the IDE login with the guard on: %v", err)
		}
		cgExec(t, apiserver, "CREATE FUNCTION public.cg_api_after() RETURNS int LANGUAGE sql AS 'SELECT 1'")
	})

	t.Run("an outdated guard the apiserver may not replace is reported", func(t *testing.T) {
		cgExec(t, super, "COMMENT ON FUNCTION "+CodeGuardFunction+"() IS 'jetstore code guard 000000000000'")
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil {
			t.Fatal(err)
		}
		if !st.CodeGuard || !strings.Contains(st.CodeGuardProblem, "not this release") {
			t.Fatalf("status = %+v; want the guard on and reported outdated", st)
		}
	})

	t.Run("the switch cannot remove a superuser's guard, and says so", func(t *testing.T) {
		t.Setenv(CodeGuardSwitch, "off")
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil {
			t.Fatalf("EnsureStatus must not fail: %v", err)
		}
		if !st.CodeGuard || !st.SwitchedOff || !strings.Contains(st.CodeGuardProblem, "a superuser must run DROP EVENT TRIGGER") {
			t.Fatalf("status = %+v; want the guard still on, saying a superuser must drop it", st)
		}
	})

	t.Run("the switch empties the function when it may not touch the trigger", func(t *testing.T) {
		cgExec(t, super, "ALTER FUNCTION "+CodeGuardFunction+"() OWNER TO cg_plain")
		t.Setenv(CodeGuardSwitch, "off")
		st, err := EnsureStatus(ctx, apiserver)
		if err != nil || st.CodeGuard || st.Removal != "emptied its function" {
			t.Fatalf("status = %+v, %v; want the guard off by an emptied function", st, err)
		}
		if on, err := CodeGuard(ctx, apiserver); err != nil || on {
			t.Fatalf("CodeGuard with an emptied function = %v, %v", on, err)
		}
		cgExec(t, ide, "CREATE FUNCTION public.cg_while_off() RETURNS int LANGUAGE sql AS 'SELECT 1'")

		t.Setenv(CodeGuardSwitch, "")
		if st, err = EnsureStatus(ctx, apiserver); err != nil || !st.CodeGuard || st.CodeGuardProblem != "" {
			t.Fatalf("with the switch unset: %+v, %v; want the guard restored", st, err)
		}
		if _, err := ide.Exec(ctx, "CREATE FUNCTION public.cg_after_on() RETURNS int LANGUAGE sql AS 'SELECT 1'"); !isRefusal(err) {
			t.Fatalf("CREATE FUNCTION as the IDE login with the guard restored: %v", err)
		}
	})
}
