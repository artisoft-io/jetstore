package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

var workspaceSchema string = os.Getenv("JETS_WORKSPACE_DB_SCHEMA_SCRIPT")

type WorkspaceDB struct {
	DB *sql.DB
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
	return &WorkspaceDB{DB: db}, nil
}

// Do sqlite statement with transaction
func (w *WorkspaceDB) DoStatement(ctx context.Context, stmt string, data [][]any) error {
	tx, err := w.DB.BeginTx(ctx, nil)
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
