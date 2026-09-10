package score

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// A FactSet is everything about one member that a briefing could be true or
// false about, read off the entity map `extractAsEntity` produces.
//
// # It reads two shapes of the same fact, and that is not a convenience
//
// A maintenance indicator reaches this package by two routes. On the **template
// arm's** entity it is a property of the pharmacy event, `Maintenance`, beside
// `Drug_Name` and `Fill_Count`. On the **model arm's** entity those four are in
// the exclusion list - `AT.5`'s design excludes the components the template
// renders from - and what remains is `Medication`, the joined string
// `Naproxen Sodium ER (maintenance Y, adherence 0.00, 5 fills: ...)` that
// `join_values` builds.
//
// **The fact set is the same fact set either way**, and a scorer that read only
// the properties would score the arm it was pointed at and refuse the other. So
// the properties win when they are present and the joined string is parsed when
// they are not, and [Drug.Source] records which route each drug came by. That is
// also what lets this package be validated against the `AT.5` artefacts at all:
// what those carry is the model arm's TOON, and the maintenance ground truth is
// inside the joined value.
//
// # What it does not carry
//
// No adherence *ratio* semantics. `F822` measured `cintel:Adherence_Ratio`
// taking exactly two values over the population, `0.00` and `1.00`, because
// `F794`'s division truncates; this package records whether a drug has a ratio
// and what it says, and makes no claim about what the number means. The
// categories that count what a model *says* about it are in checks.go and they
// count language rather than arithmetic.
type FactSet struct {
	// Drugs is one entry per pharmacy event, in entity order.
	Drugs []Drug
	// Conditions is Condition_Summary, the distinct diagnosis union the
	// projection asserts on the briefing node.
	Conditions []Condition
	// MedicalEvents and PharmacyEvents are the projected counts when the entity
	// carries them, and the list lengths otherwise - the same fallback the prose
	// renderer's countOf makes, and for the same reason.
	MedicalEvents, PharmacyEvents int
	// CareSettings and ServiceDates are the medical events, for a check that
	// wants to know whether a named setting or date is in the fact set.
	CareSettings []string
	ServiceDates []string
	// Flags is Briefing_Data_Flags: sentences of guidance the projection puts on
	// the entity. They are facts about the *prompt* rather than about the
	// member, which is I-578, and CatEchoesPrompt is the category that cares.
	Flags []string
	// Disclaimer is the intended-use notice, when the entity carries it. The
	// model arm's entity excludes it.
	Disclaimer string
	// PropertyNames is every property name in the entity map at any depth, which
	// is the closed list CatEchoesPrompt tests against.
	PropertyNames []string

	// surfaces maps a normalised surface form to the drugs that answer to it.
	// A form with more than one owner makes a match ambiguous rather than
	// arbitrated - see Finding.Ambiguous.
	surfaces map[string][]int
	// forms is surfaces' keys longest-first, so a scan takes the longest match.
	forms []string
}

// A Drug is one pharmacy event's medication as the fact set has it.
type Drug struct {
	// Name is the drug name as the projection has it, with its original case -
	// `metFORMIN HCl`, `oxyCODONE-Acetaminophen`. The case is kept because a
	// finding quotes it and a reader matching it against the artefact should see
	// what the artefact says.
	Name string
	// Maintenance is true iff the indicator is "Y".
	Maintenance bool
	// HasAdherence reports whether this drug carries an adherence ratio at all.
	// A non-maintenance drug carries none, which is what CatAdherenceAttributed
	// counts against.
	HasAdherence bool
	// Adherence is the ratio as text, "0.00" or "1.00" on this pipeline.
	Adherence string
	// Fills is the fill count, or 0 when the fact set does not say.
	Fills int
	// Source is "properties" or "joined", recording which of the two shapes this
	// drug was read from. A corpus that mixes them is a corpus whose arms had
	// different exclusion lists, which is worth being able to see.
	Source string
}

// A Condition is one Condition_Summary value, split into its code and its
// description.
type Condition struct {
	// Raw is the value as the entity holds it: `(M069) Rheumatoid arthritis,
	// unspecified`.
	Raw string
	// Code is `M069`, or "" when the value carries no parenthesised code.
	Code string
	// Description is the rest.
	Description string
	// tokens are Description's distinctive words - lower case, longer than three
	// characters, not a stop word. They are what the omission check counts, and
	// they are **words rather than substrings**, which is the one change from
	// `AT.5`'s script that moves a number: that script passed a missing
	// *rheumatoid arthritis* because the string *arthritis* occurs inside
	// *osteoarthritis*, and a word-boundary test does not.
	tokens []string
}

var (
	// selectors, parsed once. The paths are the briefing entity's, and they are
	// the same ones the prose renderer reads - see prose/entity.go, which
	// explains why the selector language is reused rather than re-implemented.
	selConditions     = mustSelector("Condition_Summary[]")
	selMedicalEvents  = mustSelector("has_Briefing_Medical_Events[]")
	selPharmacyEvents = mustSelector("has_Briefing_Pharmacy_Events[]")
	selFlags          = mustSelector("Briefing_Data_Flags[]")
	selDisclaimer     = mustSelector("Briefing_Disclaimer")

	// joinedMedication reads `join_values`' output. The adherence clause is
	// present only for a maintenance drug, which is the projection's own rule
	// and is why the group is optional here rather than defaulted.
	joinedMedication = regexp.MustCompile(
		`^(.*?)\s*\(maintenance ([YN])(?:, adherence ([0-9.]+))?(?:, (\d+) fills?:)?`)
	// conditionCode reads `(M069) Rheumatoid arthritis, unspecified`.
	conditionCode = regexp.MustCompile(`^\(([A-Za-z0-9.]+)\)\s*(.*)$`)
)

func mustSelector(raw string) briefing.Selector {
	s, err := briefing.ParseSelector(raw)
	if err != nil {
		panic(fmt.Sprintf("score: selector %q does not parse: %v", raw, err))
	}
	return s
}

// ReadFactSet builds a fact set from an entity map.
//
// An entity that carries nothing this package recognises produces an empty fact
// set rather than an error: every category then has a zero denominator and
// contributes nothing to a rate, which is the right behaviour for a member with
// no claims and the right behaviour for a caller pointing at the wrong column.
// The difference between those two is a question about the caller, not about the
// briefing, and a scorer is not the place to answer it.
func ReadFactSet(entity map[string]any) *FactSet {
	f := &FactSet{surfaces: map[string][]int{}}
	if entity == nil {
		return f
	}
	f.Disclaimer = firstText(entity, selDisclaimer)
	for _, v := range values(entity, selFlags) {
		if s := strings.TrimSpace(scalar(v)); s != "" {
			f.Flags = append(f.Flags, s)
		}
	}
	for _, v := range values(entity, selConditions) {
		raw := strings.TrimSpace(scalar(v))
		if raw == "" {
			continue
		}
		c := Condition{Raw: raw, Description: raw}
		if m := conditionCode.FindStringSubmatch(raw); m != nil {
			c.Code, c.Description = m[1], m[2]
		}
		c.tokens = distinctiveTokens(c.Description)
		f.Conditions = append(f.Conditions, c)
	}
	medical := nodes(entity, selMedicalEvents)
	for _, ev := range medical {
		if s := firstText(ev, mustSelector("Care_Setting")); s != "" {
			f.CareSettings = append(f.CareSettings, s)
		}
		if s := firstText(ev, mustSelector("Service_Date")); s != "" {
			f.ServiceDates = append(f.ServiceDates, s)
		}
	}
	pharmacy := nodes(entity, selPharmacyEvents)
	for _, ev := range pharmacy {
		if d, ok := readDrug(ev); ok {
			f.Drugs = append(f.Drugs, d)
		}
	}
	f.MedicalEvents = countOf(entity, "Medical_Event_Count", len(medical))
	f.PharmacyEvents = countOf(entity, "Pharmacy_Event_Count", len(pharmacy))
	f.PropertyNames = propertyNames(entity)
	f.indexSurfaces()
	return f
}

// readDrug reads one pharmacy event, properties first and joined value second.
// See the FactSet comment for why both.
func readDrug(ev map[string]any) (Drug, bool) {
	d := Drug{Source: "properties"}
	d.Name = strings.TrimSpace(firstText(ev, mustSelector("Drug_Name")))
	maint := strings.TrimSpace(firstText(ev, mustSelector("Maintenance")))
	joined := strings.TrimSpace(firstText(ev, mustSelector("Medication")))

	if d.Name == "" || maint == "" {
		m := joinedMedication.FindStringSubmatch(joined)
		if m == nil {
			// Neither shape. A pharmacy event with no medication is not a fact
			// about a drug, so it is dropped rather than counted as one with an
			// empty name - a drug the prose cannot possibly name would otherwise
			// inflate every drug-level denominator.
			return Drug{}, false
		}
		d.Source = "joined"
		d.Name = strings.TrimSpace(m[1])
		d.Maintenance = m[2] == "Y"
		if m[3] != "" {
			d.HasAdherence, d.Adherence = true, m[3]
		}
		if m[4] != "" {
			d.Fills, _ = strconv.Atoi(m[4])
		}
		return d, d.Name != ""
	}

	d.Maintenance = strings.EqualFold(maint, "Y")
	if a := strings.TrimSpace(firstText(ev, mustSelector("Adherence_Ratio"))); a != "" {
		d.HasAdherence, d.Adherence = true, a
	}
	if n, ok := firstInt(ev, mustSelector("Fill_Count")); ok {
		d.Fills = n
	} else {
		d.Fills = len(values(ev, mustSelector("Fill_Date[]")))
	}
	return d, true
}

// indexSurfaces builds the closed list every containment test runs over.
//
// # Three forms per drug, and the collision handling is the point
//
// A briefing does not write `metFORMIN HCl`; it writes *metformin*. So each drug
// contributes its full normalised name, a **head form** - the leading tokens up
// to the first that carries enough characters to be distinctive - and its
// longest single token when that is long enough to stand alone. `SM Calcium
// 600/Vitamin D` gives `sm calcium 600 vitamin d`, `sm calcium` and `calcium`;
// `EC-Naproxen` gives `ec naproxen` and `naproxen`.
//
// **A form two drugs answer to is kept and marked rather than dropped.** `M007`
// carries both `Vitamin D3` and `SM Calcium 600/Vitamin D`, so *vitamin* belongs
// to two drugs, and a scorer that silently picked one would be reporting its own
// tie-break as a measurement. A match on a shared form produces a finding with
// Ambiguous set, which is reported and not counted - except where every owner
// agrees, which is the one case where the ambiguity cannot change the answer.
func (f *FactSet) indexSurfaces() {
	add := func(form string, i int) {
		form = strings.TrimSpace(form)
		if form == "" {
			return
		}
		for _, have := range f.surfaces[form] {
			if have == i {
				return
			}
		}
		f.surfaces[form] = append(f.surfaces[form], i)
	}
	for i, d := range f.Drugs {
		toks := normaliseTokens(d.Name)
		if len(toks) == 0 {
			continue
		}
		add(strings.Join(toks, " "), i)
		add(strings.Join(headForm(toks), " "), i)
		if t := longestToken(toks); t != "" {
			add(t, i)
		}
	}
	f.forms = make([]string, 0, len(f.surfaces))
	for form := range f.surfaces {
		f.forms = append(f.forms, form)
	}
	// Longest first, so a scan takes the most specific form available at a
	// position: `sm calcium` beats `calcium` where both match.
	sortByLengthDesc(f.forms)
}

// HasMaintenanceDrug reports whether any drug is flagged maintenance. It is the
// condition CatMaintenanceAbsent applies under, and `AT.5`'s six members are the
// ones for which it is false.
func (f *FactSet) HasMaintenanceDrug() bool {
	for _, d := range f.Drugs {
		if d.Maintenance {
			return true
		}
	}
	return false
}

// headForm is the leading tokens of a name up to and including the first that
// makes the accumulation distinctive. `diclofenac sodium` gives `diclofenac`;
// `sm calcium 600 vitamin d` gives `sm calcium`, because `sm` alone is two
// characters and would match a great deal of English.
func headForm(toks []string) []string {
	n := 0
	for i, t := range toks {
		n += len(t)
		if n >= minDistinctive {
			return toks[:i+1]
		}
	}
	return toks
}

// minDistinctive is how many characters a surface form must carry before it is
// allowed to stand for a drug. Five is the shortest drug name in the corpus
// (`Lasix`), so anything shorter is a fragment rather than a name.
const minDistinctive = 5

func longestToken(toks []string) string {
	best := ""
	for _, t := range toks {
		if len(t) > len(best) {
			best = t
		}
	}
	if len(best) < minDistinctive+2 {
		// A single token standing for a multi-token name has to be clearly the
		// name's own word. Seven characters is where `naproxen` and `calcium`
		// sit and where `sodium` and `insulin` do not - the salts and the
		// classes are shorter than the molecules in this corpus, which is a
		// property of the corpus and is why the threshold is stated rather than
		// implied.
		return ""
	}
	return best
}

// countOf reads a projected count, falling back to the length of the list it
// counts. **The property wins when present**, which is I-516's point: the count
// is a fact of the projection after `AT.1`, and a reader that recomputed it
// would be a second answer to a settled question.
func countOf(entity map[string]any, prop string, listLen int) int {
	if v, ok := entity[prop]; ok {
		if n, ok := asInt(v); ok {
			return n
		}
	}
	return listLen
}

// propertyNames walks the entity for every property name at any depth. It is the
// closed list CatEchoesPrompt runs over, and taking it from the entity rather
// than from a literal is what makes that check decidable: the names in the
// prompt are exactly the names in the map the prompt was encoded from.
func propertyNames(node any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for k, child := range v {
				seen[k] = true
				walk(child)
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	walk(node)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sortByLengthDesc(out)
	return out
}
