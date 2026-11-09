package datatable

// update/user_git_profile writes the caller's own row only (OwnRowOnly). Every
// role holds user_profile, so before this any user could write any user's git
// profile, token included.
//
// The DB tests share user_admin_test.go's fixture and are skipped unless
// JETS_TEST_DSN names a PostgreSQL:
//
//	docker run -d --rm --name jm086-pg -e POSTGRES_PASSWORD=pw -p 5483:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5483/postgres \
//	  go test -count=1 -run 'GitProfile|BindToCaller' ./jets/datatable/

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestGitProfileStatementIsOwnRowOnly(t *testing.T) {
	stmt, ok := sqlInsertStmts["update/user_git_profile"]
	if !ok {
		t.Fatal("sqlInsertStmts has no update/user_git_profile")
	}
	if !stmt.OwnRowOnly {
		t.Error("update/user_git_profile does not set OwnRowOnly; any user could write any user's git profile")
	}
	if stmt.ProtectAdminRow {
		t.Error("update/user_git_profile sets ProtectAdminRow; the admin edits its own profile through it")
	}
	if !contains(stmt.ColumnKeys, "user_email") {
		t.Error("update/user_git_profile has no user_email column for OwnRowOnly to bind")
	}
}

func TestBindToCaller(t *testing.T) {
	rows := []map[string]any{{"user_email": "a@x"}, {"git_name": "no email"}, {"user_email": ""}, {"user_email": nil}}
	if !bindToCaller(rows, "a@x") {
		t.Fatal("refused rows that name the caller or no one")
	}
	for i, r := range rows {
		if r["user_email"] != "a@x" {
			t.Errorf("row %d: user_email %v, want the caller's", i, r["user_email"])
		}
	}
	other := []map[string]any{{"user_email": "a@x"}, {"user_email": "b@x"}}
	if bindToCaller(other, "a@x") {
		t.Error("accepted a row that names another user")
	}
	if other[1]["user_email"] != "b@x" {
		t.Error("a refusal rewrote a row")
	}
	if bindToCaller([]map[string]any{{"user_email": "A@x"}}, "a@x") {
		t.Error("the comparison is not exact")
	}
}

func (f *uaFixture) setGitName(caller, target, name string) (int, error) {
	row := map[string]any{"git_name": name, "git_email": "g@x", "git_handle": "handle", "git_token": ""}
	if target != "" {
		row["user_email"] = target
	}
	_, status, err := f.ctx.InsertRows(&DataTableAction{
		Action: "insert_rows", FromClauses: []FromClause{{Table: "update/user_git_profile"}},
		Data: []map[string]any{row},
	}, f.tokens[caller])
	return status, err
}

func (f *uaFixture) gitName(t *testing.T, email string) string {
	t.Helper()
	var name *string
	if err := f.pool.QueryRow(context.Background(),
		`SELECT git_name FROM jetsapi.users WHERE user_email = $1`, email).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name == nil {
		return ""
	}
	return *name
}

func TestGitProfileDBWritesTheCallersRowOnly(t *testing.T) {
	f := newUAFixture(t)

	// Another user's row is refused, whoever asks, and is left as it was.
	for _, caller := range []string{uaOther, uaDesig, uaAdmin} {
		status, err := f.setGitName(caller, uaOther2, "taken by "+caller)
		if status != http.StatusForbidden || !errors.Is(err, ErrNotOwnRow) {
			t.Errorf("%s writing %s's profile: %d %v, want 403 ErrNotOwnRow", caller, uaOther2, status, err)
		}
	}
	if got := f.gitName(t, uaOther2); got != "" {
		t.Errorf("%s's git_name is %q after the refusals, want it unset", uaOther2, got)
	}

	// The caller's own row is written, named or not.
	if status, err := f.setGitName(uaOther, uaOther, "mine"); status != http.StatusOK || err != nil {
		t.Errorf("%s writing its own profile: %d %v", uaOther, status, err)
	}
	if got := f.gitName(t, uaOther); got != "mine" {
		t.Errorf("%s's git_name %q, want mine", uaOther, got)
	}
	if status, err := f.setGitName(uaOther2, "", "unnamed"); status != http.StatusOK || err != nil {
		t.Errorf("%s writing its own profile with no user_email: %d %v", uaOther2, status, err)
	}
	if got := f.gitName(t, uaOther2); got != "unnamed" {
		t.Errorf("%s's git_name %q, want unnamed", uaOther2, got)
	}

	// The installation admin edits its own profile through it.
	if status, err := f.setGitName(uaAdmin, uaAdmin, "admin's"); status != http.StatusOK || err != nil {
		t.Errorf("the admin writing its own profile: %d %v", status, err)
	}
	if got := f.gitName(t, uaAdmin); got != "admin's" {
		t.Errorf("the admin's git_name %q, want admin's", got)
	}
}
