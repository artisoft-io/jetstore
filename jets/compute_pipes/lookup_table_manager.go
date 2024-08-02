package compute_pipes

import (
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

// LookupTableManager manages all lookup tables

type LookupTableManager struct {
	spec           []*LookupSpec
	LookupTableMap *map[string]LookupTable
}

type LookupTable interface {
	// Returns the lookup row associated with key
	Lookup(key *string) (*[]interface{}, error)
	// Returns the row's value associated with the lookup column
	LookupValue(row *[]interface{}, columnName string) (interface{}, error)
	// Returns the mapping between column name to pos in the returned row
	ColumnMap() map[string]int
}

func NewLookupTableManager(spec []*LookupSpec) *LookupTableManager {
	lookupTables := make(map[string]LookupTable)
	return &LookupTableManager{
		spec:           spec,
		LookupTableMap: &lookupTables,
	}
}

func (mgr *LookupTableManager) PrepareLookupTables(dbpool *pgxpool.Pool) error {
	for i := range mgr.spec {
		lookupTableConfig := mgr.spec[i]
		switch lookupTableConfig.Type {
		case "sql_lookup":
			tbl, err := NewLookupTableSql(dbpool, lookupTableConfig)
			if err != nil {
				return fmt.Errorf("while calling NewLookupTableSql: %v", err)
			}
			(*mgr.LookupTableMap)[lookupTableConfig.Key] = tbl
		default:
			return fmt.Errorf("error:unknown lookup table type: %s", lookupTableConfig.Type)
		}
	}
	return nil
}
