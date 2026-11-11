package main

// jets_init_db.sql applies to a plain PostgreSQL, and applies twice, keeping a
// role the installation added. update_db applies it at every new release and
// every workspace compile (-initBaseWorkspaceDb).
//
// It opened with CREATE EXTENSION aws_s3, which a plain PostgreSQL does not
// have, so it could not be applied there at all; and it truncated and reseeded
// jetsapi.roles and jetsapi.role_capability, so a role of the installation's own
// was deleted, with its capabilities, at the next compile.
//
// Skipped unless JETS_TEST_DSN names a PostgreSQL without aws_s3
// (postgres:16-alpine), in a database of its own built by
// jets/sessionpurge/purgetest. Run with -count=1.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func capabilitiesOf(t *testing.T, pool *pgxpool.Pool, role string) string {
	t.Helper()
	var caps string
	err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(string_agg(capability, ',' ORDER BY capability), '')
		 FROM jetsapi.role_capability WHERE role = $1`, role).Scan(&caps)
	if err != nil {
		t.Fatal(err)
	}
	return caps
}

func TestInitDbScriptAppliesTwiceAndKeepsAnInstallationRole(t *testing.T) {
	pool := purgetest.DB(t, "jm_init_db_script")
	bg := context.Background()
	buf, err := os.ReadFile(filepath.Join(purgetest.JetsDir(), "jets_init_db.sql"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(buf)
	var available bool
	if err := pool.QueryRow(bg, `SELECT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'aws_s3')`).
		Scan(&available); err != nil {
		t.Fatal(err)
	}
	if available {
		t.Skip("this PostgreSQL offers aws_s3; the test is for one that does not")
	}

	// No bind arguments: one simple-protocol script, as update_db sends it.
	if _, err := pool.Exec(bg, script); err != nil {
		t.Fatalf("first application on a PostgreSQL without aws_s3: %v", err)
	}
	opsUser := capabilitiesOf(t, pool, "ops_user")
	if opsUser != "jetstore_read,run_pipelines,user_profile" {
		t.Fatalf("ops_user after the first application: %s", opsUser)
	}

	// The installation adds a role of its own, grants a seeded role an extra
	// capability, and loses one of a seeded role's.
	for _, s := range []string{
		`INSERT INTO jetsapi.roles (role, details) VALUES ('site_auditor', 'the installation''s own role')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('site_auditor', 'jetstore_read')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('ops_user', 'workspace_ide')`,
		`DELETE FROM jetsapi.role_capability WHERE role = 'ops_user' AND capability = 'user_profile'`,
		`UPDATE jetsapi.roles SET details = 'edited' WHERE role = 'ops_user'`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("%v\n%s", err, s)
		}
	}

	if _, err := pool.Exec(bg, script); err != nil {
		t.Fatalf("second application: %v", err)
	}
	var details string
	if err := pool.QueryRow(bg, `SELECT details FROM jetsapi.roles WHERE role = 'site_auditor'`).Scan(&details); err != nil {
		t.Errorf("the installation's role is gone: %v", err)
	} else if details != "the installation's own role" {
		t.Errorf("the installation's role was changed: %q", details)
	}
	if got := capabilitiesOf(t, pool, "site_auditor"); got != "jetstore_read" {
		t.Errorf("the installation's role's capabilities: %q, want jetstore_read", got)
	}
	// A seeded role is exactly what the script seeds, as before.
	if got := capabilitiesOf(t, pool, "ops_user"); got != opsUser {
		t.Errorf("ops_user after the second application: %s, want %s", got, opsUser)
	}
	if err := pool.QueryRow(bg, `SELECT details FROM jetsapi.roles WHERE role = 'ops_user'`).Scan(&details); err != nil ||
		!strings.HasPrefix(details, "Role to load files") {
		t.Errorf("ops_user's details were not reseeded: %q %v", details, err)
	}
	var seeded int
	if err := pool.QueryRow(bg, `SELECT count(*) FROM jetsapi.roles`).Scan(&seeded); err != nil || seeded != 6 {
		t.Errorf("roles: %d (%v), want the five seeded and the installation's", seeded, err)
	}
	var installed bool
	if err := pool.QueryRow(bg, `SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'aws_s3')`).Scan(&installed); err != nil || installed {
		t.Errorf("aws_s3 installed: %v %v", installed, err)
	}
}
