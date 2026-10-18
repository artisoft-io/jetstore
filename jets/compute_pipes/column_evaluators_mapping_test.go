package compute_pipes

import (
	"strings"
	"testing"
)

// Tests for the map column evaluator when the value is rejected by the cast to
// rdf type. The case that matters is map_record without new_record, where the
// evaluator updates the input record in place: the output position then holds
// the raw input until the evaluator writes it.

func buildInPlaceMapEval(t *testing.T, name, expr string, mapExpr *MapExpression) (TransformationColumnEvaluator, *InputChannel) {
	t.Helper()
	columns := &map[string]int{"qty": 0, "amt": 1}
	source := &InputChannel{
		Name:    "in",
		Columns: columns,
		Config:  &ChannelSpec{Name: "in"},
	}
	outCh := &OutputChannel{
		Name:    "out",
		Columns: columns,
		Config:  &ChannelSpec{Name: "out"},
	}
	ctx := &BuilderContext{}
	ev, err := ctx.BuildMapTCEvaluator(source, outCh, &TransformationColumnSpec{
		Name:    name,
		Type:    "map",
		Expr:    &expr,
		MapExpr: mapExpr,
	})
	if err != nil {
		t.Fatalf("BuildMapTCEvaluator: %v", err)
	}
	markPerRecord(ev)
	return ev, source
}

// buildAccumulatingMapEval builds a map column as an aggregate or map_reduce does:
// not marked per record, its output kept across records.
func buildAccumulatingMapEval(t *testing.T, mapExpr *MapExpression) TransformationColumnEvaluator {
	t.Helper()
	expr := "qty"
	source := &InputChannel{Name: "in", Columns: &map[string]int{"qty": 0}, Config: &ChannelSpec{Name: "in"}}
	outCh := &OutputChannel{Name: "agg", Columns: &map[string]int{"qty": 0}, Config: &ChannelSpec{Name: "agg"}}
	ctx := &BuilderContext{}
	ev, err := ctx.BuildTransformationColumnEvaluator(source, outCh, &TransformationColumnSpec{
		Name: "qty", Type: "map", Expr: &expr, MapExpr: mapExpr,
	})
	if err != nil {
		t.Fatalf("BuildTransformationColumnEvaluator: %v", err)
	}
	return ev
}

func TestMapCastFailureUsesDefault(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{Default: "0", RdfType: "double"})
	record := []any{"not_a_number", "x"}
	// in place: currentValue is the input record
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error when the default replaces a rejected value, got %v", err)
	}
	v, ok := record[0].(float64)
	if !ok || v != 0 {
		t.Errorf("expecting the default 0.0 (float64), got %v (%T)", record[0], record[0])
	}
}

func TestMapCastFailureWithoutDefaultIsNull(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "double"})
	record := []any{"not_a_number", "x"}
	err := ev.Update(&record, &record)
	if err == nil || !strings.Contains(err.Error(), "while casting value to rdf type 'double'") {
		t.Errorf("expecting the cast error to be reported, got %v", err)
	}
	if record[0] != nil {
		t.Errorf("expecting null in place of the rejected value, got %v (%T)", record[0], record[0])
	}
}

func TestMapCastSuccess(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{Default: "0", RdfType: "double"})
	record := []any{"12.5", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := record[0].(float64); !ok || v != 12.5 {
		t.Errorf("expecting 12.5, got %v (%T)", record[0], record[0])
	}
}

func TestMapNullInputUsesDefault(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{Default: "0", RdfType: "double"})
	record := []any{nil, "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := record[0].(float64); !ok || v != 0 {
		t.Errorf("expecting the default 0.0 (float64), got %v (%T)", record[0], record[0])
	}
}

// An absent input with no default leaves the output as it is: this is the
// behaviour the "don't overwrite with nil" rule exists for, and it must hold.
func TestMapNullInputWithoutDefaultKeepsOutput(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "amt", "qty", &MapExpression{RdfType: "text"})
	record := []any{nil, "kept"}
	if err := ev.Update(&record, &record); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record[1] != "kept" {
		t.Errorf("expecting the output to be left as is, got %v", record[1])
	}
}

// A cleansing function error without a default is reported and the rejected
// value does not pass through.
func TestMapCleansingErrorWithoutDefaultIsNull(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{
		CleansingFunction: "find_and_replace",
		Argument:          "not a valid argument",
		RdfType:           "text",
	})
	record := []any{"raw", "x"}
	if err := ev.Update(&record, &record); err == nil {
		t.Errorf("expecting the cleansing error to be reported")
	}
	if record[0] != nil {
		t.Errorf("expecting null in place of the rejected value, got %v (%T)", record[0], record[0])
	}
}

// End to end at the evaluator level: an in-place map with a double cast and a
// default, followed by a count aggregate whose where compares the column with a
// double. A value the cast rejects must reach the where as the default, not as
// the raw string, which made the where fail with "opLT string and double".
func TestMapCastFailureThenCountWhereLT(t *testing.T) {
	ev, source := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{Default: "0", RdfType: "double"})
	countOut := &OutputChannel{
		Name:    "agg",
		Columns: &map[string]int{"neg_qty": 0},
		Config:  &ChannelSpec{Name: "agg"},
	}
	star := "*"
	ctx := &BuilderContext{}
	count, err := ctx.BuildCountTCEvaluator(source, countOut, &TransformationColumnSpec{
		Name: "neg_qty",
		Type: "count",
		Expr: &star,
		Where: &ExpressionNode{
			Lhs: &ExpressionNode{Type: "select", Expr: "qty"},
			Op:  "<",
			Rhs: &ExpressionNode{Type: "value", Expr: "-0.01"},
		},
	})
	if err != nil {
		t.Fatalf("BuildCountTCEvaluator: %v", err)
	}
	aggValue := []any{nil}
	for _, raw := range []any{"-3", "header_like_value", "5", nil} {
		record := []any{raw, "x"}
		if err := ev.Update(&record, &record); err != nil {
			t.Fatalf("map Update(%v): %v", raw, err)
		}
		if err := count.Update(&aggValue, &record); err != nil {
			t.Fatalf("count Update after mapping %v: %v", raw, err)
		}
	}
	if err := count.Done(&aggValue); err != nil {
		t.Fatalf("count Done: %v", err)
	}
	var n int64
	switch v := aggValue[0].(type) {
	case int:
		n = int64(v)
	case int64:
		n = v
	default:
		t.Fatalf("unexpected count type %T (%v)", aggValue[0], aggValue[0])
	}
	if n != 1 {
		t.Errorf("expecting 1 negative quantity, got %d", n)
	}
}

// A cleansing function that empties the value without an error (find_and_replace
// of a placeholder with "") must not leave the raw value in place: without a
// default the output is null, with one it is the default.
func TestMapCleansedToEmptyWithoutDefaultIsNull(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{
		CleansingFunction: "find_and_replace",
		Argument:          `"Bk",""`,
		RdfType:           "text",
	})
	record := []any{"Bk", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error, got %v", err)
	}
	if record[0] != nil {
		t.Errorf("expecting null in place of the emptied value, got %#v", record[0])
	}
	// a value the function does not empty is kept, cleansed
	record = []any{"12Bk3", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error, got %v", err)
	}
	if record[0] != "123" {
		t.Errorf("expecting the cleansed value 123, got %#v", record[0])
	}
}

func TestMapCleansedToEmptyUsesDefault(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{
		CleansingFunction: "find_and_replace",
		Argument:          `"Bk",""`,
		Default:           "NONE",
		RdfType:           "text",
	})
	record := []any{"Bk", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error, got %v", err)
	}
	if record[0] != "NONE" {
		t.Errorf("expecting the default NONE, got %#v", record[0])
	}
}

// A present input that is blank, empty or only white space, is an empty value:
// it is replaced, never left in place and never written as a blank string.
func TestMapBlankInput(t *testing.T) {
	for _, raw := range []any{"", "   "} {
		ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "double"})
		record := []any{raw, "x"}
		if err := ev.Update(&record, &record); err != nil {
			t.Errorf("%q: expecting no error, got %v", raw, err)
		}
		if record[0] != nil {
			t.Errorf("%q: expecting null, got %#v", raw, record[0])
		}
		ev, _ = buildInPlaceMapEval(t, "qty", "qty", &MapExpression{Default: "0", RdfType: "double"})
		record = []any{raw, "x"}
		if err := ev.Update(&record, &record); err != nil {
			t.Errorf("%q: expecting no error, got %v", raw, err)
		}
		if v, ok := record[0].(float64); !ok || v != 0 {
			t.Errorf("%q: expecting the default 0.0, got %#v", raw, record[0])
		}
		ev, _ = buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "text"})
		record = []any{raw, "x"}
		_ = ev.Update(&record, &record)
		if record[0] != nil {
			t.Errorf("%q: text: expecting null, got %#v", raw, record[0])
		}
	}
}

// An empty value with err_msg and no default: null is written and err_msg is
// reported.
func TestMapBlankInputWithErrMsg(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "text", ErrMsg: "qty is required"})
	record := []any{"  ", "x"}
	err := ev.Update(&record, &record)
	if err == nil || !strings.Contains(err.Error(), "qty is required") {
		t.Errorf("expecting the err_msg error, got %v", err)
	}
	if record[0] != nil {
		t.Errorf("expecting null, got %#v", record[0])
	}
}

// A code value mapping to "" yields an empty value: the default, else null.
func TestMapCodeValueMappingToEmpty(t *testing.T) {
	cvm := map[string]string{"N/A": "", "A": "Active"}
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "text", CodeValueMapping: cvm})
	record := []any{"N/A", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error, got %v", err)
	}
	if record[0] != nil {
		t.Errorf("expecting null for a code mapped to empty, got %#v", record[0])
	}
	record = []any{"A", "x"}
	_ = ev.Update(&record, &record)
	if record[0] != "Active" {
		t.Errorf("expecting Active, got %#v", record[0])
	}
	ev, _ = buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "text", CodeValueMapping: cvm, Default: "Unknown"})
	record = []any{"N/A", "x"}
	if err := ev.Update(&record, &record); err != nil {
		t.Errorf("expecting no error, got %v", err)
	}
	if record[0] != "Unknown" {
		t.Errorf("expecting the default Unknown, got %#v", record[0])
	}
}

// Fallback chain: two map columns onto one output, the first one with an input
// value wins and a later column with an absent input does not erase it. This is
// the absent-input rule, and it must not change.
func TestMapFallbackChainAbsentInputKeepsOutput(t *testing.T) {
	columns := &map[string]int{"primary": 0, "secondary": 1, "out": 2}
	source := &InputChannel{Name: "in", Columns: columns, Config: &ChannelSpec{Name: "in"}}
	outCh := &OutputChannel{Name: "out", Columns: columns, Config: &ChannelSpec{Name: "out"}}
	ctx := &BuilderContext{}
	build := func(expr string) TransformationColumnEvaluator {
		ev, err := ctx.BuildMapTCEvaluator(source, outCh, &TransformationColumnSpec{
			Name: "out", Type: "map", Expr: &expr, MapExpr: &MapExpression{RdfType: "text"},
		})
		if err != nil {
			t.Fatalf("BuildMapTCEvaluator: %v", err)
		}
		return ev
	}
	chain := []TransformationColumnEvaluator{build("primary"), build("secondary")}
	cases := []struct {
		primary, secondary, want any
	}{
		{"p", nil, "p"},
		{nil, "s", "s"},
		{"p", "s", "s"},
		{nil, nil, "orig"},
	}
	for _, c := range cases {
		record := []any{c.primary, c.secondary, "orig"}
		for _, ev := range chain {
			if err := ev.Update(&record, &record); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		if record[2] != c.want {
			t.Errorf("primary=%v secondary=%v: expecting %v, got %#v", c.primary, c.secondary, c.want, record[2])
		}
	}
}

// In an aggregate or map_reduce the map column's output is kept across records.
// A record whose value is rejected or empties must be skipped: neither null nor
// the default may replace the value accumulated so far, and no error is returned
// (the aggregate pipe has no on_error policy, so an error would stop the step).
func TestMapAccumulatingSkipsRejectedAndEmptyValues(t *testing.T) {
	for _, withDefault := range []bool{false, true} {
		mapExpr := &MapExpression{RdfType: "double", CleansingFunction: "find_and_replace", Argument: `"Bk",""`}
		if withDefault {
			mapExpr.Default = "0"
		}
		ev := buildAccumulatingMapEval(t, mapExpr)
		agg := []any{nil}
		for _, raw := range []any{"5", "header_like_value", "Bk", "", "  ", nil} {
			record := []any{raw}
			if err := ev.Update(&agg, &record); err != nil {
				t.Fatalf("default %v: Update(%q) returned %v, want no error", withDefault, raw, err)
			}
			want := any(5.0)
			if withDefault && raw == nil {
				// An absent input takes the default, as it always has
				want = 0.0
			}
			if agg[0] != want {
				t.Fatalf("default %v: after %q the output is %v, want %v", withDefault, raw, agg[0], want)
			}
			agg[0] = 5.0
		}
	}
}

// The same column in map_record (marked per record) writes null for those values.
func TestMapPerRecordWritesNullWhereAccumulatingSkips(t *testing.T) {
	ev := buildAccumulatingMapEval(t, &MapExpression{RdfType: "double"})
	markPerRecord(ev)
	record := []any{"header_like_value"}
	if err := ev.Update(&record, &record); err == nil {
		t.Error("want the cast error in map_record")
	}
	if record[0] != nil {
		t.Errorf("map_record output is %v, want null", record[0])
	}
}
