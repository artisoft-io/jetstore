package template

import (
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// The three readings of a document value - as text, as a number, as a date -
// delegated to `jets/agentic/briefing`, which this package already imports for
// [briefing.Selector].
//
// **This file held the third copy and `I-643` is why it no longer does.** The
// copy was made because `AY.2` was forbidden to edit `jets/agentic/briefing`;
// `AY.5` was not, and the entry's own prediction had already come true - the
// date table here and in `jets/agentic/briefing/prose` carried five layouts and
// the one in `jets/agentic/briefing/check.go` carried six, so `2025-08-14
// 10:00:00` was a date to a checker and not to a renderer.
//
// # This is not a domain dependency, and criterion 81 is the reason to say so
//
// Criterion 81 asks that the engine name nothing about briefings. It names
// nothing here either: what it takes from that package is a path syntax and
// four readings of a scalar, none of which mentions a member, a claim or a
// medication. **The import edge already existed** - `chain.go` resolves a path
// with [briefing.ParseSelector] - so nothing about the engine's shape changes,
// and the alternative was a fourth copy of a date table.
//
// # What delegating does to criterion 82, which is the question `I-647` raises
//
// Nothing, and that is the argument rather than the hope. Criterion 82 compares
// `prose.Render` with a rendering of the template document, and its subject is
// **the document**: the elements, their order, the predicates and the filters
// they name. These four functions are named by no document and were byte
// identical in both arms, so an equivalence test could never have separated
// them - two identical copies fail together. What can still separate the arms
// is `filters.go`'s ten functions, which stay copied (`I-647`), and what
// catches a shared reader whose behaviour changed is the golden on each side:
// `TestRendersSection1106Verbatim` and `TestTheAnchorFixture` both hold plan
// §10.9's printed block byte for byte.

func scalar(v any) string { return briefing.ValueText(v) }

func asInt(v any) (int, bool) { return briefing.AsInt(v) }

func asFloat(v any) (float64, bool) { return briefing.AsFloat(v) }

func asDate(v any) (time.Time, bool) { return briefing.AsDate(v) }

// asNode reads a value as a node - an object a path can be addressed into, or
// that `with` and `each` can scope a span to.
//
// It stays here rather than moving with the four above: a node is a structural
// reading that the engine's own traversal defines, not a reading of a scalar,
// and it was never duplicated.
//
// A non-object is not a node rather than being coerced into one: `extractAsEntity`
// writes a `map[string]any` for every object property, so a scalar where a node
// is expected is not a node that happens to be empty.
func asNode(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}
