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
	} {
		jrCompiler := NewCompiler("../../jetrules/test_ws", mainRuleFile, false, false, true)
		if err := jrCompiler.Compile(); err != nil {
			t.Errorf("%s: %v\n%s", mainRuleFile, err, jrCompiler.ErrorLog().String())
		}
	}
}
