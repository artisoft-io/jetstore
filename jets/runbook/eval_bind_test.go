package runbook

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func cond(t *testing.T, s string) *Condition {
	t.Helper()
	var c Condition
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		t.Fatalf("%s: %v", s, err)
	}
	return &c
}

func mustValue(t *testing.T, typ string, v any) Value {
	t.Helper()
	ty, err := ParseType(typ)
	if err != nil {
		t.Fatal(err)
	}
	val, err := NewValue(ty, v)
	if err != nil {
		t.Fatal(err)
	}
	return val
}

func testEnv(t *testing.T) MapEnv {
	ts := time.Date(2026, 11, 6, 17, 0, 0, 0, time.UTC)
	return MapEnv{
		"params.since": mustValue(t, "timestamp", "2026-11-06T17:00:00Z"),
		"params.ids":   mustValue(t, "list<session_id>", []string{"9", "10"}),
		"check.n":      mustValue(t, "integer", 9),
		"check.ratio":  mustValue(t, "numeric", 0.5),
		"check.label":  mustValue(t, "text", "9"),
		"check.sid":    mustValue(t, "session_id", "9"),
		"check.flag":   mustValue(t, "boolean", true),
		"check.blank":  mustValue(t, "text", ""),
		"check.null":   {Type: Type{Base: TypeText}, V: nil},
		"check.at":     {Type: Type{Base: TypeTimestamp}, V: ts.Add(time.Hour)},
		"many.state":   mustValue(t, "list<text>", []any{"completed", "completed", "ready"}),
		"none.state":   mustValue(t, "list<text>", []any{}),
		"done.state":   mustValue(t, "list<text>", []any{"completed", "completed"}),
		"odd.n":        {Type: Type{Base: TypeInteger}, V: int(3)}, // not canonical; Eval coerces
		"nulls.state":  mustValue(t, "list<text>", []any{"a", nil}),
	}
}

func TestEval(t *testing.T) {
	env := testEnv(t)
	cases := map[string]bool{
		// Integers compare numerically; text compares as text.
		`{"output": "check.n", "lt": 10}`:        true,
		`{"output": "check.label", "lt": "10"}`:  false,
		`{"output": "check.sid", "lt": "10"}`:    true,
		`{"output": "check.n", "eq": 9}`:         true,
		`{"output": "check.n", "ne": 9}`:         false,
		`{"output": "check.n", "ge": 9}`:         true,
		`{"output": "check.n", "gt": 9}`:         false,
		`{"output": "check.n", "le": 8}`:         false,
		`{"output": "check.n", "in": [1, 9]}`:    true,
		`{"output": "check.n", "in": [1, 2]}`:    false,
		`{"output": "check.ratio", "gt": 0.25}`:  true,
		`{"output": "check.ratio", "eq": "0.5"}`: true,
		`{"output": "check.flag", "eq": true}`:   true,
		`{"output": "odd.n", "eq": 3}`:           true,
		// Timestamps as times, whatever the spelling.
		`{"output": "check.at", "gt": "2026-11-06T17:30:00Z"}`:      true,
		`{"output": "check.at", "eq": "2026-11-06T13:00:00-05:00"}`: true,
		`{"output": "params.since", "lt": "2026-11-06 18:00:00"}`:   true,
		// empty
		`{"output": "check.blank", "empty": true}`:  true,
		`{"output": "check.null", "empty": true}`:   true,
		`{"output": "check.label", "empty": true}`:  false,
		`{"output": "none.state", "empty": true}`:   true,
		`{"output": "many.state", "empty": false}`:  true,
		`{"all_of": "nulls.state", "empty": false}`: false,
		`{"any_of": "nulls.state", "empty": true}`:  true,
		// NULL
		`{"output": "check.null", "eq": null}`: true,
		`{"output": "check.null", "eq": "x"}`:  false,
		`{"output": "check.null", "ne": "x"}`:  true,
		`{"output": "check.null", "lt": "x"}`:  false,
		`{"output": "check.null", "ge": "x"}`:  false,
		// Quantifiers.
		`{"all_of": "many.state", "eq": "completed"}`:  false,
		`{"all_of": "done.state", "eq": "completed"}`:  true,
		`{"any_of": "many.state", "eq": "ready"}`:      true,
		`{"all_of": "none.state", "eq": "completed"}`:  true,
		`{"any_of": "none.state", "eq": "completed"}`:  false,
		`{"any_of": "params.ids", "in": ["10", "11"]}`: true,
		`{"count": "many.state", "eq": 3}`:             true,
		`{"count": "none.state", "eq": 0}`:             true,
		`{"count": "params.ids", "gt": 1}`:             true,
		// A one-row result is a scalar; the list quantifiers read it as one element.
		`{"all_of": "check.label", "eq": "9"}`: true,
		`{"count": "check.n", "eq": 1}`:        true,
		// Combinators.
		`{"all": [{"output": "check.n", "eq": 9}, {"count": "many.state", "eq": 3}]}`: true,
		`{"all": [{"output": "check.n", "eq": 9}, {"count": "many.state", "eq": 2}]}`: false,
		`{"any": [{"output": "check.n", "eq": 1}, {"count": "many.state", "eq": 3}]}`: true,
		`{"not": {"output": "check.n", "eq": 9}}`:                                     false,
		// any short-circuits before a ref that would fail.
		`{"any": [{"output": "check.n", "eq": 9}, {"output": "missing.x", "eq": 1}]}`: true,
	}
	for src, want := range cases {
		got, err := cond(t, src).Eval(env)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got != want {
			t.Errorf("%s: got %v, want %v", src, got, want)
		}
	}
	errs := map[string]string{
		`{"output": "missing.x", "eq": 1}`:                                            "has no value",
		`{"output": "many.state", "eq": "completed"}`:                                 "is a list",
		`{"output": "check.n", "eq": "nine"}`:                                         "not an integer",
		`{"all": [{"output": "check.n", "eq": 9}, {"output": "missing.x", "eq": 1}]}`: "has no value",
	}
	for src, want := range errs {
		_, err := cond(t, src).Eval(env)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an error containing %q", src, err, want)
		}
	}
}

func TestConditionUnmarshalRefusesTheGrammar(t *testing.T) {
	for _, src := range []string{`[]`, `{"output": "a.b"}`, `{"output": "a.b", "eq": 1, "x": 2}`, `{"all": []}`} {
		var c Condition
		if err := json.Unmarshal([]byte(src), &c); err == nil {
			t.Errorf("%s: accepted", src)
		}
	}
	c := cond(t, `{"any": [{"output": "a.b", "in": [1, 2]}, {"not": {"count": "c.d", "ge": 3}}]}`)
	if got := c.Refs(); !reflect.DeepEqual(got, []string{"a.b", "c.d"}) {
		t.Errorf("Refs: %v", got)
	}
}

// --- Bind --------------------------------------------------------------------

func TestBind(t *testing.T) {
	env := testEnv(t)
	step := &Step{ID: "s", Type: StepQuery, Bind: map[string]string{"sessions": "params.ids", "n": "check.n"}}

	cases := []struct {
		name  string
		sql   string
		want  []string
		names [][]string
	}{
		{"a cast is not a name", "SELECT :since::date, x::text FROM t WHERE n = :n",
			[]string{"SELECT $1::date, x::text FROM t WHERE n = $2"}, [][]string{{"since", "n"}}},
		{"a repeated name is one parameter", "SELECT :n, :since, :n",
			[]string{"SELECT $1, $2, $1"}, [][]string{{"n", "since"}}},
		{"strings, quoted identifiers and E-strings are left alone",
			"SELECT ':n', 'it''s :n', E'\\' :n', \"a:n\" FROM t WHERE x = :n",
			[]string{"SELECT ':n', 'it''s :n', E'\\' :n', \"a:n\" FROM t WHERE x = $1"}, [][]string{{"n"}}},
		{"comments are left alone", "SELECT 1 -- :n\n/* :n /* nested :n */ :n */ WHERE x = :n",
			[]string{"SELECT 1 -- :n\n/* :n /* nested :n */ :n */ WHERE x = $1"}, [][]string{{"n"}}},
		{"dollar bodies are left alone", "DO $body$ BEGIN x := :n; END $body$",
			[]string{"DO $body$ BEGIN x := :n; END $body$"}, [][]string{nil}},
		{"numbered per statement", "-- lead\nUPDATE t SET a = :n;\n\nDELETE FROM t WHERE s = ANY(:sessions) AND b < :n;",
			[]string{"UPDATE t SET a = $1", "DELETE FROM t WHERE s = ANY($1) AND b < $2"}, [][]string{{"n"}, {"sessions", "n"}}},
		{"an assignment is not a name", "SELECT a := 1", []string{"SELECT a := 1"}, [][]string{nil}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Bind(c.sql, step, env)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d statements, want %d: %+v", len(got), len(c.want), got)
			}
			for i := range got {
				if got[i].SQL != c.want[i] {
					t.Errorf("statement %d:\n got %s\nwant %s", i, got[i].SQL, c.want[i])
				}
				if !reflect.DeepEqual(got[i].Names, c.names[i]) {
					t.Errorf("statement %d names: got %v, want %v", i, got[i].Names, c.names[i])
				}
				if len(got[i].Args) != len(got[i].Names) {
					t.Errorf("statement %d: %d args for %d names", i, len(got[i].Args), len(got[i].Names))
				}
			}
		})
	}

	t.Run("values are typed for pgx", func(t *testing.T) {
		got, err := Bind("SELECT :n, :since, :sessions, :ratio, :flag", &Step{Bind: map[string]string{
			"n": "check.n", "sessions": "params.ids", "ratio": "check.ratio", "flag": "check.flag"}}, env)
		if err != nil {
			t.Fatal(err)
		}
		args := got[0].Args
		if _, ok := args[0].(int64); !ok {
			t.Errorf("integer: %T", args[0])
		}
		if _, ok := args[1].(time.Time); !ok {
			t.Errorf("timestamp: %T", args[1])
		}
		if l, ok := args[2].([]string); !ok || !reflect.DeepEqual(l, []string{"9", "10"}) {
			t.Errorf("list<session_id>: %T %v", args[2], args[2])
		}
		if _, ok := args[3].(float64); !ok {
			t.Errorf("numeric: %T", args[3])
		}
		if _, ok := args[4].(bool); !ok {
			t.Errorf("boolean: %T", args[4])
		}
	})
	t.Run("a list with a NULL is a slice of pointers", func(t *testing.T) {
		got, err := Bind("SELECT :s", &Step{Bind: map[string]string{"s": "nulls.state"}}, env)
		if err != nil {
			t.Fatal(err)
		}
		l, ok := got[0].Args[0].([]*string)
		if !ok || len(l) != 2 || *l[0] != "a" || l[1] != nil {
			t.Errorf("got %T %v", got[0].Args[0], got[0].Args[0])
		}
		got, _ = Bind("SELECT :s", &Step{Bind: map[string]string{"s": "none.state"}}, env)
		if l, ok := got[0].Args[0].([]string); !ok || len(l) != 0 {
			t.Errorf("empty list: %T %v", got[0].Args[0], got[0].Args[0])
		}
	})
	refused := map[string]string{
		"SELECT :nope":                  "params.nope, which has no value",
		"SELECT 1 WHERE s = $SESSIONID": "text substitution",
		"SELECT ${x}":                   "text substitution",
		"SELECT {{x}}":                  "text substitution",
		"SELECT $1":                     "positional",
		"SELECT 'open":                  "unterminated",
	}
	for sql, want := range refused {
		_, err := Bind(sql, step, env)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an error containing %q", sql, err, want)
		}
	}
}

// --- values ------------------------------------------------------------------

func TestValues(t *testing.T) {
	ok := []struct {
		typ  string
		in   any
		want any
	}{
		{"integer", json.Number("12"), int64(12)},
		{"integer", 12.0, int64(12)},
		{"integer", "12", int64(12)},
		{"integer", int32(12), int64(12)},
		{"numeric", json.Number("1.5"), 1.5},
		{"numeric", 3, 3.0},
		{"session_id", json.Number("123"), "123"},
		{"session_id", int64(123), "123"},
		{"boolean", "true", true},
		{"text", []byte("x"), "x"},
		{"timestamp", "2026-11-06", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)},
		{"text", nil, nil},
	}
	for _, c := range ok {
		v, err := NewValue(Type{Base: c.typ}, c.in)
		if err != nil || !reflect.DeepEqual(v.V, c.want) {
			t.Errorf("%s %v: got %v %v, want %v", c.typ, c.in, v.V, err, c.want)
		}
	}
	bad := []struct {
		typ string
		in  any
	}{
		{"integer", 1.5}, {"integer", "x"}, {"session_id", "12a"}, {"session_id", ""}, {"boolean", "maybe"},
		{"timestamp", "yesterday"}, {"text", 3}, {"list<text>", "a"}, {"list<integer>", nil}, {"list<integer>", []any{"x"}},
	}
	for _, c := range bad {
		ty, _ := ParseType(c.typ)
		if _, err := NewValue(ty, c.in); err == nil {
			t.Errorf("%s %v: accepted", c.typ, c.in)
		}
	}
}

func TestParameterValues(t *testing.T) {
	rb, _ := Decode([]byte(`{"format":1,"runbook":"r","parameters":{
		"since":{"type":"timestamp"},
		"ids":{"type":"list<session_id>","default":["1"]}},"steps":[]}`))
	got, err := ParameterValues(rb, map[string]json.RawMessage{"since": json.RawMessage(`"2026-11-06T17:00:00Z"`)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got["ids"].V, []any{"1"}) || !got["ids"].Type.List {
		t.Errorf("default: %+v", got["ids"])
	}
	if _, err := ParameterValues(rb, nil); err == nil || !strings.Contains(err.Error(), "since") {
		t.Errorf("missing: %v", err)
	}
	if _, err := ParameterValues(rb, map[string]json.RawMessage{"since": json.RawMessage(`"2026-11-06"`), "x": json.RawMessage(`1`)}); err == nil {
		t.Error("undeclared: accepted")
	}
	if _, err := ParameterValues(rb, map[string]json.RawMessage{"since": json.RawMessage(`"soon"`)}); err == nil {
		t.Error("bad timestamp: accepted")
	}
}

func TestQueryOutputs(t *testing.T) {
	step := &Step{ID: "q", Type: StepQuery, Returns: map[string]string{"state": "text", "n": "integer", "ids": "list<session_id>"}}
	cols := []string{"n", "state", "ids"}

	one, err := QueryOutputs(step, cols, [][]any{{int32(4), "ready", "7"}})
	if err != nil {
		t.Fatal(err)
	}
	if one["n"].Type.List || one["n"].V != int64(4) || one["state"].V != "ready" {
		t.Errorf("one row: %+v", one)
	}
	if !one["ids"].Type.List || !reflect.DeepEqual(one["ids"].V, []any{"7"}) {
		t.Errorf("list<T> forces a list for one row: %+v", one["ids"])
	}

	two, err := QueryOutputs(step, cols, [][]any{{1, "a", "1"}, {2, nil, "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !two["n"].Type.List || !reflect.DeepEqual(two["state"].V, []any{"a", nil}) {
		t.Errorf("two rows: %+v", two)
	}
	zero, err := QueryOutputs(step, cols, nil)
	if err != nil || !zero["state"].Type.List || zero["state"].Len() != 0 {
		t.Errorf("zero rows: %+v %v", zero, err)
	}

	for name, c := range map[string][]string{
		"an undeclared column": {"n", "state", "ids", "extra"},
		"a missing column":     {"n", "state"},
		"a column twice":       {"n", "n", "state", "ids"},
	} {
		if _, err := QueryOutputs(step, c, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := QueryOutputs(step, cols, [][]any{{"x", "a", "1"}}); err == nil {
		t.Error("a value of the wrong type: accepted")
	}
}

// --- the result --------------------------------------------------------------

// The JSON names of §3.5 of the plan, exactly.
func TestResultJsonNames(t *testing.T) {
	keys := func(v any) []string {
		b, _ := json.Marshal(v)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	check := func(what string, got []string, want string) {
		w := strings.Fields(want)
		sort.Strings(w)
		if !reflect.DeepEqual(got, w) {
			t.Errorf("%s: got %v, want %v", what, got, w)
		}
	}
	check("Result", keys(Result{}), "format runbook workspace run_key status parameters environment revisions "+
		"started_by started_at finished_at released_by released_at steps")
	check("Environment", keys(Environment{}), "jetstore_release workspace_version")
	check("Revision", keys(Revision{}), "revision files from_step")
	check("StepRecord", keys(StepRecord{}), "id type iteration revision status sql_hash sql bound preview decision "+
		"outputs rows row_count truncated started_at finished_at actor error")
	check("Decision", keys(Decision{}), "by at accepted comment")
}
