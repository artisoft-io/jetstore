package datatable

// A numeric column of a table that joins two FROM entries sorts as a number.
//
// sortColumnIdentifier qualifies a sort column by the request's table, else the
// column's own table, else the only FROM entry. A table joining two FROM entries
// has no only entry, so a numeric column that names no table stays bare, and a
// bare ORDER BY name resolves to the text cast output column: 10 before 8. These
// documents name the table on each such column, which is what the client sends
// as the sort column's table on a header click (resolveSortColumn,
// jetsclient_ide/src/datatable/model.ts).

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The tables that join source_period to show a run's or a file's period.
var joinedNumericDocs = []string{
	"../workspace_assets/table_configs/pipelineExecStatusTable.tc.json",
	"../workspace_assets/table_configs/lfFileKeyStagingTable.tc.json",
	"../../jetsclient_ide/src/datatable/tables/inputLoaderStatusTable.tc.json",
	"../../jetsclient_ide/src/datatable/tables/pipelineArchiveTable.tc.json",
}

func TestJoinedTablesSortNumericColumnsAsNumbers(t *testing.T) {
	for _, path := range joinedNumericDocs {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Columns []struct {
				Name         string `json:"name"`
				Table        string `json:"table"`
				CalculatedAs string `json:"calculatedAs"`
				IsNumeric    bool   `json:"isNumeric"`
			} `json:"columns"`
			From []FromClause `json:"from"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if len(doc.From) < 2 {
			t.Fatalf("%s: %d FROM entries, want the join this test is about", path, len(doc.From))
		}
		action := &DataTableAction{FromClauses: doc.From, Limit: 10}
		for _, c := range doc.Columns {
			action.Columns = append(action.Columns, Column{Table: c.Table, Column: c.Name, CalculatedAs: c.CalculatedAs})
		}
		numeric := 0
		for _, c := range doc.Columns {
			if !c.IsNumeric || c.CalculatedAs != "" {
				continue
			}
			numeric++
			action.SortColumn, action.SortColumnTable = c.Name, c.Table
			got := orderByOf(t, action)
			if !strings.Contains(got, ".") {
				t.Errorf("%s: a header click on %s orders by %s, which sorts its text", path, c.Name, got)
			}
		}
		if numeric == 0 {
			t.Errorf("%s: no numeric column", path)
		}
	}
}
