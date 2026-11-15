package datatable

// A numeric column of a table that joins several FROM entries sorts as a number.
//
// sortColumnIdentifier qualifies a sort column by the request's table, else the
// column's own table, else the only FROM entry. A table joining two FROM entries
// has no only entry, so a numeric column that names no table stays bare, and a
// bare ORDER BY name resolves to the text cast output column: 10 before 8. Every
// such column must therefore name its table in the document, which is what the
// client sends as the sort column's table on a header click (resolveSortColumn,
// jetsclient_ide/src/datatable/model.ts).
//
// Every table document is checked, bundled and installed, so a new document
// with a join cannot add the defect back.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Where the table documents are: the ones the web app bundles, and the ones
// installed into every workspace.
var tableDocumentDirs = []string{
	"../../jetsclient_ide/src/datatable/tables",
	"../workspace_assets/table_configs",
}

// Documents this test must find with a join and a numeric column, so that a
// moved directory cannot make it pass by checking nothing.
var knownJoinedNumericDocs = []string{
	"pipelineExecStatusTable.tc.json",
	"lfFileKeyStagingTable.tc.json",
	"inputLoaderStatusTable.tc.json",
	"pipelineArchiveTable.tc.json",
	"fmInputSourceMappingUF.tc.json",
}

func TestJoinedTablesSortNumericColumnsAsNumbers(t *testing.T) {
	var paths []string
	for _, dir := range tableDocumentDirs {
		found, err := filepath.Glob(filepath.Join(dir, "*.tc.json"))
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, found...)
	}
	checked := map[string]bool{}
	for _, path := range paths {
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
			continue
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
		if numeric > 0 {
			checked[filepath.Base(path)] = true
		}
	}
	for _, name := range knownJoinedNumericDocs {
		if !checked[name] {
			t.Errorf("%s was not checked: no such document, or it has no join or no numeric column", name)
		}
	}
}
