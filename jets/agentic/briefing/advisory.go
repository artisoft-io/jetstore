package briefing

import (
	"fmt"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// A§8.3's first guardrail, in its own words: *the briefing must carry no
// imperative or advisory language - nothing a representative could relay as
// guidance. Do not rely on a prompt instruction alone; make it checkable.*
//
// # The check is structural, and that is the whole design
//
// The obvious implementation is a lexicon of modal verbs and imperative markers
// applied to every field of the briefing. It is the wrong one here, and the
// reason is measured rather than asserted: the values a briefing legitimately
// carries are clinical descriptions copied out of the workspace's own lookup
// tables, and a lexicon over those flags a large number of them. `Aftercare
// following surgery`, `Encounter for screening`, `Continuous positive airway
// pressure` and `Use of tobacco` are diagnoses, not advice, and a guardrail that
// calls them advice is one somebody switches off.
//
// So the question this file asks is not *does this string sound like advice*
// but **can this field carry a string the model wrote at all**. Every rule kind
// but one bounds a field's value to something the input entity or the projection
// already supplied:
//
//   - `grounded` - the value must equal, or be a substring of, a value the
//     sources select. `matches` compares `strings.Contains(entityValue,
//     briefingValue)`, so the **briefing's** value is the narrower one even in
//     substring mode: the model may pick which entity value to quote and not
//     what to say around it.
//   - `count_of` - the value must read as a number.
//   - `within_span` - the value must parse as a date.
//   - `derived` - the value must equal the single value its source selects.
//   - `ungrounded` - **nothing bounds it.** This is the one kind whose values are
//     the model's own prose, and it is the only place the lexicon runs.
//
// `Confine` reports the fields of that last kind when the document is saved, and
// `applyRule` scans their values when a briefing is checked. The delivered
// `patient_briefing.pv.json` has none, so the exposure is 0 by construction
// rather than 0 by a rate - which is `AK.2`'s move on `match: substring` applied
// to the guardrail rather than to the projection.
//
// # What still gets past it
//
// A closed field carries whatever the claim data carries. If a diagnosis
// description in the lookup were phrased as guidance, a briefing quoting it
// verbatim would be grounded and would pass. That is the residual and it is the
// right one to accept: A§8.3 forbids *nothing a representative could relay as
// guidance* in the sense of guidance the briefing produced, and a description
// copied character for character out of a claim is the claim speaking.
const (
	// CodeFieldAdmitsProse is a field the briefing can carry whose rule bounds
	// nothing, so its value is whatever the model wrote. A Warning rather than
	// an Error: `ungrounded` is a declared exemption with a required reason, and
	// refusing it here would delete `AK.1`'s escape hatch on `AK.3`'s authority.
	CodeFieldAdmitsProse = "briefing_field_admits_free_text"
	// CodeAdvisoryLanguage is a value on such a field that reads as guidance.
	CodeAdvisoryLanguage = "briefing_field_carries_advisory_language"
	// CodeNoDisclaimer is a briefing contract that declares the briefing's shape
	// and carries no intended-use notice.
	CodeNoDisclaimer = "briefing_has_no_disclaimer"
	// CodeDisclaimerHasNoField is a disclaimer block naming no record field.
	CodeDisclaimerHasNoField = "briefing_disclaimer_names_no_field"
	// CodeDisclaimerIsModelWritten is a disclaimer whose field the model writes.
	CodeDisclaimerIsModelWritten = "briefing_disclaimer_is_the_models_to_write"
)

// Disclaimer is A§8.3's intended-use notice, declared rather than duplicated.
//
// A§8.3: *carry the intended-use and not-medical-advice notice as a **required
// field in the output record**, populated deterministically rather than by the
// model. A banner rendered in a screen is lost the moment the text is copied,
// exported to a CRM, or read aloud; a field travels with the record.* Decision 5
// says the same thing from the commercial side: *the artefact carries a
// disclaimer field and the boundary is contractual.*
//
// # It names the field and does not carry the text
//
// The notice itself is asserted by the projection -
// `workspaces/jets_ws/jet_rules/clinical_intel/briefing_projection.jr` - onto a
// data property of the briefing class, which is what makes it a column of the
// record. **Putting the text here as well would be a second copy nothing
// compares**, which is exactly the seam `I-438` records for the response schema;
// adding a second instance of a defect while fixing something else is not a
// trade worth making. What this block declares is the *field*, and what
// `Confine` asserts about it is the part a document can assert: that the model
// is not the one writing it.
//
// # Why the model must not write it, rather than write it and be checked
//
// `KindDerived` exists for a field the model writes and a rule compares against
// a deterministic source, and the disclaimer looks like its case. It is not, for
// three reasons and the third is the one that decides it:
//
//   - a model that omits the field produces a briefing with no notice, and only
//     the check would catch it; a column asserted by a rule cannot be omitted;
//   - a constant legal sentence costs prompt and response tokens on every
//     record, against A§8.2's measurement that prompt tokens dominate this
//     workload roughly 9:1 - the arithmetic the projection is itself argued on;
//   - **the notice is the one string in the artefact that must carry imperative
//     language.** *It must not be used to make a clinical decision* is advice
//     about advice. Making it a briefing field would put a permanent
//     advisory-language exemption inside the document this file exists to
//     guard, and an exemption is where a guardrail goes to be bypassed.
type Disclaimer struct {
	Comment string `json:"comment,omitempty"`
	// Field is the record field the notice is carried on, as the domain model
	// names it - `cintel:Briefing_Disclaimer`. It is compared against the
	// briefing's own field names, so a schema that declares the notice as
	// something the model returns is refused.
	Field string `json:"field"`
}

// Confine checks that the briefing's contract leaves the model no room to write
// guidance, and that the artefact carries its notice.
//
// It is `Cover`'s sibling and runs beside it in `ParseSchema`: both are checks
// on the document rather than on a record, both are answered once when somebody
// saves the file, and both are about what the briefing *can* be rather than what
// one briefing was. `Cover` asks whether every field has a rule; `Confine` asks
// what those rules bound.
//
// A schema with no `response_format` is not asked either question - the briefing
// shape is undeclared, so there is no field set to reason about. That is the
// shape `AK.1` shipped and it keeps working.
func (s *Schema) Confine() []wsvalidate.Finding {
	if len(s.Response) == 0 {
		return nil
	}
	var out []wsvalidate.Finding

	// The notice. Required once the briefing's shape is declared, because a
	// declared shape is a delivered artefact and A§8.3 makes the notice part of
	// it. There is deliberately no way to turn this off: a guardrail with a
	// switch is bypassed by throwing the switch, which is the argument
	// `KindUngrounded`'s required `reason` is built on one level down.
	leaves, walk := responseLeaves(s.Response)
	if len(wsvalidate.ErrorsOnly(walk)) > 0 {
		// Cover has already reported the walk failures and the leaf set is
		// incomplete, so anything said from here would be about fields the walk
		// never reached.
		return nil
	}
	inResponse := make(map[string]bool, len(leaves))
	for _, leaf := range leaves {
		inResponse[leaf] = true
	}

	switch {
	case s.Disclaimer == nil:
		out = append(out, wsvalidate.Finding{
			Severity: wsvalidate.Error,
			Code:     CodeNoDisclaimer,
			Path:     "/disclaimer",
			Message: "this schema declares the shape of a briefing and no intended-use notice; " +
				"A section 8.3 carries the not-medical-advice notice as a required field of the output record, " +
				"because a banner is lost the moment the text is copied and a field travels with the record",
		})
	case strings.TrimSpace(s.Disclaimer.Field) == "":
		out = append(out, wsvalidate.Finding{
			Severity: wsvalidate.Error,
			Code:     CodeDisclaimerHasNoField,
			Path:     "/disclaimer/field",
			Message: "the disclaimer must name the record field it is carried on; a notice with no field " +
				"is a notice nobody can point at in the artefact that left the building",
		})
	default:
		field := strings.TrimSpace(s.Disclaimer.Field)
		if inResponse[field] {
			out = append(out, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     CodeDisclaimerIsModelWritten,
				Path:     "/disclaimer/field",
				Message: fmt.Sprintf(
					"the disclaimer names %q and the briefing's response_format can produce it, so the model "+
						"writes the notice; A section 8.3 asks for it to be populated deterministically, and the "+
						"notice is the one string here that must carry imperative language - a model field for it "+
						"is a standing advisory-language exemption inside the document this check guards", field),
			})
		}
		for i := range s.Rules {
			if strings.TrimSpace(s.Rules[i].Field) != field {
				continue
			}
			out = append(out, wsvalidate.Finding{
				Severity: wsvalidate.Error,
				Code:     CodeDisclaimerIsModelWritten,
				Path:     fmt.Sprintf("/rules/%d/field", i),
				Message: fmt.Sprintf(
					"there is a provenance rule for %q and that field is the disclaimer; a rule checks what the "+
						"model asserted, and the notice is not the model's to assert", field),
			})
		}
	}

	// The prose surfaces: fields the briefing can carry whose rule bounds
	// nothing. Reported by field so that the count is readable as a count.
	for _, leaf := range leaves {
		r := s.ruleFor(leaf)
		if r == nil {
			// Cover reports an uncovered leaf. Saying it twice in two
			// vocabularies would make a reader look for two defects.
			continue
		}
		if r.Kind.confines() {
			continue
		}
		out = append(out, wsvalidate.Finding{
			Severity: wsvalidate.Warning,
			Code:     CodeFieldAdmitsProse,
			Path:     "/rules",
			Message: fmt.Sprintf(
				"the briefing can carry %q and its rule is %s, which bounds the value to nothing; "+
					"this field is where the model writes its own prose, and it is scanned for imperative "+
					"and advisory language on every record rather than trusted", leaf, r.Kind),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Message < out[j].Message
	})
	return out
}

// ruleFor returns the rule covering a canonical leaf selector, or nil.
func (s *Schema) ruleFor(leaf string) *FieldRule {
	for i := range s.Rules {
		sel, err := ParseSelector(s.Rules[i].Field)
		if err != nil {
			continue
		}
		if sel.Canonical() == leaf {
			return &s.Rules[i]
		}
	}
	return nil
}

// confines reports whether a kind bounds a field's value to something the input
// entity or the projection supplied. Everything but the declared exemption does.
func (k Kind) confines() bool {
	switch k {
	case KindGrounded, KindWithinSpan, KindCountOf, KindDerived:
		return true
	}
	return false
}

// ProseSurfaces is the fields of a briefing whose values are the model's own
// prose, in the order Cover reports leaves.
//
// It exists so the count can be stated rather than inferred from a finding list:
// **the delivered contract has none**, and a number that is quoted in a plan
// section should come from the same function the guardrail uses.
func (s *Schema) ProseSurfaces() []string {
	if len(s.Response) == 0 {
		return nil
	}
	leaves, walk := responseLeaves(s.Response)
	if len(wsvalidate.ErrorsOnly(walk)) > 0 {
		return nil
	}
	var out []string
	for _, leaf := range leaves {
		if r := s.ruleFor(leaf); r != nil && !r.Kind.confines() {
			out = append(out, leaf)
		}
	}
	return out
}

// advisoryMarker reports the first imperative or advisory marker in a value.
//
// # Two families, and only one of them is position-free
//
// **Directive phrases** are guidance wherever they appear: a modal of
// obligation, a recommendation verb, or a second-person instruction. They are
// matched on word boundaries, so `Continuous` does not match `continue` and
// `Application` does not match `apply` - which is not a nicety, because those
// two spellings are common in the procedure vocabulary and the untrimmed forms
// would flag thousands of legitimate descriptions.
//
// **Bare imperative verbs** are guidance only at the start of a clause. `Take`
// opens an instruction and closes nothing in `Intake of...`; `use` is a verb in
// `Use two tablets` and a noun in `Use of tobacco`. So the imperative family is
// tested against the first word of the value and of each sentence inside it,
// and against nothing else.
//
// # It is not asked to be right about English
//
// It runs on the fields nothing else bounds, which the delivered contract has
// none of, and a false positive there costs a finding on a field somebody
// declared as free text with a written reason. That is the trade this design
// buys by being structural first: a lexicon whose blast radius is the declared
// exemptions can afford to be blunt, and one applied to every field cannot.
func advisoryMarker(v any) (string, bool) {
	s := normalise(v)
	if s == "" {
		return "", false
	}
	for _, phrase := range directivePhrases {
		if containsWord(s, phrase) {
			return phrase, true
		}
	}
	for _, clause := range clauses(s) {
		w := firstWord(clause)
		if w == "" {
			continue
		}
		if imperativeVerbs[w] {
			return w, true
		}
	}
	return "", false
}

// directivePhrases are guidance wherever they appear in a value. Modals of
// obligation first, then the verbs of recommendation, then the second-person
// forms a briefing read aloud would turn into an instruction.
var directivePhrases = []string{
	"should", "shouldn't", "must", "mustn't", "ought to", "need to", "needs to",
	"have to", "has to", "supposed to",
	"recommend", "recommends", "recommended", "recommendation", "recommendations",
	"advise", "advises", "advised", "advice", "advisable",
	"suggest", "suggests", "suggested", "urge", "urges", "urged",
	"encourage", "encourages", "encouraged",
	"be sure to", "make sure", "remember to", "please",
	"next steps", "action item", "action items", "care plan", "treatment plan",
	"plan of care", "follow-up plan", "guidance",
}

// imperativeVerbs open an instruction when they open a clause.
var imperativeVerbs = map[string]bool{
	"start": true, "stop": true, "begin": true, "take": true, "use": true,
	"avoid": true, "ensure": true, "schedule": true, "call": true, "contact": true,
	"refer": true, "monitor": true, "discuss": true, "ask": true, "check": true,
	"increase": true, "decrease": true, "continue": true, "discontinue": true,
	"switch": true, "prescribe": true, "administer": true, "consider": true,
	"review": true, "verify": true, "confirm": true, "remind": true, "offer": true,
	"apply": true, "keep": true, "try": true, "do": true, "don't": true,
	"advise": true, "encourage": true, "suggest": true, "recommend": true,
	"follow": true, "arrange": true, "book": true, "tell": true, "let": true,
}

// clauses splits a normalised value at the punctuation a sentence or a list item
// ends on. The value itself is the first clause, which is what catches a
// one-sentence field with no terminator at all.
func clauses(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '.', ';', ':', '!', '?', '\n', '\r':
			return true
		}
		return false
	})
}

func firstWord(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `-*,"'()[]`)
}

// containsWord is a word-boundary search over a normalised string. A phrase may
// contain spaces, in which case its own words are the boundary.
func containsWord(haystack, needle string) bool {
	from := 0
	for {
		i := strings.Index(haystack[from:], needle)
		if i < 0 {
			return false
		}
		i += from
		before := byte(' ')
		if i > 0 {
			before = haystack[i-1]
		}
		after := byte(' ')
		if end := i + len(needle); end < len(haystack) {
			after = haystack[end]
		}
		if !isWordByte(before) && !isWordByte(after) {
			return true
		}
		from = i + 1
	}
}

func isWordByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	}
	return b == '\''
}
