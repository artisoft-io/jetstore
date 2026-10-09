package template

import (
	"encoding/json"
	"strings"
	"testing"
)

// **This is §10.9's document with every domain word replaced, and it is here
// for one reason: the worked example is what `AY.3` has to be able to write.**
// Nothing about the engine is demonstrated by this that the tests beside it do
// not also cover piece by piece; what it demonstrates is that the pieces
// compose into the shape the notation was specified from - a notice carrying a
// `require`, a rule that is a literal, a group with a fallback, a three-term
// predicate with a parenthesis, a `with` that supplies its own guard, and an
// `each` whose body nests three conditionals.
//
// It also fixes the whitespace question a reader of §10.9 cannot answer from
// the page. **A line break in a `text` value is a space**, and the wrap
// collapses runs - so the only decision at a markup boundary is whether there
// is a space there at all, and this document makes each one deliberately. A
// template laid out over several lines for readability gets a space wherever it
// was broken, which is why `on record{{if ...}},` is written closed up.
const workedDocument = `{
  "key": "catalogue",
  "width": 78,
  "elements": [
    { "text": "{{Notice require 'the record carries no Notice; the artefact is notice, rule, prose in that order'}}" },
    { "text": "--------------------------------------------------------------------" },
    { "empty": "No activity on record for this catalogue.",
      "elements": [
        { "when": "Order_Count > 0",
          "text": "{{Order_Count|words|sentence}} {{Order_Count|plural: 'order'}} on record{{if Earliest_Date|d_MMMM != '' and Latest_Date|d_MMMM_yyyy != ''}}, between {{Earliest_Date|d_MMMM}} and {{Latest_Date|d_MMMM_yyyy}}{{end}}. {{with has_Orders[]|latest_by: Placed_Date}}Most recent: {{if Channel != ''}}{{Channel}}, {{end}}{{Placed_Date|d_MMMM}}.{{end}}" },
        { "when": "(Order_Count > 0 or Item_Count > 0) and Tag_Summary[]|count > 0",
          "text": "Tags on record: {{Tag_Summary[]|strip_code|join: ', ', ' and '}}." },
        { "when": "has_Items[]|count > 0",
          "text": "Items: {{each has_Items[] sep: '; ' last: '; and '}}{{Item_Name require 'an item carries no Item_Name'}}{{if Featured == 'Y'}}, a featured item{{end}}{{if Unit_Count != ''}}, {{Unit_Count|words}} {{Unit_Count|plural: 'unit'}}{{if Ship_Date[]|max|d_MMMM != ''}}, most recently {{Ship_Date[]|max|d_MMMM}}{{end}}{{end}}{{end}}." }
      ] }
  ]
}`

// workedRecord is decoded from json rather than written as a Go literal, so
// that every number is a float64 and every date is text - which is one of the
// two shapes a record arrives in, and the one a hand-written fixture would
// silently avoid.
const workedRecord = `{
  "Notice": "Informational only. Prepared from catalogue records for a reader who is not a specialist, and not to be relied on for anything else.",
  "Order_Count": 2,
  "Item_Count": 2,
  "Earliest_Date": "2025-06-10",
  "Latest_Date": "2025-08-14",
  "has_Orders": [
    { "Placed_Date": "2025-06-10", "Channel": "Post" },
    { "Placed_Date": "2025-08-14", "Channel": "Counter" }
  ],
  "Tag_Summary": ["(A1) Alpha", "(B2) Beta", "Gamma"],
  "has_Items": [
    { "Item_Name": "widget", "Featured": "Y", "Unit_Count": 3,
      "Ship_Date": ["2025-08-01", "2025-08-24"] },
    { "Item_Name": "gadget", "Unit_Count": 1, "Ship_Date": "2025-07-02" }
  ]
}`

func TestTheWorkedDocument(t *testing.T) {
	tmpl, err := CompileJSON([]byte(workedDocument))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if tmpl.Key() != "catalogue" || tmpl.Width() != 78 {
		t.Fatalf("key %q width %d", tmpl.Key(), tmpl.Width())
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(workedRecord), &doc); err != nil {
		t.Fatalf("record: %v", err)
	}
	got, violations := tmpl.Render(doc)
	if len(violations) != 0 {
		t.Fatalf("violations: %v", violations)
	}
	want := strings.Join([]string{
		"Informational only. Prepared from catalogue records for a reader who is not a",
		"specialist, and not to be relied on for anything else.",
		"",
		"--------------------------------------------------------------------",
		"",
		"Two orders on record, between 10 June and 14 August 2025. Most recent:",
		"Counter, 14 August.",
		"",
		"Tags on record: Alpha, Beta and Gamma.",
		"",
		"Items: widget, a featured item, three units, most recently 24 August; and",
		"gadget, one unit, most recently 2 July.",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
	}
}

// TestTheWorkedDocumentOverAnEmptyRecord is the fallback firing under a preamble
// that emits regardless, which is the arrangement a flat element list cannot
// express (§10.6).
func TestTheWorkedDocumentOverAnEmptyRecord(t *testing.T) {
	tmpl, err := CompileJSON([]byte(workedDocument))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got, violations := tmpl.Render(map[string]any{"Notice": "Informational only."})
	if len(violations) != 0 {
		t.Fatalf("violations: %v", violations)
	}
	want := "Informational only.\n\n" +
		"--------------------------------------------------------------------\n\n" +
		"No activity on record for this catalogue."
	if got != want {
		t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
	}
}

// TestTheWorkedDocumentReportsWhatIsMissing is the third refusal of the
// artefact this notation generalises, expressed in the template rather than in
// the engine - and the engine has no idea what a notice is.
func TestTheWorkedDocumentReportsWhatIsMissing(t *testing.T) {
	tmpl, err := CompileJSON([]byte(workedDocument))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	_, violations := tmpl.Render(map[string]any{"Order_Count": 1.0})
	if len(violations) != 1 || !strings.Contains(violations[0].Message, "carries no Notice") {
		t.Fatalf("violations: %v", violations)
	}
}

// TestNothingHereNamesADomain is criterion 81 asserted rather than claimed.
// **It is a weak test and is worth having anyway**: it cannot know what a
// domain word is, so it checks the four names plan §10 itself uses as the line
// between the notation and a briefing. A fifth domain word would pass it.
func TestNothingHereNamesADomain(t *testing.T) {
	for _, word := range []string{"cintel", "briefing", "medication", "member"} {
		for _, src := range []string{workedDocument, workedRecord} {
			if strings.Contains(strings.ToLower(src), word) {
				t.Errorf("a fixture names %q", word)
			}
		}
	}
}
