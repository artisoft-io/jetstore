package wsfile

// A change saved from the IDE is recorded with the signed-in user's email.
// SaveContent recorded every change as made by "system". The PostgreSQL test is
// skipped unless JETS_TEST_DSN names one (revertDB, delete_changes_test.go).

import (
	"context"
	"testing"
)

func TestChangeAuthor(t *testing.T) {
	for in, want := range map[string]string{"": "system", "  ": "system", "ke@example.com": "ke@example.com"} {
		if got := changeAuthor(in); got != want {
			t.Errorf("changeAuthor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSaveContentRecordsWhoMadeTheChange(t *testing.T) {
	pool := revertDB(t)
	bg := context.Background()
	for _, s := range []string{
		"DROP TABLE jetsapi.workspace_changes",
		`CREATE TABLE jetsapi.workspace_changes (key serial PRIMARY KEY, workspace_name text NOT NULL,
			oid bigint NOT NULL, data bytea, file_name text NOT NULL, content_type text NOT NULL,
			status text NOT NULL DEFAULT '', user_email text NOT NULL, last_update timestamp NOT NULL DEFAULT now(),
			CONSTRAINT workspace_changes_unique_cstraint UNIQUE (workspace_name, file_name))`,
	} {
		if _, err := pool.Exec(bg, s); err != nil {
			t.Fatal(err)
		}
	}
	ws, _ := stashScenario(t, "pristine")
	author := func() string {
		var email string
		if err := pool.QueryRow(bg, "SELECT user_email FROM jetsapi.workspace_changes WHERE file_name = 'process_config/a_config.json'").
			Scan(&email); err != nil {
			t.Fatal(err)
		}
		return email
	}
	if err := SaveContent(pool, ws, "process_config/a_config.json", "edit 1", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := author(); got != "ke@example.com" {
		t.Errorf("user_email %q, want ke@example.com", got)
	}
	// A later save by another user takes the row.
	if err := SaveContent(pool, ws, "process_config/a_config.json", "edit 2", "other@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := author(); got != "other@example.com" {
		t.Errorf("user_email %q, want other@example.com", got)
	}
	// No user: system, as before.
	if err := SaveContent(pool, ws, "process_config/a_config.json", "edit 3", ""); err != nil {
		t.Fatal(err)
	}
	if got := author(); got != "system" {
		t.Errorf("user_email %q, want system", got)
	}
}
