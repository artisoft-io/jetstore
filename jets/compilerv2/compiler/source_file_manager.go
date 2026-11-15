package compiler

import (
	"context"
	"database/sql"
	"maps"
	"slices"
)

// This file contains the source file manager for maping source file name to db key

type SourceFileManager struct {
	w                   *WorkspaceDB
	sourceFileNameToKey map[string]int
	// The files workspace_control already records as main rule files
	isMainInDb map[string]bool
	// A file already in workspace_control, not as a main rule file, that is
	// the main rule file now being saved (MarkAsMain)
	promotedToMain string
	lastDbKey      int
	lastKey        int
}

func NewSourceFileManager(w *WorkspaceDB) *SourceFileManager {
	return &SourceFileManager{
		w:                   w,
		sourceFileNameToKey: make(map[string]int),
		isMainInDb:          make(map[string]bool),
	}
}

// Load existing source file name from workspace_control table
func (sfm *SourceFileManager) LoadSourceFileNameToKey(ctx context.Context, db *sql.DB) error {
	rows, err := db.Query("SELECT key, source_file_name, coalesce(is_main, 0) FROM workspace_control;")
	if err != nil {
		return err
	}
	defer rows.Close()
	var key int
	var sourceFileName string
	var isMain bool
	for rows.Next() {
		err := rows.Scan(&key, &sourceFileName, &isMain)
		if err != nil {
			return err
		}
		sfm.sourceFileNameToKey[sourceFileName] = key
		sfm.isMainInDb[sourceFileName] = isMain
		if key > sfm.lastDbKey {
			sfm.lastDbKey = key
		}
	}
	sfm.lastKey = sfm.lastDbKey
	return nil
}

// Save to workspace_control any new source file names
func (sfm *SourceFileManager) SaveNewSourceFileNames(ctx context.Context, db *sql.DB) error {
	stmt := "INSERT INTO workspace_control (key, source_file_name, is_main) VALUES (?, ?, ?);"
	var data [][]any
	// In name order: the keys are already fixed, but rows inserted in map
	// order lay the table's pages out differently, so workspace.db differed
	// between two compiles of the same sources (Q-49)
	for _, sourceFileName := range slices.Sorted(maps.Keys(sfm.sourceFileNameToKey)) {
		key := sfm.sourceFileNameToKey[sourceFileName]
		if key > sfm.lastDbKey {
			isMain := false
			if sourceFileName == sfm.w.mainSourceFileName {
				isMain = true
			}
			data = append(data, []any{key, sourceFileName, isMain})
		}
	}
	if len(sfm.promotedToMain) > 0 {
		_, err := db.ExecContext(ctx, "UPDATE workspace_control SET is_main = 1 WHERE key = ?",
			sfm.sourceFileNameToKey[sfm.promotedToMain])
		if err != nil {
			return err
		}
	}
	if len(data) == 0 {
		return nil
	}
	return DoStatement(ctx, db, stmt, data)
}

// IsPreExistingMain reports whether workspace_control already records
// sourceFileName as a main rule file.
func (sfm *SourceFileManager) IsPreExistingMain(sourceFileName string) bool {
	return sfm.IsPreExisting(sourceFileName) && sfm.isMainInDb[sourceFileName]
}

// MarkAsMain makes a file workspace_control already records, as a file a main
// rule file includes, the main rule file being saved: SaveNewSourceFileNames
// sets its is_main.
func (sfm *SourceFileManager) MarkAsMain(sourceFileName string) {
	sfm.promotedToMain = sourceFileName
}

// Get the db key for a source file name, adding a new entry if not existing
func (sfm *SourceFileManager) GetOrAddDbKey(sourceFileName string) int {
	dbKey, exists := sfm.sourceFileNameToKey[sourceFileName]
	if !exists {
		sfm.lastKey++
		dbKey = sfm.lastKey
		sfm.sourceFileNameToKey[sourceFileName] = dbKey
	}
	return dbKey
}

// Verify if a source file name is pre-existing
func (sfm *SourceFileManager) IsPreExisting(sourceFileName string) bool {
	key, exists := sfm.sourceFileNameToKey[sourceFileName]
	if !exists {
		return false
	}
	if key <= sfm.lastDbKey {
		return true
	}
	return false
}
