// UploadWorkspaceAssets skips a file the database already holds (I-41).
//
// Needs JETS_TEST_DSN (any throwaway database; the test installs
// workspace_changes and drops it after); skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5477:5432 postgres:16
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5477/postgres go test ./jets/workspace/ -run Upload
package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

func workspaceChangesPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting to %s: %v", dsn, err)
	}
	t.Cleanup(pool.Close)
	b, err := os.ReadFile("../jets_schema.json")
	if err != nil {
		t.Fatalf("reading jets_schema.json: %v", err)
	}
	var defs []schema.TableDefinition
	if err := json.Unmarshal(b, &defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	for i := range defs {
		if defs[i].TableName != "workspace_changes" {
			continue
		}
		def := defs[i]
		if err := def.UpdateTableSchema(pool, true); err != nil {
			t.Fatalf("installing %s: %v", def.TableName, err)
		}
		t.Cleanup(func() { _ = def.DropTable(pool) })
	}
	return pool
}

// An unchanged lookup.db is not written again, and a changed one is. The marker
// is a user_email no upload writes, so it survives only a skipped upload.
func TestUploadWorkspaceAssetsSkipsAnUnchangedFile(t *testing.T) {
	pool := workspaceChangesPool(t)
	ctx := context.Background()

	saved := workspaceHome
	workspaceHome = t.TempDir()
	t.Cleanup(func() { workspaceHome = saved })
	const ws = "upload_ws"
	dir := filepath.Join(workspaceHome, ws)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"lookup.db", "workspace.db", "workspace.tgz", "reports.tgz"} {
		write(name, "first "+name)
	}
	if err := UploadWorkspaceAssets(pool, ws, "v1"); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	mark := func() {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE jetsapi.workspace_changes SET user_email = 'marker'
			WHERE workspace_name = $1 AND file_name = 'lookup.db'`, ws); err != nil {
			t.Fatal(err)
		}
	}
	stored := func() (data []byte, userEmail string) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT data, user_email FROM jetsapi.workspace_changes
			WHERE workspace_name = $1 AND file_name = 'lookup.db'`, ws).Scan(&data, &userEmail); err != nil {
			t.Fatal(err)
		}
		return data, userEmail
	}

	// Unchanged: the row is left as it was, marker and all.
	mark()
	if err := UploadWorkspaceAssets(pool, ws, "v1"); err != nil {
		t.Fatalf("unchanged upload: %v", err)
	}
	if data, who := stored(); who != "marker" || !bytes.Equal(data, []byte("first lookup.db")) {
		t.Errorf("an unchanged lookup.db was written again: user_email %q, data %q", who, data)
	}

	// Changed: the new bytes are written.
	write("lookup.db", "second lookup.db")
	mark()
	if err := UploadWorkspaceAssets(pool, ws, "v1"); err != nil {
		t.Fatalf("changed upload: %v", err)
	}
	if data, who := stored(); who != "system" || !bytes.Equal(data, []byte("second lookup.db")) {
		t.Errorf("a changed lookup.db was not written: user_email %q, data %q", who, data)
	}

	// The other three still reached the database.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.workspace_changes
		WHERE workspace_name = $1`, ws).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("%d rows for the workspace, want 4", n)
	}
}
