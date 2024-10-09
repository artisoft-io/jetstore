package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridgego"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
	"github.com/artisoft-io/jetstore/jets/schema"
	jw "github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DomainColumn struct {
	ColumnInfo *rete.TableColumnNode
	Predicate  *bridgego.Resource
}

type DomainTable struct {
	TableInfo      *rete.TableNode
	ClassResource  *bridgego.Resource
	Columns        []DomainColumn
	DomainKeysInfo *schema.HeadersAndDomainKeysInfo
}

func NewDomainTable(dbpool *pgxpool.Pool, tableInfo *rete.TableNode) (*DomainTable, error) {
	// Create the DomainTable from the rete model TableNode
	domainTable := &DomainTable{
		TableInfo: tableInfo,
		Columns:   make([]DomainColumn, len(tableInfo.Columns)),
	}
	for i := range tableInfo.Columns {
		c := &tableInfo.Columns[i]
		domainTable.Columns[i] = DomainColumn{
			ColumnInfo: c,
		}
	}

	// Load the Domain Key info from domain_keys_registry
	domainKeyInfo, err := schema.NewHeadersAndDomainKeysInfo(tableInfo.TableName)
	if err != nil {
		return domainTable,
			fmt.Errorf("while calling NewHeadersAndDomainKeysInfo for table %s: %v", tableInfo.TableName, err)
	}
	domainTable.DomainKeysInfo = domainKeyInfo

	// Initializing Domain Keys Info
	domainHeaders := domainTable.DomainHeaders()
	objectTypes, domainKeysJson, err := jw.GetDomainKeysInfo(dbpool, tableInfo.ClassName)
	if err != nil {
		return domainTable, fmt.Errorf("while calling GetDomainKeysInfo: %v", err)
	}
	mainObjectType := ""
	if len(*objectTypes) > 0 {
		mainObjectType = (*objectTypes)[0]
	}

	err = domainTable.DomainKeysInfo.InitializeDomainTable(domainHeaders, mainObjectType, domainKeysJson)
	if err != nil {
		return domainTable, fmt.Errorf("while calling domainTable.DomainKeysInfo.InitializeDomainTable: %v", err)
	}

	// Add jetstore engine built-in columns
	// Add reserved columns and domain keys
	for header := range domainTable.DomainKeysInfo.ReservedColumns {
		switch {
		case header == "session_id":
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName:   "session_id",
						PropertyName: "session_id",
						Type:         "text",
						AsArray:      false,
					},
				})
		case strings.HasSuffix(header, ":domain_key"):
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName:   header,
						PropertyName: header,
						Type:         "text",
						AsArray:      false,
					},
				})

		case strings.HasSuffix(header, ":shard_id"):
			domainTable.Columns = append(domainTable.Columns,
				DomainColumn{
					ColumnInfo: &rete.TableColumnNode{
						ColumnName:   header,
						PropertyName: header,
						Type:         "int",
						AsArray:      false,
					},
				})
		}
	}

	return domainTable, nil
}

func (domainTable *DomainTable) DomainHeaders() *[]string {
	domainHeaders := make([]string, len(domainTable.Columns))
	for ipos := range domainTable.Columns {
		domainHeaders[ipos] = domainTable.Columns[ipos].ColumnInfo.ColumnName
	}
	return &domainHeaders
}

type JetStoreProperties map[string]string
type OutputTableSpecs map[string]*DomainTable

// DomainTableDefinitions: Wrap the rete.TableNode into Domain Table Definition, including Domain Keys definition
// returns a mapping of the output domain tables with their column specs
func DomainTableDefinitions(dbpool *pgxpool.Pool, tableMap map[string]*rete.TableNode) (OutputTableSpecs, error) {
	domainTableMap := make(OutputTableSpecs, len(tableMap))
	for tableName, tableInfo := range tableMap {
		domainTable, err := NewDomainTable(dbpool, tableInfo)
		if err != nil {
			return domainTableMap, fmt.Errorf("while calling NewDomainTable for table %s: %v", tableName, err)
		}
		domainTableMap[tableName] = domainTable
	}
	return domainTableMap, nil
}

func (tableSpec *DomainTable) UpdateDomainTableSchema(dbpool *pgxpool.Pool, dropExisting bool) error {
	var err error
	if tableSpec == nil || len(tableSpec.Columns) == 0 {
		return errors.New("error: no table info provided from workspace")
	}

	// targetCols is a set of target columns
	targetCols := make(map[string]bool)
	for i := range tableSpec.Columns {
		targetCols[tableSpec.Columns[i].ColumnInfo.ColumnName] = true
	}

	// create the table schema definition
	tableDefinition := schema.TableDefinition{
		SchemaName: "public",
		TableName:  tableSpec.TableInfo.TableName,
		Columns:    make([]schema.ColumnDefinition, 0),
		Indexes:    make([]schema.IndexDefinition, 0),
	}
	// Add column definitions
	for icol := range tableSpec.Columns {
		col := &tableSpec.Columns[icol]
		columnDef := schema.ColumnDefinition{
			ColumnName: col.ColumnInfo.ColumnName,
			DataType:   col.ColumnInfo.Type,
			IsArray:    col.ColumnInfo.AsArray,
			IsNotNull:  col.ColumnInfo.ColumnName == "jets:key" || col.ColumnInfo.ColumnName == "session_id",
		}
		// Indexes on grouping columns
		switch {
		case strings.HasSuffix(col.ColumnInfo.ColumnName, "domain_key"):
			idxname := tableSpec.TableInfo.TableName + "_" + col.ColumnInfo.ColumnName + "_idx"
			tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
				IndexName: idxname,
				IndexDef: fmt.Sprintf(`INDEX %s ON %s  (session_id, %s ASC)`,
					pgx.Identifier{idxname}.Sanitize(),
					pgx.Identifier{tableSpec.TableInfo.TableName}.Sanitize(),
					pgx.Identifier{col.ColumnInfo.ColumnName}.Sanitize()),
			})
		case strings.HasSuffix(col.ColumnInfo.ColumnName, "shard_id"):
			columnDef.Default = "0"
			idxname := tableSpec.TableInfo.TableName + "_" + col.ColumnInfo.ColumnName + "_idx"
			tableDefinition.Indexes = append(tableDefinition.Indexes, schema.IndexDefinition{
				IndexName: idxname,
				IndexDef: fmt.Sprintf(`INDEX %s ON %s  (session_id, %s)`,
					pgx.Identifier{idxname}.Sanitize(),
					pgx.Identifier{tableSpec.TableInfo.TableName}.Sanitize(),
					pgx.Identifier{col.ColumnInfo.ColumnName}.Sanitize()),
			})
		}
		tableDefinition.Columns = append(tableDefinition.Columns, columnDef)
	}
	// Add JetStore system column
	tableDefinition.Columns = append(tableDefinition.Columns, schema.ColumnDefinition{
		ColumnName: "last_update",
		DataType:   "datetime",
		Default:    "now()",
		IsNotNull:  true,
	})
	targetCols["last_update"] = true

	tableExists := false
	if !dropExisting {
		tableExists, err = schema.DoesTableExists(dbpool, "public", tableSpec.TableInfo.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called TableExists: %w", err)
		}
	}

	if tableExists {
		existingSchema, err := schema.GetTableSchema(dbpool, "public", tableSpec.TableInfo.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called GetTableSchema: %w", err)
		}
		// check we are not missing any column
		for i := range existingSchema.Columns {
			colName := existingSchema.Columns[i].ColumnName
			_, ok := targetCols[colName]
			if !ok {
				//* TODO Report warning to log table
				log.Printf("WARNING: Table %s has a depricated columns: %s (Make sure it allows NULL or have a DEFAULT)",
					tableSpec.TableInfo.TableName, colName)
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
