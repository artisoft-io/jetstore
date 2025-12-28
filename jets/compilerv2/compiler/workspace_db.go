package compiler

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	_ "github.com/mattn/go-sqlite3"
)

var workspaceSchema string = os.Getenv("JETS_WORKSPACE_DB_SCHEMA_SCRIPT")

func init() {
	if len(workspaceSchema) == 0 {
		workspaceSchema = "/usr/local/bin/workspace_schema.sql"
	}
}

type WorkspaceDB struct {
	DB                 *sql.DB
	mainSourceFileName string
	sourceMgr          *SourceFileManager
	rm                 *WorkspaceResourceManager
	mainFileKey        int
	maxExprKey         int
	reteNode2DbKey     map[string]int
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
	w := &WorkspaceDB{
		DB:             db,
		reteNode2DbKey: make(map[string]int),
	}
	// Initialize source file manager and resource manager
	w.sourceMgr = NewSourceFileManager(w)
	w.rm = NewWorkspaceResourceManager(w)
	return w, nil
}

// Write the jetrule model into the workspace database
func (w *WorkspaceDB) SaveJetRuleModel(ctx context.Context, jetRuleModel *rete.JetruleModel) error {
	w.mainSourceFileName = jetRuleModel.MainRuleFileName
	// Load source file mapping
	var err error
	err = w.sourceMgr.LoadSourceFileNameToKey(ctx, w.DB)
	if err != nil {
		return fmt.Errorf("failed to load source file mapping: %w", err)
	}
	if w.sourceMgr.IsPreExisting(w.mainSourceFileName) {
		return fmt.Errorf("main source file %s already exists in workspace db", w.mainSourceFileName)
	}
	w.mainFileKey = w.sourceMgr.GetOrAddDbKey(w.mainSourceFileName)

	// Save resources
	err = w.rm.SaveResources(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save resources: %w", err)
	}

	// Save Classes and Tables
	err = w.SaveClassesAndTables(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save classes and tables: %w", err)
	}

	// Save JetSore Config
	err = w.SaveJetstoreConfig(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save jetstore config: %w", err)
	}

	// Add all rule sequences
	err = w.SaveRuleSequences(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save rule sequences: %w", err)
	}

	// Add all lookup_table to rete_db, will skip source file already in rete_db
	err = w.SaveLookupTables(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save lookup tables: %w", err)
	}

	// Add expressions based on filters and object expr
	err = w.SaveExpressions(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save expressions: %w", err)
	}

	// Save Rete Nodes and beta_row_config into workspace db
	err = w.SaveReteNodes(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save rete nodes: %w", err)
	}

	// Save jet rules
	err = w.SaveJetRules(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save jet rules: %w", err)
	}

	// Save metadata triples
	err = w.SaveMetadataTriples(ctx, w.DB, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save metadata triples: %w", err)
	}

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
