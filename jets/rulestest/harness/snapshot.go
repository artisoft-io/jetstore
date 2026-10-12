package harness

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/rulestest"
)

// The checks are evaluated against a Snapshot: every triple of the RDF session,
// decoded once into engine-independent Terms after the rules ran. The match is
// still the join over (s, p, o) the project definition describes (§4.2), with
// the bindings so far; it is made over the snapshot rather than over FindSPO,
// for three reasons measured on the code, two of them since fixed:
//
//   - FindSPO needs a node for every constant, so every check would make nodes
//     in the session it is checking. Reading makes nothing. (On C++ that was
//     safe only once a rete session existed: before one, the native manager
//     fell back to the meta graph, which aborted the process once an RDF
//     session was open. Fixed 2026-10-11, rules_unit_test_harness I-15.)
//   - Literals are interned by constructor (definition F28), so a node built
//     for an expected value matches only if built the way the rules built it.
//     Comparing decoded values does not depend on that.
//   - RdfNode.Equals could not be used: the Go adaptor's compared EQ's result
//     to a fresh rdf.TRUE() by pointer and was always false, and the native one
//     compared wrapper pointers that each call allocates anew. Fixed 2026-10-11
//     (I-11): Equals is node identity in both, which is the interning above.
//
// And one property the harness needs anyway: compare_engines diffs two runs,
// which is a comparison of two snapshots.

// Term kinds: the rule language's type names, as ParseRdfNodeValue reads them,
// with long folded into int and ulong into uint (one literal each in both engines).
const (
	TermNull     = "null"
	TermBlank    = "bn"
	TermResource = "resource"
	TermText     = "text"
	TermInt      = "int"
	TermUint     = "uint"
	TermDouble   = "double"
	TermDate     = "date"
	TermDatetime = "datetime"
)

// Term is one decoded node. Text is canonical for its kind: the resource name,
// the text, the integer in base 10, the date as 2006-01-02, the datetime as
// 2006-01-02T15:04:05 in UTC. A double keeps its value in Num.
type Term struct {
	Kind string
	Text string
	Num  float64
}

func (t Term) key() string { return t.Kind + "\x00" + t.Text }

// Equal compares kind and value; doubles as the engine rounds them.
func (t Term) Equal(o Term) bool {
	if t.Kind != o.Kind {
		return false
	}
	if t.Kind == TermDouble {
		return t.Num == o.Num || roundDouble(t.Num) == roundDouble(o.Num) || rdf.NearlyEqual(t.Num, o.Num)
	}
	return t.Text == o.Text
}

// String is how a term is shown in a result's found column.
func (t Term) String() string {
	switch t.Kind {
	case TermResource:
		return t.Text
	case TermText:
		return strconv.Quote(t.Text)
	case TermNull:
		return "null"
	default:
		return t.Text + "^^" + t.Kind
	}
}

// roundDouble is NewDoubleLiteral's rounding (jets/jetrules/rdf/resource_manager.go).
func roundDouble(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	v, _ := big.NewFloat(x).SetPrec(15).Float64()
	return v
}

const (
	dateLayout     = "2006-01-02"
	datetimeLayout = "2006-01-02T15:04:05"
)

// SnapshotTriple is one decoded triple.
type SnapshotTriple struct {
	S, P, O Term
}

func (t SnapshotTriple) String() string {
	return fmt.Sprintf("(%s %s %s)", t.S, t.P, t.O)
}

// Snapshot is a session's triples, indexed by subject and by predicate.
type Snapshot struct {
	Triples     []SnapshotTriple
	bySubject   map[string][]int
	byPredicate map[string][]int
}

// NewSnapshot indexes triples.
func NewSnapshot(triples []SnapshotTriple) *Snapshot {
	s := &Snapshot{
		Triples:     triples,
		bySubject:   make(map[string][]int),
		byPredicate: make(map[string][]int),
	}
	for i, t := range triples {
		s.bySubject[t.S.key()] = append(s.bySubject[t.S.key()], i)
		s.byPredicate[t.P.key()] = append(s.byPredicate[t.P.key()], i)
	}
	return s
}

// TakeSnapshot reads every triple of the session. It makes no node.
func TakeSnapshot(ses compute_pipes.JetRdfSession) (*Snapshot, error) {
	itor := ses.Find()
	defer itor.Release()
	var triples []SnapshotTriple
	for !itor.IsEnd() {
		s, err := DecodeNode(itor.GetSubject())
		if err != nil {
			return nil, fmt.Errorf("while reading the session's subjects: %w", err)
		}
		p, err := DecodeNode(itor.GetPredicate())
		if err != nil {
			return nil, fmt.Errorf("while reading the session's predicates: %w", err)
		}
		o, err := DecodeNode(itor.GetObject())
		if err != nil {
			return nil, fmt.Errorf("while reading the object of (%s %s): %w", s, p, err)
		}
		triples = append(triples, SnapshotTriple{S: s, P: p, O: o})
		itor.Next()
	}
	return NewSnapshot(triples), nil
}

// DecodeNode turns a node of either engine into a Term. The two adaptors name
// types differently (Go: named_resource, int, uint, string; C++: named_resource,
// int32/int64, uint32/uint64, string) and the native Value() cannot read a
// blank node or an unsigned literal, so those are read through String(). A blank
// node's String() carries its key on both engines (Go BN<key>, C++ BN:<key>),
// which is what keeps two blank nodes two terms.
func DecodeNode(n compute_pipes.RdfNode) (Term, error) {
	if n == nil {
		return Term{Kind: TermNull}, nil
	}
	switch tn := n.Type(); tn {
	case "null", "rdf_null_type":
		return Term{Kind: TermNull}, nil
	case "named_resource":
		name, ok := compute_pipes.GetRdfNodeValue(n).(string)
		if !ok {
			return Term{}, fmt.Errorf("resource node with value %T", n.Value())
		}
		return Term{Kind: TermResource, Text: name}, nil
	case "blank_node":
		return Term{Kind: TermBlank, Text: n.String()}, nil
	case "string":
		v, ok := n.Value().(string)
		if !ok {
			return Term{}, fmt.Errorf("text node with value %T", n.Value())
		}
		return Term{Kind: TermText, Text: v}, nil
	case "int", "int32", "int64":
		v, ok := compute_pipes.GetRdfNodeValue(n).(int)
		if !ok {
			return Term{}, fmt.Errorf("int node with value %T", n.Value())
		}
		return Term{Kind: TermInt, Text: strconv.Itoa(v)}, nil
	case "uint":
		if v, ok := compute_pipes.GetRdfNodeValue(n).(uint); ok {
			return Term{Kind: TermUint, Text: strconv.FormatUint(uint64(v), 10)}, nil
		}
		return parseUintTerm(n.String())
	case "uint32", "uint64":
		return parseUintTerm(n.String())
	case "double":
		v, ok := n.Value().(float64)
		if !ok {
			return Term{}, fmt.Errorf("double node with value %T", n.Value())
		}
		return Term{Kind: TermDouble, Text: strconv.FormatFloat(v, 'g', -1, 64), Num: v}, nil
	case "date":
		t, err := nodeTime(n, rdf.ParseDate)
		if err != nil {
			return Term{}, err
		}
		return Term{Kind: TermDate, Text: t.Format(dateLayout)}, nil
	case "datetime":
		t, err := nodeTime(n, rdf.ParseDatetime)
		if err != nil {
			return Term{}, err
		}
		return Term{Kind: TermDatetime, Text: t.UTC().Format(datetimeLayout)}, nil
	default:
		return Term{}, fmt.Errorf("node of unknown type %q: %s", tn, n.String())
	}
}

func parseUintTerm(s string) (Term, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return Term{}, fmt.Errorf("unsigned node %q: %w", s, err)
	}
	return Term{Kind: TermUint, Text: strconv.FormatUint(v, 10)}, nil
}

func nodeTime(n compute_pipes.RdfNode, parse func(string) (*time.Time, error)) (time.Time, error) {
	switch v := compute_pipes.GetRdfNodeValue(n).(type) {
	case time.Time:
		return v, nil
	case string:
		t, err := parse(v)
		if err != nil {
			return time.Time{}, fmt.Errorf("%s node %q: %w", n.Type(), v, err)
		}
		return *t, nil
	}
	if s, ok := n.Value().(string); ok {
		t, err := parse(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("%s node %q: %w", n.Type(), s, err)
		}
		return *t, nil
	}
	return time.Time{}, fmt.Errorf("%s node with value %T", n.Type(), n.Value())
}

// ExpectedTerm is the Term an object written in a test stands for, read as
// ParseRdfNodeValue reads it.
func ExpectedTerm(o *rulestest.ObjectTerm) (Term, error) {
	v := o.Value
	switch strings.TrimSpace(o.Type) {
	case "null":
		return Term{Kind: TermNull}, nil
	case "bn":
		return Term{Kind: TermBlank, Text: v}, nil
	case "resource":
		return Term{Kind: TermResource, Text: v}, nil
	case "text":
		return Term{Kind: TermText, Text: v}, nil
	case "int", "long":
		var i int
		if _, err := fmt.Sscan(v, &i); err != nil {
			return Term{}, fmt.Errorf("%q is not an %s: %v", v, o.Type, err)
		}
		return Term{Kind: TermInt, Text: strconv.Itoa(i)}, nil
	case "bool":
		// As the engines read a bool (I-45): the accepted spellings, empty is
		// null, anything else makes the test definition invalid.
		b, err := rdf.ParseBool(v)
		switch {
		case errors.Is(err, rdf.ErrNullBool):
			return Term{Kind: TermNull}, nil
		case err != nil:
			return Term{}, err
		case b:
			return Term{Kind: TermInt, Text: "1"}, nil
		default:
			return Term{Kind: TermInt, Text: "0"}, nil
		}
	case "uint", "ulong":
		var u uint
		if _, err := fmt.Sscan(v, &u); err != nil {
			return Term{}, fmt.Errorf("%q is not a %s: %v", v, o.Type, err)
		}
		return Term{Kind: TermUint, Text: strconv.FormatUint(uint64(u), 10)}, nil
	case "double":
		var f float64
		if _, err := fmt.Sscan(v, &f); err != nil {
			return Term{}, fmt.Errorf("%q is not a double: %v", v, err)
		}
		return Term{Kind: TermDouble, Text: strconv.FormatFloat(f, 'g', -1, 64), Num: f}, nil
	case "date":
		t, err := rdf.ParseDate(v)
		if err != nil {
			return Term{}, fmt.Errorf("%q is not a date: %v", v, err)
		}
		return Term{Kind: TermDate, Text: t.Format(dateLayout)}, nil
	case "datetime":
		t, err := rdf.ParseDatetime(v)
		if err != nil {
			return Term{}, fmt.Errorf("%q is not a datetime: %v", v, err)
		}
		return Term{Kind: TermDatetime, Text: t.UTC().Format(datetimeLayout)}, nil
	}
	return Term{}, fmt.Errorf("unknown type %q", o.Type)
}

// sortedTriples lists the triples in a stable order, for diffs and messages.
func (s *Snapshot) sortedTriples() []SnapshotTriple {
	out := append([]SnapshotTriple(nil), s.Triples...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}
