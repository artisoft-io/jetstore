package wsfile

// Reverting a change deletes its jetsapi.workspace_changes row only once the
// file has been restored from the stash. It used to delete the row first and
// log, then return success, when the stash had no copy: the file kept the edit,
// the row that recorded it was gone, and the screen said the revert worked.
//
// The restore tests always run. The DeleteFileChange tests are skipped unless
// JETS_TEST_DSN names a PostgreSQL; they create and drop a database of their
// own, wsfile_revert_test.

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRestoreFileFromStashRefusesWhenTheStashHasNoCopy(t *testing.T) {
	ws, file := stashScenario(t, "pristine")
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(filepath.Dir(file), "added_later.json")
	if err := os.WriteFile(added, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	err := restoreFileFromStash(ws, "process_config/added_later.json", added)
	if !errors.Is(err, ErrNoStashCopy) {
		t.Fatalf("got %v, want ErrNoStashCopy", err)
	}
	if got := read(t, added); got != "new" {
		t.Errorf("the file changed on a refused restore: %q", got)
	}
}

func TestRestoreFileFromStashRestoresAndKeepsTheMode(t *testing.T) {
	ws, file := stashScenario(t, "pristine")
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("edited"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreFileFromStash(ws, "process_config/a_config.json", file); err != nil {
		t.Fatal(err)
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("got %q, want pristine", got)
	}
	if info, _ := os.Stat(file); info.Mode().Perm() != 0600 {
		t.Errorf("mode %v, want 0600", info.Mode().Perm())
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(file), ".revert-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

// revertDB returns a pool on a database holding a minimal
// jetsapi.workspace_changes, or skips.
func revertDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN is not set")
	}
	bg := context.Background()
	admin, err := pgxpool.New(bg, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	_, _ = admin.Exec(bg, "DROP DATABASE IF EXISTS wsfile_revert_test WITH (FORCE)")
	if _, err = admin.Exec(bg, "CREATE DATABASE wsfile_revert_test"); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/wsfile_revert_test"
	pool, err := pgxpool.New(bg, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, s := range []string{
		"CREATE SCHEMA jetsapi",
		"CREATE TABLE jetsapi.workspace_changes (workspace_name text, file_name text)",
	} {
		if _, err = pool.Exec(bg, s); err != nil {
			t.Fatal(err)
		}
	}
	return pool
}

func changeRows(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT file_name FROM jetsapi.workspace_changes ORDER BY file_name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	return names
}

func TestDeleteFileChangeKeepsTheRowWhenTheFileCannotBeRestored(t *testing.T) {
	pool := revertDB(t)
	ws, file := stashScenario(t, "pristine")
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(filepath.Dir(file), "added_later.json")
	if err := os.WriteFile(added, []byte("the user's file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.workspace_changes VALUES ($1, 'process_config/added_later.json'),
		 ($1, 'process_config/a_config.json')`, ws); err != nil {
		t.Fatal(err)
	}

	err := DeleteFileChange(pool, ws, "process_config/added_later.json")
	if err == nil || !errors.Is(err, ErrNoStashCopy) ||
		!strings.Contains(err.Error(), "cannot revert process_config/added_later.json, the change is kept") {
		t.Fatalf("got %v, want the refusal naming the file", err)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != "process_config/a_config.json process_config/added_later.json" {
		t.Errorf("rows after the refused revert: %s", got)
	}
	if got := read(t, added); got != "the user's file" {
		t.Errorf("the file changed: %q", got)
	}

	// The file the stash does hold reverts, and only its row goes.
	if err := DeleteFileChange(pool, ws, "process_config/a_config.json"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("got %q, want pristine", got)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != "process_config/added_later.json" {
		t.Errorf("rows after the revert: %s", got)
	}
}

func TestDeleteAllFileChangesKeepsTheRowsWhenThereIsNoStash(t *testing.T) {
	pool := revertDB(t)
	ws, file := stashScenario(t, "pristine")
	// No StashFiles: the workspace was never stashed.
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.workspace_changes VALUES ($1, 'process_config/a_config.json')`, ws); err != nil {
		t.Fatal(err)
	}
	err := DeleteAllFileChanges(pool, ws, true, false)
	if err == nil || !strings.Contains(err.Error(), "the changes are kept") {
		t.Fatalf("got %v, want the refusal", err)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != "process_config/a_config.json" {
		t.Errorf("rows after the refused revert: %q", got)
	}

	// Stashed (here, after the edit, so it restores the edit): the rows go.
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	if err := DeleteAllFileChanges(pool, ws, true, false); err != nil {
		t.Fatal(err)
	}
	if got := changeRows(t, pool); len(got) != 0 {
		t.Errorf("rows after the revert: %v", got)
	}
}
