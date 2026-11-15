package wsfile

// A change saved from the IDE is recorded with the signed-in user's email.
// SaveContent recorded every change as made by "system". The PostgreSQL test is
// skipped unless JETS_TEST_DSN names one (revertDB, delete_changes_test.go).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestChangeAuthor(t *testing.T) {
	for in, want := range map[string]string{"": "system", "  ": "system", "ke@example.com": "ke@example.com"} {
		if got := changeAuthor(in); got != want {
			t.Errorf("changeAuthor(%q) = %q, want %q", in, got, want)
		}
	}
}

// changesDB is revertDB with jetsapi.workspace_changes as SaveContent writes it.
func changesDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := revertDB(t)
	for _, s := range []string{
		"DROP TABLE jetsapi.workspace_changes",
		`CREATE TABLE jetsapi.workspace_changes (key serial PRIMARY KEY, workspace_name text NOT NULL,
			oid bigint NOT NULL, data bytea, file_name text NOT NULL, content_type text NOT NULL,
			status text NOT NULL DEFAULT '', user_email text NOT NULL, last_update timestamp NOT NULL DEFAULT now(),
			CONSTRAINT workspace_changes_unique_cstraint UNIQUE (workspace_name, file_name))`,
	} {
		if _, err := pool.Exec(context.Background(), s); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func TestSaveContentRecordsWhoMadeTheChange(t *testing.T) {
	pool := changesDB(t)
	bg := context.Background()
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

// A file at the workspace root, workspace_control.json or
// compile_workspace.sh, is saved as an override like any other file: on disk
// and in jetsapi.workspace_changes, so that the next start applies it.
//
// It was written to disk and then refused, because the override's content type
// was the path's first segment and a root file has none: the edit stood until
// the next start and then went, with nothing recorded. Reproduced 2026-11-14
// with this test against the code before the fix: the error, and the disk
// already holding the edit.
func TestSaveContentSavesARootFileAsAnOverride(t *testing.T) {
	pool := changesDB(t)
	bg := context.Background()
	ws, _ := stashScenario(t, "pristine")
	root := filepath.Join(os.Getenv("WORKSPACES_HOME"), ws)
	for _, name := range []string{"workspace_control.json", "compile_workspace.sh"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("as shipped"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := SaveContent(pool, ws, name, "edited", "ke@example.com"); err != nil {
			t.Errorf("saving %s: %v", name, err)
		}
		if got := read(t, filepath.Join(root, name)); got != "edited" {
			t.Errorf("%s on disk is %q, want the edit", name, got)
		}
		var data, contentType string
		err := pool.QueryRow(bg, `SELECT convert_from(data, 'UTF8'), content_type FROM jetsapi.workspace_changes
			WHERE workspace_name = $1 AND file_name = $2`, ws, name).Scan(&data, &contentType)
		if err != nil {
			t.Errorf("%s has no workspace_changes row: %v", name, err)
			continue
		}
		if data != "edited" || contentType != RootContentType {
			t.Errorf("%s recorded as %q, content type %q, want the edit and %q", name, data, contentType, RootContentType)
		}
	}
}

// A save whose database write fails leaves the disk as it was, so that the
// file the IDE shows is never an edit the database does not hold.
func TestSaveContentWritesNothingWhenTheDatabaseRefuses(t *testing.T) {
	pool := revertDB(t) // workspace_changes without the columns SaveContent writes
	ws, file := stashScenario(t, "pristine")
	if err := SaveContent(pool, ws, "process_config/a_config.json", "edited", ""); err == nil {
		t.Fatal("a save the database refused reported no error")
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("the file holds %q after a refused save, want it untouched", got)
	}
}
