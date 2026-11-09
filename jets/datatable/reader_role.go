package datatable

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ReaderRole is the database role a user's read runs as.
//
// The read endpoints take SQL from the client: raw_query and raw_query_map take
// whole statements, and a data-table read takes WITH clauses, calculated columns
// and ge/le expressions. They need only jetstore_read, which every role holds.
// readOnlyTx already stops them writing; this role stops them reading what no
// screen shows. readOnlyTx switches to it with SET LOCAL ROLE, so every read
// path that runs in that transaction is covered, and a read that cannot switch
// fails rather than running with the apiserver's own privileges.
//
// What it may read:
//   - every table and view in the schemas the apiserver's database user owns,
//     and in public, as they stand at each apiserver start, and every table that
//     user creates later (default privileges);
//   - not the columns in secretColumns;
//   - not the schemas an extension installed (aws_s3's export to S3, for one),
//     so a read cannot call their functions.
//
// raw_query_tool (workspace_ide) and exec_ddl do not run in readOnlyTx and are
// unchanged: the query tool is free SQL by design.
const ReaderRole = "jets_reader"

// secretColumns are the columns no read may return, by table. Every other
// column of these tables stays readable, so the screens that list users keep
// working.
var secretColumns = map[string][]string{
	"jetsapi.users":           {"password", "git_token"},
	"jetsapi.secret_rotation": {"secret"},
}

// readerRoleLock serialises EnsureReaderRole across apiservers starting
// together: CREATE ROLE has no IF NOT EXISTS and is cluster-wide.
const readerRoleLock = 7_095_000_001

// EnsureReaderRole creates ReaderRole if it does not exist, makes the current
// database user a member so readOnlyTx can switch to it, and brings its grants
// up to date. It is idempotent and runs in one transaction, so a secret column
// is never readable between the schema-wide grant and its revoke. The
// apiserver calls it at every start.
func EnsureReaderRole(ctx context.Context, dbpool *pgxpool.Pool) error {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("while setting up the %s role: %w", ReaderRole, err)
	}
	defer tx.Rollback(ctx)
	role := pgx.Identifier{ReaderRole}.Sanitize()

	stmts := []string{
		fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", readerRoleLock),
		fmt.Sprintf(`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
				CREATE ROLE %s NOLOGIN;
			END IF;
		END $$`, ReaderRole, role),
		fmt.Sprintf("GRANT %s TO CURRENT_USER", role),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO %s", role),
	}
	for _, stmt := range stmts {
		if _, err = tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("while setting up the %s role (%s): %w", ReaderRole, strings.SplitN(stmt, "\n", 2)[0], err)
		}
	}

	schemas, err := readableSchemas(ctx, tx)
	if err != nil {
		return err
	}
	for _, schema := range schemas {
		s := pgx.Identifier{schema}.Sanitize()
		for _, stmt := range []string{
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", s, role),
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s", s, role),
		} {
			if _, err = tx.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("while granting %s on schema %s: %w", ReaderRole, schema, err)
			}
		}
	}

	for table, secrets := range secretColumns {
		if err = withholdColumns(ctx, tx, table, secrets); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("while setting up the %s role: %w", ReaderRole, err)
	}
	log.Printf("Reads run as the %s role, on schemas %v", ReaderRole, schemas)
	return nil
}

// readableSchemas are the schemas the current user owns, and public, less any
// schema an extension was installed into (other than public).
func readableSchemas(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT n.nspname FROM pg_namespace n
		WHERE (n.nspowner = (SELECT oid FROM pg_roles WHERE rolname = current_user) OR n.nspname = 'public')
		  AND n.nspname NOT LIKE 'pg\_%' AND n.nspname <> 'information_schema'
		  AND (n.nspname = 'public' OR NOT EXISTS (SELECT 1 FROM pg_extension e WHERE e.extnamespace = n.oid))
		ORDER BY n.nspname`)
	if err != nil {
		return nil, fmt.Errorf("while listing the schemas %s may read: %w", ReaderRole, err)
	}
	schemas, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while listing the schemas %s may read: %w", ReaderRole, err)
	}
	return schemas, nil
}

// withholdColumns replaces the role's table-wide SELECT on table with a grant
// on every column but secrets. A table that does not exist yet is skipped.
func withholdColumns(ctx context.Context, tx pgx.Tx, table string, secrets []string) error {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
		return fmt.Errorf("while looking for %s: %w", table, err)
	}
	if !exists {
		return nil
	}
	rows, err := tx.Query(ctx, `
		SELECT attname FROM pg_attribute
		WHERE attrelid = $1::regclass AND attnum > 0 AND NOT attisdropped AND NOT (attname = ANY($2))
		ORDER BY attnum`, table, secrets)
	if err != nil {
		return fmt.Errorf("while listing the columns of %s: %w", table, err)
	}
	columns, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("while listing the columns of %s: %w", table, err)
	}
	role := pgx.Identifier{ReaderRole}.Sanitize()
	schema, name, _ := strings.Cut(table, ".")
	t := pgx.Identifier{schema, name}.Sanitize()
	stmts := []string{
		fmt.Sprintf("REVOKE SELECT ON %s FROM %s", t, role),
		fmt.Sprintf("REVOKE SELECT (%s) ON %s FROM %s", identifierList(secrets), t, role),
	}
	if len(columns) > 0 {
		stmts = append(stmts, fmt.Sprintf("GRANT SELECT (%s) ON %s TO %s", identifierList(columns), t, role))
	}
	for _, stmt := range stmts {
		if _, err = tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("while withholding the secret columns of %s: %w", table, err)
		}
	}
	return nil
}

func identifierList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = pgx.Identifier{n}.Sanitize()
	}
	return strings.Join(quoted, ", ")
}
