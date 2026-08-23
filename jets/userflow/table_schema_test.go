package userflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tablesDir holds the 37 shipping table configurations, translated out of the
// Flutter corpus by `jetsclient_ide/src/datatable/table.test.ts` and committed
// so this side can validate the *real* configuration rather than a sample. See
// that file for why the translation is round-tripped rather than only emitted.
const tablesDir = "../../jetsclient_ide/src/datatable/tables"

func tableFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(tablesDir)
	if err != nil {
		t.Fatalf("reading %s: %v", tablesDir, err)
	}
	var files []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tc.json") {
			files = append(files, filepath.Join(tablesDir, entry.Name()))
		}
	}
	return files
}

// TestShippingTablesValidate is the rule again, on a fourth document type: a
// real configuration that fails means the schema is wrong, not the configuration.
//
// **Through ValidateTableDocument rather than through the compiled schema**,
// unlike TestShippingFlowsValidate — this is the function the save path actually
// dispatches to, and the difference between the two has bitten before (the
// mutation-testing note in `jets/datatable/workspace_file_validators.go`).
func TestShippingTablesValidate(t *testing.T) {
	files := tableFiles(t)
	if len(files) != 37 {
		t.Fatalf("expected the flows' 37 table configurations, found %d", len(files))
	}
	for _, path := range files {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".tc.json"), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			if findings := ValidateTableDocument(string(content)); len(findings) > 0 {
				t.Errorf("%s does not validate:\n%v", path, findings)
			}
		})
	}
}

// TestTableSchemaRejects probes it from the other side, with the cases that are
// specific to this document rather than generic to JSON Schema. The suite proper
// is in `negative_suite.json` and runs in both languages.
func TestTableSchemaRejects(t *testing.T) {
	cases := map[string]string{
		// The security cut, and the reason it is first. `apiAction` reaches
		// `DataTableAction.Action` and dispatches over the whole switch in
		// `jets/apiserver/api_tables.go:42`. The document has no such field and
		// every object is closed, so naming one is a schema error rather than a
		// policy decision taken somewhere else.
		"an apiAction": `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a","label":"A"}],
			"sortColumn":"a","rowsPerPage":10,"from":[{"schema":"jetsapi","table":"t"}],"apiAction":"exec_ddl"}`,
		// The two arms of the column union, from the other side: 22 of 275
		// columns have no label and every one is hidden.
		"a visible column with no label": `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a"}],
			"sortColumn":"a","rowsPerPage":10,"from":[{"schema":"jetsapi","table":"t"}]}`,
		"a static table that names a schema": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"rows":[["x"]],
			"from":[{"schema":"jetsapi","table":"t"}]}`,
		"a query table with no from clause": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"from":[]}`,
		"an unknown action type": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,
			"from":[{"schema":"jetsapi","table":"t"}],
			"actions":[{"key":"k","label":"L","action":"exec_ddl","style":"primary"}]}`,
		"no source at all": `{"schemaVersion":1,"label":"L","columns":[{"name":"a","label":"A"}],
			"sortColumn":"a","rowsPerPage":10}`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if findings := ValidateTableDocument(doc); len(findings) == 0 {
				t.Errorf("accepted %s", name)
			}
		})
	}
}

// TestValidTableDocumentIsAccepted is the other half — a suite whose bases do not
// pass proves nothing, because everything would fail for the same uninteresting
// reason.
func TestValidTableDocumentIsAccepted(t *testing.T) {
	const doc = `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a","label":"A"},
		{"name":"b","isHidden":true}],"sortColumn":"a","rowsPerPage":10,
		"from":[{"schema":"jetsapi","table":"t"}]}`
	if findings := ValidateTableDocument(doc); len(findings) > 0 {
		t.Errorf("rejected a valid document: %v", findings)
	}
}
