package compute_pipes

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
	"github.com/artisoft-io/jetstore/jets/utils"
)

// The retain window: a map_record step that names one in
// map_record_config.retain_window keeps only the records whose date column
// falls within a window around the file's date, and drops the rest. The
// window is anchored on $DATE_FILE_KEY, the date the file key carries, so a
// rerun of the same file keeps the same records. A record is dropped when its
// column carries no date (null, or a value that does not read as a date), when
// it is dated before the file date less days_before_file_date, or after the
// file date plus days_after_file_date. The bounds are inclusive.
//
// A feed that delivers far more history than a run needs is the case it is
// for. The input span and the core span (input_span.go) are measured before
// the window decides, so Pipeline Status still shows what the feed sent, and
// the dropped records are counted by reason on each node's
// pipeline_execution_details row and summed into the run's
// dropped_records_count.

// retainWindow is one map_record step's window and its drop counts. It is
// written by the one goroutine that applies the step.
type retainWindow struct {
	column                string
	pos                   int
	daysBefore, daysAfter int
	start, end            time.Time // inclusive, at midnight UTC
	noDate, before, after int64
}

// Why a retain window drops a record.
const (
	retainKeep = iota
	retainNoDate
	retainBefore
	retainAfter
)

// newRetainWindow builds a step's window, refusing a column the output channel
// does not have, a day count that is not a non-negative integer once its env
// vars are substituted, and a run whose $DATE_FILE_KEY is missing or is not a
// date.
func newRetainWindow(spec *RetainWindowSpec, outputCh *OutputChannel, env map[string]any) (*retainWindow, error) {
	if spec.Column == "" {
		return nil, fmt.Errorf("error: map_record_config retain_window names no column")
	}
	pos, ok := (*outputCh.Columns)[spec.Column]
	if !ok {
		return nil, fmt.Errorf("error: map_record_config retain_window column '%s' is not a column of output channel '%s'",
			spec.Column, outputCh.Name)
	}
	if spec.DaysBeforeFileDate == nil {
		return nil, fmt.Errorf("error: map_record_config retain_window has no days_before_file_date")
	}
	before, err := retainWindowDays("days_before_file_date", spec.DaysBeforeFileDate, env)
	if err != nil {
		return nil, err
	}
	after := 0
	if spec.DaysAfterFileDate != nil {
		if after, err = retainWindowDays("days_after_file_date", spec.DaysAfterFileDate, env); err != nil {
			return nil, err
		}
	}
	anchor, err := retainWindowAnchor(env)
	if err != nil {
		return nil, err
	}
	return &retainWindow{
		column:     spec.Column,
		pos:        pos,
		daysBefore: before,
		daysAfter:  after,
		start:      anchor.AddDate(0, 0, -before),
		end:        anchor.AddDate(0, 0, after),
	}, nil
}

// retainWindowDays reads a day count: a JSON number, or a string whose env
// vars are substituted, e.g. "${RETAIN_WINDOW_DAYS}".
func retainWindowDays(name string, value any, env map[string]any) (int, error) {
	var n int
	switch v := value.(type) {
	case float64:
		if v != math.Trunc(v) || v > math.MaxInt32 || v < math.MinInt32 {
			return 0, fmt.Errorf("error: map_record_config retain_window %s %v is not an integer", name, v)
		}
		n = int(v)
	case int:
		n = v
	case int64:
		n = int(v)
	case string:
		s := strings.TrimSpace(utils.ReplaceEnvVars(v, env))
		var err error
		if n, err = strconv.Atoi(s); err != nil {
			return 0, fmt.Errorf("error: map_record_config retain_window %s '%s' is '%s' once its env vars are substituted, which is not an integer",
				name, v, s)
		}
	default:
		return 0, fmt.Errorf("error: map_record_config retain_window %s %v is neither an integer nor a string", name, value)
	}
	if n < 0 {
		return 0, fmt.Errorf("error: map_record_config retain_window %s is %d, which is negative", name, n)
	}
	return n, nil
}

// retainWindowAnchor is the file's date, $DATE_FILE_KEY: a YYYY-MM-DD string
// as the run's start sets it, or a date when a caller leaves the file key's
// date in place.
func retainWindowAnchor(env map[string]any) (time.Time, error) {
	day := func(d time.Time) time.Time { return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC) }
	raw := env["$DATE_FILE_KEY"]
	var s string
	switch v := raw.(type) {
	case time.Time:
		if !v.IsZero() {
			return day(v), nil
		}
	case string:
		s = strings.TrimSpace(v)
	}
	if s == "" {
		return time.Time{}, fmt.Errorf("error: map_record_config retain_window is anchored on $DATE_FILE_KEY, which the run does not have (%v)", raw)
	}
	if d, err := time.Parse(time.DateOnly, s); err == nil {
		return d, nil
	}
	d, err := rdf.ParseDate(s)
	if err != nil || d == nil {
		return time.Time{}, fmt.Errorf("error: map_record_config retain_window is anchored on $DATE_FILE_KEY, and '%s' is not a date", s)
	}
	return day(*d), nil
}

// decide reads the window column of one mapped record and says whether to keep
// it, counting a drop by its reason. A value with several dates, an array, is
// kept when its dates reach into the window: before when its latest is before
// the window, after when its earliest is after it.
func (w *retainWindow) decide(record []any) bool {
	var lo, hi time.Time
	seen := false
	if w.pos < len(record) {
		eachDate(record[w.pos], func(d time.Time) {
			if !seen || d.Before(lo) {
				lo = d
			}
			if !seen || d.After(hi) {
				hi = d
			}
			seen = true
		})
	}
	switch {
	case !seen:
		w.noDate++
		return false
	case hi.Before(w.start):
		w.before++
		return false
	case lo.After(w.end):
		w.after++
		return false
	}
	return true
}

// RetainWindowDrops are a node's retain window and the records it dropped,
// merged from every map_record step of the node that names one. It is safe
// for concurrent use.
type RetainWindowDrops struct {
	mu                    sync.Mutex
	column                string
	daysBefore, daysAfter int
	noDate, before, after int64
	set                   bool
}

// merge adds one step's drops. A step whose window is on another column than
// the one already recorded is counted all the same, with a log line: its
// records were dropped whatever column it read.
func (r *RetainWindowDrops) merge(w *retainWindow) {
	if r == nil || w == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case !r.set:
		r.column, r.daysBefore, r.daysAfter, r.set = w.column, w.daysBefore, w.daysAfter, true
	case r.column != w.column || r.daysBefore != w.daysBefore || r.daysAfter != w.daysAfter:
		log.Printf("retain window: map_record steps name two windows, %q -%d/+%d days and %q -%d/+%d days; the first is recorded and the drops of both are counted",
			r.column, r.daysBefore, r.daysAfter, w.column, w.daysBefore, w.daysAfter)
	}
	r.noDate += w.noDate
	r.before += w.before
	r.after += w.after
}

// RetainWindowCounts are what a node records about its retain window.
type RetainWindowCounts struct {
	Column                string
	DaysBefore, DaysAfter int
	NoDate, Before, After int64
}

// Counts returns the node's window and drops; ok is false when no step of the
// node names a retain window.
func (r *RetainWindowDrops) Counts() (c RetainWindowCounts, ok bool) {
	if r == nil {
		return c, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return RetainWindowCounts{Column: r.column, DaysBefore: r.daysBefore, DaysAfter: r.daysAfter,
		NoDate: r.noDate, Before: r.before, After: r.after}, r.set
}

// RecordRetainWindow writes the node's retain window and its drops by reason
// to its pipeline_execution_details row, key: zeros when the window dropped
// nothing, and nothing when the node has no window.
func RecordRetainWindow(ctx context.Context, db spanExecer, key int, drops *RetainWindowDrops) error {
	c, ok := drops.Counts()
	if !ok {
		return nil
	}
	_, err := db.Exec(ctx, `UPDATE jetsapi.pipeline_execution_details
		SET (retain_window_column, retain_window_days_before, retain_window_days_after,
		     dropped_no_date_count, dropped_before_window_count, dropped_after_window_count) = ($1, $2, $3, $4, $5, $6)
		WHERE key = $7`,
		c.Column, c.DaysBefore, c.DaysAfter, c.NoDate, c.Before, c.After, key)
	if err != nil {
		return fmt.Errorf("while recording the retain window's drops on pipeline_execution_details %d: %v", key, err)
	}
	return nil
}
