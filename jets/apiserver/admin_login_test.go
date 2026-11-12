package main

import "testing"

// The apiserver and the secret-rotation Lambda must name the same administrator:
// with no -adminEmail flag the apiserver reads JETS_ADMIN_EMAIL, as the Lambda does.
func TestResolveAdminLogin(t *testing.T) {
	for _, c := range []struct {
		name, flag, env, want string
	}{
		{"neither set is admin", "", "", "admin"},
		{"the variable when the flag is not set", "", "ops@example.test", "ops@example.test"},
		{"the flag over the variable", "local@example.test", "ops@example.test", "local@example.test"},
		{"the flag alone", "local@example.test", "", "local@example.test"},
	} {
		t.Setenv("JETS_ADMIN_EMAIL", c.env)
		if got := resolveAdminLogin(c.flag); got != c.want {
			t.Errorf("%s: resolveAdminLogin(%q) = %q, want %q", c.name, c.flag, got, c.want)
		}
	}
}
