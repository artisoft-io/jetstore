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
// The engine keeps a relation's rows as a set (BetaRelation::insert_beta_row,
// jets/rete/rete_session.h) and removes a row when any parent row that makes it
// is withdrawn (ReteSession::triple_updated), without asking whether another
// parent row still makes it. A row of a vertex with pruned variables can
// therefore lose its consequents while it is still supported: the engine test
// beside this one (beta_row_pruning_engine_test.go, tag engine_rules) shows it.
//
// These tests pin what the compiler emits, so a change to the pruning is seen.

import (
	"slices"
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
