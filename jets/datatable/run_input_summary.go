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

// runInputSummaryStmt writes the two summary columns of a run's
// pipeline_execution_status row from its pipeline_execution_details rows:
//
//   - main_input_records_count, the records read from the main input: the sum
//     of input_records_count over step reducing00, the count start_reducing_cp
//     passes on as main_input_row_count;
//   - input_span_days, the days from the earliest input_span_min to the latest
//     input_span_max, which a node records when a map_record step names a
//     column in map_record_config.calculate_span_of.
//
// A node still in progress is not counted, as cpipes_execution_status_details
// does not count it. Either column is NULL when no detail row gives it.
const runInputSummaryStmt = `
UPDATE jetsapi.pipeline_execution_status pe
   SET main_input_records_count = d.n, input_span_days = d.span
  FROM (
    SELECT sum(input_records_count) FILTER (WHERE cpipes_step_id = 'reducing00')::bigint AS n,
           (max(input_span_max) - min(input_span_min))::int AS span
      FROM jetsapi.pipeline_execution_details
     WHERE pipeline_execution_status_key = $1 AND status != 'in progress'
  ) d
 WHERE pe.key = $1`

// UpdateRunInputSummary writes the run's main input count and input span to
// its pipeline_execution_status row, for the Pipeline Status table.
func UpdateRunInputSummary(ctx context.Context, db execer, peKey int) error {
	if _, err := db.Exec(ctx, runInputSummaryStmt, peKey); err != nil {
		return fmt.Errorf("while writing the input count and span of pipeline execution %d: %v", peKey, err)
	}
	return nil
}
