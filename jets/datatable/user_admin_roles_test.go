package datatable

// JM-087: update/users refuses malformed roles with 400 instead of panicking the
// handler on an unchecked type assertion. The DB test reuses newUAFixture from
// user_admin_test.go and is skipped unless JETS_TEST_DSN is set.

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestRolesOf(t *testing.T) {
	for _, c := range []struct {
		name string
		in   any
		want []string
		bad  bool
	}{
		{"a JSON array of strings", []any{"ops_user", "designated_admin"}, []string{"ops_user", "designated_admin"}, false},
		{"an empty JSON array", []any{}, []string{}, false},
		{"a Go []string", []string{"ops_user"}, []string{"ops_user"}, false},
		{"a string, not a list", "ops_user", nil, true},
		{"a number in the list", []any{"ops_user", float64(1)}, nil, true},
		{"a null in the list", []any{nil}, nil, true},
		{"an object", map[string]any{"role": "ops_user"}, nil, true},
	} {
		got, err := rolesOf(c.in)
		if c.bad {
			if !errors.Is(err, ErrMalformedRoles) {
				t.Errorf("%s: err %v, want ErrMalformedRoles", c.name, err)
			}
			continue
		}
		if err != nil || strings.Join(got, ",") != strings.Join(c.want, ",") || len(got) != len(c.want) {
			t.Errorf("%s: got %v, err %v; want %v", c.name, got, err, c.want)
		}
	}
}

func TestUserAdminDBMalformedRolesAre400(t *testing.T) {
	f := newUAFixture(t)
	for _, c := range []struct {
		name  string
		roles any
	}{
		{"a string", "ops_user"},
		{"a number in the list", []any{float64(7)}},
		{"an object", map[string]any{"role": "ops_user"}},
	} {
		var status int
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: InsertRows panicked: %v", c.name, r)
				}
			}()
			// The well-formed row first: a malformed second row refuses the whole
			// request, so the first is not written either.
			data := []map[string]any{
				{"user_email": uaOther2, "is_active": float64(0), "roles": []any{"designated_admin"}},
				{"user_email": uaOther, "is_active": float64(0), "roles": c.roles},
			}
			_, status, err = f.ctx.InsertRows(&DataTableAction{
				Action: "insert_rows", FromClauses: []FromClause{{Table: "update/users"}}, Data: data,
			}, f.tokens[uaDesig])
		}()
		if status != http.StatusBadRequest || !errors.Is(err, ErrMalformedRoles) {
			t.Errorf("%s: status %d, err %v; want 400 and ErrMalformedRoles", c.name, status, err)
		}
	}
	for _, email := range []string{uaOther, uaOther2} {
		if active, roles, ok := f.row(t, email); !ok || active != 1 || strings.Join(roles, ",") != "ops_user" {
			t.Errorf("%s after refused requests: present %v, is_active %d, roles %v", email, ok, active, roles)
		}
	}
	// And a well-formed request still writes.
	if status, err := f.update(uaDesig, uaOther); err != nil || status != http.StatusOK {
		t.Fatalf("well-formed update: status %d, err %v", status, err)
	}
}
