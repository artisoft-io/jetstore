package template

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// The ten constructs of plan §10.5, one entry each: `words`, `sentence`,
// `plural`, `d_MMMM`, `d_MMMM_yyyy`, `latest_by`, `strip_code`, `join`, `count`
// and `max`. **There is no eleventh, and adding one is R-107 firing** - the
// risk is not that a construct is wrong but that the notation grows one per
// sentence until it is `prose.go` with a parser in front of it.
//
// **Every one is total**, and where totality costs something the entry says
// here what it does instead of failing. That is what makes criterion 84 a
// property of this table rather than a claim about the package: a construct
// that could fail would be a run-time failure the compiler cannot see.
//
// The functions are `jets/agentic/briefing/prose/filters.go`'s, and nine of the
// ten are transcribed. `join` is the exception and it is I-624: `elementConditions`
// filters empty values before calling `join` and `join` itself does not skip
// them (`elementConditions`, `jets/agentic/briefing/prose/prose.go:256`), so
// folding the filter into `join` is what lets a conditions element be one chain
// rather than a chain plus a rule about what precedes it.

// shape is what a construct does to the *list* a chain carries, and it is the
// column that makes a chain checkable at compile time (§10.5).
type shape int

const (
	// shapeFirst takes the first value the chain yields and discards the rest,
	// which is `text`'s behaviour in the package this one generalises (`text`,
	// `jets/agentic/briefing/prose/entity.go:81`). A substitution is not a list
	// operation.
	shapeFirst shape = iota
	// shapeMap applies to every value and yields a list of the same length.
	shapeMap
	// shapeFold consumes the list and yields one thing, or nothing.
	shapeFold
)

func (s shape) String() string {
	switch s {
	case shapeFirst:
		return "first"
	case shapeMap:
		return "map"
	default:
		return "fold"
	}
}

// valueKind is what a chain is carrying at a point in the chain. It is a
// compile-time judgement over a run-time value, so `kindUnknown` is not a
// failure state: a path can address anything and says nothing about what it
// found, which is why C9 refuses only a *contradiction* and never an absence of
// information.
type valueKind int

const (
	kindUnknown valueKind = iota
	kindText
	kindNumber
	kindDate
	kindNode
	kindAny // what `count` takes: a list of anything at all
)

func (k valueKind) String() string {
	switch k {
	case kindText:
		return "text"
	case kindNumber:
		return "a number"
	case kindDate:
		return "a date"
	case kindNode:
		return "a node"
	case kindAny:
		return "anything"
	default:
		return "an unknown kind"
	}
}

// accepts reports whether a construct taking `want` may be applied to a chain
// carrying `got`.
//
// **Text is the one direction that widens, and the reason is `scalar`.** Every
// value in a document renders as text, so `strip_code` over a number is a
// no-op rather than a mistake and refusing it would be the over-refusal R-114
// describes. Nothing else widens: a number is not recoverable from `d_MMMM`'s
// output, and that asymmetry is the whole of what C9 catches.
func accepts(want, got valueKind) bool {
	switch {
	case want == kindAny, got == kindUnknown:
		return true
	case want == got:
		return true
	case want == kindText:
		return got == kindNumber || got == kindDate
	default:
		return false
	}
}

// argKind is what a construct's argument may be written as. C8 is this column:
// a path where a quoted literal belongs, or the reverse.
type argKind int

const (
	argLiteral argKind = iota
	argPath
)

func (a argKind) String() string {
	if a == argLiteral {
		return "a quoted literal"
	}
	return "a path"
}

// construct is one row of §10.5's table.
type construct struct {
	name  string
	shape shape
	takes valueKind
	gives valueKind
	args  []argKind

	// one is shapeFirst's and shapeMap's function, over a single value. A
	// false return is the construct yielding nothing, which empties the chain
	// and renders the empty string.
	one func(v any, args []chainArg) (any, bool)
	// fold is shapeFold's, over the whole list.
	fold func(in []any, args []chainArg) (any, bool)
}

var constructs = map[string]*construct{
	"words": {
		name: "words", shape: shapeFirst, takes: kindNumber, gives: kindText,
		one: func(v any, _ []chainArg) (any, bool) {
			n, ok := asInt(v)
			if !ok {
				// "renders anything else as digits" taken to its limit: a value
				// that is not a number at all has no spelling, and the value as
				// written is the only honest answer that is not a failure.
				return scalar(v), true
			}
			return words(n), true
		},
	},
	"sentence": {
		name: "sentence", shape: shapeFirst, takes: kindText, gives: kindText,
		one: func(v any, _ []chainArg) (any, bool) { return sentence(scalar(v)), true },
	},
	"plural": {
		name: "plural", shape: shapeFirst, takes: kindNumber, gives: kindText,
		args: []argKind{argLiteral},
		one: func(v any, args []chainArg) (any, bool) {
			n, ok := asInt(v)
			if !ok {
				// The rule is "the word as written for one"; a value that is
				// not a number is not one, so the plural is what is left.
				n = 0
			}
			return plural(n, args[0].literal), true
		},
	},
	"d_MMMM": {
		name: "d_MMMM", shape: shapeFirst, takes: kindDate, gives: kindText,
		one: func(v any, _ []chainArg) (any, bool) {
			t, ok := asDate(v)
			if !ok {
				return "", true
			}
			return dMMMM(t), true
		},
	},
	"d_MMMM_yyyy": {
		name: "d_MMMM_yyyy", shape: shapeFirst, takes: kindDate, gives: kindText,
		one: func(v any, _ []chainArg) (any, bool) {
			t, ok := asDate(v)
			if !ok {
				return "", true
			}
			return dMMMMyyyy(t), true
		},
	},
	"latest_by": {
		name: "latest_by", shape: shapeFold, takes: kindNode, gives: kindNode,
		args: []argKind{argPath},
		fold: func(in []any, args []chainArg) (any, bool) {
			nodes := make([]map[string]any, 0, len(in))
			for _, v := range in {
				if m, ok := asNode(v); ok {
					nodes = append(nodes, m)
				}
			}
			n, ok := latestBy(nodes, args[0].path)
			if !ok {
				return nil, false
			}
			return n, true
		},
	},
	"strip_code": {
		name: "strip_code", shape: shapeMap, takes: kindText, gives: kindText,
		one: func(v any, _ []chainArg) (any, bool) { return stripCode(scalar(v)), true },
	},
	"join": {
		name: "join", shape: shapeFold, takes: kindText, gives: kindText,
		args: []argKind{argLiteral, argLiteral},
		fold: func(in []any, args []chainArg) (any, bool) {
			items := make([]string, 0, len(in))
			for _, v := range in {
				// I-624: the skip is `join`'s here and is the caller's in the
				// package this generalises.
				if s := strings.TrimSpace(scalar(v)); s != "" {
					items = append(items, s)
				}
			}
			return join(items, args[0].literal, args[1].literal), true
		},
	},
	"count": {
		name: "count", shape: shapeFold, takes: kindAny, gives: kindNumber,
		fold: func(in []any, _ []chainArg) (any, bool) { return len(in), true },
	},
	"max": {
		name: "max", shape: shapeFold, takes: kindDate, gives: kindDate,
		fold: func(in []any, _ []chainArg) (any, bool) {
			t, ok := maxDate(in)
			if !ok {
				return nil, false
			}
			return t, true
		},
	},
}

// constructNames is what an unknown name is reported against, sorted so that
// the message is stable. `compileInferPromptTemplate` lists the columns when it
// refuses a placeholder (`compileInferPromptTemplate`,
// `jets/compute_pipes/pipe_transformation_infer.go:529`) and this is the same
// courtesy: the author of a template with `wordz` in it wants the ten names,
// not a lecture.
func constructNames() string {
	names := make([]string, 0, len(constructs))
	for n := range constructs {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ---------------------------------------------------------------------------
// The functions themselves, from `jets/agentic/briefing/prose/filters.go`.
// ---------------------------------------------------------------------------

// sentence capitalises the first rune.
func sentence(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

var (
	unitWords = []string{"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen",
		"eighteen", "nineteen"}
	tensWords = []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
)

// words spells a count.
//
// **It spells 0 to 999 and renders anything else as digits**, which is the
// honest way to keep it total. British hyphenation and the British "and" -
// "twenty-one", "one hundred and twelve" - because that is the house spelling.
// A count in the thousands is a case a renderer cannot be tested against; the
// numeral is correct, readable and cannot be wrong.
func words(n int) string {
	switch {
	case n < 0 || n > 999:
		return strconv.Itoa(n)
	case n < 20:
		return unitWords[n]
	case n < 100:
		if n%10 == 0 {
			return tensWords[n/10]
		}
		return tensWords[n/10] + "-" + unitWords[n%10]
	case n%100 == 0:
		return unitWords[n/100] + " hundred"
	default:
		return unitWords[n/100] + " hundred and " + words(n%100)
	}
}

// plural is the word as written for one and with an "s" otherwise. An irregular
// plural would be a lookup, which §10.5's "none needs a lookup" rules out - and
// a template that needs one writes the two words out under an `if` rather than
// making every template carry a table.
func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// dMMMM is "14 August": a day without a leading zero and the month in full.
func dMMMM(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + t.Month().String()
}

// dMMMMyyyy is "14 August 2025".
func dMMMMyyyy(t time.Time) string {
	return dMMMM(t) + " " + strconv.Itoa(t.Year())
}

// latestBy returns the node whose named property holds the latest date.
//
// A node with no readable date cannot win, and a tie resolves to the first such
// node in the document's own order - which is unspecified for an entity built
// by draining iterators over a `sync.Map`, so two nodes on one day give either.
// Both are the latest and no order over the pair is more true than the other.
func latestBy(nodes []map[string]any, sel briefing.Selector) (map[string]any, bool) {
	var best map[string]any
	var bestAt time.Time
	for _, n := range nodes {
		var t time.Time
		found := false
		for _, l := range sel.Resolve(n) {
			if l.Value == nil {
				continue
			}
			t, found = asDate(l.Value)
			break
		}
		if !found {
			continue
		}
		if best == nil || t.After(bestAt) {
			best, bestAt = n, t
		}
	}
	return best, best != nil
}

// maxDate skips values that are not dates rather than refusing them: a list is
// what the document asserted and a renderer is not the place to adjudicate it.
func maxDate(vals []any) (time.Time, bool) {
	var best time.Time
	found := false
	for _, v := range vals {
		t, ok := asDate(v)
		if !ok {
			continue
		}
		if !found || t.After(best) {
			best, found = t, true
		}
	}
	return best, found
}

// stripCode removes a leading parenthesised code: "(F1120) Alcohol dependence"
// becomes "Alcohol dependence". A value with no leading code is returned
// unchanged, which is what makes it total over a document that stops emitting
// them.
func stripCode(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "(") {
		return s
	}
	end := strings.Index(t, ")")
	if end < 0 {
		return s
	}
	return strings.TrimSpace(t[end+1:])
}

// join puts a separator between all but the last pair and a different one
// before the last. One item joins to itself and no items to "". There is no
// serial comma.
func join(items []string, sep, last string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], sep) + last + items[len(items)-1]
	}
}

// wrap is a greedy word wrap at width. A word longer than the width takes a
// line of its own rather than being split, so the function is total over any
// input and never invents a hyphen.
//
// **It is also the notation's whitespace rule.** `strings.Fields` collapses
// every run of whitespace inside a paragraph, so the only thing a template
// author decides at a markup boundary is whether there is a space there at all.
// A line break inside a template's text is a space; two spaces are one.
func wrap(s string, width int) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range fields {
		switch {
		case i == 0:
			b.WriteString(w)
			lineLen = len([]rune(w))
		case lineLen+1+len([]rune(w)) <= width:
			b.WriteString(" ")
			b.WriteString(w)
			lineLen += 1 + len([]rune(w))
		default:
			b.WriteString("\n")
			b.WriteString(w)
			lineLen = len([]rune(w))
		}
	}
	return b.String()
}
