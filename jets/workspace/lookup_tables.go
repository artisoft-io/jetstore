package workspace

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/csv"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// This file contains functions to package lookup tables into lookup.db

// Example of create table statement:
//
// DROP TABLE IF EXISTS "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG";
// CREATE TABLE "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG" (
//    __key__            INTEGER,
//    "jets:key"         TEXT NOT NULL,
//*    "NF_FEE"  REAL,
//*    "ONF_FEE"  REAL
//  );
//
// CREATE INDEX IF NOT EXISTS "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG_idx"
// 	ON "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG" ("jets:key");
//
// CREATE INDEX IF NOT EXISTS "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG_key_idx"
// 	ON "PHYSICIAN_FEE_SCHEDULE_LOOKUP_CONFIG" ("__key__");
//
// Column marked with * are specific to the lookup table
//
// From the original python code:
// To convert from the jetrule type to sqlite type
// def _convert_jetrule_type(self, jr_type: str) -> str:
//   if jr_type in  ['text', 'resource', 'date', 'datetime'] :
//       sqlite_type = 'TEXT'
//   elif jr_type in ['int','bool','uint', 'long', 'ulong']:
///        sqlite_type = 'INTEGER'
//   elif jr_type == 'double':
//        sqlite_type = 'REAL'
//   else:
//       raise Exception('_convert_jetrule_type: Type not supported: ' + jr_type)
//   return sqlite_type

func PackageLookupTablesToSqlite(lookupTables []*rete.LookupTableNode) error {
	// Open the connection to the lookup.db
	dbPath := fmt.Sprintf("%s/%s/lookup.db", workspaceHome, wprefix)
	var lookupDb *sql.DB
	var err error

	lookupDb, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("while opening lookup.db: %v", err)
	}
	defer lookupDb.Close()

	// make sure each lookup is processed only once
	processedLookup := make(map[string]bool)

	// For each lookup table, create the table in lookup.db
	for _, lookupTbl := range lookupTables {
		if _, exists := processedLookup[lookupTbl.Name]; exists {
			// already processed
			continue
		}
		processedLookup[lookupTbl.Name] = true
		
		log.Printf("Processing lookup table %s", lookupTbl.Name)
		createStmt := makeCreateStatement(lookupTbl)
		_, err = lookupDb.Exec(createStmt)
		if err != nil {
			log.Println("Create statement was:", createStmt)
			return fmt.Errorf("while creating lookup table %s: %v", lookupTbl.Name, err)
		}
		log.Printf("Created lookup table %s in lookup.db", lookupTbl.Name)

		// Load the data file to memory
		switch {
		case len(lookupTbl.CsvFile) > 0:
			filePath := fmt.Sprintf("%s/%s/%s", workspaceHome, wprefix, lookupTbl.CsvFile)
			// Load the CSV file into memory
			fs, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("while opening lookup table csv file %s: %v", filePath, err)
			}
			defer fs.Close()
			reader := csv.NewReader(fs)
			reader.LazyQuotesSpecial = true
			records, err := reader.ReadAll()
			if err != nil {
				return fmt.Errorf("while reading lookup table csv file %s: %v", filePath, err)
			}
			if len(records) < 1 {
				return fmt.Errorf("lookup table csv file %s is empty", filePath)
			}
			// First row is the header
			header := records[0]
			// Map header name to position
			headerPos := make(map[string]int)
			for i, colName := range header {
				headerPos[colName] = i
			}
			// Prepare the insert statement
			var buf strings.Builder
			buf.WriteString("INSERT INTO \"")
			buf.WriteString(lookupTbl.Name)
			buf.WriteString("\" VALUES ")
			outputRec := make([]string, len(lookupTbl.Columns)+2)
			var keyPos []int
			// Determine the position of the key columns
			for _, key := range lookupTbl.Key {
				if pos, exists := headerPos[key]; exists {
					keyPos = append(keyPos, pos)
				} else {
					return fmt.Errorf("key column %s not found in csv file %s for lookup table %s", key, filePath, lookupTbl.Name)
				}
			}
			// keep track of the rowid for each unique jets:key
			// rowid starts at 1
			rowIdByJetsKey := make(map[string]int)
			// For each record, map the columns to the output record
			isFirst := true
			for _, record := range records[1:] {
				if !isFirst {
					buf.WriteString(",\n")
				} else {
					buf.WriteString("\n")
				}
				isFirst = false

				// Prepare the output record
				// First column is __key__ is the rowid of each unique jets:key
				// Second column is "jets:key"
				// This may be a composite column, compute the "jets:key" value
				if len(keyPos) == 1 {
					outputRec[1] = quote(record[keyPos[0]])
				} else {
					var keyParts []string
					for _, kp := range keyPos {
						keyParts = append(keyParts, record[kp])
					}
					outputRec[1] = quote(strings.Join(keyParts, ""))
				}
				// Get the rowid for this jets:key
				if rid, exists := rowIdByJetsKey[outputRec[1]]; exists {
					// Existing jets:key, use the existing rowid
					outputRec[0] = strconv.Itoa(rid)
				} else {
					// New jets:key, assign a new rowid
					newRowId := len(rowIdByJetsKey) + 1
					rowIdByJetsKey[outputRec[1]] = newRowId
					outputRec[0] = strconv.Itoa(newRowId)
				}

				// Other columns
				for i, col := range lookupTbl.Columns {
					if pos, exists := headerPos[col.Name]; exists {
						// Convert the value to the appropriate type based on col.Type
						val := record[pos]
						switch col.Type {
						case "text", "string", "resource", "date", "datetime":
							outputRec[i+2] = quote(val)
						case "int", "int32", "int64", "uint", "long", "ulong":
							outputRec[i+2] = val
						case "double":
							outputRec[i+2] = val
						case "bool":
							if rdf.ParseBool(val) == 0 {
								outputRec[i+2] = "0"
							} else {
								outputRec[i+2] = "1"
							}
						default:
							outputRec[i+2] = quote(val) // Default to string
						}
					} else {
						outputRec[i+2] = "''" // Column not found, set to NULL
					}
				}
				buf.WriteString("(")
				buf.WriteString(strings.Join(outputRec, ","))
				buf.WriteString(")")

			}
			buf.WriteString(";\n")
			// Execute the insert statement
			// fmt.Println("Insert statement is:\n", buf.String())
			_, err = lookupDb.Exec(buf.String())
			if err != nil {
				return fmt.Errorf("while inserting record into lookup table %s: %v", lookupTbl.Name, err)
			}

			log.Printf("Inserted %d records into lookup table %s from csv file %s", len(records)-1, lookupTbl.Name, filePath)
		default:
			return fmt.Errorf("no data file specified or file type not supported for lookup table %s", lookupTbl.Name)
		}
	}
	return nil
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func makeCreateStatement(lookupTbl *rete.LookupTableNode) string {
	var buf strings.Builder

	buf.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS %q;\n", lookupTbl.Name))
	buf.WriteString(fmt.Sprintf("CREATE TABLE %q (\n", lookupTbl.Name))
	buf.WriteString("    __key__            INTEGER,\n")
	buf.WriteString("    \"jets:key\"         TEXT NOT NULL,\n")
	isFirst := true
	for _, col := range lookupTbl.Columns {
		if !isFirst {
			buf.WriteString(",\n")
		}
		isFirst = false
		// Convert jetrule type to sql type
		sqlType := convertJetruleType(col.Type)
		buf.WriteString(fmt.Sprintf("    %q %s", col.Name, sqlType))
	}
	buf.WriteString(");\n")

	buf.WriteString(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %q ON %q (\"jets:key\");\n", lookupTbl.Name+"_idx", lookupTbl.Name))
	buf.WriteString(fmt.Sprintf("CREATE INDEX IF NOT EXISTS %q ON %q (__key__);\n", lookupTbl.Name+"_key_idx", lookupTbl.Name))

	return buf.String()
}

func convertJetruleType(jrType string) string {
	switch jrType {
	case "text", "string", "resource", "date", "datetime":
		return "TEXT"
	case "int", "int64", "int32", "bool", "uint", "long", "ulong":
		return "INTEGER"
	case "double":
		return "REAL"
	default:
		log.Printf("WARNING convertJetruleType: Type not supported: %s", jrType)
		return "TEXT"
	}
}
