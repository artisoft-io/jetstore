package compiler

import (
	"context"
	"database/sql"
)

// This file contains the source file manager for maping source file name to db key

type SourceFileManager struct {
	w *WorkspaceDB
	sourceFileNameToKey map[string]int
	lastDbKey int
	lastKey int
}

func NewSourceFileManager(w *WorkspaceDB) *SourceFileManager {
	return &SourceFileManager{
		w: w,
		sourceFileNameToKey: make(map[string]int),
	}
}

// Load existing source file name from workspace_control table
func (sfm *SourceFileManager) LoadSourceFileNameToKey(ctx context.Context, db *sql.DB) error {
	rows, err := db.Query("SELECT key, source_file_name FROM workspace_control;")
	if err != nil {
		return err
	}
	defer rows.Close()
	var key int
	var sourceFileName string
	for rows.Next() {
		err := rows.Scan(&key, &sourceFileName)
		if err != nil {
			return err
		}
		sfm.sourceFileNameToKey[sourceFileName] = key
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
	for sourceFileName, key := range sfm.sourceFileNameToKey {
		if key > sfm.lastDbKey {
			isMain := false
			if sourceFileName == sfm.w.mainSourceFileName {
				isMain = true
			}
			data = append(data, []any{key, sourceFileName, isMain})
		}
	}
	if len(data) == 0 {
		return nil
	}
	return DoStatement(ctx, db, stmt, data)
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

// Get the db key for a source file name, adding a new entry if not existing
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

