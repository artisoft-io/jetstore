package datatable

import (
	"context"
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

func deref[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}
