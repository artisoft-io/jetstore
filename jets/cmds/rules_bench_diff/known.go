package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"

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
	// PairDoubleNear: the same subject and predicate, both objects doubles, within
	// RelTol of each other relative to the larger magnitude. For a value the Go
	// engine computed from quantised literals, which is not itself a literal and
	// so is not quantised alike.
	PairDoubleNear = "double_near"
	// PairMerged: a minted node of the OnlyOn side whose lines correspond to the
	// lines of several nodes of the other side, each of which corresponds to it
	// alone: one node where the other engine has several.
	PairMerged = "merged"
	// PairTextDoubleFormat: the same subject and predicate, both objects text,
	// equal once every number in them is written alike: parsed, quantised as the
	// Go engine quantises a literal, and written as the shortest decimal. So
	// "rate: 991" and "rate: 991.000000" are equal.
	PairTextDoubleFormat = "text_double_format"
	// PairPredicateAlias: the same subject and object, and two different
	// predicates that both match Predicate.
	PairPredicateAlias = "predicate_alias"
)

var numberInText = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)

func normaliseNumbers(s string) string {
	return numberInText.ReplaceAllStringFunc(s, func(n string) string {
		v, err := strconv.ParseFloat(n, 64)
		if err != nil {
			return n
		}
		return graphdump.FormatDouble(graphdump.Quantise(v))
	})
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
	// RelTol is double_near's tolerance.
	RelTol float64 `json:"rel_tol,omitempty"`

	s, p, o *regexp.Regexp
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
	if err := json.Unmarshal(raw, &kl); err != nil {
		return nil, fmt.Errorf("known-difference list: %w", err)
	}
	seen := map[string]bool{}
	for i := range kl.Entries {
		e := &kl.Entries[i]
		if e.ID == "" || seen[e.ID] {
			return nil, fmt.Errorf("known-difference list: entry %d: missing or repeated id %q", i, e.ID)
		}
		seen[e.ID] = true
		switch e.Scope {
		case "":
			e.Scope = ScopeCross
		case ScopeCross, ScopeSelf, ScopeAny:
		default:
			return nil, fmt.Errorf("known-difference list: %s: unknown scope %q", e.ID, e.Scope)
		}
		switch e.Pair {
		case PairDoubleNear:
			if e.RelTol <= 0 {
				return nil, fmt.Errorf("known-difference list: %s: %s needs a positive rel_tol", e.ID, e.Pair)
			}
		case PairPredicateAlias:
			if e.p == nil && e.Predicate == "" {
				return nil, fmt.Errorf("known-difference list: %s: %s needs a predicate", e.ID, e.Pair)
			}
		case "", PairDoubleQuantised, PairRelabelled, PairMerged, PairTextDoubleFormat:
		default:
			return nil, fmt.Errorf("known-difference list: %s: unknown pair kind %q", e.ID, e.Pair)
		}
		var err error
		for _, f := range []struct {
			expr string
			re   **regexp.Regexp
		}{{e.Subject, &e.s}, {e.Predicate, &e.p}, {e.Object, &e.o}} {
			if f.expr == "" {
				continue
			}
			if *f.re, err = regexp.Compile("^(?:" + f.expr + ")$"); err != nil {
				return nil, fmt.Errorf("known-difference list: %s: %w", e.ID, err)
			}
		}
		if e.Pair == "" && e.s == nil && e.p == nil && e.o == nil {
			return nil, fmt.Errorf("known-difference list: %s: matches every line; give a subject, predicate, object or pair", e.ID)
		}
	}
	return &kl, nil
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

// matchPair reports whether the pair entry e matches a (on A) against b (on B).
func (e *Entry) matchPair(a, b *line) bool {
	if e.Pair == PairPredicateAlias {
		return a.s == b.s && a.o == b.o && a.p != b.p && e.p.MatchString(a.p) && e.p.MatchString(b.p) &&
			(e.s == nil || e.s.MatchString(a.s))
	}
	if a.s != b.s || a.p != b.p || (e.p != nil && !e.p.MatchString(a.p)) || (e.s != nil && !e.s.MatchString(a.s)) {
		return false
	}
	switch e.Pair {
	case PairDoubleQuantised:
		x, ok1 := a.object.Num()
		y, ok2 := b.object.Num()
		return ok1 && ok2 && graphdump.Quantise(x) == graphdump.Quantise(y)
	case PairTextDoubleFormat:
		return a.object.Kind == "text" && b.object.Kind == "text" && a.object.Value != b.object.Value &&
			normaliseNumbers(a.object.Value) == normaliseNumbers(b.object.Value)
	case PairDoubleNear:
		x, ok1 := a.object.Num()
		y, ok2 := b.object.Num()
		return ok1 && ok2 && math.Abs(x-y) <= e.RelTol*math.Max(math.Abs(x), math.Abs(y))
	}
	return false
}
