package dblogin

// # The code guard
//
// The IDE login creates tables and owns them, and PostgreSQL runs some code
// attached to a table as whoever reads or writes it: a trigger, a row-level
// security policy, a rule's functions, a column default, a check constraint, an
// index expression, a function a view calls. The apiserver's user reads and
// writes the IDE login's tables (pipelines read workspace tables), so a
// function the IDE login wrote, attached to its own table, would run as the
// apiserver's user and could change the security tables. The grants cannot
// prevent it: the privilege that lets the login create a table in a schema is
// the one that lets it create a function there.
//
// So an event trigger, CodeGuardTrigger, refuses that DDL when the session's
// login is the IDE login:
//
//   - the commands in RefusedCommands: functions, procedures, triggers,
//     policies, rules, operators, aggregates, casts, extensions;
//   - ALTER TABLE leaving a table with row-level security enabled or forced;
//   - a default, check constraint, generated column, index expression or
//     predicate, domain constraint or view that calls a function outside
//     pg_catalog (an extension's, aws_s3's included), a volatile pg_catalog
//     function other than those in allowedVolatile (set_config, setval,
//     pg_terminate_backend, the large-object and file functions...), or one of
//     sqlRunners, which run SQL text or read a table by name as the caller
//     (query_to_xml, table_to_xml...).
//
// The third rule is there because refusing user functions is not enough on
// its own: aws_s3.query_export_to_s3 runs the SQL text it is given, and
// table_import_from_s3 writes into a table it is given, both as the caller, so
// a column default calling either would run a statement of the IDE login's
// choosing as the apiserver's user on Aurora. And query_to_xml would copy a
// secret column into a row the IDE login can read. A statement may still call
// any of them directly: it runs as the IDE login, which is what it is for.
//
// Creating an event trigger needs a superuser, or the master user
// (rds_superuser) on Amazon RDS and Aurora PostgreSQL. When Ensure cannot, it
// does not fail: it says so once, in a warning and in Status, and the guard
// can be installed by a superuser later (see the system tables page of the
// Developer's Guide). Ensure also lists the code the IDE login already owns,
// and does not drop it.
//
// CodeGuardSwitch=off makes Ensure remove the guard instead, as the
// apiserver's user: the way out of a guard that misbehaves, on an installation
// whose database only the application reaches.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	// CodeGuardTrigger is the event trigger that refuses the IDE login's
	// code-carrying DDL.
	CodeGuardTrigger = "jets_ide_code_guard"
	// CodeGuardFunction is the function it runs. It is the apiserver's user's,
	// in jetsapi, where the IDE login may not create anything.
	CodeGuardFunction = "jetsapi.jets_ide_code_guard"
	// CodeGuardSwitch is the environment variable that turns the guard off:
	// "off", in any case, makes Ensure remove it; any other value, or none,
	// keeps it on. It is the way out of a guard that misbehaves on an
	// installation where nobody can reach the database but the application.
	CodeGuardSwitch = "JETS_IDE_CODE_GUARD"

	// neutralised is the comment on a CodeGuardFunction that Ensure emptied
	// because it could neither drop nor disable the trigger.
	neutralised = "jetstore code guard neutralised by " + CodeGuardSwitch
)

// RefusedCommands are the command tags the IDE login may not run. Each creates
// or changes code that may run as another user: a function or procedure called
// from a default, a trigger, a policy, a view; a trigger, policy or rule
// itself; an operator, which a query written for the built-in one may pick up;
// an aggregate, a cast or an extension, which bring functions with them.
var RefusedCommands = []string{
	"CREATE FUNCTION", "ALTER FUNCTION",
	"CREATE PROCEDURE", "ALTER PROCEDURE", "ALTER ROUTINE",
	"CREATE TRIGGER", "ALTER TRIGGER",
	"CREATE POLICY", "ALTER POLICY",
	"CREATE RULE", "ALTER RULE",
	"CREATE OPERATOR", "ALTER OPERATOR",
	"CREATE AGGREGATE", "ALTER AGGREGATE",
	"CREATE CAST", "CREATE TRANSFORM", "CREATE LANGUAGE",
	"CREATE EXTENSION", "ALTER EXTENSION",
}

// allowedVolatile are the volatile pg_catalog functions a stored expression of
// the IDE login may call: what a default or a view uses to number or stamp a
// row. Every other volatile one changes something beyond the row (set_config,
// setval, pg_terminate_backend, pg_advisory_lock, lo_import, pg_notify, ...)
// or runs SQL text (query_to_xml, ts_stat), and would do it as the reader.
var allowedVolatile = []string{
	"nextval", "currval", "lastval",
	"random", "random_normal", "array_shuffle", "array_sample",
	"gen_random_uuid", "uuidv4", "uuidv7",
	"clock_timestamp", "timeofday",
}

// sqlRunners are the pg_catalog functions that are not volatile and still run
// SQL text or read a table by name, as the caller.
var sqlRunners = []string{
	"table_to_xml", "table_to_xmlschema", "table_to_xml_and_xmlschema",
	"schema_to_xml", "schema_to_xmlschema", "schema_to_xml_and_xmlschema",
	"database_to_xml", "database_to_xmlschema", "database_to_xml_and_xmlschema",
	"query_to_xml", "query_to_xmlschema", "query_to_xml_and_xmlschema",
	"cursor_to_xml", "cursor_to_xmlschema",
	"ts_stat", "pg_stat_get_activity", "pg_stat_get_backend_activity",
}

// ideRoleTest is true for a pg_roles row r that is the IDE group role or an IDE
// login: a role named for it and a direct member of it. The apiserver's user is
// a member of the group too, and is not named for it. It uses no function that
// fails when a role is missing, since the guard runs on every DDL statement.
func ideRoleTest(r string) string {
	return fmt.Sprintf(`(%[1]s.rolname = %[2]s OR (%[1]s.rolname LIKE %[3]s AND EXISTS (
		SELECT 1 FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles g ON g.oid = m.roleid
		WHERE m.member = %[1]s.oid AND g.rolname = %[2]s)))`,
		r, sqlText(IDE), sqlText(strings.ReplaceAll(IDE, "_", `\_`)+`\_%`))
}

// callsQuery lists the refused functions that the stored expressions of the
// relations rels and the domains types call, as (object, function). rels and
// types are SQL expressions of type oid[]. A function is found in the stored
// parse tree by its oid, which is how PostgreSQL itself records it.
func callsQuery(rels, types string) string {
	return fmt.Sprintf(`
WITH exprs(object, tree) AS (
	SELECT pg_catalog.format('the default of %%s.%%I', d.adrelid::pg_catalog.regclass, a.attname), d.adbin::text
	FROM pg_catalog.pg_attrdef d
	JOIN pg_catalog.pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum
	WHERE d.adrelid = ANY (%[1]s)
	UNION ALL
	SELECT pg_catalog.format('the constraint %%I on %%s', c.conname,
		COALESCE(NULLIF(c.conrelid, 0)::pg_catalog.regclass::text, c.contypid::pg_catalog.regtype::text)), c.conbin::text
	FROM pg_catalog.pg_constraint c
	WHERE c.conbin IS NOT NULL AND (c.conrelid = ANY (%[1]s) OR c.contypid = ANY (%[2]s))
	UNION ALL
	SELECT pg_catalog.format('the index %%s', i.indexrelid::pg_catalog.regclass),
		pg_catalog.concat_ws(' ', i.indexprs::text, i.indpred::text)
	FROM pg_catalog.pg_index i
	WHERE i.indrelid = ANY (%[1]s) OR i.indexrelid = ANY (%[1]s)
	UNION ALL
	SELECT CASE WHEN r.rulename = '_RETURN' THEN pg_catalog.format('the view %%s', r.ev_class::pg_catalog.regclass)
		ELSE pg_catalog.format('the rule %%I on %%s', r.rulename, r.ev_class::pg_catalog.regclass) END, r.ev_action::text
	FROM pg_catalog.pg_rewrite r
	WHERE r.ev_class = ANY (%[1]s)
), refs(object, fn) AS (
	SELECT e.object, m[1]::oid
	FROM exprs e, pg_catalog.regexp_matches(e.tree, ':(?:funcid|aggfnoid|winfnoid|opfuncid) (\d+)', 'g') m
	UNION
	SELECT e.object, o.oprcode::oid
	FROM exprs e, pg_catalog.regexp_matches(e.tree, ':opno (\d+)', 'g') m
	JOIN pg_catalog.pg_operator o ON o.oid = m[1]::oid
)
SELECT DISTINCT r.object, p.oid::pg_catalog.regprocedure::text
FROM refs r JOIN pg_catalog.pg_proc p ON p.oid = r.fn
WHERE p.pronamespace <> 'pg_catalog'::pg_catalog.regnamespace
   OR (p.provolatile = 'v' AND p.proname <> ALL (%[3]s))
   OR p.proname = ANY (%[4]s)
ORDER BY 1, 2`, rels, types, sqlTextArray(allowedVolatile), sqlTextArray(sqlRunners))
}

// guardFunctionSQL is the definition of CodeGuardFunction. It returns at once
// for any session but the IDE login's, so the apiserver's own DDL, a
// pipeline's included, costs one catalog lookup.
func guardFunctionSQL() string {
	return `CREATE OR REPLACE FUNCTION ` + CodeGuardFunction + `() RETURNS event_trigger
LANGUAGE plpgsql SET search_path = pg_catalog, pg_temp AS $jets_guard$
DECLARE
	refused CONSTANT text[] := ` + sqlTextArray(RefusedCommands) + `;
	cmd record;
	rels oid[] := '{}';
	types oid[] := '{}';
	found_rel regclass;
	calls text;
	who name;
BEGIN
	SELECT r.rolname INTO who FROM pg_catalog.pg_roles r
	WHERE r.rolname IN (session_user, current_user) AND ` + ideRoleTest("r") + `
	ORDER BY r.rolname = session_user DESC LIMIT 1;
	IF who IS NULL THEN
		RETURN;
	END IF;
	IF tg_tag = ANY (refused) THEN
		RAISE EXCEPTION '% is not allowed for %: the code it creates could run as another database user', tg_tag, who
			USING ERRCODE = 'insufficient_privilege',
			      HINT = 'Functions, procedures, triggers, policies, rules and operators are created by the apiserver''s database user, not from the Query Tool, a runbook, a workspace init script or a report.';
	END IF;
	FOR cmd IN SELECT * FROM pg_catalog.pg_event_trigger_ddl_commands() LOOP
		IF cmd.command_tag = ANY (refused) THEN
			RAISE EXCEPTION '% is not allowed for %: the code it creates could run as another database user', cmd.command_tag, who
				USING ERRCODE = 'insufficient_privilege',
				      HINT = 'Functions, procedures, triggers, policies, rules and operators are created by the apiserver''s database user, not from the Query Tool, a runbook, a workspace init script or a report.';
		END IF;
		IF cmd.classid = 'pg_catalog.pg_class'::regclass THEN
			rels := rels || cmd.objid;
		ELSIF cmd.classid = 'pg_catalog.pg_type'::regclass THEN
			types := types || cmd.objid;
		ELSIF cmd.classid = 'pg_catalog.pg_constraint'::regclass THEN
			SELECT rels || c.conrelid, types || c.contypid INTO rels, types
			FROM pg_catalog.pg_constraint c WHERE c.oid = cmd.objid;
		END IF;
	END LOOP;
	SELECT c.oid::regclass INTO found_rel FROM pg_catalog.pg_class c
	WHERE c.oid = ANY (rels) AND (c.relrowsecurity OR c.relforcerowsecurity) LIMIT 1;
	IF found_rel IS NOT NULL THEN
		RAISE EXCEPTION 'row-level security on % is not allowed for %: its policies would run as whoever reads the table', found_rel, who
			USING ERRCODE = 'insufficient_privilege',
			      HINT = 'ALTER TABLE ... DISABLE ROW LEVEL SECURITY, NO FORCE ROW LEVEL SECURITY.';
	END IF;
	SELECT pg_catalog.string_agg(q.object || ' calls ' || q.function, '; ') INTO calls
	FROM (` + callsQuery("rels", "types") + `) AS q(object, function);
	IF calls IS NOT NULL THEN
		RAISE EXCEPTION 'not allowed for %: %', who, calls
			USING ERRCODE = 'insufficient_privilege',
			      DETAIL = 'A default, a check, an index expression or a view runs as whoever reads or writes the table, and these functions run SQL, read a table by name or change something beyond the row.',
			      HINT = 'Call the function in the statement itself instead, or use a built-in function.';
	END IF;
END
$jets_guard$`
}

// guardVersion identifies guardFunctionSQL, so that Ensure can tell an
// up-to-date guard it may not replace (a superuser's) from an outdated one.
func guardVersion() string {
	sum := sha256.Sum256([]byte(guardFunctionSQL()))
	return "jetstore code guard " + hex.EncodeToString(sum[:6])
}

// Status is what Ensure found and did about the code guard.
type Status struct {
	// CodeGuard is true when CodeGuardTrigger is installed and enabled.
	CodeGuard bool
	// CodeGuardProblem says why the guard is off, or what is wrong with it
	// when it is on (an outdated function Ensure may not replace). Empty when
	// all is well.
	CodeGuardProblem string
	// IDECode lists what the IDE login already owns that runs as another
	// user: functions, triggers, policies, rules, row-level security, and the
	// stored expressions that call a refused function. Ensure reports it and
	// drops nothing.
	IDECode []string
	// SwitchedOff is true when CodeGuardSwitch asked for the guard to be off.
	// CodeGuard then says whether it still is on, and CodeGuardProblem why.
	SwitchedOff bool
	// Removal says how Ensure turned the guard off: dropped, disabled, or
	// its function emptied. Empty when there was nothing to remove.
	Removal string

	logins string // the start-up line's first half
}

// installCodeGuard brings CodeGuardFunction up to date and creates or enables
// CodeGuardTrigger. Each step runs in a savepoint: one that fails for want of
// privilege is rolled back and reported in Status, and the transaction goes
// on. The error is only for a transaction that can no longer go on.
func installCodeGuard(ctx context.Context, tx pgx.Tx) (Status, error) {
	var st Status
	version := guardVersion()
	err := inSavepoint(ctx, tx, func(sp pgx.Tx) error {
		if _, err := sp.Exec(ctx, guardFunctionSQL()); err != nil {
			return err
		}
		// No one calls it but the event trigger, which needs no EXECUTE.
		if _, err := sp.Exec(ctx, "REVOKE ALL ON FUNCTION "+CodeGuardFunction+"() FROM PUBLIC"); err != nil {
			return err
		}
		_, err := sp.Exec(ctx, fmt.Sprintf("COMMENT ON FUNCTION %s() IS %s", CodeGuardFunction, sqlText(version)))
		return err
	})
	if err != nil {
		if !isPrivilegeError(err) {
			return st, fmt.Errorf("while installing %s: %w", CodeGuardFunction, err)
		}
		// Another user owns it: a superuser who installed the guard by hand.
		var current *string
		if err2 := tx.QueryRow(ctx, "SELECT pg_catalog.obj_description(to_regprocedure($1), 'pg_proc')",
			CodeGuardFunction+"()").Scan(&current); err2 != nil {
			return st, fmt.Errorf("while reading the version of %s: %w", CodeGuardFunction, err2)
		}
		if current == nil || *current != version {
			st.CodeGuardProblem = fmt.Sprintf("%s is owned by another user and is not this release's (%v); its owner may give it to the apiserver's database user for one start, which brings it up to date, and take it back", CodeGuardFunction, err)
		}
	}

	var enabled string
	var function string
	err = tx.QueryRow(ctx, `SELECT e.evtenabled::text, e.evtfoid::pg_catalog.regprocedure::text
		FROM pg_catalog.pg_event_trigger e WHERE e.evtname = $1`, CodeGuardTrigger).Scan(&enabled, &function)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		err = inSavepoint(ctx, tx, func(sp pgx.Tx) error {
			_, err := sp.Exec(ctx, fmt.Sprintf("CREATE EVENT TRIGGER %s ON ddl_command_end EXECUTE FUNCTION %s()",
				ident(CodeGuardTrigger), CodeGuardFunction))
			return err
		})
		if err != nil {
			if !isPrivilegeError(err) {
				return st, fmt.Errorf("while creating the event trigger %s: %w", CodeGuardTrigger, err)
			}
			st.CodeGuardProblem = fmt.Sprintf("the database user may not create event triggers (%v)", err)
			return st, nil
		}
	case err != nil:
		return st, fmt.Errorf("while looking for the event trigger %s: %w", CodeGuardTrigger, err)
	case function != CodeGuardFunction+"()":
		st.CodeGuardProblem = fmt.Sprintf("an event trigger named %s runs %s, not %s()", CodeGuardTrigger, function, CodeGuardFunction)
		return st, nil
	case enabled != "O" && enabled != "A":
		err = inSavepoint(ctx, tx, func(sp pgx.Tx) error {
			_, err := sp.Exec(ctx, fmt.Sprintf("ALTER EVENT TRIGGER %s ENABLE", ident(CodeGuardTrigger)))
			return err
		})
		if err != nil {
			if !isPrivilegeError(err) {
				return st, fmt.Errorf("while enabling the event trigger %s: %w", CodeGuardTrigger, err)
			}
			st.CodeGuardProblem = fmt.Sprintf("the event trigger %s is disabled and the database user may not enable it (%v)", CodeGuardTrigger, err)
			return st, nil
		}
	}
	st.CodeGuard = true
	return st, nil
}

// codeGuardSwitchedOff is true when CodeGuardSwitch is "off", in any case.
func codeGuardSwitchedOff() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(CodeGuardSwitch)), "off")
}

// removeCodeGuard turns the guard off as the apiserver's user, by the first of
// these it may do: drop CodeGuardTrigger, disable it, or empty
// CodeGuardFunction so the trigger does nothing. The first two need the
// trigger's owner, the third the function's. Each runs in a savepoint, and
// none of their failures is an error: the start goes on, and Status says
// whether the guard is still on.
func removeCodeGuard(ctx context.Context, tx pgx.Tx) (Status, error) {
	st := Status{SwitchedOff: true}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_event_trigger WHERE evtname = $1)",
		CodeGuardTrigger).Scan(&exists); err != nil {
		return st, fmt.Errorf("while looking for the event trigger %s: %w", CodeGuardTrigger, err)
	}
	if !exists {
		return st, nil
	}
	steps := []struct{ done, sql string }{
		{"dropped", fmt.Sprintf("DROP EVENT TRIGGER %s", ident(CodeGuardTrigger))},
		{"disabled", fmt.Sprintf("ALTER EVENT TRIGGER %s DISABLE", ident(CodeGuardTrigger))},
		{"emptied its function", "CREATE OR REPLACE FUNCTION " + CodeGuardFunction +
			"() RETURNS event_trigger LANGUAGE plpgsql AS $jets_guard$ BEGIN END $jets_guard$; " +
			"COMMENT ON FUNCTION " + CodeGuardFunction + "() IS " + sqlText(neutralised)},
	}
	var refusals []string
	for _, step := range steps {
		err := inSavepoint(ctx, tx, func(sp pgx.Tx) error {
			_, err := sp.Exec(ctx, step.sql)
			return err
		})
		if err == nil {
			st.Removal = step.done
			return st, nil
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			return st, fmt.Errorf("while turning the code guard off: %w", err)
		}
		refusals = append(refusals, err.Error())
	}
	st.CodeGuard = true
	st.CodeGuardProblem = fmt.Sprintf("%s=off could not remove it: the database user may neither drop nor disable the event trigger %s nor replace %s() (%s); a superuser must run DROP EVENT TRIGGER %s",
		CodeGuardSwitch, CodeGuardTrigger, CodeGuardFunction, strings.Join(refusals, "; "), CodeGuardTrigger)
	return st, nil
}

// CodeGuard reports whether CodeGuardTrigger is installed and enabled in the
// database q is connected to, with a function that does something.
func CodeGuard(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) (bool, error) {
	var on bool
	err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_catalog.pg_event_trigger
		WHERE evtname = $1 AND evtenabled IN ('O', 'A') AND evtfoid = to_regprocedure($2)
		  AND pg_catalog.obj_description(evtfoid, 'pg_proc') IS DISTINCT FROM $3)`,
		CodeGuardTrigger, CodeGuardFunction+"()", neutralised).Scan(&on)
	if err != nil {
		return false, fmt.Errorf("while looking for the event trigger %s: %w", CodeGuardTrigger, err)
	}
	return on, nil
}

// ideCodeQuery lists what the IDE logins own that runs as another user.
func ideCodeQuery() string {
	return `
WITH ide AS (SELECT r.oid FROM pg_catalog.pg_roles r WHERE ` + ideRoleTest("r") + `),
ide_rels AS (SELECT c.oid FROM pg_catalog.pg_class c WHERE c.relowner IN (SELECT oid FROM ide))
SELECT pg_catalog.format('%s %s', CASE p.prokind WHEN 'p' THEN 'procedure' WHEN 'a' THEN 'aggregate' ELSE 'function' END,
	p.oid::pg_catalog.regprocedure)
FROM pg_catalog.pg_proc p WHERE p.proowner IN (SELECT oid FROM ide)
UNION ALL
SELECT pg_catalog.format('trigger %I on %s', t.tgname, t.tgrelid::pg_catalog.regclass)
FROM pg_catalog.pg_trigger t WHERE NOT t.tgisinternal AND t.tgrelid IN (SELECT oid FROM ide_rels)
UNION ALL
SELECT pg_catalog.format('policy %I on %s', p.polname, p.polrelid::pg_catalog.regclass)
FROM pg_catalog.pg_policy p WHERE p.polrelid IN (SELECT oid FROM ide_rels)
UNION ALL
SELECT pg_catalog.format('rule %I on %s', r.rulename, r.ev_class::pg_catalog.regclass)
FROM pg_catalog.pg_rewrite r WHERE r.rulename <> '_RETURN' AND r.ev_class IN (SELECT oid FROM ide_rels)
UNION ALL
SELECT pg_catalog.format('row-level security on %s', c.oid::pg_catalog.regclass)
FROM pg_catalog.pg_class c WHERE (c.relrowsecurity OR c.relforcerowsecurity) AND c.oid IN (SELECT oid FROM ide_rels)
UNION ALL
SELECT pg_catalog.format('operator %s', o.oid::pg_catalog.regoperator)
FROM pg_catalog.pg_operator o WHERE o.oprowner IN (SELECT oid FROM ide)
UNION ALL
SELECT q.object || ' calls ' || q.function FROM (` + callsQuery(
		"ARRAY(SELECT oid FROM ide_rels)",
		"ARRAY(SELECT t.oid FROM pg_catalog.pg_type t WHERE t.typowner IN (SELECT oid FROM ide))") + `) AS q(object, function)
ORDER BY 1`
}

// ideCode lists what the IDE logins own that runs as another user.
func ideCode(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}) ([]string, error) {
	rows, err := q.Query(ctx, ideCodeQuery())
	if err != nil {
		return nil, fmt.Errorf("while listing the code %s owns: %w", IDE, err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("while listing the code %s owns: %w", IDE, err)
	}
	return found, nil
}

// inSavepoint runs f in a savepoint of tx, rolled back if f fails.
func inSavepoint(ctx context.Context, tx pgx.Tx, f func(pgx.Tx) error) error {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	if err = f(sp); err != nil {
		if err2 := sp.Rollback(ctx); err2 != nil {
			return fmt.Errorf("%w; and rolling back: %v", err, err2)
		}
		return err
	}
	return sp.Commit(ctx)
}

// isPrivilegeError is true for the errors a user that is not a superuser, or
// not the owner, gets: insufficient_privilege (42501).
func isPrivilegeError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}

// sqlText is s as an SQL string literal.
func sqlText(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// sqlTextArray is names as an SQL text[] literal.
func sqlTextArray(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = sqlText(n)
	}
	return "ARRAY[" + strings.Join(quoted, ", ") + "]::text[]"
}
