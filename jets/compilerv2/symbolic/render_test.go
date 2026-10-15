package symbolic

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/compilerv2/compiler"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const example = `{
  "imports": ["data_model/hc_model.jr"],
  "rules": [
    {
      "name": "FlagLargeClaim",
      "salience": 50,
      "properties": {"flag": "healthcare"},
      "antecedents": [
        {"subject": "?clm", "predicate": "rdf:type", "object": "hc:MedicalClaim"},
        {"subject": "?clm", "predicate": "hc:amount", "object": "?amt",
         "filter": {"op": ">", "lhs": "?amt", "rhs": 1000}},
        {"not": true, "subject": "?clm", "predicate": "hc:reviewed", "object": true}
      ],
      "consequents": [
        {"subject": "?clm", "predicate": "hc:large", "object": true},
        {"subject": "?clm", "predicate": "hc:label",
         "object": {"op": "+", "lhs": {"type": "text", "value": "large:"}, "rhs": {"op": "toText", "arg": "?amt"}}}
      ]
    }
  ]
}`

func TestRenderExample(t *testing.T) {
	f, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Render(f)
	if err != nil {
		t.Fatal(err)
	}
	want := Header + `
import "data_model/hc_model.jr"

[FlagLargeClaim, s=50, flag="healthcare"]:
  (?clm rdf:type hc:MedicalClaim).
  (?clm hc:amount ?amt).[(?amt > int(1000))].
  not(?clm hc:reviewed true)
  ->
  (?clm hc:large true).
  (?clm hc:label ("large:" + (toText ?amt)))
;
`
	if r.Source != want {
		t.Errorf("rendering:\n%s\nwant:\n%s", r.Source, want)
	}
	for line, ptr := range map[int]string{
		2: "/imports/0", 4: "/rules/0", 6: "/rules/0/antecedents/1",
		8: "", 10: "/rules/0/consequents/1", 11: "/rules/0",
	} {
		if got := r.PointerAt(line); got != ptr {
			t.Errorf("line %d points at %q, want %q", line, got, ptr)
		}
	}
}

// The rendering of the example compiles, and to the rule it describes.
func TestRenderedExampleCompiles(t *testing.T) {
	f, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	f.Imports = nil
	r, err := Render(f)
	if err != nil {
		t.Fatal(err)
	}
	c := compiler.NewCompiler(t.TempDir(), "example.jr", false, false, true)
	if err := c.CompileBuffer(r.Source); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
	rules := c.JetRuleModel().Jetrules
	if len(rules) != 1 {
		t.Fatalf("%d rules", len(rules))
	}
	got := rules[0]
	if got.Salience != 50 || !got.IsValid || got.Properties["flag"] != `"healthcare"` {
		t.Errorf("rule header: salience %d, valid %t, properties %v", got.Salience, got.IsValid, got.Properties)
	}
	if len(got.Antecedents) != 3 || len(got.Consequents) != 2 {
		t.Fatalf("%d antecedents, %d consequents", len(got.Antecedents), len(got.Consequents))
	}
}

// Every refusal names the element, and none reaches the compiler.
func TestRenderRefuses(t *testing.T) {
	rule := func(body string) string {
		return `{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "rdf:type", "object": "hc:C"}], ` +
			`"consequents": [{"subject": "?x", "predicate": "hc:p", "object": 1}]` + body + `}]}`
	}
	ant := func(a string) string {
		return `{"rules": [{"name": "R", "antecedents": [` + a + `], "consequents": [{"subject": "?x", "predicate": "hc:p", "object": 1}]}]}`
	}
	for _, tc := range []struct {
		name, doc, pointer string
	}{
		{"injection through an id", ant(`{"subject": "?x) -> (?y", "predicate": "rdf:type", "object": "hc:C"}`), "/rules/0/antecedents/0/subject"},
		{"a literal subject", ant(`{"subject": 5, "predicate": "rdf:type", "object": "hc:C"}`), "/rules/0/antecedents/0/subject"},
		{"a reserved variable", ant(`{"subject": "?not", "predicate": "rdf:type", "object": "hc:C"}`), "/rules/0/antecedents/0/subject"},
		{"a bad int", ant(`{"subject": "?x", "predicate": "hc:n", "object": {"type": "int", "value": "1.5"}}`), "/rules/0/antecedents/0/object"},
		{"an unknown literal type", ant(`{"subject": "?x", "predicate": "hc:n", "object": {"type": "float", "value": "1"}}`), "/rules/0/antecedents/0/object"},
		{"an unescaped quote", ant(`{"subject": "?x", "predicate": "hc:n", "object": {"type": "text", "value": "a\"b"}}`), "/rules/0/antecedents/0/object"},
		{"a bad operator", ant(`{"subject": "?x", "predicate": "hc:n", "object": "?n", "filter": {"op": "> 0] -> (", "lhs": "?n", "rhs": 0}}`), "/rules/0/antecedents/0/filter/op"},
		{"a half operation", ant(`{"subject": "?x", "predicate": "hc:n", "object": "?n", "filter": {"op": ">", "lhs": "?n"}}`), "/rules/0/antecedents/0/filter"},
		{"s as a property", rule(`, "properties": {"s": 10}`), "/rules/0/properties/s"},
		{"i as a property", rule(`, "properties": {"i": true}`), "/rules/0/properties/i"},
		{"a bad rule name", `{"rules": [{"name": "r 1", "antecedents": [], "consequents": []}]}`, "/rules/0/name"},
		{"no antecedent", `{"rules": [{"name": "R", "antecedents": [], "consequents": [{"subject": "?x", "predicate": "hc:p", "object": 1}]}]}`, "/rules/0/antecedents"},
		{"no rule", `{"rules": []}`, "/rules"},
		{"a bad import", `{"imports": ["x.jr\"\n[R]"], "rules": []}`, "/imports/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := Parse([]byte(tc.doc))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			_, err = Render(f)
			var re *RenderError
			if !errors.As(err, &re) {
				t.Fatalf("rendered, or failed without a pointer: %v", err)
			}
			if re.Pointer != tc.pointer {
				t.Errorf("pointer %q, want %q (%s)", re.Pointer, tc.pointer, re.Message)
			}
		})
	}
	dup := `{"rules": [` + strings.Repeat(`{"name": "R", "antecedents": [{"subject": "?x", "predicate": "rdf:type", "object": "hc:C"}], "consequents": [{"subject": "?x", "predicate": "hc:p", "object": 1}]},`, 2)
	dup = strings.TrimSuffix(dup, ",") + `]}`
	f, err := Parse([]byte(dup))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Render(f); err == nil || !strings.Contains(err.Error(), "/rules/1/name") {
		t.Errorf("two rules of one name rendered: %v", err)
	}
}

// A misspelt field is refused rather than ignored: "filtre" would otherwise
// compile a rule with no filter.
func TestParseIsStrict(t *testing.T) {
	for _, doc := range []string{
		`{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "p", "object": "o", "filtre": "?x"}], "consequents": []}]}`,
		`{"rules": [], "rule": []}`,
		`{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "p", "object": {"type": "int", "value": "1", "unit": "x"}}], "consequents": []}]}`,
		`{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "p", "object": "o", "filter": {"op": "+", "lhs": 1, "rhs": 2, "arg": 3}}], "consequents": []}]}`,
		`{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "p", "object": null}], "consequents": []}]}`,
	} {
		if _, err := Parse([]byte(doc)); err == nil {
			// The last two parse into a shape Render refuses; either refusal
			// is a refusal, so check both before failing.
			f, _ := Parse([]byte(doc))
			if _, rerr := Render(f); rerr == nil {
				t.Errorf("accepted: %s", doc)
			}
		}
	}
}

// A local name the lexer reads as a keyword is written with the grammar's own
// escape and compiles back to the same id.
func TestReservedLocalNameIsEscaped(t *testing.T) {
	doc := `{"rules": [{"name": "R", "antecedents": [{"subject": "?x", "predicate": "ex:date", "object": "?d"}], ` +
		`"consequents": [{"subject": "?x", "predicate": "ex:text", "object": "?d"}]}]}`
	f, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Render(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Source, `ex:"date"`) {
		t.Fatalf("not escaped:\n%s", r.Source)
	}
	c := compiler.NewCompiler(t.TempDir(), "esc.jr", false, false, true)
	if err := c.CompileBuffer(r.Source); err != nil {
		t.Fatalf("%v\n%s", err, c.ErrorLog().String())
	}
	label := c.JetRuleModel().Jetrules[0].NormalizedLabel
	if !strings.Contains(label, "ex:date") || !strings.Contains(label, "ex:text") {
		t.Errorf("label %s", label)
	}
}

// The reserved set is every identifier-shaped literal token of the grammar.
func TestReservedMatchesGrammar(t *testing.T) {
	g4, err := os.ReadFile(filepath.Join("..", "compiler", "JetRule.g4"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, m := range regexp.MustCompile(`'([A-Za-z_][A-Za-z0-9_]*)'`).FindAllSubmatch(g4, -1) {
		want[string(m[1])] = true
	}
	for w := range want {
		if !reserved[w] {
			t.Errorf("the grammar has keyword %q and reserved does not", w)
		}
	}
	for w := range reserved {
		if !want[w] {
			t.Errorf("reserved has %q and the grammar does not", w)
		}
	}
}

// Rendering is deterministic and survives a JSON round trip unchanged.
func TestJSONRoundTrip(t *testing.T) {
	f, err := Parse([]byte(example))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Render(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	g, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(g)
	if err != nil {
		t.Fatal(err)
	}
	if a.Source != b.Source {
		t.Errorf("rendering changed through JSON:\n%s\n%s", a.Source, b.Source)
	}
	if err := validateAgainstSchema(t, []byte(example)); err != nil {
		t.Errorf("the schema refuses the example: %v", err)
	}
}

func validateAgainstSchema(t *testing.T, doc []byte) error {
	t.Helper()
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(Schema()))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("rule_file.schema.json", schemaDoc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("rule_file.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return err
	}
	return s.Validate(v)
}
