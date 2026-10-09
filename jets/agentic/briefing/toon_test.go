package briefing

import (
	"strings"
	"testing"
)

// The entity as the live config's channel encoding writes it: toon,
// remove_model_prefixes, one event unwrapped and one array of two.
const toonEntity = `has_Briefing_Medical_Events[2]:
  - Care_Setting: Emergency Room - Hospital
    Diagnosis_Code[2]: F1120,L0390
    Service_Date: 2025-06-10
  - Care_Setting: Independent Laboratory
    Diagnosis_Code: B182
    Service_Date: 2025-08-14
has_Briefing_Pharmacy_Events:
  Maintenance: N
  Medication: traMADol HCl
`

const toonSchema = `{
  "key": "patient_briefing",
  "rules": [
    {"field": "conditions[].code", "kind": "grounded",
     "sources": ["has_Briefing_Medical_Events[].Diagnosis_Code[]"]},
    {"field": "medications[].drug_name", "kind": "grounded",
     "sources": ["has_Briefing_Pharmacy_Events[].Medication[]"]},
    {"field": "medical_event_count", "kind": "count_of",
     "sources": ["has_Briefing_Medical_Events[]"]},
    {"field": "service_date", "kind": "within_span",
     "sources": ["has_Briefing_Medical_Events[].Service_Date[]"]}
  ]
}`

func toonSchemaFor(t *testing.T) *Schema {
	t.Helper()
	s, findings := ParseSchema(toonSchema)
	if s == nil {
		t.Fatalf("schema does not parse: %+v", findings)
	}
	return s
}

func TestCheckTOONGroundsEveryKind(t *testing.T) {
	brief := `{"conditions":[{"code":"B182"},{"code":"L0390"}],
	           "medications":[{"drug_name":"traMADol HCl"}],
	           "medical_event_count":2,
	           "service_date":"2025-07-01"}`
	res, err := CheckTOON(toonSchemaFor(t), toonEntity, brief)
	if err != nil {
		t.Fatalf("CheckTOON: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected no findings, got %s", res.String())
	}
	if len(res.Refs["/conditions/0/code"]) == 0 {
		t.Error("a grounded field carries no source reference")
	}
}

// toon returns numbers as float64 and everything else as text, so a briefing's
// json number and an entity's toon number compare after the same normalisation
// the json arm uses. This is the property the type loss of a round trip could
// have cost and does not.
func TestCheckTOONCountComparesAcrossEncodings(t *testing.T) {
	res, err := CheckTOON(toonSchemaFor(t), toonEntity, `{"medical_event_count":3}`)
	if err != nil {
		t.Fatalf("CheckTOON: %v", err)
	}
	if res.OK() {
		t.Fatal("expected a count finding")
	}
	if res.Findings[0].Code != CodeWrongCount {
		t.Errorf("expected %s, got %s", CodeWrongCount, res.Findings[0].Code)
	}
}

func TestDecodeTOONRefusesADocumentThatIsNotAnEntity(t *testing.T) {
	for _, content := range []string{"- a\n- b\n", "just a string\n"} {
		if _, err := DecodeTOONEntity(content); err == nil {
			t.Errorf("expected a refusal for %q", content)
		}
	}
}

func TestDecodeTOONReportsAMalformedDocument(t *testing.T) {
	// A json document is not a toon document, and a checker that shrugged would
	// report a clean briefing for every record.
	if _, err := DecodeTOONEntity(`{"a": [1, 2,`); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestCheckEncodedDispatchesAndRefusesAnUnknownEncoding(t *testing.T) {
	s := toonSchemaFor(t)
	brief := `{"medical_event_count":2}`
	if _, err := CheckEncoded(s, "toon", toonEntity, brief); err != nil {
		t.Errorf("toon: %v", err)
	}
	jsonEntity := `{"has_Briefing_Medical_Events":[{"Diagnosis_Code":"B182"},{"Diagnosis_Code":"L0390"}]}`
	if _, err := CheckEncoded(s, "", jsonEntity, brief); err != nil {
		t.Errorf("the empty encoding must be json, like ColumnEncodingSpec's own default: %v", err)
	}
	if _, err := CheckEncoded(s, "json", jsonEntity, brief); err != nil {
		t.Errorf("json: %v", err)
	}
	_, err := CheckEncoded(s, "parquet", jsonEntity, brief)
	if err == nil {
		t.Fatal("expected an encoding this checker cannot read to be an error rather than a pass")
	}
	if !strings.Contains(err.Error(), "parquet") {
		t.Errorf("the error should name the encoding, got %v", err)
	}
}

// CheckJSON used to accept a document that decoded to something other than an
// object, because Unmarshal into a map accepts json null and yields an empty
// one. An empty briefing has no findings, so a null briefing read as clean.
func TestCheckJSONRefusesANullDocument(t *testing.T) {
	s := toonSchemaFor(t)
	if _, err := CheckJSON(s, `{"a":1}`, `null`); err == nil {
		t.Error("a null briefing is not an empty briefing")
	}
	if _, err := CheckJSON(s, `null`, `{"a":1}`); err == nil {
		t.Error("a null entity is not an empty entity")
	}
}
