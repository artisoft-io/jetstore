package datatable

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// checkFormTables is the `.form.json` row's check against the workspace
// (2026-11-14): a form's `dataTable` field draws the table configuration
// `table_configs/<table>.tc.json`, and a save warns of each table the
// workspace does not hold. userflow.ValidateFormDocument is given the form
// alone and could not check it, so a form naming a missing table was found
// when the workspace next compiled, by the validator CompileWorkspace runs
// (jetsclient_ide/src/userflow/validateWorkspace.ts), or when a user opened the
// flow.
//
// **A warning, not a refusal**, and that is the conservative choice rather
// than the obvious one. A form and its table are two files saved one after the
// other, and either may come first: refusing the form until its table exists
// would make the author save the table first, an order nothing else in the
// IDE imposes. The compile still refuses a workspace whose form draws a
// missing table, so the warning is what the author sees at the save instead of
// at the compile.
//
// Only presence is checked: whether the table document is valid is its own
// save's check (userflow.ValidateTableDocument), and the compile's.
func checkFormTables(_, content string, workspace fs.FS) []wsvalidate.Finding {
	var doc struct {
		Forms map[string]struct {
			Rows [][]struct {
				Field string `json:"field"`
				Table string `json:"table"`
			} `json:"rows"`
		} `json:"forms"`
	}
	// The form validator has accepted the document, so it decodes; a document
	// that does not is the validator's to report.
	if err := json.Unmarshal([]byte(content), &doc); err != nil {
		return nil
	}
	formKeys := make([]string, 0, len(doc.Forms))
	for key := range doc.Forms {
		formKeys = append(formKeys, key)
	}
	sort.Strings(formKeys)
	var findings []wsvalidate.Finding
	present := map[string]bool{}
	for _, formKey := range formKeys {
		for i, row := range doc.Forms[formKey].Rows {
			for j, field := range row {
				if field.Field != "dataTable" || field.Table == "" {
					continue
				}
				file := tableConfigFile(field.Table)
				ok, seen := present[file]
				if !seen {
					_, err := fs.Stat(workspace, file)
					ok = err == nil
					present[file] = ok
				}
				if ok {
					continue
				}
				findings = append(findings, wsvalidate.Finding{
					Severity: wsvalidate.Warning,
					Code:     "form_table_missing",
					Message: fmt.Sprintf("form %q draws table %q, and %s does not exist: "+
						"the flow will not load, and the workspace will not compile, until it does",
						formKey, field.Table, file),
					Path: fmt.Sprintf("/forms/%s/rows/%d/%d/table", escapeJSONPointer(formKey), i, j),
				})
			}
		}
	}
	return findings
}

// tableConfigFile is the workspace path of the table configuration a form
// names by key, as the IDE reads it (`tableFile`,
// jetsclient_ide/src/userflow/validateWorkspace.ts).
func tableConfigFile(key string) string {
	return "table_configs/" + key + ".tc.json"
}
