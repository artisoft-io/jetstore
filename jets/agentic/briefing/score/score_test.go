package score_test

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing/score"
)

// The unit tests. They are deliberately small and deliberately not a second
// corpus: the corpus is the artefact test beside this one, and a fixture written
// from the same head as the code proves only that the head is consistent.
//
// What these assert is the *mechanism* - the cases the artefact test found and
// the ones a reader would want pinned so that a later change has to notice
// breaking them.

// entity builds a fact set the way the model arm's entity arrives: the drug is a
// joined `Medication` string, because the four component properties are in that
// arm's exclusion list.
func entity(conditions []any, meds ...string) map[string]any {
	events := make([]any, 0, len(meds))
	for _, m := range meds {
		events = append(events, map[string]any{"Medication": m})
	}
	e := map[string]any{
		"Briefing_Data_Flags":          "Adherence_Ratio is applicable only for maintenance drugs",
		"has_Briefing_Pharmacy_Events": events,
		"Pharmacy_Event_Count":         len(meds),
		"has_Briefing_Medical_Events":  []any{map[string]any{"Care_Setting": "Office", "Service_Date": "2025-01-01"}},
		"Medical_Event_Count":          1,
	}
	if conditions != nil {
		e["Condition_Summary"] = conditions
	}
	return e
}

func scoreOf(t *testing.T, e map[string]any, text string) *score.Report {
	t.Helper()
	return score.Score("T", score.ReadFactSet(e), text, score.Options{})
}

func TestTheFactSetReadsBothShapesOfADrug(t *testing.T) {
	// The joined value, which is what the model arm's entity carries.
	joined := score.ReadFactSet(entity(nil,
		`Naproxen Sodium ER (maintenance Y, adherence 0.00, 5 fills: 2025-02-06, 2025-04-05)`,
		`Diclofenac Sodium (maintenance N, 2 fills: 2025-09-25, 2025-12-13)`))
	if len(joined.Drugs) != 2 {
		t.Fatalf("want 2 drugs, got %d", len(joined.Drugs))
	}
	if d := joined.Drugs[0]; d.Name != "Naproxen Sodium ER" || !d.Maintenance || !d.HasAdherence || d.Adherence != "0.00" || d.Fills != 5 {
		t.Errorf("joined maintenance drug read as %+v", d)
	}
	if d := joined.Drugs[1]; d.Name != "Diclofenac Sodium" || d.Maintenance || d.HasAdherence || d.Fills != 2 {
		t.Errorf("joined non-maintenance drug read as %+v", d)
	}
	if joined.Drugs[0].Source != "joined" {
		t.Errorf("want Source joined, got %q", joined.Drugs[0].Source)
	}

	// The property shape, which is what the template arm's entity carries. The
	// two must produce the same fact set, or the scorer scores whichever arm it
	// was pointed at and refuses the other.
	props := score.ReadFactSet(map[string]any{
		"has_Briefing_Pharmacy_Events": []any{
			map[string]any{"Drug_Name": "Naproxen Sodium ER", "Maintenance": "Y", "Adherence_Ratio": "0.00", "Fill_Count": 5},
			map[string]any{"Drug_Name": "Diclofenac Sodium", "Maintenance": "N", "Fill_Count": 2},
		},
	})
	for i := range props.Drugs {
		a, b := props.Drugs[i], joined.Drugs[i]
		if a.Name != b.Name || a.Maintenance != b.Maintenance || a.HasAdherence != b.HasAdherence || a.Fills != b.Fills {
			t.Errorf("drug %d differs between the two entity shapes:\n  properties %+v\n  joined     %+v", i, a, b)
		}
	}
}

// TestASingleValuedListArrivesAsAScalar pins the rule the selector language
// holds and this package must not re-derive: `addToEntityObj` promotes a
// property to a slice only when a second value arrives, so a member with one
// pharmacy event has a map where a member with two has a list.
func TestASingleValuedListArrivesAsAScalar(t *testing.T) {
	f := score.ReadFactSet(map[string]any{
		"Condition_Summary":            "(E119) Type 2 diabetes mellitus without complications",
		"has_Briefing_Pharmacy_Events": map[string]any{"Medication": `Lasix (maintenance Y, adherence 0.00, 1 fill: 2025-09-05)`},
	})
	if len(f.Drugs) != 1 || len(f.Conditions) != 1 {
		t.Fatalf("want 1 drug and 1 condition from the scalar shape, got %d and %d", len(f.Drugs), len(f.Conditions))
	}
	if f.Conditions[0].Code != "E119" {
		t.Errorf("condition code read as %q", f.Conditions[0].Code)
	}
}

func TestMaintenanceAbsentIsExactAboutNegation(t *testing.T) {
	e := entity(nil, `Amoxicillin (maintenance N, 2 fills: 2025-05-28, 2025-07-20)`)
	cases := []struct {
		name string
		text string
		want int
	}{
		{"asserts it", "There are two pharmacy events for maintenance medications: Amoxicillin.", 1},
		{"denies it", "No maintenance medications are listed for this member.", 0},
		{"hyphenated denial", "Amoxicillin, for a non-maintenance indication, was dispensed twice.", 0},
		{"reads the flag back", "The Adherence Ratio is applicable only for maintenance drugs.", 0},
		{"says nothing", "Two fills of Amoxicillin are on record.", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scoreOf(t, e, c.text).Flagged(score.CatMaintenanceAbsent); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
}

// TestTheAppositiveClaimsOnlyItsOwnNoun is the M029 case, which is the one the
// labelled corpus found and the reason claimant exists. The briefing is correct
// and a clause-wide rule reports two errors in it.
func TestTheAppositiveClaimsOnlyItsOwnNoun(t *testing.T) {
	e := entity(nil,
		`Sertraline HCl (maintenance Y, adherence 0.00, 3 fills: 2025-01-01, 2025-02-01, 2025-03-01)`,
		`busPIRone HCl (maintenance N, 5 fills: 2025-01-03, 2025-05-14, 2025-06-22, 2025-10-10, 2025-10-18)`)
	text := "Adherence is noted as zero for sertraline HCl, a maintenance medication, with three fills " +
		"recorded, and no adherence noted for buspirone HCl, a non-maintenance medication, with five fills."
	rep := scoreOf(t, e, text)
	for _, cat := range []string{
		score.CatMaintenanceDrug, score.CatAdherenceAttributed,
		score.CatFillCountMismatch, score.CatUntrue,
	} {
		if got := rep.Flagged(cat); got != 0 {
			r := rep.Result(cat)
			t.Errorf("%s flagged %d on a correct briefing; findings: %+v", cat, got, r.Findings)
		}
	}
}

// TestAListIsClaimedWhole is the other half of the same rule: the constructions
// the model actually uses put the anchor before the list, so nothing about the
// appositive exception narrows them.
func TestAListIsClaimedWhole(t *testing.T) {
	e := entity(nil,
		`Cetirizine-Pseudoephedrine ER (maintenance N, 2 fills: 2025-08-24, 2025-06-01)`,
		`Azelastine-Fluticasone (maintenance N, 2 fills: 2025-08-16, 2025-06-02)`)
	text := "Additionally, there are two pharmacy events for maintenance medications: " +
		"Cetirizine-Pseudoephedrine ER and Azelastine-Fluticasone, with two fills each."
	if got := scoreOf(t, e, text).Flagged(score.CatMaintenanceDrug); got != 2 {
		t.Errorf("want both drugs flagged, got %d", got)
	}
}

// TestTheTemplateConstructionIsNotFlagged is the negative control in miniature.
// The template renders a maintenance clause per drug and separates drugs with a
// semicolon; the appositive rule alone would also be enough.
func TestTheTemplateConstructionIsNotFlagged(t *testing.T) {
	e := entity(nil,
		`Naproxen Sodium ER (maintenance Y, adherence 0.00, 5 fills: 2025-02-06, 2025-04-05, 2025-10-04, 2025-10-09, 2025-10-23)`,
		`Diclofenac Sodium (maintenance N, 2 fills: 2025-09-25, 2025-12-13)`)
	text := "Medications: Naproxen Sodium ER, a maintenance medication, five fills, most recently 23 October; " +
		"and Diclofenac Sodium, two fills, most recently 13 December."
	rep := scoreOf(t, e, text)
	for _, cat := range []string{score.CatMaintenanceDrug, score.CatFillCountMismatch, score.CatUntrue} {
		if got := rep.Flagged(cat); got != 0 {
			t.Errorf("%s flagged %d on the template's own construction: %+v", cat, got, rep.Result(cat).Findings)
		}
	}
}

// TestTheNoticeIsStripped is why the advisory category does not report the one
// string in the artefact that must carry imperative language.
func TestTheNoticeIsStripped(t *testing.T) {
	const notice = "Informational only. Prepared from claims data for a call-centre representative, who is not a " +
		"clinician. This is not medical advice, not a diagnosis and not a treatment recommendation, and it must " +
		"not be used to make or support a clinical decision. Downstream use is governed by the service agreement."
	const rule = "--------------------------------------------------------------------"
	e := entity(nil)
	e["Briefing_Disclaimer"] = notice
	body := "Four medical visits on record, between 20 February and 28 September 2025."
	if got := scoreOf(t, e, notice+"\n\n"+rule+"\n\n"+body).Flagged(score.CatAdvisory); got != 0 {
		t.Errorf("the notice was counted as advisory language")
	}
	// And the body still is scanned.
	if got := scoreOf(t, e, notice+"\n\n"+rule+"\n\nThe member should follow up with the prescriber.").
		Flagged(score.CatAdvisory); got != 1 {
		t.Errorf("advisory language in the body was not seen")
	}
}

// TestUnwrappingIsWhatKeepsTheImperativeFamilyHonest is the M030 false positive
// the negative control produced on the first run: `contact` opens a wrapped line
// and opens no clause.
func TestUnwrappingIsWhatKeepsTheImperativeFamilyHonest(t *testing.T) {
	wrapped := "Ten medical visits on record, between 23 June and 1 December 2025. Most\nrecent\ncontact: Urgent Care Facility, 1 December."
	if got := scoreOf(t, entity(nil), wrapped).Flagged(score.CatAdvisory); got != 0 {
		t.Errorf("a hard-wrapped line was read as an imperative clause")
	}
}

// TestTheOmissionRuleMatchesWords is the change that moves a number against
// `AT.5`'s script: `arthritis` is a substring of `osteoarthritis` and is not a
// word of it.
func TestTheOmissionRuleMatchesWords(t *testing.T) {
	e := entity([]any{
		"(M069) Rheumatoid arthritis, unspecified",
		"(M1711) Unilateral primary osteoarthritis, right knee",
	})
	text := "The patient has osteoarthritis affecting the right knee, unilateral and primary."
	res := scoreOf(t, e, text).Result(score.CatConditionOmitted)
	if res.Flagged != 1 {
		t.Errorf("want the rheumatoid arthritis value flagged and the osteoarthritis one not, got %d flagged: %+v",
			res.Flagged, res.Findings)
	}
	if res.Low > res.Flagged || res.Flagged > res.High {
		t.Errorf("the point estimate %d is outside its own bracket [%d, %d]", res.Flagged, res.Low, res.High)
	}
	// The code alone is enough, and needs no tokens.
	if got := scoreOf(t, e, "Conditions include (M069) and (M1711).").Result(score.CatConditionOmitted).Flagged; got != 0 {
		t.Errorf("naming both codes still reported %d omissions", got)
	}
}

func TestCountsAreReadInDigitsAndInWords(t *testing.T) {
	e := entity(nil)
	e["Medical_Event_Count"] = 62
	for _, text := range []string{
		"There are 62 medical events on record.",
		"Sixty-two medical visits on record, between 6 January and 13 December 2025.",
	} {
		if got := scoreOf(t, e, text).Flagged(score.CatCountMismatch); got != 0 {
			t.Errorf("a correct count was flagged: %q", text)
		}
	}
	if got := scoreOf(t, e, "The patient had three medical encounters.").Flagged(score.CatCountMismatch); got != 1 {
		t.Errorf("a wrong count in words was not flagged")
	}
}

// TestTheEchoIsReadOffTheFactSet pins the property this package leans on: the
// names in the prompt are the names in the entity, so the closed list is read
// rather than written down.
func TestTheEchoIsReadOffTheFactSet(t *testing.T) {
	e := entity(nil)
	cases := []struct {
		name string
		text string
		want int
	}{
		{"the flag verbatim", "The Adherence Ratio is applicable only for maintenance drugs.", 1},
		{"an identifier from the flag", "Note that the Adherence_Ratio flag applies only to maintenance drugs.", 1},
		{"a TOON array name", "The diagnoses are presented as separate entries in the Diagnosis[3] field.", 0},
		{"agreement rather than quotation", "Adherence ratios are not applicable as no maintenance drugs are present.", 0},
		{"neither", "Four medical visits on record.", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scoreOf(t, e, c.text).Flagged(score.CatEchoesPrompt); got != c.want {
				t.Errorf("want %d, got %d", c.want, got)
			}
		})
	}
	// The array form needs the property to be in this entity, which `Diagnosis`
	// is not at the root - it is a property of a medical event. Put one there
	// and the same sentence is an echo.
	e2 := entity(nil)
	e2["has_Briefing_Medical_Events"] = []any{map[string]any{"Diagnosis": []any{"(A) x", "(B) y", "(C) z"}}}
	if got := scoreOf(t, e2, "The diagnoses are presented as separate entries in the Diagnosis[3] field.").
		Flagged(score.CatEchoesPrompt); got != 1 {
		t.Errorf("a TOON array name from a nested property was not seen")
	}
}

// TestNotAutomatableCategoriesSaySo is the design rule asserted rather than
// documented: a category nothing measures must be present and must not read as
// a zero somebody can quote.
func TestNotAutomatableCategoriesSaySo(t *testing.T) {
	rep := scoreOf(t, entity(nil), "Anything.")
	for _, name := range []string{score.CatInvention, score.CatMispairing, score.CatImplication, score.CatMaintenanceUnderCall} {
		res := rep.Result(name)
		if res == nil {
			t.Fatalf("%s is missing from the report; a category nobody can see reads as a category that scored zero", name)
		}
		if res.Category.Soundness != score.NotAutomatable {
			t.Errorf("%s is %s, want not-automatable", name, res.Category.Soundness)
		}
		if res.Denominator != 0 {
			t.Errorf("%s has denominator %d; an unmeasured category must not carry a rate", name, res.Denominator)
		}
	}
}

// TestEveryCategoryDeclaresItself is the constructor rule the package comment
// states: a category with no residue is one nobody has thought about.
func TestEveryCategoryDeclaresItself(t *testing.T) {
	rep := scoreOf(t, entity(nil, `Lasix (maintenance Y, adherence 0.00, 1 fill: 2025-09-05)`), "Lasix, one fill.")
	seen := map[string]bool{}
	for _, res := range rep.Results {
		c := res.Category
		switch {
		case c.Name == "":
			t.Errorf("a category with no name")
		case seen[c.Name]:
			t.Errorf("%s appears twice", c.Name)
		case strings.TrimSpace(c.Question) == "":
			t.Errorf("%s declares no question", c.Name)
		case strings.TrimSpace(c.Rule) == "":
			t.Errorf("%s declares no rule", c.Name)
		case strings.TrimSpace(c.Residue) == "":
			t.Errorf("%s declares no residue", c.Name)
		case strings.TrimSpace(c.Unit) == "":
			t.Errorf("%s declares no unit", c.Name)
		case c.Soundness == score.Approximate && c.Direction == score.DirectionNone:
			t.Errorf("%s is approximate and names no error direction", c.Name)
		case c.Soundness != score.Approximate && c.Direction != score.DirectionNone:
			t.Errorf("%s is %s and names an error direction anyway", c.Name, c.Soundness)
		}
		seen[c.Name] = true
	}
}

func TestARateCarriesItsDenominator(t *testing.T) {
	c := &score.Corpus{}
	e := entity(nil, `Amoxicillin (maintenance N, 2 fills: 2025-05-28, 2025-07-20)`)
	c.Add(score.Score("M1", score.ReadFactSet(e), "Two pharmacy events for maintenance medications: Amoxicillin.", score.Options{}))
	c.Add(score.Score("M2", score.ReadFactSet(e), "Amoxicillin, two fills.", score.Options{}))
	for _, r := range c.Rates() {
		if r.Category.Name != score.CatMaintenanceAbsent {
			continue
		}
		if got := r.String(); got != "1 / 2 qualifying briefings" {
			t.Errorf("rate rendered as %q", got)
		}
	}
}

// TestAnAmbiguousMentionIsReportedAndNotCounted is the tie-break rule: a surface
// form two drugs answer to cannot decide which one a briefing meant, and an
// instrument that picked would be reporting its own choice.
//
// The case is real rather than contrived. A pharmacy event is keyed by NDC, so
// one member can carry the same drug name twice with different indicators - the
// labelled corpus has `Ibuprofen` three times and `Cyclobenzaprine HCl` twice.
// Where the duplicates agree the ambiguity cannot change the answer and the
// finding is counted; where they disagree it is reported and left out.
func TestAnAmbiguousMentionIsReportedAndNotCounted(t *testing.T) {
	disagree := entity(nil,
		`Ibuprofen (maintenance Y, adherence 0.00, 3 fills: 2025-04-12, 2025-05-07, 2025-06-13)`,
		`Ibuprofen (maintenance N, 2 fills: 2025-05-31, 2025-06-19)`)
	res := scoreOf(t, disagree, "The member is on maintenance medications including ibuprofen.").
		Result(score.CatMaintenanceDrug)
	if res.Flagged != 0 {
		t.Errorf("an ambiguous surface form was counted: %+v", res.Findings)
	}
	found := false
	for _, f := range res.Findings {
		if f.Ambiguous {
			found = true
		}
	}
	if !found {
		t.Errorf("an ambiguous surface form was silently dropped rather than reported")
	}

	// Both non-maintenance: the ambiguity cannot change the answer, so both are
	// counted and both findings say they were not uniquely attributable.
	agree := entity(nil,
		`Cyclobenzaprine HCl (maintenance N, 3 fills: 2025-01-27, 2025-05-01, 2025-08-06)`,
		`Cyclobenzaprine HCl (maintenance N, 6 fills: 2025-05-05, 2025-05-08, 2025-08-13)`)
	res = scoreOf(t, agree, "The member is on maintenance medications including cyclobenzaprine.").
		Result(score.CatMaintenanceDrug)
	if res.Flagged != 2 {
		t.Errorf("want both agreeing duplicates counted, got %d: %+v", res.Flagged, res.Findings)
	}
	for _, f := range res.Findings {
		if !f.Ambiguous {
			t.Errorf("a finding on a shared surface form did not say it was unattributable: %+v", f)
		}
	}
}
