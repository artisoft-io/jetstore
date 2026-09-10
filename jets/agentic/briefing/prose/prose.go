// Package prose renders a projected briefing entity as the briefing itself:
// facts from jetrules, prose from a template, no model.
//
// It is agentic_ai's Phase 7 `AT.3`, and it is the **control** rather than the
// product. Every model arm of that phase - the encoding experiment, the backend
// benchmark, the one-call prose arm at `AT.5` - is read against what this
// package writes over the same fact set, which is what exit criterion 74 asks
// for and what criterion 76 means by *the template and the model differ in the
// renderer and in nothing else*.
//
// The specification is plan §1.10.6 of `projects/agentic_ai/plan/phase7_plan.md`:
// four elements in a stated order under a notice and a rule, ten constructs, and
// an explicit empty case. Read that section before changing anything here; the
// order and the omissions in it are decisions with arguments behind them rather
// than styling, and each is commented at the place it is implemented.
//
// # A Go function rather than a template engine, and the reason is the word *control*
//
// §1.10.6 writes the briefing in a `{{...}}` notation. That notation is written
// to be **read** - it is how a reviewer checks that this renderer says what the
// plan decided - and reproducing it as a parsed language would buy nothing this
// task needs and cost the one property that makes the arm worth having.
//
// A template engine is a parser, an evaluator and an error surface: a
// mistyped filter name, a filter applied to the wrong arity, an unclosed `each`.
// Every one of those is a failure *of the renderer* rather than of the data, and
// an arm that can fail for reasons of its own is not a control - it is a second
// thing to debug, competing with the model arm it exists to bound. §1.10.6's own
// argument for the ten constructs is that "none can fail on well-formed input";
// in Go those ten are ten functions the compiler checks the arity and the types
// of, and the sentence becomes a property of the build rather than a claim.
//
// The cost is that the notation and this file can drift. That is paid down by
// `TestRendersSection1106Verbatim`, which holds §1.10.6's printed entity and its
// printed output as one fixture: the plan prints what the briefing should say,
// so the plan is the assertion.
//
// # Total, and what the error return is for
//
// `Render` is a total function of the entity for every briefing a well-formed
// projection produces. The error return is a **malformed map**, not missing
// data: a briefing with no events is a valid input with a specified output
// (§1.10.6's empty case), and a briefing whose pharmacy event carries no
// `Drug_Name` is a projection defect that must be loud. The three refusals are
// listed at `Render`.
//
// # Order is inherited from the input, deliberately
//
// This renderer does not sort, and the reason is measured rather than assumed.
// `extractAsEntity` builds its lists by draining iterators over a `sync.Map`
// (`WSetType`, `jets/jetrules/rdf/base_graph.go:26`), and
// `TestConditionOrderIsAProcessProperty` measures what that costs: **one
// ordering for the life of a process, and a different one in the next process**
// - twelve runs produced five of the six orderings of three conditions on
// 2026-09-09.
//
// **That is a property of the encoder and it is shared with the prompt**: the
// toon column the model arm reads is built by the same walk. Sorting here alone
// would make the two arms differ in something other than the renderer, which is
// exactly what criterion 76 forbids. The fix belongs in `extractAsEntity`, where
// it reaches both arms at once (I-562); until it lands, this renderer is a total
// function of the entity map it is given and the map is what varies.
//
// # What it deliberately does not carry
//
// `cintel:Adherence` appears nowhere in the prose, as a number or as a flag
// (§1.10.4, F754). The ratio's observation window ends at the last fill, so a
// member who **stops** filling scores higher than one who does not; what
// replaces it is what the ratio is computed from - "three fills, most recently
// 24 August". The ratio stays on the entity and reaches the model arm unchanged,
// which is what keeps the pair controlled.
package prose

import (
	"fmt"
	"strings"
)

// Width is the column the rendered briefing is wrapped to.
//
// **Measured from §1.10.6's printed output rather than chosen**: its longest
// line is 77 characters and its first line breaks before a word that would make
// 79, so a greedy wrap reproduces that block at 77 or at 78 and at no other
// width. 78 is taken; the plan's fixture does not separate the two.
const Width = 78

// rule is the horizontal rule between the notice and the prose, verbatim from
// §1.10.6 - 68 characters, which is inside Width and so is never wrapped.
const rule = "--------------------------------------------------------------------"

// emptyCase is what a briefing with no claims activity says.
//
// **It emits a sentence rather than nothing** (§1.10.6). A blank artefact under
// a notice reads as a rendering failure, and I-515's whole subject is a briefing
// that is empty when it should not be - so the one state a reader must be able
// to distinguish from a broken renderer is the one that would otherwise look
// exactly like one.
const emptyCase = "No claims activity on record for this member."

// Render writes the briefing for one projected `cintel:Briefing` entity.
//
// `entity` is `extractAsEntity`'s map for the briefing subject
// (`extractAsEntity`, `jets/compute_pipes/jetrules_extract_entity.go:55`), with
// `remove_model_prefixes` applied and **no** exclusions: plan §1.10.8's
// recommendation is one representation and two callers, the exclusion list the
// only difference between what the template sees and what the model sees. In
// particular the prose column must not exclude `cintel:Briefing_Disclaimer`,
// which the TOON column does exclude - the notice is out of the *prompt* and is
// the first thing in the *artefact*.
//
// It refuses three things and nothing else:
//
//   - a map whose root keys still carry a model prefix, which is
//     `remove_model_prefixes: false` on the column and would otherwise render an
//     empty briefing that looks like a member with no claims;
//   - a briefing with no `Briefing_Disclaimer`, because the artefact is notice,
//     rule, prose in that order (§1.10.5) and prose delivered without its notice
//     is the one output worse than no output;
//   - a pharmacy event carrying no `Drug_Name`. §1.10.6 keeps the components
//     beside the joined `Medication` value precisely so that no renderer has to
//     parse the joined one, and I-544 is the entry that decided it: recovering
//     the name from the joined value would encode `AT.6`'s separator in a second
//     place for a consumer to keep in step with. This renderer never reads
//     `Medication`, so the drift is impossible rather than merely documented.
func Render(entity map[string]any) (string, error) {
	if entity == nil {
		return "", fmt.Errorf("the briefing entity is nil")
	}
	if prefixed := prefixedKey(entity); prefixed != "" {
		return "", fmt.Errorf(
			"the briefing entity carries the prefixed property %q; the prose encoding renders the "+
				"property names the projection asserts and needs remove_model_prefixes on its column_encodings entry",
			prefixed)
	}
	notice := strings.TrimSpace(text(entity, selDisclaimer))
	if notice == "" {
		return "", fmt.Errorf(
			"the briefing entity carries no Briefing_Disclaimer; the notice is asserted unconditionally by " +
				"BP_Disclaimer10 and the rendered artefact is notice, rule, prose in that order")
	}
	body, err := renderBody(entity)
	if err != nil {
		return "", err
	}
	paragraphs := append([]string{notice, rule}, body...)
	return assemble(paragraphs), nil
}

// renderBody is the four elements of §1.10.3, in that order, one paragraph each.
func renderBody(entity map[string]any) ([]string, error) {
	medical := nodes(entity, selMedicalEvents)
	pharmacy := nodes(entity, selPharmacyEvents)
	medicalCount := countOf(entity, "Medical_Event_Count", len(medical))
	pharmacyCount := countOf(entity, "Pharmacy_Event_Count", len(pharmacy))

	if medicalCount == 0 && pharmacyCount == 0 {
		return []string{emptyCase}, nil
	}

	out := make([]string, 0, 3)
	if p := elementVolumeAndWindow(entity, medical, medicalCount); p != "" {
		out = append(out, p)
	}
	if p := elementConditions(entity); p != "" {
		out = append(out, p)
	}
	p, err := elementMedications(pharmacy, pharmacyCount)
	if err != nil {
		return nil, err
	}
	if p != "" {
		out = append(out, p)
	}
	if len(out) == 0 {
		// A count says there was activity and no element could say anything
		// about it. The empty sentence is still the honest output and is still
		// better than a blank page under a notice.
		return []string{emptyCase}, nil
	}
	return out, nil
}

// elementVolumeAndWindow is §1.10.3's element 1 and element 2 in one paragraph:
// how much and how recently, then what the last contact was.
//
// **`latest_by` is applied to the events rather than to `Latest_Service_Date`**
// (§1.10.6): the briefing-level extremum is a date, and element 2 needs the
// *encounter* the extremum belongs to - an emergency room in June and a
// laboratory draw in August are different calls.
//
// **The care setting is rendered verbatim in a labelled clause** and is not
// folded into a fluent sentence. The fluent construction needs a lowercasing and
// an article, and "at a telehealth" is what the article rule produces over the
// 52 place-of-service descriptions. Two total functions that buy fluency and
// cost exactness on the one value in the artefact whose exactness is a
// disclosure question is a bad trade, and a briefing may carry a labelled
// clause.
func elementVolumeAndWindow(entity map[string]any, medical []map[string]any, count int) string {
	if count <= 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s medical %s on record", sentence(words(count)), plural(count, "visit"))
	earliest, okE := date(entity, selEarliest)
	latest, okL := date(entity, selLatest)
	if okE && okL {
		fmt.Fprintf(&b, ", between %s and %s", dMMMM(earliest), dMMMMyyyy(latest))
	}
	b.WriteString(".")
	if last, ok := latestBy(medical, selServiceDate); ok {
		when, _ := date(last, selServiceDate)
		setting := strings.TrimSpace(text(last, selCareSetting))
		switch {
		case setting != "":
			fmt.Fprintf(&b, " Most recent contact: %s, %s.", setting, dMMMM(when))
		default:
			fmt.Fprintf(&b, " Most recent contact: %s.", dMMMM(when))
		}
	}
	return b.String()
}

// elementConditions is §1.10.3's element 3, and it is third rather than first on
// purpose. A briefing that opens with a diagnosis list reads like a clinical
// handover, and under A§8's read-aloud test the first line is the line most
// likely to be spoken. Placing it third costs nothing and buys two sentences of
// framing that make it read as claims history rather than as a finding. **Do not
// "improve" the order.**
//
// **The parenthesised diagnosis code is stripped here and stays on the entity**
// (§1.10.4): the code exists so a checker can match a whole string exactly, and
// "(F1120)" is noise to a member and jargon to a representative.
func elementConditions(entity map[string]any) string {
	conditions := make([]string, 0, 4)
	for _, v := range values(entity, selConditions) {
		if s := strings.TrimSpace(stripCode(scalar(v))); s != "" {
			conditions = append(conditions, s)
		}
	}
	if len(conditions) == 0 {
		return ""
	}
	return "Conditions on record: " + join(conditions, ", ", " and ") + "."
}

// elementMedications is §1.10.3's element 4 - the one element a representative
// can act on with no judgement at all, because "are you still filling your
// lisinopril" is a scheduling question rather than a clinical one.
//
// It reads the components (`Drug_Name`, `Maintenance`, `Fill_Count`,
// `Fill_Date`) and never the joined `Medication` value. See `Render` for why
// that is a refusal rather than a fallback.
//
// **The individual fill dates are not listed** (§1.10.4): the count and the
// latest carry the whole of what this element does, and three dates per drug is
// length for a member on four maintenance drugs.
func elementMedications(pharmacy []map[string]any, count int) (string, error) {
	if count <= 0 && len(pharmacy) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(pharmacy))
	for i, ev := range pharmacy {
		name := strings.TrimSpace(text(ev, selDrugName))
		if name == "" {
			return "", fmt.Errorf(
				"pharmacy event %d of the briefing carries no Drug_Name; the prose renderer reads the "+
					"components of a medication and never the joined Medication value, so that the "+
					"separator AT.6 writes lives in one place (I-544)", i)
		}
		var b strings.Builder
		b.WriteString(name)
		if strings.EqualFold(strings.TrimSpace(text(ev, selMaintenance)), "Y") {
			b.WriteString(", a maintenance medication")
		}
		fills, okFills := fillCount(ev)
		if okFills {
			fmt.Fprintf(&b, ", %s %s", words(fills), plural(fills, "fill"))
			if when, ok := maxDate(values(ev, selFillDate)); ok {
				fmt.Fprintf(&b, ", most recently %s", dMMMM(when))
			}
		}
		clauses = append(clauses, b.String())
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return "Medications: " + join(clauses, "; ", "; and ") + ".", nil
}

// assemble joins the paragraphs with one blank line and wraps each to Width.
//
// The result carries no trailing newline: it is the value of a column rather
// than a file.
func assemble(paragraphs []string) string {
	wrapped := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		wrapped = append(wrapped, wrap(p, Width))
	}
	return strings.Join(wrapped, "\n\n")
}
