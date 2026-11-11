// A refused sign-in says the same thing whatever the reason, so the answer does
// not tell whether a login exists. Run against a real Postgres, as
// login_response_test.go is; needs JETS_TEST_DSN.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/artisoft-io/jetstore/jets/user"
	"go.uber.org/zap"
)

func TestLoginRefusalsReadTheSame(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	pool := startupOrderDB(t, "jets_login_refusal")
	for _, s := range []string{
		"CREATE SCHEMA IF NOT EXISTS jetsapi",
		`CREATE TABLE jetsapi.users (
			user_email      text PRIMARY KEY,
			name            text NOT NULL,
			password        text NOT NULL,
			encrypted_roles text[] NOT NULL DEFAULT '{}',
			is_active       int NOT NULL DEFAULT 0,
			git_name        text NOT NULL DEFAULT '',
			git_email       text NOT NULL DEFAULT '',
			git_handle      text NOT NULL DEFAULT '')`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	const password = "a-test-password"
	hash, err := user.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct {
		email, hash string
		active      int
	}{
		{"active@example.test", string(hash), 1},
		{"pending@example.test", string(hash), 0},
		{"broken@example.test", "not-a-bcrypt-hash", 1},
	} {
		if _, err = pool.Exec(ctx, "INSERT INTO jetsapi.users (user_email, name, password, is_active) VALUES ($1, 'U', $2, $3)",
			u.email, u.hash, u.active); err != nil {
			t.Fatal(err)
		}
	}
	previousSecret := user.ApiSecret
	user.ApiSecret = "login-refusal-test"
	t.Cleanup(func() { user.ApiSecret = previousSecret })
	s := &Server{dbpool: pool, AuditLogger: zap.NewNop()}
	signIn := func(email, pw string) (int, string) {
		body, _ := json.Marshal(map[string]string{"user_email": email, "password": pw})
		rec := httptest.NewRecorder()
		s.Login(rec, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body)))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		msg, _ := out["error"].(string)
		return rec.Code, msg
	}

	for _, c := range []struct{ what, email, password string }{
		{"an unknown login", "nobody@example.test", password},
		{"a wrong password", "active@example.test", "wrong-password"},
		{"an inactive account with its password", "pending@example.test", password},
		{"an inactive account with a wrong password", "pending@example.test", "wrong-password"},
		{"a stored value that is not a bcrypt hash", "broken@example.test", password},
	} {
		code, msg := signIn(c.email, c.password)
		if code != http.StatusUnprocessableEntity || msg != errSignInRefused.Error() {
			t.Errorf("%s: %d %q, want %d %q", c.what, code, msg, http.StatusUnprocessableEntity, errSignInRefused.Error())
		}
	}
	if code, msg := signIn("active@example.test", password); code != http.StatusOK {
		t.Errorf("an active account with its password: %d %q", code, msg)
	}
}
