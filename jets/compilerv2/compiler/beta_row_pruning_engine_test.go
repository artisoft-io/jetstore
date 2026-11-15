//go:build engine_rules

package compiler

// WHAT A PRUNED VARIABLE COSTS TRUTH MAINTENANCE (see beta_row_pruning_test.go).
//
// Each case compiles one rule with this compiler, loads the workspace.db in the
// C++ engine, gives the rule three derivations of one inferred triple, erases
// the asserted triple behind one of them, and asks whether the inferred triple
// is still there. It should be: two derivations remain.
//
//   - kept: the consequent reads ?y, so the terminal vertex has three rows, the
//     inferred triple three supports, and erasing one leaves it. This passes.
//   - pruned: nothing after the second antecedent reads ?y, so the three parent
//     rows make one row of the terminal vertex, and erasing one of them removes
//     that row and retracts the triple. It is skipped: making it pass changes
//     what rules infer.
//
// It needs libjets, so it is behind a build tag; ctest runs it with the engine's
// tests, as go_beta_row_pruning_engine (jets/CMakeLists.txt):
//
//	LD_LIBRARY_PATH=<jetstore>/build/jets go test -count=1 -tags engine_rules -run TestPrunedVariableTruthMaintenance ./jets/compilerv2/compiler/

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

// overRetraction is why the pruned case is skipped.
const overRetraction = "known over-retraction: the terminal vertex's row drops ?y, so three derivations make one beta row, " +
	"and erasing the asserted triple behind one of them removes the row and retracts the inferred triple " +
	"while two derivations remain. Fixing it changes what rules infer (a row would have to count its parent rows, " +
	"or the compiler keep every variable), so it is not fixed here"

func TestPrunedVariableTruthMaintenance(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	for _, c := range []struct {
		name, rule string
		skip       string
	}{
		{"kept", keptRule, ""},
		{"pruned", prunedRule, overRetraction},
	} {
		t.Run(c.name, func(t *testing.T) {
			before, after := runThreeDerivations(t, c.rule)
			if !before {
				t.Fatal("the rule did not infer (x ex:s z)")
			}
			if !after && c.skip != "" {
				t.Skip(c.skip)
			}
			if !after {
				t.Error("erasing one of three derivations retracted (x ex:s z)")
			}
			if after && c.skip != "" {
				t.Errorf("the inferred triple survived: the over-retraction this case was skipped for is gone; remove the skip")
			}
		})
	}
}

// runThreeDerivations compiles rule, asserts (x q yi) and (yi r wi) for three i
// and (x t z), runs the rules, erases (x q y1), runs them again, and says
// whether (x ex:s z) was there after each run.
func runThreeDerivations(t *testing.T, rule string) (before, after bool) {
	t.Helper()
	dir := t.TempDir()
	const ruleFile = "jet_rules/rule_main.jr"
	if err := os.MkdirAll(filepath.Join(dir, "jet_rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ruleFile), []byte(rule), 0o644); err != nil {
		t.Fatal(err)
	}
	c := NewCompiler(dir, ruleFile, false, false, true)
	if err := c.Compile(); err != nil {
		t.Fatal(err)
	}
	if c.ErrorLog().Len() > 0 {
		t.Fatal(c.ErrorLog().String())
	}
	if err := c.SaveModel(); err != nil {
		t.Fatal(err)
	}

	js, err := bridge.LoadJetRules("beta_row_pruning", filepath.Join(dir, "workspace.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer js.ReleaseJetRules()
	rdfs, err := js.NewRDFSession()
	if err != nil {
		t.Fatal(err)
	}
	defer rdfs.ReleaseRDFSession()
	rs, err := js.NewReteSession(rdfs, ruleFile)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.ReleaseReteSession()

	r := func(name string) *bridge.Resource {
		t.Helper()
		res, err := rdfs.NewResource(name)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	insert := func(s, p, o string) {
		t.Helper()
		if _, err := rdfs.Insert(r(s), r(p), r(o)); err != nil {
			t.Fatal(err)
		}
	}
	inferred := func() bool {
		t.Helper()
		n, err := rdfs.Contains(r("x"), r("ex:s"), r("z"))
		if err != nil {
			t.Fatal(err)
		}
		return n > 0
	}
	run := func() {
		t.Helper()
		if msg, err := rs.ExecuteRules(); err != nil {
			t.Fatalf("%v: %s", err, msg)
		}
	}

	for _, i := range []string{"1", "2", "3"} {
		insert("x", "ex:q", "y"+i)
		insert("y"+i, "ex:r", "w"+i)
	}
	insert("x", "ex:t", "z")
	run()
	before = inferred()
	if _, err := rdfs.Erase(r("x"), r("ex:q"), r("y1")); err != nil {
		t.Fatal(err)
	}
	run()
	return before, inferred()
}
