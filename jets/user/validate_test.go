package user

import "testing"

// A login is checked against the database, not against an email format; only
// registration requires an email. rules_unit_test_harness I-32, 2026-10-11: with
// -adminEmail at its default, "admin", the built-in administrator could not sign
// in to the React app, and a login that is not the configured administrator's was
// refused as "invalid Email" before the database was asked.
func TestValidateLoginDoesNotRequireAnEmail(t *testing.T) {
	saved := AdminEmail
	t.Cleanup(func() { AdminEmail = saved })
	AdminEmail = "admin@example.com"

	for _, login := range []string{"admin", "operator", "ada@example.com", "admin@example.com"} {
		u := &User{Email: login, Password: "x"}
		if err := u.Validate("login"); err != nil {
			t.Errorf("Validate(login) refused %q: %v", login, err)
		}
	}
	if err := (&User{Email: "", Password: "x"}).Validate("login"); err == nil {
		t.Error("Validate(login) accepted an empty login")
	}
	if err := (&User{Email: "admin", Password: ""}).Validate("login"); err == nil {
		t.Error("Validate(login) accepted an empty password")
	}
}

func TestValidateRegistrationStillRequiresAnEmail(t *testing.T) {
	saved := AdminEmail
	t.Cleanup(func() { AdminEmail = saved })
	AdminEmail = "admin"

	good := "Correct-Horse-9!"
	if err := (&User{Name: "Ada", Email: "ada@example.com", Password: good}).Validate(""); err != nil {
		t.Fatalf("registration refused a valid user: %v", err)
	}
	if err := (&User{Name: "Ada", Email: "operator", Password: good}).Validate(""); err == nil || err.Error() != "invalid Email" {
		t.Errorf("registration accepted a login that is not an email: %v", err)
	}
}
