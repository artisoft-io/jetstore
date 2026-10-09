package compute_pipes

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/compute_pipes/pipesmodel"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LookupTableManager manages all lookup tables

type LookupTableManager struct {
	spec           []*LookupSpec
	envSettings    map[string]any
	LookupTableMap map[string]LookupTable
	isVerbose      bool
}

// LookupTable moved to `pipesmodel` with Phase 9's P9-T02, because it is what
// OperatorArgs.Lookups hands to a site operator: the contract package has to be
// able to name the type it carries. The alias keeps this package's spelling, so
// LookupTableS3, LookupTableSql and every built-in that holds one are unchanged
// — and a site operator is handed the same object a built-in gets rather than a
// second interface shaped for it.
type LookupTable = pipesmodel.LookupTable

// Lookup is one entry of OperatorArgs.Lookups: a loaded table beside the key the
// step named it by. Re-exported so a site names one package rather than two.
type Lookup = pipesmodel.Lookup

func NewLookupTableManager(spec []*LookupSpec, envSettings map[string]any, isVerbose bool) *LookupTableManager {
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
