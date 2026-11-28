package wsfile

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// An override of a JetStore-owned asset, and whether JetStore has changed the
// asset under it since (JM-033, 2026-11-27).
//
// **An override outranks the image for as long as it exists.** A user flow
// fixed in the IDE while a JetStore release was pending is written back over
// the installed file at every start (SyncWorkspaceFiles, jets/workspace), so
// when the release ships the fix, the override silently hides it, and the user
// keeps running the old flow with their edit. Nothing compared the two.
//
// So a save of an owned asset (workspace_assets.InstalledHash) records the hash
// of the version installed at that moment, its base, in
// jetsapi.workspace_override_base, and each start compares the base with the
// version installed now. The result is the override's row's `status`, which
// the IDE's Workspace Changes list shows as *Change Status*, and a line in the
// start-up log for each override JetStore has changed under.
//
// **The base belongs to one override, not to the file.** It is keyed by the
// workspace_changes row's key (change_key), so a save of the same override
// keeps the base it had, and a new override of the file, after a revert, takes
// a new one. A re-save is often an unrelated edit, and must not clear the
// flag: what clears it is reverting the override, which brings back the new
// JetStore version, and making the change again on it.
//
// **The status lives in workspace_changes.status, a column nothing wrote**
// (always ''), because update_db does not change that table's schema
// (jets/schema/schema.go skips it) and the IDE's list already shows it. The
// base needs its own table for the same reason.

// The values of workspace_changes.status for an override of a JetStore-owned
// asset. Any other override's status is ”.
const (
	// AssetOverrideCurrent: the asset installed now is the one the override
	// was made on.
	AssetOverrideCurrent = "overrides JetStore asset"
	// AssetOverrideStale: JetStore has changed the asset since the override
	// was made, and the override hides the change.
	AssetOverrideStale = "JetStore asset updated since override"
	// AssetOverrideUnknown: the override was saved before bases were
	// recorded, or with no base table, so whether it hides a change is unknown.
	AssetOverrideUnknown = "overrides JetStore asset, base unknown"
)

const overrideBaseTableExistsStmt = `SELECT to_regclass('jetsapi.workspace_override_base') IS NOT NULL`

// OverrideAsset is an override of a JetStore-owned asset, as CheckOverrideBases
// found it.
type OverrideAsset struct {
	FileName string
	Status   string
	// BaseHash is the installed asset's hash when the override was made, ""
	// when none was recorded. InstalledHash is the installed asset's hash now.
	BaseHash      string
	InstalledHash string
}

// overrideBaseTableExists says whether update_db has created
// jetsapi.workspace_override_base: the code may reach a database before the
// migration, and then records and flags nothing.
func overrideBaseTableExists(ctx context.Context, dbpool *pgxpool.Pool) (bool, error) {
	var exists bool
	if err := dbpool.QueryRow(ctx, overrideBaseTableExistsStmt).Scan(&exists); err != nil {
		return false, fmt.Errorf("while looking for jetsapi.workspace_override_base: %w", err)
	}
	return exists, nil
}

// workspaceDir is the local workspace's directory under WORKSPACES_HOME.
func workspaceDir(workspaceName string) string {
	return filepath.Join(strings.TrimSpace(os.Getenv("WORKSPACES_HOME")), workspaceName)
}

// overrideStatus is the status of an override of an owned asset whose base
// is baseHash ("" when none is recorded) and whose asset is installed as
// installedHash.
func overrideStatus(baseHash, installedHash string) string {
	switch {
	case baseHash == "":
		return AssetOverrideUnknown
	case baseHash != installedHash:
		return AssetOverrideStale
	default:
		return AssetOverrideCurrent
	}
}

// isAssetOverrideStatus says whether status is one this file writes, so that
// a row that stops being an owned asset's override is cleared, and no other
// value is overwritten.
func isAssetOverrideStatus(status string) bool {
	return status == AssetOverrideCurrent || status == AssetOverrideStale || status == AssetOverrideUnknown
}

// RecordOverrideBase records the base of the override changeKey, the
// workspace_changes row SaveContent has just written for fileName, when the
// file is a JetStore-owned asset: the hash of the version installed now, kept
// if the row already has one, and sets the row's status. A file that is not an
// owned asset, or a database without the base table, records nothing.
func RecordOverrideBase(dbpool *pgxpool.Pool, workspaceName, fileName string, changeKey int) error {
	installed, owned, err := workspace_assets.InstalledHash(workspaceDir(workspaceName), fileName)
	if err != nil || !owned {
		return err
	}
	ctx := context.Background()
	exists, err := overrideBaseTableExists(ctx, dbpool)
	if err != nil || !exists {
		return err
	}
	// A row of another override of the file (a key that is not this one) is
	// replaced; this override's own row is kept, base and all.
	var base string
	err = dbpool.QueryRow(ctx, `INSERT INTO jetsapi.workspace_override_base
			(workspace_name, file_name, change_key, asset_sha256)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT ON CONSTRAINT workspace_override_base_file_uk DO UPDATE
			SET (change_key, asset_sha256, recorded_at) = (EXCLUDED.change_key, EXCLUDED.asset_sha256, DEFAULT)
			WHERE jetsapi.workspace_override_base.change_key <> EXCLUDED.change_key
		RETURNING asset_sha256`,
		workspaceName, fileName, changeKey, installed).Scan(&base)
	if errors.Is(err, pgx.ErrNoRows) {
		// No row returned: the conflict's WHERE kept this override's row.
		err = dbpool.QueryRow(ctx, `SELECT asset_sha256 FROM jetsapi.workspace_override_base
			WHERE workspace_name = $1 AND file_name = $2`, workspaceName, fileName).Scan(&base)
	}
	if err != nil {
		return fmt.Errorf("while recording the base of the override of %s: %w", fileName, err)
	}
	_, err = dbpool.Exec(ctx, `UPDATE jetsapi.workspace_changes SET status = $2 WHERE key = $1`,
		changeKey, overrideStatus(base, installed))
	if err != nil {
		return fmt.Errorf("while setting the status of the override of %s: %w", fileName, err)
	}
	return nil
}

// CheckOverrideBases compares the base of each override of a JetStore-owned
// asset in the workspace with the version installed now, sets each override's
// status accordingly, clears the status of a row that is no longer an owned
// asset's override, and deletes the bases of overrides that are gone. It
// returns the owned assets' overrides, in file name order. A database without
// the base table returns none and changes nothing.
//
// The apiserver's start calls it (checkWorkspaceVersion), which is when the
// installed version changes: a new image carries a new install.
func CheckOverrideBases(dbpool *pgxpool.Pool, workspaceName string) ([]OverrideAsset, error) {
	ctx := context.Background()
	exists, err := overrideBaseTableExists(ctx, dbpool)
	if err != nil || !exists {
		return nil, err
	}
	rows, err := dbpool.Query(ctx, `SELECT c.key, c.file_name, c.status, coalesce(b.asset_sha256, '')
		FROM jetsapi.workspace_changes c
		LEFT JOIN jetsapi.workspace_override_base b
			ON b.workspace_name = c.workspace_name AND b.file_name = c.file_name AND b.change_key = c.key
		WHERE c.workspace_name = $1
		ORDER BY c.file_name`, workspaceName)
	if err != nil {
		return nil, fmt.Errorf("while reading the overrides of workspace %s: %w", workspaceName, err)
	}
	type change struct {
		key                    int
		fileName, status, base string
	}
	var changes []change
	for rows.Next() {
		var c change
		if err := rows.Scan(&c.key, &c.fileName, &c.status, &c.base); err != nil {
			rows.Close()
			return nil, err
		}
		changes = append(changes, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	dir := workspaceDir(workspaceName)
	var owned []OverrideAsset
	for _, c := range changes {
		installed, isOwned, err := workspace_assets.InstalledHash(dir, c.fileName)
		if err != nil {
			return nil, fmt.Errorf("while reading the installed version of %s: %w", c.fileName, err)
		}
		status := ""
		if isOwned {
			status = overrideStatus(c.base, installed)
			owned = append(owned, OverrideAsset{FileName: c.fileName, Status: status,
				BaseHash: c.base, InstalledHash: installed})
		} else if !isAssetOverrideStatus(c.status) {
			continue
		}
		if status == c.status {
			continue
		}
		if _, err := dbpool.Exec(ctx, `UPDATE jetsapi.workspace_changes SET status = $2 WHERE key = $1`,
			c.key, status); err != nil {
			return nil, fmt.Errorf("while setting the status of the override of %s: %w", c.fileName, err)
		}
	}
	// The bases of overrides that were reverted or committed since.
	if _, err := dbpool.Exec(ctx, `DELETE FROM jetsapi.workspace_override_base b
		WHERE b.workspace_name = $1 AND NOT EXISTS (SELECT 1 FROM jetsapi.workspace_changes c
			WHERE c.workspace_name = b.workspace_name AND c.file_name = b.file_name AND c.key = b.change_key)`,
		workspaceName); err != nil {
		return nil, fmt.Errorf("while deleting the bases of removed overrides: %w", err)
	}
	return owned, nil
}

// LogOverrideBases writes CheckOverrideBases' result to the log: a line for
// each override that hides a JetStore change, and one naming the overrides
// whose base is unknown.
func LogOverrideBases(assets []OverrideAsset) {
	var unknown []string
	for _, a := range assets {
		switch a.Status {
		case AssetOverrideStale:
			log.Printf("*** WARNING: the override of %s hides a JetStore update: JetStore has changed this asset "+
				"since the override was saved (installed then %.12s, now %.12s). Compare them, then revert the "+
				"override in the Workspace IDE, and make the change again if it is still needed",
				a.FileName, a.BaseHash, a.InstalledHash)
		case AssetOverrideUnknown:
			unknown = append(unknown, a.FileName)
		}
	}
	if len(unknown) > 0 {
		log.Printf("Overrides of JetStore assets saved before their base was recorded, which may hide a JetStore update: %s",
			strings.Join(unknown, ", "))
	}
}
