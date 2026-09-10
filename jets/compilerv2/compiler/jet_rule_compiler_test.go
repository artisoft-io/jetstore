package compiler

import (
	"testing"
)

// Compile a whole rule set the way the workspace compile does — through the
// import chain, with resources extracted from the rules.
//
// It used to name /home/michel/projects/repos/usi_ws, a checkout that is not
// part of this repository and no longer exists on any machine, so the test
// failed everywhere with "no such file or directory". The workspace under
// jets/jetrules/test_ws is in the repository and exercises the same path: a
// main rule file importing a data model, a lookup model and lookup table
// declarations.
func TestCompiler1(t *testing.T) {
	for _, mainRuleFile := range []string{
		"jet_rules/test_lookup_main.jr",
		"jet_rules/test_looping_main.jr",
		// The join_values rule set. An operator name is an opaque string from the
		// lexer to the factory -- binaryOp reduces to Identifier and nothing
		// whitelists the name -- so a rule using an operator is the only thing that
		// exercises it through the compiler at all. Its own test is
		// TestJoinValuesCompilesAndRuns in jets/jetrules/rete; this line is what
		// makes a break in it show up in the compiler's suite as well.
		"jet_rules/test_join_values_main.jr",
	} {
		jrCompiler := NewCompiler("../../jetrules/test_ws", mainRuleFile, false, false, true)
		if err := jrCompiler.Compile(); err != nil {
			t.Errorf("%s: %v\n%s", mainRuleFile, err, jrCompiler.ErrorLog().String())
		}
	}
}
