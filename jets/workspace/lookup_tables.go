package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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

// PackageLookupTablesToSqlite rebuilds the workspace's lookup.db
// unconditionally. The compile calls EnsureLookupDb, which skips the rebuild
// when the inputs are unchanged (lookup_db_fingerprint.go).
func PackageLookupTablesToSqlite(lookupTables []*rete.LookupTableNode) error {
	wsDir := fmt.Sprintf("%s/%s", workspaceHome, wprefix)
	tables := uniqueLookupTables(lookupTables)
	sqliteVersion, err := sqliteLibraryVersion()
	if err != nil {
		return fmt.Errorf("while reading the sqlite library version: %w", err)
	}
	fingerprint, err := lookupDbFingerprint(tables, wsDir, sqliteVersion)
	if err != nil {
		fingerprint = ""
	}
	return packageLookupTables(tables, wsDir, filepath.Join(wsDir, "lookup.db"), fingerprint, sqliteVersion)
}

// packageLookupTables builds lookup.db from tables, which must already be
// deduplicated, and records fingerprint in it when there is one.
//
// **It builds a new file and renames it over dbPath**, where it used to drop
// and recreate each table inside the existing file. That left a lookup removed
// from the rules in lookup.db for good, let a damaged file break the build, and
// meant a rebuild over an old file did not produce the bytes of a fresh one —
// which is what a skipped rebuild has to equal. The rename also means a failed
// build leaves the previous lookup.db in place rather than half of a new one.
func packageLookupTables(tables []*rete.LookupTableNode, wsDir, dbPath, fingerprint, sqliteVersion string) error {
	buildPath := dbPath + ".building"
	for _, p := range []string{buildPath, buildPath + "-journal", buildPath + "-wal", buildPath + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("while removing %s: %v", p, err)
		}
	}
	if err := buildLookupDb(tables, wsDir, buildPath, fingerprint, sqliteVersion); err != nil {
		os.Remove(buildPath)
		return err
	}
	// A journal left beside the old file by a crash would be replayed into the
	// new one on its next open.
	for _, p := range []string{dbPath + "-journal", dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("while removing %s: %v", p, err)
		}
	}
	if err := os.Rename(buildPath, dbPath); err != nil {
		return fmt.Errorf("while moving the new lookup.db into place: %v", err)
	}
	return nil
}

func buildLookupDb(tables []*rete.LookupTableNode, wsDir, dbPath, fingerprint, sqliteVersion string) error {
	lookupDb, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("while opening lookup.db: %v", err)
	}
	defer lookupDb.Close()

	// For each lookup table, create the table in lookup.db
	for _, lookupTbl := range tables {
		log.Printf("Processing lookup table %s", lookupTbl.Name)
		createStmt := makeCreateStatement(lookupTbl)
		_, err = lookupDb.Exec(createStmt)
		if err != nil {
			log.Println("Create statement was:", createStmt)
			return fmt.Errorf("while creating lookup table %s: %v", lookupTbl.Name, err)
		}

		// Load the data file to memory
		switch {
		case len(lookupTbl.CsvFile) > 0:
			filePath := fmt.Sprintf("%s/%s", wsDir, lookupTbl.CsvFile)
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
			for irec, record := range records[1:] {
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
							// Empty is null and anything outside the accepted
							// spellings stops the compile, naming where (I-45).
							// Both engines serve a NULL bool column as rdf null.
							b, err := rdf.ParseBool(val)
							switch {
							case errors.Is(err, rdf.ErrNullBool):
								outputRec[i+2] = "NULL"
							case err != nil:
								return fmt.Errorf("lookup table %s, csv file %s, row %d, column %s: %v",
									lookupTbl.Name, filePath, irec+2, col.Name, err)
							case b:
								outputRec[i+2] = "1"
							default:
								outputRec[i+2] = "0"
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
	// Last, so a file cut short is unlikely to carry it.
	if fingerprint != "" {
		if err = writeLookupDbFingerprint(lookupDb, fingerprint, sqliteVersion); err != nil {
			return fmt.Errorf("while recording the lookup.db fingerprint: %v", err)
		}
	}
	return lookupDb.Close()
}

func quote(s string) string {
	if "NULL" == strings.ToUpper(s) {
		return "NULL"
	}
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
