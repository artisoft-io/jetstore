package briefing

import (
	"fmt"
	"strconv"
	"strings"
)

// A Selector addresses a *set* of values in a decoded document, and that is the
// one way it differs from the path language beside it.
//
// `InferMappingSpec.Path` (`jets/compute_pipes/pipes_model.go:1334`) is dot
// notation where *a numeric element indicates the position in an array*, because
// a mapping writes one column and needs exactly one value. A provenance source
// needs the opposite: *codes must appear in the input* is a question about every
// event, and a selector that could name event 0 would answer it about one.
//
// So the grammar has no indices:
//
//	selector := segment ( "." segment )*
//	segment  := name [ "[]" ]
//
// `has_Medical_Events[].Diagnosis` selects the diagnosis of every medical event.
// `member.plan_id` selects one value. Both sides of a rule use it - a briefing
// field is `recent_events[].code` as readily as an entity source is - so there
// is one language to learn rather than two, and each located value carries its
// own RFC 6901 pointer for the finding to point at.
type Selector struct {
	raw      string
	segments []segment
}

type segment struct {
	name string
	each bool
}

// Located is one value a Selector resolved to, and where it was.
type Located struct {
	// Pointer is RFC 6901 into the document the selector was resolved against,
	// with the escaping wsvalidate.Finding's own doc says the validator owes:
	// "~" is "~0" and "/" is "~1", because only this package knows whether its
	// segments can carry either. An entity property is `jets:key` before
	// remove_model_prefixes has run and a briefing field is whatever the schema
	// names, so both can.
	Pointer string
	Value   any
}

// ParseSelector reads a selector, or says why it cannot.
//
// It refuses an empty segment, a numeric segment and a `[]` anywhere but at the
// end of a segment. The numeric refusal is the load-bearing one: `events.0.code`
// is valid `InferMappingSpec` notation and reads as though it works here, and
// silently selecting one event would make a membership check pass on a briefing
// that cites the wrong one.
func ParseSelector(raw string) (Selector, error) {
	if strings.TrimSpace(raw) == "" {
		return Selector{}, fmt.Errorf("a selector cannot be empty")
	}
	parts := strings.Split(raw, ".")
	segs := make([]segment, 0, len(parts))
	for _, p := range parts {
		s := segment{name: p}
		if strings.HasSuffix(p, "[]") {
			s.each = true
			s.name = strings.TrimSuffix(p, "[]")
		}
		switch {
		case s.name == "":
			return Selector{}, fmt.Errorf("selector %q has an empty segment", raw)
		case strings.Contains(s.name, "["), strings.Contains(s.name, "]"):
			return Selector{}, fmt.Errorf(
				"selector %q: %q is malformed; [] selects every element and must close a segment", raw, p)
		}
		if _, err := strconv.Atoi(s.name); err == nil {
			return Selector{}, fmt.Errorf(
				"selector %q: %q is an array index; a provenance selector addresses every element with [] "+
					"rather than one by position", raw, p)
		}
		segs = append(segs, s)
	}
	return Selector{raw: raw, segments: segs}, nil
}

// String is the selector as it was written, which is what a finding quotes.
func (s Selector) String() string { return s.raw }

// IsZero reports an unparsed selector.
func (s Selector) IsZero() bool { return len(s.segments) == 0 }

// Resolve returns every value the selector addresses in doc, each with its
// pointer. A path that is not there resolves to nothing rather than to an error:
// an absent property is a real state of an input entity, and it is the *rule*
// that decides whether nothing is a failure.
//
// **A `[]` segment over a scalar yields that scalar**, and this is not a
// convenience. `addToEntityObj`
// (`jets/compute_pipes/jetrules_extract_entity.go:99`) promotes a property to a
// slice only when a second value arrives, so a multi-valued property with one
// value is serialised as a bare scalar. Treating `[]` as *array only* would make
// every membership check fail on a patient with one event, which is the patient
// a guardrail is least entitled to be wrong about.
func (s Selector) Resolve(doc any) []Located {
	if s.IsZero() {
		return nil
	}
	return resolveFrom(s.segments, doc, "")
}

func resolveFrom(segs []segment, node any, ptr string) []Located {
	if len(segs) == 0 {
		return []Located{{Pointer: ptr, Value: node}}
	}
	obj, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	seg := segs[0]
	child, ok := obj[seg.name]
	if !ok || child == nil {
		return nil
	}
	childPtr := ptr + "/" + escapePointerToken(seg.name)
	if !seg.each {
		return resolveFrom(segs[1:], child, childPtr)
	}
	items, ok := child.([]any)
	if !ok {
		// One value, serialised without its slice. See the doc comment.
		return resolveFrom(segs[1:], child, childPtr)
	}
	out := make([]Located, 0, len(items))
	for i, item := range items {
		out = append(out, resolveFrom(segs[1:], item, childPtr+"/"+strconv.Itoa(i))...)
	}
	return out
}

// Covers reports whether this selector resolves to the given pointer in doc. It
// is how the closure check decides a populated field is spoken for, and it is
// deliberately a resolution rather than a string comparison: `events[].code`
// and `/events/2/code` are the same place and do not look alike.
func (s Selector) Covers(doc any, pointer string) bool {
	for _, l := range s.Resolve(doc) {
		if l.Pointer == pointer {
			return true
		}
	}
	return false
}

// escapePointerToken applies RFC 6901 §3. wsvalidate says in terms that its
// Finding does not escape and that the validator must, because only the
// validator knows whether its identifiers can carry "/" or "~".
func escapePointerToken(tok string) string {
	tok = strings.ReplaceAll(tok, "~", "~0")
	return strings.ReplaceAll(tok, "/", "~1")
}

// leaves walks a decoded document and returns a pointer for every scalar it
// holds, in document order for maps by sorted key so that a run is comparable
// with itself. Phase 3's F118 is why the sort is here rather than left to the
// map: a checker whose output reshuffles between runs cannot be diffed.
func leaves(node any, ptr string, out *[]Located) {
	switch v := node.(type) {
	case map[string]any:
		for _, k := range sortedKeys(v) {
			leaves(v[k], ptr+"/"+escapePointerToken(k), out)
		}
	case []any:
		for i, item := range v {
			leaves(item, ptr+"/"+strconv.Itoa(i), out)
		}
	case nil:
		// An explicit null is not a populated field, so it carries no
		// obligation. A field the model left out and a field it set to null are
		// the same absence to a reader.
	default:
		*out = append(*out, Located{Pointer: ptr, Value: v})
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// sort.Strings without the import cost of a second package in this file.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
