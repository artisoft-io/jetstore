package compute_pipes

import (
	"testing"
)

type fakeLookupTable struct{}

func (fakeLookupTable) Lookup(key *string) (*[]any, error)                     { return nil, nil }
func (fakeLookupTable) LookupValue(row *[]any, columnName string) (any, error) { return nil, nil }
func (fakeLookupTable) ColumnMap() map[string]int                              { return map[string]int{"lk_col": 0} }
func (fakeLookupTable) IsEmptyTable() bool                                     { return false }
func (fakeLookupTable) Size() int64                                            { return 1 }

func doLookupBuildTest(key, value LookupColumnSpec) error {
	name := "tbl"
	spec := &TransformationColumnSpec{
		Type:         "lookup",
		LookupName:   &name,
		LookupKey:    []LookupColumnSpec{key},
		LookupValues: []LookupColumnSpec{value},
	}
	source := &InputChannel{
		Name:    "in",
		Columns: &map[string]int{"key1": 0},
	}
	outputCh := &OutputChannel{
		Channel: make(chan []any),
		Columns: &map[string]int{"out_col": 0},
	}
	ctx := &BuilderContext{
		lookupTableManager: &LookupTableManager{
			LookupTableMap: map[string]LookupTable{"tbl": fakeLookupTable{}},
		},
	}
	_, err := ctx.BuildLookupTCEvaluator(source, outputCh, spec)
	return err
}

func lkExpr(s string) *string { return &s }

func TestLookupBuildAcceptsValidSpec(t *testing.T) {
	key := LookupColumnSpec{Type: "select", Expr: lkExpr("key1")}
	value := LookupColumnSpec{Type: "select", Expr: lkExpr("lk_col"), Name: "out_col"}
	if err := doLookupBuildTest(key, value); err != nil {
		t.Fatalf("valid lookup spec rejected: %v", err)
	}
}

func TestLookupRejectsUnknownKeyType(t *testing.T) {
	// The key and value type switches had no default case: an unknown type left
	// a nil evaluator that panicked on the first row; the builder must reject it.
	key := LookupColumnSpec{Type: "bogus", Expr: lkExpr("key1")}
	value := LookupColumnSpec{Type: "select", Expr: lkExpr("lk_col"), Name: "out_col"}
	if err := doLookupBuildTest(key, value); err == nil {
		t.Error("expecting an unknown lookup key type to be rejected")
	}
}

func TestLookupRejectsUnknownValueType(t *testing.T) {
	key := LookupColumnSpec{Type: "select", Expr: lkExpr("key1")}
	value := LookupColumnSpec{Type: "bogus", Expr: lkExpr("lk_col"), Name: "out_col"}
	if err := doLookupBuildTest(key, value); err == nil {
		t.Error("expecting an unknown lookup value type to be rejected")
	}
}

func TestLookupRejectsKeyWithoutExpr(t *testing.T) {
	// A select key without expr used to be a nil-pointer panic at DAG build.
	key := LookupColumnSpec{Type: "select"}
	value := LookupColumnSpec{Type: "select", Expr: lkExpr("lk_col"), Name: "out_col"}
	if err := doLookupBuildTest(key, value); err == nil {
		t.Error("expecting a lookup key without expr to be rejected")
	}
}

func TestLookupRejectsUnknownKeyColumn(t *testing.T) {
	// An unchecked lookup used to map an unknown key column to position 0,
	// silently keying the lookup on the wrong column; the builder must reject it.
	key := LookupColumnSpec{Type: "select", Expr: lkExpr("not_a_column")}
	value := LookupColumnSpec{Type: "select", Expr: lkExpr("lk_col"), Name: "out_col"}
	if err := doLookupBuildTest(key, value); err == nil {
		t.Error("expecting a lookup key that is not an input column to be rejected")
	}
}
