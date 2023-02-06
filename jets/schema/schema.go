package schema

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file provide functions to manage the postgres table definition based on the workspace metadata
// and api table specs

// Define Database Table Structure
// This table/column definition structure is used by the api services and by update_db.
type TableDefinition struct {
	SchemaName       string               `json:"schemaName"`
	TableName        string               `json:"tableName"`
	Columns          []ColumnDefinition   `json:"columns"`
	Indexes          []IndexDefinition    `json:"indexes"`
	TableConstraints []string             `json:"tableConstraints"`
}
type ColumnDefinition struct {
	ColumnName string  `json:"columnName"`
	DataType   string  `json:"dataType"`
	Default    string  `json:"default"`
	IsArray    bool    `json:"isArray"`
	IsNotNull  bool    `json:"isNotNull"`
	IsPK       bool    `json:"isPK"`
}
type IndexDefinition struct {
	IndexName string    `json:"indexName"`
	IndexDef  string    `json:"indexDef"`
}

func GetTableSchema(dbpool *pgxpool.Pool, schema string, table string) (*TableDefinition, error) {
	if dbpool == nil {
		return nil, errors.New("dbpool is required")
	}
	result := TableDefinition{SchemaName: schema, TableName: table}
	// Get the column definitions
	result.Columns = make([]ColumnDefinition, 0)
	rows, err := dbpool.Query(context.Background(), 
		`SELECT column_name, data_type, udt_name 
		 	FROM information_schema.columns 
			WHERE table_schema = $1 AND table_name = $2`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("while getting definition of table: %s in schema %s", table, schema)
	}
	defer rows.Close()
	for rows.Next() { // Iterate and fetch the records from result cursor
		var cd ColumnDefinition
		var dt, udt string
		rows.Scan(&cd.ColumnName, &dt, &udt)
		if dt == "ARRAY" {
			cd.IsArray = true
		}
		udt = strings.TrimPrefix(udt, "_")
		switch udt {
		case "timestamp":
			cd.DataType = "datetime"
		case "int4":
			cd.DataType = "int"
		case "int8":
			cd.DataType = "long"
		case "float4", "float8":
			cd.DataType = "double"
		default:						// date, text
			cd.DataType = udt
		}
		// Note: we're not setting IsNotNull and IsPk as it's not needed on read.
		//       It's required when create table only
		result.Columns = append(result.Columns, cd)
	}
	// Get the index definitions
	result.Indexes = make([]IndexDefinition, 0)
	rows, err = dbpool.Query(context.Background(), 
		`SELECT indexname, indexdef 
		 	FROM pg_catalog.pg_indexes 
			WHERE schemaname = $1 AND tablename = $2`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("while getting definition of table's indexes: table %s in schema %s", table, schema)
	}
	defer rows.Close()
	for rows.Next() { // Iterate and fetch the records from result cursor
		var idxdef IndexDefinition
		rows.Scan(&idxdef.IndexName, &idxdef.IndexDef)
		result.Indexes = append(result.Indexes, idxdef)
	}

	return &result, nil
}

func DoesTableExists(dbpool *pgxpool.Pool, schemaName, tableName string) (bool, error) {
	if dbpool == nil {
		return false, fmt.Errorf("error: dbpool required")
	}
	exists := false
	err := dbpool.QueryRow(context.Background(), 
		"select exists (select from pg_tables where schemaname = $1 and tablename = $2)", 
		schemaName, tableName).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("TableExists query failed: %v", err)
	}
	return exists, err
}

func ToPgType(dt string) string {
	switch dt {
	case "int", "bool":
		return "integer"
	case "uint", "long", "ulong":
		return "bigint"
	case "double":
		return "double precision"
	case "resource", "volatile_resource", "text":
		return "text"
	case "date":
		return "date"
	case "datetime":
		return "timestamp without time zone"
	default:
		return dt
	}
}

// TableDefinition Methods
// -----------------------
func (tableDefinition *TableDefinition) UpdateTableSchema(dbpool *pgxpool.Pool, dropExisting bool) (err error) {
	if dbpool == nil || len(tableDefinition.Columns) == 0 {
		return errors.New("error: arguments dbpool and tableDefinition are required")
	}
	if tableDefinition.TableName == "users" {
		dropExisting = false
	}
	// make sure the table schema exists
	stmt := fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgx.Identifier{tableDefinition.SchemaName}.Sanitize())
	log.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating schema: %v", err)
	}

	tableExists := false
	if !dropExisting {
		tableExists, err = DoesTableExists(dbpool, tableDefinition.SchemaName, tableDefinition.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called TableExists: %w", err)
		}
	}
	if tableExists {
		existingSchema, err := GetTableSchema(dbpool, tableDefinition.SchemaName, tableDefinition.TableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema getting exisiting table schema for table %s: %w", tableDefinition.TableName, err)
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

func (tableDefinition *TableDefinition) CreateTable(dbpool *pgxpool.Pool) error {
	if dbpool == nil {
		return errors.New("error: dbpool required")
	}
	// drop stmt
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	log.Println(stmt)
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %v", err)
	}

	// create stmt
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	// colon defs 
	buf.WriteString("(")
	isFirst := true
	for _, col := range tableDefinition.Columns {
		if !isFirst {
			buf.WriteString(", ")	
		}
		isFirst = false
		buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
		buf.WriteString(" ")
		ctype := ToPgType(col.DataType)
		if ctype == "integer" && col.IsPK && !col.IsArray {
			buf.WriteString(" SERIAL PRIMARY KEY ")
		} else {
			buf.WriteString(ctype)
			if col.IsArray {
				buf.WriteString(" ARRAY ")
			}
			if len(col.Default) > 0 {
				buf.WriteString(" DEFAULT ")
				buf.WriteString(col.Default)
			}
			if col.IsNotNull {
				buf.WriteString(" NOT NULL ")
			}	
			if col.IsPK {
				buf.WriteString(" PRIMARY KEY ")
			}	
		}
	}
	for iconst := range tableDefinition.TableConstraints {
		buf.WriteString(", ")
		buf.WriteString(tableDefinition.TableConstraints[iconst])
	}
	buf.WriteString(");\n")
	// index defs 
	for _, idx := range tableDefinition.Indexes {
		buf.WriteString(idx.IndexDef)
		buf.WriteString(" ;\n")
	}
	// Execute the statements
	stmt = buf.String()
	log.Println(stmt)
	log.Println("---")
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table schema: %v", err)
	}
	return nil
}

func (tableDefinition *TableDefinition) UpdateTable(dbpool *pgxpool.Pool, existingSchema *TableDefinition) error {
	// alter stmt
	var buf strings.Builder
	buf.WriteString("ALTER TABLE IF EXISTS ")
	buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, existingSchema.TableName}.Sanitize())
	buf.WriteString(" ")
	// column defs
	isFirst := true
	for _, col := range tableDefinition.Columns {
		if !isFirst {
			buf.WriteString(", ")
		}
		isFirst = false
		buf.WriteString("ADD COLUMN IF NOT EXISTS ")
		buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
		buf.WriteString(" ")
		buf.WriteString(ToPgType(col.DataType))
		if col.IsArray {
			buf.WriteString(" ARRAY")
		}
	}
	buf.WriteString(" ;\n")
	// index defs
	for _, idx := range tableDefinition.Indexes {
		foundIt := false
		for i := range existingSchema.Indexes {
			if idx.IndexName == existingSchema.Indexes[i].IndexName {
				foundIt = true
				break
			}
		}
		if !foundIt {
			buf.WriteString(idx.IndexDef)
			buf.WriteString(" ;\n")
		}
	}
	//* TODO Add consideration for change in some of tableDefinition.TableConstraints
	// Execute the statements
	stmt := buf.String()
	log.Println(stmt)
	log.Println("---")
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while updating table schema: %v", err)
	}
	return nil
}

// Utility function to check if session exist
func IsSessionExists(dbpool *pgxpool.Pool, sessionId string) (bool, error) {
	if sessionId == "" {
		return false, fmt.Errorf("error: cannot have empty session")
	}
	var nrows int
	err := dbpool.QueryRow(context.Background(),
		`SELECT count(*) FROM jetsapi.session_registry WHERE session_id = $1`, sessionId).Scan(&nrows)
	if err != nil {
		return false, fmt.Errorf("while reading jetsapi.session_registry table: %v", err)
	}
	if nrows > 0 {
		return true, nil
	}
	return false, nil
}

func RegisterSession(dbpool *pgxpool.Pool, sessionId string) error {
	if sessionId == "" {
		return fmt.Errorf("error: cannot have empty session")
	}
	stmt := `INSERT INTO jetsapi.session_registry (session_id) VALUES ($1) ON CONFLICT DO NOTHING`
	_, err := dbpool.Exec(context.Background(), stmt, sessionId)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.session_registry table: %v", err)
	}
	log.Printf("Registered session '%s' in jetsapi.session_registry table", sessionId)
	return nil
}
