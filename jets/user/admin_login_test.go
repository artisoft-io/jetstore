package user

import "testing"

func TestAdminLoginFromEnv(t *testing.T) {
	for _, c := range []struct {
		name, env, want string
	}{
		{"unset or empty is the default", "", DefaultAdminLogin},
		{"the variable, when set", "ops@example.test", "ops@example.test"},
		{"a login that is not an email", "root", "root"},
	} {
		t.Setenv(AdminLoginEnvVar, c.env)
		if got := AdminLoginFromEnv(); got != c.want {
			t.Errorf("%s: AdminLoginFromEnv() = %q, want %q", c.name, got, c.want)
		}
	}
}
