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

// A file added from the IDE in a folder the workspace does not have, a new
// runbook's folder, is saved as any edit is: the folder is made, the file is
// written, and the override is recorded, so every start applies it. Its revert
// deletes it, since the stash taken at the start has no copy of it.
//
// SaveContent wrote the file without making its folder until 2026-11-22, so
// such a save recorded the override and then failed on the disk.
func TestSaveContentAddsAFileInANewFolderAndItsRevertDeletesIt(t *testing.T) {
	pool := changesDB(t)
	bg := context.Background()
	ws, _ := stashScenario(t, "pristine")
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	const name = "runbooks/health_check/count.sql"
	file := filepath.Join(os.Getenv("WORKSPACES_HOME"), ws, filepath.FromSlash(name))
	if err := SaveContent(pool, ws, name, "SELECT 1", "ke@example.com"); err != nil {
		t.Fatalf("saving a file in a new folder: %v", err)
	}
	if got := read(t, file); got != "SELECT 1" {
		t.Errorf("on disk %q, want the content", got)
	}
	var data, contentType string
	if err := pool.QueryRow(bg, `SELECT convert_from(data, 'UTF8'), content_type FROM jetsapi.workspace_changes
		WHERE workspace_name = $1 AND file_name = $2`, ws, name).Scan(&data, &contentType); err != nil {
		t.Fatalf("no workspace_changes row: %v", err)
	}
	if data != "SELECT 1" || contentType != "runbooks" {
		t.Errorf("recorded %q, content type %q", data, contentType)
	}

	if err := DeleteFileChange(pool, ws, name); err != nil {
		t.Fatalf("reverting the added file: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("the added file is still there after its revert: %v", err)
	}
	// The folders added with it go too, and the folder the workspace was
	// deployed with stays.
	root := filepath.Join(os.Getenv("WORKSPACES_HOME"), ws)
	if _, err := os.Stat(filepath.Join(root, "runbooks")); !os.IsNotExist(err) {
		t.Errorf("the folders added with the file are still there after its revert: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "process_config")); err != nil {
		t.Errorf("the revert removed a folder the workspace was deployed with: %v", err)
	}
	if rows := changeRows(t, pool); len(rows) != 0 {
		t.Errorf("rows left after the revert: %v", rows)
	}
}
