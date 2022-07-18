package workspace

// This file contains the postgresql schema adaptor
// for creating domain table and their extensions

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/artisoft-io/jetstore/jets/schema"
)

// ExtTableInfo: multi value arg for extending tables with volatile fields
type ExtTableInfo map[string][]string

func (tableSpec *DomainTable) UpdateDomainTableSchema(dbpool *pgxpool.Pool, dropExisting bool, extVR []string) error {
	var err error
	if len(tableSpec.Columns) == 0 {
		return errors.New("error: no tables provided from workspace")
	}
	// convert the virtual resource to column names
	extCols := make([]string, len(extVR))
	for i := range extVR {
		extCols[i] = strings.ToLower(extVR[i])
	}
	// targetCols is a set of target schema + ext volatile resource
	targetCols := make(map[string]bool)
	for _, c := range tableSpec.Columns {
		targetCols[c.ColumnName] = true
	}
	for _, vr := range extCols {
		targetCols[vr] = true
	}

	// create the table schema definition
	tableDefinition := schema.TableDefinition{
		SchemaName: "public",
		TableName: tableSpec.TableName,
		Columns: make([]schema.ColumnDefinition, 0),
		Indexes: make([]schema.IndexDefinition, 0),
	}
	// Add column definitions
	for icol := range tableSpec.Columns {
		col := tableSpec.Columns[icol]
		tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
			ColumnName: col.ColumnName,
			DataType: col.DataType,
			IsArray: col.IsArray,
			IsNotNull: col.ColumnName == "jets:key",
		})
	}
	// Add extension columns
	for _, extc := range extCols {
		tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
			ColumnName: extc,
			DataType: "text",
			IsArray: true,
		})
	}
	// Add jetstore engine columns
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "session_id",
		DataType: "text",
		IsNotNull: true,
	})
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "shard_id",
		DataType: "int",
		Default: "0",
		IsNotNull: true,
	})
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "last_update",
		DataType: "datetime",
		Default: "now()",
		IsNotNull: true,
	})
	// Primary index definitions
	idxname := tableSpec.TableName + "_primary_idx"
	tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
		IndexName: idxname,
		IndexDef: fmt.Sprintf(`CREATE INDEX %s ON %s  ("jets:key", session_id, last_update DESC)`,
			pgx.Identifier{idxname}.Sanitize(),
			pgx.Identifier{tableSpec.TableName}.Sanitize()),
	})
	// Indexes on grouping columns
	for icol := range tableSpec.Columns {
		col := &tableSpec.Columns[icol]
		if col.IsGrouping {
			idxname := tableSpec.TableName+"_"+col.ColumnName+"_idx"
			tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
				IndexName: idxname,
				IndexDef: fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s  (%s ASC, "jets:key", session_id, last_update DESC)`,
					pgx.Identifier{idxname}.Sanitize(),
					pgx.Identifier{tableSpec.TableName}.Sanitize(),
					pgx.Identifier{col.ColumnName}.Sanitize()),
			})
		}
	}	
	// Shard index
	idxname = tableSpec.TableName + "_shard_idx"
	tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition {
		IndexName: idxname,
		IndexDef: fmt.Sprintf(`CREATE INDEX %s ON %s  (shard_id)`,
			pgx.Identifier{idxname}.Sanitize(),
			pgx.Identifier{tableSpec.TableName}.Sanitize()),
	})

	tableExists := false
	if !dropExisting {
		tableExists, err = schema.DoesTableExists(dbpool, "public", tableSpec.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called TableExists: %w", err)
		}
	}

	if tableExists {
		existingSchema, err := schema.GetTableSchema(dbpool, "public", tableSpec.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called GetDomainColumns: %w", err)
		}
		// check we are not missing any column
		for i := range existingSchema.Columns {
			colName := existingSchema.Columns[i].ColumnName
			_, ok := targetCols[colName]
			if !ok {
				return fmt.Errorf("error: cannot update existing table with removed columns: %s", colName)
			}
		}
		err = tableDefinition.UpdateTable(dbpool, existingSchema)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called UpdateTable: %w", err)
		}
	} else {
		err = tableDefinition.CreateTable(dbpool)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called CreateTable: %w", err)
		}
	}
	return nil
}

// func GetDomainColumns(dbpool *pgxpool.Pool, tableDefinition *schema.TableDefinition) (map[string]DomainColumn, error) {
// 	if dbpool == nil {
// 		return nil, errors.New("dbpool is required")
// 	}
// 	result := make(map[string]DomainColumn)
// 	for i := range tableDefinition.Columns {
// 		columnDef := &tableDefinition.Columns[i]
// 		var dc DomainColumn
// 		dc.ColumnName = columnDef.ColumnName
// 		dc.DataType = columnDef.DataType
// 		dc.IsArray = columnDef.IsArray
// 		result[dc.ColumnName] = dc
// 	}
// 	return result, nil
// }
