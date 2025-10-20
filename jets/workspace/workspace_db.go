package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"database/sql"
	"fmt"
)

// Do sqlite statement with transaction
func DoStatement(db *sql.DB, ctx context.Context, stmt string, data [][]any) error {
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
