package compute_pipes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func spanPipe(t *testing.T, column string, results *ChannelResults) *MapRecordTransformationPipe {
	t.Helper()
	spec := &TransformationSpec{
		Type:            "map_record",
		MapRecordConfig: &MapRecordSpec{CalculateSpanOf: column},
	}
	source := &InputChannel{
		Name:    "in",
		Columns: &map[string]int{"key": 0, "dos": 1},
		Config:  &ChannelSpec{Name: "in"},
	}
	outputCh := &OutputChannel{
		Name:    "out",
		Channel: make(chan []any, 16),
		Columns: &map[string]int{"key": 0, "dos": 1},
		Config:  &ChannelSpec{Name: "out", Columns: []string{"key", "dos"}},
	}
	ctx := &BuilderContext{done: make(chan struct{}), chResults: results}
	pipe, err := ctx.NewMapRecordTransformationPipe(source, outputCh, spec)
	if err != nil {
		t.Fatalf("map_record with calculate_span_of %q rejected: %v", column, err)
	}
	return pipe
}

func spanDay(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}

// The span is the earliest and latest date of the named column over the
// records the step mapped: a date, a date as text, a date in an array; a null
// and a value that is not a date are skipped.
func TestMapRecordCalculatesTheSpanOfItsColumn(t *testing.T) {
	results := &ChannelResults{InputSpan: &InputSpan{}}
	pipe := spanPipe(t, "dos", results)
	for _, rec := range [][]any{
		{"a", spanDay("2026-03-05")},
		{"b", "2026-01-31"},
		{"c", nil},
		{"d", "not a date"},
		{"e", []any{spanDay("2026-04-02"), spanDay("2026-02-10")}},
		{"f", time.Date(2026, 3, 1, 23, 30, 0, 0, time.UTC)},
	} {
		if err := pipe.Apply(&rec); err != nil {
			t.Fatalf("Apply(%v): %v", rec, err)
		}
	}
	if column, _, _, ok := results.InputSpan.Values(); ok {
		t.Fatalf("the span was recorded before the step was done, column %q", column)
	}
	if err := pipe.Done(); err != nil {
		t.Fatal(err)
	}
	column, lo, hi, ok := results.InputSpan.Values()
	if !ok || column != "dos" || !lo.Equal(spanDay("2026-01-31")) || !hi.Equal(spanDay("2026-04-02")) {
		t.Errorf("span = %q %v %v %v, want dos 2026-01-31 2026-04-02 true", column, lo, hi, ok)
	}
}

// A step that names a column its output channel does not have is refused when
// the pipe is built, not after a run that recorded nothing.
func TestMapRecordRefusesASpanColumnItDoesNotOutput(t *testing.T) {
	_, err := doMapRecordBuildTest(&MapRecordSpec{CalculateSpanOf: "no_such_column"})
	if err == nil || !strings.Contains(err.Error(), "calculate_span_of") {
		t.Errorf("expecting the unknown span column to be refused, got %v", err)
	}
}

// Two steps merge into one span when they name the same column; a step naming
// another column is left out of it.
func TestInputSpanMergesOneColumn(t *testing.T) {
	s := &InputSpan{}
	s.merge(&spanTracker{column: "dos", min: spanDay("2026-02-01"), max: spanDay("2026-02-20"), seen: true})
	s.merge(&spanTracker{column: "dos", min: spanDay("2026-01-15"), max: spanDay("2026-02-10"), seen: true})
	s.merge(&spanTracker{column: "other", min: spanDay("2020-01-01"), max: spanDay("2030-01-01"), seen: true})
	s.merge(&spanTracker{column: "dos"}) // a step that saw no date
	column, lo, hi, ok := s.Values()
	if !ok || column != "dos" || !lo.Equal(spanDay("2026-01-15")) || !hi.Equal(spanDay("2026-02-20")) {
		t.Errorf("span = %q %v %v %v, want dos 2026-01-15 2026-02-20 true", column, lo, hi, ok)
	}
}

type recordingExecer struct {
	sql  string
	args []any
}

func (r *recordingExecer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	r.sql, r.args = sql, args
	return pgconn.CommandTag{}, nil
}

type failingExecer struct{}

func (failingExecer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("must not be called")
}

// A node with no span writes nothing; one with a span writes its column and
// its two dates to its own detail row.
func TestRecordInputSpan(t *testing.T) {
	if err := RecordInputSpan(context.Background(), failingExecer{}, 7, &InputSpan{}); err != nil {
		t.Errorf("a node with no span wrote: %v", err)
	}
	if err := RecordInputSpan(context.Background(), failingExecer{}, 7, nil); err != nil {
		t.Errorf("a node with no span collector wrote: %v", err)
	}
	s := &InputSpan{}
	s.merge(&spanTracker{column: "dos", min: spanDay("2026-01-15"), max: spanDay("2026-02-20"), seen: true})
	r := &recordingExecer{}
	if err := RecordInputSpan(context.Background(), r, 7, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.sql, "pipeline_execution_details") || len(r.args) != 4 ||
		r.args[0] != "dos" || r.args[1] != "2026-01-15" || r.args[2] != "2026-02-20" || r.args[3] != 7 {
		t.Errorf("RecordInputSpan wrote %q with %v", r.sql, r.args)
	}
}
