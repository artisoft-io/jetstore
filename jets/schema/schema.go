package schema

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/artisoft-io/jetstore/jets/workspace"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file provide functions to manage the postgres table definition based on the workspace metadata
// and the provided extensions

// ExtTableInfo: multi value arg for extending tables with volatile fields
type ExtTableInfo map[string][]string

func UpdateTableSchema(dbpool *pgxpool.Pool, tableName string, tableSpec *workspace.DomainTable, dropExisting bool, extVR []string) (err error) {
	if len(tableSpec.Columns) == 0 {
		return fmt.Errorf("error: no tables provided from workspace")
	}
	// targetCols is a set of target schema + ext volatile resource
	targetCols := make(map[string]bool)
	for _, c := range tableSpec.Columns {
		targetCols[c.ColumnName] = true
	}
	for _,vr := range extVR {
		targetCols[vr] = true
	}
	tableExists := false
	if dbpool != nil && !dropExisting {
		tableExists, err = TableExists(dbpool, tableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called TableExists: %w", err)
		}
}
// var existingColumns map[string]workspace.DomainColumn
	if tableExists {
		existingColumns, err := GetDomainColumns(dbpool, tableName)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called GetDomainColumns: %w", err)
		}
			// check we are not missing any column
		for colName := range existingColumns {
			switch colName {
			case "shard_id", "last_update", "session_id":
				continue
			default:
				_,ok := targetCols[colName]
				if !ok {
					return fmt.Errorf("error: cannot update existing table with removed columns: %s", colName)
				}
			}
		}

		err = UpdateTable(dbpool, tableName, tableSpec.Columns, extVR, existingColumns)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called UpdateTable: %w", err)
		}
	} else {
		err = CreateTable(dbpool, tableName, tableSpec.Columns,extVR)
		if err != nil {
			return fmt.Errorf("while UpdateTableSchema called CreateTable: %w", err)
		}	
	}
	return nil
}

// Support Functions
// --------------------------------------------------------------------------------------
func UpdateTable(dbpool *pgxpool.Pool, tableName string, columns []workspace.DomainColumn, extVR []string, existingColumns map[string]workspace.DomainColumn) error {
	// alter stmt
	var buf strings.Builder
	buf.WriteString("ALTER TABLE IF EXISTS ")
	buf.WriteString(pgx.Identifier{tableName}.Sanitize())
	buf.WriteString(" ")
	isFirst := true
	for _, col := range columns {
		_, ok := existingColumns[col.ColumnName]
		if !ok {
			if !isFirst {
				buf.WriteString(", ")
			}
			//*
			fmt.Println("ADDING COLUMN:",col.ColumnName,"range",col.DataType,"is_array?",col.IsArray)
			isFirst = false
			buf.WriteString("ADD COLUMN IF NOT EXISTS ")
			buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
			buf.WriteString(" ")
			buf.WriteString(toPgType(col.DataType))
			if col.IsArray {
				buf.WriteString(" ARRAY")
			}
		}
	}
	for _,vr := range extVR {
		_, ok := existingColumns[vr]
		if !ok {
			if !isFirst {
				buf.WriteString(", ")
			}
			isFirst = false
			buf.WriteString("ADD COLUMN IF NOT EXISTS ")
			buf.WriteString(pgx.Identifier{vr}.Sanitize())
			buf.WriteString(" TEXT ARRAY")
		}
	}
	buf.WriteString(" ;")
	// note isFirst remains true if there is no column to add, therefre skip the update stmt
	stmt := buf.String()
	log.Println(stmt)
	if !isFirst {
		_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while updating table: %v", err)
		}
	} else {
		log.Println("No changes to table",tableName,"skipping it.")
	}
	return nil
}

func TableExists(dbpool *pgxpool.Pool, tableName string) (exists bool, err error) {
	if dbpool == nil {
		return false, nil
	}
	err = dbpool.QueryRow(context.Background(), "select exists (select from pg_tables where schemaname = 'public' and tablename = $1)", tableName).Scan(&exists)
	if err != nil {
		err = fmt.Errorf("QueryRow failed: %v", err)
	}
	return exists, err
}
func toPgType(dt string) string {
	switch dt {
	case "int":
		return "integer"
	case "uint", "long", "ulong":
		return "bigint"
	case "double":
		return "double precision"
	case "resource", "volatile_resource", "date", "datetime":
		return "text"
	default:
		return dt // date, text
	}
}

func CreateTable(dbpool *pgxpool.Pool, tableName string, columns []workspace.DomainColumn, extVR []string) error {
	// drop stmt
	stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", pgx.Identifier{tableName}.Sanitize())
	if dbpool != nil {
		_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while droping table: %v", err)
		}	
	} else {
		fmt.Println(stmt)
	}

	// create stmt
	var buf strings.Builder
	buf.WriteString("CREATE TABLE IF NOT EXISTS ")
	buf.WriteString(pgx.Identifier{tableName}.Sanitize())
	buf.WriteString("(")
	for _,col := range columns {
		buf.WriteString(pgx.Identifier{col.ColumnName}.Sanitize())
		buf.WriteString(" ")
		buf.WriteString(toPgType(col.DataType))
		if col.IsArray {
			buf.WriteString(" ARRAY")
		}
		buf.WriteString(", ")
	}
	// add ext columns
	for _,vr := range extVR {
		buf.WriteString(pgx.Identifier{vr}.Sanitize())
		buf.WriteString(" TEXT ARRAY, ")
	}
	buf.WriteString("session_id TEXT, ")
	buf.WriteString("shard_id integer DEFAULT 0 NOT NULL, ")
	buf.WriteString("last_update timestamp without time zone DEFAULT now() NOT NULL ")
	buf.WriteString(");")
	stmt = buf.String()
	if dbpool != nil {
		_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating table: %v", err)
		}
	} else {
		fmt.Println(stmt)
	}
	
	// primary index stmt
	stmt = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s  (jets__key, session_id, last_update DESC);", 
		pgx.Identifier{tableName+"_primary_idx"}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize())
	if dbpool != nil {
		_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating primary index: %v", err)
		}
	} else {
		fmt.Println(stmt)
	}

	// shard index stmt
	stmt = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (shard_id);", 
		pgx.Identifier{tableName+"_shard_idx"}.Sanitize(),
		pgx.Identifier{tableName}.Sanitize())
	if dbpool != nil {
			_, err := dbpool.Exec(context.Background(), stmt)
		if err != nil {
			return fmt.Errorf("error while creating shard index: %v", err)
		}
	} else {
		fmt.Println(stmt)
	}
	return nil
}

func GetDomainColumns(dbpool *pgxpool.Pool, tableName string) (map[string]workspace.DomainColumn, error) {
	result := make(map[string]workspace.DomainColumn)
	if dbpool == nil {
		return result, nil
	}
	rows, err := dbpool.Query(context.Background(), "SELECT column_name, data_type, udt_name FROM information_schema.columns WHERE table_name = $1", tableName)
	if err != nil {
		return result, fmt.Errorf("while getting volatile resources from workspace db: %v", err)
	}
	defer rows.Close()
	for rows.Next() { // Iterate and fetch the records from result cursor
		var dc workspace.DomainColumn
		var dt, udt string
		rows.Scan(&dc.ColumnName, &dt, &udt)
		if dt == "ARRAY" {
			dc.IsArray = true
		}
		udt = strings.TrimPrefix(udt, "_")
		switch udt {
		case "timestamp":
			dc.DataType = "datetime"
		case "int4":
			dc.DataType = "int"
		case "int8":
			dc.DataType = "long"
		case "float8":
			dc.DataType = "double"
		default:
			dc.DataType = udt
		}
		result[dc.ColumnName] = dc
	}
	return result, nil
}
