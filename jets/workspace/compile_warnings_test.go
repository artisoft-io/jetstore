package workspace

// A compile that succeeds and warns: the warnings are in the compile's output,
// which is what the registry stores as last_git_log and View Last Log shows.
// Until 2026-10-12 a successful compile wrote only its parse log there, so a
// warning reached the process log alone (rules_unit_test_harness I-55).

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestCompileWarnings(t *testing.T) {
	errorLog := "warning: identifier 'ex:x' must be defined in a declaration section before use, creating as resource\n" +
		"\n" +
		"** warning: rules A (a.jr, s=100), B (a.jr, s=5) have the same antecedents, different saliences, and all infer (?x01 ex:a ?x02); ...\n"
	got := compileWarnings(errorLog, "")
	for _, want := range []string{"Compilation warnings:\n", "  warning: identifier 'ex:x'", "  ** warning: rules A (a.jr, s=100)"} {
		if !strings.Contains(got, want) {
			t.Errorf("compileWarnings does not contain %q:\n%s", want, got)
		}
	}
	if compileWarnings("", "") != "" || compileWarnings("\n\n", "") != "" {
		t.Errorf("no warnings must give no heading")
	}
	// The identifier warning is written to the parse log as well, which the
	// compile output already carries: it is not repeated.
	parseLog := "warning: identifier 'ex:x' must be defined in a declaration section before use, creating as resource\n"
	got = compileWarnings(errorLog, parseLog)
	if strings.Contains(got, "identifier") || !strings.Contains(got, "** warning: rules A") {
		t.Errorf("the parse log's warning must be left out and the other kept:\n%s", got)
	}
}

const sameConsequentRules = `# A rule copied to change its salience, with the original kept.
import "data_model/jets_model.jr"

[UT_CW_10, s=100]:
  (?s rdf:type jets:Entity).
  (?s jets:key ?k)
  ->
  (?s rdfs:label ?k)
;

[UT_CW_20, s=5]:
  (?s rdf:type jets:Entity).
  (?s jets:key ?k)
  ->
  (?s rdfs:label ?k)
;
`

func TestCompileOutputCarriesTheWarnings(t *testing.T) {
	if os.Getenv("RUT_I55_CHILD") == "1" {
		ctl, err := rete.LoadWorkspaceControl(filepath.Join(workspaceHome, "test_ws", "workspace_control.json"))
		if err != nil {
			t.Fatal(err)
		}
		ctl.WorkspaceName = "test_ws"
		out, err := compileWorkspaceV2(nil, ctl, "rut_i55")
		if err != nil {
			t.Fatalf("a warning must not fail the compile: %v\n%s", err, out)
		}
		for _, want := range []string{
			"Compilation warnings:\n  ** warning: rules UT_CW_10",
			"UT_CW_20",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the compile output does not carry %q:\n%s", want, out)
			}
		}
		// A warning the parse log already carries is not shown twice.
		if n := strings.Count(out, "warning: identifier 'rdfs:label'"); n != 1 {
			t.Errorf("the identifier warning appears %d times, want once:\n%s", n, out)
		}
		return
	}
	home := t.TempDir()
	ws := filepath.Join(home, "test_ws")
	if out, err := exec.Command("cp", "-r", "../jetrules/test_ws", ws).CombinedOutput(); err != nil {
		t.Fatalf("copying test_ws: %v\n%s", err, out)
	}
	_ = os.Remove(filepath.Join(ws, "workspace.db"))
	if err := os.WriteFile(filepath.Join(ws, "jet_rules", "same_consequent_main.jr"), []byte(sameConsequentRules), 0644); err != nil {
		t.Fatal(err)
	}
	ctlPath := filepath.Join(ws, "workspace_control.json")
	raw, err := os.ReadFile(ctlPath)
	if err != nil {
		t.Fatal(err)
	}
	var ctl map[string]any
	if err := json.Unmarshal(raw, &ctl); err != nil {
		t.Fatal(err)
	}
	ctl["rule_sets"] = append(ctl["rule_sets"].([]any), "jet_rules/same_consequent_main.jr")
	if raw, err = json.Marshal(ctl); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ctlPath, raw, 0644); err != nil {
		t.Fatal(err)
	}
	schema, err := filepath.Abs("../workspace_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCompileOutputCarriesTheWarnings$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), "RUT_I55_CHILD=1", "WORKSPACES_HOME="+home, "WORKSPACE=test_ws",
		"JETS_WORKSPACE_DB_SCHEMA_SCRIPT="+schema)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child compile: %v\n%s", err, out)
	}
}
