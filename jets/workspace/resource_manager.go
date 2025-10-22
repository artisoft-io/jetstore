package workspace

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains resource manager for persisting resources from compiler's json output to workspace db (workspace.db)

type ResourceManager struct {
	w *WorkspaceDB
	resourceKeyToDbKey map[int]int
}

func NewResourceManager(w *WorkspaceDB) *ResourceManager {
	return &ResourceManager{
		w: w,
		resourceKeyToDbKey: make(map[int]int),
	}
}

func (rm *ResourceManager) SaveResources(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {

	var maxKey int
	err := db.QueryRow("SELECT max(key) FROM resources").Scan(&maxKey)
	if err != nil {
		return fmt.Errorf("failed to query resources: %w", err)
	}

	stmt := "INSERT INTO resources (key, type, id, value, symbol, is_binded, inline, source_file_key, vertex, row_pos) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	var data [][]any
	for _, resource := range jetRuleModel.Resources {
		if len(resource.SourceFileName) == 0 {
			resource.SourceFileName = rm.w.mainSourceFileName
		}
		var value, symbol, isBinded, vertex, varPos, id any
		if resource.Type == "symbol" {
			symbol = resource.Value
		} else {
			if len(resource.Value) > 0 {
				value = resource.Value
			}
		}
		if resource.Type == "var" {
			isBinded = resource.IsBinded
			vertex = resource.Vertex
			varPos = resource.VarPos
		}
		if len(resource.Id) > 0 {
			id = resource.Id
		}
		fileKey := rm.w.sourceMgr.GetOrAddDbKey(resource.SourceFileName)
		data = append(data, []any{
			maxKey + 1,
			resource.Type,
			id,
			value,
			symbol,
			isBinded,
			resource.Inline,
			fileKey,
			vertex,
			varPos,
		})
		rm.resourceKeyToDbKey[resource.Key] = maxKey + 1
		maxKey++
	}
	if len(data) == 0 {
		return nil
	}
	return DoStatement(ctx, db, stmt, data)
}

func (rm *ResourceManager) GetDbKey(resourceKey int) (int, bool) {
	dbKey, exists := rm.resourceKeyToDbKey[resourceKey]
	return dbKey, exists
}