package compiler

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

// Three ways a rule file's text was misread without a word: a rule property
// written in quotes kept its quotes when the compiler read it, a cast whose
// text held an opening parenthesis dropped the term it was in, and a string
// literal written across lines lost the spaces at the ends of its lines.

// compileSource compiles one main rule file and fails the test on an error.
func compileSource(t *testing.T, src string) *Compiler {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "main.jr", src)
	c := NewCompiler(dir, "main.jr", false, false, true)
	c.listener.autoAddResources = true
	if err := c.Compile(); err != nil {
		t.Fatalf("compile: %v\n%s", err, c.ErrorLog().String())
	}
	return c
}

func ruleNamed(t *testing.T, m *rete.JetruleModel, name string) *rete.JetruleNode {
	t.Helper()
	for _, r := range m.Jetrules {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no rule %s", name)
	return nil
}

func resourceByKey(m *rete.JetruleModel, key int) *rete.ResourceNode {
	for _, r := range m.Resources {
		if r.Key == key {
			return r
		}
	}
	return nil
}

// A property is read as the value it stands for, quoted or not, and is kept
// as it was written.
func TestQuotedRulePropertiesAreReadAsStrings(t *testing.T) {
	c := compileSource(t, strings.Join([]string{
		`[Quoted, o="false", s="10", aggregate_check="false"]: (?x rdf:type acme:A) -> (?x acme:p 1);`,
		`[Bare, o=false, s=10, aggregate_check=false]: (?x rdf:type acme:A) -> (?x acme:q 1);`,
		`[QuotedTrue, o="true", s="-5"]: (?x rdf:type acme:A) -> (?x acme:r 1);`,
		`[Default]: (?x rdf:type acme:A) -> (?x acme:s 1);`,
		`[Described, description="a \"quoted\" word"]: (?x rdf:type acme:A) -> (?x acme:t 1);`,
	}, "\n"))
	m := c.JetRuleModel()
	for _, tc := range []struct {
		rule          string
		optimization  bool
		salience      int
		aggregateSkip bool
	}{
		{"Quoted", false, 10, true},
		{"Bare", false, 10, true},
		{"QuotedTrue", true, -5, false},
		{"Default", true, 100, false},
	} {
		r := ruleNamed(t, m, tc.rule)
		if r.Optimization != tc.optimization {
			t.Errorf("%s: optimization %v, want %v", tc.rule, r.Optimization, tc.optimization)
		}
		if r.Salience != tc.salience {
			t.Errorf("%s: salience %d, want %d", tc.rule, r.Salience, tc.salience)
		}
		if got := aggregateCheckOptedOut(r); got != tc.aggregateSkip {
			t.Errorf("%s: aggregate check opted out %v, want %v", tc.rule, got, tc.aggregateSkip)
		}
	}

	// Kept as written: in the properties, so in rule_properties, and in the label.
	quoted := ruleNamed(t, m, "Quoted")
	if quoted.Properties["o"] != `"false"` || quoted.Properties["s"] != `"10"` {
		t.Errorf("properties %v, want them as written", quoted.Properties)
	}
	if !strings.HasPrefix(quoted.AuthoredLabel, `[Quoted, o="false", s="10", aggregate_check="false"]:`) {
		t.Errorf("label %s does not carry the properties as written", quoted.AuthoredLabel)
	}
	described := ruleNamed(t, m, "Described")
	if got := RuleProperty(described, "description"); got != `a "quoted" word` {
		t.Errorf("description read as %q", got)
	}
	if got := RuleProperty(described, "absent"); got != "" {
		t.Errorf("an absent property read as %q", got)
	}
}

// A quoted salience that is not an integer is still refused, naming the value
// as written, and so is an empty one.
func TestQuotedSalienceMustBeAnInteger(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{`[R1, s="high"]: (?x rdf:type acme:A) -> (?x acme:p 1);`, `invalid salience value '"high"' in rule R1`},
		{`[R1, s=""]: (?x rdf:type acme:A) -> (?x acme:p 1);`, `invalid salience value '""' in rule R1`},
	} {
		dir := t.TempDir()
		write(t, dir, "main.jr", tc.src)
		c := NewCompiler(dir, "main.jr", false, false, true)
		c.listener.autoAddResources = true
		if err := c.Compile(); err == nil {
			t.Errorf("%s compiled", tc.src)
		}
		if !strings.Contains(c.ErrorLog().String(), tc.want) {
			t.Errorf("%s: error log %q does not say %q", tc.src, c.ErrorLog().String(), tc.want)
		}
	}
}

// A cast's value may hold parentheses: it is everything between the first '('
// and the closing ')'. The rules are not optimised, so their terms are read
// in the order written.
func TestCastValueMayHoldParentheses(t *testing.T) {
	c := compileSource(t, strings.Join([]string{
		`[R1, o=false]: (?x rdf:type acme:A).(?x acme:p text("a(b")).(?x acme:q ?w).[(?w != text("g(h)"))] -> (?x acme:r text("c(d"));`,
		`[R2, o=false]: (?x rdf:type acme:A).(?x acme:p text("a)b(")) -> (?x acme:s date("2024-01-02"));`,
		`triple(acme:s, acme:p, text("e((f"));`,
	}, "\n"))
	m := c.JetRuleModel()

	r1 := ruleNamed(t, m, "R1")
	if len(r1.Antecedents) != 3 || len(r1.Consequents) != 1 {
		t.Fatalf("R1 has %d antecedents and %d consequents, want 3 and 1: %s",
			len(r1.Antecedents), len(r1.Consequents), r1.AuthoredLabel)
	}
	if o := resourceByKey(m, r1.Antecedents[1].ObjectKey); o == nil || o.Type != "text" || o.Value != "a(b" {
		t.Errorf("R1 antecedent object %+v, want text a(b", o)
	}
	if r1.Antecedents[2].Filter == nil {
		t.Errorf("R1 lost the filter on its third antecedent: %s", r1.AuthoredLabel)
	}
	if r1.Consequents[0].ObjectExpr == nil {
		t.Errorf("R1 lost its consequent's object: %s", r1.AuthoredLabel)
	}

	r2 := ruleNamed(t, m, "R2")
	if len(r2.Antecedents) != 2 {
		t.Fatalf("R2 has %d antecedents, want 2: %s", len(r2.Antecedents), r2.AuthoredLabel)
	}
	if o := resourceByKey(m, r2.Antecedents[1].ObjectKey); o == nil || o.Value != "a)b(" {
		t.Errorf("R2 antecedent object %+v, want text a)b(", o)
	}

	if len(m.Triples) != 1 || m.Triples[0].O == nil || m.Triples[0].O.Value != "e((f" {
		t.Fatalf("triples %+v, want one whose object is text e((f", m.Triples)
	}

	texts := map[string]bool{}
	for _, r := range m.Resources {
		if r.Type == "text" && r.Inline {
			texts[r.Value] = true
		}
	}
	for _, want := range []string{"a(b", "g(h)", "c(d", "a)b(", "e((f"} {
		if !texts[want] {
			t.Errorf("no inline text literal %q; have %v", want, texts)
		}
	}
}

// A cast that cannot be read is a located compile error, never a dropped term.
func TestUnreadableCastIsAnError(t *testing.T) {
	c := NewCompiler("./testdata", "jetrule.jr", false, true, false)
	s := c.listener
	for _, txt := range []string{`xyz("a(b")`, `bool("maybe(")`, `oops)`} {
		s.errorLog.Reset()
		if atom := s.parseObjectAtom(txt, ""); atom != nil {
			t.Errorf("%s read as %+v, want a refusal", txt, atom)
		}
		if !strings.Contains(s.errorLog.String(), "error") {
			t.Errorf("%s refused without an error: %q", txt, s.errorLog.String())
		}
	}
}

// A literal written across lines keeps the spaces and tabs at the ends of its
// lines, in a file with LF or CRLF line ends.
func TestMultiLineLiteralKeepsItsWhitespace(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		src := strings.Join([]string{
			`text multi = "first line  `,
			"\tsecond line\t",
			`    third";`,
			`[R1, description="A rule `,
			`    described on two lines"]: (?x rdf:type acme:A) -> (?x acme:p "a  `,
			`  b");`,
			`  [R2]: (?x rdf:type acme:A) -> (?x acme:q 1);  `,
		}, eol)
		c := compileSource(t, src)
		m := c.JetRuleModel()
		var multi *rete.ResourceNode
		texts := map[string]bool{}
		for _, r := range m.Resources {
			if r.Id == "multi" {
				multi = r
			}
			if r.Type == "text" && r.Inline {
				texts[r.Value] = true
			}
		}
		if multi == nil || multi.Value != "first line  \n\tsecond line\t\n    third" {
			t.Errorf("%q: multi is %+v", eol, multi)
		}
		if !texts["a  \n  b"] {
			t.Errorf("%q: no inline literal %q; have %v", eol, "a  \n  b", texts)
		}
		if got := RuleProperty(ruleNamed(t, m, "R1"), "description"); got != "A rule \n    described on two lines" {
			t.Errorf("%q: description read as %q", eol, got)
		}
		ruleNamed(t, m, "R2")
	}
}

// Whitespace the parser does not skip is still removed from around a line, as
// it was before lines kept their spaces, so a file that compiled still does.
func TestLineWithOtherWhitespaceIsStillTrimmed(t *testing.T) {
	c := compileSource(t, strings.Join([]string{
		"\f[R1]: (?x rdf:type acme:A) -> (?x acme:p 1); ",
		"  [R2]: (?x rdf:type acme:A) -> (?x acme:q 1);\v",
	}, "\n"))
	m := c.JetRuleModel()
	ruleNamed(t, m, "R1")
	ruleNamed(t, m, "R2")

	for _, tc := range []struct{ raw, want string }{
		{"", ""},
		{"   ", ""},
		{"\r", ""},
		{"  a b  ", "  a b  "},
		{"\ta\t\r", "\ta\t"},
		{"\fa", "a"},
		{"a ", "a"},
		{" \f a ", "a"},
	} {
		if got := bufferLine(tc.raw, strings.TrimSpace(tc.raw)); got != tc.want {
			t.Errorf("bufferLine(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
