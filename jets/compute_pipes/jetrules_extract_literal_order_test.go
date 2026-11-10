package compute_pipes

// The order of a multi-valued literal column (JM-047). extractLiteralValue
// collected a subject's values in the order the graph returned them, which is
// the order of a hash set seeded per process, so the same data gave cells whose
// values were listed differently from one run to the next. The values are now
// sorted by type: numbers numerically, dates chronologically, text bytewise.
//
// The stub nodes are jetrules_extract_entity_order_test.go's.

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func TestSortCellValues(t *testing.T) {
	cases := []struct {
		name string
		in   []any
		want []any
	}{
		{"numbers numerically, not as text",
			[]any{10, 9, 100, -1},
			[]any{-1, 9, 10, 100}},
		{"ints, uints and doubles together",
			[]any{10, 2.5, uint(3), 9, -0.5},
			[]any{-0.5, 2.5, uint(3), 9, 10}},
		{"dates chronologically",
			[]any{date(2026, 1, 2), date(2025, 12, 31), date(2026, 1, 1)},
			[]any{date(2025, 12, 31), date(2026, 1, 1), date(2026, 1, 2)}},
		{"datetimes chronologically",
			[]any{date(2026, 1, 1).Add(time.Hour), date(2026, 1, 1).Add(time.Minute), date(2026, 1, 1)},
			[]any{date(2026, 1, 1), date(2026, 1, 1).Add(time.Minute), date(2026, 1, 1).Add(time.Hour)}},
		{"text bytewise",
			[]any{"b", "a", "B", "9", "10"},
			[]any{"10", "9", "B", "a", "b"}},
		{"mixed: null, numbers, dates, text",
			[]any{"x", 3, date(2026, 1, 1), nil, 1.5, "a"},
			[]any{nil, 1.5, 3, date(2026, 1, 1), "a", "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := append([]any(nil), c.in...)
			sortCellValues(got)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// The sorted cell must be the same whatever order the values arrive in,
// including for values that compare equal by number and differ by type or sign,
// NaN, and integers too large for a double to tell apart.
func TestSortCellValuesIgnoresInputOrder(t *testing.T) {
	const big = 1 << 62
	values := []any{
		1, uint(1), 1.0, 0, 0.0, math.Copysign(0, -1), math.NaN(), math.Inf(-1), math.Inf(1),
		big, big + 1, float64(big), uint(big + 2), -3,
		date(2026, 3, 1), date(2026, 3, 1).In(time.FixedZone("x", 3600)),
		"z", "", nil, true,
	}
	// With the type, so that values printing alike are told apart; DeepEqual
	// cannot be used, as NaN is not equal to itself.
	render := func(vs []any) string {
		var s string
		for _, v := range vs {
			s += fmt.Sprintf("%T:%v|", v, v)
		}
		return s
	}
	first := append([]any(nil), values...)
	sortCellValues(first)
	want := render(first)
	r := rand.New(rand.NewSource(47))
	for range 500 {
		got := append([]any(nil), values...)
		r.Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		sortCellValues(got)
		if g := render(got); g != want {
			t.Fatalf("order depends on input order:\n got %s\nwant %s", g, want)
		}
	}
}

func TestExtractLiteralValueSortsMultipleValues(t *testing.T) {
	ctx := &JrPoolWorker{
		config:                   &JetrulesSpec{},
		multiValueDataProperties: map[string]bool{"m:nums": true, "m:dates": true},
		column2RdfType:           map[string]string{"s:note": "text", "s:count": "int"},
	}
	extract := func(pname string, objects ...RdfNode) any {
		return ctx.extractLiteralValue(nil, res("subject"), pname, objects, 0, &JetrulesOutputChan{})
	}

	if got, want := extract("m:nums", num(10), num(9), dbl(9.5)), []any{9, 9.5, 10}; !reflect.DeepEqual(got, want) {
		t.Errorf("multi-value numbers: got %v, want %v", got, want)
	}
	if got, want := extract("m:dates", day(2026, 2, 1), day(2025, 7, 4)), []any{date(2025, 7, 4), date(2026, 2, 1)}; !reflect.DeepEqual(got, want) {
		t.Errorf("multi-value dates: got %v, want %v", got, want)
	}
	// A text property that is not multi-value keeps its values, sorted.
	if got, want := extract("s:note", txt("b"), txt("a")), []any{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("single-value text with two values: got %v, want %v", got, want)
	}
	// One value is untouched: a scalar for a single-value property, a
	// one-element array for a multi-value one.
	if got := extract("s:count", num(10)); got != 10 {
		t.Errorf("single value: got %v, want 10", got)
	}
	if got, want := extract("m:nums", num(10)), []any{10}; !reflect.DeepEqual(got, want) {
		t.Errorf("multi-value with one value: got %v, want %v", got, want)
	}
}
