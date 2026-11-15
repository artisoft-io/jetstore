package compiler

import (
	"database/sql"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// workspace.db says which file each rule is written in (jet_rules.authored_file_key)
// and which files each main rule file includes (main_support_files), while
// jet_rules.source_file_key stays the main rule file whose network holds the rule.

const ruleFilesResources = `resource rdfType = "rdf:type";
resource ex:Item = "ex:Item";
resource ex:out = "ex:out";
`

// compileMain compiles one main rule file of dir into dir/workspace.db, as the
// workspace compile does for each main rule file in turn.
func compileMain(t *testing.T, dir, mainFile string) {
	t.Helper()
	c := NewCompiler(dir, mainFile, true, false, true)
	if err := c.Compile(); err != nil {
		t.Fatalf("compile %s: %v\n%s", mainFile, err, c.ErrorLog().String())
	}
}

func useRepositorySchema(t *testing.T) {
	t.Helper()
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })
}

func openWorkspaceDB(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(dir, "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func queryStrings(t *testing.T, db *sql.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// The included files' order is the reader's, depth first, and a rule's file is
// the one it is written in, including a class's generated ci_ rule.
func TestWorkspaceDBRecordsEachRulesFile(t *testing.T) {
	useRepositorySchema(t)
	dir := t.TempDir()
	write(t, dir, "b.jr", `class ex:Item {
  $base_classes = [owl:Thing],
  $data_properties = [ex:out as int]
};
[RB]: (?x rdfType ex:Item) -> (?x ex:out 3);
`)
	write(t, dir, "a.jr", `import "b.jr"
[RA]: (?x rdfType ex:Item) -> (?x ex:out 2);
`)
	write(t, dir, "c.jr", `[RC]: (?x rdfType ex:Item) -> (?x ex:out 4);
`)
	write(t, dir, "main.jr", ruleFilesResources+`import "a.jr"
[RM]: (?x rdfType ex:Item) -> (?x ex:out 1);
import "c.jr"
`)
	compileMain(t, dir, "main.jr")
	db := openWorkspaceDB(t, dir)

	got := queryStrings(t, db, `SELECT m.source_file_name || ' > ' || s.source_file_name
	  FROM main_support_files ms
	  JOIN workspace_control m ON m.key = ms.main_file_key
	  JOIN workspace_control s ON s.key = ms.support_file_key
	  ORDER BY ms.rowid`)
	want := []string{"main.jr > a.jr", "main.jr > b.jr", "main.jr > c.jr"}
	if !slices.Equal(got, want) {
		t.Errorf("main_support_files: got %q, want %q", got, want)
	}

	got = queryStrings(t, db, `SELECT jr.name || ' in ' || a.source_file_name || ', under ' || m.source_file_name
	  FROM jet_rules jr
	  JOIN workspace_control m ON m.key = jr.source_file_key
	  JOIN workspace_control a ON a.key = jr.authored_file_key
	  ORDER BY jr.name`)
	want = []string{
		"RA in a.jr, under main.jr",
		"RB in b.jr, under main.jr",
		"RC in c.jr, under main.jr",
		"RM in main.jr, under main.jr",
		"ci_ex:Item in b.jr, under main.jr",
	}
	if !slices.Equal(got, want) {
		t.Errorf("jet_rules files: got %q, want %q", got, want)
	}

	got = queryStrings(t, db, `SELECT source_file_name || '=' || is_main FROM workspace_control ORDER BY key`)
	// b.jr is named by its class before the includes are recorded, so it keeps
	// the key it had before main_support_files was written.
	want = []string{"main.jr=1", "b.jr=0", "a.jr=0", "c.jr=0"}
	if !slices.Equal(got, want) {
		t.Errorf("workspace_control: got %q, want %q", got, want)
	}
}

// A file two main rule files include is one workspace_control row, a
// main_support_files row under each, and its rule a jet_rules row under each
// main rule file with the same authored file.
func TestWorkspaceDBSharedIncludedFile(t *testing.T) {
	useRepositorySchema(t)
	dir := t.TempDir()
	write(t, dir, "shared.jr", ruleFilesResources+`[RS]: (?x rdfType ex:Item) -> (?x ex:out 1);
`)
	write(t, dir, "main1.jr", `import "shared.jr"
[R1]: (?x rdfType ex:Item) -> (?x ex:out 2);
`)
	write(t, dir, "main2.jr", `import "shared.jr"
[R2]: (?x rdfType ex:Item) -> (?x ex:out 3);
`)
	compileMain(t, dir, "main1.jr")
	compileMain(t, dir, "main2.jr")
	db := openWorkspaceDB(t, dir)

	got := queryStrings(t, db, `SELECT source_file_name || '=' || is_main FROM workspace_control ORDER BY key`)
	want := []string{"main1.jr=1", "shared.jr=0", "main2.jr=1"}
	if !slices.Equal(got, want) {
		t.Errorf("workspace_control: got %q, want %q", got, want)
	}
	got = queryStrings(t, db, `SELECT m.source_file_name || ' > ' || s.source_file_name
	  FROM main_support_files ms
	  JOIN workspace_control m ON m.key = ms.main_file_key
	  JOIN workspace_control s ON s.key = ms.support_file_key
	  ORDER BY ms.rowid`)
	want = []string{"main1.jr > shared.jr", "main2.jr > shared.jr"}
	if !slices.Equal(got, want) {
		t.Errorf("main_support_files: got %q, want %q", got, want)
	}
	got = queryStrings(t, db, `SELECT m.source_file_name || ': ' || jr.name || ' in ' || a.source_file_name
	  FROM jet_rules jr
	  JOIN workspace_control m ON m.key = jr.source_file_key
	  JOIN workspace_control a ON a.key = jr.authored_file_key
	  ORDER BY m.source_file_name, jr.name`)
	want = []string{
		"main1.jr: R1 in main1.jr",
		"main1.jr: RS in shared.jr",
		"main2.jr: R2 in main2.jr",
		"main2.jr: RS in shared.jr",
	}
	if !slices.Equal(got, want) {
		t.Errorf("jet_rules: got %q, want %q", got, want)
	}
}

// Main rule files are compiled in name order, so one may already be recorded
// as a file an earlier main rule file includes. It then becomes a main rule
// file with the key it has; compiling a main rule file twice still fails.
func TestWorkspaceDBIncludedMainRuleFile(t *testing.T) {
	useRepositorySchema(t)
	dir := t.TempDir()
	write(t, dir, "b_main.jr", ruleFilesResources+`[RB]: (?x rdfType ex:Item) -> (?x ex:out 1);
`)
	write(t, dir, "a_main.jr", `import "b_main.jr"
[RA]: (?x rdfType ex:Item) -> (?x ex:out 2);
`)
	compileMain(t, dir, "a_main.jr")
	compileMain(t, dir, "b_main.jr")
	db := openWorkspaceDB(t, dir)

	got := queryStrings(t, db, `SELECT source_file_name || '=' || is_main FROM workspace_control ORDER BY key`)
	want := []string{"a_main.jr=1", "b_main.jr=1"}
	if !slices.Equal(got, want) {
		t.Errorf("workspace_control: got %q, want %q", got, want)
	}
	got = queryStrings(t, db, `SELECT m.source_file_name || ': ' || jr.name || ' in ' || a.source_file_name
	  FROM jet_rules jr
	  JOIN workspace_control m ON m.key = jr.source_file_key
	  JOIN workspace_control a ON a.key = jr.authored_file_key
	  ORDER BY m.source_file_name, jr.name`)
	want = []string{
		"a_main.jr: RA in a_main.jr",
		"a_main.jr: RB in b_main.jr",
		"b_main.jr: RB in b_main.jr",
	}
	if !slices.Equal(got, want) {
		t.Errorf("jet_rules: got %q, want %q", got, want)
	}

	c := NewCompiler(dir, "b_main.jr", true, false, true)
	err := c.Compile()
	if err == nil || !strings.Contains(err.Error(), "already exists in workspace db") {
		t.Fatalf("compiling b_main.jr a second time: got %v, want an already-exists error", err)
	}
}
