package sessionpurge

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// The table-data purge: a second retention clock, shorter than the session's,
// for the rows a session loaded or produced. A session past it keeps its
// registration, its runs, their details, its agentic rows and its summary, and
// loses its rows in every domain table (every input_registry.table_name,
// resolved and guarded as the session deletion resolves them) and in
// jetsapi.process_errors, which stores the encoded rules session and messages
// that can quote a value. session_registry.table_data_purged_at then records
// that it is done, so the session is never selected again.
//
// No pipeline reads a table (its input channels are memory, input, stage or
// generator), so what depends on these rows is a report run again later and a
// person reading them.

// ProcessErrorsTable is the key in TableDataReport.Rows for jetsapi.process_errors.
const ProcessErrorsTable = "jetsapi.process_errors"

// TableDataReport says what a table-data purge did, or for a dry run what it
// would have done.
type TableDataReport struct {
	// Candidates is the sessions selected: registered, past the table-data clock,
	// not past the session's own, not queued for deletion and not yet purged.
	Candidates int
	// Done is the sessions whose table data was deleted and which were marked.
	Done int
	// Refused is the sessions with a run on them, or reading them, pending or
	// submitted; they are left unmarked and selected again by the next run.
	Refused int
	// Failed is the sessions whose purge failed and was rolled back; they are
	// selected again by the next run.
	Failed int
	// NotReached is the sessions not started because the deadline passed.
	NotReached int
	// Skipped is the sessions found deleted, queued for deletion or already
	// marked by the time their turn came.
	Skipped                         int
	RefusedSessions, FailedSessions []string
	FailedErrors                    map[string]string
	// Rows is the rows deleted per table: domain tables by their
	// input_registry.table_name, process_errors as ProcessErrorsTable.
	Rows map[string]int64
}

func newTableDataReport() TableDataReport {
	return TableDataReport{Rows: map[string]int64{}, FailedErrors: map[string]string{}}
}

// Summary is the report in a line, for a log.
func (r *TableDataReport) Summary() string {
	return fmt.Sprintf("%d sessions' table data deleted of %d due, %d refused (a run pending or submitted), %d failed, "+
		"%d not reached before the deadline, %d deleted or queued meanwhile", r.Done, r.Candidates, r.Refused, r.Failed,
		r.NotReached, r.Skipped)
}

// TxBeginner is a pool, whose Begin starts a transaction, or a transaction,
// whose Begin starts a savepoint.
type TxBeginner interface {
	Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

// tableDataCandidatesStmt selects the sessions due: registered on or before $1
// (the table-data clock), after $2 (the session's own clock, so a session the
// purge deletes whole this run is not selected), not yet purged, and with no
// deletion queued or failed. Oldest first.
const tableDataCandidatesStmt = `SELECT sr.session_id FROM jetsapi.session_registry sr
	WHERE sr.last_update <= $1::timestamptz AND sr.last_update > $2::timestamptz
		AND sr.table_data_purged_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM jetsapi.session_deletion d
			WHERE d.session_id = sr.session_id AND d.status <> 'done')
	ORDER BY sr.last_update, sr.session_id`

// PurgeTableData deletes the table data of the sessions registered on or before
// before and after wholeBefore, which is the session deletion's own cutoff, so
// the sessions it deletes whole are left to it. Each session is one transaction
// of db: its rows in every domain table and in process_errors are deleted, then
// its session_registry row is marked. A session with a run pending or submitted
// on it or reading it is refused, as the session deletion refuses it, and left
// unmarked. A session that fails is rolled back, counted and logged; it is
// selected again by the next run. The deadline and the context are checked
// before each session.
//
// In a real run (opts.DryRun false) db is the pool, and each session takes the
// session lock (LockSession) first, so it is ordered against a pipeline start
// and against a deletion of the same session. In a dry run db is the dry run's
// transaction, read after its session deletions: each session is a savepoint of
// it, no lock is taken, and the caller rolls the whole of it back.
//
// It returns an error only when it cannot read its work: the tables or the
// candidates, as on a database whose session_registry has no
// table_data_purged_at yet.
func PurgeTableData(ctx context.Context, db TxBeginner, before, wholeBefore time.Time, opts Options) (TableDataReport, error) {
	report := newTableDataReport()
	rows, err := db.Query(ctx, tableDataCandidatesStmt, before, wholeBefore)
	if err != nil {
		return report, fmt.Errorf("while reading the sessions whose table data is due: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return report, fmt.Errorf("while reading the sessions whose table data is due: %w", err)
	}
	report.Candidates = len(ids)
	if len(ids) == 0 {
		return report, nil
	}
	all, err := resolveTables(ctx, db)
	if err != nil {
		return report, err
	}
	var tables []table
	for _, t := range all {
		if t.domain || t.name == ProcessErrorsTable {
			tables = append(tables, t)
		}
	}
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			report.NotReached += len(ids) - i
			return report, err
		}
		if !opts.Deadline.IsZero() && !time.Now().Before(opts.Deadline) {
			report.NotReached += len(ids) - i
			log.Printf("sessionpurge: deadline %s passed, the table data of %d sessions is left for the next run",
				opts.Deadline.Format(time.RFC3339), len(ids)-i)
			break
		}
		res, err := purgeSessionTableData(ctx, db, id, tables, !opts.DryRun)
		switch {
		case errors.Is(err, errDryRunBroken):
			report.NotReached += len(ids) - i
			return report, err
		case err != nil:
			report.Failed++
			report.FailedSessions = append(report.FailedSessions, id)
			report.FailedErrors[id] = err.Error()
			log.Printf("sessionpurge: the table data of session %s failed to delete and was rolled back: %v", id, err)
		case res.held:
			report.Skipped++
		case res.refused:
			report.Refused++
			report.RefusedSessions = append(report.RefusedSessions, id)
		default:
			report.Done++
			for name, n := range res.rows {
				report.Rows[name] += n
			}
		}
	}
	return report, nil
}

// purgeSessionTableData does one session's table-data purge in a transaction
// (or savepoint) of db, committed (or released) when the session is done and
// rolled back otherwise. With lock, the session lock is taken first.
func purgeSessionTableData(ctx context.Context, db TxBeginner, id string, tables []table, lock bool) (res result, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		if !lock {
			return res, fmt.Errorf("%w: savepoint: %v", errDryRunBroken, err)
		}
		return res, err
	}
	done := false
	defer func() {
		if done {
			return
		}
		if rbErr := tx.Rollback(context.Background()); rbErr != nil && !lock {
			err = fmt.Errorf("%w: rolling back to the savepoint after %v: %v", errDryRunBroken, err, rbErr)
		}
	}()
	res, err = tableDataRows(ctx, tx, id, tables, lock)
	if err != nil || res.held || res.refused {
		return res, err
	}
	if err = tx.Commit(ctx); err != nil {
		if !lock {
			return res, fmt.Errorf("%w: releasing the savepoint: %v", errDryRunBroken, err)
		}
		return res, fmt.Errorf("while committing: %w", err)
	}
	done = true
	return res, nil
}

// tableDataRows does the work of purgeSessionTableData in tx.
func tableDataRows(ctx context.Context, tx pgx.Tx, id string, tables []table, lock bool) (res result, err error) {
	if lock {
		if err = LockSession(ctx, tx, id); err != nil {
			return res, err
		}
	}
	// Read again under the lock: a deletion that ran meanwhile removed the
	// registration, queued one, or another purge marked it.
	var due bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM jetsapi.session_registry sr
		WHERE sr.session_id = $1 AND sr.table_data_purged_at IS NULL
			AND NOT EXISTS (SELECT 1 FROM jetsapi.session_deletion d
				WHERE d.session_id = sr.session_id AND d.status <> 'done'))`, id).Scan(&due)
	if err != nil {
		return res, fmt.Errorf("while reading the session's registration: %w", err)
	}
	if !due {
		res.held = true
		return res, nil
	}
	var active bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status
		WHERE `+activeRunWhere("$1")+`)`, id).Scan(&active)
	if err != nil {
		return res, fmt.Errorf("while checking for an active run: %w", err)
	}
	if active {
		res.refused = true
		return res, nil
	}
	res.rows = map[string]int64{}
	for _, t := range tables {
		tag, err := tx.Exec(ctx, "DELETE FROM "+t.ident+" WHERE "+pgx.Identifier{t.column}.Sanitize()+" = $1", id)
		if err != nil {
			return res, fmt.Errorf("on %s: %w", t.name, err)
		}
		res.rows[t.name] += tag.RowsAffected()
	}
	if _, err = tx.Exec(ctx, `UPDATE jetsapi.session_registry SET table_data_purged_at = now()
		WHERE session_id = $1`, id); err != nil {
		return res, fmt.Errorf("while marking the session's table data purged: %w", err)
	}
	return res, nil
}
