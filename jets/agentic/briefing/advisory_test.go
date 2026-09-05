package briefing

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The delivered contract: the shape of workspaces/jets_ws/provenance/
// patient_briefing.pv.json, reduced to what this file is about. Every rule is of
// a kind that bounds its field's value, so the briefing has no surface on which
// the model writes prose of its own.
const confinedDoc = `{
  "key": "patient_briefing",
  "disclaimer": {"field": "cintel:Briefing_Disclaimer"},
  "response_format": {
    "type": "object", "additionalProperties": false,
    "properties": {
      "conditions": {"type": "array", "items": {"type": "object", "additionalProperties": false,
        "properties": {"code": {"type": "string"}, "name": {"type": "string"}}}},
      "medical_event_count": {"type": "integer"},
      "earliest_service_date": {"type": "string"}
    }
  },
  "rules": [
    {"field": "conditions[].code", "kind": "grounded", "match": "exact", "sources": ["e[].Diagnosis_Code[]"]},
    {"field": "conditions[].name", "kind": "grounded", "match": "exact", "sources": ["e[].Diagnosis_Name[]"]},
    {"field": "medical_event_count", "kind": "count_of", "sources": ["e[]"]},
    {"field": "earliest_service_date", "kind": "within_span", "sources": ["e[].Service_Date[]"]}
  ]
}`

// Criterion 53's first clause, established about the contract rather than about
// one briefing: there is no field on which the model writes its own prose.
func TestTheDeliveredContractHasNoProseSurface(t *testing.T) {
	s, findings := ParseSchema(confinedDoc)
	if s == nil {
		t.Fatalf("expected the contract to parse: %+v", findings)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
	if got := s.ProseSurfaces(); len(got) != 0 {
		t.Fatalf("expected 0 prose surfaces over 4 fields, got %d: %v", len(got), got)
	}
}

// **Negative control 1, and it is the one that needs no lexicon at all.** Give
// the delivered contract a briefing whose condition name is an instruction and
// the grounding rule refuses it by name, because the sentence appears nowhere in
// the input entity. This is what *structural* means: the guardrail against
// advisory language on a bounded field is the bound, not a word list.
func TestAdvisoryLanguageOnABoundedFieldIsRefusedByItsBound(t *testing.T) {
	s, findings := ParseSchema(confinedDoc)
	if s == nil {
		t.Fatalf("schema does not parse: %+v", findings)
	}
	entity := `{"e":[{"Diagnosis_Code":"B182","Diagnosis_Name":"Chronic viral hepatitis C","Service_Date":"2025-08-14"}]}`
	brief := `{"conditions":[{"code":"B182","name":"Start a statin and schedule a follow-up"}],
	           "medical_event_count":1,"earliest_service_date":"2025-08-14"}`
	res, err := CheckJSON(s, entity, brief)
	if err != nil {
		t.Fatalf("CheckJSON: %v", err)
	}
	if res.OK() {
		t.Fatal("a briefing whose condition name is an instruction must not pass")
	}
	if !hasFindingCode(res.Findings, CodeUngrounded) {
		t.Fatalf("expected %s, got %s", CodeUngrounded, res.String())
	}
	t.Logf("negative control 1: %s", res.String())
}

// **Negative control 2**: introduce a field the contract does not bound, and the
// document says so when it is saved. A Warning rather than an Error, because
// `ungrounded` is AK.1's declared exemption with a required reason and refusing
// it outright here would delete that escape hatch on this task's authority.
func TestConfineNamesAFieldTheContractDoesNotBound(t *testing.T) {
	doc := strings.Replace(confinedDoc,
		`"earliest_service_date": {"type": "string"}`,
		`"earliest_service_date": {"type": "string"}, "summary": {"type": "string"}`, 1)
	doc = strings.Replace(doc,
		`{"field": "earliest_service_date", "kind": "within_span", "sources": ["e[].Service_Date[]"]}`,
		`{"field": "earliest_service_date", "kind": "within_span", "sources": ["e[].Service_Date[]"]},
		 {"field": "summary", "kind": "ungrounded", "reason": "a free-text recap the representative reads first"}`, 1)
	s, findings := ParseSchema(doc)
	if s == nil {
		t.Fatalf("an ungrounded field is a declared exemption and must still save: %+v", findings)
	}
	if !hasCode(findings, CodeFieldAdmitsProse) {
		t.Fatalf("expected %s, got %+v", CodeFieldAdmitsProse, findings)
	}
	var msg string
	for _, f := range findings {
		if f.Code == CodeFieldAdmitsProse {
			msg = f.Message
			if f.Severity != wsvalidate.Warning {
				t.Errorf("a declared exemption is a warning, got %s", f.Severity)
			}
		}
	}
	if !strings.Contains(msg, "summary") {
		t.Errorf("the finding must name the field, got %q", msg)
	}
	if got := s.ProseSurfaces(); len(got) != 1 || got[0] != "summary" {
		t.Errorf("ProseSurfaces = %v, want [summary]", got)
	}
	t.Logf("negative control 2: %s", msg)

	// And the value on that field is scanned on every record.
	entity := `{"e":[{"Diagnosis_Code":"B182","Diagnosis_Name":"Chronic viral hepatitis C","Service_Date":"2025-08-14"}]}`
	brief := `{"conditions":[{"code":"B182","name":"Chronic viral hepatitis C"}],
	           "medical_event_count":1,"earliest_service_date":"2025-08-14",
	           "summary":"The member should discuss statin therapy with their provider."}`
	res, err := CheckJSON(s, entity, brief)
	if err != nil {
		t.Fatalf("CheckJSON: %v", err)
	}
	if !hasFindingCode(res.Findings, CodeAdvisoryLanguage) {
		t.Fatalf("expected %s, got %s", CodeAdvisoryLanguage, res.String())
	}
	t.Logf("negative control 3: %s", res.String())

	// The same field with a value that asserts rather than advises passes,
	// which is what stops the exemption from being useless.
	res, err = CheckJSON(s, entity, strings.Replace(brief,
		"The member should discuss statin therapy with their provider.",
		"Two claims in the last six months, both outpatient.", 1))
	if err != nil {
		t.Fatalf("CheckJSON: %v", err)
	}
	if !res.OK() {
		t.Fatalf("a declarative free-text field must pass: %s", res.String())
	}
}

// A§8.3's notice is a required field of the output record, so a contract that
// declares a briefing and no notice is refused.
func TestConfineRequiresADisclaimer(t *testing.T) {
	doc := strings.Replace(confinedDoc,
		`"disclaimer": {"field": "cintel:Briefing_Disclaimer"},`, "", 1)
	s, findings := ParseSchema(doc)
	if s != nil {
		t.Fatal("a briefing contract with no intended-use notice must be refused")
	}
	if !hasCode(findings, CodeNoDisclaimer) {
		t.Fatalf("expected %s, got %+v", CodeNoDisclaimer, findings)
	}
	t.Logf("negative control 4: %s", findings[0].Message)
}

// *Populated deterministically rather than by the model*, made mechanical: the
// notice may not be a field the response schema produces, and may not have a
// provenance rule. Both would mean the model wrote it and something checked it
// afterwards, which is not the same claim.
func TestConfineRefusesADisclaimerTheModelWrites(t *testing.T) {
	doc := strings.Replace(confinedDoc,
		`"disclaimer": {"field": "cintel:Briefing_Disclaimer"}`,
		`"disclaimer": {"field": "notice"}`, 1)
	doc = strings.Replace(doc,
		`"earliest_service_date": {"type": "string"}`,
		`"earliest_service_date": {"type": "string"}, "notice": {"type": "string"}`, 1)
	doc = strings.Replace(doc,
		`{"field": "earliest_service_date", "kind": "within_span", "sources": ["e[].Service_Date[]"]}`,
		`{"field": "earliest_service_date", "kind": "within_span", "sources": ["e[].Service_Date[]"]},
		 {"field": "notice", "kind": "derived", "sources": ["disclaimer"]}`, 1)
	s, findings := ParseSchema(doc)
	if s != nil {
		t.Fatal("a disclaimer the model returns must be refused")
	}
	if !hasCode(findings, CodeDisclaimerIsModelWritten) {
		t.Fatalf("expected %s, got %+v", CodeDisclaimerIsModelWritten, findings)
	}
	t.Logf("negative control 5: %s", findings[0].Message)
}

func TestConfineRefusesADisclaimerWithNoField(t *testing.T) {
	doc := strings.Replace(confinedDoc,
		`"disclaimer": {"field": "cintel:Briefing_Disclaimer"}`,
		`"disclaimer": {"comment": "not medical advice"}`, 1)
	if s, findings := ParseSchema(doc); s != nil || !hasCode(findings, CodeDisclaimerHasNoField) {
		t.Fatalf("expected %s, got %+v", CodeDisclaimerHasNoField, findings)
	}
}

// The notice this project ships, and the reason it is not a briefing field.
//
// **It would fail the advisory check.** *It must not be used to make a clinical
// decision* is advice about advice, and every workable phrasing of a
// not-medical-advice notice is. A briefing field for it would need a permanent
// exemption from the check inside the document the check guards; a record column
// the model never sees needs none, because the check runs over the model's
// answer and the notice is not in it.
func TestTheNoticeWouldFailTheCheckItIsNotSubjectTo(t *testing.T) {
	if marker, found := advisoryMarker(shippedNotice); !found {
		t.Fatalf("expected the notice to read as advisory, got no marker")
	} else {
		t.Logf("the notice carries %q, which is why it is not the model's to write", marker)
	}
}

// shippedNotice is the text briefing_projection.jr asserts onto
// cintel:Briefing_Disclaimer. It is duplicated here as a *sample* rather than as
// a source of truth - the rule file is the source - and the test it supports is
// about the shape of such a sentence rather than about this wording.
const shippedNotice = "Informational only. Prepared from claims data for a call-centre representative, " +
	"who is not a clinician. This is not medical advice, not a diagnosis and not a treatment " +
	"recommendation, and it must not be used to make or support a clinical decision."

func TestAdvisoryMarkerReadsGuidanceAndNotDescriptions(t *testing.T) {
	guidance := []string{
		"The member should schedule a follow-up.",
		"Recommend increasing adherence support.",
		"Start a statin.",
		"Please contact the care manager.",
		"Avoid NSAIDs.",
		"We advise a medication review.",
		"Next steps: refer to the nurse line.",
		"You need to discuss this with the prescriber.",
	}
	for _, s := range guidance {
		if _, found := advisoryMarker(s); !found {
			t.Errorf("expected %q to read as guidance", s)
		}
	}
	// Values a briefing legitimately carries, drawn from the shapes the
	// workspace's own vocabularies use. None of these is advice, and the word
	// boundaries are what keep them clean: `Continuous` is not `continue` and
	// `Application` is not `apply`.
	descriptions := []string{
		"Chronic viral hepatitis C",
		"Cellulitis, unspecified",
		"Aftercare following surgery on the circulatory system",
		"Encounter for screening for malignant neoplasm of colon",
		"Continuous positive airway pressure ventilation",
		"Application of short arm cast",
		"Personal history of nicotine dependence",
		"traMADol HCl",
		"Two claims in the last six months, both outpatient.",
	}
	for _, s := range descriptions {
		if marker, found := advisoryMarker(s); found {
			t.Errorf("%q is a description and was flagged on %q", s, marker)
		}
	}
}

// The false positives, pinned rather than tuned away.
//
// These are real values from the workspace's own vocabularies and the detector
// flags every one of them. **That is the measurement this design is built on
// rather than a defect to fix**: over the four lookups the rate is 646 of
// 207,655 distinct values, and the flags are `Contact with paper-cutter`,
// `Ensure Bone Health Revigor`, `Monitor Intraocular Press During Vitrectomy`
// and `Care Plan Develop & Document`. Every one is a diagnosis, a drug or a
// procedure that a briefing may legitimately name.
//
// **None of them is ever scanned**, because each lands on a field a grounding
// rule bounds, and the lexicon runs only where nothing does. Tightening the
// lexicon until these pass would trade the false positives for false negatives
// on the fields that *are* scanned; leaving the lexicon blunt and narrowing its
// domain gives up neither. Asserting that they still flag is what stops a later
// tuning pass from quietly making the structural argument unnecessary and then
// removing it.
func TestTheLexiconFlagsRealDescriptionsAndNeverSeesThem(t *testing.T) {
	for _, s := range []string{
		"Contact with paper-cutter, subsequent encounter",
		"Ensure Bone Health Revigor",
		"Monitor Intraocular Press During Vitrectomy",
		"Care Plan Develop & Document",
		"Fluoroscopy Of Bilateral Upper Extremity Veins, Guidance",
		"Use of tobacco",
	} {
		if _, found := advisoryMarker(s); !found {
			t.Errorf("%q was a measured false positive and no longer flags; if the lexicon was tightened, "+
				"re-measure it against the lookups before believing the improvement", s)
		}
	}
	// And the structural fact that makes them harmless: the delivered contract
	// has no field the lexicon runs on at all.
	s, findings := ParseSchema(confinedDoc)
	if s == nil {
		t.Fatalf("schema does not parse: %+v", findings)
	}
	if got := s.ProseSurfaces(); len(got) != 0 {
		t.Fatalf("the lexicon would run on %v; the whole argument is that it runs on nothing", got)
	}
}

// The delivered contract itself, read from the workspace rather than restated.
//
// Every other test in this package works on a document written beside it, which
// agrees with the hand that wrote it - phase 3's I-147. This one opens
// `workspaces/jets_ws/provenance/patient_briefing.pv.json`, the file the save
// path validates and the pipeline is configured against, and asks it the two
// questions criterion 53 is graded on: does it carry a notice, and is there any
// field on which the model writes prose of its own.
func TestTheWorkspaceContractIsConfined(t *testing.T) {
	path := filepath.Join(workspaceRoot(t), "provenance", "patient_briefing.pv.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the jets_ws provenance schema is not in this checkout (%s)", path)
	}
	s, findings := ParseSchema(string(content))
	if s == nil {
		t.Fatalf("the delivered contract does not parse: %+v", findings)
	}
	if len(findings) != 0 {
		t.Fatalf("the delivered contract is not clean: %+v", findings)
	}
	if s.Disclaimer == nil || s.Disclaimer.Field != "cintel:Briefing_Disclaimer" {
		t.Fatalf("the notice is not declared on the record field: %+v", s.Disclaimer)
	}
	surfaces := s.ProseSurfaces()
	if len(surfaces) != 0 {
		t.Fatalf("the delivered briefing has %d field(s) the model writes freely: %v", len(surfaces), surfaces)
	}
	t.Logf("%s: %d rules, 0 prose surfaces, notice on %s", s.Key, len(s.Rules), s.Disclaimer.Field)
}

// --- The measurement -------------------------------------------------------

// TestAdvisoryLexiconAgainstTheWorkspaceVocabularies is why the check is
// structural rather than lexical, and it is a measurement rather than an
// assertion.
//
// It runs the shipped detector over every value the briefing's grounded fields
// can carry - the diagnosis descriptions, the Elixhauser condition descriptions
// and the drug names in `workspaces/jets_ws/lookups/` - and reports how many it
// flags. Those are the false positives a lexical guardrail would produce if it
// were applied where the structural one is applied instead. The rate is logged
// with its denominator rather than asserted, because it is a property of the
// vocabulary and would change when the vocabulary does; what is asserted is that
// the detector is not silent over the corpus, since a lexicon that flags nothing
// at all is one that would prove nothing by flagging nothing.
//
// The lookups are in the parent checkout - `workspaces/` are submodules of
// jetstore_agentic_ai and not of this repo - so the test skips when they are
// absent, on eval_test.go's precedent, and JETS_BRIEFING_VOCAB_ROOT overrides
// the path.
func TestAdvisoryLexiconAgainstTheWorkspaceVocabularies(t *testing.T) {
	root := filepath.Join(workspaceRoot(t), "lookups")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("the jets_ws lookups are not in this checkout (%s); set JETS_BRIEFING_WORKSPACE", root)
	}
	type vocab struct {
		file, column, field string
	}
	total, flagged := 0, 0
	for _, v := range []vocab{
		{"diagnosis_info_lookup.csv", "Description", "conditions[].name"},
		{"elixhauser_conditions_desc.csv", "Description", "conditions[].name"},
		{"drug_info_lookup.csv", "DRUG_NAME", "medications[].drug_name"},
		{"procedure_info_lookup.csv", "Description", "(not in the briefing)"},
	} {
		values, err := distinctColumn(filepath.Join(root, v.file), v.column)
		if err != nil {
			t.Fatalf("reading %s: %v", v.file, err)
		}
		hits := map[string]string{}
		for value := range values {
			if marker, found := advisoryMarker(value); found {
				hits[value] = marker
			}
		}
		total += len(values)
		flagged += len(hits)
		t.Logf("%-32s %s: %6d distinct values, %5d flagged (%.2f%%)",
			v.file, v.field, len(values), len(hits), 100*float64(len(hits))/float64(len(values)))
		for _, sample := range firstN(hits, 6) {
			t.Logf("        e.g. %s", sample)
		}
	}
	t.Logf("TOTAL: %d flagged of %d distinct vocabulary values (%.2f%%)",
		flagged, total, 100*float64(flagged)/float64(total))
	if flagged == 0 {
		t.Error("the lexicon flags nothing over the whole corpus, which would make this measurement vacuous")
	}
}

// workspaceRoot locates `workspaces/jets_ws`.
//
// It is in the *parent* checkout - the workspaces are submodules of
// jetstore_agentic_ai and not of this repo - so a worktree that initialised only
// jetstore_ai has none, and the tests that use it skip rather than fail. That is
// eval_test.go's `repoRootWithWorkspaces` precedent, and the override exists for
// the same reason: the parent is not always one level up.
func workspaceRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("JETS_BRIEFING_WORKSPACE"); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// jets/agentic/briefing -> jetstore_ai -> the parent checkout.
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", "workspaces", "jets_ws"))
}

func distinctColumn(path, column string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	at := -1
	for i, h := range header {
		if strings.TrimSpace(h) == column {
			at = i
		}
	}
	if at < 0 {
		return nil, fmt.Errorf("no column %q in %s", column, path)
	}
	out := map[string]bool{}
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if at < len(rec) && strings.TrimSpace(rec[at]) != "" {
			out[rec[at]] = true
		}
	}
	return out, nil
}

func firstN(hits map[string]string, n int) []string {
	var keys []string
	for k := range hits {
		keys = append(keys, k)
	}
	if len(keys) > n {
		keys = keys[:n]
	}
	var out []string
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%q on %q", k, hits[k]))
	}
	return out
}

func hasFindingCode(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
