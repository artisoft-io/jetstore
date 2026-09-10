package prose

import (
	"strings"
	"testing"
	"time"
)

// shippedNotice is the text `BP_Disclaimer10` asserts in
// `workspaces/jets_ws/jet_rules/clinical_intel/briefing_projection.jr`, and it
// is what plan §1.10.6 prints above the rule. It is reproduced here rather than
// imported because `jets/agentic/briefing`'s copy is in a test file of another
// package.
const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision. " +
	"Downstream use is governed by the service agreement."

func d(y, m, day int) time.Time {
	return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
}

// section1106Entity is the entity plan §1.10.6 prints, transcribed property for
// property.
//
// **The types are the ones `extractAsEntity` produces, not the ones the plan's
// listing shows.** A date is a `time.Time` because `GetRdfNodeValue` unwraps an
// `rdf.LDate` to one (`GetRdfNodeValue`,
// `jets/compute_pipes/jetrules_interface.go:100`), a count is an `int`, and a
// multi-valued property is a bare scalar until a second value arrives
// (`addToEntityObj`, `jets/compute_pipes/jetrules_extract_entity.go:112`) - which
// is why `Diagnosis` is a string on the second medical event and a `[]any` on
// the first. `TestPipelineEntityShape` in the external test file demonstrates
// that this transcription is what the real encoder writes.
func section1106Entity() map[string]any {
	return map[string]any{
		"Briefing_Member_ID":  "900123456",
		"Briefing_Disclaimer": shippedNotice,
		"Condition_Summary": []any{
			"(F1120) Alcohol dependence",
			"(L0390) Cellulitis",
			"(B182) Chronic viral hepatitis C",
		},
		"Medical_Event_Count":   2,
		"Pharmacy_Event_Count":  2,
		"Earliest_Service_Date": d(2025, 6, 10),
		"Latest_Service_Date":   d(2025, 8, 14),
		"has_Briefing_Medical_Events": []any{
			map[string]any{
				"Care_Setting": "Emergency Room - Hospital",
				"Diagnosis":    []any{"(F1120) Alcohol dependence", "(L0390) Cellulitis"},
				"Service_Date": d(2025, 6, 10),
			},
			map[string]any{
				"Care_Setting": "Independent Laboratory",
				"Diagnosis":    "(B182) Chronic viral hepatitis C",
				"Service_Date": d(2025, 8, 14),
			},
		},
		"has_Briefing_Pharmacy_Events": []any{
			map[string]any{
				"Medication":  "traMADol HCl (maintenance N, 1 fill: 2025-07-02)",
				"Drug_Name":   "traMADol HCl",
				"Maintenance": "N",
				"Fill_Count":  1,
				"Fill_Date":   d(2025, 7, 2),
			},
			map[string]any{
				"Medication": "lisinopril (maintenance Y, adherence 0.89, 3 fills: " +
					"2025-06-15, 2025-07-20, 2025-08-24)",
				"Drug_Name":   "lisinopril",
				"Maintenance": "Y",
				"Fill_Count":  3,
				"Fill_Date":   []any{d(2025, 6, 15), d(2025, 7, 20), d(2025, 8, 24)},
				"Adherence":   0.89,
			},
		},
	}
}

// section1106Rendered is the block plan §1.10.6 prints under "Rendered over the
// entity above", byte for byte.
const section1106Rendered = `Informational only. Prepared from claims data for a call-centre
representative, who is not a clinician. This is not medical advice, not a
diagnosis and not a treatment recommendation, and it must not be used to make
or support a clinical decision. Downstream use is governed by the service
agreement.

--------------------------------------------------------------------

Two medical visits on record, between 10 June and 14 August 2025. Most recent
contact: Independent Laboratory, 14 August.

Conditions on record: Alcohol dependence, Cellulitis and Chronic viral
hepatitis C.

Medications: traMADol HCl, one fill, most recently 2 July; and lisinopril, a
maintenance medication, three fills, most recently 24 August.`

// TestRendersSection1106Verbatim is the single most load-bearing assertion in
// this package: the plan prints the briefing it decided on, so the plan is the
// assertion and drift between §1.10.6 and this renderer fails here.
func TestRendersSection1106Verbatim(t *testing.T) {
	got, err := Render(section1106Entity())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != section1106Rendered {
		t.Errorf("rendered output differs from plan §1.10.6.\n--- want ---\n%s\n--- got ---\n%s",
			section1106Rendered, got)
	}
}

// The renderer never reads `Medication`, so removing it changes nothing. This is
// I-544's resolution asserted rather than commented: the joined value is the
// prompt's and the components are the template's, and a renderer that quietly
// fell back to parsing the joined value would pass every other test in this file.
func TestJoinedMedicationValueIsNeverRead(t *testing.T) {
	entity := section1106Entity()
	for _, ev := range entity["has_Briefing_Pharmacy_Events"].([]any) {
		delete(ev.(map[string]any), "Medication")
	}
	got, err := Render(entity)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != section1106Rendered {
		t.Error("dropping the joined Medication value changed the prose, so something reads it")
	}
}

// And a pharmacy event that carries only the joined value is a refusal rather
// than a parse.
func TestAPharmacyEventWithoutDrugNameIsRefused(t *testing.T) {
	entity := section1106Entity()
	ev := entity["has_Briefing_Pharmacy_Events"].([]any)[0].(map[string]any)
	delete(ev, "Drug_Name")
	_, err := Render(entity)
	if err == nil {
		t.Fatal("expected a refusal for a pharmacy event with no Drug_Name")
	}
	if !strings.Contains(err.Error(), "Drug_Name") {
		t.Errorf("the refusal should name the missing component, got %v", err)
	}
}

func TestAdherenceAppearsNowhereInTheProse(t *testing.T) {
	got, err := Render(section1106Entity())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// §1.10.4: as a number and as a flag. The ratio is on the entity - it
	// reaches the model arm - and the prose says three fills and a date instead.
	for _, forbidden := range []string{"0.89", "adherence", "Adherence", "0.9", "coverage"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the prose carries %q; §1.10.4 keeps the adherence ratio out of it entirely", forbidden)
		}
	}
	if !strings.Contains(got, "three fills, most recently 24 August") {
		t.Error("what replaces the ratio is the count and the latest fill, and it is missing")
	}
}

func TestTheMemberIdAndTheCodesAreNotInTheProse(t *testing.T) {
	got, err := Render(section1106Entity())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "900123456") {
		t.Error("the member id is on the record and is not in the briefing (§1.10.4)")
	}
	for _, code := range []string{"(F1120)", "(L0390)", "(B182)"} {
		if strings.Contains(got, code) {
			t.Errorf("the diagnosis code %s is stripped in the prose and stays on the entity", code)
		}
	}
}

// The notice is above the prose and the rule is between them (§1.10.5). A notice
// a representative reaches after the content is a notice that arrives after they
// have started talking.
func TestTheNoticeIsAboveTheProse(t *testing.T) {
	got, err := Render(section1106Entity())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	noticeAt := strings.Index(got, "Informational only.")
	ruleAt := strings.Index(got, rule)
	proseAt := strings.Index(got, "Two medical visits")
	if noticeAt != 0 || ruleAt < noticeAt || proseAt < ruleAt {
		t.Errorf("the artefact must be notice, rule, prose in that order; got offsets %d %d %d",
			noticeAt, ruleAt, proseAt)
	}
}

func TestABriefingWithNoNoticeIsRefused(t *testing.T) {
	entity := section1106Entity()
	delete(entity, "Briefing_Disclaimer")
	if _, err := Render(entity); err == nil {
		t.Fatal("expected a refusal: prose delivered without its notice is worse than no output")
	}
}

// remove_model_prefixes: false is a configuration error that would otherwise
// render as a member with no claims - the properties are all there and none of
// them is found under the name the renderer looks for.
func TestAPrefixedEntityIsRefusedRatherThanRenderedEmpty(t *testing.T) {
	entity := map[string]any{
		"cintel:Briefing_Disclaimer":  shippedNotice,
		"cintel:Medical_Event_Count":  2,
		"cintel:Pharmacy_Event_Count": 0,
	}
	_, err := Render(entity)
	if err == nil {
		t.Fatal("expected a refusal for a prefixed entity")
	}
	if !strings.Contains(err.Error(), "remove_model_prefixes") {
		t.Errorf("the refusal should name the setting, got %v", err)
	}
}

func TestRenderCases(t *testing.T) {
	base := func(mut func(map[string]any)) map[string]any {
		e := map[string]any{"Briefing_Disclaimer": shippedNotice}
		mut(e)
		return e
	}
	cases := []struct {
		name string
		mut  func(map[string]any)
		// want is the body: what follows the notice and the rule.
		want string
	}{
		{
			// §1.10.6's empty case. A blank briefing under a notice reads as a
			// rendering failure, and I-515's whole subject is a briefing that is
			// empty when it should not be.
			name: "no events at all",
			mut: func(e map[string]any) {
				e["Medical_Event_Count"] = 0
				e["Pharmacy_Event_Count"] = 0
			},
			want: "No claims activity on record for this member.",
		},
		{
			name: "no counts and no lists is the same empty case",
			mut:  func(e map[string]any) {},
			want: "No claims activity on record for this member.",
		},
		{
			name: "medical only, one visit",
			mut: func(e map[string]any) {
				e["Medical_Event_Count"] = 1
				e["Pharmacy_Event_Count"] = 0
				e["Earliest_Service_Date"] = d(2025, 3, 4)
				e["Latest_Service_Date"] = d(2025, 3, 4)
				e["has_Briefing_Medical_Events"] = map[string]any{
					"Care_Setting": "Telehealth",
					"Service_Date": d(2025, 3, 4),
				}
			},
			want: "One medical visit on record, between 4 March and 4 March 2025. Most recent\n" +
				"contact: Telehealth, 4 March.",
		},
		{
			// The pharmacy count reaches the prose only through the medication
			// list; §1.10.6's template renders it nowhere else.
			name: "pharmacy only",
			mut: func(e map[string]any) {
				e["Medical_Event_Count"] = 0
				e["Pharmacy_Event_Count"] = 1
				e["has_Briefing_Pharmacy_Events"] = map[string]any{
					"Drug_Name":   "metformin",
					"Maintenance": "Y",
					"Fill_Count":  2,
					"Fill_Date":   []any{d(2025, 1, 9), d(2025, 2, 11)},
				}
			},
			want: "Medications: metformin, a maintenance medication, two fills, most recently 11\nFebruary.",
		},
		{
			name: "one condition",
			mut: func(e map[string]any) {
				e["Medical_Event_Count"] = 0
				e["Pharmacy_Event_Count"] = 1
				e["Condition_Summary"] = "(E119) Type 2 diabetes"
				e["has_Briefing_Pharmacy_Events"] = map[string]any{
					"Drug_Name":  "metformin",
					"Fill_Count": 1,
				}
			},
			want: "Conditions on record: Type 2 diabetes.\n\nMedications: metformin, one fill.",
		},
		{
			name: "two conditions take the last separator only",
			mut: func(e map[string]any) {
				e["Medical_Event_Count"] = 0
				e["Pharmacy_Event_Count"] = 0
				e["Condition_Summary"] = []any{"(A) Asthma", "(B) Bronchitis"}
				// A count of zero on both would be the empty case, so give the
				// briefing one event to be about.
				e["Medical_Event_Count"] = 1
				e["has_Briefing_Medical_Events"] = map[string]any{"Service_Date": d(2025, 5, 5)}
			},
			want: "One medical visit on record. Most recent contact: 5 May.\n\n" +
				"Conditions on record: Asthma and Bronchitis.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(base(tc.mut))
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			want := wrap(shippedNotice, Width) + "\n\n" + rule + "\n\n" + tc.want
			if got != want {
				t.Errorf("--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}

// Forty conditions is where §1.10.6 says the template visibly runs out. The
// assertion is what it *does* rather than that it is good: it emits forty, in
// the projection's order, and that is the salience gap Q-94 is about.
func TestFortyConditionsAreAllEmitted(t *testing.T) {
	entity := map[string]any{
		"Briefing_Disclaimer":  shippedNotice,
		"Medical_Event_Count":  1,
		"Pharmacy_Event_Count": 0,
		"has_Briefing_Medical_Events": map[string]any{
			"Care_Setting": "Independent Laboratory",
			"Service_Date": d(2025, 8, 14),
		},
	}
	conditions := make([]any, 0, 40)
	for i := range 40 {
		conditions = append(conditions, "(Z"+string(rune('A'+i%26))+") Dx-"+words(i))
	}
	entity["Condition_Summary"] = conditions
	got, err := Render(entity)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n := strings.Count(got, "Dx-"); n != 40 {
		t.Errorf("expected all forty conditions, got %d", n)
	}
	// It is one paragraph, wrapped, and the representative reads all of it. The
	// template ranks nothing - that is the finding, not a defect to fix here.
	if strings.Count(got, "\n\n") != 3 {
		t.Errorf("forty conditions should still be one paragraph; got %d", strings.Count(got, "\n\n"))
	}
}

// A member with two encounters on the same day: `latest_by` resolves the tie to
// one of them and never fails. The assertion is that a briefing is produced and
// names one of the two, because no order over the pair is more true.
func TestATieOnTheLatestServiceDateStillRenders(t *testing.T) {
	entity := map[string]any{
		"Briefing_Disclaimer":  shippedNotice,
		"Medical_Event_Count":  2,
		"Pharmacy_Event_Count": 0,
		"has_Briefing_Medical_Events": []any{
			map[string]any{"Care_Setting": "Urgent Care Facility", "Service_Date": d(2025, 8, 14)},
			map[string]any{"Care_Setting": "Independent Laboratory", "Service_Date": d(2025, 8, 14)},
		},
	}
	got, err := Render(entity)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(got, "Urgent Care Facility") && !strings.Contains(got, "Independent Laboratory") {
		t.Errorf("neither tied encounter was named: %s", got)
	}
}

// The care setting is rendered verbatim, and Telehealth is the value that
// decides it: a fluent construction produces "at a telehealth".
func TestTheCareSettingIsVerbatim(t *testing.T) {
	for _, setting := range []string{
		"Telehealth",
		"Prison/Correctional Facility",
		"Intermediate Care/Mentally Retarded Facility",
	} {
		entity := map[string]any{
			"Briefing_Disclaimer":         shippedNotice,
			"Medical_Event_Count":         1,
			"Pharmacy_Event_Count":        0,
			"has_Briefing_Medical_Events": map[string]any{"Care_Setting": setting, "Service_Date": d(2025, 8, 14)},
		}
		got, err := Render(entity)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		flat := strings.Join(strings.Fields(got), " ")
		if !strings.Contains(flat, "contact: "+setting+",") {
			t.Errorf("the care setting %q is not carried verbatim in its labelled clause: %s", setting, got)
		}
	}
}

// A briefing whose properties arrive as decoded json or toon rather than from
// `extractAsEntity` renders identically: every number is a float64 and every
// date is text, and the accessors normalise both.
func TestDecodedDocumentTypesRenderTheSame(t *testing.T) {
	entity := map[string]any{
		"Briefing_Disclaimer":   shippedNotice,
		"Medical_Event_Count":   float64(2),
		"Pharmacy_Event_Count":  float64(0),
		"Earliest_Service_Date": "2025-06-10",
		"Latest_Service_Date":   "2025-08-14",
		"has_Briefing_Medical_Events": []any{
			map[string]any{"Care_Setting": "Emergency Room - Hospital", "Service_Date": "2025-06-10"},
			map[string]any{"Care_Setting": "Independent Laboratory", "Service_Date": "2025-08-14"},
		},
	}
	got, err := Render(entity)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "Two medical visits on record, between 10 June and 14 August 2025. Most recent\n" +
		"contact: Independent Laboratory, 14 August."
	if !strings.HasSuffix(got, want) {
		t.Errorf("--- want suffix ---\n%s\n--- got ---\n%s", want, got)
	}
}

// Every line of a rendered briefing is inside Width, whatever the input. A word
// longer than the width takes a line of its own rather than being split.
func TestNoLineExceedsTheWidth(t *testing.T) {
	entity := section1106Entity()
	entity["has_Briefing_Medical_Events"].([]any)[1].(map[string]any)["Care_Setting"] =
		strings.Repeat("Supercalifragilistic", 6)
	got, err := Render(entity)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > Width && !strings.Contains(line, "Supercalifragilistic") {
			t.Errorf("line of %d characters: %q", len([]rune(line)), line)
		}
	}
}
