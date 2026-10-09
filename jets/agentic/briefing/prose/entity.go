package prose

import (
	"fmt"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// The paths this renderer reads, as `briefing.Selector`s.
//
// **The selector language is reused rather than re-implemented**, and the reason
// is one rule that is easy to get wrong twice. `addToEntityObj` promotes a
// property to a slice only when a second value arrives
// (`addToEntityObj`, `jets/compute_pipes/jetrules_extract_entity.go:112`), so a
// multi-valued property holding one value is a bare scalar and not a
// one-element list - a member with one medical event and a member with two do
// not have the same shape of map. `Selector.Resolve` already treats a `[]`
// segment over a scalar as that scalar (`Resolve`,
// `jets/agentic/briefing/selector.go:104`) and says in its own comment why: a
// traversal that insisted on an array would be wrong about exactly the patient a
// guardrail is least entitled to be wrong about. A second traversal here would
// be a second place to hold that rule.
var (
	selDisclaimer     = mustSelector("Briefing_Disclaimer")
	selConditions     = mustSelector("Condition_Summary[]")
	selEarliest       = mustSelector("Earliest_Service_Date")
	selLatest         = mustSelector("Latest_Service_Date")
	selMedicalEvents  = mustSelector("has_Briefing_Medical_Events[]")
	selPharmacyEvents = mustSelector("has_Briefing_Pharmacy_Events[]")
	selServiceDate    = mustSelector("Service_Date")
	selCareSetting    = mustSelector("Care_Setting")
	selDrugName       = mustSelector("Drug_Name")
	selMaintenance    = mustSelector("Maintenance")
	selFillCount      = mustSelector("Fill_Count")
	selFillDate       = mustSelector("Fill_Date[]")
)

// mustSelector parses a selector literal of this file. A failure is a typo in
// the line above it rather than a runtime condition, so it is raised at package
// initialisation where it cannot reach a briefing.
func mustSelector(raw string) briefing.Selector {
	s, err := briefing.ParseSelector(raw)
	if err != nil {
		panic(fmt.Sprintf("prose: selector %q does not parse: %v", raw, err))
	}
	return s
}

// values returns every value a selector addresses, in the order the entity map
// holds them. See the package comment for why that order is not sorted here.
func values(node any, sel briefing.Selector) []any {
	located := sel.Resolve(node)
	out := make([]any, 0, len(located))
	for _, l := range located {
		if l.Value != nil {
			out = append(out, l.Value)
		}
	}
	return out
}

// nodes returns the object-valued results of a selector - the medical and
// pharmacy event maps. A non-object among them is dropped rather than rendered:
// `extractAsEntity` writes a `map[string]any` for every object property, so a
// scalar there is not an event.
func nodes(node any, sel briefing.Selector) []map[string]any {
	vals := values(node, sel)
	out := make([]map[string]any, 0, len(vals))
	for _, v := range vals {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// text is the first value a selector addresses, rendered as a string, or "".
func text(node any, sel briefing.Selector) string {
	for _, v := range values(node, sel) {
		return scalar(v)
	}
	return ""
}

// date is the first value a selector addresses, read as a date.
func date(node any, sel briefing.Selector) (time.Time, bool) {
	for _, v := range values(node, sel) {
		return asDate(v)
	}
	return time.Time{}, false
}

// countOf reads a count property, falling back to the length of the list it
// counts when the property is absent.
//
// **The property wins when it is present**, and that is `AT.1`'s point rather
// than a preference: I-516 moved the four arithmetic fields into
// `briefing_projection.jr` so that the count is a fact of the projection, and a
// renderer that recomputed it would be a second answer to a question the
// projection now settles. The fallback is for an entity projected before that
// landed, where the list is the only thing that can be counted.
func countOf(entity map[string]any, prop string, listLen int) int {
	if v, ok := entity[prop]; ok {
		if n, ok := asInt(v); ok {
			return n
		}
	}
	return listLen
}

// fillCount is a pharmacy event's fill count: the projected `Fill_Count` when it
// is there, otherwise the number of `Fill_Date` values, which is the same fact
// counted rather than a different one. Both absent reports false, and the
// medication is named with no fill clause.
func fillCount(ev map[string]any) (int, bool) {
	for _, v := range values(ev, selFillCount) {
		if n, ok := asInt(v); ok {
			return n, true
		}
	}
	if dates := values(ev, selFillDate); len(dates) > 0 {
		return len(dates), true
	}
	return 0, false
}

// prefixedKey reports a root property that still carries a model prefix, which
// is the tell for `remove_model_prefixes: false` on the column.
//
// It looks at the root only. A prefixed root means the whole map is prefixed -
// `extractAsEntity` passes the flag down its own recursion - so one look is the
// whole check, and looking deeper would report the same fact several times.
func prefixedKey(entity map[string]any) string {
	for k := range entity {
		if strings.Contains(k, ":") {
			return k
		}
	}
	return ""
}

// scalar, asInt and asDate are one line each because the implementations moved.
//
// **I-563 and I-643 are resolved by [briefing.ValueText], [briefing.AsInt] and
// [briefing.AsDate]**, which this package could not call when it was written:
// the originals were unexported and `AT.3` was forbidden to edit
// `jets/agentic/briefing/`. `AY.5` is the task that was allowed to, and the
// divergence the entries predicted had already happened - the date table here
// carried five layouts and the checker's carried six, so `2025-08-14 10:00:00`
// was a date to `briefing.Check` and not to this renderer.
//
// **They stay as local names rather than being inlined at the call sites**, and
// that is a smaller change on purpose: `prose.go` and `filters.go` are the
// renderer criterion 82 compares against, and a diff that touched thirty lines
// of them to save three here would put the control arm's text in the way of
// reading what actually changed.
//
// **What is not delegated is `filters.go`'s ten functions**, which are copied
// into the engine and stay copied - that is I-647, and the argument for it is
// the opposite of this one: a filter is named by the template document, so two
// implementations that disagree are a difference criterion 82 is meant to see.
// A value reader is named by neither document.
func scalar(v any) string { return briefing.ValueText(v) }

func asInt(v any) (int, bool) { return briefing.AsInt(v) }

func asDate(v any) (time.Time, bool) { return briefing.AsDate(v) }
