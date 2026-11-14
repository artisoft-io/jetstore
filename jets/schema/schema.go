package schema

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/dblogin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// pgTypes is the closed set of column types a table definition may name, and
// the PostgreSQL type each one is written as. It is the whole of what reaches the
// DDL from a type name: CreateTable, UpdateTable and compute pipes'
// CreateOutputTable put the mapped value in a CREATE or ALTER statement and send
// it on the simple protocol, which accepts several statements, so a type passed
// through as written would let whoever wrote it run SQL as the apiserver's
// database user. A name not listed is refused.
//
// Three groups:
//   - the JetRules data property types, and their compute pipes aliases;
//   - what ToPgType returns, so mapping a mapped type again is harmless (an
//     output table written with "double precision" keeps working);
//   - the PostgreSQL types jets_schema.json names directly (jsonb, timestamptz,
//     bytea, boolean), which reached the DDL as written before the list closed.
//
// An array is not a type name here: it is the column's own flag (IsArray in a
// table definition, as_array in a compute pipes column), which appends ARRAY.
var pgTypes = map[string]string{
	"int":               "integer",
	"int32":             "integer",
	"bool":              "integer",
	"uint":              "bigint",
	"uint32":            "bigint",
	"long":              "bigint",
	"ulong":             "bigint",
	"int64":             "bigint",
	"uint64":            "bigint",
	"double":            "double precision",
	"resource":          "text",
	"volatile_resource": "text",
	"text":              "text",
	"string":            "text",
	"date":              "date",
	"datetime":          "timestamp without time zone",
	// what the names above map to
	"integer":                     "integer",
	"bigint":                      "bigint",
	"double precision":            "double precision",
	"timestamp without time zone": "timestamp without time zone",
	// named by jets_schema.json
	"jsonb":       "jsonb",
	"timestamptz": "timestamptz",
	"bytea":       "bytea",
	"boolean":     "boolean",
}

// ToPgType returns the PostgreSQL column type for the type name dt, or an error
// when dt is not one of the names in PgTypeNames. See pgTypes for why the list is
// closed.
func ToPgType(dt string) (string, error) {
	if pgType, ok := pgTypes[dt]; ok {
		return pgType, nil
	}
	if dt == "" {
		return "", fmt.Errorf("no column type given; it must be one of %s", strings.Join(PgTypeNames(), ", "))
	}
	return "", fmt.Errorf("unknown column type %q; it must be one of %s", dt, strings.Join(PgTypeNames(), ", "))
}

// PgTypeNames are the type names ToPgType accepts, sorted.
func PgTypeNames() []string {
	names := make([]string, 0, len(pgTypes))
	for name := range pgTypes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
	if err = ensureSchema(dbpool, tableDefinition.SchemaName); err != nil {
		return err
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

// ensureSchema creates schemaName if it does not exist and, when it did not,
// grants the restricted logins on it what they hold on every other schema
// (dblogin.GrantSchema). Their default privileges cover the tables created
// later but not a schema, so without it a schema created at run time stays out
// of their reach until the apiserver next starts and dblogin.Ensure runs.
//
// A failed grant does not fail the table update: the logins may not exist yet
// (update_db's migration runs before Ensure on a new database), and the next
// Ensure grants on every schema there is. It is logged.
func ensureSchema(dbpool *pgxpool.Pool, schemaName string) error {
	ctx := context.Background()
	var exists bool
	err := dbpool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", schemaName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error while looking for schema %s: %v", schemaName, err)
	}
	if exists {
		return nil
	}
	_, err = dbpool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgx.Identifier{schemaName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("error while creating schema: %v", err)
	}
	switch err = dblogin.GrantSchema(ctx, dbpool, schemaName); {
	case errors.Is(err, dblogin.ErrNoLogins):
		log.Printf("Created schema %s; %v, which grants on it when it runs", schemaName, err)
	case err != nil:
		log.Printf("Warning: created schema %s but could not grant the restricted logins on it, "+
			"so they cannot reach it until the apiserver next starts: %v", schemaName, err)
	default:
		log.Printf("Created schema %s and granted the restricted logins on it", schemaName)
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

// CreateTable creates the table from scratch, on a database where it has never
// existed.
//
// **A column or an index marked Deleted is a tombstone and is skipped here.** It
// exists in the definition so that UpdateTable can emit the DROP that removes it
// from a database that already has it (`UpdateTable`, the branch at
// `jets/schema/schema.go:434`); on a fresh install there is nothing to drop, and
// creating it is how a table gets a column no code writes.
//
// This did not test the flag until 2026-09-04, and the consequence was not
// cosmetic: `sharding_config_json` and `reducing_config_json` are deleted **and**
// `isNotNull` with no default, so a freshly created jetsapi.cpipes_execution_status
// could not be written by the sharding start's own insert, which names neither
// (`StartShardingComputePipes`, `jets/compute_pipes/actions_start_sharding_cp.go:385`).
// Running the migration a second time cleared it, which is why every long-lived
// deployment was unaffected and why it survived.
//
// **The two halves are coupled and skipping only the columns is worse than
// skipping neither.** Two deleted indexes are declared over deleted columns —
// compute_pipes_shard_registry_idx2 over jets_partition and _idx3 over sc_id — so a
// CreateTable that dropped the columns and kept the indexes would fail the fresh
// install outright rather than leaving it merely wrong.
func (tableDefinition *TableDefinition) CreateTable(dbpool *pgxpool.Pool) error {
	log.Println("Creating Table", tableDefinition.TableName)
	if dbpool == nil {
		return errors.New("error: dbpool required")
	}
	// A definition whose columns are all tombstones would emit "CREATE TABLE x()",
	// which is a syntax error naming the paren rather than the cause. Say so here.
	liveColumns := 0
	for i := range tableDefinition.Columns {
		if !tableDefinition.Columns[i].Deleted {
			liveColumns++
		}
	}
	if liveColumns == 0 {
		return fmt.Errorf(
			"error: table %s.%s has no column that is not marked deleted; nothing to create",
			tableDefinition.SchemaName, tableDefinition.TableName)
	}
	// The create statement is built before the existing table is dropped, so a
	// column type ToPgType refuses leaves the table as it was.
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	// colon defs
	buf.WriteString("(\n")
	isFirst := true
	for _, col := range tableDefinition.Columns {
		if col.Deleted {
			continue
		}
		if !isFirst {
			buf.WriteString(",\n")
		}
		isFirst = false
		buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
		buf.WriteString(" ")
		ctype, err := ToPgType(col.DataType)
		if err != nil {
			return fmt.Errorf("error: table %s.%s, column %s: %v",
				tableDefinition.SchemaName, tableDefinition.TableName, col.ColumnName, err)
		}
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
	// index defs -- a deleted index is a tombstone, see the note on this function
	for _, idx := range tableDefinition.Indexes {
		if idx.Deleted {
			continue
		}
		buf.WriteString("CREATE ")
		buf.WriteString(idx.IndexDef)
		buf.WriteString(" ;\n")
	}
	createStmt := buf.String()

	// drop stmt
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableDefinition.SchemaName, tableDefinition.TableName}.Sanitize())
	_, err := dbpool.Exec(context.Background(), stmt)
	if err != nil {
		return fmt.Errorf("error while droping table: %v", err)
	}
	// Execute the statements
	_, err = dbpool.Exec(context.Background(), createStmt)
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
			ctype, err := ToPgType(col.DataType)
			if err != nil {
				return fmt.Errorf("error: table %s.%s, column %s: %v",
					tableDefinition.SchemaName, existingSchema.TableName, col.ColumnName, err)
			}
			buf.WriteString(ctype)
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
			buf.WriteString(pgx.Identifier{constaint.Name}.Sanitize())
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
			buf.WriteString(pgx.Identifier{tableDefinition.SchemaName, idx.IndexName}.Sanitize())
			buf.WriteString(" ;\n")
		}
	}

	// Execute the statements
	stmt := buf.String()
	// //* PRINT STMT
	// if tableDefinition.TableName == "cpipes_execution_status_details" {
	// 	log.Println(stmt)
	// }
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
