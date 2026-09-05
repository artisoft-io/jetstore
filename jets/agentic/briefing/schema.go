package briefing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// Kind is what obligation a field carries. The five are A§8.3's own sentence
// split at its commas plus one escape hatch, rather than a taxonomy this package
// invented:
//
//	"Codes must appear in the input, no diagnosis outside the input set"  -> KindGrounded
//	"date ranges within the observed span"                               -> KindWithinSpan
//	"populated deterministically rather than by the model" (the disclaimer) -> KindDerived
//
// KindCountOf has no clause of its own and is here because a count is the one
// claim a membership test cannot see: *three recent visits* is ungroundable by
// asking whether "3" appears in the entity, and a briefing for a representative
// under time pressure is mostly counts and dates.
type Kind string

const (
	// KindGrounded: every value of the field must appear among the values the
	// rule's sources select.
	KindGrounded Kind = "grounded"
	// KindWithinSpan: every value must parse as a date and fall inside the span
	// of the dates the sources select.
	KindWithinSpan Kind = "within_span"
	// KindCountOf: the value must equal the number of entity items the sources
	// select.
	KindCountOf Kind = "count_of"
	// KindDerived: the field is not the model's to assert and must equal the
	// single value its source selects.
	KindDerived Kind = "derived"
	// KindUngrounded: the field is declared exempt, and `reason` is required.
	//
	// **The escape hatch is the honest part of the design and it is narrow on
	// purpose.** A guardrail with no exemption gets bypassed by turning it off;
	// one whose exemptions are a required sentence in a reviewed document gets
	// bypassed in writing. `reason` is not free text for the machine - it is the
	// thing a reviewer reads, and Validate refuses an empty one.
	KindUngrounded Kind = "ungrounded"
)

// Match is how a value is compared with an entity value, for KindGrounded.
type Match string

const (
	// MatchExact compares after trimming and case folding. The default.
	MatchExact Match = "exact"
	// MatchSubstring passes when the field's value appears inside an entity
	// value.
	//
	// **It is the weaker check and a schema opts into it per field**, because it
	// admits a field that names half a code. It exists because the one worked
	// briefing input in this tree encodes a diagnosis as
	// `(B182) Chronic viral hepatitis C`
	// (`tools/sample_projects/patient_summary_go/prompt.md`), so a briefing
	// field carrying `B182` is grounded and exact-only would call it invented.
	// Refusing to ship the mode would not make that briefing safer; it would
	// make the guardrail unusable on the only input anybody has.
	MatchSubstring Match = "substring"
)

// FieldRule is one field's obligation.
type FieldRule struct {
	Comment string `json:"comment,omitempty"`
	// Field is a selector into the briefing. It may select many values -
	// `care_gaps[].code` is one rule over every gap - and each is checked and
	// reported separately.
	Field string `json:"field"`
	Kind  Kind   `json:"kind"`
	// Sources are selectors into the input entity. More than one is a
	// disjunction: a value grounded by any source is grounded, which is what
	// "appears in the input" means when a code can arrive as a diagnosis or as a
	// procedure.
	Sources []string `json:"sources,omitempty"`
	// Match applies to KindGrounded only; MatchExact when empty.
	Match Match `json:"match,omitempty"`
	// Reason is required by KindUngrounded and refused by every other kind.
	Reason string `json:"reason,omitempty"`

	field   Selector
	sources []Selector
}

// Schema is the provenance contract for one briefing shape.
//
// It is a document rather than a Go type on purpose, and that is what lets this
// package be built before the projection it checks. `AK.2` writes a schema; it
// does not write a checker, and it cannot add a field without adding a rule
// because Check refuses a field no rule covers.
type Schema struct {
	Comment string `json:"comment,omitempty"`
	// Key names the briefing shape this schema is for. It is not read by
	// anything here and is required anyway: a provenance schema with no name is
	// one nobody can say a briefing was checked against, and *which schema*
	// belongs in the audit record beside *which model* and *which prompt
	// version* (A§8.3's last guardrail).
	Key string `json:"key"`
	// Version is the schema version, recorded for the same reason.
	Version string `json:"version,omitempty"`
	// Response is the fielded briefing's own shape, as the json schema the
	// inference operator passes to the model as `response_format`
	// (`ResponseFormat`, `jets/compute_pipes/pipes_model.go:1290`). It is
	// optional and declaring it is what turns the runtime closure into a
	// property of the briefing rather than of a run - see Cover.
	//
	// **It lives here rather than only in the pipeline config because the two
	// halves of gap 18 are one document.** A provenance schema that does not
	// know what fields the briefing has can only wait for one to arrive.
	Response json.RawMessage `json:"response_format,omitempty"`
	Rules    []FieldRule     `json:"rules"`
}

// ParseSchema decodes a provenance schema and reports what is wrong with it.
//
// It returns wsvalidate.Finding rather than this package's Finding, because a
// schema **is** a file and that is the type the repository's save path already
// speaks (`Validator`, `jets/wsvalidate/wsvalidate.go:66`). Whether it gets a
// suffix row in the per-suffix validator table is `AK.2`'s to decide, once the
// schema has a home and a suffix; this signature is what makes that a row rather
// than a rewrite.
//
// A schema with any Error finding is returned nil: a checker configured from a
// broken contract would report on a briefing it cannot have understood, which is
// worse than reporting nothing.
func ParseSchema(content string) (*Schema, []wsvalidate.Finding) {
	var s Schema
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return nil, []wsvalidate.Finding{{
			Severity: wsvalidate.Error,
			Code:     "briefing_schema_not_readable",
			Message:  fmt.Sprintf("the provenance schema could not be read: %v", err),
		}}
	}
	findings := s.compile()
	for _, f := range findings {
		if f.Severity == wsvalidate.Error {
			return nil, findings
		}
	}
	// Cover runs here rather than in compile() because it is a check on the
	// document and compile() is on Check's per-record path. A schema whose rules
	// do not compile is not asked whether they cover the briefing: the answer
	// would name fields whose selectors never parsed.
	findings = append(findings, s.Cover()...)
	for _, f := range findings {
		if f.Severity == wsvalidate.Error {
			return nil, findings
		}
	}
	return &s, findings
}

// compile parses every selector and checks each rule against its kind. It is
// separate from ParseSchema so that a Schema built in Go - by a test, or by
// AK.2's generator before it has a file to write - goes through the same gate as
// one read from disk.
func (s *Schema) compile() []wsvalidate.Finding {
	var out []wsvalidate.Finding
	add := func(code, msg, path string) {
		out = append(out, wsvalidate.Finding{
			Severity: wsvalidate.Error, Code: code, Message: msg, Path: path,
		})
	}
	if strings.TrimSpace(s.Key) == "" {
		add("briefing_schema_has_no_key", "a provenance schema must name the briefing shape it is for", "/key")
	}
	if len(s.Rules) == 0 {
		add("briefing_schema_has_no_rules",
			"a provenance schema with no rules refuses every populated field; declare the fields or do not install it",
			"/rules")
	}
	seen := map[string]int{}
	for i := range s.Rules {
		r := &s.Rules[i]
		path := fmt.Sprintf("/rules/%d", i)
		if prev, dup := seen[r.Field]; dup {
			add("briefing_schema_duplicate_field",
				fmt.Sprintf("field %q already has a rule at /rules/%d; two rules over one field is an ambiguity rather than a conjunction",
					r.Field, prev),
				path+"/field")
		} else if r.Field != "" {
			seen[r.Field] = i
		}
		sel, err := ParseSelector(r.Field)
		if err != nil {
			add("briefing_schema_bad_field_selector", err.Error(), path+"/field")
		}
		r.field = sel
		r.sources = r.sources[:0]
		for j, raw := range r.Sources {
			ssel, err := ParseSelector(raw)
			if err != nil {
				add("briefing_schema_bad_source_selector", err.Error(), fmt.Sprintf("%s/sources/%d", path, j))
				continue
			}
			r.sources = append(r.sources, ssel)
		}
		switch r.Kind {
		case KindGrounded, KindWithinSpan, KindCountOf:
			if len(r.Sources) == 0 {
				add("briefing_schema_rule_has_no_source",
					fmt.Sprintf("a %s rule must name at least one source in the input entity", r.Kind),
					path+"/sources")
			}
		case KindDerived:
			if len(r.Sources) != 1 {
				add("briefing_schema_rule_has_no_source",
					"a derived rule names exactly one source: the value the projection put there, which the field must equal",
					path+"/sources")
			}
		case KindUngrounded:
			if strings.TrimSpace(r.Reason) == "" {
				add("briefing_schema_exemption_has_no_reason",
					"an ungrounded field must say why in `reason`; an exemption nobody has to write down is a guardrail nobody has to argue with",
					path+"/reason")
			}
			if len(r.Sources) > 0 {
				add("briefing_schema_exemption_has_sources",
					"an ungrounded field is exempt from grounding, so its sources would never be consulted",
					path+"/sources")
			}
		case "":
			add("briefing_schema_rule_has_no_kind",
				fmt.Sprintf("rule for %q has no kind; the kinds are grounded, within_span, count_of, derived, ungrounded", r.Field),
				path+"/kind")
		default:
			add("briefing_schema_unknown_kind",
				fmt.Sprintf("unknown kind %q; the kinds are grounded, within_span, count_of, derived, ungrounded", r.Kind),
				path+"/kind")
		}
		if r.Kind != KindUngrounded && strings.TrimSpace(r.Reason) != "" {
			add("briefing_schema_reason_on_grounded_rule",
				fmt.Sprintf("`reason` states why a field is exempt and this rule is %s; a checked field's rationale belongs in `comment`", r.Kind),
				path+"/reason")
		}
		switch r.Match {
		case "", MatchExact, MatchSubstring:
		default:
			add("briefing_schema_unknown_match",
				fmt.Sprintf("unknown match %q; the modes are exact and substring", r.Match),
				path+"/match")
		}
		if r.Match != "" && r.Kind != KindGrounded {
			add("briefing_schema_match_on_wrong_kind",
				fmt.Sprintf("`match` decides how a value is compared with the entity and only a grounded rule compares; this rule is %s", r.Kind),
				path+"/match")
		}
	}
	return out
}

// Compile prepares a Schema built in Go for Check, and reports the same findings
// ParseSchema would. Check calls it, so a caller that has already compiled pays
// only for the selector parses.
func (s *Schema) Compile() []wsvalidate.Finding { return s.compile() }
