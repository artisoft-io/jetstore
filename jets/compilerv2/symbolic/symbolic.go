// Package symbolic is an authoring form for JetRules written as JSON, for an
// author that reproduces JSON and not JetRules (jetstore_agentic_ai I-250).
//
// **The measurement it answers.** On granite4.1:3b, over two runs, the
// compile_rule_file tool's content arm carried a rule file intact 6 and 4 times
// in 18, while validate_cpipes_config's carried a whole cpipes config intact 17
// and 16 times — same model, same run — and every fragment dropped was a
// JetRules construct: a rule label, an import line (jetstore_agentic_ai F176).
// The variable is the notation.
//
// **The form is the compiler's own JSON form with the keys inlined.** A
// compiled .jr.json references every resource by an integer key into a
// resources array, and the key is file-local — assigned in import order — so an
// author cannot emit one even in principle. Here every key site holds the
// resource itself: an identifier string where the resource has an id (a
// variable, a declared resource, a declared literal), and a {type, value}
// literal where it has none. A wrong id then compiles to a new resource and is
// reported; a wrong integer would have been a valid rule about something else.
//
// **It is rendered to JetRules source and compiled by the unchanged compiler**,
// rather than entering the compiler beside the parser. The renderer writes
// every structural token the measurement found dropped — the brackets, the
// import line, the arrow, the terminator — so the author never writes one, and
// the parser stays the authority on what a rule file is: a renderer that drifts
// from the grammar produces text the parser refuses, loudly, where a second
// entry point would have accepted it quietly.
//
// **The suffix is .jrs.json and not .jr.json**, which is what I-250 proposed:
// .jr.json is the name the compiler writes its output to, and every workspace
// .gitignore lists *.jr.json. An authored file of that name would be ignored by
// git and overwritten by the next compile of a main file with the same stem.
package symbolic

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

// Suffix is the file suffix of a symbolic rule file.
const Suffix = ".jrs.json"

// RuleFile is one symbolic rule file: the files it imports and its rules.
// Class, resource and lookup declarations are not part of the form — a rule
// references the data model's vocabulary, it does not define it — so a
// RuleFile imports the .jr files that declare what its rules name.
type RuleFile struct {
	// Imports are workspace-relative paths, rendered as import statements in
	// order, e.g. "data_model/jets_model.jr".
	Imports []string `json:"imports,omitempty"`
	Rules   []Rule   `json:"rules"`
}

// Rule is one rule. The compiler's own outputs — labels, vertexes, beta
// relations, validity — are not part of it; the compiler computes them.
type Rule struct {
	Name string `json:"name"`
	// Salience renders as the s= property. Absent is the compiler's default.
	Salience *int `json:"salience,omitempty"`
	// Optimization renders as the o= property. Absent is the compiler's
	// default, which is true.
	Optimization *bool `json:"optimization,omitempty"`
	// Properties are the rule's other header properties, each a string, a
	// boolean or an integer — the three things a property value can be. "s"
	// and "o" are refused here: each thing has one way to be said.
	Properties  map[string]PropertyValue `json:"properties,omitempty"`
	Antecedents []Antecedent             `json:"antecedents"`
	Consequents []Consequent             `json:"consequents"`
}

// Antecedent is one term of a rule's condition.
type Antecedent struct {
	Not       bool  `json:"not,omitempty"`
	Subject   Term  `json:"subject"`
	Predicate Term  `json:"predicate"`
	Object    Term  `json:"object"`
	Filter    *Expr `json:"filter,omitempty"`
}

// Consequent is one term of a rule's conclusion. Its object is an expression,
// which is most often a single term.
type Consequent struct {
	Subject   Term `json:"subject"`
	Predicate Term `json:"predicate"`
	Object    Expr `json:"object"`
}

// Term is a resource, inlined. Exactly one of ID and Literal is set.
//
// In JSON a term is a string — an id: "?claim", "rdf:type", "hc:MedicalClaim",
// "_0:index", or the name of a declared resource or literal — or a literal
// with no id: {"type": "int", "value": "5"}. A JSON number and a JSON boolean
// are accepted as shorthand for an int or double literal and for the keywords
// true and false.
type Term struct {
	ID      string
	Literal *Literal
}

// Literal is a resource with no id: a value written in place.
//
// Value is the literal's text exactly as the compiler stores it, which for a
// text, date, datetime or bool literal is the text between the quotes, not
// unescaped.
type Literal struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// Expr is a filter or a consequent's object: a term, a binary operation or a
// unary one. Exactly one shape is set.
//
// In JSON a leaf is a term in any of its forms, a binary operation is
// {"op": "+", "lhs": ..., "rhs": ...} and a unary one is {"op": "not", "arg": ...}.
type Expr struct {
	Term     *Term
	Op       string
	Lhs, Rhs *Expr
	Arg      *Expr
}

// PropertyValue is a rule property's value: one of string, bool or int64.
type PropertyValue struct {
	V any
}

// ---------------------------------------------------------------------------
// JSON
// ---------------------------------------------------------------------------

// Parse decodes a symbolic rule file strictly: an unknown field anywhere is an
// error, because an author who misspells "filter" has written a different
// rule from the one they meant and silence would compile it.
func Parse(data []byte) (*RuleFile, error) {
	var f RuleFile
	if err := strictUnmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("unexpected data after the JSON value")
	}
	return nil
}

func (t Term) MarshalJSON() ([]byte, error) {
	switch {
	case t.Literal != nil && t.ID != "":
		return nil, fmt.Errorf("a term is an id or a literal, not both")
	case t.Literal != nil:
		if t.Literal.Type == "keyword" && (t.Literal.Value == "true" || t.Literal.Value == "false") {
			return []byte(t.Literal.Value), nil
		}
		return json.Marshal(t.Literal)
	default:
		return json.Marshal(t.ID)
	}
}

func (t *Term) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("empty term")
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*t = Term{ID: s}
		return nil
	case '{':
		var l Literal
		if err := strictUnmarshal(data, &l); err != nil {
			return fmt.Errorf("a literal term is {\"type\": ..., \"value\": ...}: %w", err)
		}
		*t = Term{Literal: &l}
		return nil
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		*t = Term{Literal: &Literal{Type: "keyword", Value: fmt.Sprint(b)}}
		return nil
	case 'n':
		return fmt.Errorf("a term cannot be JSON null; the keyword null is {\"type\": \"keyword\", \"value\": \"null\"}")
	default:
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("a term is a string, a {type, value} literal, a number or a boolean: %w", err)
		}
		typ := "double"
		if _, err := n.Int64(); err == nil {
			typ = "int"
		}
		*t = Term{Literal: &Literal{Type: typ, Value: n.String()}}
		return nil
	}
}

func (e Expr) MarshalJSON() ([]byte, error) {
	switch {
	case e.Term != nil:
		return json.Marshal(e.Term)
	case e.Arg != nil:
		return json.Marshal(struct {
			Op  string `json:"op"`
			Arg *Expr  `json:"arg"`
		}{e.Op, e.Arg})
	default:
		return json.Marshal(struct {
			Op  string `json:"op"`
			Lhs *Expr  `json:"lhs"`
			Rhs *Expr  `json:"rhs"`
		}{e.Op, e.Lhs, e.Rhs})
	}
}

func (e *Expr) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '{' {
		var keys map[string]json.RawMessage
		if err := json.Unmarshal(data, &keys); err != nil {
			return err
		}
		if _, isOp := keys["op"]; isOp {
			var op struct {
				Op  string `json:"op"`
				Lhs *Expr  `json:"lhs"`
				Rhs *Expr  `json:"rhs"`
				Arg *Expr  `json:"arg"`
			}
			if err := strictUnmarshal(data, &op); err != nil {
				return fmt.Errorf("an operation is {\"op\", \"lhs\", \"rhs\"} or {\"op\", \"arg\"}: %w", err)
			}
			*e = Expr{Op: op.Op, Lhs: op.Lhs, Rhs: op.Rhs, Arg: op.Arg}
			return nil
		}
	}
	var t Term
	if err := t.UnmarshalJSON(data); err != nil {
		return err
	}
	*e = Expr{Term: &t}
	return nil
}

func (p PropertyValue) MarshalJSON() ([]byte, error) { return json.Marshal(p.V) }

func (p *PropertyValue) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("empty property value")
	}
	switch data[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		p.V = s
	case 't', 'f':
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		p.V = b
	default:
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("a property value is a string, a boolean or an integer: %w", err)
		}
		i, err := n.Int64()
		if err != nil {
			return fmt.Errorf("a property value is a string, a boolean or an integer, got %s", n)
		}
		p.V = i
	}
	return nil
}

// sortedKeys returns a map's keys in order, so rendering is deterministic: two
// renderings of one document must be the same text.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

//go:embed rule_file.schema.json
var schemaJSON []byte

// Schema returns the JSON Schema of the form, which is what a tool signature
// publishes. The Go types and Render are the authority on what renders; the
// schema is held to them by TestCorpusRoundTrip.
func Schema() []byte { return schemaJSON }
