package datatable

import (
	"context"
	"fmt"
	"testing"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
)

// A run's main input count is the reducing00 nodes' input_records_count, its
// input span the days from the earliest to the latest span date of its nodes;
// a node still in progress counts for neither, and a run whose nodes recorded
// neither keeps both columns NULL. The archive and the history view carry both.
func TestUpdateRunInputSummary(t *testing.T) {
	pool := purgetest.DB(t, "run_input_summary")
	ctx := context.Background()
	keys := purgetest.Seed(t, pool, purgetest.Session{ID: "1700000000001", Client: "c", Runs: []string{"completed"}})
	other := purgetest.Seed(t, pool, purgetest.Session{ID: "1700000000002", Client: "c", Runs: []string{"completed"}})
	key := keys[0]
	purgetest.Exec(t, pool, `DELETE FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key = $1`, key)
	for _, d := range []struct {
		step, status string
		n            int
		lo, hi       any
	}{
		{"reducing00", "completed", 100, "2026-01-10", "2026-03-01"},
		{"reducing00", "completed", 50, "2025-12-20", "2026-02-01"},
		{"reducing00", "in progress", 7, "2020-01-01", "2030-01-01"},
		{"reducing01", "completed", 999, nil, nil},
	} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_details
			(pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
			 session_id, source_period_key, status, user_email, cpipes_step_id, input_records_count,
			 input_span_column, input_span_min, input_span_max)
			VALUES (1, $1, 'c', 'proc', 's', '1700000000001', 1, $2, 'system', $3, $4, 'dos', $5::date, $6::date)`,
			key, d.status, d.step, d.n, d.lo, d.hi)
	}
	if err := UpdateRunInputSummary(ctx, pool, key); err != nil {
		t.Fatal(err)
	}
	if err := UpdateRunInputSummary(ctx, pool, other[0]); err != nil {
		t.Fatal(err)
	}
	read := func(k int) (n *int64, span *int32) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT main_input_records_count, input_span_days
			FROM jetsapi.pipeline_execution_history WHERE key = $1`, k).Scan(&n, &span); err != nil {
			t.Fatal(err)
		}
		return n, span
	}
	// 2025-12-20 to 2026-03-01 is 71 days.
	if n, span := read(key); n == nil || *n != 150 || span == nil || *span != 71 {
		t.Errorf("run summary = %v %v, want 150 and 71", deref(n), deref(span))
	}
	// The seeded run's one detail row has no step and no span.
	if n, span := read(other[0]); n != nil || span != nil {
		t.Errorf("a run with no reducing00 row and no span = %v %v, want NULL and NULL", deref(n), deref(span))
	}
	// Archived, the run keeps both.
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_status_archive
		SELECT *, now(), 'user' FROM jetsapi.pipeline_execution_status WHERE key = $1`, key)
	purgetest.Exec(t, pool, `DELETE FROM jetsapi.pipeline_execution_status WHERE key = $1`, key)
	if n, span := read(key); n == nil || *n != 150 || span == nil || *span != 71 {
		t.Errorf("archived run summary = %v %v, want 150 and 71", deref(n), deref(span))
	}
}

// The core span merges the nodes' day counts and runs from the nearest-rank
// 1st percentile date to the 99th; the dropped count sums the nodes' drops of
// every reason. A node in progress counts for neither; a run whose nodes
// recorded no window has no dropped count, and one whose window dropped
// nothing has 0. Input Count still counts every record read.
func TestUpdateRunCoreSpanAndDroppedCount(t *testing.T) {
	pool := purgetest.DB(t, "run_core_span")
	ctx := context.Background()
	runs := map[string]int{}
	for i, name := range []string{"spread", "one_day", "no_window", "window_no_drops"} {
		id := fmt.Sprintf("170000000010%d", i)
		runs[name] = purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "c", Runs: []string{"completed"}})[0]
		purgetest.Exec(t, pool, `DELETE FROM jetsapi.pipeline_execution_details WHERE pipeline_execution_status_key = $1`,
			runs[name])
	}
	type node struct {
		run, status         string
		n                   int
		days                any // the day counts, as JSON text, or nil
		before              any // retain_window_days_before, nil for no window
		noDate, early, late any
	}
	for _, d := range []node{
		// 200 dates in all: P1 is the 2nd, on 2026-01-01; P99 the 198th, on
		// 2026-01-10. The span runs from 2020-01-01 to 2026-02-01.
		{"spread", "completed", 120, `{"2020-01-01": 1, "2026-01-01": 50, "2026-01-10": 48}`, 30, 3, 10, 2},
		{"spread", "completed", 105, `{"2026-01-05": 100, "2026-02-01": 1}`, 30, 0, 5, 0},
		{"spread", "in progress", 999, `{"1990-01-01": 5000, "2040-01-01": 5000}`, 30, 100, 100, 100},
		{"spread", "completed", 0, nil, nil, nil, nil, nil},
		{"one_day", "completed", 4, `{"2026-03-03": 4}`, nil, nil, nil, nil},
		{"no_window", "completed", 4, nil, nil, nil, nil, nil},
		{"window_no_drops", "completed", 4, `{"2026-03-03": 4}`, 400, 0, 0, 0},
	} {
		var lo, hi any
		if d.days != nil {
			lo, hi = "2026-01-01", "2026-01-01"
			if d.run == "spread" && d.status == "completed" {
				lo, hi = "2020-01-01", "2026-02-01"
			}
		}
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_details
			(pipeline_config_key, pipeline_execution_status_key, client, process_name, main_input_session_id,
			 session_id, source_period_key, status, user_email, cpipes_step_id, input_records_count,
			 input_span_column, input_span_min, input_span_max, input_span_day_counts,
			 retain_window_column, retain_window_days_before, retain_window_days_after,
			 dropped_no_date_count, dropped_before_window_count, dropped_after_window_count)
			VALUES (1, $1, 'c', 'proc', 's', 'x', 1, $2, 'system', 'reducing00', $3,
			        'dos', $4::date, $5::date, $6::jsonb, 'dos', $7, 0, $8, $9, $10)`,
			runs[d.run], d.status, d.n, lo, hi, d.days, d.before, d.noDate, d.early, d.late)
	}
	for _, k := range runs {
		if err := UpdateRunInputSummary(ctx, pool, k); err != nil {
			t.Fatal(err)
		}
	}
	for name, want := range map[string][4]any{
		"spread":          {int64(225), int32(2223), int32(9), int64(20)},
		"one_day":         {int64(4), int32(0), int32(0), nil},
		"no_window":       {int64(4), nil, nil, nil},
		"window_no_drops": {int64(4), int32(0), int32(0), int64(0)},
	} {
		var n, dropped *int64
		var span, core *int32
		if err := pool.QueryRow(ctx, `SELECT main_input_records_count, input_span_days, input_core_span_days,
			dropped_records_count FROM jetsapi.pipeline_execution_history WHERE key = $1`, runs[name]).
			Scan(&n, &span, &core, &dropped); err != nil {
			t.Fatal(err)
		}
		if got := [4]any{deref(n), deref(span), deref(core), deref(dropped)}; got != want {
			t.Errorf("%s: count, span, core span, dropped = %v, want %v", name, got, want)
		}
	}
}

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
