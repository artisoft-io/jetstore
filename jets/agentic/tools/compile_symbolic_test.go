package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// I-250's arm: the rules as a symbolic JSON document, rendered by the tool.

const symbolicRules = `{
  "imports": ["data_model/jets_model.jr"],
  "rules": [
    {
      "name": "SymbolicArm1",
      "salience": 50,
      "antecedents": [
        {"subject": "?e", "predicate": "rdf:type", "object": "jets:Entity"},
        {"subject": "?e", "predicate": "jets:key", "object": "?k"}
      ],
      "consequents": [
        {"subject": "?e", "predicate": "jets:completed", "object": true}
      ]
    }
  ]
}`

func callSymbolic(t *testing.T, args string) *CompileReport {
	t.Helper()
	out, err := CompileRuleFile(context.Background(), fixture(t), json.RawMessage(args))
	if err != nil {
		t.Fatalf("a verdict about the rules is not a tool failure: %v", err)
	}
	return out.(*CompileReport)
}

// flatArgs is the canonical form of the arm: the document's own fields are
// the arguments.
func flatArgs(t *testing.T, doc string) string {
	t.Helper()
	var d struct {
		Imports json.RawMessage `json:"imports"`
		Rules   json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		t.Fatal(err)
	}
	return `{"rules": ` + string(d.Rules) + `, "imports": ` + string(d.Imports) + `}`
}

func TestCompileRuleFile_SymbolicRulesCompile(t *testing.T) {
	report := callSymbolic(t, flatArgs(t, symbolicRules))
	if !report.Valid {
		t.Fatalf("expected a clean compile, got %+v", report.Diagnostics)
	}
	if !strings.Contains(report.Source, "[SymbolicArm1, s=50]:") ||
		!strings.Contains(report.Source, `import "data_model/jets_model.jr"`) {
		t.Errorf("the report does not carry the rendered source:\n%s", report.Source)
	}
}

// The arm's other readings: a whole document under 'rules', and a string
// holding either form, each of which a model sends often enough to matter
// (I-250's measurement) and each of which has one meaning.
func TestCompileRuleFile_SymbolicRulesOtherReadings(t *testing.T) {
	quotedDoc, _ := json.Marshal(symbolicRules)
	var d struct {
		Rules json.RawMessage `json:"rules"`
	}
	_ = json.Unmarshal([]byte(symbolicRules), &d)
	quotedArray, _ := json.Marshal(string(d.Rules))
	for _, args := range []string{
		`{"rules": ` + symbolicRules + `}`,
		`{"rules": ` + string(quotedDoc) + `}`,
		`{"rules": ` + string(quotedArray) + `, "imports": ["data_model/jets_model.jr"]}`,
		`{"rules": ` + strings.TrimSuffix(strings.TrimSpace(string(d.Rules)), "]") + `, {"imports": ["data_model/jets_model.jr"]}]}`,
	} {
		if report := callSymbolic(t, args); !report.Valid {
			t.Errorf("expected a clean compile of %.80s..., got %+v", args, report.Diagnostics)
		}
	}
}

// Imports beside a whole document would be two lists of imports, and imports
// with no rules belong to nothing.
func TestCompileRuleFile_SymbolicImportsHaveOnePlace(t *testing.T) {
	report := callSymbolic(t, `{"rules": `+symbolicRules+`, "imports": ["data_model/jets_model.jr"]}`)
	if report.Valid || len(report.Diagnostics) != 1 || report.Diagnostics[0].Pointer != "/imports" {
		t.Errorf("imports beside a whole document were not refused at /imports: %+v", report)
	}
	twice := `{"imports": ["data_model/jets_model.jr"], "rules": [` + `{"name": "R", "antecedents": [{"subject": "?x", "predicate": "rdf:type", "object": "jets:Entity"}], ` +
		`"consequents": [{"subject": "?x", "predicate": "jets:completed", "object": true}]}, {"imports": ["data_model/jets_model.jr"]}]}`
	if report := callSymbolic(t, twice); report.Valid || report.Diagnostics[0].Pointer != "/rules/1" {
		t.Errorf("imports given twice were not refused at /rules/1: %+v", report)
	}
	if _, err := CompileRuleFile(context.Background(), fixture(t),
		json.RawMessage(`{"rule_path": "data_model/jets_model.jr", "imports": ["x.jr"]}`)); err == nil {
		t.Error("imports beside rule_path were accepted")
	}
}

// A document the renderer refuses comes back as an invalid report pointing at
// the element, before anything is compiled.
func TestCompileRuleFile_SymbolicRefusalIsLocated(t *testing.T) {
	doc := strings.Replace(symbolicRules, `"object": "?k"`, `"object": "?k) -> (?e jets:x ?y"`, 1)
	report := callSymbolic(t, flatArgs(t, doc))
	if report.Valid || len(report.Diagnostics) != 1 {
		t.Fatalf("expected one refusal, got %+v", report)
	}
	if got := report.Diagnostics[0].Pointer; got != "/rules/0/antecedents/1/object" {
		t.Errorf("pointer %q", got)
	}
	if report.Source != "" {
		t.Errorf("a refused document has no rendering, got:\n%s", report.Source)
	}
}

// A compiler diagnostic raised on a line of the rendering is pointed back at
// the element of the document the line came from. A bool literal outside the
// accepted spellings is one the renderer passes and the compiler refuses, at
// its line (checkBoolLiteral).
func TestCompileRuleFile_SymbolicCompileErrorIsPointed(t *testing.T) {
	doc := strings.Replace(symbolicRules, `"object": "?k"`, `"object": {"type": "bool", "value": "maybe"}`, 1)
	report := callSymbolic(t, flatArgs(t, doc))
	if report.Valid {
		t.Fatalf("expected the compile to fail:\n%s", report.Source)
	}
	var pointed bool
	for _, d := range report.Diagnostics {
		if d.Pointer == "/rules/0/antecedents/1" {
			pointed = true
		}
	}
	if !pointed {
		t.Errorf("no diagnostic points at /rules/0/antecedents/1: %+v", report.Diagnostics)
	}
}

func TestCompileRuleFile_SymbolicIsExclusive(t *testing.T) {
	for _, args := range []string{
		`{"rules": ` + symbolicRules + `, "rule_text": "text x = \"y\";"}`,
		`{"rules": ` + symbolicRules + `, "rule_path": "data_model/jets_model.jr"}`,
	} {
		twice := `{"imports": ["data_model/jets_model.jr"], "rules": [` + `{"name": "R", "antecedents": [{"subject": "?x", "predicate": "rdf:type", "object": "jets:Entity"}], ` +
			`"consequents": [{"subject": "?x", "predicate": "jets:completed", "object": true}]}, {"imports": ["data_model/jets_model.jr"]}]}`
		if report := callSymbolic(t, twice); report.Valid || report.Diagnostics[0].Pointer != "/rules/1" {
			t.Errorf("imports given twice were not refused at /rules/1: %+v", report)
		}
		if _, err := CompileRuleFile(context.Background(), fixture(t), json.RawMessage(args)); err == nil {
			t.Errorf("accepted two arms at once: %s", args)
		}
	}
}
