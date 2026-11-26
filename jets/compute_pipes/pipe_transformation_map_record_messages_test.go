package compute_pipes

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Tests for the messages map_record stores on its error channel: every one
// starts with "map_record: ", and a map column's rejection names the output
// column as well as its position.

// newMapRecordWithErrorChannel builds a map_record over the input columns
// key1, key2 and the output columns of spec, with an error channel the test
// reads the stored rows from.
func newMapRecordWithErrorChannel(t *testing.T, spec *TransformationSpec, outColumns []string) (*MapRecordTransformationPipe, errorRows) {
	t.Helper()
	source := &InputChannel{
		Name:    "in",
		Columns: &map[string]int{"key1": 0, "key2": 1},
		Config:  &ChannelSpec{Name: "in", Columns: []string{"key1", "key2"}},
	}
	outMap := make(map[string]int, len(outColumns))
	for i, c := range outColumns {
		outMap[c] = i
	}
	outputCh := &OutputChannel{
		Name:    "out",
		Channel: make(chan []any, 10),
		Columns: &outMap,
		Config:  &ChannelSpec{Name: "out", Columns: outColumns},
	}
	ctx := &BuilderContext{done: make(chan struct{})}
	pipe, err := ctx.NewMapRecordTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatal(err)
	}
	columns := append(append([]string{}, legacyProcessErrorColumns...), ProcessErrorDiscriminatorColumns...)
	errCh, _ := newTestErrorChannel("process_errors.out", columns)
	rows := make(chan []any, 10)
	errCh.Channel = rows
	pipe.errorOutputCh = errCh
	return pipe, errorRows{rows: rows, columns: *errCh.Columns}
}

// errorRows is the receiving end of the test's error channel.
type errorRows struct {
	rows    chan []any
	columns map[string]int
}

// storedMessages applies the records and returns the error_message of every
// row map_record stored, in order.
func storedMessages(t *testing.T, pipe *MapRecordTransformationPipe, errCh errorRows, records ...[]any) []string {
	t.Helper()
	for _, r := range records {
		record := append([]any{}, r...)
		if err := pipe.Apply(&record); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}
	var msgs []string
	for {
		select {
		case row := <-errCh.rows:
			if op := row[errCh.columns["operator_type"]]; op != "map_record" {
				t.Errorf("operator_type: got %v, want map_record", op)
			}
			msgs = append(msgs, row[errCh.columns["error_message"]].(string))
		default:
			return msgs
		}
	}
}

func mapColumnSpec(name, expr string, mapExpr *MapExpression) TransformationColumnSpec {
	return TransformationColumnSpec{Name: name, Type: "map", Expr: &expr, MapExpr: mapExpr}
}

func checkMessages(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("stored %d rows, want %d: %q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d:\n got  %q\n want %q", i, got[i], want[i])
		}
	}
}

// err_msg on an absent input and on a blank one: the stored message names the
// output column, then its position, then err_msg.
func TestMapRecordStoresErrMsgWithTheColumnName(t *testing.T) {
	spec := &TransformationSpec{
		Type:            "map_record",
		NewRecord:       true,
		MapRecordConfig: &MapRecordSpec{MaxErrorCount: 10},
		Columns: []TransformationColumnSpec{
			mapColumnSpec("id", "key1", &MapExpression{RdfType: "text"}),
			mapColumnSpec("hc:amount", "key2", &MapExpression{RdfType: "text", ErrMsg: "amount is required"}),
		},
	}
	pipe, errCh := newMapRecordWithErrorChannel(t, spec, []string{"id", "hc:amount"})
	got := storedMessages(t, pipe, errCh, []any{"a", nil}, []any{"b", "  "})
	want := "map_record: error mapping column hc:amount at output position 1: amount is required"
	checkMessages(t, got, want, want)
}

// A value the cleansing function rejects: the stored message names the output
// column, then its position, then the cleansing function's message.
func TestMapRecordStoresCleansingErrorWithTheColumnName(t *testing.T) {
	spec := &TransformationSpec{
		Type:            "map_record",
		NewRecord:       true,
		MapRecordConfig: &MapRecordSpec{MaxErrorCount: 10},
		Columns: []TransformationColumnSpec{
			mapColumnSpec("service_date", "key1", &MapExpression{CleansingFunction: "validate_date", RdfType: "text"}),
		},
	}
	pipe, errCh := newMapRecordWithErrorChannel(t, spec, []string{"service_date"})
	got := storedMessages(t, pipe, errCh, []any{"not a date", nil})
	if len(got) != 1 {
		t.Fatalf("stored %d rows, want 1: %q", len(got), got)
	}
	head := "map_record: error mapping column service_date at output position 0: "
	if !strings.HasPrefix(got[0], head) || len(got[0]) == len(head) {
		t.Errorf("got %q, want %q followed by the cleansing function's message", got[0], head)
	}
}

// A value the cast to rdf type rejects keeps the cast's message, after the
// prefix.
func TestMapRecordStoresCastErrorWithThePrefix(t *testing.T) {
	spec := &TransformationSpec{
		Type:            "map_record",
		NewRecord:       true,
		MapRecordConfig: &MapRecordSpec{MaxErrorCount: 10},
		Columns: []TransformationColumnSpec{
			mapColumnSpec("qty", "key1", &MapExpression{RdfType: "double"}),
		},
	}
	pipe, errCh := newMapRecordWithErrorChannel(t, spec, []string{"qty"})
	got := storedMessages(t, pipe, errCh, []any{"abc", nil})
	if len(got) != 1 {
		t.Fatalf("stored %d rows, want 1: %q", len(got), got)
	}
	if head := "map_record: while casting value to rdf type 'double': "; !strings.HasPrefix(got[0], head) {
		t.Errorf("got %q, want it to start %q", got[0], head)
	}
}

// A case expression's error keeps its own form after the prefix, the map
// column's message inside it included.
func TestMapRecordStoresCaseExprErrorWithThePrefix(t *testing.T) {
	expr := "key2"
	spec := &TransformationSpec{
		Type:            "map_record",
		NewRecord:       true,
		MapRecordConfig: &MapRecordSpec{MaxErrorCount: 10},
		Columns: []TransformationColumnSpec{{
			Name:     "status",
			Type:     "case",
			CaseExpr: []CaseExpression{},
			ElseExpr: []*TransformationColumnSpec{{
				Name: "status", Type: "map", Expr: &expr,
				MapExpr: &MapExpression{RdfType: "text", ErrMsg: "status is required"},
			}},
		}},
	}
	pipe, errCh := newMapRecordWithErrorChannel(t, spec, []string{"status"})
	got := storedMessages(t, pipe, errCh, []any{"a", nil})
	checkMessages(t, got, "map_record: while evaluating case_expr else clause: "+
		"error mapping column at output position 0: status is required")
}

// stubColumnEval returns err from every Update.
type stubColumnEval struct{ err error }

func (s stubColumnEval) Update(currentValue *[]any, input *[]any) error { return s.err }
func (s stubColumnEval) Done(currentValue *[]any) error                 { return nil }

// Any column evaluator's error gains the prefix once: a lookup's, a wrapped
// map column error (named only when it is the evaluator's own error), and a
// message that already carries the prefix.
func TestMapRecordPrefixesEveryEvaluatorErrorOnce(t *testing.T) {
	spec := &TransformationSpec{
		Type:            "map_record",
		NewRecord:       true,
		MapRecordConfig: &MapRecordSpec{MaxErrorCount: 10},
	}
	pipe, errCh := newMapRecordWithErrorChannel(t, spec, []string{"out_col"})
	mce := &mappingColumnError{outputName: "out_col", outputPos: 0, detail: "bad"}
	pipe.columnEvaluators = []TransformationColumnEvaluator{
		stubColumnEval{errors.New("while making the lookup key: no value")},
		stubColumnEval{errors.New("while fetching the lookup row: not found")},
		stubColumnEval{fmt.Errorf("while evaluating case_expr then clause #0: %w", mce)},
		stubColumnEval{errors.New("map_record: already prefixed")},
	}
	got := storedMessages(t, pipe, errCh, []any{"a", "b"})
	checkMessages(t, got,
		"map_record: while making the lookup key: no value",
		"map_record: while fetching the lookup row: not found",
		"map_record: while evaluating case_expr then clause #0: error mapping column at output position 0: bad",
		"map_record: already prefixed",
	)
}

// The map column's own error is unchanged, so an operator that stores it
// inside its own message (the inference operators do) stores what it always
// has: no prefix and no column name.
func TestMapColumnErrorKeepsItsForm(t *testing.T) {
	ev, _ := buildInPlaceMapEval(t, "qty", "qty", &MapExpression{RdfType: "text", ErrMsg: "qty is required"})
	record := []any{nil, "x"}
	err := ev.Update(&record, &record)
	if want := "error mapping column at output position 0: qty is required"; err == nil || err.Error() != want {
		t.Errorf("got %v, want %q", err, want)
	}
}
