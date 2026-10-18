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
	return ev, source
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
