package jetrules_go_adaptor

import (
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// The end-to-end check for the briefing projection (agentic_ai AK.2), and it lives
// here because this is where the real rdf session meets the real encoder.
//
// **Nothing between the entity and the guardrail is stubbed.** A projected
// cintel:Briefing is asserted as triples into an rdf.RdfSession, serialised by
// `EncodeColumnData` (`EncodeColumnData`,
// `jets/compute_pipes/jetrules_extract_entity.go:13`) with the settings
// `workspaces/jets_ws/pipes_config/patient_profile.pc.json` uses - toon,
// remove_model_prefixes, the exclusions - and the string that comes out is what
// `briefing.CheckTOON` is given. A test that hand-wrote the toon would agree with
// the hand that wrote it, which is the failure phase 3's U.2 recorded as I-147:
// four validation layers and a green suite passed a call that could not work,
// because the stub was shaped like the caller.
//
// What it does not exercise is the *rules* that build the projection. Those are
// `workspaces/jets_ws/jet_rules/clinical_intel/briefing_projection.jr` and they
// need a rete session over a compiled workspace, which is a deployment rather
// than a test. What is verified about them is that they compile.

func newSession(t *testing.T) (*rdf.RdfSession, *rdf.ResourceManager) {
	t.Helper()
	// NewRdfSession makes its own child manager and locks the root, so the
	// session's manager is the one a test may still add resources to.
	s := rdf.NewRdfSession(rdf.NewResourceManager(nil), rdf.NewRdfGraph("meta"))
	return s, s.ResourceMgr
}

func insert(t *testing.T, s *rdf.RdfSession, subject, predicate string, object *rdf.Node, rm *rdf.ResourceManager) {
	t.Helper()
	if _, err := s.Insert(rm.NewResource(subject), rm.NewResource(predicate), object); err != nil {
		t.Fatalf("insert (%s, %s): %v", subject, predicate, err)
	}
}

// projectedBriefing asserts the shape briefing_projection.jr produces: one
// cintel:Briefing with two medical events and one pharmacy event, and the member
// id that the channel's exclude_properties keeps out of the prompt.
func projectedBriefing(t *testing.T) (*rdf.RdfSession, *rdf.ResourceManager) {
	t.Helper()
	s, rm := newSession(t)
	ins := func(subj, pred string, obj *rdf.Node) { insert(t, s, subj, pred, obj, rm) }
	date := func(y, m, d int) *rdf.Node {
		v := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		return rm.NewDateLiteral(rdf.LDate{Date: &v})
	}

	ins("briefing", "rdf:type", rm.NewResource("cintel:Briefing"))
	ins("briefing", "jets:key", rm.NewTextLiteral("k-1"))
	ins("briefing", "cintel:Briefing_Member_ID", rm.NewTextLiteral("M-4471"))
	// A§8.3's intended-use notice, asserted by BP_Disclaimer10 with no condition
	// beyond the briefing existing. It is on the record and out of the prompt,
	// which is the second half of what the exclusions here are doing.
	ins("briefing", "cintel:Briefing_Disclaimer", rm.NewTextLiteral(shippedNotice))

	ins("briefing", "cintel:has_Briefing_Medical_Events", rm.NewResource("event1"))
	ins("event1", "rdf:type", rm.NewResource("cintel:Briefing_Medical_Event"))
	ins("event1", "cintel:Service_Date", date(2025, 8, 14))
	ins("event1", "cintel:Diagnosis_Code", rm.NewTextLiteral("B182"))
	ins("event1", "cintel:Diagnosis_Name", rm.NewTextLiteral("Chronic viral hepatitis C"))
	ins("event1", "cintel:Condition", rm.NewTextLiteral("Liver disease, mild"))
	ins("event1", "cintel:Care_Setting", rm.NewTextLiteral("Independent Laboratory"))

	// Two codes on one event, which is what makes the multi-valued case real.
	ins("briefing", "cintel:has_Briefing_Medical_Events", rm.NewResource("event2"))
	ins("event2", "rdf:type", rm.NewResource("cintel:Briefing_Medical_Event"))
	ins("event2", "cintel:Service_Date", date(2025, 6, 10))
	ins("event2", "cintel:Diagnosis_Code", rm.NewTextLiteral("L0390"))
	ins("event2", "cintel:Diagnosis_Code", rm.NewTextLiteral("F1120"))
	ins("event2", "cintel:Diagnosis_Name", rm.NewTextLiteral("Cellulitis, unspecified"))
	ins("event2", "cintel:Diagnosis_Name", rm.NewTextLiteral("Opioid dependence, uncomplicated"))
	ins("event2", "cintel:Care_Setting", rm.NewTextLiteral("Emergency Room - Hospital"))

	// One pharmacy event, so the F516 case is in the fixture rather than in a
	// separate test: a multi-valued property with one value serialises unwrapped.
	ins("briefing", "cintel:has_Briefing_Pharmacy_Events", rm.NewResource("fill1"))
	ins("fill1", "rdf:type", rm.NewResource("cintel:Briefing_Pharmacy_Event"))
	ins("fill1", "cintel:Fill_Date", date(2025, 6, 10))
	ins("fill1", "cintel:Medication", rm.NewTextLiteral("traMADol HCl"))
	ins("fill1", "cintel:Adherence", rm.NewDoubleLiteral(0.9090909090909091))
	ins("fill1", "cintel:Maintenance", rm.NewTextLiteral("N"))
	return s, rm
}

// encodeAsPipeline runs the real encoder with patient_profile.pc.json's settings.
func encodeAsPipeline(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager, encoding string) string {
	t.Helper()
	ce := &compute_pipes.JrSpecialColumnEncoding{
		Config: &compute_pipes.ColumnEncodingSpec{
			Column:              "cintel:Briefing_Input",
			EntityEncoding:      encoding,
			RemoveModelPrefixes: true,
		},
		ExcludeProperties: map[string]bool{
			"jets:key":                    true,
			"rdf:type":                    true,
			"jets:source_period_sequence": true,
			"cintel:Briefing_Member_ID":   true,
			"cintel:Briefing_Disclaimer":  true,
		},
	}
	out := ce.EncodeColumnData(&JetRdfSessionGo{rdfSession: s}, &RdfNodeGo{node: rm.NewResource("briefing")})
	text, ok := out.(string)
	if !ok {
		t.Fatalf("EncodeColumnData returned %T: %v", out, out)
	}
	return text
}

// briefingSchema is the provenance contract of
// workspaces/jets_ws/provenance/patient_briefing.pv.json, as a document.
const briefingSchema = `{
  "key": "patient_briefing",
  "version": "1.1.0",
  "disclaimer": {"field": "cintel:Briefing_Disclaimer"},
  "response_format": {
    "type": "object",
    "additionalProperties": false,
    "properties": {
      "conditions": {"type": "array", "items": {"type": "object", "additionalProperties": false,
        "properties": {"code": {"type": "string"}, "name": {"type": "string"}}}},
      "medications": {"type": "array", "items": {"type": "object", "additionalProperties": false,
        "properties": {"drug_name": {"type": "string"}, "maintenance": {"type": "string"}}}},
      "medical_event_count": {"type": "integer"},
      "pharmacy_event_count": {"type": "integer"},
      "earliest_service_date": {"type": "string"},
      "latest_service_date": {"type": "string"}
    }
  },
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "exact",
     "sources": ["has_Briefing_Medical_Events[].Diagnosis_Code[]"]},
    {"field": "conditions[].name", "kind": "grounded", "match": "exact",
     "sources": ["has_Briefing_Medical_Events[].Diagnosis_Name[]",
                 "has_Briefing_Medical_Events[].Condition[]"]},
    {"field": "medications[].drug_name", "kind": "grounded", "match": "exact",
     "sources": ["has_Briefing_Pharmacy_Events[].Medication[]"]},
    {"field": "medications[].maintenance", "kind": "grounded", "match": "exact",
     "sources": ["has_Briefing_Pharmacy_Events[].Maintenance[]"]},
    {"field": "medical_event_count", "kind": "count_of",
     "sources": ["has_Briefing_Medical_Events[]"]},
    {"field": "pharmacy_event_count", "kind": "count_of",
     "sources": ["has_Briefing_Pharmacy_Events[]"]},
    {"field": "earliest_service_date", "kind": "within_span",
     "sources": ["has_Briefing_Medical_Events[].Service_Date[]"]},
    {"field": "latest_service_date", "kind": "within_span",
     "sources": ["has_Briefing_Medical_Events[].Service_Date[]"]}
  ]
}`

// A briefing the projection supports at every field.
const groundedBriefing = `{
  "conditions": [
    {"code": "B182", "name": "Chronic viral hepatitis C"},
    {"code": "F1120", "name": "Opioid dependence, uncomplicated"}
  ],
  "medications": [{"drug_name": "traMADol HCl", "maintenance": "N"}],
  "medical_event_count": 2,
  "pharmacy_event_count": 1,
  "earliest_service_date": "2025-06-10",
  "latest_service_date": "2025-08-14"
}`

func schemaFor(t *testing.T) *briefing.Schema {
	t.Helper()
	s, findings := briefing.ParseSchema(briefingSchema)
	if s == nil {
		t.Fatalf("the provenance schema does not parse: %+v", findings)
	}
	return s
}

func TestProjectedEntityGroundsABriefing(t *testing.T) {
	s, rm := projectedBriefing(t)
	for _, encoding := range []string{"toon", "json"} {
		t.Run(encoding, func(t *testing.T) {
			entity := encodeAsPipeline(t, s, rm, encoding)
			res, err := briefing.CheckEncoded(schemaFor(t), encoding, entity, groundedBriefing)
			if err != nil {
				t.Fatalf("CheckEncoded: %v", err)
			}
			if !res.OK() {
				t.Fatalf("expected no findings, got %s", res.String())
			}
			// Every populated leaf of the briefing resolved to somewhere in the
			// entity: that is A§8.3's source reference per field, computed.
			for _, field := range []string{"/conditions/0/code", "/medications/0/drug_name",
				"/medical_event_count", "/latest_service_date"} {
				if len(res.Refs[field]) == 0 {
					t.Errorf("%s carries no source reference", field)
				}
			}
		})
	}
}

// shippedNotice is the text `BP_Disclaimer10` asserts in
// `workspaces/jets_ws/jet_rules/clinical_intel/briefing_projection.jr`.
const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision. " +
	"Downstream use is governed by the service agreement."

// The notice is on the record and out of the prompt, and **the exclusion is what
// puts it out** - which is the point of testing it in both directions rather
// than only the passing one.
//
// `cintel:Briefing` is an allowlist: a property no rule copies onto it cannot
// reach the model. The notice breaks that symmetry in the same way
// `cintel:Briefing_Member_ID` does, and for the same reason - the record must
// carry it and the prompt must not - so it is kept out by an
// `exclude_properties` entry, which is a denylist. **Delete that entry and a
// legal notice is in every prompt, and nothing fails.** This is agentic_ai
// I-439's second instance rather than its first, and the second is what turns
// a one-property exception into the argument for an `include_properties`.
func TestTheNoticeIsOnTheRecordAndNotInThePrompt(t *testing.T) {
	s, rm := projectedBriefing(t)
	for _, encoding := range []string{"toon", "json"} {
		entity := encodeAsPipeline(t, s, rm, encoding)
		if contains(entity, "Briefing_Disclaimer") || contains(entity, "not medical advice") {
			t.Errorf("%s encoding carries the intended-use notice:\n%s", encoding, entity)
		}
	}
	// Without the exclusion it is there, so the exclusion is doing the work.
	ce := &compute_pipes.JrSpecialColumnEncoding{
		Config: &compute_pipes.ColumnEncodingSpec{
			Column:              "cintel:Briefing_Input",
			EntityEncoding:      "toon",
			RemoveModelPrefixes: true,
		},
		ExcludeProperties: map[string]bool{"jets:key": true, "rdf:type": true},
	}
	out, ok := ce.EncodeColumnData(&JetRdfSessionGo{rdfSession: s}, &RdfNodeGo{node: rm.NewResource("briefing")}).(string)
	if !ok {
		t.Fatal("EncodeColumnData did not return a string")
	}
	if !contains(out, "Briefing_Disclaimer") {
		t.Fatalf("without the exclusion the notice should reach the prompt, so that removing the "+
			"exclusion is a change with a consequence a test can see:\n%s", out)
	}
}

func TestExcludedMemberIdIsNotInThePrompt(t *testing.T) {
	s, rm := projectedBriefing(t)
	for _, encoding := range []string{"toon", "json"} {
		entity := encodeAsPipeline(t, s, rm, encoding)
		if contains(entity, "M-4471") || contains(entity, "Briefing_Member_ID") {
			t.Errorf("%s encoding carries the member id:\n%s", encoding, entity)
		}
	}
}

func TestInventedFieldsAreRefused(t *testing.T) {
	s, rm := projectedBriefing(t)
	entity := encodeAsPipeline(t, s, rm, "toon")

	cases := []struct {
		name, brief, wantCode string
	}{
		{
			// The failure A§8.3's prompt instruction was the only guard against.
			name: "a diagnosis outside the input set",
			brief: `{"conditions":[{"code":"E119","name":"Type 2 diabetes mellitus without complications"}],
			         "medications":[],"medical_event_count":2,"pharmacy_event_count":1,
			         "earliest_service_date":"2025-06-10","latest_service_date":"2025-08-14"}`,
			wantCode: briefing.CodeUngrounded,
		},
		{
			// A prefix of a code the member does have. This is the case that
			// makes exact matching worth the projection: under match: substring
			// it would be grounded by B182.
			name: "a code that is a prefix of one the member has",
			brief: `{"conditions":[{"code":"B18","name":"Chronic viral hepatitis C"}],
			         "medications":[],"medical_event_count":2,"pharmacy_event_count":1,
			         "earliest_service_date":"2025-06-10","latest_service_date":"2025-08-14"}`,
			wantCode: briefing.CodeUngrounded,
		},
		{
			name: "a count that disagrees with the entity",
			brief: `{"conditions":[],"medications":[],"medical_event_count":7,"pharmacy_event_count":1,
			         "earliest_service_date":"2025-06-10","latest_service_date":"2025-08-14"}`,
			wantCode: briefing.CodeWrongCount,
		},
		{
			name: "a date outside the observed span",
			brief: `{"conditions":[],"medications":[],"medical_event_count":2,"pharmacy_event_count":1,
			         "earliest_service_date":"2019-01-01","latest_service_date":"2025-08-14"}`,
			wantCode: briefing.CodeOutOfSpan,
		},
		{
			// The closure. A field nobody declared is a finding even though the
			// response_format cannot produce it - a model that ignores its
			// format is exactly the case a guardrail is for.
			name: "a field the schema says nothing about",
			brief: `{"conditions":[],"medications":[],"medical_event_count":2,"pharmacy_event_count":1,
			         "earliest_service_date":"2025-06-10","latest_service_date":"2025-08-14",
			         "recommended_action":"Advise the member to schedule a follow-up"}`,
			wantCode: briefing.CodeNoRule,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := briefing.CheckTOON(schemaFor(t), entity, tc.brief)
			if err != nil {
				t.Fatalf("CheckTOON: %v", err)
			}
			found := false
			for _, f := range res.Findings {
				if f.Code == tc.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected a %s finding, got %s", tc.wantCode, res.String())
			}
		})
	}
}

// The pharmacy list holds one entry, so cintel:Medication serialises as a bare
// scalar rather than an array (F516). The selector's [] over a scalar is what
// keeps the check working for the member a guardrail is least entitled to be
// wrong about, and this asserts it through the real encoder rather than through
// a hand-written document.
func TestSingleValuedPropertySerialisesUnwrappedAndStillGrounds(t *testing.T) {
	s, rm := projectedBriefing(t)
	toon := encodeAsPipeline(t, s, rm, "toon")
	if contains(toon, "has_Briefing_Pharmacy_Events[") {
		t.Fatalf("expected the single pharmacy event to serialise unwrapped:\n%s", toon)
	}
	res, err := briefing.CheckTOON(schemaFor(t), toon, groundedBriefing)
	if err != nil {
		t.Fatalf("CheckTOON: %v", err)
	}
	if !res.OK() {
		t.Fatalf("expected no findings, got %s", res.String())
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// A source selector whose last segment has no `[]` stops grounding the moment
// the property carries two values, and this is the trap the fixture above was
// built to find rather than a hypothetical.
//
// `Selector.Resolve` returns the whole slice as one value for a segment without
// `[]`, so the comparison is against "[f1120 l0390]" and fails. The direction of
// the failure is the safe one - a false finding rather than a false pass - but
// it is silent in the case an author is most likely to test, because F516 means
// the same selector works perfectly for a member with one event.
//
// So: put `[]` on a leaf source selector unless the property cannot be
// multi-valued, and note that `[]` over a scalar yields that scalar, so it costs
// nothing to write it anyway.
func TestSourceSelectorWithoutBracketsMissesTheSecondValue(t *testing.T) {
	s, rm := projectedBriefing(t)
	entity := encodeAsPipeline(t, s, rm, "toon")
	schema := `{"key":"k","rules":[{"field":"conditions[].code","kind":"grounded",
	  "sources":["has_Briefing_Medical_Events[].Diagnosis_Code"]}]}`
	parsed, findings := briefing.ParseSchema(schema)
	if parsed == nil {
		t.Fatalf("schema does not parse: %+v", findings)
	}
	// B182 is the only value of its property on its event, so it grounds.
	res, err := briefing.CheckTOON(parsed, entity, `{"conditions":[{"code":"B182"}]}`)
	if err != nil {
		t.Fatalf("CheckTOON: %v", err)
	}
	if !res.OK() {
		t.Fatalf("the single-valued case should still ground: %s", res.String())
	}
	// F1120 shares its property with L0390, so the selector resolves to the slice.
	res, err = briefing.CheckTOON(parsed, entity, `{"conditions":[{"code":"F1120"}]}`)
	if err != nil {
		t.Fatalf("CheckTOON: %v", err)
	}
	if res.OK() {
		t.Fatal("expected the multi-valued case to be reported ungrounded without []")
	}
}
