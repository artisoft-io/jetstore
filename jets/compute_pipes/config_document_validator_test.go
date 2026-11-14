package compute_pipes

import (
	"encoding/json"
	"testing"

	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The save check refuses an output table column type exactly when the start
// does (validateOutputTableColumnTypes, which ValidatePipeSpecConfig calls), for
// every name in the closed list and for names outside it, on every column name.
func TestConfigDocumentColumnTypesAgreeWithTheStart(t *testing.T) {
	types := append(schema.PgTypeNames(), "", "money", "TEXT", "text[]", "text); DROP TABLE x; --")
	for _, column := range []string{"jets:key", "session_id", "amount"} {
		for _, rdfType := range types {
			cfg := &ComputePipesConfig{
				ClusterConfig: &ClusterSpec{},
				OutputTables: []*TableSpec{{Key: "out", Name: "public.out",
					Columns: []TableColumnSpec{{Name: column, RdfType: rdfType}}}},
			}
			raw, err := json.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			startRefuses := validateOutputTableColumnTypes(cfg) != nil
			saveRefuses := len(wsvalidate.ErrorsOnly(ValidateConfigDocument(string(raw)))) > 0
			if startRefuses != saveRefuses {
				t.Errorf("column %q, rdf_type %q: the start refuses it: %v, the save refuses it: %v",
					column, rdfType, startRefuses, saveRefuses)
			}
		}
	}
}
