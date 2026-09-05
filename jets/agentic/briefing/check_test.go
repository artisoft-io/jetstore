package briefing

import (
	"testing"
)

// entityJSON is the shape tools/sample_projects/patient_summary_go/prompt.md
// carries, cut to four events and rendered as json. The dates are unpadded
// because that file's are - see dateLayouts - and one property is deliberately
// single-valued and unwrapped, because addToEntityObj serialises it that way.
const entityJSON = `{
  "Disclaimer": "Informational only. Not medical advice.",
  "Adherence_Ratio": 0.82,
  "has_Medical_Events": [
    {"Date_From": "2025-1-17", "Diagnosis": "(I330) Acute and subacute infective endocarditis",
     "Place_Of_Service": "Inpatient Hospital"},
    {"Date_From": "2025-2-12", "Diagnosis": "(I330) Acute and subacute infective endocarditis",
     "Place_Of_Service": "Home"},
    {"Date_From": "2025-8-14", "Diagnosis": "(B182) Chronic viral hepatitis C",
     "Place_Of_Service": "Independent Laboratory"},
    {"Date_From": "2025-11-13", "Diagnosis": "(F1120) Opioid dependence, uncomplicated",
     "Place_Of_Service": "Office"}
  ],
  "has_Pharmacy_Events": {"Fill_Date": "2025-9-15", "Drug": "Buprenorphine"}
}`

const briefingSchemaJSON = `{
  "key": "call_centre_briefing",
  "version": "0.1.0",
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "substring",
     "sources": ["has_Medical_Events[].Diagnosis"]},
    {"field": "last_visit", "kind": "within_span",
     "sources": ["has_Medical_Events[].Date_From"]},
    {"field": "visit_count", "kind": "count_of", "sources": ["has_Medical_Events[]"]},
    {"field": "disclaimer", "kind": "derived", "sources": ["Disclaimer"]},
    {"field": "last_fill", "kind": "grounded", "sources": ["has_Pharmacy_Events[].Drug"]}
  ]
}`

func schemaFor(t *testing.T, doc string) *Schema {
	t.Helper()
	s, findings := ParseSchema(doc)
	if s == nil {
		t.Fatalf("schema did not compile: %v", codesOf(findings))
	}
	return s
}

func check(t *testing.T, brief string) *Result {
	t.Helper()
	res, err := CheckJSON(schemaFor(t, briefingSchemaJSON), entityJSON, brief)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return res
}

func findingCodes(r *Result) []string {
	out := make([]string, 0, len(r.Findings))
	for _, f := range r.Findings {
		out = append(out, f.Code)
	}
	return out
}

func TestGroundedBriefingPasses(t *testing.T) {
	res := check(t, `{
	  "conditions": [{"code": "I330"}, {"code": "B182"}],
	  "last_visit": "2025-08-14",
	  "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.",
	  "last_fill": "Buprenorphine"
	}`)
	if !res.OK() {
		t.Fatalf("a grounded briefing was refused: %v", res.String())
	}
	// A§8.3 asks for a source reference per populated field. It is computed, so
	// there is one for every field a grounding rule passed.
	for _, ptr := range []string{"/conditions/0/code", "/conditions/1/code", "/last_visit",
		"/visit_count", "/disclaimer", "/last_fill"} {
		if len(res.Refs[ptr]) == 0 {
			t.Errorf("no source reference resolved for %s", ptr)
		}
	}
	if got := res.Refs["/conditions/0/code"]; len(got) != 2 {
		// I330 appears in two events, and both are the answer to "where did this
		// come from".
		t.Errorf("/conditions/0/code resolved to %v, want both I330 events", got)
	}
	if got := res.Refs["/last_fill"]; len(got) != 1 || got[0] != "/has_Pharmacy_Events/Drug" {
		t.Errorf("the unwrapped single-valued property resolved to %v", got)
	}
}

// The headline case: a code the model invented. Phase 4 measured 20 of 29
// hypotheses sitting at loci triage did not find present, which is this failure
// in the diagnostic half of the project.
func TestInventedCodeIsRefused(t *testing.T) {
	res := check(t, `{
	  "conditions": [{"code": "E119"}],
	  "last_visit": "2025-08-14",
	  "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.",
	  "last_fill": "Buprenorphine"
	}`)
	if res.OK() {
		t.Fatal("an invented diagnosis code was accepted")
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeUngrounded {
		t.Fatalf("findings %v", findingCodes(res))
	}
	f := res.Findings[0]
	if f.Path != "/conditions/0/code" {
		t.Errorf("the finding points at %q", f.Path)
	}
	if len(f.Sources) != 1 || f.Sources[0] != "has_Medical_Events[].Diagnosis" {
		t.Errorf("the finding does not say what was consulted: %v", f.Sources)
	}
}

func TestDateOutsideTheObservedSpanIsRefused(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "2026-03-01", "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.", "last_fill": "Buprenorphine"
	}`)
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeOutOfSpan {
		t.Fatalf("findings %v (%s)", findingCodes(res), res.String())
	}
}

func TestDateInsideTheSpanPasses(t *testing.T) {
	// The span is 2025-1-17 to 2025-11-13, written unpadded in the entity and
	// padded in the briefing. Both parse.
	res := check(t, `{
	  "conditions": [], "last_visit": "2025-01-17", "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.", "last_fill": "Buprenorphine"
	}`)
	if !res.OK() {
		t.Fatalf("%s", res.String())
	}
}

func TestWrongCountIsRefused(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "2025-08-14", "visit_count": 7,
	  "disclaimer": "Informational only. Not medical advice.", "last_fill": "Buprenorphine"
	}`)
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeWrongCount {
		t.Fatalf("findings %v (%s)", findingCodes(res), res.String())
	}
}

// A§8.3 wants the disclaimer populated deterministically rather than by the
// model. Nothing but a comparison establishes that it was.
func TestRewrittenDisclaimerIsRefused(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "2025-08-14", "visit_count": 4,
	  "disclaimer": "This summary is accurate and may be relied upon.",
	  "last_fill": "Buprenorphine"
	}`)
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeNotDerived {
		t.Fatalf("findings %v (%s)", findingCodes(res), res.String())
	}
}

// The closure, and the property that makes this checker usable before the
// projection exists: a field nobody declared is refused rather than ignored.
func TestUndeclaredFieldIsRefused(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "2025-08-14", "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.",
	  "last_fill": "Buprenorphine",
	  "recommended_action": "Advise the member to see a cardiologist."
	}`)
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeNoRule {
		t.Fatalf("findings %v (%s)", findingCodes(res), res.String())
	}
	if res.Findings[0].Path != "/recommended_action" {
		t.Errorf("the finding points at %q", res.Findings[0].Path)
	}
}

func TestNullAndEmptyAssertNothing(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "2025-08-14", "visit_count": 4,
	  "disclaimer": "Informational only. Not medical advice.",
	  "last_fill": "Buprenorphine",
	  "notes": null
	}`)
	if !res.OK() {
		t.Fatalf("an explicit null was treated as an assertion: %s", res.String())
	}
}

func TestUngroundedExemptionIsHonoured(t *testing.T) {
	s := schemaFor(t, `{"key":"k","rules":[
	  {"field":"greeting","kind":"ungrounded","reason":"a fixed salutation asserts nothing about the member"}]}`)
	res, err := CheckJSON(s, entityJSON, `{"greeting": "Good morning"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK() {
		t.Fatalf("%s", res.String())
	}
	if len(res.Refs) != 0 {
		t.Errorf("an exempt field resolved a source reference: %v", res.Refs)
	}
}

func TestExactMatchIsTheDefaultAndIsStricter(t *testing.T) {
	// The same value under the same source: substring grounds it, exact does not.
	brief := `{"c": "B182"}`
	sub := schemaFor(t, `{"key":"k","rules":[{"field":"c","kind":"grounded","match":"substring",
	  "sources":["has_Medical_Events[].Diagnosis"]}]}`)
	exact := schemaFor(t, `{"key":"k","rules":[{"field":"c","kind":"grounded",
	  "sources":["has_Medical_Events[].Diagnosis"]}]}`)
	if r, _ := CheckJSON(sub, entityJSON, brief); !r.OK() {
		t.Errorf("substring refused a code that is in the input: %s", r.String())
	}
	if r, _ := CheckJSON(exact, entityJSON, brief); r.OK() {
		t.Error("exact accepted a value that is only part of an entity value")
	}
}

func TestUnreadableValuesAreRefusedRatherThanSkipped(t *testing.T) {
	res := check(t, `{
	  "conditions": [], "last_visit": "sometime last summer", "visit_count": "several",
	  "disclaimer": "Informational only. Not medical advice.", "last_fill": "Buprenorphine"
	}`)
	got := findingCodes(res)
	if len(got) != 2 {
		t.Fatalf("findings %v (%s)", got, res.String())
	}
	want := map[string]bool{CodeUnparseableDate: true, CodeNotANumber: true}
	for _, c := range got {
		if !want[c] {
			t.Errorf("unexpected finding %s", c)
		}
	}
}

// An empty span admits every date, so it is reported rather than passed.
func TestEmptySpanIsReported(t *testing.T) {
	s := schemaFor(t, `{"key":"k","rules":[
	  {"field":"d","kind":"within_span","sources":["has_Dental_Events[].Date_From"]}]}`)
	res, err := CheckJSON(s, entityJSON, `{"d": "2025-05-01"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != CodeNoSpan {
		t.Fatalf("findings %v", findingCodes(res))
	}
}

func TestFindingOrderIsStable(t *testing.T) {
	brief := `{
	  "conditions": [{"code": "E119"}, {"code": "Z993"}],
	  "last_visit": "2026-03-01", "visit_count": 9,
	  "disclaimer": "Informational only. Not medical advice.",
	  "last_fill": "Buprenorphine", "extra": "x"
	}`
	var first []string
	for i := 0; i < 20; i++ {
		got := findingCodes(check(t, brief))
		if i == 0 {
			first = got
			continue
		}
		if len(got) != len(first) {
			t.Fatalf("run %d: %v vs %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d differs at %d: %v vs %v", i, j, got, first)
			}
		}
	}
	if len(first) != 5 {
		t.Fatalf("expected five findings, got %v", first)
	}
}

func TestCheckJSONRefusesUnreadableInput(t *testing.T) {
	s := schemaFor(t, briefingSchemaJSON)
	if _, err := CheckJSON(s, `{`, `{}`); err == nil {
		t.Error("an unreadable entity was accepted")
	}
	if _, err := CheckJSON(s, entityJSON, `not json`); err == nil {
		t.Error("an unreadable briefing was accepted")
	}
}

// The empty briefing is the degenerate case worth pinning: nothing is asserted,
// so nothing is unsupported. It is not evidence that the check ran.
func TestEmptyBriefingHasNoFindings(t *testing.T) {
	res := check(t, `{}`)
	if !res.OK() {
		t.Fatalf("%s", res.String())
	}
	if res.String() != "briefing provenance: no findings" {
		t.Errorf("String() is %q", res.String())
	}
}
