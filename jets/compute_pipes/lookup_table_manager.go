package compute_pipes

import (
	"context"
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

// PrepareLookupTables loads every lookup. dbpool is the pipeline's pool: the
// sql_lookup queries do not run on it but on the reader login, opened from it
// when there is one to run and closed when they have loaded
// (lookup_reader_login.go).
func (mgr *LookupTableManager) PrepareLookupTables(dbpool *pgxpool.Pool) error {
	var reader *pgxpool.Pool
	for _, spec := range mgr.spec {
		if spec.Type == "sql_lookup" {
			var err error
			if reader, err = openLookupReader(context.Background(), dbpool); err != nil {
				return fmt.Errorf("while opening the database login the lookup queries run on: %w", err)
			}
			defer reader.Close()
			break
		}
	}
	for i := range mgr.spec {
		lookupTableConfig := mgr.spec[i]
		switch lookupTableConfig.Type {

		case "sql_lookup":
			tbl, err := NewLookupTableSql(reader, lookupTableConfig, mgr.envSettings, mgr.isVerbose)
			if err != nil {
				return fmt.Errorf("while calling NewLookupTableSql: %w", err)
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
