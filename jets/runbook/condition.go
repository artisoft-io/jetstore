package runbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Quantifiers of a leaf condition: how the ref's value is read.
const (
	QOutput = "output" // a scalar
	QAllOf  = "all_of" // every element of a list
	QAnyOf  = "any_of" // some element of a list
	QCount  = "count"  // the list's length, compared as an integer
)

// Comparison operators of a leaf condition.
const (
	OpEq    = "eq"
	OpNe    = "ne"
	OpLt    = "lt"
	OpLe    = "le"
	OpGt    = "gt"
	OpGe    = "ge"
	OpIn    = "in"    // the operand is an array; true when the value equals one element
	OpEmpty = "empty" // the operand is a boolean; see Condition.Eval
)

var quantifiers = []string{QOutput, QAllOf, QAnyOf, QCount}
var operators = []string{OpEq, OpNe, OpLt, OpLe, OpGt, OpGe, OpIn, OpEmpty}

func isOneOf(s string, set []string) bool {
	for _, x := range set {
		if s == x {
			return true
		}
	}
	return false
}

// Condition is a JSON condition. Exactly one form is set:
//
//	{"all": [c, ...]}            All
//	{"any": [c, ...]}            Any
//	{"not": c}                   Not
//	{"<quantifier>": ref, "<op>": operand}   Quantifier, Ref, Op, Operand
//
// There are no functions and no arithmetic: a condition can only compare.
type Condition struct {
	All []*Condition
	Any []*Condition
	Not *Condition

	Quantifier string
	Ref        string
	Op         string
	// Operand is the comparison value as written. It is coerced to the ref's
	// type when the condition is evaluated (an integer for count).
	Operand json.RawMessage
}

// MarshalJSON writes the condition in its JSON form.
func (c *Condition) MarshalJSON() ([]byte, error) {
	m := map[string]any{}
	switch {
	case c.All != nil:
		m["all"] = c.All
	case c.Any != nil:
		m["any"] = c.Any
	case c.Not != nil:
		m["not"] = c.Not
	default:
		m[c.Quantifier] = c.Ref
		if len(c.Operand) == 0 {
			m[c.Op] = nil
		} else {
			m[c.Op] = c.Operand
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON reads a condition, refusing anything outside the grammar.
func (c *Condition) UnmarshalJSON(b []byte) error {
	var fs findings
	parsed := parseCondition(b, "", &fs)
	if err := fs.err(); err != nil {
		return err
	}
	*c = *parsed
	return nil
}

// Refs lists every ref the condition reads, in document order.
func (c *Condition) Refs() []string {
	var out []string
	c.walk(func(leaf *Condition) { out = append(out, leaf.Ref) })
	return out
}

func (c *Condition) walk(fn func(leaf *Condition)) {
	if c == nil {
		return
	}
	switch {
	case c.All != nil:
		for _, s := range c.All {
			s.walk(fn)
		}
	case c.Any != nil:
		for _, s := range c.Any {
			s.walk(fn)
		}
	case c.Not != nil:
		c.Not.walk(fn)
	default:
		fn(c)
	}
}

// parseCondition reads raw as a condition, reporting at ptr. It returns a
// condition even when it reports errors, so that later checks can go on; it
// returns nil only when raw is not an object.
func parseCondition(raw json.RawMessage, ptr string, fs *findings) *Condition {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		fs.errorf(CodeCondition, ptr, "a condition is a JSON object, such as {\"output\": \"step.column\", \"eq\": 0}")
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, comb := range []string{"all", "any", "not"} {
		raw, ok := m[comb]
		if !ok {
			continue
		}
		c := &Condition{}
		if len(m) != 1 {
			fs.errorf(CodeCondition, ptr, fmt.Sprintf("%q takes no other key in the same object; found %s", comb, strings.Join(keys, ", ")))
		}
		p := pointer(ptr, comb)
		if comb == "not" {
			if sub := parseCondition(raw, p, fs); sub != nil {
				c.Not = sub
			} else {
				c.Not = &Condition{}
			}
			return c
		}
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
			fs.errorf(CodeCondition, p, fmt.Sprintf("%q takes a non-empty array of conditions", comb))
		}
		subs := make([]*Condition, 0, len(list))
		for i, r := range list {
			if sub := parseCondition(r, fmt.Sprintf("%s/%d", p, i), fs); sub != nil {
				subs = append(subs, sub)
			}
		}
		if comb == "all" {
			c.All = subs
		} else {
			c.Any = subs
		}
		return c
	}

	c := &Condition{}
	for _, k := range keys {
		switch {
		case isOneOf(k, quantifiers):
			if c.Quantifier != "" {
				fs.errorf(CodeCondition, pointer(ptr, k), fmt.Sprintf("a condition reads one ref; %q and %q are both given", c.Quantifier, k))
				continue
			}
			c.Quantifier = k
			if err := json.Unmarshal(m[k], &c.Ref); err != nil {
				fs.errorf(CodeCondition, pointer(ptr, k), fmt.Sprintf("%q takes a ref as a string, such as \"step.column\" or \"params.name\"", k))
			}
		case isOneOf(k, operators):
			if c.Op != "" {
				fs.errorf(CodeCondition, pointer(ptr, k), fmt.Sprintf("a condition has one operator; %q and %q are both given", c.Op, k))
				continue
			}
			c.Op = k
			c.Operand = m[k]
		default:
			fs.errorf(CodeCondition, pointer(ptr, k), fmt.Sprintf("unknown condition key %q (want one of output, all_of, any_of, count; one of eq, ne, lt, le, gt, ge, in, empty; or all, any, not)", k))
		}
	}
	if c.Quantifier == "" {
		fs.errorf(CodeCondition, ptr, "a condition names its ref with output, all_of, any_of or count")
	}
	if c.Op == "" {
		fs.errorf(CodeCondition, ptr, "a condition has an operator: eq, ne, lt, le, gt, ge, in or empty")
		return c
	}
	p := pointer(ptr, c.Op)
	switch c.Op {
	case OpIn:
		var arr []json.RawMessage
		if err := json.Unmarshal(c.Operand, &arr); err != nil {
			fs.errorf(CodeCondition, p, "\"in\" takes an array")
		}
	case OpEmpty:
		if c.Quantifier == QCount {
			fs.errorf(CodeCondition, p, "\"count\" compares a length; test for an empty list with {\"output\": ref, \"empty\": true} or {\"count\": ref, \"eq\": 0}")
		}
		var b bool
		if err := json.Unmarshal(c.Operand, &b); err != nil {
			fs.errorf(CodeCondition, p, "\"empty\" takes a boolean")
		}
	default:
		if t := bytes.TrimSpace(c.Operand); len(t) > 0 && (t[0] == '[' || t[0] == '{') {
			fs.errorf(CodeCondition, p, fmt.Sprintf("%q compares with a scalar; use \"in\" for a set", c.Op))
		}
	}
	return c
}

// Eval evaluates the condition over env.
//
// A leaf reads its ref's Value and compares with the operand coerced to the
// value's base type: integers and numerics numerically, timestamps as times,
// session ids as numbers, text as text. A scalar is read as a one-element list
// by all_of, any_of and count, because a query that returned one row makes
// each column a scalar. NULL equals only NULL, and every ordering against it
// is false.
//
//	output   the value must be a scalar, except under "empty"
//	all_of   every element satisfies the comparison; true for an empty list
//	any_of   some element does; false for an empty list
//	count    the number of elements, compared as an integer
//	empty    true when the value is NULL, an empty text or an empty list
//
// A ref with no value (a step that was skipped or has not run) is an error,
// not false: a condition that cannot be evaluated does not quietly pass or
// fail.
func (c *Condition) Eval(env Env) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("no condition")
	}
	switch {
	case c.All != nil:
		for _, s := range c.All {
			ok, err := s.Eval(env)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case c.Any != nil:
		for _, s := range c.Any {
			ok, err := s.Eval(env)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case c.Not != nil:
		ok, err := c.Not.Eval(env)
		return !ok, err
	}

	v, ok := env.Lookup(c.Ref)
	if !ok {
		return false, fmt.Errorf("%s has no value: its step was skipped or has not run", c.Ref)
	}
	base := v.Type.Base
	if c.Quantifier == QCount {
		base = TypeInteger
	}
	if c.Op == OpEmpty {
		var want bool
		if err := json.Unmarshal(c.Operand, &want); err != nil {
			return false, fmt.Errorf("%s: \"empty\" takes a boolean", c.Ref)
		}
		switch c.Quantifier {
		case QOutput:
			return isEmpty(v) == want, nil
		case QAllOf, QAnyOf:
			n := 0
			elems := v.Elems()
			for _, e := range elems {
				if isEmptyScalar(e) == want {
					n++
				}
			}
			if c.Quantifier == QAllOf {
				return n == len(elems), nil
			}
			return n > 0, nil
		}
		return false, fmt.Errorf("%s: %q cannot be used with \"empty\"", c.Ref, c.Quantifier)
	}

	operands, err := c.operands(base)
	if err != nil {
		return false, fmt.Errorf("%s: %v", c.Ref, err)
	}
	test := func(x any) (bool, error) {
		x, err := coerceScalar(base, x)
		if err != nil {
			return false, fmt.Errorf("%s holds a value that is not a %s: %v", c.Ref, base, err)
		}
		return compareOp(base, c.Op, x, operands)
	}

	switch c.Quantifier {
	case QOutput:
		if v.Type.List {
			return false, fmt.Errorf("%s is a list of %d; read it with all_of, any_of or count", c.Ref, v.Len())
		}
		return test(v.V)
	case QCount:
		return test(int64(v.Len()))
	case QAllOf:
		for _, e := range v.Elems() {
			ok, err := test(e)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case QAnyOf:
		for _, e := range v.Elems() {
			ok, err := test(e)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("unknown quantifier %q", c.Quantifier)
}

// operands coerces the operand to base: one value, or the elements of "in".
func (c *Condition) operands(base string) ([]any, error) {
	if c.Op == OpIn {
		var arr []json.RawMessage
		if err := json.Unmarshal(c.Operand, &arr); err != nil {
			return nil, fmt.Errorf("\"in\" takes an array")
		}
		out := make([]any, len(arr))
		for i, r := range arr {
			v, err := ParseValue(Type{Base: base}, r)
			if err != nil {
				return nil, fmt.Errorf("\"in\" element %d: %v", i, err)
			}
			out[i] = v.V
		}
		return out, nil
	}
	raw := c.Operand
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("null")
	}
	v, err := ParseValue(Type{Base: base}, raw)
	if err != nil {
		return nil, fmt.Errorf("%q operand: %v", c.Op, err)
	}
	return []any{v.V}, nil
}

func compareOp(base, op string, x any, operands []any) (bool, error) {
	if op == OpIn {
		for _, o := range operands {
			if eq, err := equal(base, x, o); err != nil || eq {
				return eq, err
			}
		}
		return false, nil
	}
	o := operands[0]
	switch op {
	case OpEq:
		return equal(base, x, o)
	case OpNe:
		eq, err := equal(base, x, o)
		return !eq, err
	}
	cmp, ok, err := compareScalars(base, x, o)
	if err != nil || !ok {
		return false, err
	}
	switch op {
	case OpLt:
		return cmp < 0, nil
	case OpLe:
		return cmp <= 0, nil
	case OpGt:
		return cmp > 0, nil
	case OpGe:
		return cmp >= 0, nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

func equal(base string, a, b any) (bool, error) {
	if a == nil || b == nil {
		return a == nil && b == nil, nil
	}
	cmp, _, err := compareScalars(base, a, b)
	return cmp == 0, err
}

func isEmpty(v Value) bool {
	if v.Type.List {
		return v.Len() == 0
	}
	return isEmptyScalar(v.V)
}

func isEmptyScalar(x any) bool {
	if x == nil {
		return true
	}
	s, ok := x.(string)
	return ok && s == ""
}
