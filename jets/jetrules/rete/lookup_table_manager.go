package rete

import "fmt"

// LookupTableManager manages the LookupTable for JetStore Workspace
// This component was called LookupSqlHelper in the c++ version

type LookupTableManager struct {
	LookupTableMap *map[string]LookupTable
}

func NewLookupTableManager(jetruleModel *JetruleModel) (*LookupTableManager, error) {
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
			lookupTablesByName[lookupTableConfig.Name] = NewLookupTableSqlite3(lookupTableConfig)
		default:
			return nil, fmt.Errorf("error: unknown lookup_tables.data_file_info.format for lookup table %s", lookupTableConfig.Name)
		}
	}
	return &LookupTableManager{
		LookupTableMap: &lookupTablesByName,
	}, nil
}