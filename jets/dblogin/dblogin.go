// Package dblogin sets up the two restricted database logins that SQL sent by
// a user runs on, and opens connections as them.
//
// # Why logins, and not SET ROLE
//
// A user's SQL used to run on the apiserver's own connection, either as the
// apiserver's database user or after SET LOCAL ROLE to a restricted role. A
// session may switch back to any role its *session user* (the login) is a
// member of, and the statement itself can do it: set_config('role', ...) in a
// DO block or a function call. So a role switch on the apiserver's login
// restricts nothing that the SQL being restricted cannot undo. The restriction
// holds only when the session user itself is the restricted role, which is what
// these logins are. Their session user is a member of nothing but its group
// role, so there is nothing to switch back to.
//
// # The roles
//
//   - Reader, jets_reader: every read a user sends SQL on (raw_query,
//     raw_query_map, the data-table read and its SQL fragments, a runbook's
//     query and wait steps). It reads every table in the schemas the
//     apiserver's user owns, and public, except the secret columns and the
//     login table, and calls no extension's functions.
//   - IDE, jets_ide: the free-SQL paths that write (the Query Tool, Submit DDL,
//     a runbook's ddl steps), the workspace's own init scripts (update_db) and
//     its reports (run_reports).
//     It reads and writes every table those schemas hold, and creates tables
//     outside jetsapi, except that it only reads the security tables (users,
//     roles, role_capability, secret_rotation), without their secret columns.
//     It holds no TRIGGER privilege: a trigger it put on a table the apiserver
//     writes would run as the apiserver.
//
// Each group role is NOLOGIN and holds the grants. The login that connects is
// a per-database member of it (LoginName), because roles are cluster-wide and
// each database keeps its own password: two databases on one cluster would
// otherwise reset each other's. The apiserver's user is a member of both
// logins, so it may cancel their statements (a runbook's Abort) and owns, by
// membership, the tables the IDE login creates.
//
// # The password
//
// Generated once per database, kept in jetsapi.restricted_login, which only the
// apiserver's user may read, and sent to PostgreSQL as a SCRAM verifier, so the
// password itself is never in a statement a server log could record. No
// deployment setting is involved, and every apiserver task and update_db read
// the same one.
package dblogin

import (
	"context"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// Reader is the group role a user's reads run as.
	Reader = "jets_reader"
	// IDE is the group role the free-SQL paths, the workspace scripts and the
	// workspace's reports run as.
	IDE = "jets_ide"

	// LoginTable holds each login's password. Neither login may read it.
	LoginTable = "jetsapi.restricted_login"

	// ensureLock serialises Ensure across apiservers and update_db starting
	// together: CREATE ROLE has no IF NOT EXISTS and is cluster-wide.
	ensureLock = 7_095_000_001
)

// SecretColumns are the columns neither login may read, by table. Every other
// column of these tables stays readable, so the screens that list users work.
var SecretColumns = map[string][]string{
	"jetsapi.users":           {"password", "git_token"},
	"jetsapi.secret_rotation": {"secret"},
}

// SecurityTables are the tables the IDE login may only read: who the users
// are, what each role may do, and the secrets. They change through
// jets_init_db.sql and the Users screen, both on the apiserver's own user.
var SecurityTables = []string{
	"jetsapi.users",
	"jetsapi.roles",
	"jetsapi.role_capability",
	"jetsapi.secret_rotation",
}

// ideExtensions are the extensions whose functions the IDE login may call:
// Aurora's export to S3, which Submit DDL is used for.
var ideExtensions = []string{"aws_s3", "aws_commons"}

// LoginName is the login role of group in the current database of q.
func LoginName(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, group string) (string, error) {
	var db string
	if err := q.QueryRow(ctx, "SELECT current_database()").Scan(&db); err != nil {
		return "", fmt.Errorf("while reading the database name: %w", err)
	}
	return loginName(group, db), nil
}

func loginName(group, database string) string {
	sum := sha256.Sum256([]byte(database))
	return group + "_" + hex.EncodeToString(sum[:5])
}

// Ensure creates both group roles and their logins if they do not exist, sets
// each login's password, and brings the grants up to date for the tables the
// database holds now. It is idempotent and runs in one transaction, so no
// secret column or security table is open between a schema-wide grant and its
// revoke. The apiserver and update_db call it at every start.
func Ensure(ctx context.Context, dbpool *pgxpool.Pool) error {
	tx, err := dbpool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("while setting up the restricted logins: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, fmt.Sprintf("SELECT pg_advisory_xact_lock(%d)", ensureLock)); err != nil {
		return fmt.Errorf("while setting up the restricted logins: %w", err)
	}
	// The migration creates jetsapi; a database set up in another order gets it
	// here, so the login table has somewhere to go.
	if _, err = tx.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		return fmt.Errorf("while creating the jetsapi schema: %w", err)
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS `+LoginTable+` (
		login_role  TEXT PRIMARY KEY,
		password    TEXT NOT NULL,
		last_update TIMESTAMP NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("while creating %s: %w", LoginTable, err)
	}
	if _, err = tx.Exec(ctx, "REVOKE ALL ON "+LoginTable+" FROM PUBLIC"); err != nil {
		return fmt.Errorf("while closing %s: %w", LoginTable, err)
	}

	var database string
	if err = tx.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return fmt.Errorf("while reading the database name: %w", err)
	}
	logins := map[string]string{}
	for _, group := range []string{Reader, IDE} {
		login := loginName(group, database)
		logins[group] = login
		if err = ensureLogin(ctx, tx, group, login); err != nil {
			return err
		}
	}

	schemas, err := ownedSchemas(ctx, tx)
	if err != nil {
		return err
	}
	if err = grantReader(ctx, tx, schemas, logins[IDE]); err != nil {
		return err
	}
	if err = grantIDE(ctx, tx, schemas); err != nil {
		return err
	}
	// Last, so the schema-wide grants above cannot reopen it.
	for _, group := range []string{Reader, IDE} {
		if _, err = tx.Exec(ctx, fmt.Sprintf("REVOKE ALL ON %s FROM %s", LoginTable, ident(group))); err != nil {
			return fmt.Errorf("while closing %s to %s: %w", LoginTable, group, err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("while setting up the restricted logins: %w", err)
	}
	log.Printf("Restricted logins %s (reads) and %s (Query Tool, Submit DDL, runbook ddl, workspace scripts) on schemas %v",
		logins[Reader], logins[IDE], schemas)
	return nil
}

// ensureLogin creates group (NOLOGIN) and login (a member of group and of
// nothing else) if they do not exist, makes the current user a member of the
// login, and sets the login's password from LoginTable, generating it the
// first time.
func ensureLogin(ctx context.Context, tx pgx.Tx, group, login string) error {
	for _, role := range []string{group, login} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%s') THEN
				CREATE ROLE %s NOLOGIN;
			END IF;
		END $$`, role, ident(role))); err != nil {
			return fmt.Errorf("while creating the role %s: %w", role, err)
		}
	}
	var password string
	err := tx.QueryRow(ctx, "SELECT password FROM "+LoginTable+" WHERE login_role = $1", login).Scan(&password)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if password, err = newPassword(); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO "+LoginTable+" (login_role, password) VALUES ($1, $2)", login, password); err != nil {
			return fmt.Errorf("while recording the password of %s: %w", login, err)
		}
	case err != nil:
		return fmt.Errorf("while reading the password of %s: %w", login, err)
	}
	verifier, err := scramVerifier(password)
	if err != nil {
		return err
	}
	// The password statement on its own, so an error cannot carry the verifier.
	if _, err = tx.Exec(ctx, fmt.Sprintf("ALTER ROLE %s WITH LOGIN PASSWORD '%s'", ident(login), verifier)); err != nil {
		return fmt.Errorf("while setting the password of %s: %w", login, err)
	}
	for _, stmt := range []string{
		fmt.Sprintf("GRANT %s TO %s", ident(group), ident(login)),
		// The apiserver's user: to cancel the login's statements and to own,
		// by membership, what the login creates.
		fmt.Sprintf("GRANT %s TO CURRENT_USER", ident(login)),
		// For the reader's SET LOCAL ROLE of releases before this one, which a
		// rolling deployment may still run for a minute.
		fmt.Sprintf("GRANT %s TO CURRENT_USER", ident(group)),
	} {
		if _, err = tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("while setting up the login %s (%s): %w", login, stmt, err)
		}
	}
	return checkLogin(ctx, tx, group, login)
}

// checkLogin refuses a login that could do more than its group: one holding an
// attribute that bypasses grants, or a member of any role but its group. The
// attributes are not set by ensureLogin, because before PostgreSQL 16 a user
// that is not a superuser may not name REPLICATION or BYPASSRLS even to turn
// them off; a login it created has none of them, and this says so if that
// stopped being true.
func checkLogin(ctx context.Context, tx pgx.Tx, group, login string) error {
	var privileged bool
	var others []string
	err := tx.QueryRow(ctx, `
		SELECT r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls,
		       ARRAY(SELECT g.rolname::text FROM pg_auth_members m JOIN pg_roles g ON g.oid = m.roleid
		             WHERE m.member = r.oid AND g.rolname <> $2 ORDER BY 1)
		FROM pg_roles r WHERE r.rolname = $1`, login, group).Scan(&privileged, &others)
	if err != nil {
		return fmt.Errorf("while checking the login %s: %w", login, err)
	}
	if privileged {
		return fmt.Errorf("the login %s holds SUPERUSER, CREATEROLE, CREATEDB, REPLICATION or BYPASSRLS; remove it (ALTER ROLE %s NOSUPERUSER ...) and start again", login, login)
	}
	if len(others) > 0 {
		return fmt.Errorf("the login %s is a member of %v besides %s; revoke them and start again", login, others, group)
	}
	return nil
}

// grantReader gives Reader SELECT on every table in schemas, now and later,
// less the secret columns, and nothing in an extension's schema. ideLogin's
// tables are readable too: they are created by the IDE login, not by the
// current user, so they need a default of their own.
func grantReader(ctx context.Context, tx pgx.Tx, schemas []string, ideLogin string) error {
	role := ident(Reader)
	stmts := []string{
		"ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO " + role,
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES FOR ROLE %s GRANT SELECT ON TABLES TO %s", ident(ideLogin), role),
	}
	for _, schema := range schemas {
		s := ident(schema)
		stmts = append(stmts,
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", s, role),
			fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA %s TO %s", s, role))
	}
	if err := execAll(ctx, tx, "granting "+Reader, stmts); err != nil {
		return err
	}
	for table, secrets := range SecretColumns {
		if err := withholdColumns(ctx, tx, Reader, table, secrets); err != nil {
			return err
		}
	}
	return nil
}

// grantIDE gives IDE what the free-SQL paths need on every table in schemas,
// now and later, then takes the security tables back to a read without their
// secret columns. It may create tables in every schema but jetsapi.
func grantIDE(ctx context.Context, tx pgx.Tx, schemas []string) error {
	role := ident(IDE)
	const tablePrivileges = "SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES"
	stmts := []string{
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES GRANT %s ON TABLES TO %s", tablePrivileges, role),
		"ALTER DEFAULT PRIVILEGES GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO " + role,
	}
	for _, schema := range schemas {
		s := ident(schema)
		stmts = append(stmts,
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", s, role),
			fmt.Sprintf("GRANT %s ON ALL TABLES IN SCHEMA %s TO %s", tablePrivileges, s, role),
			fmt.Sprintf("REVOKE TRIGGER ON ALL TABLES IN SCHEMA %s FROM %s", s, role),
			fmt.Sprintf("GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA %s TO %s", s, role))
		if schema == "jetsapi" {
			stmts = append(stmts, fmt.Sprintf("REVOKE CREATE ON SCHEMA %s FROM %s", s, role))
		} else {
			stmts = append(stmts, fmt.Sprintf("GRANT CREATE ON SCHEMA %s TO %s", s, role))
		}
	}
	if err := execAll(ctx, tx, "granting "+IDE, stmts); err != nil {
		return err
	}
	for _, table := range SecurityTables {
		exists, err := tableExists(ctx, tx, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf("REVOKE ALL ON %s FROM %s", qualified(table), role)); err != nil {
			return fmt.Errorf("while closing %s to %s: %w", table, IDE, err)
		}
		if secrets, ok := SecretColumns[table]; ok {
			if err = withholdColumns(ctx, tx, IDE, table, secrets); err != nil {
				return err
			}
		} else if _, err = tx.Exec(ctx, fmt.Sprintf("GRANT SELECT ON %s TO %s", qualified(table), role)); err != nil {
			return fmt.Errorf("while opening %s to %s: %w", table, IDE, err)
		}
	}
	// Aurora's export to S3, when the database offers it.
	rows, err := tx.Query(ctx, `SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace
		WHERE e.extname = ANY($1) ORDER BY 1`, ideExtensions)
	if err != nil {
		return fmt.Errorf("while listing the extensions %s may call: %w", IDE, err)
	}
	extSchemas, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return fmt.Errorf("while listing the extensions %s may call: %w", IDE, err)
	}
	stmts = stmts[:0]
	for _, schema := range extSchemas {
		s := ident(schema)
		stmts = append(stmts,
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", s, role),
			fmt.Sprintf("GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA %s TO %s", s, role))
	}
	return execAll(ctx, tx, "granting "+IDE+" the export functions", stmts)
}

// ownedSchemas are the schemas the current user owns, and public, less any
// schema an extension was installed into (other than public).
func ownedSchemas(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT n.nspname FROM pg_namespace n
		WHERE (n.nspowner = (SELECT oid FROM pg_roles WHERE rolname = current_user) OR n.nspname = 'public')
		  AND n.nspname NOT LIKE 'pg\_%' AND n.nspname <> 'information_schema'
		  AND (n.nspname = 'public' OR NOT EXISTS (SELECT 1 FROM pg_extension e WHERE e.extnamespace = n.oid))
		ORDER BY n.nspname`)
	if err != nil {
		return nil, fmt.Errorf("while listing the schemas the restricted logins reach: %w", err)
	}
	schemas, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while listing the schemas the restricted logins reach: %w", err)
	}
	return schemas, nil
}

// withholdColumns replaces role's table-wide SELECT on table with a grant on
// every column but secrets. A table that does not exist yet is skipped.
func withholdColumns(ctx context.Context, tx pgx.Tx, group, table string, secrets []string) error {
	exists, err := tableExists(ctx, tx, table)
	if err != nil || !exists {
		return err
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
	role, t := ident(group), qualified(table)
	stmts := []string{
		fmt.Sprintf("REVOKE SELECT ON %s FROM %s", t, role),
		fmt.Sprintf("REVOKE SELECT (%s) ON %s FROM %s", identList(secrets), t, role),
	}
	if len(columns) > 0 {
		stmts = append(stmts, fmt.Sprintf("GRANT SELECT (%s) ON %s TO %s", identList(columns), t, role))
	}
	return execAll(ctx, tx, "withholding the secret columns of "+table+" from "+group, stmts)
}

// Open opens a pool on the same server and database as base, connected as
// group's login with the password in LoginTable, and checks it connects. Ensure
// must have run.
func Open(ctx context.Context, base *pgxpool.Pool, group string, maxConns int32) (*pgxpool.Pool, error) {
	login, err := LoginName(ctx, base, group)
	if err != nil {
		return nil, err
	}
	var password string
	if err = base.QueryRow(ctx, "SELECT password FROM "+LoginTable+" WHERE login_role = $1", login).Scan(&password); err != nil {
		return nil, fmt.Errorf("while reading the password of %s (has the apiserver or update_db set it up?): %w", login, err)
	}
	cfg := base.Config()
	cfg.ConnConfig.User = login
	cfg.ConnConfig.Password = password
	params := map[string]string{}
	for k, v := range cfg.ConnConfig.RuntimeParams {
		params[k] = v
	}
	params["application_name"] = "jetstore_" + group
	cfg.ConnConfig.RuntimeParams = params
	cfg.MaxConns = maxConns
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("while opening the %s login: %w", login, err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("while connecting as %s: %w", login, err)
	}
	return pool, nil
}

// newPassword is 32 random bytes, URL-safe base64: ASCII, so SASLprep leaves it
// as it is.
func newPassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("while generating a password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// scramVerifier is the SCRAM-SHA-256 verifier PostgreSQL stores for password
// (RFC 5802, RFC 7677), in the form ALTER ROLE ... PASSWORD accepts as already
// encrypted: SCRAM-SHA-256$<iterations>:<salt>$<StoredKey>:<ServerKey>.
func scramVerifier(password string) (string, error) {
	const iterations = 4096
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("while generating a salt: %w", err)
	}
	salted, err := pbkdf2.Key(sha256.New, password, salt, iterations, sha256.Size)
	if err != nil {
		return "", fmt.Errorf("while deriving the password key: %w", err)
	}
	mac := func(key []byte, msg string) []byte {
		h := hmac.New(sha256.New, key)
		h.Write([]byte(msg))
		return h.Sum(nil)
	}
	clientKey := mac(salted, "Client Key")
	storedKey := sha256.Sum256(clientKey)
	serverKey := mac(salted, "Server Key")
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", iterations, b64(salt), b64(storedKey[:]), b64(serverKey)), nil
}

func tableExists(ctx context.Context, tx pgx.Tx, table string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
		return false, fmt.Errorf("while looking for %s: %w", table, err)
	}
	return exists, nil
}

func execAll(ctx context.Context, tx pgx.Tx, what string, stmts []string) error {
	for _, stmt := range stmts {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("while %s (%s): %w", what, stmt, err)
		}
	}
	return nil
}

func ident(name string) string { return pgx.Identifier{name}.Sanitize() }

func qualified(table string) string {
	schema, name, _ := strings.Cut(table, ".")
	return pgx.Identifier{schema, name}.Sanitize()
}

func identList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = ident(n)
	}
	return strings.Join(quoted, ", ")
}
