package wsfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspace_assets "github.com/artisoft-io/jetstore/jets/workspace_assets"
)

// The section contract, from the end that produces it.
//
// **No client-side corpus can guard this surface**, and that is the reason these
// tests exist rather than a reason to write them somewhere cheaper. The Flutter
// app has five generated corpora and all five are built by walking client
// registries; the section list is *server data*, so none of them closes over it.
// The mirror of this file is jetsclient/test/workspace_section_contract_test.dart,
// which carries the same declaration and the same checksum and drives the client's
// half against it.

// The checksum of SectionDeclaration(), duplicated in the Dart test named above.
//
// **Change the section table and this test fails; update only this constant and
// the Dart test fails.** That is the whole mechanism, and it is the corpus
// pattern the Flutter suite already uses, generated from the other end.
const expectedDeclarationChecksum = "fnv1a32:b3fc79eb"

// fnv1a32 matches jetsclient/test/corpus_support.dart's `checksum`, which is
// hand-rolled there for the same reason it is hand-rolled here: this is drift
// detection between two repositories' worth of code, not a security boundary.
func fnv1a32(s string) string {
	var hash uint32 = 0x811c9dc5
	for _, b := range []byte(s) {
		hash ^= uint32(b)
		hash *= 16777619
	}
	const hex = "0123456789abcdef"
	out := []byte("fnv1a32:00000000")
	for i := 0; i < 8; i++ {
		out[len(out)-1-i] = hex[(hash>>(4*i))&0xf]
	}
	return string(out)
}

func TestTheSectionDeclarationIsWhatBothEndsAgreedOn(t *testing.T) {
	got := fnv1a32(SectionDeclaration())
	if got != expectedDeclarationChecksum {
		t.Fatalf("the section contract changed.\n"+
			"declaration:\n%s\nchecksum: %s, want %s\n\n"+
			"Update this constant AND the copy in "+
			"jetsclient/test/workspace_section_contract_test.dart, which decides "+
			"whether the client needs a form for the section you changed.",
			SectionDeclaration(), got, expectedDeclarationChecksum)
	}
}

func TestEverySectionIsWellFormedAndDistinct(t *testing.T) {
	seen := make(map[string]bool, len(WorkspaceSections))
	for _, s := range WorkspaceSections {
		if s.Dir == "" || s.Label == "" || len(s.Filters) == 0 {
			t.Errorf("section %+v is missing a directory, a label or a filter", s)
		}
		if seen[s.Dir] {
			// Two rows for one directory would emit the heading twice and give
			// the client two answers to the same question.
			t.Errorf("section %q is declared twice", s.Dir)
		}
		seen[s.Dir] = true
	}
}

// The predicate itself, enforced rather than re-read.
//
// **This is the assertion the whole design rests on**, so it is written as a
// literal set rather than derived from the table it is checking: deriving it
// would make the test agree with whatever the table says. Adding a compiling
// section means editing this list, which is the point at which somebody has to
// answer *do this section's files compile into `workspace.db`?* rather than
// copy the row above.
//
// Measured 2026-08-25 across cedargate_ws, jets_ws, usi_ws and walrus_ws at the
// commits jetstore_agentic_ai pins: seed the closure with workspace_control.json's
// `rule_sets`, follow `import`, and the reached directories are data_model,
// jet_rules and lookups. pipes_config, process_config and reports are reached
// zero times in all four.
func TestExactlyThreeSectionsCompileIntoTheWorkspaceDb(t *testing.T) {
	want := map[string]CompiledView{
		"data_model": DataModelView,
		"jet_rules":  JetRulesView,
		"lookups":    LookupsView,
	}

	got := make(map[string]CompiledView)
	for _, s := range WorkspaceSections {
		if s.CompiledView != NoCompiledView {
			got[s.Dir] = s.CompiledView
		}
	}

	if len(got) != len(want) {
		t.Fatalf("compiled views: got %v, want %v", got, want)
	}
	for dir, view := range want {
		if got[dir] != view {
			t.Errorf("section %q: got compiled view %q, want %q", dir, got[dir], view)
		}
	}
}

// A section node carries its compiled view, and no other node carries one.
//
// The `omitempty` tag is what makes the second half testable from the wire: a
// viewless section and a directory are both silent on the field, so the client's
// "is there a view here" question has one answer shape.
func TestOnlySectionNodesCarryACompiledView(t *testing.T) {
	root := t.TempDir()
	// data_model compiles; pipes_config does not. Give each a file and a
	// subdirectory so the assertion covers all three node types.
	for _, dir := range []string{"data_model/entities", "pipes_config/nested"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		"data_model/classes.jr":             "// a class",
		"data_model/entities/nested.jr":     "// nested",
		"pipes_config/main.pc.json":         "{}",
		"pipes_config/nested/other.pc.json": "{}",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	nodes, err := BuildWorkspaceFileStructure(root, "ws")
	if err != nil {
		t.Fatal(err)
	}

	byKey := make(map[string]*WorkspaceNode, len(nodes))
	for _, n := range nodes {
		byKey[n.Key] = n
	}

	if n := byKey["data_model"]; n == nil || n.CompiledView != "data_model" {
		t.Errorf("data_model should carry its compiled view, got %+v", n)
	}
	if n := byKey["pipes_config"]; n == nil || n.CompiledView != "" {
		t.Errorf("pipes_config has no compiled view and must say nothing, got %+v", n)
	}

	// Walk everything below the headings: no dir and no file may claim a view.
	var walk func(*WorkspaceNode)
	walk = func(n *WorkspaceNode) {
		if n.Type != "section" && n.CompiledView != "" {
			t.Errorf("a %s node claims compiled view %q: %+v", n.Type, n.CompiledView, n)
		}
		if n.Children != nil {
			for _, c := range *n.Children {
				walk(c)
			}
		}
	}
	for _, n := range nodes {
		if n.Type == "section" && n.Children != nil {
			for _, c := range *n.Children {
				walk(c)
			}
		} else if n.Type != "section" {
			walk(n)
		}
	}
}

// What the client actually parses.
//
// The field is read by name from JSON in Dart, so a rename here is invisible to
// the Go compiler and fatal to the client. This test pins the name.
func TestTheWireFieldIsNamedCompiledView(t *testing.T) {
	root := t.TempDir()
	nodes, err := BuildWorkspaceFileStructure(root, "ws")
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(nodes)
	if err != nil {
		t.Fatal(err)
	}

	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	// An empty workspace still emits every heading — that is F.0a's failure-mode
	// change, and it is what makes the section list a contract rather than a
	// consequence of what somebody happened to author.
	if len(decoded) != len(WorkspaceSections)+2 {
		t.Fatalf("want %d sections plus the two root files, got %d",
			len(WorkspaceSections), len(decoded))
	}

	withView := 0
	for _, n := range decoded {
		v, present := n["compiled_view"]
		if !present {
			continue
		}
		withView++
		if s, ok := v.(string); !ok || s == "" {
			t.Errorf("compiled_view present and not a non-empty string: %v", v)
		}
	}
	if withView != 3 {
		t.Errorf("want the three compiling sections to carry compiled_view, got %d", withView)
	}
}

func TestTheTwoRootFilesAreFilesAndCarryNoView(t *testing.T) {
	root := t.TempDir()
	nodes, err := BuildWorkspaceFileStructure(root, "ws")
	if err != nil {
		t.Fatal(err)
	}

	tail := nodes[len(nodes)-2:]
	for i, want := range []string{"compile_workspace.sh", "workspace_control.json"} {
		n := tail[i]
		if n.Type != "file" || n.PageMatchKey != want || n.CompiledView != "" {
			t.Errorf("root file %d: got %+v, want a viewless file node for %q", i, n, want)
		}
	}
}

// Every asset the installer puts in a workspace is visible in the IDE.
//
// **The two lists are in different packages and nothing joined them until this
// test.** `workspace_assets.AssetGroups` decides what is written into a
// workspace; `WorkspaceSections` decides what the file tree shows. They agree by
// convention, and on 2026-08-26 they did not: U.2 added `.apply.json` to the
// embed glob and not to the `user_flows` filters, so the fourth document of every
// projected flow was installed and unopenable — 45 documents, 42 nodes.
//
// The assertion is one-directional on purpose. A section may list a suffix the
// installer never writes (a workspace authors its own flows, and should), but a
// suffix JetStore *installs* and the IDE cannot show is a file a knowledge
// engineer is told to edit and cannot find.
func TestEveryInstalledAssetIsVisibleInTheIde(t *testing.T) {
	filtersByDir := map[string]map[string]bool{}
	for _, s := range WorkspaceSections {
		set := map[string]bool{}
		for _, f := range s.Filters {
			set[f] = true
		}
		filtersByDir[s.Dir] = set
	}

	for _, g := range workspace_assets.AssetGroups {
		filters, hasSection := filtersByDir[g.Dir]
		if !hasSection {
			t.Errorf("%s is installed into every workspace and no Section names it, so the IDE does not show the directory at all", g.Dir)
			continue
		}
		names, err := workspace_assets.Names(g.Dir)
		if err != nil {
			t.Fatalf("listing %s: %v", g.Dir, err)
		}
		for _, name := range names {
			if !matchesAFilter(name, filters) {
				t.Errorf("%s/%s is installed and no filter of the %q section lists its suffix; it is invisible in the IDE",
					g.Dir, name, g.Dir)
			}
		}
	}
}

// matchesAFilter reports whether any filter is a suffix of name, which is how
// Section.Filters is applied. Longest match is not needed: the question is
// whether *some* filter shows the file.
func matchesAFilter(name string, filters map[string]bool) bool {
	for filter := range filters {
		if strings.HasSuffix(name, filter) {
			return true
		}
	}
	return false
}
