package main

// This file contains functions and data struct for information
// from the workspace sqlite database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	"github.com/artisoft-io/jetstore/jets/bridge"
)

type WorkspaceDb struct {
	Dsn string
	db *sql.DB
}

type DomainColumn struct {
	PropertyName string
	ColumnName string
	Predicate *bridge.Resource
	DataType string
	IsArray bool
	mappingSpec *ProcessMap		// set for input sources
}

type DomainTable struct {
	Name string
	Columns []DomainColumn
}

type DomainColumnMapping map[string]*DomainTable

func OpenWorkspaceDb(dsn string) (*WorkspaceDb, error) {
	log.Println("Opening workspace database...")
	db, err := sql.Open("sqlite3", dsn) // Open the created SQLite File
	if err != nil {
		return nil, fmt.Errorf("while opening workspace db: %v",err)
	}
	return &WorkspaceDb{dsn, db}, nil
}

func (workspaceDb *WorkspaceDb)Close() {
	if workspaceDb.db != nil {
		workspaceDb.db.Close()
	}
}

// return a slice containing the data property spec, using Domain Column
func (workspaceDb *WorkspaceDb) loadDataProperties(domainClass string) ([]DomainColumn, error) {
	domainColumns := make([]DomainColumn, 1)
	if workspaceDb.db == nil {
		return domainColumns, fmt.Errorf("error while loading data properties for class from workspace db, db connection is not opened")
	}
	// get the domain class key
	var domainClassKey int
	err := workspaceDb.db.QueryRow("SELECT key FROM domain_classes WHERE name = ?", domainClass).Scan(&domainClassKey)
	if err != nil {
		return domainColumns, fmt.Errorf("while loading domain class key from workspace db: %v", err)
	}
	// load the class data properties
	dataPropertyRow, err := workspaceDb.db.Query("SELECT name, type, as_array FROM data_properties WHERE domain_class_key = ?", domainClassKey)
	if err != nil {
		return domainColumns, fmt.Errorf("while loading domain class data properties info from workspace db: %v",err)
	}
	defer dataPropertyRow.Close()
	for dataPropertyRow.Next() { // Iterate and fetch the records from result cursor
		var domainColumn DomainColumn
		dataPropertyRow.Scan(&domainColumn.PropertyName, &domainColumn.DataType, &domainColumn.IsArray)
		domainColumns = append(domainColumns, domainColumn)
	}
	return domainColumns, nil
}

// returns a mapping of the domain tables with their column specs
func (workspaceDb *WorkspaceDb)loadDomainColumnMapping() (DomainColumnMapping, error) {
	columnMap := make(DomainColumnMapping)
	if workspaceDb.db == nil {
		return columnMap, fmt.Errorf("error while loading domain tables from workspace db, db connection is not opened")
	}
	
	// Get the the domainColumn infor for each table
	domainTablesRow, err := workspaceDb.db.Query("SELECT key, name FROM domain_tables")
	if err != nil {
		return columnMap, fmt.Errorf("while loading domain tables from workspace db: %v",err)
	}
	defer domainTablesRow.Close()
	for domainTablesRow.Next() { // Iterate and fetch the records from result cursor
		var tableKey int
		var tableName string
		domainTablesRow.Scan(&tableKey, &tableName)
		// read the domain table column info
		log.Println("Reading table",tableName,"info...")
		domainColumnsRow, err := workspaceDb.db.Query("SELECT dc.name, dp.name, dc.type, dc.as_array FROM domain_columns dc OUTER LEFT JOIN data_properties dp ON dc.data_property_key = dp.key WHERE domain_table_key = ?", tableKey)
		if err != nil {
			return columnMap, fmt.Errorf("while loading domain table columns info from workspace db: %v",err)
		}
		defer domainColumnsRow.Close()
		domainColumns := DomainTable{Name: tableName, Columns: make([]DomainColumn, 1)}
		for domainColumnsRow.Next() { // Iterate and fetch the records from result cursor
			var domainColumn DomainColumn
			domainColumnsRow.Scan(&domainColumn.ColumnName, &domainColumn.PropertyName, &domainColumn.DataType, &domainColumn.IsArray)
			log.Println("  - Column:",domainColumn.ColumnName,", (property",domainColumn.PropertyName,"), is_array?",domainColumn.IsArray)
			domainColumns.Columns = append(domainColumns.Columns, domainColumn)
		}
		log.Println("Got",len(domainColumns.Columns),"columns")
		columnMap[tableName] = &domainColumns
	}
	return columnMap, nil
}
