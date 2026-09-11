package template

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The three readings of a document value: as text, as a number, as a date.
//
// **This is a third copy of `dateLayouts` and the first two are I-563.** The
// original is unexported in `jets/agentic/briefing` and the second copy is
// unexported in `jets/agentic/briefing/prose`; this package may not import
// either. Copying is what the visibility leaves, and the alternative worth
// having is one exported reader in `jets/agentic/briefing` that all three call
// - which is a change to a package `AY.2` is told not to touch, so it is
// recorded as I-643 rather than made here.

// scalar renders one document value as text.
//
// The shapes it must accept are the shapes a value can arrive in. From
// `extractAsEntity` a value is what `GetRdfNodeValue` returns - a string, an
// int, a uint, a float64 or a `time.Time` (`GetRdfNodeValue`,
// `jets/compute_pipes/jetrules_interface.go:96`). From a decoded json or toon
// document every number is a float64 and every date is text. Both render the
// same way here, which is what makes a template's output independent of how its
// input was encoded.
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

// asFloat reads a value for the four ordering operators of a predicate.
//
// **It is a second reader rather than `asInt` reused**, because a predicate
// compares and does not count: `Adherence_Ratio >= 0.8` is a sensible thing for
// a template to ask and truncating it to 0 would answer a different question.
// `asInt`'s truncation is right where the value is a count and wrong here.
func asFloat(v any) (float64, bool) {
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
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// dateLayouts is what a date can look like on the way in. The padded pair is
// what the entity encoder writes; the unpadded pair is what a hand-written
// document carries, and `jets/agentic/briefing`'s own checker accepts both for
// that reason (`dateLayouts`, `jets/agentic/briefing/check.go:299`).
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

// asNode reads a value as a node - an object a path can be addressed into, or
// that `with` and `each` can scope a span to.
//
// A non-object is not a node rather than being coerced into one: `extractAsEntity`
// writes a `map[string]any` for every object property, so a scalar where a node
// is expected is not a node that happens to be empty.
func asNode(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}
