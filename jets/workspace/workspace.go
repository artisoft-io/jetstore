package workspace

// This package contains functions and data struct for information
// from the workspace sqlite database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/bridge"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
)

type WorkspaceDb struct {
	Dsn string
	db  *sql.DB
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
	TableName     string
	ClassName     string
	ClassResource *bridge.Resource
	Columns       []DomainColumn
}

type JetStoreProperties map[string]string

type OutputTableSpecs map[string]*DomainTable

func OpenWorkspaceDb(dsn string) (*WorkspaceDb, error) {
	log.Println("Opening workspace database...")
	db, err := sql.Open("sqlite3", dsn) // Open the created SQLite File
	if err != nil {
		return nil, fmt.Errorf("while opening workspace db: %v", err)
	}
	return &WorkspaceDb{dsn, db}, nil
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
	if strings.HasPrefix(dataProperty, "_0:") {
		return "text", true, nil
	}
	var dataType string
	var asArray bool
	err := workspaceDb.db.QueryRow("SELECT type, as_array FROM data_properties WHERE name = ?", dataProperty).Scan(&dataType, &asArray)
	if err != nil {
		return dataType, asArray, fmt.Errorf("while looking up range data type for data_property %s: %v", dataProperty, err)
	}
	return dataType, asArray, nil
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

// loadDomainColumnMapping: returns a mapping of the output domain tables with their column specs
// if allTble is true, return all otherwise, filter using outTableFilter
func (workspaceDb *WorkspaceDb) LoadDomainColumnMapping(allTbl bool, outTableFilter map[string]bool) (OutputTableSpecs, error) {
	columnMap := make(OutputTableSpecs)
	if workspaceDb.db == nil {
		return columnMap, fmt.Errorf("error while loading domain tables from workspace db, db connection is not opened")
	}

	// Get the the domainColumn infor for each table
	domainTablesRow, err := workspaceDb.db.Query("SELECT key, name FROM domain_tables")
	if err != nil {
		return columnMap, fmt.Errorf("while loading domain tables from workspace db: %v", err)
	}
	defer domainTablesRow.Close()
	for domainTablesRow.Next() { // Iterate and fetch the records from result cursor
		var tableKey int
		var tableName string
		domainTablesRow.Scan(&tableKey, &tableName)

		// read the domain table column info
		if allTbl || outTableFilter[tableName] {
			log.Println("Reading table", tableName, "info...")
			domainColumnsRow, err := workspaceDb.db.Query(
				"SELECT dc.name, dp.name, dc.type, dc.as_array, dc.is_grouping FROM domain_columns dc OUTER LEFT JOIN data_properties dp ON dc.data_property_key = dp.key WHERE domain_table_key = ?", tableKey)
			if err != nil {
				return columnMap, fmt.Errorf("while loading domain table columns info from workspace db: %v", err)
			}
			defer domainColumnsRow.Close()
			domainColumns := DomainTable{TableName: tableName, Columns: make([]DomainColumn, 0)}
			for domainColumnsRow.Next() { // Iterate and fetch the records from result cursor
				var domainColumn DomainColumn
				domainColumnsRow.Scan(&domainColumn.ColumnName, &domainColumn.PropertyName, &domainColumn.DataType, &domainColumn.IsArray, &domainColumn.IsGrouping)
				log.Println("  - Column:", domainColumn.ColumnName, ", (property", domainColumn.PropertyName, "), is_array?", domainColumn.IsArray, ", is_grouping?", domainColumn.IsGrouping)
				domainColumns.Columns = append(domainColumns.Columns, domainColumn)
			}
			log.Println("Got", len(domainColumns.Columns), "columns")

			// add the corresponding class name
			err = workspaceDb.db.QueryRow(
				"SELECT dc.name FROM domain_tables dt LEFT JOIN domain_classes dc WHERE dt.name = ? AND dt.domain_class_key = dc.key",
				tableName).Scan(&domainColumns.ClassName)

			if err != nil {
				return columnMap, fmt.Errorf("while loading ClassName from workspace db for TableName %s: %v", tableName, err)
			}
			columnMap[tableName] = &domainColumns
		}
	}
	return columnMap, nil
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
