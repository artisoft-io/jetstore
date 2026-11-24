package compute_pipes

import (
	"context"
	"encoding/json"
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
//
// The core span is the same column read more narrowly: the days from the
// nearest-rank 1st percentile date to the 99th, over the records with a date.
// Each node also keeps how many records it mapped on each day and records the
// counts with its span (input_span_day_counts); the status update merges the
// nodes' counts into pipeline_execution_status.input_core_span_days. A feed
// that sends a few claims years older than the rest has a long input span and
// a short core span, and the two together say how much history it sent.
//
// Both are measured on the records as delivered: a step observes a record
// before its retain window (retain_window.go) decides whether to keep it.

// spanTracker is one map_record step's earliest and latest date in its span
// column. It is written by the one goroutine that applies the step.
type spanTracker struct {
	column   string
	pos      int
	min, max time.Time
	seen     bool
	// days counts the dates seen on each day, keyed by the day's number since
	// the Unix epoch.
	days map[int64]int64
}

// eachDate calls f with every date a mapped value carries, at midnight UTC. A
// value that is not a date, after the mapping cast it to the column's type, is
// read as text and parsed; one that does not parse carries no date, nor does a
// null. An array carries the dates among its elements.
func eachDate(value any, f func(time.Time)) {
	day := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC) }
	switch v := value.(type) {
	case time.Time:
		f(day(v))
	case *time.Time:
		if v != nil {
			f(day(*v))
		}
	case string:
		if d, err := rdf.ParseDate(v); err == nil && d != nil {
			f(day(*d))
		}
	case []any:
		for _, e := range v {
			if d, ok := e.(time.Time); ok {
				f(day(d))
			}
		}
	}
}

// observe reads the span column of one mapped record.
func (t *spanTracker) observe(record []any) {
	if t.pos >= len(record) {
		return
	}
	eachDate(record[t.pos], t.add)
}

func (t *spanTracker) add(d time.Time) {
	if !t.seen || d.Before(t.min) {
		t.min = d
	}
	if !t.seen || d.After(t.max) {
		t.max = d
	}
	t.seen = true
	if t.days == nil {
		t.days = map[int64]int64{}
	}
	t.days[d.Unix()/secondsPerDay]++
}

const secondsPerDay = 24 * 60 * 60

// InputSpan is a node's input span, merged from every map_record step of the
// node that names a span column. It is safe for concurrent use.
type InputSpan struct {
	mu       sync.Mutex
	column   string
	min, max time.Time
	seen     bool
	days     map[int64]int64
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
	if s.days == nil {
		s.days = make(map[int64]int64, len(t.days))
	}
	for d, n := range t.days {
		s.days[d] += n
	}
}

// DayCounts returns how many dates the node saw on each day, keyed by the day
// as YYYY-MM-DD; nil when no step recorded a date.
func (s *InputSpan) DayCounts() map[string]int64 {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.seen {
		return nil
	}
	out := make(map[string]int64, len(s.days))
	for d, n := range s.days {
		out[time.Unix(d*secondsPerDay, 0).UTC().Format(time.DateOnly)] = n
	}
	return out
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

// RecordInputSpan writes the node's input span, and its count of dates per
// day, to its pipeline_execution_details row, key. It writes nothing when the
// node recorded no span.
func RecordInputSpan(ctx context.Context, db spanExecer, key int, span *InputSpan) error {
	column, lo, hi, ok := span.Values()
	if !ok {
		return nil
	}
	days, err := json.Marshal(span.DayCounts())
	if err != nil {
		return fmt.Errorf("while encoding the input span's day counts: %v", err)
	}
	_, err = db.Exec(ctx, `UPDATE jetsapi.pipeline_execution_details
		SET (input_span_column, input_span_min, input_span_max, input_span_day_counts) = ($1, $2::date, $3::date, $4::jsonb)
		WHERE key = $5`,
		column, lo.Format(time.DateOnly), hi.Format(time.DateOnly), string(days), key)
	if err != nil {
		return fmt.Errorf("while recording the input span on pipeline_execution_details %d: %v", key, err)
	}
	return nil
}
