package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	jw "github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/artisoft-io/jetstore/jets/bridge"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

type WorkspaceDb struct {
	Dsn string
	db  *sql.DB
	Dbpool *pgxpool.Pool
}

type DomainColumn struct {
	PropertyName string
	ColumnName   string
	Predicate    *bridge.Resource
	DataType     string
	IsArray      bool
	IsGrouping   bool
}

type DomainTable struct {
	TableName        string
	ClassName        string
	ClassResource    *bridge.Resource
	Columns          []DomainColumn
	DomainKeysInfo   *schema.HeadersAndDomainKeysInfo
}

func NewDomainTable(tableName string) (DomainTable, error) {
	domainTable := DomainTable{
		TableName: tableName, 
		Columns: make([]DomainColumn, 0),
	}
	// Load the Domain Key info from domain_keys_registry
	domainKeyInfo, err := schema.NewHeadersAndDomainKeysInfo(tableName)
	if err != nil {
		return domainTable, fmt.Errorf("while calling NewHeadersAndDomainKeysInfo: %v", err)
	}
	domainTable.DomainKeysInfo = domainKeyInfo
	return domainTable, nil
}

func (domainTable *DomainTable)DomainHeaders() *[]string {
	domainHeaders := make([]string, len(domainTable.Columns))
	for ipos := range domainTable.Columns {
		domainHeaders[ipos] = domainTable.Columns[ipos].ColumnName
	}
	return &domainHeaders
}

type JetStoreProperties map[string]string

type OutputTableSpecs map[string]*DomainTable

func OpenWorkspaceDb(dsn string) (*WorkspaceDb, error) {
	fmt.Println("-- Opening workspace database...")
	db, err := sql.Open("sqlite3", dsn) // Open the created SQLite File
	if err != nil {
		return nil, fmt.Errorf("while opening workspace db: %v", err)
	}
	return &WorkspaceDb{Dsn: dsn, db: db}, nil
}

func (workspaceDb *WorkspaceDb) Close() {
	if workspaceDb.db != nil {
		workspaceDb.db.Close()
	}
}

// GetTableName: Get the table name of the workspace
func (workspaceDb *WorkspaceDb) GetTableNames() ([]string, error) {
	var tableNames []string
	rows, err := workspaceDb.db.Query("SELECT name FROM domain_tables")
	if err != nil {
		return tableNames, fmt.Errorf("while getting domain table names: %v", err)
	}
	tableNames = make([]string, 0)
	defer rows.Close()
	for rows.Next() {
		var tblName string
		rows.Scan(&tblName)
		tableNames = append(tableNames, tblName)
	}
	return tableNames, nil
}

// GetRangeDataType: Get the data type for the range of the dataProperty arg
func (workspaceDb *WorkspaceDb) GetRangeDataType(dataProperty string) (string, bool, error) {
	switch {
	case strings.HasPrefix(dataProperty, "_0:"):
		return "text", true, nil

	case dataProperty == "jets:source_period_sequence":
		return "int", false, nil

	default:
		var dataType string
		var asArray bool
		err := workspaceDb.db.QueryRow("SELECT type, as_array FROM data_properties WHERE name = ?", dataProperty).Scan(&dataType, &asArray)
		if err != nil {
			return dataType, asArray, fmt.Errorf("while looking up range data type for data_property %s: %v", dataProperty, err)
		}
		return dataType, asArray, nil
	}
}

// GetRuleSetNames: Get the slice of ruleset name for ruleseq (rule sequence) name
func (workspaceDb *WorkspaceDb) GetRuleSetNames(ruleseq string) ([]string, error) {
	var rulesets []string

	rows, err := workspaceDb.db.Query(
		"SELECT main_ruleset_name FROM rule_sequences rs OUTER LEFT JOIN main_rule_sets mrs ON mrs.rule_sequence_key = rs.key WHERE name = ? ORDER BY seq ASC", ruleseq)
	if err != nil {
		return rulesets, fmt.Errorf("while loading domain table columns info from workspace db: %v", err)
	}
	rulesets = make([]string, 0)
	defer rows.Close()
	for rows.Next() {
		var rs_name string
		rows.Scan(&rs_name)
		log.Println("  - rs_name:", rs_name)
		rulesets = append(rulesets, rs_name)
	}
	return rulesets, nil
}

// GetVolatileResources: return list of volatile resources
func (workspaceDb *WorkspaceDb) GetVolatileResources() ([]string, error) {
	var result []string
	rows, err := workspaceDb.db.Query("select value from resources where type='volatile_resource'")
	if err != nil {
		return result, fmt.Errorf("while getting volatile resources from workspace db: %v", err)
	}
	defer rows.Close()
	for rows.Next() { // Iterate and fetch the records from result cursor
		var vr string
		rows.Scan(&vr)
		result = append(result, vr)
	}
	return result, nil
}

// LoadDomainTableDefinitions: Load the Domain Table Definition, including Domain Keys definition
// returns a mapping of the output domain tables with their column specs
// if allTble is true, return all otherwise, filter using outTableFilter
func (workspaceDb *WorkspaceDb) LoadDomainTableDefinitions(allTbl bool, outTableFilter map[string]bool) (OutputTableSpecs, error) {
	domainTableMap := make(OutputTableSpecs)
	if workspaceDb.db == nil {
		return domainTableMap, fmt.Errorf("error while loading domain tables from workspace db, db connection is not opened")
	}

	// Get the the domainColumn info for each table
	domainTablesRow, err := workspaceDb.db.Query("SELECT key, name FROM domain_tables")
	if err != nil {
		return domainTableMap, fmt.Errorf("while loading domain tables from workspace db: %v", err)
	}
	defer domainTablesRow.Close()
	for domainTablesRow.Next() { // Iterate and fetch the records from result cursor
		var tableKey int
		var tableName string
		domainTablesRow.Scan(&tableKey, &tableName)

		// read the domain table column info
		if allTbl || outTableFilter[tableName] {
			fmt.Println("-- Reading table", tableName, "info...")
			domainColumnsRow, err := workspaceDb.db.Query(
				"SELECT dc.name, dp.name, dc.type, dc.as_array, dc.is_grouping FROM domain_columns dc OUTER LEFT JOIN data_properties dp ON dc.data_property_key = dp.key WHERE domain_table_key = ?", tableKey)
			if err != nil {
				return domainTableMap, fmt.Errorf("while loading domain table columns info from workspace db: %v", err)
			}
			defer domainColumnsRow.Close()
			domainTable, err := NewDomainTable(tableName)
			if err != nil {
				return domainTableMap, fmt.Errorf("while calling NewDomainTable from workspace db for TableName %s: %v", tableName, err)
			}
			for domainColumnsRow.Next() { // Iterate and fetch the records from result cursor
				var domainColumn DomainColumn
				domainColumnsRow.Scan(&domainColumn.ColumnName, &domainColumn.PropertyName, &domainColumn.DataType, &domainColumn.IsArray, &domainColumn.IsGrouping)
				// // for devel
				// fmt.Println("--   - Column:", domainColumn.ColumnName, ", (property", domainColumn.PropertyName, "), is_array?", domainColumn.IsArray, ", is_grouping?", domainColumn.IsGrouping)
				domainTable.Columns = append(domainTable.Columns, domainColumn)
			}

			// add the corresponding class name
			err = workspaceDb.db.QueryRow(
				"SELECT dc.name FROM domain_tables dt LEFT JOIN domain_classes dc WHERE dt.name = ? AND dt.domain_class_key = dc.key",
				tableName).Scan(&domainTable.ClassName)

			if err != nil {
				return domainTableMap, fmt.Errorf("while loading ClassName from workspace db for TableName %s: %v", tableName, err)
			}
			// Initializing Domain Keys Info
			domainHeaders := domainTable.DomainHeaders()
			objectTypes, domainKeysJson, err := jw.GetDomainKeysInfo(workspaceDb.Dbpool, domainTable.ClassName)
			if err != nil {
				return domainTableMap, fmt.Errorf("while calling GetDomainKeysInfo: %v", err)
			}
			mainObjectType := ""
			if len(*objectTypes) > 0 {
				mainObjectType = (*objectTypes)[0]
			}

			err = domainTable.DomainKeysInfo.InitializeDomainTable(*domainHeaders, mainObjectType, domainKeysJson)
			if err != nil {
				return domainTableMap, fmt.Errorf("while calling domainTable.DomainKeysInfo.InitializeDomainTable: %v", err)
			}

			// // for devel
			// fmt.Println("Domain Keys Info for table:",tableName)
			// fmt.Println(domainTable.DomainKeysInfo)
			domainTableMap[tableName] = &domainTable
		}
	}
	return domainTableMap, nil
}

// loadJetStoreProperties: returns a mapping of the output domain tables with their column specs
func (workspaceDb *WorkspaceDb) LoadJetStoreProperties(ruleset string) (JetStoreProperties, error) {
	result := make(JetStoreProperties)
	if workspaceDb.db == nil {
		return result, fmt.Errorf("error while loading JetStore properties from workspace db, db connection is not opened")
	}

	// Get properties
	pRow, err := workspaceDb.db.Query("SELECT jp.config_key, jp.config_value FROM jetstore_config jp, workspace_control wc WHERE jp.source_file_key = wc.key AND wc.source_file_name = ?", ruleset)
	if err != nil {
		return result, fmt.Errorf("while loading JetStore properties from workspace db: %v", err)
	}
	defer pRow.Close()
	for pRow.Next() { // Iterate and fetch the records from result cursor
		var propertyKey, propertyValue string
		pRow.Scan(&propertyKey, &propertyValue)

		result[propertyKey] = propertyValue
	}
	return result, nil
}


func (tableSpec *DomainTable) UpdateDomainTableSchema(dbpool *pgxpool.Pool, dropExisting bool, extVR []string) error {
	var err error
	if len(tableSpec.Columns) == 0 {
		return errors.New("error: no tables provided from workspace")
	}
	// Get the ObjectTypes associated with the Domain Keys
	objectTypes, _, err := jw.GetDomainKeysInfo(dbpool, tableSpec.ClassName)
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
				//* TODO Report warning to log table
				log.Printf("WARNING: Table %s has a depricated columns: %s (Make sure it allows NULL or have a DEFAULT)", tableSpec.TableName, colName)
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
