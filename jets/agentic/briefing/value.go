package briefing

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The three readings of a briefing value - as text, as a number, as a date -
// and this file exists because there were three copies of them.
//
// # Why they are here rather than in each reader
//
// A second copy and then a third were recorded: `scalar`, `asInt`,
// `asDate` and `dateLayouts` were written out in `jets/agentic/briefing`,
// `jets/agentic/briefing/prose` and `jets/agentic/template`, each time because
// the package that needed them could not import the package that had them. The
// two later copies were made under a rule forbidding an edit to this package,
// so the duplication is a record of task boundaries rather than a design.
//
// **The date table is the one that mattered, and it had already diverged.**
// `dateLayouts` decides what a date can look like on the way in; this package's
// copy carried six layouts and the other two carried five. So a date written
// `2025-08-14 10:00:00` was a date to [Check] and not to either renderer: the
// renderer's `asDate` returns false, the clause is omitted, and the briefing
// says less than the entity does with nothing reporting it. **No briefing is
// known to have carried one** - the engine writes a date as `2025-8-14` and a
// datetime with a `T` (see [dateLayouts]) - so this is the divergence rather
// than an incident, and it was in the tree before anybody edited a copy, which
// is what the copies were predicted to do.
//
// **The union is what is exported, not the intersection.** Narrowing the
// checker to the renderers' five would refuse an input it accepts today, which
// is a regression in a guardrail to buy tidiness; widening the renderers to six
// only lets them render a date they used to drop. The `*time.Time` arm is the
// renderers' and is kept for the same reason.
//
// # What this does not unify
//
// `jets/agentic/briefing/score` has its own `scalar` and `asInt` and they are
// **not** delegated here (`scalar`, `jets/agentic/briefing/score/text.go:611`).
// Its default arm returns `""` where [ValueText] returns `fmt.Sprintf("%v", x)`,
// so a non-scalar reaches a scorer's word count as nothing rather than as
// `map[...]`; that is a difference a scoring change should be measured against
// rather than absorbed by a refactor whose subject is duplication.
//
// And [normalise] is not a fourth copy of [ValueText]. It renders for
// *comparison* - folded, whitespace-collapsed - which is a different question
// from what a briefing prints.

// ValueText renders one briefing value as text.
//
// The shapes it must accept are the shapes a value can arrive in. From
// `extractAsEntity` a value is what `GetRdfNodeValue` returns - a string, an
// int, a uint, a float64 or a `time.Time` (`GetRdfNodeValue`,
// `jets/compute_pipes/jetrules_interface.go:161`). From a decoded json or toon
// document every number is a float64 and every date is text. Both render the
// same way here, which is what makes a template's output independent of how its
// input was encoded - and is what `TestDecodedDocumentTypesRenderTheSame`
// asserts on both sides of criterion 82.
func ValueText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case time.Time:
		return x.Format(dateOnly)
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

// AsInt reads a count. A float is truncated rather than rounded, because toon
// and json return every number as a float64 and a count that arrives as 2.0 is
// the integer 2.
func AsInt(v any) (int, bool) {
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
	case json.Number:
		n, err := strconv.Atoi(x.String())
		return n, err == nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	default:
		return 0, false
	}
}

// AsFloat reads a value for a comparison rather than for a count.
//
// **It is a second reader rather than [AsInt] reused**, because a predicate
// compares and does not count: `Adherence_Ratio >= 0.8` is a sensible thing for
// a template to ask and truncating it to 0 would answer a different question.
// [AsInt]'s truncation is right where the value is a count and wrong here.
//
// The `json.Number` arm is this package's checker's, which reads a decoded
// briefing document; the wider integer arms are the engine's.
func AsFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

const dateOnly = "2006-01-02"

// dateLayouts is what a date can look like on the way in.
//
// **The unpadded date is what the entity writer emits.** On the C++ engine, the
// only rule engine, `extractAsEntity` reads a literal through `GetRdfNodeValue`
// (`jets/compute_pipes/jetrules_interface.go`), which passes a string through,
// and the native node's value is `bridge.Resource.AsInterface("")`
// (`jets/bridge/bridge.go`): a date comes back as `fmt.Sprintf("%d-%d-%d")`
// text, `2025-8-14`, and a datetime as the engine's ISO text,
// `2024-03-15T10:20:30.123456`. So the `2006-1-2` layout is the one a date in
// an entity map is read by, and `prose/pipeline_test.go` pins that text. A
// datetime is read by the padded `2006-01-02T15:04:05`, because `time.Parse`
// accepts a fractional second the layout does not name.
//
// The padded `2006-01-02` is what a `time.Time` becomes in the toon arm of
// `EncodeColumnData` (`jets/compute_pipes/jetrules_extract_entity.go`), whose
// time formatter writes it, and the hand-built maps in this package's tests
// carry `time.Time`. No value from the C++ engine is a `time.Time`, so in
// production that formatter does not fire. Both forms are accepted, and the tree's one worked briefing input,
// `tools/sample_projects/patient_summary_go/prompt.md`, carries the unpadded
// one too (`2025-8-14`, `2025-1-17`). Until 2026-11 this comment said the
// padded pair was what the writer emits, which held only for the toon arm on
// the retired Go engine, whose dates reached the map as `time.Time`.
//
// **The space-separated variant is the sixth and no comment has ever explained
// it.** It was in this package's table and in neither of the two copies, which
// is how the divergence [ValueText]'s file comment describes came to be. It is
// kept because dropping it would narrow what [Check] accepts today, and
// a guardrail is the wrong place to pay for tidiness.
var dateLayouts = []string{
	dateOnly,
	"2006-01-02T15:04:05",
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-1-2",
	"2006-1-2T15:04:05",
}

// AsDate reads a value as a date, by the layouts above.
//
// A blank string is refused before the layouts are tried rather than by them.
// The loop would reach the same answer; saying it here is what makes *a
// property that is there and says nothing is not a property that is there*
// - the rule for `require` - one line of code rather than a convention.
func AsDate(v any) (time.Time, bool) {
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
