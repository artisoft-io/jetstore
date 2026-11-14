package compiler

import (
	"strings"
	"testing"

	"github.com/antlr4-go/antlr/v4"
	"github.com/artisoft-io/jetstore/jets/compilerv2/parser"
	"github.com/artisoft-io/jetstore/jets/jetrules/rete"
)

func TestUnquoteString(t *testing.T) {
	for _, tc := range []struct{ token, want string }{
		{`"no escapes here"`, `no escapes here`},
		{`""`, ``},
		{`"\""`, `"`},
		{`"a\\b"`, `a\b`},
		{`"\t"`, "\t"},
		{`"a\nb\rc"`, "a\nb\rc"},
		{`"ends with \\"`, `ends with \`},
		{`"\\\\"`, `\\`},
		{`"\\d"`, `\d`},
		{`"\\\""`, `\"`},
		// Any other backslash is kept, with the character after it.
		{`"^(\d\d\d)"`, `^(\d\d\d)`},
		{`",\s+(\w+)"`, `,\s+(\w+)`},
		{`"\bA\x"`, `\bA\x`},
		{`"{\"validation\":\"passed\"}"`, `{"validation":"passed"}`},
		{`"MERGED \"MARKET\" CHARGE BACK"`, `MERGED "MARKET" CHARGE BACK`},
		{`"(-?\d*,?\d+(\.?\d{1,2})?)"`, `(-?\d*,?\d+(\.?\d{1,2})?)`},
		{`"été \t ok"`, "été \t ok"},
		// Text that is not a double-quoted string is not a STRING token.
		{`true`, `true`},
		{`acme:p`, `acme:p`},
		{`create_uuid_resource()`, `create_uuid_resource()`},
		{`"`, `"`},
		{`'single'`, `'single'`},
	} {
		if got := UnquoteString(tc.token); got != tc.want {
			t.Errorf("UnquoteString(%s) = %q, want %q", tc.token, got, tc.want)
		}
	}
}

func TestQuoteString(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{`plain`, `"plain"`},
		{``, `""`},
		{`"`, `"\""`},
		{`a\b`, `"a\b"`},
		{`^(\d\d\d)`, `"^(\d\d\d)"`},
		{`a\\b`, `"a\\\b"`},
		{`ends with \`, `"ends with \\"`},
		{`\t`, `"\\t"`},
		{"\t\n\r", `"\t\n\r"`},
		{"\\\t", `"\\\t"`},
		{`{"validation":"passed"}`, `"{\"validation\":\"passed\"}"`},
	} {
		if got := QuoteString(tc.value); got != tc.want {
			t.Errorf("QuoteString(%q) = %s, want %s", tc.value, got, tc.want)
		}
	}
}

// Every value survives QuoteString then the lexer then UnquoteString, which is
// the round trip the symbolic renderer and the reader's source_file directive
// rely on. The values are built from the characters an escape involves.
func TestQuoteStringRoundTripsThroughTheLexer(t *testing.T) {
	alphabet := []string{`\`, `"`, "t", "n", "r", "d", "\t", "\n", "\r", "x", "é"}
	values := []string{""}
	for _, a := range alphabet {
		values = append(values, a)
		for _, b := range alphabet {
			values = append(values, a+b)
			for _, c := range alphabet {
				values = append(values, a+b+c)
			}
		}
	}
	for _, v := range values {
		quoted := QuoteString(v)
		lexer := parser.NewJetRuleLexer(antlr.NewInputStream(quoted + " ;"))
		lexer.RemoveErrorListeners()
		tokens := lexer.GetAllTokens()
		if len(tokens) == 0 || tokens[0].GetTokenType() != parser.JetRuleLexerSTRING || tokens[0].GetText() != quoted {
			t.Fatalf("%q: QuoteString gave %s, which does not lex as one STRING", v, quoted)
		}
		if got := UnquoteString(tokens[0].GetText()); got != v {
			t.Fatalf("%q: quoted as %s, read back as %q", v, quoted, got)
		}
	}
}

// The lexer takes \" and \\ as escapes, so a string may contain a quote and
// may end in a backslash; a backslash before anything else is an ordinary
// character.
func TestStringTokenEscapes(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want []string
	}{
		{`"a\"b" "c"`, []string{`"a\"b"`, `"c"`}},
		{`"a\\" "c"`, []string{`"a\\"`, `"c"`}},
		{`"\d" "c"`, []string{`"\d"`, `"c"`}},
	} {
		lexer := parser.NewJetRuleLexer(antlr.NewInputStream(tc.src))
		lexer.RemoveErrorListeners()
		var got []string
		for _, tok := range lexer.GetAllTokens() {
			if tok.GetTokenType() == parser.JetRuleLexerSTRING {
				got = append(got, tok.GetText())
			}
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%s lexes as %q, want %q", tc.src, got, tc.want)
		}
	}
}

// Every place the compiler reads a STRING token stores the unescaped value:
// literal declarations, resource and volatile resource values, a rule's terms
// in both forms, a triple, a compiler directive, a rule sequence and a lookup
// table.
func TestStringLiteralEscapesAreReadEverywhere(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "main.jr", strings.Join([]string{
		`@JetCompilerDirective note = "say \"hi\"";`,
		`text plain = "no escapes";`,
		`text quote = "\"";`,
		`text backslash = "a\\b";`,
		`text trailing = "ends with \\";`,
		`text tab = "a\tb";`,
		`text newline = "a\nb";`,
		`text cr = "a\rb";`,
		`text json = "{\"validation\":\"passed\"}";`,
		`text digits = "^(\d\d\d)";`,
		`text firstName = ",\s+(\w+)";`,
		`text escapedDigit = "\\d";`,
		`date day = "2024-01-02";`,
		`resource acme:quoted = "acme:a\"b";`,
		`volatile_resource vol = "v\\w";`,
		`lookup_table acme:L {`,
		`  $csv_file = "lookups/a\\b.csv",`,
		`  $key = ["k\"1"],`,
		`  $columns = ["col1" as text]`,
		`};`,
		`rule_sequence seq {`,
		`  $main_rule_sets = ["set\\1.jr"]`,
		`};`,
		`[R1]: (?x rdf:type acme:A).(?x acme:p "in \"term\"").(?x acme:q ?w).[(?w != text("cast\\ed"))] -> (?x acme:r "out\tterm");`,
		`[R2]: (?x rdf:type acme:A).(?x acme:p ?v).[(?v literal_regex "^(\d+)")] -> (?x acme:s (?v literal_regex "x\"y"));`,
		`triple(acme:s, acme:p, "tri\\ple");`,
	}, "\n"))
	c := NewCompiler(dir, "main.jr", false, false, true)
	c.listener.autoAddResources = true
	if err := c.Compile(); err != nil {
		t.Fatalf("compile: %v\n%s", err, c.ErrorLog().String())
	}
	model := c.JetRuleModel()

	byID := map[string]*rete.ResourceNode{}
	texts := map[string]bool{}
	for _, r := range model.Resources {
		if r.Id != "" {
			byID[r.Id] = r
		}
		if r.Type == "text" && r.Inline {
			texts[r.Value] = true
		}
	}
	for id, want := range map[string]string{
		"plain":        "no escapes",
		"quote":        `"`,
		"backslash":    `a\b`,
		"trailing":     `ends with \`,
		"tab":          "a\tb",
		"newline":      "a\nb",
		"cr":           "a\rb",
		"json":         `{"validation":"passed"}`,
		"digits":       `^(\d\d\d)`,
		"firstName":    `,\s+(\w+)`,
		"escapedDigit": `\d`,
		"day":          "2024-01-02",
		"acme:quoted":  `acme:a"b`,
		"vol":          `_0:v\w`, // a volatile resource keeps its prefix
	} {
		r, ok := byID[id]
		if !ok {
			t.Errorf("%s: no resource", id)
			continue
		}
		if r.Value != want {
			t.Errorf("%s: stored %q, want %q", id, r.Value, want)
		}
	}
	for _, want := range []string{`in "term"`, `cast\ed`, "out\tterm", `^(\d+)`, `x"y`, `tri\ple`} {
		if !texts[want] {
			t.Errorf("no inline text literal %q; have %v", want, texts)
		}
	}
	if got := model.CompilerDirectives["note"]; got != `say "hi"` {
		t.Errorf("directive note = %q", got)
	}
	if len(model.RuleSequences) != 1 || len(model.RuleSequences[0].RuleSets) != 1 || model.RuleSequences[0].RuleSets[0] != `set\1.jr` {
		t.Errorf("rule sequence: %+v", model.RuleSequences)
	}
	if len(model.LookupTables) != 1 {
		t.Fatalf("lookup tables: %+v", model.LookupTables)
	}
	if lt := model.LookupTables[0]; lt.CsvFile != `lookups/a\b.csv` || len(lt.Key) != 1 || lt.Key[0] != `k"1` {
		t.Errorf("lookup table: csv %q, key %q", lt.CsvFile, lt.Key)
	}
}

// The reader's source_file directive quotes the file name, so a name the
// listener reads back is the name the reader was given.
func TestSourceFileDirectiveQuotesTheName(t *testing.T) {
	name := `odd "name" \t.jr`
	reader := NewRuleFileReader("", name, func(_, _ string) (string, error) { return "text x = \"y\";\n", nil })
	combined, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	c := NewCompiler("", name, false, false, true)
	if err := c.CompileBuffer(combined); err != nil {
		t.Fatalf("compile: %v\n%s", err, c.ErrorLog().String())
	}
	if got := c.listener.currentRuleFileName; got != name {
		t.Errorf("source file read back as %q, want %q", got, name)
	}
}
