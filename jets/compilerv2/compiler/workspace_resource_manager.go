package compiler

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains resource manager for persisting resources from compiler's json output to workspace db (workspace.db)

type WorkspaceResourceManager struct {
	w                  *WorkspaceDB
	resourceKeyToDbKey map[int]int
}

func NewWorkspaceResourceManager(w *WorkspaceDB) *WorkspaceResourceManager {
	return &WorkspaceResourceManager{
		w:                  w,
		resourceKeyToDbKey: make(map[int]int),
	}
}

func (rm *WorkspaceResourceManager) SaveResources(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {

	// Load existing resources to avoid duplicates
	rows, err := db.Query("SELECT key, type, id, value, is_binded, row_pos FROM resources")
	if err != nil {
		return fmt.Errorf("failed to query existing resources: %w", err)
	}
	defer rows.Close()
	existingResources := make(map[string]int)
	for rows.Next() {
		var key int
		var typ, id, value sql.NullString
		var isBinded sql.NullBool
		var rowPos sql.NullInt64
		err := rows.Scan(&key, &typ, &id, &value, &isBinded, &rowPos)
		if err != nil {
			return fmt.Errorf("failed to scan existing resource: %w", err)
		}
		r := rete.ResourceNode{
			Type:     typ.String,
			Id:       id.String,
			Value:    value.String,
			IsBinded: isBinded.Bool,
			VarPos:   int(rowPos.Int64),
		}
		existingResources[r.UniqueKey()] = key
	}
	var maxKeySql sql.NullInt64
	err = db.QueryRow("SELECT max(key) FROM resources").Scan(&maxKeySql)
	if err != nil {
		return fmt.Errorf("failed to query max key for resources: %w", err)
	}
	maxKey := int(maxKeySql.Int64)

	// Note: inline, vertex and source_file_key are not used, columns should be removed
	stmt := "INSERT INTO resources (key, type, id, value, symbol, is_binded, row_pos, source_file_key) VALUES (?, ?, ?, ?, ?, ?, ?, 0)"
	var data [][]any
	for _, resource := range jetRuleModel.Resources {
		if resource.Type == "volatile_resource" {
			resource.Value = strings.TrimPrefix(resource.Value, "_0:")
		}
		// Check if resource already exists
		if dbKey, exists := existingResources[resource.UniqueKey()]; exists {
			rm.resourceKeyToDbKey[resource.Key] = dbKey
			continue
		}

		var value, symbol, isBinded, varPos, id any
		if resource.Type == "symbol" {
			symbol = resource.Value
		} else {
			if len(resource.Value) > 0 {
				value = resource.Value
			}
		}
		if resource.Type == "var" {
			isBinded = resource.IsBinded
			varPos = resource.VarPos
		}
		if len(resource.Id) > 0 {
			id = resource.Id
		}
		maxKey++
		data = append(data, []any{
			maxKey,
			resource.Type,
			id,
			value,
			symbol,
			isBinded,
			varPos,
		})
		existingResources[resource.UniqueKey()] = maxKey
		rm.resourceKeyToDbKey[resource.Key] = maxKey
	}
	if len(data) == 0 {
		return nil
	}
	return DoStatement(ctx, db, stmt, data)
}

func (rm *WorkspaceResourceManager) GetDbKey(resourceKey int) (int, bool) {
	dbKey, exists := rm.resourceKeyToDbKey[resourceKey]
	return dbKey, exists
}
