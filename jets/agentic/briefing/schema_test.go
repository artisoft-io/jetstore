package briefing

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

const goodSchema = `{
  "key": "call_centre_briefing",
  "version": "0.1.0",
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "substring",
     "sources": ["has_Medical_Events[].Diagnosis"]},
    {"field": "last_visit", "kind": "within_span",
     "sources": ["has_Medical_Events[].Date_From"]},
    {"field": "visit_count", "kind": "count_of",
     "sources": ["has_Medical_Events[]"]},
    {"field": "disclaimer", "kind": "derived", "sources": ["Disclaimer"]},
    {"field": "greeting", "kind": "ungrounded",
     "reason": "a fixed salutation asserts nothing about the member"}
  ]
}`

func codesOf(fs []wsvalidate.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Code)
	}
	return out
}

func hasCode(fs []wsvalidate.Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestParseSchemaGood(t *testing.T) {
	s, findings := ParseSchema(goodSchema)
	if len(findings) != 0 {
		t.Fatalf("a good schema reported %v", codesOf(findings))
	}
	if s == nil || s.Key != "call_centre_briefing" || len(s.Rules) != 5 {
		t.Fatalf("parsed schema is %+v", s)
	}
}

func TestParseSchemaRefusals(t *testing.T) {
	cases := []struct {
		name, doc, code string
	}{
		{"not json", `{`, "briefing_schema_not_readable"},
		{"unknown field", `{"key":"k","rules":[],"kinds":[]}`, "briefing_schema_not_readable"},
		{"no key", `{"rules":[{"field":"a","kind":"ungrounded","reason":"r"}]}`, "briefing_schema_has_no_key"},
		{"no rules", `{"key":"k","rules":[]}`, "briefing_schema_has_no_rules"},
		{"no kind", `{"key":"k","rules":[{"field":"a"}]}`, "briefing_schema_rule_has_no_kind"},
		{"unknown kind", `{"key":"k","rules":[{"field":"a","kind":"vibes"}]}`, "briefing_schema_unknown_kind"},
		{"grounded with no source", `{"key":"k","rules":[{"field":"a","kind":"grounded"}]}`,
			"briefing_schema_rule_has_no_source"},
		{"derived with two sources",
			`{"key":"k","rules":[{"field":"a","kind":"derived","sources":["x","y"]}]}`,
			"briefing_schema_rule_has_no_source"},
		{"exemption with no reason", `{"key":"k","rules":[{"field":"a","kind":"ungrounded"}]}`,
			"briefing_schema_exemption_has_no_reason"},
		{"exemption with sources",
			`{"key":"k","rules":[{"field":"a","kind":"ungrounded","reason":"r","sources":["x"]}]}`,
			"briefing_schema_exemption_has_sources"},
		{"reason on a checked rule",
			`{"key":"k","rules":[{"field":"a","kind":"grounded","sources":["x"],"reason":"r"}]}`,
			"briefing_schema_reason_on_grounded_rule"},
		{"unknown match",
			`{"key":"k","rules":[{"field":"a","kind":"grounded","sources":["x"],"match":"fuzzy"}]}`,
			"briefing_schema_unknown_match"},
		{"match on the wrong kind",
			`{"key":"k","rules":[{"field":"a","kind":"count_of","sources":["x"],"match":"exact"}]}`,
			"briefing_schema_match_on_wrong_kind"},
		{"duplicate field",
			`{"key":"k","rules":[{"field":"a","kind":"grounded","sources":["x"]},
			  {"field":"a","kind":"grounded","sources":["y"]}]}`,
			"briefing_schema_duplicate_field"},
		{"bad field selector",
			`{"key":"k","rules":[{"field":"a.0.b","kind":"grounded","sources":["x"]}]}`,
			"briefing_schema_bad_field_selector"},
		{"bad source selector",
			`{"key":"k","rules":[{"field":"a","kind":"grounded","sources":["x.0.y"]}]}`,
			"briefing_schema_bad_source_selector"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, findings := ParseSchema(tc.doc)
			if s != nil {
				t.Errorf("a schema with an error was returned for use anyway")
			}
			if !hasCode(findings, tc.code) {
				t.Fatalf("want %s, got %v", tc.code, codesOf(findings))
			}
		})
	}
}

// The findings carry a JSON Pointer into the schema, because a message is not
// something an editor can jump to (wsvalidate's own argument, from this side).
func TestSchemaFindingsCarryAPointer(t *testing.T) {
	_, findings := ParseSchema(`{"key":"k","rules":[
	  {"field":"a","kind":"grounded","sources":["x"]},
	  {"field":"b","kind":"ungrounded"}]}`)
	for _, f := range findings {
		if f.Code == "briefing_schema_exemption_has_no_reason" && f.Path != "/rules/1/reason" {
			t.Errorf("pointer is %q, want /rules/1/reason", f.Path)
		}
	}
}

func TestCompileIsIdempotent(t *testing.T) {
	s, _ := ParseSchema(goodSchema)
	if f := s.Compile(); len(f) != 0 {
		t.Fatalf("second compile reported %v", codesOf(f))
	}
	if f := s.Compile(); len(f) != 0 {
		t.Fatalf("third compile reported %v", codesOf(f))
	}
	// The parsed selectors survive, so a re-compile does not silently empty the
	// rule set it is about to check with.
	if got := len(s.Rules[0].sources); got != 1 {
		t.Fatalf("sources after two compiles: %d", got)
	}
}

func TestCheckRefusesAnUncompilableSchema(t *testing.T) {
	s := &Schema{Key: "k", Rules: []FieldRule{{Field: "a", Kind: "vibes"}}}
	if _, err := Check(s, map[string]any{}, map[string]any{"a": 1}); err == nil {
		t.Fatal("Check accepted a schema that does not compile")
	} else if !strings.Contains(err.Error(), "does not compile") {
		t.Errorf("error is %v", err)
	}
	if _, err := Check(nil, map[string]any{}, map[string]any{}); err == nil {
		t.Fatal("Check accepted a nil schema")
	}
}
