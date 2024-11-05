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
	SchemaName       string                 `json:"schemaName"`
	TableName        string                 `json:"tableName"`
	Columns          []ColumnDefinition     `json:"columns"`
	Indexes          []IndexDefinition      `json:"indexes"`
	TableConstraints []ConstraintDefinition `json:"tableConstraints"`
	Deleted          bool                   `json:"deleted"`
}
type ColumnDefinition struct {
	ColumnName  string `json:"columnName"`
	Description string `json:"description"`
	DataType    string `json:"dataType"`
	Default     string `json:"default"`
	IsArray     bool   `json:"isArray"`
	IsNotNull   bool   `json:"isNotNull"`
	Deprecated  bool   `json:"deprecated"`
	Deleted     bool   `json:"deleted"`
	IsPK        bool   `json:"isPK"`
}
type IndexDefinition struct {
	IndexName string `json:"indexName"`
	IndexDef  string `json:"indexDef"`
	Deleted   bool   `json:"deleted"`
}
type ConstraintDefinition struct {
	Name       string `json:"name"`
	Definition string `json:"definition"`
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
		default: // date, text
			cd.DataType = udt
		}
		// Note: we're not setting IsNotNull and IsPk as it's not needed on read.
		//       It's required when create table only
		result.Columns = append(result.Columns, cd)
	}
	rows.Close()

	// Get the index definitions
	result.Indexes = make([]IndexDefinition, 0)
	rows, err = dbpool.Query(context.Background(),
		`SELECT indexname, indexdef 
		 	FROM pg_catalog.pg_indexes 
			WHERE schemaname = $1 AND tablename = $2`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("while getting definition of table's indexes: table %s in schema %s", table, schema)
	}
	for rows.Next() { // Iterate and fetch the records from result cursor
		var idxdef IndexDefinition
		rows.Scan(&idxdef.IndexName, &idxdef.IndexDef)
		result.Indexes = append(result.Indexes, idxdef)
	}
	rows.Close()

	// Get the UNIQUE CONSTRAINT definitions
	result.TableConstraints = make([]ConstraintDefinition, 0)
	rows, err = dbpool.Query(context.Background(),
		`SELECT
		con.conname
		FROM
			pg_catalog.pg_constraint con
			INNER JOIN pg_catalog.pg_class rel ON rel.oid = con.conrelid
			INNER JOIN pg_catalog.pg_namespace nsp ON nsp.oid = connamespace
		WHERE
			nsp.nspname = $1
			AND rel.relname = $2
			AND con.contype = 'u'`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("while getting definition of table's unique constraints: table %s in schema %s", table, schema)
	}
	for rows.Next() { // Iterate and fetch the records from result cursor
		var cdef ConstraintDefinition
		rows.Scan(&cdef.Name)
		result.TableConstraints = append(result.TableConstraints, cdef)
	}
	rows.Close()

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
	case "int", "bool", "int32":
		return "integer"
	case "uint", "long", "ulong", "int64", "uint64", "uint32":
		return "bigint"
	case "double":
		return "double precision"
	case "resource", "volatile_resource", "text", "string":
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
	// fmt.Println(stmt)
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
		if tableDefinition.TableName == "workspace_changes" {
			log.Println("SKIPING table workspace_changes to prevent locking - must be updated at deployment time via deploy scripts")
			return nil
		}
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
func (tableDefinition *TableDefinition) DropTable(dbpool *pgxpool.Pool) error {
	if dbpool == nil {
		return errors.New("error: argument dbpool is required")
	}
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table %s: %v", tableDefinition.TableName, err)
	}
	return nil
}

func (tableDefinition *TableDefinition) CreateTable(dbpool *pgxpool.Pool) error {
	log.Println("Creating Table", tableDefinition.TableName)
	if dbpool == nil {
		return errors.New("error: dbpool required")
	}
	// drop stmt
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	// fmt.Println(stmt)
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %v", err)
	}

	// create stmt
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	// colon defs
	buf.WriteString("(\n")
	isFirst := true
	for _, col := range tableDefinition.Columns {
		if !isFirst {
			buf.WriteString(",\n")
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
	for _, constraint := range tableDefinition.TableConstraints {
		buf.WriteString(",\n")
		buf.WriteString(constraint.Definition)
	}
	buf.WriteString(");\n")
	// index defs
	for _, idx := range tableDefinition.Indexes {
		buf.WriteString("CREATE ")
		buf.WriteString(idx.IndexDef)
		buf.WriteString(" ;\n")
	}
	// Execute the statements
	stmt = buf.String()
	// fmt.Println(stmt)
	_, err = dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while creating table schema: %v", err)
	}
	return nil
}

func (tableDefinition *TableDefinition) UpdateTable(dbpool *pgxpool.Pool, existingSchema *TableDefinition) error {
	log.Println("Updating Table", tableDefinition.TableName)
	// alter stmt
	var buf strings.Builder

	// column defs and constraints
	buf.WriteString("ALTER TABLE IF EXISTS ")
	buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, existingSchema.TableName}.Sanitize())
	buf.WriteString(" ")
	isFirst := true
	for i := range tableDefinition.Columns {
		col := &tableDefinition.Columns[i]
		if !isFirst {
			buf.WriteString(", ")
		}
    isFirst = false
		if col.Deleted {
			// Drop deleted columns
			buf.WriteString("\nDROP COLUMN IF EXISTS ")
			buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
			buf.WriteString(" ")
		} else {
			buf.WriteString("\nADD COLUMN IF NOT EXISTS ")
			buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
			buf.WriteString(" ")
			buf.WriteString(ToPgType(col.DataType))
			if col.IsArray {
				buf.WriteString(" ARRAY")
			}
			if len(col.Default) > 0 {
				buf.WriteString(" DEFAULT ")
				buf.WriteString(col.Default)
			}
			if col.IsNotNull {
				buf.WriteString(" NOT NULL ")
			}
		}
	}
	// unique constraints - add / delete constaints
	// Add new constraints
	for i := range tableDefinition.TableConstraints {
		constaint := &tableDefinition.TableConstraints[i]
		foundIt := false
		for i := range existingSchema.TableConstraints {
			if constaint.Name == existingSchema.TableConstraints[i].Name {
				foundIt = true
				break
			}
		}
		if !foundIt {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			buf.WriteString("\nADD ")
			buf.WriteString(constaint.Definition)
			buf.WriteString(" ")
		}
	}
	// Drop removed constraints
	for i := range existingSchema.TableConstraints {
		constaint := &existingSchema.TableConstraints[i]
		foundIt := false
		for i := range tableDefinition.TableConstraints {
			if constaint.Name == tableDefinition.TableConstraints[i].Name {
				foundIt = true
				break
			}
		}
		if !foundIt {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			buf.WriteString("\nDROP CONSTRAINT IF EXISTS ")
			buf.WriteString(constaint.Name)
			buf.WriteString(" ")
		}
	}
	buf.WriteString(" ;\n")

	// index defs add / delete indexes
	// Add new indexes
	for i := range tableDefinition.Indexes {
		idx := &tableDefinition.Indexes[i]
		if !idx.Deleted {
			buf.WriteString("CREATE INDEX IF NOT EXISTS ")
			buf.WriteString(strings.TrimPrefix(strings.TrimPrefix(idx.IndexDef, "INDEX"), "index"))
			buf.WriteString(" ;\n")
		} else {
			buf.WriteString("DROP INDEX IF EXISTS ")
			buf.WriteString(tableDefinition.SchemaName)
			buf.WriteString(".")
			buf.WriteString(fmt.Sprintf("\"%s\"", idx.IndexName))
			buf.WriteString(" ;\n")
		}
	}

	// Execute the statements
	stmt := buf.String()
	//* PRINT STMT
	// log.Println(stmt)
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

// Register the session in session_registry,
// sourceType is the source_type of the entity saved on that session_id, which is "file" for loader and "domain_table" for server
func RegisterSession(dbpool *pgxpool.Pool, sourceType, client string, sessionId string, sourcePeriodKey int) error {
	if sessionId == "" {
		return fmt.Errorf("error: cannot have empty session")
	}
	// Get the source_period details for denormalization
	var monthPeriod, weekPeriod, dayPeriod int
	err := dbpool.QueryRow(context.Background(),
		`SELECT month_period, week_period, day_period FROM jetsapi.source_period WHERE key = $1`, sourcePeriodKey).Scan(
		&monthPeriod, &weekPeriod, &dayPeriod)
	if err != nil {
		return fmt.Errorf("while reading jetsapi.source_period table for key %d: %v", sourcePeriodKey, err)
	}
	// Insert into the session_registry
	stmt := `INSERT INTO jetsapi.session_registry (source_type, session_id, client, source_period_key, month_period, week_period, day_period) 
		VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING`
	_, err = dbpool.Exec(context.Background(), stmt, sourceType, sessionId, client, sourcePeriodKey, monthPeriod, weekPeriod, dayPeriod)
	if err != nil {
		return fmt.Errorf("error inserting in jetsapi.session_registry table: %v", err)
	}
	log.Printf("Registered session '%s' with source_period_key %d for client '%s' from '%s' in jetsapi.session_registry table",
		sessionId, sourcePeriodKey, client, sourceType)
	return nil
}
