package compiler

// A BETA ROW DROPS THE VARIABLES NOTHING BELOW IT READS.
//
// BuildBetaNodesRecursively (jet_rule_listener_rete.go) keeps in a vertex's beta
// row only the variables that the vertex's own triple binds or that a vertex or
// consequent below it reads; every other variable bound above it is a pruned
// variable and is left out of the row initialiser written to workspace.db
// (beta_row_config). So a variable an earlier antecedent joined on is dropped
// as soon as nothing below needs it, and several parent rows that differ only in
// it make one row of the vertex.
//
// The engine keeps a relation's rows as a set, and a row of such a vertex keeps
// the set of its supports: for each derivation, the values of the parent row at
// the positions the row leaves out (NodeVertex::support_key_positions, computed
// from the row initialiser at load, and BetaRelation's support sets,
// jets/rete/beta_relation.h). Withdrawing a derivation removes the row only when
// none is left. Until 2026-11 it removed the row at the first withdrawal, so a row
// still supported lost its consequents; the engine test beside this one
// (beta_row_pruning_engine_test.go, tag engine_rules) holds the fix.
//
// These tests pin what the compiler emits, so a change to the pruning is seen. The
// engine relies on one property of it: every variable a vertex's own triple binds
// is in its row, so the parent row's left-out values tell its derivations apart.

import (
	"slices"
	"strings"
	"testing"
)

// prunedRule is the shape: ?y is the join variable of the second antecedent and
// nothing after it reads ?y or ?w.
const prunedRule = `@JetCompilerDirective source_file = "pruned.jr";
	resource ex:q = "ex:q";
	resource ex:r = "ex:r";
	resource ex:t = "ex:t";
	resource ex:s = "ex:s";
	resource ex:u = "ex:u";
	[Pruned]: (?x ex:q ?y).(?y ex:r ?w).(?x ex:t ?z) -> (?x ex:s ?z);
`

// keptRule has the same antecedents, and a consequent that reads ?y, so no
// variable is pruned.
const keptRule = `@JetCompilerDirective source_file = "kept.jr";
	resource ex:q = "ex:q";
	resource ex:r = "ex:r";
	resource ex:t = "ex:t";
	resource ex:s = "ex:s";
	resource ex:u = "ex:u";
	[Kept]: (?x ex:q ?y).(?y ex:r ?w).(?x ex:t ?z) -> (?x ex:s ?z).(?x ex:u ?y);
`

// negatedRule's third vertex is a negation, which needs ?w and so comes after the
// second antecedent: its row is (?w, ?x), and ?y is pruned. With one ?w shared by
// every ?y, each ?y is a derivation of that one row.
const negatedRule = `@JetCompilerDirective source_file = "negated.jr";
	resource ex:q = "ex:q";
	resource ex:r = "ex:r";
	resource ex:n = "ex:n";
	resource ex:s = "ex:s";
	resource ex:a = "ex:a";
	[Negated]: (?x ex:q ?y).(?y ex:r ?w).not(?w ex:n ?x) -> (?x ex:s ex:a);
`

// filteredRule is prunedRule with a filter on the pruned vertex.
const filteredRule = `@JetCompilerDirective source_file = "filtered.jr";
	resource ex:q = "ex:q";
	resource ex:r = "ex:r";
	resource ex:t = "ex:t";
	resource ex:s = "ex:s";
	[Filtered]: (?x ex:q ?y).(?y ex:r ?w).(?x ex:t ?z).[?z != ex:q] -> (?x ex:s ?z);
`

// selfJoinRule joins a predicate with itself: a reflexive triple (a q a) is then both
// the first antecedent's triple and the second's join of one derivation.
const selfJoinRule = `@JetCompilerDirective source_file = "selfjoin.jr";
	resource ex:q = "ex:q";
	resource ex:s = "ex:s";
	[SelfJoin]: (?x ex:q ?y).(?y ex:q ?z) -> (?x ex:s ?z);
`

// selfJoinPrunedRule is selfJoinRule with a third vertex whose row (?x, ?w) leaves
// out ?y and ?z.
const selfJoinPrunedRule = `@JetCompilerDirective source_file = "selfjoinp.jr";
	resource ex:q = "ex:q";
	resource ex:s = "ex:s";
	resource ex:t = "ex:t";
	[SelfJoinP]: (?x ex:q ?y).(?y ex:q ?z).(?x ex:t ?w) -> (?x ex:s ?w);
`

// chainRule joins a predicate with itself three times: on a cycle (a q b), (b q a)
// one triple is the first antecedent's and the third's.
const chainRule = `@JetCompilerDirective source_file = "chain.jr";
	resource ex:q = "ex:q";
	resource ex:s = "ex:s";
	[Chain]: (?x ex:q ?y).(?y ex:q ?z).(?z ex:q ?w) -> (?x ex:s ?w);
`

// droppedParentVars returns, by antecedent vertex, the variables of its parent's
// beta row that its own beta row initialiser does not carry.
func droppedParentVars(t *testing.T, c *Compiler) map[int][]string {
	t.Helper()
	nodes := c.JetRuleModel().ReteNodes
	dropped := make(map[int][]string)
	for _, n := range nodes {
		if n.Type != "antecedent" || n.ParentVertex <= 0 {
			continue
		}
		var own []string
		for _, b := range n.BetaVarNodes {
			own = append(own, b.Id)
		}
		for _, b := range nodes[n.ParentVertex].BetaVarNodes {
			if !slices.Contains(own, b.Id) {
				dropped[n.Vertex] = append(dropped[n.Vertex], b.Id)
			}
		}
		// PrunedVars is the compiler's own account of the same thing
		if !slices.Equal(n.PrunedVars, dropped[n.Vertex]) {
			t.Errorf("vertex %d: pruned %v, and its row leaves out %v of its parent's", n.Vertex, n.PrunedVars, dropped[n.Vertex])
		}
	}
	return dropped
}

func TestBetaRowDropsAJoinVariable(t *testing.T) {
	c := NewCompiler("./testdata", "pruned.jr", false, false, true)
	if err := c.CompileBuffer(prunedRule); err != nil {
		t.Fatal(err)
	}
	if c.ErrorLog().Len() > 0 {
		t.Fatal(c.ErrorLog().String())
	}
	var join string
	for _, n := range c.JetRuleModel().ReteNodes {
		if n.Type == "antecedent" && n.Vertex == 2 {
			for _, b := range n.BetaVarNodes {
				if b.IsBinded && b.VarPos == 1 {
					join = b.Id
				}
			}
		}
	}
	if join == "" {
		t.Fatal("vertex 2 does not join on its parent's second column")
	}
	dropped := droppedParentVars(t, c)
	if !slices.Contains(dropped[3], join) {
		t.Errorf("vertex 3 keeps %s, the join variable of vertex 2; dropped %v", join, dropped)
	}
	if len(dropped) != 1 || len(dropped[3]) != 2 {
		t.Errorf("dropped %v, want vertex 3 alone to drop two variables", dropped)
	}
}

func TestBetaRowKeepsAVariableAConsequentReads(t *testing.T) {
	c := NewCompiler("./testdata", "kept.jr", false, false, true)
	if err := c.CompileBuffer(keptRule); err != nil {
		t.Fatal(err)
	}
	if c.ErrorLog().Len() > 0 {
		t.Fatal(c.ErrorLog().String())
	}
	dropped := droppedParentVars(t, c)
	if len(dropped[3]) != 1 {
		t.Errorf("dropped %v, want vertex 3 to drop ?w alone", dropped)
	}
}

// TestBetaRowKeepsEveryVariableOfItsTriple pins what the engine's support sets rely
// on: a vertex's row carries every variable of its own triple, so two derivations
// of one row differ only in the parent row's left-out values.
func TestBetaRowKeepsEveryVariableOfItsTriple(t *testing.T) {
	for _, r := range []struct{ file, rule string }{
		{"pruned.jr", prunedRule}, {"kept.jr", keptRule}, {"negated.jr", negatedRule}, {"filtered.jr", filteredRule},
	} {
		c := NewCompiler("./testdata", r.file, false, false, true)
		if err := c.CompileBuffer(r.rule); err != nil {
			t.Fatal(err)
		}
		if c.ErrorLog().Len() > 0 {
			t.Fatal(c.ErrorLog().String())
		}
		pruned := 0
		for _, n := range c.JetRuleModel().ReteNodes {
			if n.Type != "antecedent" {
				continue
			}
			pruned += len(n.PrunedVars)
			var row []string
			for _, b := range n.BetaVarNodes {
				row = append(row, b.Id)
			}
			for _, tok := range strings.FieldsFunc(n.NormalizedLabel, func(r rune) bool { return strings.ContainsRune(" ().", r) }) {
				if strings.HasPrefix(tok, "?") && !slices.Contains(row, tok) {
					t.Errorf("%s: vertex %d %s: its row %v leaves out %s of its own triple", r.file, n.Vertex, n.NormalizedLabel, row, tok)
				}
			}
		}
		// every rule here has a vertex that prunes (kept.jr's prunes ?w), or the
		// property is not tested on a row that leaves anything out
		if pruned == 0 {
			t.Errorf("%s: %d pruned variables", r.file, pruned)
		}
	}
}
