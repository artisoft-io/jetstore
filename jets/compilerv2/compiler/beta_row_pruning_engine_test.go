//go:build engine_rules

package compiler

// WHAT A PRUNED VARIABLE COSTS TRUTH MAINTENANCE (see beta_row_pruning_test.go).
//
// Each case compiles one rule with this compiler, loads the workspace.db in the
// C++ engine, gives the rule several derivations of one inferred triple, withdraws
// them one by one, and checks after each run that the inferred triple is there
// exactly while a derivation is left.
//
//   - kept: the consequent reads ?y, so the terminal vertex has one row per
//     derivation and the inferred triple one support per row.
//   - pruned: nothing after the second antecedent reads ?y, so the derivations make
//     one row of the terminal vertex. Until 2026-11 erasing the asserted triple
//     behind one of them removed that row and retracted the inferred triple while
//     the others remained; the row now keeps the set of its supports (BetaRelation's
//     support sets, jets/rete/beta_relation.h) and goes with the last.
//   - negated: the pruned vertex is a negation, whose row is withdrawn by a triple
//     that matches it for every parent row at once, and made again when it leaves.
//   - filtered: the pruned vertex has a filter, evaluated on the row.
//
// It needs libjets, so it is behind a build tag; ctest runs it with the engine's
// tests, as go_beta_row_pruning_engine (jets/CMakeLists.txt):
//
//	LD_LIBRARY_PATH=<jetstore>/build/jets go test -count=1 -tags engine_rules -run 'TruthMaintenance$' ./jets/compilerv2/compiler/

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artisoft-io/jetstore/jets/bridge"
)

func TestPrunedVariableTruthMaintenance(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	for _, c := range []struct {
		name, rule string
	}{
		{"kept", keptRule},
		{"pruned", prunedRule},
		{"filtered", filteredRule},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newRuleSession(t, c.rule)
			for _, i := range []string{"1", "2", "3"} {
				s.insert("x", "ex:q", "y"+i)
				s.insert("y"+i, "ex:r", "w"+i)
			}
			s.insert("x", "ex:t", "z")
			s.run()
			s.want("three derivations", "x", "ex:s", "z", true)

			s.erase("x", "ex:q", "y1")
			s.run()
			s.want("y1 erased, two derivations left", "x", "ex:s", "z", true)

			// the second antecedent's triple, not the first: the derivation goes
			// at the pruned vertex's parent
			s.erase("y2", "ex:r", "w2")
			s.run()
			s.want("y2's join erased, one derivation left", "x", "ex:s", "z", true)

			s.erase("x", "ex:q", "y3")
			s.run()
			s.want("every derivation erased", "x", "ex:s", "z", false)

			s.insert("x", "ex:q", "y1")
			s.run()
			s.want("y1 asserted again", "x", "ex:s", "z", true)

			s.erase("x", "ex:t", "z")
			s.run()
			s.want("the triple the terminal vertex joins erased", "x", "ex:s", "z", false)
		})
	}

	t.Run("negated", func(t *testing.T) {
		s := newRuleSession(t, negatedRule)
		for _, i := range []string{"1", "2", "3"} {
			s.insert("x", "ex:q", "y"+i)
			s.insert("y"+i, "ex:r", "w")
		}
		s.run()
		s.want("three derivations", "x", "ex:s", "ex:a", true)

		s.erase("x", "ex:q", "y1")
		s.run()
		s.want("y1 erased, two derivations left", "x", "ex:s", "ex:a", true)

		s.insert("w", "ex:n", "x")
		s.run()
		s.want("the negated triple asserted", "x", "ex:s", "ex:a", false)

		s.erase("w", "ex:n", "x")
		s.run()
		s.want("the negated triple erased", "x", "ex:s", "ex:a", true)

		s.erase("x", "ex:q", "y2")
		s.run()
		s.want("y2 erased, one derivation left", "x", "ex:s", "ex:a", true)

		s.erase("x", "ex:q", "y3")
		s.run()
		s.want("every derivation erased", "x", "ex:s", "ex:a", false)
	})
}

// TestDeletedTripleJoinTruthMaintenance: a triple that is both a parent row's and the
// join below it, deleted. The parent's callback removes the parent row, and the graph
// no longer holds the triple when the retraction asks it for the row's joins, so until
// 2026-11 the derivation was not withdrawn and its inference stayed; at a vertex that
// keeps supports, the stale support kept the row after its other derivations went. A
// retraction now also offers the deleted triple as a join (ReteSession::DeletedTriple).
func TestDeletedTripleJoinTruthMaintenance(t *testing.T) {
	saved := workspaceSchema
	workspaceSchema = filepath.Join("..", "..", "workspace_schema.sql")
	t.Cleanup(func() { workspaceSchema = saved })

	t.Run("reflexive", func(t *testing.T) {
		s := newRuleSession(t, selfJoinRule)
		s.insert("a", "ex:q", "a")
		s.run()
		s.want("(a q a) asserted", "a", "ex:s", "a", true)
		s.erase("a", "ex:q", "a")
		s.run()
		s.want("(a q a) erased", "a", "ex:s", "a", false)
	})

	t.Run("reflexive_pruned", func(t *testing.T) {
		s := newRuleSession(t, selfJoinPrunedRule)
		s.insert("a", "ex:q", "a")
		s.insert("a", "ex:q", "b")
		s.insert("b", "ex:q", "c")
		s.insert("a", "ex:t", "w")
		s.run()
		s.want("three derivations", "a", "ex:s", "w", true)
		s.erase("a", "ex:q", "a")
		s.run()
		s.want("(a q a) erased, a derivation through b left", "a", "ex:s", "w", true)
		s.erase("a", "ex:q", "b")
		s.run()
		s.want("every derivation erased", "a", "ex:s", "w", false)
	})

	t.Run("cycle", func(t *testing.T) {
		s := newRuleSession(t, chainRule)
		s.insert("a", "ex:q", "b")
		s.insert("b", "ex:q", "a")
		s.run()
		s.want("the cycle asserted", "a", "ex:s", "b", true)
		s.want("the cycle asserted", "b", "ex:s", "a", true)
		s.erase("a", "ex:q", "b")
		s.run()
		s.want("(a q b) erased", "a", "ex:s", "b", false)
		s.want("(a q b) erased", "b", "ex:s", "a", false)
	})
}

// ruleSession is one rule compiled and loaded in the C++ engine, with an rdf session
// and a rete session over it
type ruleSession struct {
	t    *testing.T
	rdfs *bridge.RDFSession
	rs   *bridge.ReteSession
}

func newRuleSession(t *testing.T, rule string) *ruleSession {
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
	t.Cleanup(func() { js.ReleaseJetRules() })
	rdfs, err := js.NewRDFSession()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rdfs.ReleaseRDFSession() })
	rs, err := js.NewReteSession(rdfs, ruleFile)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rs.ReleaseReteSession() })
	return &ruleSession{t: t, rdfs: rdfs, rs: rs}
}

func (s *ruleSession) r(name string) *bridge.Resource {
	s.t.Helper()
	res, err := s.rdfs.NewResource(name)
	if err != nil {
		s.t.Fatal(err)
	}
	return res
}

func (s *ruleSession) insert(sub, pred, obj string) {
	s.t.Helper()
	if _, err := s.rdfs.Insert(s.r(sub), s.r(pred), s.r(obj)); err != nil {
		s.t.Fatal(err)
	}
}

func (s *ruleSession) erase(sub, pred, obj string) {
	s.t.Helper()
	if _, err := s.rdfs.Erase(s.r(sub), s.r(pred), s.r(obj)); err != nil {
		s.t.Fatal(err)
	}
}

func (s *ruleSession) run() {
	s.t.Helper()
	if msg, err := s.rs.ExecuteRules(); err != nil {
		s.t.Fatalf("%v: %s", err, msg)
	}
}

// want checks whether (sub pred obj) is in the session after `step`
func (s *ruleSession) want(step, sub, pred, obj string, present bool) {
	s.t.Helper()
	n, err := s.rdfs.Contains(s.r(sub), s.r(pred), s.r(obj))
	if err != nil {
		s.t.Fatal(err)
	}
	if (n > 0) != present {
		s.t.Errorf("%s: (%s %s %s) present is %v, want %v", step, sub, pred, obj, n > 0, present)
	}
}
