package datatable

import "testing"

// The confinement is the whole of the security argument for GetWorkspaceDocument,
// so it is tested as a function rather than inferred from the handler.
//
// The action exists because the runtime read had inherited the IDE's capability:
// once the eleven flows became workspace assets, `workspace_ide` was what a user
// needed to *open a flow*, and only knowledge_engineer holds it. Lowering the
// capability is only safe if the path cannot reach anything else in the
// workspace, and "anything else" includes the rules, the pipeline configurations
// and the client config.
func TestDocumentPathOK(t *testing.T) {
	allowed := []string{
		"user_flows/loadFilesUF.uf.json",
		"user_flows/loadFilesUF.ua.json",
		"user_flows/loadFilesUF.form.json",
		// A projection's fourth document. Not a UserFlow document, but it is read
		// by the same running app for the same reason.
		"user_flows/qc_metrics.apply.json",
		"table_configs/pipelineExecStatusTable.tc.json",
	}
	for _, name := range allowed {
		if !documentPathOK(name) {
			t.Errorf("%s is a document this path must serve", name)
		}
	}

	refused := map[string]string{
		"jet_rules/main.jr":                     "a directory the map does not name",
		"pipes_config/jets_loader.pc.json":      "another project's file type, in another directory",
		"data_model/jets_model.jr":              "the data model",
		"lookups/codes.csv":                     "lookup data",
		"process_config/workspace_init_db.sql":  "SQL",
		"workspace_control.json":                "a root file, with no directory at all",
		"user_flows/jets_assets_manifest.json":  "a real file in the right directory, wrong suffix",
		"user_flows/evil.pc.json":               "a suffix that directory does not serve",
		"table_configs/loadFilesUF.uf.json":     "a suffix the *other* directory serves",
		"user_flows/nested/loadFilesUF.uf.json": "a second separator",
		"user_flows/../jet_rules/main.jr":       "a traversal that starts inside an allowed directory",
		"../user_flows/loadFilesUF.uf.json":     "a traversal that starts outside one",
		"/user_flows/loadFilesUF.uf.json":       "an absolute path, whose first segment is empty",
		"user_flows/":                           "a directory, not a document",
		"user_flows/.uf.json":                   "only the suffix, which names nothing and is a dotfile",
		"":                                      "nothing",
		"user_flows":                            "the directory name alone",
	}
	for name, why := range refused {
		if documentPathOK(name) {
			t.Errorf("%s must be refused: %s", name, why)
		}
	}
}

// Every suffix the map serves belongs to a document type the running app reads,
// and the two directories are the two `WorkspaceSections` rows a flow needs. A
// third directory added here is a decision about what an ordinary user may read.
func TestDocumentDirsAreTheTwoTheAppReads(t *testing.T) {
	if len(documentDirs) != 2 {
		t.Fatalf("documentDirs has %d directories; adding one widens what jetstore_read can read", len(documentDirs))
	}
	for _, dir := range []string{"user_flows", "table_configs"} {
		if _, ok := documentDirs[dir]; !ok {
			t.Errorf("%s is missing; FlowStore reads from it", dir)
		}
	}
}
