package wsfile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// This file contains function to save client configuration to local workspace

// SaveWorkspaceFileContent --------------------------------------------------------------------------
// Function to save client config in local workspace file system and in database
func SaveClientConfig(dbpool *pgxpool.Pool, workspaceName, clientName string) error {
	var buf strings.Builder
	var err error
	buf.WriteString("-- =============================================================================================================\n")
	buf.WriteString(fmt.Sprintf("-- %s\n", clientName))
	buf.WriteString("-- =============================================================================================================\n")
	buf.WriteString("-- Jets Database Init Script\n")

	// Approach, for each table of interest:
	//	- specify the delete and the select criteria
	//	- Get the list of columns and data type of the table from jets schema definition
	//	- Make the select statement with the list of columns
	//	- Create the insert statement with the ist of columns
	//	- Make an list of interface{} based on columns spec to read each row
	//	- Write each row based on based on the data type of the interface{}
	//  - Write the insert termination clause (ON CONFLICT DO NOTHING;)
	jetsSchema, err := getJetsSchema()
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("While reading JetStore DB schema from file: %v", err))
	}

	// jetsapi.client_registry
	tableName := "client_registry"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, true, true, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// jetsapi.client_org_registry
	tableName = "client_org_registry"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, true, true, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// jetsapi.source_config
	tableName = "source_config"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, true, false, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// jetsapi.rule_configv2
	tableName = "rule_configv2"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, true, false, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// jetsapi.process_input
	tableName = "process_input"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, false, true, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// jetsapi.pipeline_config
	tableName = "pipeline_config"
	err = writeTable(dbpool, jetsSchema, tableName, "client", clientName, true, false, &buf)
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
	}

	// Get the list of staging tables from source_config table
	stagingTables := make([]string, 0)
	stmt := fmt.Sprintf("SELECT table_name FROM jetsapi.source_config WHERE client = '%s'", clientName)
	rows, err := dbpool.Query(context.Background(), stmt)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf(fmt.Sprintf("While reading staging table names from table source_config: %v", err))
		}
	} else {
		for rows.Next() {
			var stagingTableName string
			if err := rows.Scan(&stagingTableName); err != nil {
				rows.Close()
				return err
			}
			stagingTables = append(stagingTables, stagingTableName)
		}
		rows.Close()	
	}

	// Write jetsapi.process_mapping
	tableName = "process_mapping"
	for i := range stagingTables {
		err = writeTable(dbpool, jetsSchema, tableName, "table_name", stagingTables[i], true, true, &buf)
		if err != nil {
			return fmt.Errorf(fmt.Sprintf("while reading table %s: %v", tableName, err))
		}	
	}
	buf.WriteString("\n-- End of Export Client Script\n")

	// Now save buf to file and database as an override
	err = SaveContent(dbpool, workspaceName, 
		fmt.Sprintf("process_config/%s_workspace_init_db.sql", strings.ToLower(clientName)),
		buf.String())
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("While saving %s client to local worspace and DB: %v", clientName, err))
	}
	
	return nil
}

// Write table values into buf, assuming whereColumn is of type text
func writeTable(dbpool *pgxpool.Pool, jetsSchema *map[string]schema.TableDefinition, 
	tableName, whereColumn, whereValue string, skipKeyColumn, oneFieldPerRow bool, buf *strings.Builder) error {

	buf.WriteString(fmt.Sprintf("\n-- Table %s\n", tableName))
	columnNames, err := getColumns(jetsSchema, skipKeyColumn, tableName)
	if err != nil {
		return err
	}
	columnNamesStr := strings.Join(columnNames, ",")
	buf.WriteString(fmt.Sprintf("DELETE FROM jetsapi.\"%s\" WHERE \"%s\" = '%s';\n", tableName, whereColumn, whereValue))
	stmt := fmt.Sprintf("SELECT %s FROM jetsapi.\"%s\" WHERE \"%s\" = '%s'", columnNamesStr, tableName, whereColumn, whereValue)
	rows, err := dbpool.Query(context.Background(), stmt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf(fmt.Sprintf("While reading from table %s: %v", tableName, err))
	}
	defer rows.Close()
	isFirst := true
	for rows.Next() {
		columnValues, err := makeColumnValues(jetsSchema, skipKeyColumn, tableName)
		if err != nil {
			return err
		}
		if err := rows.Scan(columnValues...); err != nil {
			return err
		}
		if !isFirst {
			buf.WriteString(",\n")
		} else {
			// Write the INSERT only if has rows
			buf.WriteString(fmt.Sprintf("INSERT INTO jetsapi.\"%s\" (%s) VALUES\n", tableName, columnNamesStr))
		}
		isFirst = false
		writeRow(oneFieldPerRow, buf, &columnValues)
	}
	if isFirst {
		buf.WriteString(fmt.Sprintf("-- Table %s has no row for %s = %s\n", tableName, whereColumn, whereValue))
	} else {
		buf.WriteString("\nON CONFLICT DO NOTHING;\n\n")
	}
	
	return nil
}

func escapeTics(str string) string {
	return strings.ReplaceAll(str, "'", "''")
}

func getColumns(jetsSchema *map[string]schema.TableDefinition, skipKeyColumn bool, tableName string) ([]string, error) {
	result := make([]string, 0)
	tableDef, ok := (*jetsSchema)[tableName]
	if !ok {
		return nil, fmt.Errorf("table definition not found for table %s", tableName)
	}
	for i := range tableDef.Columns {
		if tableDef.Columns[i].ColumnName != "last_update" && (!skipKeyColumn || tableDef.Columns[i].ColumnName != "key") {
			result = append(result, tableDef.Columns[i].ColumnName)
		}
	}
	return result, nil
}

// Prepare the container that will hold a row of values
// The returned []interface{} is a list of pointers to the proper sql type based on the schema
func makeColumnValues(jetsSchema *map[string]schema.TableDefinition, skipKeyColumn bool, tableName string) ([]interface{}, error) {
	result := make([]interface{}, 0)
	tableDef, ok := (*jetsSchema)[tableName]
	if !ok {
		return nil, fmt.Errorf("table definition not found for table %s", tableName)
	}
	for i := range tableDef.Columns {
		if tableDef.Columns[i].ColumnName != "last_update" && (!skipKeyColumn || tableDef.Columns[i].ColumnName != "key") {
			switch tableDef.Columns[i].DataType {
			case "int", "bool":
				if tableDef.Columns[i].IsArray {
					result = append(result, &sql.NullString{})
				} else {
					result = append(result, &sql.NullInt32{})
				}
			case "uint", "long", "ulong":
				if tableDef.Columns[i].IsArray {
					result = append(result, &sql.NullString{})
				} else {
					result = append(result, &sql.NullInt64{})
				}
			case "double":
				if tableDef.Columns[i].IsArray {
					result = append(result, &sql.NullString{})
				} else {
					result = append(result, &sql.NullFloat64{})
				}
			default:
				result = append(result, &sql.NullString{})
			}
		}
	}
	return result, nil
}

func writeRow(oneFieldPerRow bool, buf *strings.Builder, columnValues *[]interface{}) {
	filedDelimit := ",\n   "
	if oneFieldPerRow {
		filedDelimit = ", "
	}
	isFirst := true
	buf.WriteString("  (")
	for i := range *columnValues {
		if !isFirst {
			buf.WriteString(filedDelimit)
		}
		isFirst = false
		switch v := ((*columnValues)[i]).(type) {
		case *sql.NullString:
			if v.Valid {
				buf.WriteString(fmt.Sprintf("'%v'", escapeTics(v.String)))
			} else {
				buf.WriteString("NULL")
			}
		case *sql.NullInt32:
			if v.Valid {
				buf.WriteString(fmt.Sprintf("%v", v.Int32))
			} else {
				buf.WriteString("NULL")
			}
		case *sql.NullBool:
			if v.Valid {
				buf.WriteString(fmt.Sprintf("%v", v.Bool))
			} else {
				buf.WriteString("NULL")
			}
		case *sql.NullInt64:
			if v.Valid {
				buf.WriteString(fmt.Sprintf("%v", v.Int64))
			} else {
				buf.WriteString("NULL")
			}
		case *sql.NullFloat64:
			if v.Valid {
				buf.WriteString(fmt.Sprintf("%v", v.Float64))
			} else {
				buf.WriteString("NULL")
			}
		}
	}
	buf.WriteString(")")
}

func getJetsSchema() (*map[string]schema.TableDefinition, error) {
	// read jetstore sys tables definition using schema in json from location specified by env var
	schemaFname := os.Getenv("JETS_SCHEMA_FILE")
	if len(schemaFname) == 0 {
		schemaFname = "jets_schema.json"
	}
	// open json file
	file, err := os.Open(schemaFname)
	if err != nil {
		return nil, fmt.Errorf("error while opening jetstore schema file: %v", err)
	}
	defer file.Close()
	// open and decode the schema definition
	dec := json.NewDecoder(file)
	var schemaDef []schema.TableDefinition
	if err := dec.Decode(&schemaDef); err != nil {
		return nil, fmt.Errorf("error while decoding jstore schema: %v", err)
	}
	result := make(map[string]schema.TableDefinition)
	for i := range schemaDef {
		result[schemaDef[i].TableName] = schemaDef[i]
	}
	return &result, nil
}
