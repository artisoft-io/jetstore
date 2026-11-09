package datatable

// User administration on the user_admin capability (2026-11-07): update/users and
// delete/users moved off AdminOnly, the designated_admin role that holds it, and
// ProtectAdminRow, which keeps both statements off the installation admin's own row
// whoever calls them.
//
// The first tests are pure. The ones named TestUserAdminDB* need a PostgreSQL and
// are skipped unless JETS_TEST_DSN names one. They create a database of their own,
// srg_user_admin, build users, roles and role_capability from jets_schema.json, and
// seed the two role tables from the shipped jets_init_db.sql rather than from rows
// written here, so they test the roles a deployment gets. Locally:
//
//	docker run -d --rm --name srg-b-pg -e POSTGRES_PASSWORD=pw -p 5482:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5482/postgres \
//	  go test -count=1 -run 'UserAdmin|WritesAdminRow|AdminRow' ./jets/datatable/

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestUserStatementsGateOnUserAdmin(t *testing.T) {
	for _, name := range []string{"update/users", "delete/users"} {
		stmt, ok := sqlInsertStmts[name]
		if !ok {
			t.Fatalf("sqlInsertStmts has no %s", name)
		}
		if stmt.Capability != "user_admin" {
			t.Errorf("%s requires %q, want user_admin", name, stmt.Capability)
		}
		if stmt.AdminOnly {
			t.Errorf("%s is AdminOnly; a role holding user_admin could never pass it", name)
		}
		if !stmt.ProtectAdminRow {
			t.Errorf("%s does not set ProtectAdminRow; a user administrator could lock the admin out", name)
		}
		if !contains(stmt.ColumnKeys, "user_email") {
			t.Errorf("%s has no user_email column key, which is what ProtectAdminRow reads", name)
		}
	}
}

// Every statement that protects the admin's row must carry user_email, or the check
// reads nothing and passes everything.
func TestProtectAdminRowStatementsCarryUserEmail(t *testing.T) {
	for name, stmt := range sqlInsertStmts {
		if stmt.ProtectAdminRow && !contains(stmt.ColumnKeys, "user_email") {
			t.Errorf("%s sets ProtectAdminRow and has no user_email column key", name)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestWritesAdminRow(t *testing.T) {
	const admin = "admin"
	cases := []struct {
		name  string
		rows  []map[string]any
		admin string
		want  bool
	}{
		{"no rows", nil, admin, false},
		{"another user", []map[string]any{{"user_email": "a@b.test"}}, admin, false},
		{"the admin", []map[string]any{{"user_email": admin}}, admin, true},
		{"the admin after another", []map[string]any{{"user_email": "a@b.test"}, {"user_email": admin}}, admin, true},
		{"case differs, as the statement's WHERE would", []map[string]any{{"user_email": "Admin"}}, admin, false},
		{"no user_email", []map[string]any{{"is_active": 1}}, admin, false},
		{"nil user_email", []map[string]any{{"user_email": nil}}, admin, false},
		{"not a string", []map[string]any{{"user_email": 42}}, "42", true},
		{"no admin configured", []map[string]any{{"user_email": ""}}, "", false},
	}
	for _, c := range cases {
		if got := writesAdminRow(c.rows, c.admin); got != c.want {
			t.Errorf("%s: writesAdminRow = %v, want %v", c.name, got, c.want)
		}
	}
}

// The refusal is an answer about the row, not the caller: a 403 with its own text,
// and not the collapsed ErrRefused a missing capability gets.
func TestAdminRowRefusalHasItsOwnMessage(t *testing.T) {
	if errors.Is(ErrAdminRowProtected, ErrRefused) || ErrAdminRowProtected.Error() == ErrRefused.Error() {
		t.Error("ErrAdminRowProtected reads as a capability refusal")
	}
	if !strings.Contains(ErrAdminRowProtected.Error(), "admin") {
		t.Errorf("ErrAdminRowProtected does not say whose row: %q", ErrAdminRowProtected)
	}
}

// The refusal happens before InsertRows writes anything: the ProtectAdminRow check
// precedes the first statement of InsertRows that reaches the database.
//
// It inspects the AST rather than rendering it: ast.Fprint follows identifiers back
// to their declarations, so a rendered statement naming ctx also prints the
// receiver's type and its Dbpool field (see TestNoGateHardCodesTheUnauthorizedStatus).
func TestInsertRowsChecksTheAdminRowBeforeWriting(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "data_table_action.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "InsertRows" || fn.Recv == nil {
			continue
		}
		checkAt, effectAt := -1, -1
		for i, stmt := range fn.Body.List {
			ast.Inspect(stmt, func(n ast.Node) bool {
				var name string
				switch x := n.(type) {
				case *ast.SelectorExpr:
					name = x.Sel.Name
				case *ast.Ident:
					name = x.Name
				default:
					return true
				}
				if checkAt < 0 && name == "writesAdminRow" {
					checkAt = i
				}
				if effectAt < 0 && (name == "Dbpool" || name == "InsertPipelineExecutionStatus") {
					effectAt = i
				}
				return true
			})
		}
		if checkAt < 0 {
			t.Fatal("InsertRows never calls writesAdminRow")
		}
		if effectAt < 0 {
			t.Fatal("InsertRows reaches the database nowhere this test can see; the guard has gone blind")
		}
		if effectAt >= 0 && effectAt < checkAt {
			t.Errorf("InsertRows reaches the database at statement %d and checks the admin's row at %d", effectAt, checkAt)
		}
		return
	}
	t.Fatal("InsertRows not found")
}

// ----------------------------------------------------------------------------
// Against PostgreSQL.

const (
	uaAdmin    = "admin@ua.test"
	uaDesig    = "designated@ua.test"
	uaEngineer = "engineer@ua.test"
	uaOther    = "other@ua.test"
	uaOther2   = "other2@ua.test"
)

type uaFixture struct {
	ctx    *DataTableContext
	pool   *pgxpool.Pool
	tokens map[string]string
}

func uaDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set; see the header of this file")
	}
	bg := context.Background()
	admin, err := pgxpool.New(bg, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	var n int
	if err = admin.QueryRow(bg, "SELECT count(*) FROM pg_database WHERE datname = 'srg_user_admin'").Scan(&n); err != nil {
		admin.Close()
		t.Fatalf("looking for the test database: %v", err)
	}
	if n == 0 {
		if _, err = admin.Exec(bg, "CREATE DATABASE srg_user_admin"); err != nil {
			admin.Close()
			t.Fatalf("creating the test database: %v", err)
		}
	}
	admin.Close()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/srg_user_admin"
	pool, err := pgxpool.New(bg, u.String())
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seededRoleBlock returns the part of the shipped jets_init_db.sql that fills
// jetsapi.roles and jetsapi.role_capability, from the first TRUNCATE to the end of
// the role_capability INSERT.
func seededRoleBlock(t *testing.T) string {
	t.Helper()
	buf, err := os.ReadFile("../jets_init_db.sql")
	if err != nil {
		t.Fatalf("reading ../jets_init_db.sql: %v", err)
	}
	script := string(buf)
	start := strings.Index(script, "TRUNCATE jetsapi.roles;")
	insert := strings.Index(script, "INSERT INTO jetsapi.role_capability")
	if start < 0 || insert < start {
		t.Fatal("jets_init_db.sql: the roles and role_capability blocks were not found in order")
	}
	end := strings.Index(script[insert:], "\n;")
	if end < 0 {
		t.Fatal("jets_init_db.sql: the role_capability INSERT has no terminating ';' line")
	}
	return script[start : insert+end+2]
}

func newUAFixture(t *testing.T) *uaFixture {
	t.Helper()
	pool := uaDB(t)
	bg := context.Background()
	if _, err := pool.Exec(bg, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatal(err)
	}
	defs := rutTableDefs(t)
	for _, name := range []string{"users", "roles", "role_capability"} {
		if _, err := pool.Exec(bg, "DROP TABLE IF EXISTS jetsapi."+name); err != nil {
			t.Fatal(err)
		}
		def, ok := defs[name]
		if !ok {
			t.Fatalf("jets_schema.json carries no definition for jetsapi.%s", name)
		}
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("creating jetsapi.%s: %v", name, err)
		}
	}
	// No bind arguments, so the block goes as one simple-protocol script.
	if _, err := pool.Exec(bg, seededRoleBlock(t)); err != nil {
		t.Fatalf("running the role blocks of jets_init_db.sql: %v", err)
	}

	previousAdmin, previousSecret := user.AdminEmail, user.ApiSecret
	user.AdminEmail, user.ApiSecret = uaAdmin, "srg-user-admin-test"
	t.Cleanup(func() { user.AdminEmail, user.ApiSecret = previousAdmin, previousSecret })

	seed := []struct {
		email string
		roles []string
	}{
		{uaAdmin, []string{}},
		{uaDesig, []string{"designated_admin"}},
		{uaEngineer, []string{"knowledge_engineer"}},
		{uaOther, []string{"ops_user"}},
		{uaOther2, []string{"ops_user"}},
	}
	f := &uaFixture{ctx: &DataTableContext{Dbpool: pool}, pool: pool, tokens: map[string]string{}}
	for _, s := range seed {
		_, err := pool.Exec(bg, `INSERT INTO jetsapi.users (user_email, name, password, is_active, encrypted_roles)
			VALUES ($1, $1, 'x', 1, $2)`, s.email, s.roles)
		if err != nil {
			t.Fatalf("seeding %s: %v", s.email, err)
		}
		tok, err := user.CreateToken(s.email)
		if err != nil {
			t.Fatal(err)
		}
		f.tokens[s.email] = tok
	}
	return f
}

// update sets is_active to 0 and the roles to {ops_user} on each row, as the edit
// dialog does; numbers arrive as float64 from JSON.
func (f *uaFixture) update(caller string, emails ...string) (int, error) {
	data := make([]map[string]any, len(emails))
	for i, e := range emails {
		data[i] = map[string]any{"user_email": e, "is_active": float64(0), "roles": []any{"ops_user"}}
	}
	_, status, err := f.ctx.InsertRows(&DataTableAction{
		Action: "insert_rows", FromClauses: []FromClause{{Table: "update/users"}}, Data: data,
	}, f.tokens[caller])
	return status, err
}

func (f *uaFixture) delete(caller string, emails ...string) (int, error) {
	data := make([]map[string]any, len(emails))
	for i, e := range emails {
		data[i] = map[string]any{"user_email": e}
	}
	_, status, err := f.ctx.InsertRows(&DataTableAction{
		Action: "insert_rows", FromClauses: []FromClause{{Table: "delete/users"}}, Data: data,
	}, f.tokens[caller])
	return status, err
}

// row returns is_active and the roles, and false when the row is gone.
func (f *uaFixture) row(t *testing.T, email string) (int, []string, bool) {
	t.Helper()
	var active int
	var roles []string
	err := f.pool.QueryRow(context.Background(),
		`SELECT is_active, encrypted_roles FROM jetsapi.users WHERE user_email = $1`, email).Scan(&active, &roles)
	if err != nil {
		return 0, nil, false
	}
	return active, roles, true
}

func (f *uaFixture) capabilities(t *testing.T, role string) []string {
	t.Helper()
	rows, err := f.pool.Query(context.Background(),
		`SELECT capability FROM jetsapi.role_capability WHERE role = $1`, role)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// B.1: the seeded designated_admin holds knowledge_engineer's nine capabilities plus
// delete_session and user_admin, eleven in all, and knowledge_engineer holds neither.
func TestUserAdminDBSeededRoles(t *testing.T) {
	f := newUAFixture(t)
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.roles WHERE role = 'designated_admin'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("designated_admin in roles: count %d, err %v", n, err)
	}
	ke := f.capabilities(t, "knowledge_engineer")
	da := f.capabilities(t, "designated_admin")
	sort.Strings(ke)
	sort.Strings(da)
	if len(ke) != 9 {
		t.Errorf("knowledge_engineer holds %d capabilities, want 9: %v", len(ke), ke)
	}
	want := append(append([]string{}, ke...), "delete_session", "user_admin")
	sort.Strings(want)
	if strings.Join(da, ",") != strings.Join(want, ",") {
		t.Errorf("designated_admin holds %v, want %v", da, want)
	}
	if len(da) != 11 {
		t.Errorf("designated_admin holds %d capabilities, want 11", len(da))
	}
	for _, c := range []string{"delete_session", "user_admin"} {
		if contains(ke, c) {
			t.Errorf("knowledge_engineer holds %s", c)
		}
	}
}

func TestUserAdminDBDesignatedAdminWritesOtherUsers(t *testing.T) {
	f := newUAFixture(t)
	if status, err := f.update(uaDesig, uaOther); err != nil || status != http.StatusOK {
		t.Fatalf("update another user: status %d, err %v", status, err)
	}
	active, roles, ok := f.row(t, uaOther)
	if !ok || active != 0 || strings.Join(roles, ",") != "ops_user" {
		t.Errorf("after update: present %v, is_active %d, roles %v", ok, active, roles)
	}
	if status, err := f.delete(uaDesig, uaOther); err != nil || status != http.StatusOK {
		t.Fatalf("delete another user: status %d, err %v", status, err)
	}
	if _, _, ok := f.row(t, uaOther); ok {
		t.Error("the deleted user's row is still there")
	}
}

func TestUserAdminDBDesignatedAdminIsRefusedTheAdminRow(t *testing.T) {
	f := newUAFixture(t)
	for _, c := range []struct {
		name string
		call func() (int, error)
	}{
		{"update", func() (int, error) { return f.update(uaDesig, uaAdmin) }},
		{"delete", func() (int, error) { return f.delete(uaDesig, uaAdmin) }},
		// The admin's row second: nothing of the request is written, not even the
		// first row.
		{"update, admin row second", func() (int, error) { return f.update(uaDesig, uaOther, uaAdmin) }},
		{"delete, admin row second", func() (int, error) { return f.delete(uaDesig, uaOther2, uaAdmin) }},
	} {
		status, err := c.call()
		if status != http.StatusForbidden || !errors.Is(err, ErrAdminRowProtected) {
			t.Errorf("%s: status %d, err %v; want 403 and ErrAdminRowProtected", c.name, status, err)
		}
	}
	if active, _, ok := f.row(t, uaAdmin); !ok || active != 1 {
		t.Errorf("the admin's row: present %v, is_active %d", ok, active)
	}
	if active, roles, ok := f.row(t, uaOther); !ok || active != 1 || strings.Join(roles, ",") != "ops_user" {
		t.Errorf("a refused request wrote its first row: present %v, is_active %d, roles %v", ok, active, roles)
	}
	if _, _, ok := f.row(t, uaOther2); !ok {
		t.Error("a refused request deleted its first row")
	}
}

func TestUserAdminDBKnowledgeEngineerIsRefused(t *testing.T) {
	f := newUAFixture(t)
	if status, err := f.update(uaEngineer, uaOther); status != http.StatusForbidden || !errors.Is(err, ErrRefused) {
		t.Errorf("update: status %d, err %v; want 403 and ErrRefused", status, err)
	}
	if status, err := f.delete(uaEngineer, uaOther); status != http.StatusForbidden || !errors.Is(err, ErrRefused) {
		t.Errorf("delete: status %d, err %v; want 403 and ErrRefused", status, err)
	}
	if active, _, ok := f.row(t, uaOther); !ok || active != 1 {
		t.Errorf("a refused caller changed the row: present %v, is_active %d", ok, active)
	}
}

func TestUserAdminDBAdminWritesOtherRowsNotItsOwn(t *testing.T) {
	f := newUAFixture(t)
	if status, err := f.update(uaAdmin, uaOther); err != nil || status != http.StatusOK {
		t.Fatalf("admin updates another user: status %d, err %v", status, err)
	}
	if status, err := f.delete(uaAdmin, uaOther); err != nil || status != http.StatusOK {
		t.Fatalf("admin deletes another user: status %d, err %v", status, err)
	}
	if status, err := f.update(uaAdmin, uaAdmin); status != http.StatusForbidden || !errors.Is(err, ErrAdminRowProtected) {
		t.Errorf("admin updates its own row: status %d, err %v", status, err)
	}
	if status, err := f.delete(uaAdmin, uaAdmin); status != http.StatusForbidden || !errors.Is(err, ErrAdminRowProtected) {
		t.Errorf("admin deletes its own row: status %d, err %v", status, err)
	}
	if active, _, ok := f.row(t, uaAdmin); !ok || active != 1 {
		t.Errorf("the admin's row: present %v, is_active %d", ok, active)
	}
}

// A role name is now written by a user administrator, not only by the admin, and
// GetUserByEmail binds the roles rather than splicing them into its statement. A
// role carrying a quote is a role nobody holds capabilities for, not SQL.
func TestUserAdminDBARoleNameIsNotSQL(t *testing.T) {
	f := newUAFixture(t)
	hostile := "ops_user') OR ('1'='1"
	if _, err := f.pool.Exec(context.Background(),
		`UPDATE jetsapi.users SET encrypted_roles = $1 WHERE user_email = $2`, []string{hostile}, uaOther); err != nil {
		t.Fatal(err)
	}
	u, err := user.GetUserByEmail(f.pool, uaOther)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if caps := u.GetCapabilities(); len(caps) != 0 {
		t.Errorf("a role named %q conferred %v", hostile, caps)
	}
	// And a real role still resolves.
	u, err = user.GetUserByEmail(f.pool, uaDesig)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if !u.HasCapability("user_admin") || !u.HasCapability("delete_session") {
		t.Errorf("designated_admin resolves to %v", u.GetCapabilities())
	}
}
