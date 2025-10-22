package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

var workspaceSchema string = os.Getenv("JETS_WORKSPACE_DB_SCHEMA_SCRIPT")

type WorkspaceDB struct {
	DB *sql.DB
	mainSourceFileName string
	sourceMgr *SourceFileManager
	rm *ResourceManager
}

func NewWorkspaceDB(dbPath string) (*WorkspaceDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	// Update workspace.db schema if needed
	schemaScript, err := os.ReadFile(workspaceSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}
	_, err = db.Exec(string(schemaScript))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}
	w := &WorkspaceDB{DB: db}
	w.sourceMgr = NewSourceFileManager(w)
	w.rm = NewResourceManager(w)
	return w, nil
}

// Write the jetrule model into the workspace database
func (w *WorkspaceDB) SaveJetRuleModel(ctx context.Context, jetRuleModel *rete.JetruleModel) error {
	w.mainSourceFileName = jetRuleModel.MainRuleFileName
	// Load source file mapping
	var err error
	w.sourceMgr = NewSourceFileManager(w)
	err = w.sourceMgr.LoadSourceFileNameToKey(ctx, w.DB)
	if err != nil {
		return fmt.Errorf("failed to load source file mapping: %w", err)
	}

	// Save resources
	err = w.rm.SaveResources(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save resources: %w", err)
	}

	//*TODO Save the other tables

	// Last, save the source file mapping back to workspace_control
	err = w.sourceMgr.SaveNewSourceFileNames(ctx, w.DB)
	if err != nil {
		return fmt.Errorf("failed to save source file mapping: %w", err)
	}

	// All done
	return nil
}

// Do sqlite statement with transaction
func DoStatement(ctx context.Context, db *sql.DB, stmt string, data [][]any) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	pstmt, err := tx.PrepareContext(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	for _, row := range data {
		_, err := pstmt.Exec(row...)
		if err != nil {
			return fmt.Errorf("failed to execute statement: %w", err)
		}
	}

	return tx.Commit()
}
