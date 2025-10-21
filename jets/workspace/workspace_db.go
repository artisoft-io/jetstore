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
	sourceFileNameToKey map[string]int
	resourceSKeyToKey map[string]int
	lastWorkspaceControlKey int
	lastWorkspaceControlKeyInDb int
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

// Write the jetrule model into the workspace database
func (w *WorkspaceDB) SaveJetRuleModel(ctx context.Context, jetRuleModel *rete.JetruleModel) error {
	// Load source file mapping
	var err error
	w.sourceFileNameToKey, err = w.loadSourceFileNameToKey(ctx)

	// Save resources
	err = w.saveResources(ctx, jetRuleModel)
	if err != nil {
		return fmt.Errorf("failed to save resources: %w", err)
	}

	//*TODO Save the other tables

	// Last, save the source file mapping back to workspace_control
	if w.lastWorkspaceControlKeyInDb < w.lastWorkspaceControlKey {
		// We have new source files to add
		stmt := "INSERT INTO workspace_control (key, source_file_name, is_main) VALUES (?, ?, ?);"
		var data [][]any
		for sourceFileName, key := range w.sourceFileNameToKey {
			if key > w.lastWorkspaceControlKeyInDb {
				isMain := false
				if sourceFileName == jetRuleModel.MainRuleFileName {
					isMain = true
				}
				data = append(data, []any{key, sourceFileName, isMain})
			}
		}
		if len(data) > 0 {
			err = w.DoStatement(ctx, stmt, data)
			if err != nil {
				return fmt.Errorf("failed to save workspace_control: %w", err)
			}
		}
	}

	// All done
	return nil
}

// Load mapping of source file name to key
// Keep track of the lastWorkspaceControlKey for files that needs to be added
// We wil use lastWorkspaceControlKeyInDb to only add new entries
func (w *WorkspaceDB) loadSourceFileNameToKey(ctx context.Context) (map[string]int, error) {
	rows, err := w.DB.QueryContext(ctx, "SELECT key, source_file_name FROM workspace_control")
	if err != nil {
		return nil, fmt.Errorf("failed to query workspace_control: %w", err)
	}
	defer rows.Close()
	mapping := make(map[string]int)
	for rows.Next() {
		var key int
		var sourceFileName string
		err := rows.Scan(&key, &sourceFileName)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace_control: %w", err)
		}
		mapping[sourceFileName] = key
		if key > w.lastWorkspaceControlKey {
			w.lastWorkspaceControlKey = key
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over workspace_control: %w", err)
	}
	w.lastWorkspaceControlKeyInDb = w.lastWorkspaceControlKey
	return mapping, nil
}

// Save the resources
func (w *WorkspaceDB) saveResources(ctx context.Context, jetRuleModel *rete.JetruleModel) error {
	// Load the existing resource, keep a mapping of SKey to Key
	var err error
	w.resourceSKeyToKey, err = w.loadResourcesSKey(ctx)
	if err != nil {
		return fmt.Errorf("failed to load existing resources: %w", err)
	}

	// Save resources
	// build the data to insert, skipping existing resources
	stmt := "INSERT INTO resources (key, type, id, value, is_binded, inline, source_file_key, vertex, row_pos)" +
		" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);"
	var data [][]any
	for _, res := range jetRuleModel.Resources {
		skey := res.SKey()
		if _, exists := w.resourceSKeyToKey[skey]; !exists {
			fileKey := w.getFileKey(res.SourceFileName)			
			data = append(data, []any{res.Key, res.Type, res.Id, res.Value, res.IsBinded, res.Inline, fileKey, res.Vertex, res.VarPos})
		}
	}
	if len(data) == 0 {
		return nil
	}
	return w.DoStatement(ctx, stmt, data)
}

func (w *WorkspaceDB) getFileKey(sourceFileName string) int {
	if key, exists := w.sourceFileNameToKey[sourceFileName]; exists {
		return key
	}
	// New source file
	w.lastWorkspaceControlKey++
	w.sourceFileNameToKey[sourceFileName] = w.lastWorkspaceControlKey
	return w.lastWorkspaceControlKey
}

// Load all resources of type 'resource' or 'literal' from the workspace database
// returns a mapping of resource SKey (string) to Key (int) where SKey is Type + Value
func (w *WorkspaceDB) loadResourcesSKey(ctx context.Context) (map[string]int, error) {
	rows, err := w.DB.QueryContext(ctx, "SELECT key, type, value FROM resources WHERE type != 'keyword'")
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %w", err)
	}
	defer rows.Close()
	mapping := make(map[string]int)
	for rows.Next() {
		var res rete.ResourceNode
		err := rows.Scan(&res.Key, &res.Type, &res.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to scan resource: %w", err)
		}
		mapping[res.SKey()] = res.Key
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over resources: %w", err)
	}
	return mapping, nil
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
