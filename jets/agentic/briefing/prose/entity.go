package prose

import (
	"fmt"
	"strconv"
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

// scalar renders one entity value as text.
//
// The three shapes it must accept are the three the entity can arrive in. From
// `extractAsEntity` a value is what `GetRdfNodeValue` returns - a string, an
// int, a uint, a float64 or a `time.Time` (`GetRdfNodeValue`,
// `jets/compute_pipes/jetrules_interface.go:96`). From a decoded json or toon
// document every number is a float64 and every date is text. Both are rendered
// the same way here.
func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format("2006-01-02")
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// asInt reads a count. A float is truncated rather than rounded, because toon
// and json return every number as a float64 and a count that arrives as 2.0 is
// the integer 2.
func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case uint:
		return int(x), true
	case uint32:
		return int(x), true
	case uint64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	default:
		return 0, false
	}
}

// dateLayouts is what a date can look like on the way in.
//
// A date reaches this renderer as a `time.Time` when it comes straight from
// `extractAsEntity`, which is the wiring `EncodeColumnData` uses, and as text
// when the entity has been through json or toon. The padded pair is what the
// encoder writes; the unpadded pair is what the one hand-written briefing input
// in the tree carries, and `jets/agentic/briefing`'s own checker accepts both
// for that reason (`dateLayouts`, `jets/agentic/briefing/check.go:299`).
//
// **This is a second copy of that table and it is one this package could not
// avoid**: the original is unexported and `AT.3` may not edit files under
// `jets/agentic/briefing/`. Recorded as I-563.
var dateLayouts = []string{
	"2006-01-02",
	"2006-01-02T15:04:05",
	time.RFC3339,
	"2006-1-2",
	"2006-1-2T15:04:05",
}

func asDate(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x, true
	case *time.Time:
		if x == nil {
			return time.Time{}, false
		}
		return *x, true
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return time.Time{}, false
		}
		for _, layout := range dateLayouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}
