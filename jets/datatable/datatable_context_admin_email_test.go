package datatable

// JM-085: NewDataTableContext keeps the admin email it is given and falls back to
// JETS_ADMIN_EMAIL only when it is given none.

import "testing"

func TestNewDataTableContextAdminEmail(t *testing.T) {
	t.Setenv("JETS_ADMIN_EMAIL", "env-admin@example.test")
	flag := "flag-admin@example.test"
	empty := ""
	for _, c := range []struct {
		name string
		in   *string
		want string
	}{
		{"the flag, when given, is kept", &flag, "flag-admin@example.test"},
		{"nil falls back to the environment", nil, "env-admin@example.test"},
		{"empty falls back to the environment", &empty, "env-admin@example.test"},
	} {
		ctx := NewDataTableContext(nil, false, false, nil, c.in)
		if ctx.AdminEmail == nil {
			t.Errorf("%s: AdminEmail is nil, want %q", c.name, c.want)
			continue
		}
		if *ctx.AdminEmail != c.want {
			t.Errorf("%s: AdminEmail %q, want %q", c.name, *ctx.AdminEmail, c.want)
		}
	}
	if flag != "flag-admin@example.test" {
		t.Errorf("the caller's flag value was overwritten: %q", flag)
	}
}
