package datatable

// The SQL a user sends runs on the two restricted logins (restricted_logins.go,
// package dblogin). This test pins what each may do, through the handlers a
// request reaches, and that a statement cannot switch back to the apiserver's
// own role: the escape that a SET LOCAL ROLE on the apiserver's login allowed.
//
// Skipped unless JETS_TEST_DSN names a throwaway PostgreSQL whose user may
// create roles and databases (see reader_role_test.go):
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5472/postgres go test ./datatable -run RestrictedLogins -count=1

import (
	"context"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
)

func TestRestrictedLoginsHoldAgainstTheSQLTheyRun(t *testing.T) {
	pool := jm095Pool(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"users", "roles", "role_capability", "secret_rotation", "source_config"} {
		if err := defs[name].UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	for _, s := range []string{
		`INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles, git_token) VALUES
			('reader@test', 'Reader', 'hash-r', 1, '{reader}', 'token-r'),
			('ke@test', 'KE', 'hash-k', 1, '{knowledge_engineer}', 'token-k')`,
		`INSERT INTO jetsapi.roles (role, details) VALUES ('reader', 'r'), ('knowledge_engineer', 'k')`,
		`INSERT INTO jetsapi.role_capability (role, capability) VALUES
			('reader', 'jetstore_read'),
			('knowledge_engineer', 'jetstore_read'), ('knowledge_engineer', 'workspace_ide'),
			('knowledge_engineer', 'run_pipelines')`,
		`INSERT INTO jetsapi.secret_rotation (secret, last_update) VALUES ('s3cret', now())`,
		`INSERT INTO jetsapi.source_config (object_type, client, org, automated, table_name, domain_keys_json, user_email)
			VALUES ('Eligibility', 'ACME', '', 0, 'acme_eligibility', '{}', 'ke@test')`,
		// Tables the apiserver's user owns, as the loader's and the compile's are.
		`CREATE TABLE public.domain_probe (key integer, name text)`,
		`INSERT INTO public.domain_probe VALUES (1, 'a')`,
		`CREATE TABLE public.acme_eligibility (key integer)`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatalf("setting up: %v\n%s", err, s)
		}
	}
	if err := EnsureRestrictedLogins(bg, pool); err != nil {
		t.Fatalf("EnsureRestrictedLogins: %v", err)
	}
	t.Cleanup(CloseRestrictedLogins)

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = "admin@test", "jm150-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })
	readerToken, err := user.CreateToken("reader@test")
	if err != nil {
		t.Fatal(err)
	}
	keToken, err := user.CreateToken("ke@test")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &DataTableContext{Dbpool: pool}

	read := func(sql string) error {
		_, _, err := ctx.ExecRawQuery(&DataTableAction{Action: "raw_query", RawQuery: sql}, readerToken, CapabilityReadData)
		return err
	}
	queryTool := func(sql string) error {
		_, _, err := ctx.ExecRawQuery(&DataTableAction{Action: "raw_query_tool", RawQuery: sql}, keToken, CapabilityQueryTool)
		return err
	}
	submitDDL := func(sql string) error {
		_, _, err := ctx.ExecDataManagementStatement(&DataTableAction{Action: "exec_ddl", RawQuery: sql}, keToken)
		return err
	}
	dropTable := func(schema, table string) error {
		_, _, err := ctx.DropTable(&DataTableAction{Action: "drop_table",
			Data: []map[string]any{{"schemaName": schema, "tableName": table}}}, keToken)
		return err
	}
	refused := func(what string, err error, want string) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err %v, want %q", what, err, want)
		}
		for _, secret := range []string{"hash-r", "hash-k", "token-", "s3cret"} {
			if err != nil && strings.Contains(err.Error(), secret) {
				t.Errorf("%s: the error carries a secret: %v", what, err)
			}
		}
	}
	allowed := func(what string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s: %v", what, err)
		}
	}
	rolesOf := func(email string) string {
		t.Helper()
		var roles string
		if err := pool.QueryRow(bg, `SELECT encrypted_roles::text FROM jetsapi.users WHERE user_email = $1`, email).Scan(&roles); err != nil {
			t.Fatal(err)
		}
		return roles
	}

	// The escape: a statement switching back to the apiserver's role. The
	// login is a member of nothing it could switch to.
	const owner = "'" + jm095Owner + "'"
	refused("a read switching to the apiserver's role",
		read(`DO $$ BEGIN PERFORM set_config('role', `+owner+`, true); END $$`), "permission denied to set role")
	refused("a read resetting its role, then reading a password",
		read(`DO $$ BEGIN PERFORM set_config('role', 'none', true);
			RAISE EXCEPTION 'leak: %', (SELECT string_agg(password, ',') FROM jetsapi.users); END $$`), "permission denied")
	refused("a read through query_to_xml after resetting its role",
		read(`SELECT set_config('role', 'none', true), query_to_xml('SELECT git_token FROM jetsapi.users', true, false, '')`),
		"permission denied")
	refused("the Query Tool switching to the apiserver's role",
		queryTool(`DO $$ BEGIN PERFORM set_config('role', `+owner+`, true); END $$`), "permission denied to set role")
	refused("Submit DDL resetting its role, then granting itself a role",
		submitDDL(`DO $$ BEGIN PERFORM set_config('role', 'none', true);
			UPDATE jetsapi.users SET encrypted_roles = '{designated_admin}' WHERE user_email = 'ke@test'; END $$`),
		"permission denied")

	// The security tables: read, never written, from either free-SQL path.
	refused("the Query Tool updating a user's roles",
		queryTool(`UPDATE jetsapi.users SET encrypted_roles = '{designated_admin}' WHERE user_email = 'ke@test'`), "permission denied")
	refused("Submit DDL granting a capability",
		submitDDL(`INSERT INTO jetsapi.role_capability (role, capability) VALUES ('knowledge_engineer', 'user_admin')`), "permission denied")
	refused("Submit DDL deleting a role", submitDDL(`DELETE FROM jetsapi.roles`), "permission denied")
	refused("Submit DDL truncating role_capability", submitDDL(`TRUNCATE jetsapi.role_capability`), "permission denied")
	refused("the Query Tool reading a password", queryTool(`SELECT password FROM jetsapi.users`), "permission denied")
	refused("the Query Tool reading the logins' passwords", queryTool(`SELECT * FROM jetsapi.restricted_login`), "permission denied")
	refused("a read of the logins' passwords", read(`SELECT * FROM jetsapi.restricted_login`), "permission denied")
	allowed("the Query Tool reading users' other columns", queryTool(`SELECT user_email, encrypted_roles FROM jetsapi.users`))
	allowed("the Query Tool reading role_capability", queryTool(`SELECT role, capability FROM jetsapi.role_capability`))
	if got := rolesOf("ke@test"); got != "{knowledge_engineer}" {
		t.Errorf("ke@test's roles are %s after the refusals", got)
	}

	// What it may not create or change the definition of.
	refused("Submit DDL creating a table in jetsapi", submitDDL(`CREATE TABLE jetsapi.ide_probe (a integer)`), "permission denied")
	refused("Submit DDL dropping a table it did not create", submitDDL(`DROP TABLE public.domain_probe`), "must be owner")
	refused("the hint names the login", submitDDL(`DROP TABLE public.domain_probe`), IDERole)
	allowed("Submit DDL creating a trigger function", submitDDL(`CREATE FUNCTION public.ide_trigger() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$`))
	refused("Submit DDL putting a trigger on the apiserver's table",
		submitDDL(`CREATE TRIGGER ide_trigger BEFORE INSERT ON public.domain_probe FOR EACH ROW EXECUTE FUNCTION public.ide_trigger()`),
		"permission denied")

	// What the Query Tool, Submit DDL and the workspace scripts are for.
	allowed("the Query Tool updating a domain table", queryTool(`UPDATE public.domain_probe SET name = 'b'`))
	allowed("Submit DDL truncating a domain table", submitDDL(`TRUNCATE public.domain_probe`))
	allowed("Submit DDL writing jetsapi configuration",
		submitDDL(`UPDATE jetsapi.source_config SET automated = 1 WHERE table_name = 'acme_eligibility'`))
	allowed("a workspace script's setval",
		queryTool(`SELECT setval(pg_get_serial_sequence('jetsapi.source_config', 'key'), 100)`))
	allowed("Submit DDL creating a table", submitDDL(`CREATE TABLE public.ide_probe (a integer)`))
	allowed("Submit DDL writing its table", submitDDL(`INSERT INTO public.ide_probe VALUES (1)`))
	allowed("a read of the table it created", read(`SELECT a FROM public.ide_probe`))
	allowed("Submit DDL dropping the table it created", submitDDL(`DROP TABLE public.ide_probe`))
	allowed("Submit DDL creating a table to keep", submitDDL(`CREATE TABLE public.ide_kept (a integer)`))
	if _, err = pool.Exec(bg, `DROP TABLE public.ide_kept`); err != nil {
		t.Errorf("the apiserver's user dropping a table the IDE login created: %v", err)
	}

	// Drop Staging Table: on the apiserver's own pool, staging tables only.
	refused("Drop Staging Table naming jetsapi.users", dropTable("jetsapi", "users"), "schema public")
	refused("Drop Staging Table naming a domain table", dropTable("public", "domain_probe"), "not the staging table")
	allowed("Drop Staging Table dropping a staging table the apiserver's user owns", dropTable("public", "acme_eligibility"))
	var gone bool
	if err = pool.QueryRow(bg, `SELECT to_regclass('public.acme_eligibility') IS NULL`).Scan(&gone); err != nil || !gone {
		t.Errorf("the staging table is still there (%v)", err)
	}
	var users int
	if err = pool.QueryRow(bg, `SELECT count(*) FROM jetsapi.users`).Scan(&users); err != nil || users != 2 {
		t.Errorf("jetsapi.users has %d rows (%v)", users, err)
	}
}

// Ensure refuses a login that holds more than its group, and runs again
// cleanly: it runs at every apiserver start and every update_db.
func TestRestrictedLoginsRefuseAPrivilegedLogin(t *testing.T) {
	pool := jm095Pool(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA jetsapi"); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRestrictedLogins(bg, pool); err != nil {
		t.Fatalf("EnsureRestrictedLogins: %v", err)
	}
	t.Cleanup(CloseRestrictedLogins)
	var login string
	if err := pool.QueryRow(bg, `SELECT login_role FROM jetsapi.restricted_login WHERE login_role LIKE 'jets_ide_%'`).Scan(&login); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(bg, "CREATE ROLE jm150_extra NOLOGIN; GRANT jm150_extra TO "+login); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP ROLE IF EXISTS jm150_extra") })
	err := EnsureRestrictedLogins(bg, pool)
	if err == nil || !strings.Contains(err.Error(), "jm150_extra") {
		t.Errorf("a login that is a member of another role: err %v", err)
	}
	if _, err = pool.Exec(bg, "REVOKE jm150_extra FROM "+login); err != nil {
		t.Fatal(err)
	}
	if err = EnsureRestrictedLogins(bg, pool); err != nil {
		t.Errorf("EnsureRestrictedLogins after the repair: %v", err)
	}
}
