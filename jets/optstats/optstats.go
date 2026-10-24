// Package optstats is the rule optimizer's statistics file: what a session's graph
// holds when each main rule set ends, counted for the predicates and constants
// that the rule set's antecedents name, so that the compiler can order a rule's
// antecedents by the rows each would add (rules_benchmark Phase 9).
//
// The file is jet_rules/optimizer_statistics.json in a workspace, committed or
// synced into it as a workspace override. rules_bench -stats writes one, a
// pipeline's rules step writes one partial file per worker, and a run_reports
// function merges the partials into the workspace's file. The compiler reads it.
//
// **It imports the standard library only**, so that the compiler, rules_bench and
// compute_pipes can all import it.
//
// # The format, version 1
//
//		{"format": "jetstore.optimizer_statistics", "version": 1,
//		 "rule_sets": {"jet_rules/main.jr": {
//		   "sessions": 100,
//		   "predicates": {"rdf:type": {"triples": 250000, "subjects": 240000, "objects": 12}},
//		   "subject_constants": {"acme:param": {"acme:Config": 100}},
//		   "object_constants":  {"rdf:type": {"acme:Claim": 100000}}}}}
//
//	  - A rule set is keyed by its main rule file as workspace_control.json's
//	    rule_sets names it: the compiler's main rule file name.
//	  - Every count is an integer, the sum over the sessions of that session's value.
//	    A reader divides by sessions. A partial file has the same shape, and merging
//	    adds every count, sessions included, so a merge is exact.
//	  - predicates holds one entry per constant predicate of an antecedent, positive
//	    or negated, of the rule set's compiled network, and nothing else: its
//	    triples, distinct subjects and distinct objects.
//	  - subject_constants[p][s] counts the triples (s, p, *), for each antecedent
//	    whose subject and predicate are constants; object_constants[p][o] counts
//	    (*, p, o), for each antecedent whose predicate and object are constants.
//	  - Zeros are written. No other constant and no data value enters the file.
//	  - A term is keyed by its form in the graph (TermKey).
//	  - The file is encoding/json with two-space indentation and a final newline, so
//	    the same statistics give the same bytes (Encode).
package optstats

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"os"
	"sort"
)

// Format is the file's format field.
const Format = "jetstore.optimizer_statistics"

// Version is the format's version this package reads and writes.
const Version = 1

// FileName is where a workspace holds its statistics, relative to its root.
const FileName = "jet_rules/optimizer_statistics.json"

// File is a statistics file.
type File struct {
	Format   string              `json:"format"`
	Version  int                 `json:"version"`
	RuleSets map[string]*RuleSet `json:"rule_sets"`
}

// RuleSet is one main rule set's statistics, summed over its sessions.
type RuleSet struct {
	Sessions   int64                 `json:"sessions"`
	Predicates map[string]*Predicate `json:"predicates"`
	// SubjectConstants[p][s] is the number of triples (s, p, *).
	SubjectConstants map[string]map[string]int64 `json:"subject_constants"`
	// ObjectConstants[p][o] is the number of triples (*, p, o).
	ObjectConstants map[string]map[string]int64 `json:"object_constants"`
}

// Predicate is one predicate's counts.
type Predicate struct {
	Triples  int64 `json:"triples"`
	Subjects int64 `json:"subjects"`
	Objects  int64 `json:"objects"`
}

// New is an empty statistics file.
func New() *File {
	return &File{Format: Format, Version: Version, RuleSets: map[string]*RuleSet{}}
}

// NewRuleSet is a rule set with no sessions and no keys.
func NewRuleSet() *RuleSet {
	return &RuleSet{
		Predicates:       map[string]*Predicate{},
		SubjectConstants: map[string]map[string]int64{},
		ObjectConstants:  map[string]map[string]int64{},
	}
}

// ForRuleSet is the statistics of the rule set keyed ruleSet, or nil when the file
// has none: the compiler then orders that rule set as it would with no file.
func (f *File) ForRuleSet(ruleSet string) *RuleSet {
	if f == nil {
		return nil
	}
	return f.RuleSets[ruleSet]
}

// Decode reads and validates a statistics file. The format and version are
// checked first, so that a file of another version is refused as such rather
// than as malformed. Then the file is read strictly: every field name must be
// the format's, in its exact case, and no object may hold a key twice, though
// encoding/json alone would accept both (checkKeys). Then it is validated.
func Decode(r io.Reader) (*File, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var head struct {
		Format  *string `json:"format"`
		Version *int    `json:"version"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return nil, fmt.Errorf("malformed statistics file: %w", err)
	}
	if head.Format == nil || *head.Format != Format {
		got := "none"
		if head.Format != nil {
			got = fmt.Sprintf("%q", *head.Format)
		}
		return nil, fmt.Errorf("not a statistics file: format %s, want %q", got, Format)
	}
	if head.Version == nil || *head.Version != Version {
		got := "none"
		if head.Version != nil {
			got = fmt.Sprint(*head.Version)
		}
		return nil, fmt.Errorf("statistics file version %s is not supported: this JetStore reads version %d", got, Version)
	}
	if err := checkKeys(b); err != nil {
		return nil, fmt.Errorf("malformed statistics file: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("malformed statistics file: %w", err)
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return &f, nil
}

// Read reads and validates the statistics file at path. Its errors name the path.
// A missing file is an error that errors.Is(err, fs.ErrNotExist) recognises.
func Read(path string) (*File, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	f, err := Decode(fd)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Encode writes f, validated, as encoding/json with two-space indentation and a
// final newline. Maps are written in key order, so the same statistics give the
// same bytes.
func Encode(w io.Writer, f *File) error {
	b, err := Marshal(f)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// Marshal is Encode's bytes.
func Marshal(f *File) ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Write writes f to path, through a temporary file renamed into place.
func Write(path string, f *File) error {
	b, err := Marshal(f)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Validate checks f against the format: its format and version; every rule set
// with at least one session; counts that are not negative and that a sum of
// sessions could give (distinct subjects and objects no more than triples, and
// none when there are no triples); and each constant pair's predicate among the
// rule set's predicates, with no more triples than it.
func (f *File) Validate() error {
	if f == nil {
		return errors.New("no statistics file")
	}
	if f.Format != Format {
		return fmt.Errorf("not a statistics file: format %q, want %q", f.Format, Format)
	}
	if f.Version != Version {
		return fmt.Errorf("statistics file version %d is not supported: this JetStore reads version %d", f.Version, Version)
	}
	if f.RuleSets == nil {
		return errors.New("malformed statistics file: no rule_sets")
	}
	for _, name := range sortedKeys(f.RuleSets) {
		if err := f.RuleSets[name].validate(name); err != nil {
			return fmt.Errorf("malformed statistics file: rule set %q: %w", name, err)
		}
	}
	return nil
}

func (rs *RuleSet) validate(name string) error {
	switch {
	case name == "":
		return errors.New("an empty rule set name")
	case rs == nil:
		return errors.New("null")
	case rs.Sessions < 1:
		return fmt.Errorf("%d sessions, want at least 1", rs.Sessions)
	case rs.Predicates == nil:
		return errors.New("no predicates")
	case rs.SubjectConstants == nil:
		return errors.New("no subject_constants")
	case rs.ObjectConstants == nil:
		return errors.New("no object_constants")
	}
	for _, p := range sortedKeys(rs.Predicates) {
		c := rs.Predicates[p]
		switch {
		case p == "":
			return errors.New("an empty predicate")
		case c == nil:
			return fmt.Errorf("predicate %q: null", p)
		case c.Triples < 0 || c.Subjects < 0 || c.Objects < 0:
			return fmt.Errorf("predicate %q: a negative count", p)
		case c.Subjects > c.Triples || c.Objects > c.Triples:
			return fmt.Errorf("predicate %q: more distinct subjects or objects than triples", p)
		case c.Triples > 0 && (c.Subjects == 0 || c.Objects == 0):
			return fmt.Errorf("predicate %q: triples with no subject or no object", p)
		}
	}
	for _, side := range []struct {
		name  string
		pairs map[string]map[string]int64
	}{{"subject_constants", rs.SubjectConstants}, {"object_constants", rs.ObjectConstants}} {
		for _, p := range sortedKeys(side.pairs) {
			c := rs.Predicates[p]
			if c == nil {
				return fmt.Errorf("%s: predicate %q is not among the predicates", side.name, p)
			}
			if side.pairs[p] == nil {
				return fmt.Errorf("%s[%q]: null", side.name, p)
			}
			for _, t := range sortedKeys(side.pairs[p]) {
				n := side.pairs[p][t]
				switch {
				case t == "":
					return fmt.Errorf("%s[%q]: an empty term", side.name, p)
				case n < 0:
					return fmt.Errorf("%s[%q][%q]: a negative count", side.name, p, t)
				case n > c.Triples:
					return fmt.Errorf("%s[%q][%q]: %d triples, more than the predicate's %d", side.name, p, t, n, c.Triples)
				}
			}
		}
	}
	return nil
}

// Merge adds the files' statistics into a new file: every count, and the
// sessions, summed per rule set. It is exact. A rule set merged from two files
// must have the same predicates and constant pairs in both, as two partials of
// one compiled network do (zeros are written); otherwise the rule sets were
// counted against different networks, and Merge refuses them.
func Merge(files ...*File) (*File, error) {
	out := New()
	for i, f := range files {
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("file %d: %w", i+1, err)
		}
		for _, name := range sortedKeys(f.RuleSets) {
			if err := out.add(name, f.RuleSets[name]); err != nil {
				return nil, fmt.Errorf("file %d: %w", i+1, err)
			}
		}
	}
	return out, nil
}

// add adds rs into f's rule set name, or copies it in when f has none. The sums
// are made on a copy and kept only when none overflows, so a refused add leaves
// f as it was.
func (f *File) add(name string, rs *RuleSet) error {
	dst := f.RuleSets[name]
	if dst == nil {
		f.RuleSets[name] = rs.clone()
		return nil
	}
	if !dst.sameKeys(rs) {
		return fmt.Errorf("rule set %q: the predicates or constant pairs differ, so the two were counted against different networks", name)
	}
	sum := dst.clone()
	var err error
	add := func(a *int64, b int64) {
		if err == nil {
			*a, err = checkedAdd(*a, b)
		}
	}
	add(&sum.Sessions, rs.Sessions)
	for p, c := range rs.Predicates {
		d := sum.Predicates[p]
		add(&d.Triples, c.Triples)
		add(&d.Subjects, c.Subjects)
		add(&d.Objects, c.Objects)
	}
	for _, side := range []struct{ dst, src map[string]map[string]int64 }{
		{sum.SubjectConstants, rs.SubjectConstants}, {sum.ObjectConstants, rs.ObjectConstants}} {
		for p, m := range side.src {
			for t, n := range m {
				v := side.dst[p][t]
				add(&v, n)
				side.dst[p][t] = v
			}
		}
	}
	if err != nil {
		return fmt.Errorf("rule set %q: %w", name, err)
	}
	f.RuleSets[name] = sum
	return nil
}

// checkedAdd is a + b for counts, which are not negative, or an error when the
// sum does not fit: the merge is exact or refused.
func checkedAdd(a, b int64) (int64, error) {
	if b > math.MaxInt64-a {
		return 0, fmt.Errorf("a count overflows: %d + %d", a, b)
	}
	return a + b, nil
}

func (rs *RuleSet) clone() *RuleSet {
	c := NewRuleSet()
	c.Sessions = rs.Sessions
	for p, v := range rs.Predicates {
		x := *v
		c.Predicates[p] = &x
	}
	for p, m := range rs.SubjectConstants {
		c.SubjectConstants[p] = maps.Clone(m)
	}
	for p, m := range rs.ObjectConstants {
		c.ObjectConstants[p] = maps.Clone(m)
	}
	return c
}

func (rs *RuleSet) sameKeys(o *RuleSet) bool {
	return sameKeySet(rs.Predicates, o.Predicates) &&
		samePairKeys(rs.SubjectConstants, o.SubjectConstants) &&
		samePairKeys(rs.ObjectConstants, o.ObjectConstants)
}

func sameKeySet[V any](a, b map[string]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func samePairKeys(a, b map[string]map[string]int64) bool {
	if !sameKeySet(a, b) {
		return false
	}
	for k := range a {
		if !sameKeySet(a[k], b[k]) {
			return false
		}
	}
	return true
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The per-session values the compiler's estimate reads. Each divides a sum by
// the rule set's sessions, and reports false when the rule set has no entry for
// what is asked.

// Has reports whether predicate p has statistics.
func (rs *RuleSet) Has(p string) bool {
	return rs != nil && rs.Predicates[p] != nil
}

// N is p's triples per session.
func (rs *RuleSet) N(p string) (float64, bool) {
	if !rs.Has(p) {
		return 0, false
	}
	return float64(rs.Predicates[p].Triples) / float64(rs.Sessions), true
}

// DS is p's distinct subjects per session, floored at 1.
func (rs *RuleSet) DS(p string) (float64, bool) {
	if !rs.Has(p) {
		return 0, false
	}
	return max(float64(rs.Predicates[p].Subjects)/float64(rs.Sessions), 1), true
}

// DO is p's distinct objects per session, floored at 1.
func (rs *RuleSet) DO(p string) (float64, bool) {
	if !rs.Has(p) {
		return 0, false
	}
	return max(float64(rs.Predicates[p].Objects)/float64(rs.Sessions), 1), true
}

// SubjectCount is the triples (s, p, *) per session, for a subject key s; false
// when the file has no such pair.
func (rs *RuleSet) SubjectCount(p, s string) (float64, bool) {
	return rs.pairCount(rs.subjectConstants(), p, s)
}

// ObjectCount is the triples (*, p, o) per session, for an object key o; false
// when the file has no such pair.
func (rs *RuleSet) ObjectCount(p, o string) (float64, bool) {
	return rs.pairCount(rs.objectConstants(), p, o)
}

func (rs *RuleSet) subjectConstants() map[string]map[string]int64 {
	if rs == nil {
		return nil
	}
	return rs.SubjectConstants
}

func (rs *RuleSet) objectConstants() map[string]map[string]int64 {
	if rs == nil {
		return nil
	}
	return rs.ObjectConstants
}

func (rs *RuleSet) pairCount(pairs map[string]map[string]int64, p, t string) (float64, bool) {
	n, ok := pairs[p][t]
	if !ok {
		return 0, false
	}
	return float64(n) / float64(rs.Sessions), true
}

// PredicateNames is the rule set's predicates, in key order.
func (rs *RuleSet) PredicateNames() []string {
	if rs == nil {
		return nil
	}
	return sortedKeys(rs.Predicates)
}

// keyShape is the format's object structure, for checkKeys: an object with
// named fields, a map whose values all have one shape, or a scalar.
type keyShape struct {
	fields map[string]*keyShape
	elem   *keyShape
}

var (
	scalarShape    = &keyShape{}
	predicateShape = &keyShape{fields: map[string]*keyShape{
		"triples": scalarShape, "subjects": scalarShape, "objects": scalarShape}}
	pairsShape   = &keyShape{elem: &keyShape{elem: scalarShape}}
	ruleSetShape = &keyShape{fields: map[string]*keyShape{
		"sessions":          scalarShape,
		"predicates":        {elem: predicateShape},
		"subject_constants": pairsShape,
		"object_constants":  pairsShape,
	}}
	fileShape = &keyShape{fields: map[string]*keyShape{
		"format": scalarShape, "version": scalarShape, "rule_sets": {elem: ruleSetShape}}}
)

// checkKeys walks the document's tokens and refuses what encoding/json would let
// through: a field name in another case than the format's ("Sessions"), which
// it matches case-insensitively, and a key repeated in one object, of which it
// keeps the last. Values are checked by the decode that follows.
func checkKeys(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	return walkKeys(dec, fileShape, "")
}

func walkKeys(dec *json.Decoder, shape *keyShape, path string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if delim == '[' {
		for dec.More() {
			if err := walkKeys(dec, scalarShape, path+"[]"); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key := tok.(string)
		if seen[key] {
			return fmt.Errorf("%s: key %q appears twice", orRoot(path), key)
		}
		seen[key] = true
		child := scalarShape
		switch {
		case shape.fields != nil:
			c, ok := shape.fields[key]
			if !ok {
				return fmt.Errorf("%s: unknown field %q", orRoot(path), key)
			}
			child = c
		case shape.elem != nil:
			child = shape.elem
		}
		if err := walkKeys(dec, child, path+"/"+key); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

func orRoot(path string) string {
	if path == "" {
		return "/"
	}
	return path
}
