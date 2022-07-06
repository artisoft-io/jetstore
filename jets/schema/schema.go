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
	SchemaName string
	TableName  string
	Columns    map[string]ColumnDefinition
	Indexes    map[string]IndexDefinition
}
type ColumnDefinition struct {
	ColumnName string
	DataType   string
	Default    string
	IsArray    bool
	IsNotNull  bool
	IsPK       bool
}
type IndexDefinition struct {
	IndexName string
	IndexDef  string
}

func GetTableSchema(dbpool *pgxpool.Pool, schema string, table string) (*TableDefinition, error) {
	if dbpool == nil {
		return nil, errors.New("dbpool is required")
	}
	result := TableDefinition{SchemaName: schema, TableName: table}
	// Get the column definitions
	result.Columns = make(map[string]ColumnDefinition)
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
		default:						//* date, text (to confirm)
			cd.DataType = udt
		}
		// Note: we're not setting IsNotNull and IsPk as it's not needed on read.
		//       It's required when create table only
		result.Columns[cd.ColumnName] = cd
	}
	// Get the index definitions
	result.Indexes = make(map[string]IndexDefinition)
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
		result.Indexes[idxdef.IndexName] = idxdef
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
	for icol := range tableDefinition.Columns {
		col := tableDefinition.Columns[icol]
		if !isFirst {
			buf.WriteString(", ")	
		}
		isFirst = false
		buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
		buf.WriteString(" ")
		buf.WriteString(ToPgType(col.DataType))
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
	}
	buf.WriteString(");")
	// index defs 
	for icol := range tableDefinition.Indexes {
		buf.WriteString(tableDefinition.Indexes[icol].IndexDef)
		buf.WriteString(" ;")
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
	buf.WriteString(" ;")
	// index defs
	for _, idx := range tableDefinition.Indexes {
		_, ok := existingSchema.Indexes[idx.IndexName]
		if !ok {
			buf.WriteString(idx.IndexDef)
			buf.WriteString(" ;")
		}
	}
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
