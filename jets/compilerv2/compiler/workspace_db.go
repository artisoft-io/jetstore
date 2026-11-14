package compiler

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/utils"
	_ "github.com/mattn/go-sqlite3"
)

// workspaceSchema is the workspace.db schema script: JETS_WORKSPACE_DB_SCHEMA_SCRIPT,
// else workspace_schema.sql in JETS_BIN_DIR, else /usr/local/bin/workspace_schema.sql.
var workspaceSchema string = os.Getenv("JETS_WORKSPACE_DB_SCHEMA_SCRIPT")

func init() {
	if len(workspaceSchema) == 0 {
		workspaceSchema = utils.JetsBinPath("workspace_schema.sql")
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
	expression2DbKey   map[string]int

	// The files the main rule file includes, in the order they were read
	// (SetIncludedFiles), saved to main_support_files.
	includedFiles []string
}

func NewWorkspaceDB(ctx context.Context, basePath string) (*WorkspaceDB, error) {
	dbPath := fmt.Sprintf("%s/workspace.db", basePath)
	log.Println("Saving workspace.db to", dbPath)
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
		DB:               db,
		reteNode2DbKey:   make(map[string]int),
		expression2DbKey: make(map[string]int),
	}
	// Initialize source file manager and resource manager
	w.sourceMgr = NewSourceFileManager(w)
	w.rm = NewWorkspaceResourceManager(w)
	// Load source file mapping
	err = w.sourceMgr.LoadSourceFileNameToKey(ctx, w.DB)
	if err != nil {
		return nil, fmt.Errorf("failed to load source file mapping: %w", err)
	}
	return w, nil
}

// SetIncludedFiles names the files the main rule file includes, directly or
// through another included file, in the order the compiler read them, for
// SaveJetRuleModel to record in main_support_files. The rule model does not
// carry them.
func (w *WorkspaceDB) SetIncludedFiles(fileNames []string) {
	w.includedFiles = fileNames
}

// Write the jetrule model into the workspace database
func (w *WorkspaceDB) SaveJetRuleModel(ctx context.Context, jetRuleModel *rete.JetruleModel) error {
	var err error
	w.mainSourceFileName = jetRuleModel.MainRuleFileName
	// A main rule file workspace_control already records as one is being saved
	// a second time, which is refused. One it records only as a file another
	// main rule file includes (that one was compiled first: the workspace
	// compile goes in name order) becomes a main rule file now, keeping its key.
	if w.sourceMgr.IsPreExisting(w.mainSourceFileName) {
		if w.sourceMgr.IsPreExistingMain(w.mainSourceFileName) {
			return fmt.Errorf("main source file %s already exists in workspace db", w.mainSourceFileName)
		}
		w.sourceMgr.MarkAsMain(w.mainSourceFileName)
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

	// Save the files the main rule file includes. After the classes and the
	// lookup tables, so a file one of those names keeps the key it had before
	// main_support_files was written, and before the rules, which name the
	// file each is written in.
	err = w.SaveMainSupportFiles(ctx, w.DB)
	if err != nil {
		return fmt.Errorf("failed to save main support files: %w", err)
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
