package compiler

// This file contains helper methods for saving the compiled workspace to workspace.db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Save Classes into workspace db
func (w *WorkspaceDB) SaveClassesAndTables(ctx context.Context, db *sql.DB, jetRuleModel *rete.JetruleModel) error {
	// Load existing classes put them in a set and keep tack of the max key
	var maxClassKey int
	className2Key := make(map[string]int)
	newClasses := make(map[string]bool)
	rows, err := db.Query("SELECT key, name FROM domain_classes")
	if err != nil {
		return fmt.Errorf("failed to query domain_classes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var name string
		err = rows.Scan(&key, &name)
		if err != nil {
			return fmt.Errorf("failed to scan class row: %w", err)
		}
		className2Key[name] = key
		if key > maxClassKey {
			maxClassKey = key
		}
	}
	// Insert new classes that are not in className2Key
	classStmt := "INSERT INTO domain_classes (key, name, as_table, source_file_key) VALUES (?, ?, ?, ?)"
	classData := make([][]any, 0, len(jetRuleModel.Classes))
	// Insert classes
	for _, class := range jetRuleModel.Classes {
		if className2Key[class.Name] == 0 {
			fileKey := w.sourceMgr.GetOrAddDbKey(class.SourceFileName)
			maxClassKey++
			className2Key[class.Name] = maxClassKey
			newClasses[class.Name] = true
			classData = append(classData, []any{maxClassKey, class.Name, class.AsTable, fileKey})
		}
	}
	// Execute class insert
	if len(classData) > 0 {
		err = DoStatement(ctx, db, classStmt, classData)
		if err != nil {
			return fmt.Errorf("failed to insert classes: %w", err)
		}
	}

	// Insert data properties for the new classes
	dataPropertiesStmt := "INSERT INTO data_properties (key, domain_class_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
	dataPropertiesData := make([][]any, 0)
	var maxDataPropKey int
	dataProperties2Key := make(map[string]int)
	err = db.QueryRow("SELECT max(key) FROM data_properties").Scan(&maxDataPropKey)
	if err != nil && !strings.Contains(err.Error(), "converting NULL to int is unsupported") {
		return fmt.Errorf("failed to query data_properties: %w", err)
	}
	for _, class := range jetRuleModel.Classes {
		if newClasses[class.Name] {
			classKey := className2Key[class.Name]
			for _, dp := range class.DataProperties {
				maxDataPropKey++
				dataProperties2Key[dp.Name] = maxDataPropKey
				dataPropertiesData = append(dataPropertiesData, []any{
					maxDataPropKey,
					classKey,
					dp.Name,
					dp.Type,
					dp.AsArray,
				})
			}
		}
	}
	// Execute data properties insert
	if len(dataPropertiesData) > 0 {
		err = DoStatement(ctx, db, dataPropertiesStmt, dataPropertiesData)
		if err != nil {
			return fmt.Errorf("failed to insert data properties: %w", err)
		}
	}

	// Insert base classes for the new classes
	baseClassStmt := "INSERT INTO base_classes (domain_class_key, base_class_key) VALUES (?, ?)"
	baseClassData := make([][]any, 0, 2*len(jetRuleModel.Classes))
	for _, class := range jetRuleModel.Classes {
		if newClasses[class.Name] {
			// Insert it's base classes
			classKey := className2Key[class.Name]
			for _, baseClass := range class.BaseClasses {
				baseClassData = append(baseClassData, []any{classKey, className2Key[baseClass]})
			}
		}
	}
	// Execute base classes insert
	if len(baseClassData) > 0 {
		err = DoStatement(ctx, db, baseClassStmt, baseClassData)
		if err != nil {
			return fmt.Errorf("failed to insert base classes: %w", err)
		}
	}

	return w.SaveTables(ctx, db, className2Key, dataProperties2Key, jetRuleModel)
}

// Save Tables into workspace db
func (w *WorkspaceDB) SaveTables(ctx context.Context, db *sql.DB, className2Key,
	dataProperties2Key map[string]int, jetRuleModel *rete.JetruleModel) error {

	// Load existing tables put them in a set and keep tack of the max key
	var maxTableKey int
	existingTables := make(map[string]int)
	rows, err := db.Query("SELECT key, name FROM domain_tables")
	if err != nil {
		return fmt.Errorf("failed to query domain_tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key int
		var name string
		err = rows.Scan(&key, &name)
		if err != nil {
			return fmt.Errorf("failed to scan table row: %w", err)
		}
		existingTables[name] = key
		if key > maxTableKey {
			maxTableKey = key
		}
	}
	// Insert new tables that are not in existingTables
	tableStmt := "INSERT INTO domain_tables (key, domain_class_key, name) VALUES (?, ?, ?)"
	tableData := make([][]any, 0, len(jetRuleModel.Tables))
	columnStmt := "INSERT INTO domain_columns (domain_table_key, data_property_key, name, type, as_array) VALUES (?, ?, ?, ?, ?)"
	columnData := make([][]any, 0)

	// Insert tables & table columns
	for _, table := range jetRuleModel.Tables {
		if existingTables[table.TableName] == 0 {
			maxTableKey++
			existingTables[table.TableName] = maxTableKey
			classKey := className2Key[table.ClassName]
			tableData = append(tableData, []any{maxTableKey, classKey, table.TableName})

			// Table's columns
			for _, column := range table.Columns {
				dataPropertyKey := dataProperties2Key[column.PropertyName]
				columnData = append(columnData, []any{maxTableKey, dataPropertyKey, column.ColumnName, column.Type, column.AsArray})
			}
		}
	}
	// Execute table insert
	if len(tableData) > 0 {
		err = DoStatement(ctx, db, tableStmt, tableData)
		if err != nil {
			return fmt.Errorf("failed to insert tables: %w", err)
		}
	}

	// Execute column insert
	if len(columnData) > 0 {
		err = DoStatement(ctx, db, columnStmt, columnData)
		if err != nil {
			return fmt.Errorf("failed to insert columns: %w", err)
		}
	}
	return nil
}
