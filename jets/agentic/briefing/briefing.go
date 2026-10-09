// Package briefing checks that a generated briefing asserts nothing its input
// entity does not support.
//
// A§8.3's second guardrail, in its own words: *no field may assert anything
// without a corresponding event in the input entity. Codes must appear in the
// input, no diagnosis outside the input set, date ranges within the observed
// span - and now also a source reference per populated field.* This package is
// that sentence made executable, and phase-5 plan §1.2 is the argument for
// building it before the projection it checks.
//
// # Why this is a rule table and not a rete session
//
// Gap 16 names the mechanism - *minimum-necessary projection + output
// validation as jetrules* - and the phrase binds two halves to one engine.
// **Only the first half can have it.** The projection runs inside the rule
// session that builds the entity, on working memory, which is what jetrules is
// for. The validation runs after the model has answered, and by then the entity
// has left the session as a serialised text literal:
//
//   - the entity is written out by `EncodeColumnData`
//     (`jets/compute_pipes/jetrules_extract_entity.go:13`) into one column, as
//     json or toon;
//   - that encoding has **no reader**. `JrColumnEncodings`
//     (`jets/compute_pipes/pipe_transformation_jetrules.go:36`) is built for
//     output channels alone (`:189`) and consumed at one site on the way out
//     (`jets/compute_pipes/jetrules_pool_worker.go:517`);
//   - a downstream rule session would receive it flat. `assertInputRow`
//     (`jets/compute_pipes/jetrules_pool_worker.go:589`) asserts one subject per
//     record and one triple per column, and `NewRdfNode`
//     (`jets/compute_pipes/jetrules_pool_worker.go:711`) has no case for a
//     nested value at all - a `map[string]any` is an error, not a subgraph.
//
// So a rete detector downstream of the model would be reasoning about an opaque
// string. Re-hydrating a serialised entity into working memory is a mechanism
// nobody has built and is larger than the check it would carry. **This is phase
// 3's N.3 arriving from the other side**: there a rule session was ruled out
// because the data lived in a table it cannot read, here because the data has
// been flattened into a literal on its way past.
//
// What is *not* ruled out is rules. Criterion 52's contrast is a rule against a
// prompt, not a rete session against a `switch`, and what this package
// evaluates is a declared table of per-field obligations that an author writes
// and a machine checks. `jets/agentic/triage`'s nine predicates and
// `jets/agentic/observe`'s two detectors are the same shape and the same
// argument.
//
// # Both operands are on one record, which is why the check is cheap
//
// The inference operator augments its input record in place and its input and
// output channels are required to share a `ChannelSpec` (`OllamaSpec`,
// `jets/compute_pipes/pipes_model.go:1259`). So the serialised entity that went
// into the prompt and the fields the response was mapped onto
// (`InferMappingSpec`, `jets/compute_pipes/pipes_model.go:1334`) are columns of
// the same row. **The check needs no join, no history and no second grain** -
// which is the property that made a rule table affordable and is worth stating
// because it is not true of anything else this project checks.
//
// # It is total, and it fails closed
//
// A populated field with no rule is a finding (`CodeNoRule`). That is the whole
// of *no field asserts anything*: a guardrail that only checks the fields
// somebody remembered to declare is one that a new field walks straight past.
// It is also what makes this package usable before `AK.2` exists - the schema
// is a document rather than a hard-coded field list, and the projection is
// built against a checker that already refuses everything it has not been told
// about.
package briefing

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

// The finding codes. They are strings rather than an enum for the reason
// wsvalidate.Finding.Code is one: a code travels into a repair prompt and out of
// an HTTP handler, and neither can hold a Go constant.
const (
	// CodeNoRule is a populated briefing field the schema says nothing about.
	// The default answer to *may this field assert this* is no.
	CodeNoRule = "briefing_field_has_no_rule"
	// CodeUngrounded is a value that appears nowhere the rule's sources select.
	CodeUngrounded = "briefing_field_not_in_input_entity"
	// CodeOutOfSpan is a date outside the observed span of the rule's sources.
	CodeOutOfSpan = "briefing_date_outside_observed_span"
	// CodeWrongCount is a count that disagrees with the entity it counts.
	CodeWrongCount = "briefing_count_disagrees_with_input_entity"
	// CodeNotDerived is a deterministic field the model was allowed to write
	// over. The disclaimer is the field this exists for: A§8.3 asks for it to be
	// *populated deterministically rather than by the model*, and nothing but a
	// comparison establishes that it was.
	CodeNotDerived = "briefing_derived_field_does_not_match_source"
	// CodeNoSpan is a within_span rule whose sources yielded no parseable date,
	// so the span it would compare against does not exist. Reported rather than
	// passed: an empty span admits every date, which is the failure mode a
	// guardrail must not have.
	CodeNoSpan = "briefing_observed_span_is_empty"
	// CodeUnparseableDate is a value a within_span rule cannot read as a date.
	CodeUnparseableDate = "briefing_date_not_parseable"
	// CodeNotANumber is a value a count_of rule cannot read as a number.
	CodeNotANumber = "briefing_count_not_a_number"
)

// Finding is one field of a briefing that its input entity does not support.
//
// It **embeds** wsvalidate.Finding rather than redefining it. The shape is
// identical - a severity, a code, a message and an RFC 6901 pointer at the thing
// - and that package imports nothing, so reusing it costs no coupling. What a
// provenance finding needs on top is `Sources`: the entity paths that were
// searched. A§8.3 calls a source reference the thing that makes an error
// *correctable rather than merely regrettable*, and a finding that says a claim
// is unsupported without saying what was consulted is not correctable.
//
// Composing rather than extending is `triage.Extent`'s move over
// `observe.Extent` (`Extent`, `jets/agentic/triage/evidence.go:25`), for the
// same reason: the embedded type is somebody else's contract and gains nothing
// from this one.
type Finding struct {
	wsvalidate.Finding
	// Sources are the entity selectors the rule consulted, verbatim from the
	// schema. Empty for CodeNoRule, which is a finding about the schema rather
	// than about the entity.
	Sources []string `json:"sources,omitempty"`
}

// Result is what Check returns: what is wrong, and what grounded the rest.
type Result struct {
	// Findings is every unsupported field, in a stable order by pointer then
	// code. Stable rather than document order, and Check's own comment says why
	// the difference is worth stating.
	Findings []Finding `json:"findings,omitempty"`
	// Refs is A§8.3's *source reference per populated field*, computed rather
	// than trusted. It maps the RFC 6901 pointer of each field that passed a
	// grounding rule to the entity pointers that grounded it.
	//
	// **Asking the model to emit its own source reference would check nothing**:
	// a model that invents a diagnosis invents a citation for it just as
	// readily, and 20 of 29 hypotheses sitting at loci triage did not find
	// present (phase-4 §19) is that failure measured. What is worth carrying
	// forward is the reference the checker resolved.
	Refs map[string][]string `json:"refs,omitempty"`
}

// OK reports whether nothing is wrong. A Result with warnings and no errors is
// not OK for this check: every finding this package emits is an Error today,
// and the severity exists so a deployment can soften one later rather than
// because one is soft now.
func (r *Result) OK() bool { return len(r.Findings) == 0 }

// Errors is the findings that block, in wsvalidate's sense.
func (r *Result) Errors() []Finding {
	out := make([]Finding, 0, len(r.Findings))
	for _, f := range r.Findings {
		if f.Severity == wsvalidate.Error {
			out = append(out, f)
		}
	}
	return out
}

// String renders a result the way a process_errors row wants it: one line per
// finding, pointer first. It is deliberately not the JSON encoding - a row-level
// error message is read by a person before it is parsed by anything.
func (r *Result) String() string {
	if r.OK() {
		return "briefing provenance: no findings"
	}
	var b strings.Builder
	for i, f := range r.Findings {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(f.Path)
		b.WriteString(": ")
		b.WriteString(f.Message)
	}
	return b.String()
}
