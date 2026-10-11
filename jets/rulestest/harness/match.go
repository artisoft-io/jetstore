package harness

import (
	"fmt"
	"sort"
	"strings"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// Binding maps ?variables to the terms they matched.
type Binding map[string]Term

func (b Binding) clone() Binding {
	c := make(Binding, len(b)+2)
	for k, v := range b {
		c[k] = v
	}
	return c
}

// String lists the binding by variable name, for the found column.
func (b Binding) String() string {
	names := make([]string, 0, len(b))
	for n := range b {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n + "=" + b[n].String()
	}
	return strings.Join(parts, ", ")
}

// maxFound caps how many bindings or values a found column lists.
const maxFound = 5

// slot is one position of a pattern triple: a constant term or a variable.
type slot struct {
	variable string
	term     Term
}

func (s slot) String() string {
	if s.variable != "" {
		return s.variable
	}
	return s.term.String()
}

type patternTriple struct{ s, p, o slot }

func (t patternTriple) String() string { return fmt.Sprintf("(%s %s %s)", t.s, t.p, t.o) }

func resourceSlot(name string) slot {
	if rulestest.IsVariableName(name) {
		return slot{variable: name}
	}
	return slot{term: Term{Kind: TermResource, Text: name}}
}

func compilePattern(triples []rulestest.Triple) ([]patternTriple, error) {
	out := make([]patternTriple, len(triples))
	for i, t := range triples {
		out[i] = patternTriple{s: resourceSlot(t.S), p: resourceSlot(t.P)}
		switch {
		case t.O == nil:
			return nil, fmt.Errorf("triple %d has no object", i)
		case t.O.IsVariable():
			out[i].o = slot{variable: t.O.Variable}
		default:
			term, err := ExpectedTerm(t.O)
			if err != nil {
				return nil, fmt.Errorf("the object of triple %d: %w", i, err)
			}
			out[i].o = slot{term: term}
		}
	}
	return out, nil
}

// resolve gives a slot's term under b, or false when it is an unbound variable.
func (s slot) resolve(b Binding) (Term, bool) {
	if s.variable == "" {
		return s.term, true
	}
	t, ok := b[s.variable]
	return t, ok
}

// bind extends b with s := t, or reports a conflict with an earlier binding
// (the same variable twice in one triple, or across triples).
func (s slot) bind(b Binding, t Term) bool {
	if s.variable == "" {
		return s.term.Equal(t)
	}
	if prev, ok := b[s.variable]; ok {
		return prev.Equal(t)
	}
	b[s.variable] = t
	return true
}

// joinStats records how far a failed join got, for its found column.
type joinStats struct {
	deepest     int // the furthest triple the join reached and could not satisfy, -1 if none
	atDeepest   int // bindings that reached it
	firstAtDeep Binding
}

// Match returns every binding of the pattern's variables under which all of
// its triples are in the snapshot: a nested-loop join, triple by triple, with
// the variables bound so far. Bindings are sorted, so the first is stable
// across runs and engines.
func (s *Snapshot) match(pattern []patternTriple) ([]Binding, joinStats) {
	st := joinStats{deepest: -1}
	var out []Binding
	var walk func(i int, b Binding)
	walk = func(i int, b Binding) {
		if i == len(pattern) {
			out = append(out, b)
			return
		}
		pt := pattern[i]
		found := false
		for _, ti := range s.candidates(pt, b) {
			t := s.Triples[ti]
			nb := b.clone()
			if pt.s.bind(nb, t.S) && pt.p.bind(nb, t.P) && pt.o.bind(nb, t.O) {
				found = true
				walk(i+1, nb)
			}
		}
		if !found && i >= st.deepest {
			if i > st.deepest {
				st.deepest, st.atDeepest, st.firstAtDeep = i, 0, b
			}
			st.atDeepest++
		}
	}
	walk(0, Binding{})
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out, st
}

// candidates narrows by the subject, then the predicate, when either is known.
func (s *Snapshot) candidates(pt patternTriple, b Binding) []int {
	if t, ok := pt.s.resolve(b); ok {
		return s.bySubject[t.key()]
	}
	if t, ok := pt.p.resolve(b); ok {
		return s.byPredicate[t.key()]
	}
	all := make([]int, len(s.Triples))
	for i := range all {
		all[i] = i
	}
	return all
}

// objectsOf lists the objects of (s, p), for explaining a failed triple.
func (s *Snapshot) objectsOf(subject, predicate Term) []Term {
	var out []Term
	for _, ti := range s.bySubject[subject.key()] {
		if s.Triples[ti].P.Equal(predicate) {
			out = append(out, s.Triples[ti].O)
		}
	}
	return out
}

// EvaluateCheck evaluates one check against the snapshot. list is
// rulestest.KindExpected or rulestest.KindExcluded, the list the check came
// from. The result's SessionName is the caller's to set. An error means the
// check cannot be evaluated as written (an object that does not parse as its
// type, a count expression that does not build): the test is in error, not failed.
func (s *Snapshot) EvaluateCheck(c *rulestest.Check, list string) (rulestest.CheckResult, error) {
	r := rulestest.CheckResult{Kind: c.Kind(list), Check: c.String()}
	if c.Count != nil {
		return s.evaluateCount(c, r)
	}
	pattern, err := compilePattern(c.Triples())
	if err != nil {
		return r, err
	}
	bindings, st := s.match(pattern)
	switch list {
	case rulestest.KindExpected:
		if len(bindings) > 0 {
			r.Outcome = rulestest.OutcomePass
			r.Found = describeBindings(bindings)
		} else {
			r.Outcome = rulestest.OutcomeFail
			r.Found = s.explainNoMatch(pattern, st)
		}
	case rulestest.KindExcluded:
		if len(bindings) == 0 {
			r.Outcome = rulestest.OutcomePass
			r.Found = "no match"
		} else {
			r.Outcome = rulestest.OutcomeFail
			r.Found = describeBindings(bindings)
		}
	default:
		return r, fmt.Errorf("unknown check list %q", list)
	}
	return r, nil
}

func describeBindings(bindings []Binding) string {
	if len(bindings) == 1 && len(bindings[0]) == 0 {
		return "found"
	}
	parts := make([]string, 0, maxFound)
	for i, b := range bindings {
		if i == maxFound {
			break
		}
		parts = append(parts, "{"+b.String()+"}")
	}
	s := fmt.Sprintf("%d match", len(bindings))
	if len(bindings) > 1 {
		s += "es"
	}
	s += ": " + strings.Join(parts, " ")
	if len(bindings) > maxFound {
		s += " ..."
	}
	return s
}

// explainNoMatch says which triple stopped the join and, when its subject and
// predicate are known there, what objects they do have.
func (s *Snapshot) explainNoMatch(pattern []patternTriple, st joinStats) string {
	if st.deepest < 0 {
		return "no match"
	}
	pt := pattern[st.deepest]
	var msg string
	if len(pattern) == 1 {
		msg = "no match"
	} else {
		msg = fmt.Sprintf("no match for triple %d of %d %s", st.deepest+1, len(pattern), pt)
		if st.deepest > 0 {
			msg += fmt.Sprintf(", reached by %d binding(s) of the triples before it, first {%s}", st.atDeepest, st.firstAtDeep)
		}
	}
	subj, okS := pt.s.resolve(st.firstAtDeep)
	pred, okP := pt.p.resolve(st.firstAtDeep)
	if okS && okP {
		objs := s.objectsOf(subj, pred)
		if len(objs) == 0 {
			msg += fmt.Sprintf("; (%s %s) has no object", subj, pred)
		} else {
			parts := make([]string, 0, maxFound)
			for i, o := range objs {
				if i == maxFound {
					parts = append(parts, "...")
					break
				}
				parts = append(parts, o.String())
			}
			msg += fmt.Sprintf("; (%s %s) has %s", subj, pred, strings.Join(parts, ", "))
		}
	}
	return msg
}

// evaluateCount evaluates {"s", "p", "count": ExpressionNode} against
// {"count": n}, n an int64 as parseValue reads a constant (definition F33).
// With a pattern, every binding must satisfy it, and a pattern that binds
// nothing fails the check: "every member has one name" is not shown by an
// empty session.
func (s *Snapshot) evaluateCount(c *rulestest.Check, r rulestest.CheckResult) (rulestest.CheckResult, error) {
	eval, err := compute_pipes.ExprBuilderContext{}.BuildExprNodeEvaluator("count", nil, c.Count)
	if err != nil {
		return r, fmt.Errorf("the count expression: %w", err)
	}
	if eval == nil {
		return r, fmt.Errorf("the count expression is empty")
	}
	bindings := []Binding{{}}
	if len(c.Pattern) > 0 {
		pattern, err := compilePattern(c.Pattern)
		if err != nil {
			return r, err
		}
		var st joinStats
		bindings, st = s.match(pattern)
		if len(bindings) == 0 {
			r.Outcome = rulestest.OutcomeFail
			r.Found = "the pattern matched nothing, so there is no subject to count: " + s.explainNoMatch(pattern, st)
			return r, nil
		}
	}
	subj, pred := resourceSlot(c.S), resourceSlot(c.P)
	var failed, passed []string
	for _, b := range bindings {
		st, okS := subj.resolve(b)
		pt, okP := pred.resolve(b)
		if !okS || !okP {
			return r, fmt.Errorf("the count's s or p is a variable the pattern does not bind")
		}
		n := int64(len(s.objectsOf(st, pt)))
		v, err := eval.Eval(map[string]any{"count": n})
		if err != nil {
			return r, fmt.Errorf("evaluating the count expression with count=%d: %w", n, err)
		}
		label := fmt.Sprintf("count=%d", n)
		if len(b) > 0 {
			label = fmt.Sprintf("{%s} count=%d", b, n)
		}
		if compute_pipes.ToBool(v) {
			passed = append(passed, label)
		} else {
			failed = append(failed, label)
		}
	}
	list := passed
	r.Outcome = rulestest.OutcomePass
	if len(failed) > 0 {
		list = failed
		r.Outcome = rulestest.OutcomeFail
	}
	found := strings.Join(list[:min(len(list), maxFound)], "; ")
	if len(list) > maxFound {
		found += "; ..."
	}
	if len(bindings) > 1 {
		if len(failed) > 0 {
			found = fmt.Sprintf("%d of %d bindings fail: %s", len(failed), len(bindings), found)
		} else {
			found = fmt.Sprintf("all %d bindings pass: %s", len(bindings), found)
		}
	}
	r.Found = found
	return r, nil
}
