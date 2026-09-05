// The startup order, run against a real Postgres.
//
// **This can only be tested against a database whose schema predates the release,
// which is the whole reason the defect shipped.** checkJetStoreSchema creates the
// whole jetsapi schema when jetstore_release is absent, so a fresh database is
// current before checkWorkspaceVersion ever runs and never reproduces anything. The
// failing case is the other one: an existing database, an image carrying a newer
// jets_schema.json, and a compile in between. So these tests install the *previous*
// workspace_version, record the *previous* release, and then run the sequence.
//
// The precedents beside this are `jets/schema/migrate_workspace_version_test.go`,
// which exercises the migration and its rollback, and
// `jets/schema/create_table_deleted_test.go`, whose header carries the docker line.
// Neither can see this: both arrange the schema and then ask about the schema,
// where what failed on 2026-09-05 was a *caller* reaching a table between two steps.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test -count=1 ./jets/apiserver/
package main

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The release the deployment was on, and the one the image carries. JetStore
// compares these as strings (`checkDomainTablesVersion`, above), which is what the
// timestamp format makes safe.
const (
	deployedRelease = "1788190686"
	imageRelease    = "1788587129"
)

// startupOrderDB is a database of this suite's own. It is dropped and recreated
// rather than cleaned, because what is being arranged is the *absence* of columns
// and a leftover from a previous run is indistinguishable from a passing fix.
func startupOrderDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		admin.Close()
		t.Fatalf("dropping %s: %v", name, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("creating %s: %v", name, err)
	}
	admin.Close()
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// installPreviousRelease arranges the database the apiserver met on 2026-09-05: a
// jetstore_release naming the deployed version, and the workspace_version table as
// it shipped until d8bf4231 -- a key, a version, and a unique constraint on the
// version alone.
//
// It installs the two tables rather than the whole schema on purpose. The point of
// arrangement is what workspace_version does *not* have; adding the other 41 tables
// would make the fixture slower and no more faithful.
func installPreviousRelease(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatalf("creating the jetsapi schema: %v", err)
	}
	release := schema.TableDefinition{
		SchemaName: "jetsapi",
		TableName:  "jetstore_release",
		Columns: []schema.ColumnDefinition{
			{ColumnName: "version", DataType: "text", IsPK: true, IsNotNull: true},
			{ColumnName: "name", DataType: "text", Default: "'JetStore Timestamp Version'", IsNotNull: true},
		},
	}
	if err := release.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("installing jetstore_release: %v", err)
	}
	before := schema.TableDefinition{
		SchemaName: "jetsapi",
		TableName:  "workspace_version",
		Columns: []schema.ColumnDefinition{
			{ColumnName: "key", DataType: "int", IsPK: true},
			{ColumnName: "version", DataType: "text", IsNotNull: true},
		},
		TableConstraints: []schema.ConstraintDefinition{{
			Name:       "workspace_version_unique_cstraint",
			Definition: "CONSTRAINT workspace_version_unique_cstraint UNIQUE (version)",
		}},
	}
	if err := before.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("installing the pre-d8bf4231 workspace_version: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO jetsapi.jetstore_release (version) VALUES ($1)", deployedRelease); err != nil {
		t.Fatalf("recording the deployed release: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO jetsapi.workspace_version (version) VALUES ($1)", deployedRelease); err != nil {
		t.Fatalf("seeding workspace_version: %v", err)
	}
}

// hasColumn asks the catalogue rather than the schema file, so a test that passes
// says the database changed and not that the definition did.
func hasColumn(t *testing.T, pool *pgxpool.Pool, table, column string) bool {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_schema = 'jetsapi' AND table_name = $1 AND column_name = $2`,
		table, column).Scan(&n)
	if err != nil {
		t.Fatalf("reading the columns of %s: %v", table, err)
	}
	return n > 0
}

// useImageRelease points the process at the image's schema file and release, which
// is what RunUpdateDb's child inherited already: it passes os.Environ() through and
// sets no cmd.Dir, so running the migration in process reads the same file
// (`RunUpdateDb`, `jets/datatable/workspace_helper_functions.go:198`).
func useImageRelease(t *testing.T) {
	t.Helper()
	t.Setenv("JETS_VERSION", imageRelease)
	t.Setenv("JETS_SCHEMA_FILE", "../jets_schema.json")
}

// TestTheCompileFailsAgainstThePreviousReleaseSchema is the defect, reproduced.
//
// It asserts the 2026-09-05 failure rather than merely the absence of a column,
// because the two are not the same claim: the column had been absent on that
// database for as long as it existed, and what was new was a writer for it.
func TestTheCompileFailsAgainstThePreviousReleaseSchema(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	useImageRelease(t)
	pool := startupOrderDB(t, "startup_order_unfixed")
	installPreviousRelease(t, pool)

	err := workspace.UpdateWorkspaceVersionDb(pool, "jets_ws", imageRelease)
	if err == nil {
		t.Fatal("expected the compile's version insert to fail against the previous release's schema")
	}
	// SQLSTATE 42703 is undefined_column, which is what the deployment logged.
	if !strings.Contains(err.Error(), "42703") || !strings.Contains(err.Error(), "workspace_name") {
		t.Fatalf("expected an undefined_column error naming workspace_name, got: %v", err)
	}
}

// TestTheSystemTablesAreMigratedBeforeTheWorkspaceCompile is the fix.
//
// Same arrangement, same call, with the new step in between and nothing else
// changed. checkWorkspaceVersion is not called directly because it stashes files,
// syncs from S3-backed blobs and shells out to a compiler; UpdateWorkspaceVersionDb
// is the statement inside it that failed, and it is the whole of what the ordering
// has to make possible.
func TestTheSystemTablesAreMigratedBeforeTheWorkspaceCompile(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	useImageRelease(t)
	pool := startupOrderDB(t, "startup_order_fixed")
	installPreviousRelease(t, pool)

	if hasColumn(t, pool, "workspace_version", "workspace_name") {
		t.Fatal("the arrangement is wrong: workspace_name exists before the migration")
	}

	s := &Server{dbpool: pool}
	if err := s.checkSystemTablesVersion(); err != nil {
		t.Fatalf("checkSystemTablesVersion: %v", err)
	}

	for _, col := range []string{"workspace_name", "workspace_commit"} {
		if !hasColumn(t, pool, "workspace_version", col) {
			t.Errorf("workspace_version.%s was not added by the migration", col)
		}
	}
	if err := workspace.UpdateWorkspaceVersionDb(pool, "jets_ws", imageRelease); err != nil {
		t.Fatalf("the compile's version insert still fails after the migration: %v", err)
	}
	var name string
	if err := pool.QueryRow(context.Background(),
		"SELECT workspace_name FROM jetsapi.workspace_version WHERE version = $1",
		imageRelease).Scan(&name); err != nil {
		t.Fatalf("reading back the recorded version: %v", err)
	}
	if name != "jets_ws" {
		t.Errorf("workspace_name: got %q, want jets_ws", name)
	}

	// The release is deliberately still the deployed one: recording it stays with
	// checkDomainTablesVersion, so a start that fails after this point repeats both
	// halves rather than skipping the second.
	var release string
	if err := pool.QueryRow(context.Background(),
		"SELECT MAX(version) FROM jetsapi.jetstore_release").Scan(&release); err != nil {
		t.Fatalf("reading the release: %v", err)
	}
	if release != deployedRelease {
		t.Errorf("the early step recorded the release; it should not have: got %q", release)
	}
}

// TestTheMigrationIsGatedOnTheReleaseAndNotTheWorkspace is the objection to the
// gate the handoff proposed, made into a test.
//
// A workspace version changes on every recompile and a release does not. Gating the
// early migration on a workspace-version change would run it far more often than
// the step it is being lifted out of, which is the opposite of the intent. Here the
// database is already at the image's release and the workspace is not, which is the
// ordinary recompile: the step must do nothing.
func TestTheMigrationIsGatedOnTheReleaseAndNotTheWorkspace(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	useImageRelease(t)
	pool := startupOrderDB(t, "startup_order_gate")
	installPreviousRelease(t, pool)
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO jetsapi.jetstore_release (version) VALUES ($1)", imageRelease); err != nil {
		t.Fatalf("recording the image release: %v", err)
	}

	s := &Server{dbpool: pool}
	if err := s.checkSystemTablesVersion(); err != nil {
		t.Fatalf("checkSystemTablesVersion: %v", err)
	}
	if hasColumn(t, pool, "workspace_version", "workspace_name") {
		t.Error("the step migrated a database already at the deployed release")
	}
}

// TestTheMigrationIsIdempotent covers the cost of the fix rather than the fix.
//
// checkDomainTablesVersion still passes -migrateDb, so on a release bump the system
// tables are migrated twice in one start. That is affordable only if the second
// pass is a no-op, and this is the assertion that says so.
func TestTheMigrationIsIdempotent(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	useImageRelease(t)
	pool := startupOrderDB(t, "startup_order_twice")
	installPreviousRelease(t, pool)

	s := &Server{dbpool: pool}
	if err := s.checkSystemTablesVersion(); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if err := s.checkSystemTablesVersion(); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if !hasColumn(t, pool, "workspace_version", "workspace_name") {
		t.Error("the second pass removed what the first added")
	}
	if err := workspace.UpdateWorkspaceVersionDb(pool, "jets_ws", imageRelease); err != nil {
		t.Fatalf("the compile's version insert fails after two passes: %v", err)
	}
}
