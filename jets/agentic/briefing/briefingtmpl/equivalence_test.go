package briefingtmpl_test

// **This is criterion 82 and it is a byte comparison per fixture.**
//
// `AY.4` asks that the seventeen tests of `jets/agentic/briefing/prose` run
// unchanged against the configured engine. They do - they are untouched and
// they still pass - and what this file adds is the comparison those tests
// cannot make themselves: the same input through `prose.Render` and through
// [briefingtmpl.Render], byte for byte, so that criterion 82 is checked on
// every run rather than asserted once.
//
// # The fixtures are transcribed, and that is the one weakness worth naming
//
// `section1106Entity` and `TestRenderCases`' six mutations are unexported
// helpers of `package prose`'s own test binary, so no other package can call
// them, and `AY.3` may not add a file to that directory. **So they are
// transcribed here rather than shared** (`I-679`), and the transcription is
// pinned rather than trusted: every case asserts the two renderers agree, and
// the anchor case additionally asserts that both produce plan §10.9's printed
// block byte for byte. A transcription that had drifted would fail the second
// assertion with `prose.Render` on the wrong side of it, which is the failure
// a copied fixture is supposed to be able to hide.
//
// What it cannot catch is a fixture `prose_test.go` gains *later*. That is the
// residual and it is recorded rather than engineered around.

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/briefingtmpl"
	"github.com/artisoft-io/jetstore/jets/agentic/briefing/prose"
)

// shippedNotice is what `BP_Disclaimer10` asserts in
// `workspaces/jets_ws/jet_rules/clinical_intel/briefing_projection.jr`.
const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision. " +
	"Downstream use is governed by the service agreement."

func d(y, m, day int) time.Time {
	return time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC)
}

// section1106Entity is `prose_test.go`'s fixture, transcribed property for
// property - **with the types `extractAsEntity` produces**, which is the half a
// hand-written fixture gets wrong: a date is a `time.Time`, a count is an
// `int`, and a multi-valued property holding one value is a bare scalar rather
// than a one-element list (`addToEntityObj`,
// `jets/compute_pipes/jetrules_extract_entity.go:112`). `Diagnosis` is a string
// on the second medical event and a `[]any` on the first for exactly that
// reason, and `Fill_Date` likewise.
func section1106Entity() map[string]any {
	return map[string]any{
		"Briefing_Member_ID":  "900123456",
		"Briefing_Disclaimer": shippedNotice,
		"Condition_Summary": []any{
			"(B182) Chronic viral hepatitis C",
			"(F1120) Alcohol dependence",
			"(L0390) Cellulitis",
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
				"Medication": "lisinopril (maintenance Y, adherence 0.89, 3 fills: " +
					"2025-06-15, 2025-07-20, 2025-08-24)",
				"Drug_Name":   "lisinopril",
				"Maintenance": "Y",
				"Fill_Count":  3,
				"Fill_Date":   []any{d(2025, 6, 15), d(2025, 7, 20), d(2025, 8, 24)},
				"Adherence":   0.89,
			},
			map[string]any{
				"Medication":  "traMADol HCl (maintenance N, 1 fill: 2025-07-02)",
				"Drug_Name":   "traMADol HCl",
				"Maintenance": "N",
				"Fill_Count":  1,
				"Fill_Date":   d(2025, 7, 2),
			},
		},
	}
}

// section1106Rendered is what plan §10.9 prints under "Over P7 §1.10.6's entity
// that renders", byte for byte, and it is what `prose_test.go` asserts.
const section1106Rendered = `Informational only. Prepared from claims data for a call-centre
representative, who is not a clinician. This is not medical advice, not a
diagnosis and not a treatment recommendation, and it must not be used to make
or support a clinical decision. Downstream use is governed by the service
agreement.

--------------------------------------------------------------------

Two medical visits on record, between 10 June and 14 August 2025. Most recent
contact: Independent Laboratory, 14 August.

Conditions on record: Chronic viral hepatitis C, Alcohol dependence and
Cellulitis.

Medications: lisinopril, a maintenance medication, three fills, most recently
24 August; and traMADol HCl, one fill, most recently 2 July.`

// agree renders one entity both ways and reports what the two produced.
//
// **It compares the error as a state and the output as bytes**, which is the
// only honest comparison available: the two callers refuse the same inputs and
// say so in their own words - `prose.Render` in a sentence it composes, this
// one by reporting the template's own `require` message - and criterion 82 is
// about the artefact rather than about the diagnosis. Where the diagnosis
// matters the case asserts it by hand.
func agree(t *testing.T, entity map[string]any) (string, error) {
	t.Helper()
	wantOut, wantErr := prose.Render(entity)
	gotOut, gotErr := briefingtmpl.Render(entity)
	switch {
	case wantErr != nil && gotErr == nil:
		t.Fatalf("prose refused and the template did not.\nprose: %v\ntemplate rendered:\n%s", wantErr, gotOut)
	case wantErr == nil && gotErr != nil:
		t.Fatalf("the template refused and prose did not.\ntemplate: %v\nprose rendered:\n%s", gotErr, wantOut)
	case wantErr != nil:
		return "", gotErr
	}
	if gotOut != wantOut {
		t.Fatalf("the configured engine and prose.Render differ.\n--- prose ---\n%s\n--- template ---\n%s",
			wantOut, gotOut)
	}
	return gotOut, nil
}

// TestTheDocumentCompiles is C1 to C12 over the one document that matters. A
// compile failure here is a defect in `patient_profile_briefing.json`, because
// the document is a constant of the build.
func TestTheDocumentCompiles(t *testing.T) {
	tmpl, err := briefingtmpl.Compiled()
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if tmpl.Key() != briefingtmpl.Key {
		t.Errorf("the document registers under %q and the package constant says %q", tmpl.Key(), briefingtmpl.Key)
	}
	if tmpl.Width() != prose.Width {
		t.Errorf("the document wraps at %d and prose.Width is %d", tmpl.Width(), prose.Width)
	}
}

// TestTheAnchorFixture is `TestRendersSection1106Verbatim`, `TestTheNoticeIsAboveTheProse`,
// `TestAdherenceAppearsNowhereInTheProse` and `TestTheMemberIdAndTheCodesAreNotInTheProse`
// at once: they assert four properties of one output, and the output is
// asserted here byte for byte against the block the plan prints.
func TestTheAnchorFixture(t *testing.T) {
	got, err := agree(t, section1106Entity())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != section1106Rendered {
		t.Fatalf("the rendered artefact differs from plan §10.9.\n--- want ---\n%s\n--- got ---\n%s",
			section1106Rendered, got)
	}
}

// TestTheJoinedMedicationValueIsNeverRead is I-544 asserted rather than
// commented: the joined value is the prompt's and the components are the
// template's, and a document that quietly fell back to parsing the joined value
// would pass every other case in this file.
func TestTheJoinedMedicationValueIsNeverRead(t *testing.T) {
	entity := section1106Entity()
	for _, ev := range entity["has_Briefing_Pharmacy_Events"].([]any) {
		delete(ev.(map[string]any), "Medication")
	}
	got, err := agree(t, entity)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if got != section1106Rendered {
		t.Error("dropping the joined Medication value changed the prose, so the document reads it")
	}
}

// TestTheTwoRefusalsThatAreTemplateAssertions is §10.7's split demonstrated:
// the two claims about the record are `require`s in the document, and each
// still names the property it is about.
func TestTheTwoRefusalsThatAreTemplateAssertions(t *testing.T) {
	t.Run("a pharmacy event with no Drug_Name", func(t *testing.T) {
		entity := section1106Entity()
		ev := entity["has_Briefing_Pharmacy_Events"].([]any)[0].(map[string]any)
		delete(ev, "Drug_Name")
		_, err := agree(t, entity)
		if err == nil {
			t.Fatal("expected a refusal")
		}
		if !strings.Contains(err.Error(), "Drug_Name") {
			t.Errorf("the refusal should name the missing component, got %v", err)
		}
	})
	t.Run("a briefing with no notice", func(t *testing.T) {
		entity := section1106Entity()
		delete(entity, "Briefing_Disclaimer")
		if _, err := agree(t, entity); err == nil {
			t.Fatal("expected a refusal: prose delivered without its notice is worse than no output")
		}
	})
	// **And the third refusal is the caller's**, which is the half of §10.7
	// that this package rather than the document carries (I-623).
	t.Run("a prefixed entity", func(t *testing.T) {
		_, err := agree(t, map[string]any{
			"cintel:Briefing_Disclaimer":  shippedNotice,
			"cintel:Medical_Event_Count":  2,
			"cintel:Pharmacy_Event_Count": 0,
		})
		if err == nil {
			t.Fatal("expected a refusal for a prefixed entity")
		}
		if !strings.Contains(err.Error(), "remove_model_prefixes") {
			t.Errorf("the refusal should name the setting, got %v", err)
		}
	})
}

// TestRenderCases is `prose_test.go`'s six subcases, transcribed with their
// names, and each asserts the body the table there asserts as well as agreeing
// with `prose.Render`. **The body is asserted as well as compared** because two
// renderers agreeing on the wrong answer is the one failure a comparison cannot
// see.
func TestRenderCases(t *testing.T) {
	base := func(mut func(map[string]any)) map[string]any {
		e := map[string]any{"Briefing_Disclaimer": shippedNotice}
		mut(e)
		return e
	}
	cases := []struct {
		name string
		mut  func(map[string]any)
		want string
	}{
		{
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
				e["Condition_Summary"] = []any{"(A) Asthma", "(B) Bronchitis"}
				e["Pharmacy_Event_Count"] = 0
				e["Medical_Event_Count"] = 1
				e["has_Briefing_Medical_Events"] = map[string]any{"Service_Date": d(2025, 5, 5)}
			},
			want: "One medical visit on record. Most recent contact: 5 May.\n\n" +
				"Conditions on record: Asthma and Bronchitis.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := agree(t, base(tc.mut))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			want := wrapNotice() + "\n\n" + rule + "\n\n" + tc.want
			if got != want {
				t.Errorf("--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}

// TestFortyConditionsAreAllEmitted is the one structural assertion of the
// seventeen: exactly three blank-line separators, which is notice, rule, volume,
// conditions and the medications element correctly not emitting.
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
		conditions = append(conditions, "(Z"+string(rune('A'+i%26))+") Dx-"+spell(i))
	}
	entity["Condition_Summary"] = conditions
	got, err := agree(t, entity)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if n := strings.Count(got, "Dx-"); n != 40 {
		t.Errorf("expected all forty conditions, got %d", n)
	}
	if n := strings.Count(got, "\n\n"); n != 3 {
		t.Errorf("forty conditions should still be one paragraph; got %d separators", n)
	}
}

// TestATieOnTheLatestServiceDateStillRenders: `latest_by` resolves a tie to the
// first node in the document's order and never fails. **Both renderers resolve
// it the same way and that is a fact rather than a coincidence** - the engine's
// `latest_by` and `prose`'s `latestBy` walk the same slice with the same strict
// `After`, so the comparison in `agree` is meaningful here rather than vacuous.
func TestATieOnTheLatestServiceDateStillRenders(t *testing.T) {
	got, err := agree(t, map[string]any{
		"Briefing_Disclaimer":  shippedNotice,
		"Medical_Event_Count":  2,
		"Pharmacy_Event_Count": 0,
		"has_Briefing_Medical_Events": []any{
			map[string]any{"Care_Setting": "Urgent Care Facility", "Service_Date": d(2025, 8, 14)},
			map[string]any{"Care_Setting": "Independent Laboratory", "Service_Date": d(2025, 8, 14)},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(got, "Urgent Care Facility") && !strings.Contains(got, "Independent Laboratory") {
		t.Errorf("neither tied encounter was named: %s", got)
	}
}

// TestTheCareSettingIsVerbatim: Telehealth is the value that decides it, because
// a fluent construction produces "at a telehealth" over the 52 place-of-service
// descriptions.
func TestTheCareSettingIsVerbatim(t *testing.T) {
	for _, setting := range []string{
		"Telehealth",
		"Prison/Correctional Facility",
		"Intermediate Care/Mentally Retarded Facility",
	} {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer":         shippedNotice,
			"Medical_Event_Count":         1,
			"Pharmacy_Event_Count":        0,
			"has_Briefing_Medical_Events": map[string]any{"Care_Setting": setting, "Service_Date": d(2025, 8, 14)},
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		flat := strings.Join(strings.Fields(got), " ")
		if !strings.Contains(flat, "contact: "+setting+",") {
			t.Errorf("the care setting %q is not carried verbatim in its labelled clause: %s", setting, got)
		}
	}
}

// TestDecodedDocumentTypesRenderTheSame is the second of the two shapes a
// record arrives in: every number a float64 and every date text, which is what
// json and toon produce. **It is the shape a hand-written fixture silently
// avoids**, and the constructs normalise both.
func TestDecodedDocumentTypesRenderTheSame(t *testing.T) {
	got, err := agree(t, map[string]any{
		"Briefing_Disclaimer":   shippedNotice,
		"Medical_Event_Count":   float64(2),
		"Pharmacy_Event_Count":  float64(0),
		"Earliest_Service_Date": "2025-06-10",
		"Latest_Service_Date":   "2025-08-14",
		"has_Briefing_Medical_Events": []any{
			map[string]any{"Care_Setting": "Emergency Room - Hospital", "Service_Date": "2025-06-10"},
			map[string]any{"Care_Setting": "Independent Laboratory", "Service_Date": "2025-08-14"},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	want := "Two medical visits on record, between 10 June and 14 August 2025. Most recent\n" +
		"contact: Independent Laboratory, 14 August."
	if !strings.HasSuffix(got, want) {
		t.Errorf("--- want suffix ---\n%s\n--- got ---\n%s", want, got)
	}
}

// TestNoLineExceedsTheWidth: the wrap is the document's `width` applied per
// paragraph, and a word longer than the width takes a line of its own rather
// than being split.
func TestNoLineExceedsTheWidth(t *testing.T) {
	entity := section1106Entity()
	entity["has_Briefing_Medical_Events"].([]any)[1].(map[string]any)["Care_Setting"] =
		strings.Repeat("Supercalifragilistic", 6)
	got, err := agree(t, entity)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > prose.Width && !strings.Contains(line, "Supercalifragilistic") {
			t.Errorf("line of %d characters: %q", len([]rune(line)), line)
		}
	}
}

// TestTheTwoBehavioursNoFixtureSees is `AY.3`'s two decisions asserted, and it
// is the whole reason they are decisions rather than inheritances.
//
// **`I-621` - the count fallbacks - is reproduced**, and the argument is that
// dropping it does not cost a sentence. Without the fallback an entity carrying
// events and no count fails every element gate *and* the group's guard, so the
// fallback fires and the briefing says **No claims activity on record for this
// member** about a member with claims. That is an affirmative false statement
// rather than an omission, and it is the failure mode `prose.Render`'s
// prefixed-entity refusal exists to prevent. `I-621` priced it as a briefing
// "missing a sentence"; it is the briefing.
//
// **`I-622` - `Maintenance` matched with `EqualFold` - is reproduced**, and it
// is exact rather than approximate: `strings.EqualFold(s, "Y")` over a trimmed
// value is true for `y` and for `Y` and for nothing else, so
// `Maintenance == 'Y' or Maintenance == 'y'` is that function written out. It
// costs four words and buys the one element a representative acts on.
//
// Neither assertion can be made through `prose.Render` and then compared,
// because agreement is what is being demonstrated - so each case asserts the
// text, and `agree` asserts that `prose.Render` says the same thing.
func TestTheTwoBehavioursNoFixtureSees(t *testing.T) {
	t.Run("I-621 a medical count absent beside a populated list", func(t *testing.T) {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer": shippedNotice,
			"has_Briefing_Medical_Events": []any{
				map[string]any{"Care_Setting": "Telehealth", "Service_Date": d(2025, 3, 4)},
				map[string]any{"Care_Setting": "Independent Laboratory", "Service_Date": d(2025, 5, 5)},
			},
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if strings.Contains(got, "No claims activity") {
			t.Fatalf("a member with two events was told there is no claims activity:\n%s", got)
		}
		if !strings.Contains(got, "Two medical visits on record.") {
			t.Errorf("the fallback did not count the list:\n%s", got)
		}
	})
	t.Run("I-621 a fill count absent beside fill dates", func(t *testing.T) {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer":  shippedNotice,
			"Medical_Event_Count":  0,
			"Pharmacy_Event_Count": 1,
			"has_Briefing_Pharmacy_Events": map[string]any{
				"Drug_Name": "metformin",
				"Fill_Date": []any{d(2025, 1, 9), d(2025, 2, 11)},
			},
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !strings.Contains(unwrapped(got), "metformin, two fills, most recently 11 February") {
			t.Errorf("the fill-date fallback did not fire:\n%s", got)
		}
	})
	t.Run("I-622 a lower-case maintenance indicator", func(t *testing.T) {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer":  shippedNotice,
			"Medical_Event_Count":  0,
			"Pharmacy_Event_Count": 1,
			"has_Briefing_Pharmacy_Events": map[string]any{
				"Drug_Name":   "lisinopril",
				"Maintenance": " y ",
				"Fill_Count":  1,
			},
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !strings.Contains(got, "a maintenance medication") {
			t.Errorf("a lower-case maintenance indicator was not honoured:\n%s", got)
		}
	})
}

// TestTheEmptyCaseIsReachedByBothOfItsPredicates is F1149 asserted, and it is
// the one place §10.9's document and this one differ in structure rather than
// in wording. `renderBody` returns the empty sentence when both counts are zero
// - **before any element runs** - and again when every element produced
// nothing. §10.9 writes the first as a conjunct on the conditions element,
// which leaves the medications element ungated; this document writes it as a
// nested group, so the guard is one predicate covering all three.
//
// The input that separates the two is a pharmacy count of zero beside a
// populated event list and no medical activity: `prose.Render` emits the empty
// sentence and §10.9's document would emit `Medications:`.
func TestTheEmptyCaseIsReachedByBothOfItsPredicates(t *testing.T) {
	t.Run("both counts zero suppresses an element that would otherwise emit", func(t *testing.T) {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer":  shippedNotice,
			"Medical_Event_Count":  0,
			"Pharmacy_Event_Count": 0,
			"has_Briefing_Pharmacy_Events": map[string]any{
				"Drug_Name":  "metformin",
				"Fill_Count": 1,
			},
			"Condition_Summary": "(E119) Type 2 diabetes",
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !strings.HasSuffix(got, "No claims activity on record for this member.") {
			t.Errorf("the early return was not reproduced:\n%s", got)
		}
	})
	t.Run("counts that say there was activity and elements that say nothing", func(t *testing.T) {
		got, err := agree(t, map[string]any{
			"Briefing_Disclaimer":  shippedNotice,
			"Medical_Event_Count":  0,
			"Pharmacy_Event_Count": 2,
		})
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if !strings.HasSuffix(got, "No claims activity on record for this member.") {
			t.Errorf("the second predicate was not reproduced:\n%s", got)
		}
	})
}

// TestAnEmptyConditionDoesNotEmitAnEmptyClause is the second place §10.9 and
// this document differ. §10.9 gates the conditions element on
// `Condition_Summary[]|count > 0`, which counts values; `elementConditions`
// filters values that are empty **after** the code is stripped and only then
// asks whether anything is left. A condition that is nothing but its code is
// the input that separates them, and §10.9's gate would emit
// `Conditions on record: .`
func TestAnEmptyConditionDoesNotEmitAnEmptyClause(t *testing.T) {
	got, err := agree(t, map[string]any{
		"Briefing_Disclaimer":  shippedNotice,
		"Medical_Event_Count":  1,
		"Pharmacy_Event_Count": 0,
		"Condition_Summary":    "(E119)",
		"has_Briefing_Medical_Events": map[string]any{
			"Care_Setting": "Telehealth",
			"Service_Date": d(2025, 3, 4),
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(got, "Conditions on record") {
		t.Errorf("a condition that is nothing but its code emitted a clause:\n%s", got)
	}
}

// --- helpers ---------------------------------------------------------------

// rule is the horizontal rule, 68 hyphens, verbatim from `prose.go`. It is
// inside `width` and so is never wrapped.
const rule = "--------------------------------------------------------------------"

// wrapNotice is the notice as the artefact carries it. It is written out rather
// than computed, because computing it with the renderer under test would make
// the expectation agree with the thing it is checking.
func wrapNotice() string {
	return strings.Join([]string{
		"Informational only. Prepared from claims data for a call-centre",
		"representative, who is not a clinician. This is not medical advice, not a",
		"diagnosis and not a treatment recommendation, and it must not be used to make",
		"or support a clinical decision. Downstream use is governed by the service",
		"agreement.",
	}, "\n")
}

// spell is `words` for 0 to 39, which is all `TestFortyConditionsAreAllEmitted`
// needs. It is here so that the fixture does not have to reach into either
// renderer for a name, which would make the input a function of the thing under
// test.
func spell(n int) string {
	units := []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen",
		"eighteen", "nineteen"}
	if n < 20 {
		return units[n]
	}
	if n%10 == 0 {
		return "twenty"
	}
	return "twenty-" + units[n%10]
}

func unwrapped(s string) string { return strings.Join(strings.Fields(s), " ") }
