// The sign-in response, run against a real Postgres.
//
// `Login` reads the user from `jetsapi.users` before it builds its response, so
// the response cannot be reached without a database. This installs the one table
// it reads, seeds one active user, and asks the handler itself -- what the React
// client parses is the JSON this writes, not the map it is built from.
//
// The build information is the first reason for it (jetstore_maintenance_02
// Phase 1, D05): `jetstore_version` and `jetstore_git_sha` are what the UI footer
// renders, and each is an environment variable that is simply absent on a
// workstation, so the test pins both the value and the presence of the key.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally, as for startup_order_test.go:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test -count=1 -run TestLogin ./jets/apiserver/
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

const (
	loginTestEmail    = "footer@example.test"
	loginTestPassword = "a-test-password"
)

// loginResponse signs the seeded user in through the handler and decodes the body.
func loginResponse(t *testing.T) map[string]any {
	t.Helper()
	ctx := context.Background()
	pool := startupOrderDB(t, "jets_login_response")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS jetsapi"); err != nil {
		t.Fatalf("creating the jetsapi schema: %v", err)
	}
	// The columns `user.GetUserByEmail` selects, with the defaults
	// jets/jets_schema.json gives them; nothing else of the table is read.
	if _, err := pool.Exec(ctx, `CREATE TABLE jetsapi.users (
		user_email      text PRIMARY KEY,
		name            text NOT NULL,
		password        text NOT NULL,
		encrypted_roles text[] NOT NULL DEFAULT '{}',
		is_active       int NOT NULL DEFAULT 0,
		git_name        text NOT NULL DEFAULT '',
		git_email       text NOT NULL DEFAULT '',
		git_handle      text NOT NULL DEFAULT '')`); err != nil {
		t.Fatalf("installing jetsapi.users: %v", err)
	}
	hash, err := user.Hash(loginTestPassword)
	if err != nil {
		t.Fatalf("hashing the test password: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO jetsapi.users (user_email, name, password, is_active) VALUES ($1, 'Footer', $2, 1)",
		loginTestEmail, string(hash)); err != nil {
		t.Fatalf("seeding the user: %v", err)
	}

	// CreateToken signs with ApiSecret; any value will do for a token nobody checks.
	previousSecret := user.ApiSecret
	user.ApiSecret = "login-response-test"
	t.Cleanup(func() { user.ApiSecret = previousSecret })

	s := &Server{dbpool: pool, AuditLogger: zap.NewNop()}
	body, _ := json.Marshal(map[string]string{"user_email": loginTestEmail, "password": loginTestPassword})
	rec := httptest.NewRecorder()
	s.Login(rec, httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("login answered %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decoding the login response: %v", err)
	}
	return out
}

// TestLoginReturnsBuildInformation: both build values travel at sign-in, under
// the keys `jetsclient_ide/src/api/client.ts` reads.
func TestLoginReturnsBuildInformation(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	t.Setenv("JETS_VERSION", "1759338000")
	t.Setenv("JETS_GIT_SHA", "jets_ai-6aeb79068")

	out := loginResponse(t)
	if got := out["jetstore_version"]; got != "1759338000" {
		t.Errorf("jetstore_version = %#v, want the JETS_VERSION the image carries", got)
	}
	if got := out["jetstore_git_sha"]; got != "jets_ai-6aeb79068" {
		t.Errorf("jetstore_git_sha = %#v, want the JETS_GIT_SHA the image carries", got)
	}
}

// TestLoginBuildInformationUnset: a workstation apiserver has no JETS_GIT_SHA,
// and the key is still sent, as an empty string -- the client renders the date
// alone rather than reading a missing field as a defect.
func TestLoginBuildInformationUnset(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	t.Setenv("JETS_VERSION", "")
	t.Setenv("JETS_GIT_SHA", "")

	out := loginResponse(t)
	for _, key := range []string{"jetstore_version", "jetstore_git_sha"} {
		got, present := out[key]
		if !present {
			t.Errorf("%s is missing from the login response", key)
		} else if got != "" {
			t.Errorf("%s = %#v, want an empty string", key, got)
		}
	}
}
