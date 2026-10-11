package compiler

import (
	"strings"
	"testing"
)

// A comment line between an antecedent and its filter is legal and changes
// nothing. The filter is part of the antecedent before it, and the grammar used
// to admit a comment only between whole antecedents, so commenting out the
// line above a filter turned the filter into a stray '[' that the parser read
// as the start of a new rule (rules_unit_test_harness I-37, 2026-10-11).
//
// Every variant must compile to the same rule as the version with no comments,
// with the filter still attached to the antecedent it follows.
func TestJetRuleListener_CommentBeforeFilter(t *testing.T) {
	const header = `@JetCompilerDirective source_file = "comment_filter.jr";
		resource abc:list = "abc:list";
		resource abc:loop = "abc:loop";
		resource abc:done = "abc:done";
`
	rule := func(body string) string {
		return header + "\n[R01]:\n" + body + "\n->\n(?s abc:done ?n);\n"
	}
	baseline := `(?s rdf:type jets:State).
		(?s abc:loop ?n).
		[(?s exist_not abc:list)]`

	compile := func(t *testing.T, src string) string {
		t.Helper()
		c := NewCompiler("", "comment_filter.jr", false, false, true)
		if err := c.CompileBuffer(src); err != nil {
			t.Fatalf("compile failed: %v\n%s", err, c.ErrorLog().String())
		}
		if c.ErrorLog().Len() > 0 {
			t.Fatalf("unexpected error log:\n%s", c.ErrorLog().String())
		}
		rules := c.JetRuleModel().Jetrules
		if len(rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(rules))
		}
		return rules[0].NormalizedLabel
	}

	want := compile(t, rule(baseline))
	if !strings.Contains(want, "[(") {
		t.Fatalf("baseline has no filter, so the comparison proves nothing: %s", want)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"commented-out antecedent between the term and its filter", `(?s rdf:type jets:State).
		(?s abc:loop ?n).
#  not(?s abc:other ?n).
		[(?s exist_not abc:list)]`},
		{"comment after a term with no dot", `(?s rdf:type jets:State).
		(?s abc:loop ?n)
		# why the filter is here
		[(?s exist_not abc:list)]`},
		{"several comment lines", `(?s rdf:type jets:State).
		(?s abc:loop ?n).
		# one
		# two
		[(?s exist_not abc:list)]`},
		{"comments around the filter as well", `(?s rdf:type jets:State).
		# before the term
		(?s abc:loop ?n).
		# between the term and its filter
		[(?s exist_not abc:list)]
		# after the filter`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := compile(t, rule(tc.body)); got != want {
				t.Errorf("comments changed the rule:\n got %s\nwant %s", got, want)
			}
		})
	}
}
