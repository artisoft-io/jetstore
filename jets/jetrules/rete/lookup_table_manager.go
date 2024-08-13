package rete

import (
	"database/sql"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	_ "modernc.org/sqlite"
)

// LookupTableManager manages the LookupTable for JetStore Workspace
// This component was called LookupSqlHelper in the c++ version

type LookupTableManager struct {
	LookupTableMap map[string]LookupTable
}

// NewLookupTableManager creates the LookupTableManager which is used in the context of a
// single rete network (main rule file or a rule sequence).
// This manager opens the DB connection to the sqlite file lookup.db located in the workspace root.
// This connection is used by the lookup table of type 'sqlite3' (default type when not specified).
func NewLookupTableManager(rmgr *rdf.ResourceManager, metaGraph *rdf.RdfGraph, jetruleModel *JetruleModel) (*LookupTableManager, error) {
	// Open the connection to the lookup.db
	dbPath := fmt.Sprintf("%s/%s/lookup.db", workspaceHome, wprefix)
	var lookupDb *sql.DB
	var err error

	lookupTablesByName := make(map[string]LookupTable)
	for i := range jetruleModel.LookupTables {
		lookupTableConfig := &jetruleModel.LookupTables[i]
		lookupTableDataInfo := lookupTableConfig.DataInfo
		dataStorageFormat := "sqlite3"
		if lookupTableDataInfo != nil {
			dataStorageFormat = lookupTableDataInfo.Format
		}
		switch dataStorageFormat {
		case "sqlite3":
			if lookupDb == nil {
				// Open the DB since we have a sqlite3 lookup
				lookupDb, err = sql.Open("sqlite", dbPath)
				if err != nil {
					return nil, fmt.Errorf("while opening lookup.db: %v", err)
				}
			}
			lookupTable, err := NewLookupTableSqlite3(rmgr, metaGraph, lookupTableConfig, lookupDb)
			if err != nil {
				return nil, fmt.Errorf("while creating lookup talbe %s: %v", lookupTableConfig.Name, err)
			}
			lookupTablesByName[lookupTableConfig.Name] = lookupTable
		default:
			return nil, fmt.Errorf("error: unknown lookup_tables.data_file_info.format for lookup table %s", lookupTableConfig.Name)
		}
	}
	return &LookupTableManager{
		LookupTableMap: lookupTablesByName,
	}, nil
}