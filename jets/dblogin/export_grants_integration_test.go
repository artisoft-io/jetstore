package dblogin

// The export functions' grants against a real PostgreSQL, on the layout RDS
// and Aurora give aws_s3: an extension registered in public whose functions
// live in a schema of its own. Skipped unless JETS_TEST_DSN names a throwaway
// server whose user is a superuser; see the header of
// codeguard_integration_test.go.
//
// A stock PostgreSQL has no aws_s3, so pgcrypto stands in for it: created in
// public, then given a schema and a function the way aws_s3's script creates
// its own.

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeExport makes pgcrypto, in public, the owner of schema fake_s3 and its
// function query_export_to_s3, which PUBLIC may not call, and makes it the
// only extension IDE may call for the rest of the test. pool creates them, so
// their owner is pool's user.
func fakeExport(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	saved := ideExtensions
	ideExtensions = []string{"pgcrypto"}
	t.Cleanup(func() { ideExtensions = saved })
	cgExec(t, pool,
		"CREATE EXTENSION IF NOT EXISTS pgcrypto SCHEMA public",
		"CREATE SCHEMA fake_s3",
		`CREATE FUNCTION fake_s3.query_export_to_s3(query text) RETURNS bigint LANGUAGE plpgsql AS $$
			BEGIN EXECUTE query; RETURN 1; END $$`,
		"REVOKE EXECUTE ON FUNCTION fake_s3.query_export_to_s3(text) FROM PUBLIC",
		"ALTER EXTENSION pgcrypto ADD SCHEMA fake_s3",
		"ALTER EXTENSION pgcrypto ADD FUNCTION fake_s3.query_export_to_s3(text)",
	)
	var ns string
	if err := pool.QueryRow(context.Background(),
		"SELECT extnamespace::regnamespace::text FROM pg_extension WHERE extname = 'pgcrypto'").Scan(&ns); err != nil {
		t.Fatal(err)
	}
	if ns != "public" {
		t.Fatalf("pgcrypto is registered in %s; the test needs public, as aws_s3 is", ns)
	}
}

// ideMayExport calls the export function as the IDE login.
func ideMayExport(t *testing.T, apiserver *pgxpool.Pool) {
	t.Helper()
	ide, err := Open(context.Background(), apiserver, IDE, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ide.Close()
	cgExec(t, ide, "SELECT fake_s3.query_export_to_s3('SELECT 1')")
}

// The apiserver's user owns the extension's schema. Granting on the schema
// the extension is registered in, public, left IDE without the function: the
// report's export failed with "permission denied".
func TestExportGrantsReachTheFunctionsSchema(t *testing.T) {
	apiserver, _ := cgDB(t, "dbl_export_owner", "SUPERUSER")
	ctx := context.Background()
	cgSetup(t, apiserver)
	fakeExport(t, apiserver)

	st, err := EnsureStatus(ctx, apiserver)
	if err != nil {
		t.Fatalf("EnsureStatus: %v", err)
	}
	if st.ExportProblem != "" {
		t.Fatalf("ExportProblem = %q; want none", st.ExportProblem)
	}
	ideMayExport(t, apiserver)

	// Only the extension's own functions: none other in public is granted.
	cgExec(t, apiserver, "CREATE FUNCTION public.not_exported() RETURNS int LANGUAGE sql AS 'SELECT 1'",
		"REVOKE EXECUTE ON FUNCTION public.not_exported() FROM PUBLIC")
	if _, err := EnsureStatus(ctx, apiserver); err != nil {
		t.Fatal(err)
	}
	var may bool
	if err := apiserver.QueryRow(ctx, "SELECT has_function_privilege($1, 'public.not_exported()', 'EXECUTE')", IDE).Scan(&may); err != nil {
		t.Fatal(err)
	}
	if may {
		t.Errorf("%s may call public.not_exported(), which is no export function", IDE)
	}
}

// Another user owns the extension's schema and gave the apiserver's user its
// privileges WITH GRANT OPTION, as RDS gives its master user aws_s3: the
// apiserver's grants work.
func TestExportGrantsWithGrantOption(t *testing.T) {
	apiserver, super := cgDB(t, "dbl_export_option", "NOSUPERUSER CREATEROLE")
	ctx := context.Background()
	cgSetup(t, apiserver)
	fakeExport(t, super)
	cgExec(t, super,
		"GRANT USAGE ON SCHEMA fake_s3 TO dbl_export_option WITH GRANT OPTION",
		"GRANT EXECUTE ON FUNCTION fake_s3.query_export_to_s3(text) TO dbl_export_option WITH GRANT OPTION")

	st, err := EnsureStatus(ctx, apiserver)
	if err != nil {
		t.Fatalf("EnsureStatus: %v", err)
	}
	if st.ExportProblem != "" {
		t.Fatalf("ExportProblem = %q; want none", st.ExportProblem)
	}
	ideMayExport(t, apiserver)
}

// Another user owns the extension's schema, and the apiserver's user may use
// it without the right to pass that on. PostgreSQL then warns and grants
// nothing; Ensure says so, and the statements it gives are the whole fix.
func TestExportGrantsThatGiveNothingAreReported(t *testing.T) {
	apiserver, super := cgDB(t, "dbl_export_other", "NOSUPERUSER CREATEROLE")
	ctx := context.Background()
	cgSetup(t, apiserver)
	fakeExport(t, super)
	var owner string
	if err := super.QueryRow(ctx, "SELECT current_user").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	cgExec(t, super,
		"GRANT USAGE ON SCHEMA fake_s3 TO dbl_export_other",
		"GRANT EXECUTE ON FUNCTION fake_s3.query_export_to_s3(text) TO dbl_export_other")

	st, err := EnsureStatus(ctx, apiserver)
	if err != nil {
		t.Fatalf("EnsureStatus: %v", err)
	}
	for _, want := range []string{
		"schema fake_s3", "dbl_export_other neither owns", "as " + owner + " (the owner)",
		`GRANT USAGE ON SCHEMA "fake_s3" TO "jets_ide"`,
		`GRANT EXECUTE ON FUNCTION fake_s3.query_export_to_s3(text) TO "jets_ide"`,
	} {
		if !strings.Contains(st.ExportProblem, want) {
			t.Errorf("ExportProblem lacks %q:\n%s", want, st.ExportProblem)
		}
	}

	_, fix, _ := strings.Cut(st.ExportProblem, "(the owner): ")
	cgExec(t, super, strings.Split(fix, "; ")...)
	if st, err = EnsureStatus(ctx, apiserver); err != nil {
		t.Fatal(err)
	}
	if st.ExportProblem != "" {
		t.Fatalf("after the owner's grants, ExportProblem = %q; want none", st.ExportProblem)
	}
	ideMayExport(t, apiserver)
}
