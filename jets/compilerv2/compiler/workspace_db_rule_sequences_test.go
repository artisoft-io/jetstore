package compiler

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// rule_sequences.source_file_key names the file the sequences are declared in,
// workspace_control.json, which workspace_control records once (is_main = 0),
// and main_rule_sets names each main rule file by its existing key.

// saveRuleSequences saves wc's rule sequences into dir/workspace.db, as the
// workspace compile does once every main rule file is compiled.
func saveRuleSequences(t *testing.T, dir string, wc *rete.WorkspaceControl) error {
	t.Helper()
	wdb, err := NewWorkspaceDB(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.DB.Close()
	return wdb.SaveRuleSequences(context.Background(), wdb.DB, wc)
}

func compileTwoMains(t *testing.T) string {
	t.Helper()
	useRepositorySchema(t)
	dir := t.TempDir()
	write(t, dir, "main1.jr", ruleFilesResources+`[R1]: (?x rdfType ex:Item) -> (?x ex:out 1);
`)
	write(t, dir, "main2.jr", ruleFilesResources+`[R2]: (?x rdfType ex:Item) -> (?x ex:out 2);
`)
	compileMain(t, dir, "main1.jr")
	compileMain(t, dir, "main2.jr")
	return dir
}

func TestWorkspaceDBRuleSequenceSourceFile(t *testing.T) {
	dir := compileTwoMains(t)
	db := openWorkspaceDB(t, dir)
	before := queryStrings(t, db, `SELECT key || ' ' || source_file_name || '=' || is_main FROM workspace_control ORDER BY key`)

	wc := rete.NewWorkspaceControl([]string{"main1.jr", "main2.jr"}, []rete.RuleSequence{
		{Name: "seqA", RuleSets: []string{"main2.jr", "main1.jr"}},
		{Name: "seqB", RuleSets: []string{"main1.jr"}},
	})
	if err := saveRuleSequences(t, dir, wc); err != nil {
		t.Fatal(err)
	}

	// One row added, after the existing ones, which keep their keys
	got := queryStrings(t, db, `SELECT key || ' ' || source_file_name || '=' || is_main FROM workspace_control ORDER BY key`)
	want := append(slices.Clone(before), "3 workspace_control.json=0")
	if !slices.Equal(got, want) {
		t.Errorf("workspace_control: got %q, want %q", got, want)
	}

	// Every sequence's source file and every step's main rule file resolve
	got = queryStrings(t, db, `SELECT rs.name || ' in ' || coalesce(wc.source_file_name, 'NONE')
	  FROM rule_sequences rs LEFT JOIN workspace_control wc ON wc.key = rs.source_file_key
	  ORDER BY rs.name`)
	want = []string{"seqA in workspace_control.json", "seqB in workspace_control.json"}
	if !slices.Equal(got, want) {
		t.Errorf("rule_sequences: got %q, want %q", got, want)
	}
	got = queryStrings(t, db, `SELECT rs.name || ' ' || mrs.seq || ' ' || mrs.main_ruleset_name || '=' || coalesce(wc.source_file_name, 'NONE') || ',' || wc.is_main
	  FROM main_rule_sets mrs
	  JOIN rule_sequences rs ON rs.key = mrs.rule_sequence_key
	  LEFT JOIN workspace_control wc ON wc.key = mrs.ruleset_file_key
	  ORDER BY rs.name, mrs.seq`)
	want = []string{
		"seqA 0 main2.jr=main2.jr,1",
		"seqA 1 main1.jr=main1.jr,1",
		"seqB 0 main1.jr=main1.jr,1",
	}
	if !slices.Equal(got, want) {
		t.Errorf("main_rule_sets: got %q, want %q", got, want)
	}

	// Saved again, as a second save into the same workspace.db: nothing changes
	if err := saveRuleSequences(t, dir, wc); err != nil {
		t.Fatalf("saving the rule sequences a second time: %v", err)
	}
	got = queryStrings(t, db, `SELECT (SELECT count(*) FROM workspace_control) || ' ' ||
	  (SELECT count(*) FROM rule_sequences) || ' ' || (SELECT count(*) FROM main_rule_sets)`)
	if want := []string{"3 2 3"}; !slices.Equal(got, want) {
		t.Errorf("row counts after a second save: got %q, want %q", got, want)
	}
}

// A workspace with no rule sequence gets no workspace_control.json row.
func TestWorkspaceDBNoRuleSequence(t *testing.T) {
	dir := compileTwoMains(t)
	if err := saveRuleSequences(t, dir, rete.NewWorkspaceControl([]string{"main1.jr", "main2.jr"}, nil)); err != nil {
		t.Fatal(err)
	}
	db := openWorkspaceDB(t, dir)
	got := queryStrings(t, db, `SELECT source_file_name FROM workspace_control ORDER BY key`)
	if want := []string{"main1.jr", "main2.jr"}; !slices.Equal(got, want) {
		t.Errorf("workspace_control: got %q, want %q", got, want)
	}
}

// A sequence naming a file that is not a compiled main rule file is refused,
// rather than given a key that names nothing.
func TestWorkspaceDBRuleSequenceUnknownMain(t *testing.T) {
	dir := compileTwoMains(t)
	wc := rete.NewWorkspaceControl([]string{"main1.jr"}, []rete.RuleSequence{
		{Name: "seqA", RuleSets: []string{"main1.jr", "missing.jr"}},
	})
	err := saveRuleSequences(t, dir, wc)
	if err == nil || !strings.Contains(err.Error(), "missing.jr") {
		t.Fatalf("got %v, want an error naming missing.jr", err)
	}
	db := openWorkspaceDB(t, dir)
	got := queryStrings(t, db, `SELECT (SELECT count(*) FROM workspace_control) || ' ' || (SELECT count(*) FROM rule_sequences)`)
	if want := []string{"2 0"}; !slices.Equal(got, want) {
		t.Errorf("row counts: got %q, want %q", got, want)
	}
}
