package wsfile

// Reverting a change deletes its jetsapi.workspace_changes row only once the
// file has been restored from the stash, or deleted when it was added since the
// stash was taken. It used to delete the row first and
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

// A file added since the stash was taken is deleted by its revert, and its row
// goes; a file the stash holds is restored. With no stash at all, and for a name
// the stash leaves out on purpose, nothing is deleted: the revert is refused and
// the row and the file are kept.
func TestDeleteFileChangeDeletesAFileAddedSinceTheStash(t *testing.T) {
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
	gitignore := filepath.Join(filepath.Dir(filepath.Dir(file)), ".gitignore")
	if err := os.WriteFile(gitignore, []byte("build/"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.workspace_changes VALUES ($1, 'process_config/added_later.json'),
		 ($1, 'process_config/a_config.json'), ($1, '.gitignore')`, ws); err != nil {
		t.Fatal(err)
	}

	if err := DeleteFileChange(pool, ws, "process_config/added_later.json"); err != nil {
		t.Fatalf("reverting the added file: %v", err)
	}
	if _, err := os.Stat(added); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the added file is still there (%v)", err)
	}

	// The stash leaves .gitignore out on purpose: absent from it, but not added.
	err := DeleteFileChange(pool, ws, ".gitignore")
	if err == nil || !errors.Is(err, ErrNoStashCopy) || !strings.Contains(err.Error(), "the change is kept") {
		t.Fatalf("reverting .gitignore: got %v, want the refusal", err)
	}
	if got := read(t, gitignore); got != "build/" {
		t.Errorf(".gitignore changed: %q", got)
	}

	// The file the stash holds reverts.
	if err := DeleteFileChange(pool, ws, "process_config/a_config.json"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("got %q, want pristine", got)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != ".gitignore" {
		t.Errorf("rows after the reverts: %s", got)
	}
}

func TestDeleteFileChangeKeepsTheRowWhenThereIsNoStash(t *testing.T) {
	pool := revertDB(t)
	ws, file := stashScenario(t, "pristine")
	// No StashFiles: as on a workstation, every file is absent from the stash.
	added := filepath.Join(filepath.Dir(file), "added_later.json")
	if err := os.WriteFile(added, []byte("the user's file"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.workspace_changes VALUES ($1, 'process_config/added_later.json')`, ws); err != nil {
		t.Fatal(err)
	}
	err := DeleteFileChange(pool, ws, "process_config/added_later.json")
	if err == nil || !errors.Is(err, ErrNoStashCopy) ||
		!strings.Contains(err.Error(), "cannot revert process_config/added_later.json, the change is kept") {
		t.Fatalf("got %v, want the refusal naming the file", err)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != "process_config/added_later.json" {
		t.Errorf("rows after the refused revert: %s", got)
	}
	if got := read(t, added); got != "the user's file" {
		t.Errorf("the file changed: %q", got)
	}
}

func TestDeleteAllFileChangesDeletesAFileAddedSinceTheStash(t *testing.T) {
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
	if err := DeleteAllFileChanges(pool, ws, true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(added); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the added file is still there (%v)", err)
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("got %q, want pristine", got)
	}
	if got := changeRows(t, pool); len(got) != 0 {
		t.Errorf("rows after the revert: %v", got)
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

// A revert of every change keeps the compiled assets, as the button does
// (keepWorkspaceAndLookupDb): their rows stay, and the stash's start-up copies
// are not put over the compiled files.
func TestDeleteAllFileChangesKeepsTheCompiledAssets(t *testing.T) {
	pool := revertDB(t)
	ws, file := stashScenario(t, "pristine")
	dir := filepath.Dir(filepath.Dir(file))
	tgz := filepath.Join(dir, "workspace.tgz")
	db := filepath.Join(dir, "workspace.db")
	if err := os.WriteFile(tgz, []byte("start-up tgz"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := StashFiles(ws); err != nil {
		t.Fatal(err)
	}
	// The compile since start-up, and an edit.
	if err := os.WriteFile(tgz, []byte("compiled tgz"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, []byte("compiled db"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("the user's edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.workspace_changes VALUES ($1, 'process_config/a_config.json'),
		 ($1, 'workspace.tgz'), ($1, 'workspace.db'), ($1, 'lookup.db'), ($1, 'reports.tgz')`, ws); err != nil {
		t.Fatal(err)
	}
	if err := DeleteAllFileChanges(pool, ws, true, true); err != nil {
		t.Fatal(err)
	}
	if got := read(t, file); got != "pristine" {
		t.Errorf("the edited file: got %q, want pristine", got)
	}
	if got := read(t, tgz); got != "compiled tgz" {
		t.Errorf("workspace.tgz: got %q, want the compiled one", got)
	}
	if got := read(t, db); got != "compiled db" {
		t.Errorf("workspace.db: got %q, want the compiled one", got)
	}
	if got := strings.Join(changeRows(t, pool), " "); got != "lookup.db reports.tgz workspace.db workspace.tgz" {
		t.Errorf("rows after the revert: %s", got)
	}
}
