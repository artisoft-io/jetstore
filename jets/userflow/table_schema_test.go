package userflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tablesDir holds the 50 shipping table configurations, all but two translated out of the
// Flutter corpus by `jetsclient_ide/src/datatable/table.test.ts` and committed
// so this side can validate the *real* configuration rather than a sample. See
// that file for why the translation is round-tripped rather than only emitted.
//
// **37 of them are the flows' and ten are not.** `pipelineExecStatusTable` is
// registered on the non-flow side and rendered by `homeFiltersUF` (plan F18), so
// F.5 could not author that flow without it; `workspaceRegistryTable` is the
// `/workspaces` screen's, added by C.2; C.7 added `pipelineExecDetailsTable` and
// `cpipesExecDetailsTable`, the tables of `/executionStatusDetails/:session_id`
// and `/executionStatsDetails/:session_id`. All four are translated out of
// `screens/fixtures/screen_configs.json` by the same code path, and the count
// here grows once per screen track C ports rather than once per phase.
// `/workspaces` screen's, added by track C's C.2; `queryToolResultSetTable` is
// `/queryTool`'s, added by C.4 and the only one in either corpus that asks the
// server for a *statement* rather than for a structure. All three are translated
// out of `screens/fixtures/screen_configs.json` by the same code path, and the
// count here grows once per screen track C ports rather than once per phase.
// F.5 could not author that flow without it; C.7 added
// `pipelineExecDetailsTable` and `cpipesExecDetailsTable`, the tables of
// `/executionStatusDetails/:session_id` and `/executionStatsDetails/:session_id`;
// C.6 added `inputLoaderStatusTable` (the home screen's first tab),
// `inputTable` (`/domainTableViewer/…`) and `inputFileViewerTable`
// (`/filePreviewPath/:file_key`). All of them are translated out of
// `screens/fixtures/screen_configs.json` by the same code path.
//
// **The home screen's other two tables are not here and that is not an
// omission.** `pipelineExecStatusTable` arrived with F.5, and `inputRegistryTable`
// is one of the flows' 37 — it is registered in
// `jetsclient/lib/modules/user_flows/start_pipeline/data_table_config.dart` and
// rendered by the home screen, which is F18 read backwards.
//
// **Two of the 49 are authored rather than translated, and they are the first.**
// `wsLookupTableTable` and `wsLookupColumnTable` are the `lookups` compiled view
// (C.3a), which the Flutter app never built — so there is no Dart configuration
// to measure and nothing for a round trip to compare against. That changes
// nothing on this side: this test reads the directory and validates what it
// finds, and an authored document that does not validate fails here exactly as a
// translated one would.
//
// **The count here is deliberately a literal and deliberately duplicated.** It is
// the one assertion that fails when a document is emitted and not committed, or
// committed and not emitted — the TypeScript side compares the directory against
// what it just produced, so it cannot notice a file that reached neither.
// **37 of them are the flows' and three are not.** `pipelineExecStatusTable` is
// registered on the non-flow side and rendered by `homeFiltersUF` (plan F18), so
// F.5 could not author that flow without it; `workspaceRegistryTable` is the
// `/workspaces` screen's, added by track C's C.2; `queryToolResultSetTable` is
// `/queryTool`'s, added by C.4 and the only one in either corpus that asks the
// server for a *statement* rather than for a structure; C.7 added
// `pipelineExecDetailsTable` and `cpipesExecDetailsTable`, the tables of
// `/executionStatusDetails/:session_id` and `/executionStatsDetails/:session_id`.
// C.9 added five for `/processErrors/:session_id` — its own table, the repeated
// one inside `viewInputRecordsDialog`, and the three of the rule session
// explorer, which are the first `source: "formState"` documents to exist. C.13
// added two for `/userAdmin` and C.10 one for `/ruleConfig`.
//
// **Two of those five are hand-authored rather than translated, and this test is
// where a reader will first meet the fact.** `reteSessionEntityKeyTable` and
// `reteSessionEntityDetailsTable` name a Dart closure that no corpus can carry —
// see `jetsclient_ide/src/datatable/table.test.ts`, `HAND_AUTHORED_KEYS`, where
// each is compared field by field against the corpus configuration anyway. The
// count here does not distinguish them, deliberately: what this test asserts is
// that every document in the directory passes the Go validator, and how a
// document came to be written is not that question.
//
// The count grows once per screen track C ports rather than once per phase.
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
	if len(files) != 61 {
		t.Fatalf("expected the flows' 37 table configurations plus the non-flow ones, found %d", len(files))
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
		// `DoDataTableAction` (`jets/apiserver/api_tables.go`).
		//
		// **The document has a field for it as of track C's C.2, and this case is
		// unchanged.** It is a closed three-member enum — `workspace_read`,
		// `preview_file`, `raw_query_tool`, with `read` said by omission — chosen
		// because those are the four values the two Flutter corpora hold between
		// them. What could never be authored still cannot be, and it now fails
		// against a named allowlist rather than against
		// `additionalProperties: false`, which is a stronger statement of the same
		// property: the reason is *this is not one of the authorities a table may
		// reach* rather than *this document has no such field*.
		"an apiAction outside the enum": `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a","label":"A"}],
			"sortColumn":"a","rowsPerPage":10,"from":[{"schema":"jetsapi","table":"t"}],"apiAction":"exec_ddl"}`,
		// `read` is the default and is spelled by leaving the field out; admitting
		// it as a member would give one meaning two spellings.
		"an apiAction of read, which is said by omission": `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a","label":"A"}],
			"sortColumn":"a","rowsPerPage":10,"from":[{"schema":"jetsapi","table":"t"}],"apiAction":"read"}`,
		// A static table sends nothing, so the field is on the query arm only.
		"an apiAction on a static table": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"rows":[["x"]],
			"apiAction":"workspace_read"}`,
		// C.2's row gate. An empty conjunction is vacuously true and would enable
		// the button it was written to restrict, which is the one way a gate can
		// fail that looks like no gate at all.
		"an enableWhen with an empty conjunction": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,
			"from":[{"schema":"jetsapi","table":"t"}],
			"actions":[{"key":"k","label":"L","action":"doAction","style":"primary","enableWhen":[[]]}]}`,
		"an enableWhen with an unknown comparison": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,
			"from":[{"schema":"jetsapi","table":"t"}],
			"actions":[{"key":"k","label":"L","action":"doAction","style":"primary",
			"enableWhen":[[{"column":"a","is":"startsWith","value":"x"}]]}]}`,
		// The two arms of the column union, from the other side: 22 of 275
		// columns have no label and every one is hidden.
		"a visible column with no label": `{"schemaVersion":1,"source":"query","label":"L","columns":[{"name":"a"}],
			"sortColumn":"a","rowsPerPage":10,"from":[{"schema":"jetsapi","table":"t"}]}`,
		"a static table that names a schema": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"rows":[["x"]],
			"from":[{"schema":"jetsapi","table":"t"}]}`,
		"a query table with no from clause": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"from":[]}`,
		// C.4's relaxation, from the side that must stay closed. A *query* table
		// may now declare no columns, because the server describes the result when
		// the request names none; a *static* table may not, because its rows are
		// compiled in and nothing supplies headers for them.
		"a static table with no columns": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[],"sortColumn":"a","rowsPerPage":10,"rows":[["x"]]}`,
		"a static table with no sortColumn": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[{"name":"a","label":"A"}],"rowsPerPage":10,"rows":[["x"]]}`,
		// `requestColumnDef` is a request, and "do not request" is said by leaving
		// it out — the same shape `apiAction`'s absent `read` takes.
		"a requestColumnDef of false": `{"schemaVersion":1,"source":"query","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,
			"from":[{"schema":"jetsapi","table":"t"}],"requestColumnDef":false}`,
		"a requestColumnDef on a static table": `{"schemaVersion":1,"source":"static","label":"L",
			"columns":[{"name":"a","label":"A"}],"sortColumn":"a","rowsPerPage":10,"rows":[["x"]],
			"requestColumnDef":true}`,
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

// TestQueryTableWithNoColumnsIsAccepted states C.4's relaxation positively.
//
// **A relaxation asserted only from the rejecting side is half a change**: every
// negative above would still pass if the schema had simply stopped admitting the
// document this describes. `queryToolResultSetTable` is the shipping instance and
// `TestShippingTablesValidate` covers it, but that test would go quiet if the file
// were removed, whereas this one says what the schema means.
func TestQueryTableWithNoColumnsIsAccepted(t *testing.T) {
	// No columns, no sortColumn, a where clause naming no column, and a
	// requestColumnDef — the four things that were impossible before C.4, in one
	// document, because they arrive together on the one table that has them.
	const doc = `{"schemaVersion":1,"source":"query","label":"Query Result","columns":[],
		"rowsPerPage":1,"apiAction":"raw_query_tool","requestColumnDef":true,
		"from":[{"schema":"public"}],"where":[{"formStateKey":"query.ready"}]}`
	if findings := ValidateTableDocument(doc); len(findings) > 0 {
		t.Errorf("rejected a query table whose columns come from the server: %v", findings)
	}
}
