package wsfile

// An override of a JetStore-owned asset records the version it was made on,
// and a start says when JetStore has changed the asset under it (JM-033).
// Needs JETS_TEST_DSN (revertDB, delete_changes_test.go).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/dbutils"
	"github.com/jackc/pgx/v5/pgxpool"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

const ownedFile = "user_flows/flow.uf.json"

func sha(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// installFlow writes user_flows/flow.uf.json as an install would: the file,
// and its directory's manifest recording its hash. It returns the workspace.
func installFlow(t *testing.T, home, ws, content string) {
	t.Helper()
	dir := filepath.Join(home, ws, "user_flows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "flow.uf.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	m, _ := json.Marshal(map[string]any{"assets": map[string]string{"flow.uf.json": sha(content)}})
	if err := os.WriteFile(filepath.Join(dir, workspace_assets.ManifestName), m, 0644); err != nil {
		t.Fatal(err)
	}
}

// overrideBaseDB is changesDB with jetsapi.workspace_override_base made from
// its definition in jets_schema.json, as update_db makes it.
func overrideBaseDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := changesDB(t)
	t.Setenv("JETS_SCHEMA_FILE", "../../jets_schema.json")
	jetsSchema, err := getJetsSchema()
	if err != nil {
		t.Fatal(err)
	}
	def, ok := (*jetsSchema)["workspace_override_base"]
	if !ok {
		t.Fatal("jets_schema.json has no workspace_override_base")
	}
	if err = def.UpdateTableSchema(pool, true); err != nil {
		t.Fatal(err)
	}
	return pool
}

func changeStatus(t *testing.T, pool *pgxpool.Pool, fileName string) string {
	t.Helper()
	var status string
	if err := pool.QueryRow(context.Background(),
		"SELECT status FROM jetsapi.workspace_changes WHERE file_name = $1", fileName).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestAnOverrideOfAnOwnedAssetIsFlaggedWhenJetStoreChangesIt(t *testing.T) {
	pool := overrideBaseDB(t)
	bg := context.Background()
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "test_ws"
	installFlow(t, home, ws, "v1")
	if err := os.MkdirAll(filepath.Join(home, ws, "jet_rules"), 0755); err != nil {
		t.Fatal(err)
	}

	// Saved on v1: current, and a file of the workspace's own is not flagged.
	if err := SaveContent(pool, ws, ownedFile, "edited", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := SaveContent(pool, ws, "jet_rules/a.jr", "rule", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := changeStatus(t, pool, ownedFile); got != AssetOverrideCurrent {
		t.Fatalf("status after the save %q, want %q", got, AssetOverrideCurrent)
	}
	if got := changeStatus(t, pool, "jet_rules/a.jr"); got != "" {
		t.Errorf("a file of the workspace's own has status %q", got)
	}

	// A new image installs v2 under the override: the start flags it.
	installFlow(t, home, ws, "v2")
	assets, err := CheckOverrideBases(pool, ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].FileName != ownedFile || assets[0].Status != AssetOverrideStale ||
		assets[0].BaseHash != sha("v1") || assets[0].InstalledHash != sha("v2") {
		t.Fatalf("CheckOverrideBases = %+v, want %s stale from v1 to v2", assets, ownedFile)
	}
	if got := changeStatus(t, pool, ownedFile); got != AssetOverrideStale {
		t.Fatalf("status after the start %q, want %q", got, AssetOverrideStale)
	}
	LogOverrideBases(assets)

	// Saving the override again does not clear the flag: the base is the
	// override's, and the edit may be unrelated.
	if err := SaveContent(pool, ws, ownedFile, "edited again", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := changeStatus(t, pool, ownedFile); got != AssetOverrideStale {
		t.Errorf("status after a re-save %q, want %q", got, AssetOverrideStale)
	}

	// Reverted (the row goes), then made again on v2: a new override, current.
	if _, err := pool.Exec(bg, "DELETE FROM jetsapi.workspace_changes WHERE file_name = $1", ownedFile); err != nil {
		t.Fatal(err)
	}
	if err := SaveContent(pool, ws, ownedFile, "edited on v2", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := changeStatus(t, pool, ownedFile); got != AssetOverrideCurrent {
		t.Errorf("status of the new override %q, want %q", got, AssetOverrideCurrent)
	}

	// The base of an override that is gone is deleted at the next start.
	if _, err := pool.Exec(bg, `INSERT INTO jetsapi.workspace_override_base
		(workspace_name, file_name, change_key, asset_sha256) VALUES ($1, 'user_flows/gone.uf.json', 999, 'x')`, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckOverrideBases(pool, ws); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(bg, "SELECT count(*) FROM jetsapi.workspace_override_base WHERE workspace_name = $1", ws).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d bases left, want the one of the live override", n)
	}
}

// An override saved before bases were recorded is flagged as unknown, and one
// that stops being an owned asset's override has its status cleared.
func TestAnOverrideWithNoBaseIsUnknown(t *testing.T) {
	pool := overrideBaseDB(t)
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "test_ws"
	installFlow(t, home, ws, "v1")
	fo := dbutils.FileDbObject{WorkspaceName: ws, FileName: ownedFile, ContentType: "user_flows", UserEmail: "system"}
	if _, err := fo.WriteObject(pool, []byte("an old override")); err != nil {
		t.Fatal(err)
	}
	assets, err := CheckOverrideBases(pool, ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].Status != AssetOverrideUnknown {
		t.Fatalf("CheckOverrideBases = %+v, want one unknown", assets)
	}
	if got := changeStatus(t, pool, ownedFile); got != AssetOverrideUnknown {
		t.Errorf("status %q, want %q", got, AssetOverrideUnknown)
	}
	// JetStore stops shipping it: the manifest loses the entry.
	if err := os.WriteFile(filepath.Join(home, ws, "user_flows", workspace_assets.ManifestName),
		[]byte(`{"assets": {}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if assets, err = CheckOverrideBases(pool, ws); err != nil || len(assets) != 0 {
		t.Fatalf("CheckOverrideBases = %+v, %v, want none", assets, err)
	}
	if got := changeStatus(t, pool, ownedFile); got != "" {
		t.Errorf("status %q, want it cleared", got)
	}
}

// On a database update_db has not migrated, a save records nothing and a
// start flags nothing, and neither fails.
func TestNoBaseTableRecordsNothing(t *testing.T) {
	pool := changesDB(t)
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "test_ws"
	installFlow(t, home, ws, "v1")
	if err := SaveContent(pool, ws, ownedFile, "edited", "ke@example.com"); err != nil {
		t.Fatal(err)
	}
	if got := changeStatus(t, pool, ownedFile); got != "" {
		t.Errorf("status %q with no base table", got)
	}
	if assets, err := CheckOverrideBases(pool, ws); err != nil || assets != nil {
		t.Errorf("CheckOverrideBases = %+v, %v, want nothing", assets, err)
	}
}

// Commit Workspace leaves out the overrides of owned assets and keeps their
// rows (JM-032): OwnedAssetOverrides names them without the base table, and
// the deletion after the commit keeps the rows it is given.
func TestACommitKeepsTheOverridesOfOwnedAssets(t *testing.T) {
	pool := changesDB(t)
	bg := context.Background()
	home := t.TempDir()
	t.Setenv("WORKSPACES_HOME", home)
	const ws = "test_ws"
	installFlow(t, home, ws, "v1")
	for _, name := range []string{ownedFile, "jet_rules/a.jr", "workspace.db"} {
		fo := dbutils.FileDbObject{WorkspaceName: ws, FileName: name, ContentType: "x", UserEmail: "system"}
		if _, err := fo.WriteObject(pool, []byte("edited")); err != nil {
			t.Fatal(err)
		}
	}
	owned, err := OwnedAssetOverrides(pool, ws)
	if err != nil {
		t.Fatal(err)
	}
	if len(owned) != 1 || owned[0] != ownedFile {
		t.Fatalf("OwnedAssetOverrides = %v, want [%s]", owned, ownedFile)
	}
	if err := DeleteAllFileChanges(pool, ws, false, true, owned...); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(bg, "SELECT file_name FROM jetsapi.workspace_changes ORDER BY file_name")
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		left = append(left, n)
	}
	rows.Close()
	if strings.Join(left, ",") != ownedFile+",workspace.db" {
		t.Errorf("rows left %v, want the owned asset's override and the compiled asset", left)
	}
}
