package prose

// The ten constructs of plan §1.10.6, one function each: `words`, `sentence`,
// `plural`, `d_MMMM`, `d_MMMM_yyyy`, `latest_by`, `strip_code`, `join`, `count`
// and `max`. §1.10.6's `count` is `len` of what an accessor returned and its
// `each` is a `for` loop, so neither takes a function here.
//
// **Every one is total and none needs a lookup**, which is the property that
// makes this arm a control rather than a second thing to debug. Where totality
// costs something - a number too large to spell, a string with no code to strip
// - the function says here what it does instead of failing.

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/artisoft-io/jetstore/jets/agentic/briefing"
)

// sentence capitalises the first rune. It is applied to `words` at the head of
// element 1 and nowhere else.
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
// A briefing carrying a four-figure event count is a member nobody has, and a
// renderer that spelled it would be carrying code for a case it cannot be tested
// against; the numeral is correct, readable and cannot be wrong.
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

// plural is the `plural: "visit"` filter: the word as written for one, and with
// an "s" otherwise. The two words it is ever applied to are "visit" and "fill",
// both regular, and an irregular plural would be a lookup - which §1.10.6's
// "none needs a lookup" rules out.
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
// **It is applied to the events rather than to `Latest_Service_Date`**
// (§1.10.6): element 2 needs the encounter the extremum belongs to, and the
// briefing-level extremum is a bare date that belongs to nothing.
//
// A node with no readable date cannot win, and a tie is resolved to the first
// such node in the entity's order - which is unspecified, so two encounters on
// one day give either. Both are "the most recent contact" and no order over the
// pair is more true than the other; see the package comment for where the
// instability comes from and where it belongs fixed.
func latestBy(nodes []map[string]any, sel briefing.Selector) (map[string]any, bool) {
	var best map[string]any
	var bestAt time.Time
	for _, n := range nodes {
		t, ok := date(n, sel)
		if !ok {
			continue
		}
		if best == nil || t.After(bestAt) {
			best, bestAt = n, t
		}
	}
	return best, best != nil
}

// maxDate is the `max` filter over a list of dates. Values that are not dates
// are skipped rather than refused: a fill list is what the projection asserted
// and a renderer is not the place to adjudicate it.
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

// stripCode removes a leading parenthesised diagnosis code: "(F1120) Alcohol
// dependence" becomes "Alcohol dependence".
//
// **The code is stripped in the prose and stays on the entity** (§1.10.4). It
// exists so a checker can match a whole string exactly, and it is noise to a
// member and jargon to a representative. A value with no leading code is
// returned unchanged, which is what makes this total over a projection that
// stops emitting them.
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

// join is the `join: ", ", " and "` filter: a separator between all but the last
// pair and a different one before the last. One item joins to itself, and no
// items to "". There is no serial comma, which is what §1.10.6 prints.
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

// wrap is a greedy word wrap at width. A word longer than the width takes a line
// of its own rather than being split, so the function is total over any input
// and never invents a hyphen.
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
