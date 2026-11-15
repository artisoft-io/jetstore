package runbook

import (
	"strings"
	"testing"
)

func TestForbiddenStatement(t *testing.T) {
	cases := map[string]string{
		"COMMIT":                               "transaction control",
		"/* x */ commit":                       "transaction control",
		"-- a comment\n  BEGIN":                "transaction control",
		"start transaction":                    "transaction control",
		"PREPARE TRANSACTION 'x'":              "transaction control",
		"SET LOCAL ROLE postgres":              "identity",
		"set session authorization x":          "identity",
		"RESET ROLE":                           "identity",
		"SET TRANSACTION READ WRITE":           "transaction",
		"RESET ALL":                            "settings",
		"COPY public.t TO PROGRAM 'true'":      "COPY",
		"LOAD 'x'":                             "shared library",
		"DISCARD ALL":                          "session",
		"LISTEN x":                             "session",
		"(SELECT 1)":                           "",
		"SELECT 'COMMIT'":                      "",
		"SET LOCAL work_mem = '4MB'":           "",
		"PREPARE q AS SELECT 1":                "",
		"UPDATE t SET role = 'x'":              "",
		"INSERT INTO t VALUES (1)":             "",
		"\"commit\"":                           "",
		"WITH x AS (SELECT 1) SELECT * FROM x": "",
		"/* never closed":                      "",
		"CREATE TABLE copy_of_t (a int)":       "",
		"DO $$ BEGIN PERFORM 1; END $$":        "",
		"SELECT 1; COMMIT":                     "",
	}
	for sql, want := range cases {
		got := ForbiddenStatement(sql)
		if (want == "") != (got == "") || !strings.Contains(got, want) {
			t.Errorf("ForbiddenStatement(%q) = %q, want %q", sql, got, want)
		}
	}
}

// Bind refuses what Validate refuses, because a paused run binds the files it
// reads again.
func TestBindRefusesForbiddenStatement(t *testing.T) {
	_, err := Bind("INSERT INTO t VALUES (1);\n\nCOMMIT;", &Step{ID: "d", Type: StepDDL}, MapEnv{})
	if err == nil || !strings.Contains(err.Error(), "statement on line 3 is refused: COMMIT is transaction control") {
		t.Fatalf("got %v", err)
	}
}
