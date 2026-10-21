package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/rulesbench/graphdump"
)

//go:embed known_differences.json
var defaultKnown []byte

// Scopes: an entry applies to a comparison of two engines, of one engine with
// itself, or to both.
const (
	ScopeCross = "cross"
	ScopeSelf  = "self"
	ScopeAny   = "any"
)

// Pair kinds: an entry that matches a line only on A against a line only on B.
const (
	// PairDoubleQuantised: the same subject and predicate, both objects doubles,
	// equal once both are quantised as the Go engine quantises a double literal.
	PairDoubleQuantised = "double_quantised"
	// PairRelabelled: the same line up to the labels of minted nodes, under a
	// one-to-one correspondence of labels between A and B built from the lines
	// that pair unambiguously. The node is on both sides and was renamed because
	// its neighbourhood differs; that difference is in its own lines, which are
	// classified on their own. A node that corresponds to two (two lookup subjects
	// merged into one, say) has no correspondence, and its lines stay unmatched.
	PairRelabelled = "relabelled"
	// PairDoubleNear: the same subject and predicate, both objects doubles, at most
	// Quanta apart, a quantum being the step of the Go engine's 15-bit grid at the
	// larger magnitude (graphdump.Quantum). For a value the Go engine computed from
	// quantised literals, which is not itself a literal and so is not quantised
	// alike.
	PairDoubleNear = "double_near"
	// PairMerged: a minted node of the OnlyOn side whose lines correspond to the
	// lines of several nodes of the other side, each of which corresponds to it
	// alone: one node where the other engine has several. Subject and Predicate
	// are its anchor: the node must be the object of a line of its side whose
	// subject and predicate match them.
	PairMerged = "merged"
	// PairTextDoubleFormat: the same subject and predicate, both objects text,
	// that differ only in how their numbers are written: the same text between the
	// numbers, and each pair of numbers equal as written, or one written as C++'s
	// std::to_string writes a double (six decimals) and the other not, and equal
	// once both are quantised as the Go engine quantises a literal. So "rate: 991"
	// and "rate: 991.000000" match; "NPI 1234567890" and "NPI 1234567891" do not.
	PairTextDoubleFormat = "text_double_format"
	// PairPredicateAlias: the same subject and object, and two different
	// predicates that both match Predicate.
	PairPredicateAlias = "predicate_alias"
	// PairLiteralRegexGroup: the same subject and predicate, both objects text; the
	// OnlyOn side's text (Go's literal_regex: the whole match) contains the other's
	// (C++'s: capture group 1) and is longer, and matches Object.
	PairLiteralRegexGroup = "literal_regex_group"
)

// pairFields is what each pair kind reads besides its kind: s, p, o and only_on,
// and whether s or p (or both) are required. A field set on a kind that does not
// read it is refused.
var pairFields = map[string]struct {
	s, p, o, onlyOn bool
	need            string
}{
	PairRelabelled:        {},
	PairMerged:            {s: true, p: true, onlyOn: true, need: "sp"},
	PairDoubleQuantised:   {s: true, p: true},
	PairDoubleNear:        {s: true, p: true},
	PairTextDoubleFormat:  {s: true, p: true},
	PairPredicateAlias:    {s: true, p: true, need: "p"},
	PairLiteralRegexGroup: {s: true, p: true, o: true, onlyOn: true, need: "p"},
}

// KnownList is the known-difference file.
type KnownList struct {
	Comment string  `json:"comment,omitempty"`
	Entries []Entry `json:"entries"`
}

// Entry is one known difference. Every field that is set must match; a line no
// entry matches is unclassified.
type Entry struct {
	ID string `json:"id"`
	// Refs are where the difference is recorded: a section of
	// jets/jetrules/rete/README.md, a register entry of projects/rules_benchmark.
	Refs        []string `json:"refs"`
	Description string   `json:"description"`
	// Scope is cross (the default), self or any.
	Scope string `json:"scope,omitempty"`
	// OnlyOn is the label of the side the line is on, such as go or native.
	OnlyOn string `json:"only_on,omitempty"`
	// Subject, Predicate and Object are regular expressions over the term as
	// written in the dump, anchored at both ends.
	Subject   string `json:"subject,omitempty"`
	Predicate string `json:"predicate,omitempty"`
	Object    string `json:"object,omitempty"`
	// Pair names a pair kind; the entry then matches two lines, one on each side.
	Pair string `json:"pair,omitempty"`
	// Quanta is double_near's tolerance, in quanta of the Go engine's grid.
	Quanta float64 `json:"quanta,omitempty"`
	// Member and File confine the entry to the dumps whose member and file name
	// (<member>/<file> in the tree) they match: regular expressions, anchored at
	// both ends. For a difference known in one member of one workload only. They
	// narrow an entry; they do not make one that matches every line acceptable.
	Member string `json:"member,omitempty"`
	File   string `json:"file,omitempty"`

	s, p, o, m, f *regexp.Regexp
}

// LoadKnown reads a known-difference file, or the embedded one when path is "".
func LoadKnown(path string) (*KnownList, error) {
	raw := defaultKnown
	if path != "" {
		var err error
		if raw, err = os.ReadFile(path); err != nil {
			return nil, err
		}
	}
	var kl KnownList
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&kl); err != nil {
		return nil, fmt.Errorf("known-difference list: %w", err)
	}
	seen := map[string]bool{}
	for i := range kl.Entries {
		e := &kl.Entries[i]
		if e.ID == "" || seen[e.ID] {
			return nil, fmt.Errorf("known-difference list: entry %d: missing or repeated id %q", i, e.ID)
		}
		seen[e.ID] = true
		if err := e.validate(); err != nil {
			return nil, fmt.Errorf("known-difference list: %s: %w", e.ID, err)
		}
	}
	return &kl, nil
}

func (e *Entry) validate() error {
	switch e.Scope {
	case "":
		e.Scope = ScopeCross
	case ScopeCross, ScopeSelf, ScopeAny:
	default:
		return fmt.Errorf("unknown scope %q", e.Scope)
	}
	if e.Pair != "" {
		f, ok := pairFields[e.Pair]
		if !ok {
			return fmt.Errorf("unknown pair kind %q", e.Pair)
		}
		for _, c := range []struct {
			set, read bool
			name      string
		}{{e.Subject != "", f.s, "subject"}, {e.Predicate != "", f.p, "predicate"},
			{e.Object != "", f.o, "object"}, {e.OnlyOn != "", f.onlyOn, "only_on"}} {
			if c.set && !c.read {
				return fmt.Errorf("pair kind %s does not read %s", e.Pair, c.name)
			}
		}
		if strings.Contains(f.need, "s") && e.Subject == "" {
			return fmt.Errorf("pair kind %s needs a subject", e.Pair)
		}
		if strings.Contains(f.need, "p") && e.Predicate == "" {
			return fmt.Errorf("pair kind %s needs a predicate", e.Pair)
		}
		if e.Pair == PairLiteralRegexGroup && e.OnlyOn == "" {
			return fmt.Errorf("pair kind %s needs only_on, the side holding the whole match", e.Pair)
		}
	}
	if (e.Pair == PairDoubleNear) != (e.Quanta != 0) {
		return fmt.Errorf("quanta is double_near's, and double_near needs it")
	}
	if e.Quanta < 0 {
		return fmt.Errorf("quanta must be positive")
	}
	for _, f := range []struct {
		expr string
		re   **regexp.Regexp
	}{{e.Subject, &e.s}, {e.Predicate, &e.p}, {e.Object, &e.o}, {e.Member, &e.m}, {e.File, &e.f}} {
		if f.expr == "" {
			continue
		}
		var err error
		if *f.re, err = regexp.Compile("^(?:" + f.expr + ")$"); err != nil {
			return err
		}
	}
	if e.Pair == "" && e.s == nil && e.p == nil && e.o == nil {
		return fmt.Errorf("matches every line; give a subject, predicate, object or pair")
	}
	return nil
}

// inFile reports whether the entry applies to the dump member/file.
func (e *Entry) inFile(member, file string) bool {
	return (e.m == nil || e.m.MatchString(member)) && (e.f == nil || e.f.MatchString(file))
}

func (e *Entry) inScope(scope string) bool {
	return e.Scope == ScopeAny || e.Scope == scope
}

// matchLine reports whether the single-line entry e matches l on side.
func (e *Entry) matchLine(l *line, side string) bool {
	if e.Pair != "" || (e.OnlyOn != "" && e.OnlyOn != side) {
		return false
	}
	return (e.s == nil || e.s.MatchString(l.s)) &&
		(e.p == nil || e.p.MatchString(l.p)) &&
		(e.o == nil || e.o.MatchString(l.o))
}

// matchPair reports whether the value-pair entry e matches x against y, x being
// the OnlyOn side's line when the entry names one.
func (e *Entry) matchPair(x, y *line) bool {
	if e.s != nil && !(e.s.MatchString(x.s) && e.s.MatchString(y.s)) {
		return false
	}
	if e.Pair == PairPredicateAlias {
		return x.s == y.s && x.o == y.o && x.p != y.p && e.p.MatchString(x.p) && e.p.MatchString(y.p)
	}
	if x.s != y.s || x.p != y.p || (e.p != nil && !e.p.MatchString(x.p)) {
		return false
	}
	switch e.Pair {
	case PairDoubleQuantised:
		a, ok1 := x.object.Num()
		b, ok2 := y.object.Num()
		return ok1 && ok2 && graphdump.Quantise(a) == graphdump.Quantise(b)
	case PairDoubleNear:
		a, ok1 := x.object.Num()
		b, ok2 := y.object.Num()
		return ok1 && ok2 && math.Abs(a-b) <= e.Quanta*graphdump.Quantum(math.Max(math.Abs(a), math.Abs(b)))
	case PairTextDoubleFormat:
		return x.object.Kind == "text" && y.object.Kind == "text" && sameUpToDoubleFormat(x.object.Value, y.object.Value)
	case PairLiteralRegexGroup:
		return x.object.Kind == "text" && y.object.Kind == "text" && (e.o == nil || e.o.MatchString(x.o)) &&
			len(x.object.Value) > len(y.object.Value) && strings.Contains(x.object.Value, y.object.Value)
	}
	return false
}

var (
	numberInText = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)
	// toStringForm is std::to_string's double: six decimals.
	toStringForm = regexp.MustCompile(`^-?[0-9]+\.[0-9]{6}$`)
)

// sameUpToDoubleFormat reports whether a and b differ, and only in numbers one
// side writes as std::to_string does and the other does not; see
// PairTextDoubleFormat.
func sameUpToDoubleFormat(a, b string) bool {
	if a == b {
		return false
	}
	na, nb := numberInText.FindAllStringIndex(a, -1), numberInText.FindAllStringIndex(b, -1)
	if len(na) != len(nb) {
		return false
	}
	pa, pb := 0, 0
	for i := range na {
		if a[pa:na[i][0]] != b[pb:nb[i][0]] {
			return false
		}
		u, v := a[na[i][0]:na[i][1]], b[nb[i][0]:nb[i][1]]
		pa, pb = na[i][1], nb[i][1]
		if u == v {
			continue
		}
		if toStringForm.MatchString(u) == toStringForm.MatchString(v) {
			return false
		}
		x, err1 := strconv.ParseFloat(u, 64)
		y, err2 := strconv.ParseFloat(v, 64)
		if err1 != nil || err2 != nil || graphdump.Quantise(x) != graphdump.Quantise(y) {
			return false
		}
	}
	return a[pa:] == b[pb:]
}
