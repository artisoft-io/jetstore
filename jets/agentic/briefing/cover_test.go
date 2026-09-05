package briefing

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

func coverFindings(t *testing.T, doc string) []wsvalidate.Finding {
	t.Helper()
	_, findings := ParseSchema(doc)
	return findings
}

const coveredDoc = `{
  "key": "b",
  "response_format": {
    "type": "object", "additionalProperties": false,
    "properties": {
      "conditions": {"type": "array", "items": {"type": "object", "additionalProperties": false,
        "properties": {"code": {"type": "string"}}}},
      "count": {"type": "integer"}
    }
  },
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "sources": ["e[].c[]"]},
    {"field": "count", "kind": "count_of", "sources": ["e[]"]}
  ]
}`

func TestCoverAcceptsATotalSchema(t *testing.T) {
	s, findings := ParseSchema(coveredDoc)
	if s == nil {
		t.Fatalf("expected the document to parse: %+v", findings)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

// The half that makes criterion 52 a property of the briefing rather than of a
// run: a field the response_format can produce and no rule covers is refused
// when the document is saved, not when a model happens to populate it.
func TestCoverRefusesAFieldWithNoRule(t *testing.T) {
	doc := strings.Replace(coveredDoc,
		`"count": {"type": "integer"}`,
		`"count": {"type": "integer"}, "note": {"type": "string"}`, 1)
	findings := coverFindings(t, doc)
	if !hasCode(findings, CodeResponseFieldHasNoRule) {
		t.Fatalf("expected %s, got %+v", CodeResponseFieldHasNoRule, findings)
	}
	if !strings.Contains(findings[0].Message, "note") {
		t.Errorf("the finding should name the field, got %q", findings[0].Message)
	}
}

// The other half, and it is not symmetry for its own sake: a rule over a field
// the briefing cannot carry is a guardrail that has stopped firing and says
// nothing about having stopped.
func TestCoverRefusesARuleThatCanNeverFire(t *testing.T) {
	doc := strings.Replace(coveredDoc,
		`{"field": "count", "kind": "count_of", "sources": ["e[]"]}`,
		`{"field": "count", "kind": "count_of", "sources": ["e[]"]},
		 {"field": "conditions[].severity", "kind": "grounded", "sources": ["e[].s[]"]}`, 1)
	findings := coverFindings(t, doc)
	if !hasCode(findings, CodeRuleFieldNotInResponse) {
		t.Fatalf("expected %s, got %+v", CodeRuleFieldNotInResponse, findings)
	}
}

func TestCoverRefusesAnOpenObject(t *testing.T) {
	doc := strings.Replace(coveredDoc, `"type": "object", "additionalProperties": false,
    "properties": {
      "conditions"`, `"type": "object",
    "properties": {
      "conditions"`, 1)
	findings := coverFindings(t, doc)
	if !hasCode(findings, CodeResponseOpen) {
		t.Fatalf("expected %s, got %+v", CodeResponseOpen, findings)
	}
}

func TestCoverRefusesConstructsItCannotEnumerate(t *testing.T) {
	cases := map[string]string{
		"oneOf":       `{"oneOf": [{"type": "string"}]}`,
		"ref":         `{"$ref": "#/$defs/x"}`,
		"no type":     `{"description": "a field"}`,
		"open object": `{"type": "object", "properties": {"x": {"type": "string"}}}`,
		"bare array":  `{"type": "array"}`,
	}
	for name, leaf := range cases {
		t.Run(name, func(t *testing.T) {
			doc := strings.Replace(coveredDoc, `"count": {"type": "integer"}`,
				`"count": `+leaf, 1)
			findings := coverFindings(t, doc)
			if len(wsvalidate.ErrorsOnly(findings)) == 0 {
				t.Fatalf("expected a refusal, got %+v", findings)
			}
		})
	}
}

func TestCoverRefusesAnArrayOfArrays(t *testing.T) {
	doc := strings.Replace(coveredDoc, `"count": {"type": "integer"}`,
		`"count": {"type": "array", "items": {"type": "array", "items": {"type": "string"}}}`, 1)
	findings := coverFindings(t, doc)
	if !hasCode(findings, CodeResponseUnsupported) {
		t.Fatalf("expected %s, got %+v", CodeResponseUnsupported, findings)
	}
}

// A schema with no response_format is what AK.1 shipped and it keeps working:
// the runtime closure is the only totality it has, which is exactly the state
// this function improves on rather than replaces.
func TestCoverIsSilentWithoutAResponseFormat(t *testing.T) {
	s, findings := ParseSchema(`{"key":"b","rules":[
	  {"field":"a","kind":"grounded","sources":["e[]"]}]}`)
	if s == nil {
		t.Fatalf("expected the document to parse: %+v", findings)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestValidateProvenanceDocumentIsTheSaveTimeCheck(t *testing.T) {
	if findings := ValidateProvenanceDocument(coveredDoc); len(findings) != 0 {
		t.Fatalf("expected a clean document, got %+v", findings)
	}
	doc := strings.Replace(coveredDoc, `"count": {"type": "integer"}`,
		`"count": {"type": "integer"}, "note": {"type": "string"}`, 1)
	if len(wsvalidate.ErrorsOnly(ValidateProvenanceDocument(doc))) == 0 {
		t.Fatal("the save path must refuse a briefing field with no rule")
	}
	if DocumentSuffix != ".pv.json" {
		t.Errorf("the suffix is the validator table's key: %q", DocumentSuffix)
	}
}

func TestSelectorCanonicalIsWhatCoverCompares(t *testing.T) {
	for raw, want := range map[string]string{
		"a":            "a",
		"a[]":          "a[]",
		"a[].b":        "a[].b",
		"a[].b[].c":    "a[].b[].c",
		"conditions[]": "conditions[]",
	} {
		sel, err := ParseSelector(raw)
		if err != nil {
			t.Fatalf("ParseSelector(%q): %v", raw, err)
		}
		if got := sel.Canonical(); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", raw, got, want)
		}
	}
}
