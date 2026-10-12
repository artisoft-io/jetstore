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

func TestCompileRuleFile_SymbolicRulesCompile(t *testing.T) {
	report := callSymbolic(t, `{"rules": `+symbolicRules+`}`)
	if !report.Valid {
		t.Fatalf("expected a clean compile, got %+v", report.Diagnostics)
	}
	if !strings.Contains(report.Source, "[SymbolicArm1, s=50]:") ||
		!strings.Contains(report.Source, `import "data_model/jets_model.jr"`) {
		t.Errorf("the report does not carry the rendered source:\n%s", report.Source)
	}
}

// A model sends a JSON document as a string holding it often enough that the
// two must mean the same thing.
func TestCompileRuleFile_SymbolicRulesAsAString(t *testing.T) {
	quoted, _ := json.Marshal(symbolicRules)
	report := callSymbolic(t, `{"rules": `+string(quoted)+`}`)
	if !report.Valid {
		t.Fatalf("expected a clean compile, got %+v", report.Diagnostics)
	}
}

// A document the renderer refuses comes back as an invalid report pointing at
// the element, before anything is compiled.
func TestCompileRuleFile_SymbolicRefusalIsLocated(t *testing.T) {
	doc := strings.Replace(symbolicRules, `"object": "?k"`, `"object": "?k) -> (?e jets:x ?y"`, 1)
	report := callSymbolic(t, `{"rules": `+doc+`}`)
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
	report := callSymbolic(t, `{"rules": `+doc+`}`)
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
		if _, err := CompileRuleFile(context.Background(), fixture(t), json.RawMessage(args)); err == nil {
			t.Errorf("accepted two arms at once: %s", args)
		}
	}
}
