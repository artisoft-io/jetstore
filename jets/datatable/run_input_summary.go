package datatable

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// execer is the one method UpdateRunInputSummary needs, so a test can pass a
// pool or a transaction.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// runInputSummaryStmt writes the summary columns of a run's
// pipeline_execution_status row from its pipeline_execution_details rows:
//
//   - main_input_records_count, the records read from the main input: the sum
//     of input_records_count over step reducing00, the count start_reducing_cp
//     passes on as main_input_row_count. It counts every record read, before a
//     retain window drops any;
//   - input_span_days, the days from the earliest input_span_min to the latest
//     input_span_max, which a node records when a map_record step names a
//     column in map_record_config.calculate_span_of;
//   - input_core_span_days, the days from the nearest-rank 1st percentile date
//     of that column to the 99th, over the nodes' input_span_day_counts merged
//     by day: the first day whose running count reaches ceil(0.01 n), to the
//     first whose running count reaches ceil(0.99 n), for n dates in all;
//   - dropped_records_count, the records a map_record retain window dropped,
//     all reasons, over the nodes that recorded a window.
//
// A node still in progress is not counted, as cpipes_execution_status_details
// does not count it. A column is NULL when no detail row gives it.
const runInputSummaryStmt = `
WITH d AS (
  SELECT sum(input_records_count) FILTER (WHERE cpipes_step_id = 'reducing00')::bigint AS n,
         (max(input_span_max) - min(input_span_min))::int AS span,
         sum(coalesce(dropped_no_date_count, 0) + coalesce(dropped_before_window_count, 0)
             + coalesce(dropped_after_window_count, 0))
           FILTER (WHERE retain_window_days_before IS NOT NULL)::bigint AS dropped
    FROM jetsapi.pipeline_execution_details
   WHERE pipeline_execution_status_key = $1 AND status != 'in progress'
), days AS (
  SELECT e.key::date AS day, sum(e.value::bigint) AS n
    FROM jetsapi.pipeline_execution_details pd
   CROSS JOIN LATERAL jsonb_each_text(pd.input_span_day_counts) e
   WHERE pd.pipeline_execution_status_key = $1 AND pd.status != 'in progress'
   GROUP BY e.key::date
), running AS (
  SELECT day, sum(n) OVER (ORDER BY day) AS c, sum(n) OVER () AS total FROM days
), core AS (
  SELECT (min(day) FILTER (WHERE c >= ceil(0.99 * total))
          - min(day) FILTER (WHERE c >= ceil(0.01 * total)))::int AS span
    FROM running
)
UPDATE jetsapi.pipeline_execution_status pe
   SET main_input_records_count = d.n, input_span_days = d.span,
       input_core_span_days = core.span, dropped_records_count = d.dropped
  FROM d, core
 WHERE pe.key = $1`

// UpdateRunInputSummary writes the run's main input count, input span, core
// span and dropped count to its pipeline_execution_status row, for the
// Pipeline Status table.
func UpdateRunInputSummary(ctx context.Context, db execer, peKey int) error {
	if _, err := db.Exec(ctx, runInputSummaryStmt, peKey); err != nil {
		return fmt.Errorf("while writing the input count and span of pipeline execution %d: %v", peKey, err)
	}
	return nil
}
