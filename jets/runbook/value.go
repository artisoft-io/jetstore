package runbook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Base types of parameters, `returns` and `records`.
const (
	TypeText      = "text"
	TypeInteger   = "integer"
	TypeNumeric   = "numeric"
	TypeBoolean   = "boolean"
	TypeTimestamp = "timestamp"
	// TypeSessionID is a text of digits.
	TypeSessionID = "session_id"
)

var baseTypes = map[string]bool{
	TypeText: true, TypeInteger: true, TypeNumeric: true,
	TypeBoolean: true, TypeTimestamp: true, TypeSessionID: true,
}

// Type is a declared type: a base type, or `list<base>`.
type Type struct {
	Base string
	List bool
}

// ParseType reads `text`, `list<integer>` and so on. Lists do not nest.
func ParseType(s string) (Type, error) {
	t := strings.TrimSpace(s)
	if strings.HasPrefix(t, "list<") && strings.HasSuffix(t, ">") {
		inner := strings.TrimSpace(t[len("list<") : len(t)-1])
		if baseTypes[inner] {
			return Type{Base: inner, List: true}, nil
		}
		return Type{}, fmt.Errorf("unknown list element type %q", inner)
	}
	if baseTypes[t] {
		return Type{Base: t}, nil
	}
	return Type{}, fmt.Errorf("unknown type %q (want text, integer, numeric, boolean, timestamp, session_id or list<T> of one)", s)
}

func (t Type) String() string {
	if t.List {
		return "list<" + t.Base + ">"
	}
	return t.Base
}

// Elem is the type of one element: the type itself for a scalar.
func (t Type) Elem() Type { return Type{Base: t.Base} }

// Value is a typed value in an Env: a parameter, or a step's output.
//
// V holds, for a scalar, nil (SQL NULL) or the canonical Go value of the base
// type:
//
//	text, session_id  string
//	integer           int64
//	numeric           float64
//	boolean           bool
//	timestamp         time.Time, in UTC
//
// and for a list, a []any of those (nil elements allowed). A query output
// declared as a scalar type may still hold a list at run time — a result with
// other than one row makes each column a list — so read Type.List and V
// together: a list Value has Type.List true.
type Value struct {
	Type Type
	V    any
}

// Len is the number of elements: a list's length, 1 for a scalar.
func (v Value) Len() int {
	if v.Type.List {
		l, _ := v.V.([]any)
		return len(l)
	}
	return 1
}

// Elems is a list's elements, or a scalar as a one-element list.
func (v Value) Elems() []any {
	if v.Type.List {
		l, _ := v.V.([]any)
		return l
	}
	return []any{v.V}
}

// MarshalJSON writes V: a timestamp as RFC 3339, the rest as JSON.
func (v Value) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.V)
}

// Env resolves a ref to a value: `params.<name>`, or `<step_id>.<column>`.
type Env interface {
	Lookup(ref string) (Value, bool)
}

// MapEnv is an Env keyed by ref: "params.since_utc", "check.state".
type MapEnv map[string]Value

// Lookup implements Env.
func (m MapEnv) Lookup(ref string) (Value, bool) {
	v, ok := m[ref]
	return v, ok
}

// ParamRef is the ref of a parameter: "params.<name>".
func ParamRef(name string) string { return "params." + name }

// OutputRef is the ref of a step's output: "<step_id>.<column>".
func OutputRef(stepID, column string) string { return stepID + "." + column }

// NewValue coerces a Go value to t. It accepts the canonical values above and
// what a JSON decoder or a database driver commonly hands over: any integer or
// float kind, json.Number, a decimal or RFC 3339 string, and for a list any
// slice. It refuses a value that does not fit (a fractional integer, a
// session id that is not all digits).
func NewValue(t Type, v any) (Value, error) {
	if !t.List {
		c, err := coerceScalar(t.Base, v)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: t, V: c}, nil
	}
	if v == nil {
		return Value{}, fmt.Errorf("a %s cannot be null", t)
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return Value{}, fmt.Errorf("a %s must be an array, got %T", t, v)
	}
	out := make([]any, rv.Len())
	for i := range out {
		c, err := coerceScalar(t.Base, rv.Index(i).Interface())
		if err != nil {
			return Value{}, fmt.Errorf("element %d: %w", i, err)
		}
		out[i] = c
	}
	return Value{Type: t, V: out}, nil
}

// ParseValue decodes a JSON value as t: a parameter given at start, a
// default, a manual record, a condition's operand.
func ParseValue(t Type, raw json.RawMessage) (Value, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return Value{}, fmt.Errorf("not a JSON value: %v", err)
	}
	return NewValue(t, v)
}

// ListValue builds a list Value of base type b from scalars: what a query
// with other than one row produces for a column.
func ListValue(base string, elems []any) (Value, error) {
	return NewValue(Type{Base: base, List: true}, elems)
}

var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999Z07",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
}

func coerceScalar(base string, v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	if p := reflect.ValueOf(v); p.Kind() == reflect.Pointer {
		if p.IsNil() {
			return nil, nil
		}
		return coerceScalar(base, p.Elem().Interface())
	}
	switch base {
	case TypeText:
		switch x := v.(type) {
		case string:
			return x, nil
		case []byte:
			return string(x), nil
		}
	case TypeSessionID:
		var s string
		switch x := v.(type) {
		case string:
			s = x
		case json.Number:
			s = x.String()
		default:
			if i, ok := asInt64(v); ok {
				s = strconv.FormatInt(i, 10)
			}
		}
		if s != "" && isDigits(s) {
			return s, nil
		}
		return nil, fmt.Errorf("a session_id must be a text of digits, got %v", v)
	case TypeInteger:
		if i, ok := asInt64(v); ok {
			return i, nil
		}
		switch x := v.(type) {
		case json.Number:
			if i, err := x.Int64(); err == nil {
				return i, nil
			}
		case string:
			if i, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64); err == nil {
				return i, nil
			}
		case float64, float32:
			f := reflect.ValueOf(x).Float()
			if f == math.Trunc(f) && math.Abs(f) < 1<<63 {
				return int64(f), nil
			}
		}
		return nil, fmt.Errorf("not an integer: %v", v)
	case TypeNumeric:
		if i, ok := asInt64(v); ok {
			return float64(i), nil
		}
		switch x := v.(type) {
		case float64:
			return x, nil
		case float32:
			return float64(x), nil
		case json.Number:
			if f, err := x.Float64(); err == nil {
				return f, nil
			}
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
				return f, nil
			}
		}
		return nil, fmt.Errorf("not a number: %v", v)
	case TypeBoolean:
		switch x := v.(type) {
		case bool:
			return x, nil
		case string:
			if b, err := strconv.ParseBool(strings.TrimSpace(x)); err == nil {
				return b, nil
			}
		}
		return nil, fmt.Errorf("not a boolean: %v", v)
	case TypeTimestamp:
		switch x := v.(type) {
		case time.Time:
			return x.UTC(), nil
		case string:
			s := strings.TrimSpace(x)
			for _, layout := range timestampLayouts {
				if t, err := time.Parse(layout, s); err == nil {
					return t.UTC(), nil
				}
			}
			return nil, fmt.Errorf("not a timestamp (want RFC 3339, e.g. 2026-11-06T17:00:00Z): %q", x)
		}
		return nil, fmt.Errorf("not a timestamp: %v", v)
	default:
		return nil, fmt.Errorf("unknown type %q", base)
	}
	return nil, fmt.Errorf("not a %s: %v (%T)", base, v, v)
}

func asInt64(v any) (int64, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := rv.Uint()
		if u <= math.MaxInt64 {
			return int64(u), true
		}
	}
	return 0, false
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// compareScalars orders two canonical scalars of one base type. ok is false
// when either is NULL, which no comparison but eq and ne can see.
func compareScalars(base string, a, b any) (cmp int, ok bool, err error) {
	if a == nil || b == nil {
		return 0, false, nil
	}
	switch base {
	case TypeText:
		x, y := a.(string), b.(string)
		return strings.Compare(x, y), true, nil
	case TypeSessionID:
		// Digits: compare as numbers, so "9" < "10".
		x, okx := new(big.Int).SetString(a.(string), 10)
		y, oky := new(big.Int).SetString(b.(string), 10)
		if !okx || !oky {
			return strings.Compare(a.(string), b.(string)), true, nil
		}
		return x.Cmp(y), true, nil
	case TypeInteger:
		x, y := a.(int64), b.(int64)
		switch {
		case x < y:
			return -1, true, nil
		case x > y:
			return 1, true, nil
		}
		return 0, true, nil
	case TypeNumeric:
		x, y := a.(float64), b.(float64)
		switch {
		case x < y:
			return -1, true, nil
		case x > y:
			return 1, true, nil
		}
		return 0, true, nil
	case TypeBoolean:
		x, y := a.(bool), b.(bool)
		switch {
		case x == y:
			return 0, true, nil
		case !x:
			return -1, true, nil
		}
		return 1, true, nil
	case TypeTimestamp:
		return a.(time.Time).Compare(b.(time.Time)), true, nil
	}
	return 0, false, fmt.Errorf("cannot compare values of type %q", base)
}
