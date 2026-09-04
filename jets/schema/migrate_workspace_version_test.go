// The workspace_version migration, and its rollback, run against a real Postgres.
//
// Two columns is the cheap half. The expensive half is that the table's unique
// constraint was on the version string alone across every workspace, so naming the
// workspace makes that constraint visibly wrong -- and **changing a constraint on a
// populated table is a different class of migration from the ADD COLUMN IF NOT
// EXISTS that preceded it** (I-333, Q-41). This exercises the change on a table
// that already holds rows, and it exercises the way back.
//
// Needs JETS_TEST_DSN; skipped otherwise. See create_table_deleted_test.go for the
// one-line docker command.
package schema

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// workspaceVersionBefore is the table as it shipped until 2026-09-04: a key, a
// version, and a unique constraint on the version alone.
func workspaceVersionBefore() TableDefinition {
	return TableDefinition{
		SchemaName: "jetsapi",
		TableName:  "workspace_version",
		Columns: []ColumnDefinition{
			{ColumnName: "key", DataType: "int", IsPK: true},
			{ColumnName: "version", DataType: "text", IsNotNull: true},
		},
		TableConstraints: []ConstraintDefinition{{
			Name:       "workspace_version_unique_cstraint",
			Definition: "CONSTRAINT workspace_version_unique_cstraint UNIQUE (version)",
		}},
	}
}

// workspaceVersionRolledBack is the definition a rollback ships: the two new
// columns marked deleted so UpdateTable drops them, and the old constraint
// restored. It is written out rather than derived, because a rollback that is
// derived from the thing being rolled back is not a rollback anybody can review.
func workspaceVersionRolledBack() TableDefinition {
	def := workspaceVersionBefore()
	def.Columns = append(def.Columns,
		ColumnDefinition{ColumnName: "workspace_name", DataType: "text", Deleted: true},
		ColumnDefinition{ColumnName: "workspace_commit", DataType: "text", Deleted: true},
	)
	return def
}

func constraintNames(t *testing.T, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT con.conname FROM pg_catalog.pg_constraint con
		 INNER JOIN pg_catalog.pg_class rel ON rel.oid = con.conrelid
		 INNER JOIN pg_catalog.pg_namespace nsp ON nsp.oid = connamespace
		 WHERE nsp.nspname = 'jetsapi' AND rel.relname = $1 AND con.contype = 'u'
		 ORDER BY con.conname`, table)
	if err != nil {
		t.Fatalf("reading constraints of %s: %v", table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scanning constraint name: %v", err)
		}
		out = append(out, s)
	}
	return out
}

// populatedWorkspaceVersion installs the pre-migration table and seeds it with the
// shape a real deployment holds: version strings and nothing else.
func populatedWorkspaceVersion(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatalf("creating jetsapi schema: %v", err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS jetsapi.workspace_version"); err != nil {
		t.Fatalf("dropping workspace_version: %v", err)
	}
	before := workspaceVersionBefore()
	if err := before.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("installing the pre-migration workspace_version: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO jetsapi.workspace_version (version) VALUES ('1788190686'), ('1788216335')`); err != nil {
		t.Fatalf("seeding workspace_version: %v", err)
	}
}

// TestWorkspaceVersionMigration is the forward half: the ALTER on a populated
// table, through the same UpdateTableSchema call update_db -migrateDb makes.
func TestWorkspaceVersionMigration(t *testing.T) {
	pool := schemaTestPool(t, "ah2_migration")
	ctx := context.Background()
	populatedWorkspaceVersion(t, pool)

	after := loadTableDefinition(t, "workspace_version")
	if err := after.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating a populated workspace_version: %v", err)
	}

	// The existing rows survive, and the new columns are null on them rather than
	// defaulted to something a reader would mistake for a recorded workspace.
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.workspace_version
		 WHERE workspace_name IS NULL AND workspace_commit IS NULL`).Scan(&n); err != nil {
		t.Fatalf("reading migrated rows: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 surviving rows with both new columns null, got %d", n)
	}

	// The constraint moved.
	got := strings.Join(constraintNames(t, pool, "workspace_version"), ",")
	if got != "workspace_version_unique_cstraint_v2" {
		t.Errorf("unique constraints after migration: got %q, want only the v2 constraint", got)
	}
}

// TestMigratedConstraintPermitsTwoWorkspacesAtOneVersion is what the migration
// buys, stated as behaviour. The version is a unix timestamp or the JetStore
// version, so two workspaces compiled in the same second collided under the old
// constraint and ON CONFLICT DO NOTHING kept whichever arrived first.
func TestMigratedConstraintPermitsTwoWorkspacesAtOneVersion(t *testing.T) {
	pool := schemaTestPool(t, "ah2_two_workspaces")
	ctx := context.Background()
	populatedWorkspaceVersion(t, pool)
	after := loadTableDefinition(t, "workspace_version")
	if err := after.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	stmt := `INSERT INTO jetsapi.workspace_version (version, workspace_name, workspace_commit)
	         VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	if _, err := pool.Exec(ctx, stmt, "1788300000", "ws_a", strings.Repeat("a", 40)); err != nil {
		t.Fatalf("inserting ws_a: %v", err)
	}
	if _, err := pool.Exec(ctx, stmt, "1788300000", "ws_b", strings.Repeat("b", 40)); err != nil {
		t.Fatalf("inserting ws_b at the same version: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.workspace_version WHERE version = '1788300000'`).Scan(&n); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != 2 {
		t.Errorf("expected both workspaces recorded at one version, got %d rows", n)
	}

	// And the same (workspace, version) twice is still one row, so the writer's
	// ON CONFLICT DO NOTHING keeps the idempotence it had.
	if _, err := pool.Exec(ctx, stmt, "1788300000", "ws_a", strings.Repeat("a", 40)); err != nil {
		t.Fatalf("re-inserting ws_a: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.workspace_version WHERE version = '1788300000'`).Scan(&n); err != nil {
		t.Fatalf("counting after repeat: %v", err)
	}
	if n != 2 {
		t.Errorf("a repeated (workspace, version) was not deduplicated: %d rows", n)
	}
}

// TestWorkspaceVersionRollback is the way back, and it is tested because a
// migration that can fail with an untested rollback is worse than one nobody
// wrote. The rollback is: restore the old definition with the two columns marked
// deleted, and re-run update_db -migrateDb. No hand-written SQL, and it goes
// through the same UpdateTable the forward migration used.
func TestWorkspaceVersionRollback(t *testing.T) {
	pool := schemaTestPool(t, "ah2_rollback")
	ctx := context.Background()
	populatedWorkspaceVersion(t, pool)
	after := loadTableDefinition(t, "workspace_version")
	if err := after.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO jetsapi.workspace_version (version, workspace_name, workspace_commit)
		 VALUES ('1788400000', 'ws_a', $1)`, strings.Repeat("a", 40)); err != nil {
		t.Fatalf("writing a post-migration row: %v", err)
	}

	back := workspaceVersionRolledBack()
	if err := back.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("rolling back a populated workspace_version: %v", err)
	}

	// The columns are gone and the old constraint is back.
	if columnExists(t, pool, "workspace_version", "workspace_name") {
		t.Error("workspace_name survived the rollback")
	}
	if columnExists(t, pool, "workspace_version", "workspace_commit") {
		t.Error("workspace_commit survived the rollback")
	}
	got := strings.Join(constraintNames(t, pool, "workspace_version"), ",")
	if got != "workspace_version_unique_cstraint" {
		t.Errorf("unique constraints after rollback: got %q, want only the original", got)
	}

	// **No version is lost.** The rollback drops what the migration added and
	// nothing else; the three version strings are all still there.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.workspace_version`).Scan(&n); err != nil {
		t.Fatalf("counting after rollback: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3 rows to survive the rollback, got %d", n)
	}
}

// TestWorkspaceVersionRollbackFailsLoudlyOnDuplicateVersions is the honest half,
// and it is the reason the rollback has a window rather than being free.
//
// The migration exists precisely so two workspaces can share a version string.
// Once one pair does, the old constraint no longer describes the data, and the
// rollback **cannot** succeed. What matters is that it fails rather than
// discarding a row, and that the error names the constraint -- so an operator gets
// a decision to make instead of silent data loss.
func TestWorkspaceVersionRollbackFailsLoudlyOnDuplicateVersions(t *testing.T) {
	pool := schemaTestPool(t, "ah2_rollback_conflict")
	ctx := context.Background()
	populatedWorkspaceVersion(t, pool)
	after := loadTableDefinition(t, "workspace_version")
	if err := after.UpdateTableSchema(pool, false); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	stmt := `INSERT INTO jetsapi.workspace_version (version, workspace_name) VALUES ($1, $2)`
	if _, err := pool.Exec(ctx, stmt, "1788500000", "ws_a"); err != nil {
		t.Fatalf("inserting ws_a: %v", err)
	}
	if _, err := pool.Exec(ctx, stmt, "1788500000", "ws_b"); err != nil {
		t.Fatalf("inserting ws_b: %v", err)
	}

	back := workspaceVersionRolledBack()
	err := back.UpdateTableSchema(pool, false)
	if err == nil {
		t.Fatal("expected the rollback to fail once two workspaces share a version")
	}
	if !strings.Contains(err.Error(), "workspace_version_unique_cstraint") {
		t.Errorf("the rollback error does not name the constraint: %v", err)
	}

	// The failure is atomic: UpdateTable issues one ALTER, so nothing is dropped.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.workspace_version`).Scan(&n); err != nil {
		t.Fatalf("counting after the failed rollback: %v", err)
	}
	if n != 4 {
		t.Errorf("a failed rollback changed the data: expected 4 rows, got %d", n)
	}
	if !columnExists(t, pool, "workspace_version", "workspace_name") {
		t.Error("a failed rollback dropped workspace_name")
	}
}
