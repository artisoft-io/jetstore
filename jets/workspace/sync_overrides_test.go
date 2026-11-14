package workspace

import "testing"

// An override of a page of the workspace guide, or of its settings file, does
// not make the apiserver's start-up compile the workspace; every other override
// still does, including one whose path only names docs/ on the way elsewhere.
func TestOnlyTheWorkspaceGuideIsExemptFromTheCompile(t *testing.T) {
	for fileName, want := range map[string]bool{
		"docs/index.md":                        false,
		"docs/customisation/observe.md":        false,
		"docs/index.mdx":                       false,
		"docs/workspace_docs.json":             false,
		"docs/images/diagram.png":              false,
		"jet_rules/main_rules.jr":              true,
		"data_model/classes.jr":                true,
		"lookups/codes.csv":                    true,
		"pipes_config/README.md":               true,
		"pipes_config/docs/notes.md":           true,
		"docs/../jet_rules/main_rules.jr":      true,
		"mydocs/index.md":                      true,
		"workspace_control.json":               true,
		"reports/report.json":                  true,
		"process_config/workspace_init_db.sql": true,
		"docs\\..\\lookups\\codes.csv":         true,
		"docs\\customisation\\observe.md":      false,
		"./docs/customisation/observe.md":      false,
	} {
		if got := overrideRequiresCompile(fileName); got != want {
			t.Errorf("overrideRequiresCompile(%q) = %v, want %v", fileName, got, want)
		}
	}
}
