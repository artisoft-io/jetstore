package compute_pipes

import (
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

// LookupTableManager manages all lookup tables

type LookupTableManager struct {
	spec           []*LookupSpec
	envSettings    map[string]interface{}
	LookupTableMap map[string]LookupTable
	isVerbose      bool
}

type LookupTable interface {
	// Returns the lookup row associated with key
	Lookup(key *string) (*[]interface{}, error)
	// Returns the row's value associated with the lookup column
	LookupValue(row *[]interface{}, columnName string) (interface{}, error)
	// Returns the mapping between column name to pos in the returned row
	ColumnMap() map[string]int
	// Return true if the table is empty, ColumnMap is empty as well
	IsEmptyTable() bool
}

func NewLookupTableManager(spec []*LookupSpec, envSettings map[string]interface{}, isVerbose bool) *LookupTableManager {
	return &LookupTableManager{
		spec:           spec,
		envSettings:    envSettings,
		LookupTableMap: make(map[string]LookupTable),
		isVerbose:      isVerbose,
	}
}

func (mgr *LookupTableManager) PrepareLookupTables(dbpool *pgxpool.Pool) error {
	for i := range mgr.spec {
		lookupTableConfig := mgr.spec[i]
		switch lookupTableConfig.Type {
		case "sql_lookup":
			tbl, err := NewLookupTableSql(dbpool, lookupTableConfig, mgr.envSettings, mgr.isVerbose)
			if err != nil {
				return fmt.Errorf("while calling NewLookupTableSql: %v", err)
			}
			mgr.LookupTableMap[lookupTableConfig.Key] = tbl
		case "s3_csv_lookup":
			tbl, err := NewLookupTableS3(dbpool, lookupTableConfig, mgr.envSettings, mgr.isVerbose)
			if err != nil {
				return fmt.Errorf("while calling NewLookupTableS3: %v", err)
			}
			mgr.LookupTableMap[lookupTableConfig.Key] = tbl
		default:
			return fmt.Errorf("error:unknown lookup table type: %s", lookupTableConfig.Type)
		}
	}
	return nil
}
