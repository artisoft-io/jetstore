package optstats

import (
	"fmt"
	"sync"
)

// Wanted is what a rule set's statistics count: the constant predicates of its
// antecedents, and its constant (subject, predicate) and (predicate, object)
// pairs, every term by its key (TermKey). It is built from the rule set's
// compiled antecedents, positive and negated, with AddAntecedent.
type Wanted struct {
	Predicates map[string]bool
	// SubjectPairs[p][s] and ObjectPairs[p][o] are the constant pairs.
	SubjectPairs map[string]map[string]bool
	ObjectPairs  map[string]map[string]bool
}

// NewWanted is a Wanted with nothing in it.
func NewWanted() *Wanted {
	return &Wanted{
		Predicates:   map[string]bool{},
		SubjectPairs: map[string]map[string]bool{},
		ObjectPairs:  map[string]map[string]bool{},
	}
}

// AddAntecedent records one antecedent (s p o). Each argument is the term's key,
// or "" for a variable or a constant that is not counted (TermKey's false). An
// antecedent with no constant predicate adds nothing.
func (w *Wanted) AddAntecedent(s, p, o string) {
	if p == "" {
		return
	}
	w.Predicates[p] = true
	if s != "" {
		addPair(w.SubjectPairs, p, s)
	}
	if o != "" {
		addPair(w.ObjectPairs, p, o)
	}
}

func addPair(pairs map[string]map[string]bool, p, t string) {
	m := pairs[p]
	if m == nil {
		m = map[string]bool{}
		pairs[p] = m
	}
	m[t] = true
}

// Counter counts one session's graph for one rule set: what a Wanted asks for and
// nothing else. A term is identified by a comparable value of the caller's
// choosing, such as the engine's identity key for the node or the term's key
// itself; the counter is told each wanted term's identity when it is made.
// Distinct subjects and objects are counted by identity, so two identities must
// be equal exactly when the graph holds one node.
//
// A Counter is used by one goroutine. Add each triple of the graph once.
type Counter struct {
	wanted *Wanted
	// preds holds the wanted predicates the session has, by identity.
	preds map[any]*predCounter
}

type predCounter struct {
	key      string
	triples  int64
	subjects map[any]struct{}
	objects  map[any]struct{}
	// subjectPairs and objectPairs count the constant pairs the session has, by
	// the constant's identity.
	subjectPairs map[any]*pairCounter
	objectPairs  map[any]*pairCounter
}

type pairCounter struct {
	key string
	n   int64
}

// NewCounter is a counter for w over a session in which identity gives each
// wanted term's identity, from its key. A term for which identity reports false
// is one the session does not hold, and its counts are zero. Identities must be
// comparable.
func NewCounter(w *Wanted, identity func(key string) (any, bool)) *Counter {
	c := &Counter{wanted: w, preds: map[any]*predCounter{}}
	for p := range w.Predicates {
		id, ok := identity(p)
		if !ok {
			continue
		}
		pc := &predCounter{key: p, subjects: map[any]struct{}{}, objects: map[any]struct{}{}}
		pc.subjectPairs = pairCounters(w.SubjectPairs[p], identity)
		pc.objectPairs = pairCounters(w.ObjectPairs[p], identity)
		c.preds[id] = pc
	}
	return c
}

func pairCounters(terms map[string]bool, identity func(string) (any, bool)) map[any]*pairCounter {
	if len(terms) == 0 {
		return nil
	}
	m := make(map[any]*pairCounter, len(terms))
	for t := range terms {
		if id, ok := identity(t); ok {
			m[id] = &pairCounter{key: t}
		}
	}
	return m
}

// Wants reports whether the triples with predicate p are counted. A caller may
// skip the others without reading their subject and object.
func (c *Counter) Wants(p any) bool {
	_, ok := c.preds[p]
	return ok
}

// Add counts the triple (s, p, o), given by identities. A triple whose predicate
// is not wanted is ignored.
func (c *Counter) Add(s, p, o any) {
	pc := c.preds[p]
	if pc == nil {
		return
	}
	pc.triples++
	pc.subjects[s] = struct{}{}
	pc.objects[o] = struct{}{}
	if x := pc.subjectPairs[s]; x != nil {
		x.n++
	}
	if x := pc.objectPairs[o]; x != nil {
		x.n++
	}
}

// Session is the session's counts, as a rule set of one session. Every wanted
// predicate and pair is in it, with zeros for what the session does not hold.
func (c *Counter) Session() *RuleSet {
	rs := NewRuleSet()
	rs.Sessions = 1
	for p := range c.wanted.Predicates {
		rs.Predicates[p] = &Predicate{}
	}
	for p, m := range c.wanted.SubjectPairs {
		rs.SubjectConstants[p] = zeros(m)
	}
	for p, m := range c.wanted.ObjectPairs {
		rs.ObjectConstants[p] = zeros(m)
	}
	for _, pc := range c.preds {
		rs.Predicates[pc.key] = &Predicate{
			Triples:  pc.triples,
			Subjects: int64(len(pc.subjects)),
			Objects:  int64(len(pc.objects)),
		}
		for _, x := range pc.subjectPairs {
			rs.SubjectConstants[pc.key][x.key] = x.n
		}
		for _, x := range pc.objectPairs {
			rs.ObjectConstants[pc.key][x.key] = x.n
		}
	}
	return rs
}

func zeros(terms map[string]bool) map[string]int64 {
	m := make(map[string]int64, len(terms))
	for t := range terms {
		m[t] = 0
	}
	return m
}

// Accumulator sums sessions' statistics into one file, per rule set. It is safe
// for concurrent use, so that the workers of one pool may share it.
type Accumulator struct {
	mu   sync.Mutex
	file *File
}

// NewAccumulator is an accumulator with no sessions.
func NewAccumulator() *Accumulator {
	return &Accumulator{file: New()}
}

// Add adds a session's statistics for a rule set, as Counter.Session gives them,
// or any rule set's sums. It fails as Merge does, when the rule set was counted
// against another network than the sessions already added.
func (a *Accumulator) Add(ruleSet string, rs *RuleSet) error {
	if err := rs.validate(ruleSet); err != nil {
		return fmt.Errorf("rule set %q: %w", ruleSet, err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.file.add(ruleSet, rs)
}

// Len is the number of rule sets with statistics.
func (a *Accumulator) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.file.RuleSets)
}

// File is a copy of the statistics added so far.
func (a *Accumulator) File() *File {
	a.mu.Lock()
	defer a.mu.Unlock()
	f := New()
	for name, rs := range a.file.RuleSets {
		f.RuleSets[name] = rs.clone()
	}
	return f
}
