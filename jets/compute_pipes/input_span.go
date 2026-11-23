package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/jackc/pgx/v5/pgconn"
)

// The input span: the days from the earliest to the latest value of one
// column, over the records a map_record step maps. A map_record step names the
// column, by its output (canonical) name, in map_record_config.calculate_span_of.
// Each node keeps the earliest and latest date it mapped into that column and
// records them on its pipeline_execution_details row; when the run ends the
// status update reads them across the nodes into
// pipeline_execution_status.input_span_days, which Pipeline Status shows.

// spanTracker is one map_record step's earliest and latest date in its span
// column. It is written by the one goroutine that applies the step.
type spanTracker struct {
	column   string
	pos      int
	min, max time.Time
	seen     bool
}

// observe reads the span column of one mapped record. A value that is not a
// date, after the mapping cast it to the column's type, is read as text and
// parsed; one that does not parse is skipped, as is a null.
func (t *spanTracker) observe(record []any) {
	if t.pos >= len(record) {
		return
	}
	switch v := record[t.pos].(type) {
	case time.Time:
		t.add(v)
	case *time.Time:
		if v != nil {
			t.add(*v)
		}
	case string:
		if d, err := rdf.ParseDate(v); err == nil && d != nil {
			t.add(*d)
		}
	case []any:
		for _, e := range v {
			if d, ok := e.(time.Time); ok {
				t.add(d)
			}
		}
	}
}

func (t *spanTracker) add(d time.Time) {
	d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	if !t.seen || d.Before(t.min) {
		t.min = d
	}
	if !t.seen || d.After(t.max) {
		t.max = d
	}
	t.seen = true
}

// InputSpan is a node's input span, merged from every map_record step of the
// node that names a span column. It is safe for concurrent use.
type InputSpan struct {
	mu       sync.Mutex
	column   string
	min, max time.Time
	seen     bool
}

// merge adds one step's span. A step naming another column than the one
// already recorded is left out, with a log line: one span is recorded per run.
func (s *InputSpan) merge(t *spanTracker) {
	if s == nil || t == nil || !t.seen {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.column == "":
		s.column = t.column
	case s.column != t.column:
		log.Printf("input span: map_record steps name two span columns, %q and %q; the span of %q is not recorded",
			s.column, t.column, t.column)
		return
	}
	if !s.seen || t.min.Before(s.min) {
		s.min = t.min
	}
	if !s.seen || t.max.After(s.max) {
		s.max = t.max
	}
	s.seen = true
}

// Values returns the span column and its earliest and latest date; ok is false
// when no step recorded a date.
func (s *InputSpan) Values() (column string, min, max time.Time, ok bool) {
	if s == nil {
		return "", time.Time{}, time.Time{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.column, s.min, s.max, s.seen
}

type spanExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// RecordInputSpan writes the node's input span to its pipeline_execution_details
// row, key. It writes nothing when the node recorded no span.
func RecordInputSpan(ctx context.Context, db spanExecer, key int, span *InputSpan) error {
	column, lo, hi, ok := span.Values()
	if !ok {
		return nil
	}
	_, err := db.Exec(ctx, `UPDATE jetsapi.pipeline_execution_details
		SET (input_span_column, input_span_min, input_span_max) = ($1, $2::date, $3::date) WHERE key = $4`,
		column, lo.Format(time.DateOnly), hi.Format(time.DateOnly), key)
	if err != nil {
		return fmt.Errorf("while recording the input span on pipeline_execution_details %d: %v", key, err)
	}
	return nil
}
