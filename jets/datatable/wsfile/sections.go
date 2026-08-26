package wsfile

import "net/url"

// The Workspace IDE's section list, and the one fact about a section that the
// client cannot work out for itself.
//
// **The IDE shows a workspace from both sides of the compiler.** The nodes below
// a section heading are the *source* assets that go into compilerv2; the heading
// itself queries `workspace.db`, the *compiled* artifact, which is why the Data
// Model heading opens on Domain Classes and Data Properties rather than on a file
// list. That duality is the feature. What was missing is any statement of which
// sections have a second side at all.
//
// Before this file the section list was eight near-identical VisitDirWrapper
// calls in workspace_data_table_action.go, and the client decided what a heading
// showed by composing `"workspace.<dir>.form"` and looking it up in its own
// registry — so "this section has no compiled view" and "the lookup failed" were
// the same event at both ends, and the second was a throw inside an async menu
// delegate. Task C.1 of the ui_refresh project, 2026-08-25.

// CompiledView names the view of `workspace.db` a section's heading shows, and
// the empty value means the section has none.
//
// **The predicate is: do this section's files compile into `workspace.db`?**
// It is deliberately about the compile relationship and not about subject
// matter. "Is this section an engine resource" is the phrasing that looks
// equivalent and is not: a process sequence is an engine concept, and its files
// compiled into nothing — `rule_sequences` is filled from workspace_control.json's
// inline array by SaveRuleSequences (jets/jetrules/rete/workspace_control.go:26),
// never from the `.jr` under that heading. That section was removed on
// 2026-08-23; the predicate it broke is why this type carries the definition.
//
// The measurement behind the three non-empty values, re-verified 2026-08-25
// against all four workspaces at the commits this branch pins: seed the closure
// with workspace_control.json's `rule_sets`, follow `import`, and the reached
// directories are `data_model` (31 files), `jet_rules` (149) and `lookups` (15).
// `pipes_config`, `process_config` and `reports` are reached zero times in every
// one of the four. **`lookups` is reached by import in three workspaces and by
// `$csv_file` in the fourth** — usi_ws declares its lookup tables inside
// `jet_rules/` and keeps only the CSV data under `lookups/` — which is a fact
// about where a workspace puts a declaration rather than about the section, and
// is why this is a property of the section rather than a per-workspace query.
type CompiledView string

const (
	// NoCompiledView is a section whose files do not compile into workspace.db.
	// It is not a gap: there is nothing for a heading to show, ever.
	NoCompiledView CompiledView = ""

	DataModelView CompiledView = "data_model"
	JetRulesView  CompiledView = "jet_rules"
	LookupsView   CompiledView = "lookups"
)

// Section is one heading of the Workspace IDE's file tree.
//
// Adding a row is the whole of adding a section, which is the shape
// workspace_file_validators.go's per-suffix table already took and the only
// interface in this repository that has been extended by a second author and
// held. Filters are matched by suffix.
type Section struct {
	// Dir is the workspace-relative directory, and is also the section node's
	// key and pageMatchKey.
	Dir string
	// Label is what the heading reads in the IDE.
	Label string
	// Filters are the filename suffixes this section lists. A file matching none
	// of them is not shown, and a directory named by no Section is not shown at
	// all however many files it holds.
	Filters []string
	// CompiledView is the workspace.db view this section's heading shows, or
	// NoCompiledView. See the type.
	CompiledView CompiledView
}

// WorkspaceSections is the section list the apiserver emits, in display order.
//
// **Eight sections, and it has been eight throughout** — `process_sequence` was
// removed and `table_configs` added on the same day, 2026-08-23, so a count
// carried forward from before that day is right by coincidence and its
// membership is wrong.
var WorkspaceSections = []Section{
	{
		Dir: "data_model", Label: "Data Model",
		Filters: []string{".jr", ".csv"}, CompiledView: DataModelView,
	},
	{
		Dir: "jet_rules", Label: "Jets Rules",
		Filters: []string{".jr", ".jr.sql"}, CompiledView: JetRulesView,
	},
	{
		// Declared here, and not built in the Flutter client. That is the state
		// this table exists to make sayable: the view is possible and absent,
		// rather than impossible. **It was built in React on 2026-08-25** by
		// ui_refresh's C.3a — jetsclient_ide/src/screens/views/lookups.view.json
		// and two authored table documents — rather than in jetsclient, because
		// track X deletes that app and a view built there is discarded by
		// construction (their I-45, decided 2026-08-23 by the user).
		//
		// This comment said "it is scheduled as C.3a" until C.3a corrected it.
		// A note naming a future trigger goes silent when the trigger passes, so
		// the task that fires it is the only reader that can be relied on.
		Dir: "lookups", Label: "Lookups",
		Filters: []string{".jr", ".csv"}, CompiledView: LookupsView,
	},
	{
		Dir: "pipes_config", Label: "Pipes Config",
		Filters: []string{".pc.json"}, CompiledView: NoCompiledView,
	},
	{
		// **This entry is the whole of making a new file type visible**, which is
		// worth saying because a workspace directory that no Section names does
		// not appear in the IDE at all. S.3 created `user_flows/` and would have
		// shipped it invisible without a row. All three suffixes are listed
		// because a flow, its actions and its form are edited together.
		Dir: "user_flows", Label: "User Flows",
		Filters: []string{".uf.json", ".ua.json", ".form.json"}, CompiledView: NoCompiledView,
	},
	{
		// The fifth authored document type (task I.3, 2026-08-23). It sits in its
		// own directory rather than beside the flows because a table
		// configuration is *shared*: two flows may name the same table, so it is
		// keyed by table rather than by flow — which is also why the client
		// resolves it separately (FlowStore.load, jetsclient_ide/src/userflow/store.ts).
		Dir: "table_configs", Label: "Table Configurations",
		Filters: []string{".tc.json"}, CompiledView: NoCompiledView,
	},
	{
		Dir: "process_config", Label: "Process Configuration",
		Filters: []string{"workspace_init_db.sql"}, CompiledView: NoCompiledView,
	},
	{
		Dir: "reports", Label: "Reports",
		Filters: []string{".sql", ".json"}, CompiledView: NoCompiledView,
	},
}

// workspaceRootFiles are the two files listed at the tree's root rather than
// under a heading. They are files and not sections, so they carry no compiled
// view and never could.
var workspaceRootFiles = []struct {
	Key      string
	FileName string
	Label    string
}{
	{"compile_workspace", "compile_workspace.sh", "Compile Workspace Script"},
	{"workspace_control", "workspace_control.json", "Workspace Control"},
}

// Visit walks one section's directory and returns its node, stamped with the
// section's compiled view.
//
// A directory that does not exist yields an empty section rather than an error;
// see visitDir for why that failure mode had to change.
func (s Section) Visit(root, workspaceName string) (*WorkspaceNode, error) {
	filters := s.Filters
	node, err := VisitDirWrapper(root, s.Dir, s.Label, &filters, workspaceName)
	if err != nil {
		return nil, err
	}
	node.CompiledView = string(s.CompiledView)
	return node, nil
}

// BuildWorkspaceFileStructure returns the whole `workspace_file_structure`
// payload for one workspace: every section of WorkspaceSections in order, then
// the two root files.
//
// It is a free function over a filesystem path rather than a method on the HTTP
// context so that it can be tested against a workspace built in t.TempDir() —
// which is the only end of this contract the server can prove on its own.
func BuildWorkspaceFileStructure(root, workspaceName string) ([]*WorkspaceNode, error) {
	resultData := make([]*WorkspaceNode, 0, len(WorkspaceSections)+len(workspaceRootFiles))

	for _, section := range WorkspaceSections {
		node, err := section.Visit(root, workspaceName)
		if err != nil {
			return nil, err
		}
		resultData = append(resultData, node)
	}

	for _, f := range workspaceRootFiles {
		resultData = append(resultData, &WorkspaceNode{
			Key:          f.Key,
			Type:         "file",
			PageMatchKey: f.FileName,
			Label:        f.Label,
			RoutePath:    "/workspace/:workspace_name/home",
			RouteParams: map[string]string{
				"workspace_name": workspaceName,
				"file_name":      url.QueryEscape(f.FileName),
				"label":          f.FileName,
			},
		})
	}

	return resultData, nil
}

// SectionDeclaration renders the section contract as the canonical string both
// ends check themselves against: one `dir=compiledView` per section, in display
// order, newline separated.
//
// **It exists because no client-side corpus can enumerate this surface.** The
// section list is server data, so the Flutter app's five generated corpora — all
// built by walking client registries — close over none of it. The Dart test
// `jetsclient/test/workspace_section_contract_test.dart` carries a copy of this
// declaration and a checksum of it, and this package's test carries the same
// checksum; changing the table above fails the Go test, and updating the Go test
// alone fails the Dart one.
func SectionDeclaration() string {
	out := make([]byte, 0, 256)
	for _, s := range WorkspaceSections {
		out = append(out, s.Dir...)
		out = append(out, '=')
		out = append(out, string(s.CompiledView)...)
		out = append(out, '\n')
	}
	return string(out)
}
