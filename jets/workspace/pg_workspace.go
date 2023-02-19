package workspace

// This file contains the postgresql schema adaptor
// for creating domain table and their extensions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// ExtTableInfo: multi value arg for extending tables with volatile fields
type ExtTableInfo map[string][]string

func GetDomainKeysInfo(dbpool *pgxpool.Pool, rdfType string) (*[]string, *string, error) {
	objectTypes := make([]string, 0)
	var domainKeysJson string
	stmt := "SELECT object_types, domain_keys_json FROM jetsapi.domain_keys_registry WHERE entity_rdf_type=$1"
	err := dbpool.QueryRow(context.Background(), stmt, rdfType).Scan(&objectTypes, &domainKeysJson)
	if err != nil && err.Error() != "no rows in result set" {
		log.Printf("Error in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %v", rdfType, err)
		return &objectTypes, &domainKeysJson, 
			fmt.Errorf("in GetDomainKeysInfo while querying domain_keys_registry for rdfType %s: %w", rdfType, err)
	}	
	return &objectTypes, &domainKeysJson, nil
}

func (tableSpec *DomainTable) UpdateDomainTableSchema(dbpool *pgxpool.Pool, dropExisting bool, extVR []string) error {
	var err error
	if len(tableSpec.Columns) == 0 {
		return errors.New("error: no tables provided from workspace")
	}
	// Get the ObjectTypes associated with the Domain Keys
	objectTypes, _, err := GetDomainKeysInfo(dbpool, tableSpec.ClassName)
	if err != nil {
		return err
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
	// Add jetstore engine built-in columns
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "session_id",
		DataType: "text",
		IsNotNull: true,
	})
	targetCols["session_id"] = true

	for _,objectType := range *objectTypes {
		domainKey := fmt.Sprintf("%s:domain_key", objectType)
		shardId := fmt.Sprintf("%s:shard_id", objectType)

		tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
			ColumnName: domainKey,
			DataType: "text",
			Default: "",
			IsNotNull: true,
		})
		targetCols[domainKey] = true

		tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
			ColumnName: shardId,
			DataType: "int",
			Default: "0",
			IsNotNull: true,
		})
		targetCols[shardId] = true

		// Indexes on grouping columns
		idxname := tableSpec.TableName+"_"+domainKey+"_idx"
		tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
			IndexName: idxname,
			IndexDef: fmt.Sprintf(`INDEX IF NOT EXISTS %s ON %s  (session_id, %s ASC)`,
				pgx.Identifier{idxname}.Sanitize(),
				pgx.Identifier{tableSpec.TableName}.Sanitize(),
				pgx.Identifier{domainKey}.Sanitize()),
		})
		idxname = tableSpec.TableName + "_" + shardId + "_idx"
		tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
			IndexName: idxname,
			IndexDef: fmt.Sprintf(`INDEX IF NOT EXISTS %s ON %s  (session_id, %s)`,
				pgx.Identifier{idxname}.Sanitize(),
				pgx.Identifier{tableSpec.TableName}.Sanitize(),
				pgx.Identifier{shardId}.Sanitize()),
		})
	}
	
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "last_update",
		DataType: "datetime",
		Default: "now()",
		IsNotNull: true,
	})
	targetCols["last_update"] = true

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
			return fmt.Errorf("while UpdateTableSchema called GetTableSchema: %w", err)
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
