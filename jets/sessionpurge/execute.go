package sessionpurge

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Options govern one Execute or ExecuteRequests.
type Options struct {
	Deadline time.Time // stop starting sessions after this; zero for none
	DryRun   bool      // every transaction rolled back; nothing written
	// Reasons limits the queued rows taken to those with one of these reasons;
	// empty takes every reason. The apiserver passes ReasonUser, so a user's
	// Delete never waits behind a retention backlog; the purge lambda passes none.
	Reasons []Reason
}

// Report says what a run did, or for a dry run what it would have done.
type Report struct {
	Done, Failed, Refused int
	// NotReached is the sessions not started because the deadline passed. They
	// stay queued for the next run.
	NotReached int
	// Skipped is the queued sessions another executor held when this one reached
	// them.
	Skipped int
	// ByReason is Done by the deletion's reason.
	ByReason map[Reason]int
	// RefusedSessions and FailedSessions name the sessions behind Refused and
	// Failed.
	RefusedSessions, FailedSessions []string
	// Rows is the rows deleted per table, and under ArchiveTable the
	// pipeline_execution_status rows archived. Domain tables are keyed by their
	// input_registry.table_name, the fixed tables by their qualified name.
	Rows map[string]int64
}

func newReport() Report {
	return Report{ByReason: map[Reason]int{}, Rows: map[string]int64{}}
}

// ArchiveTable is the key in Report.Rows for the status rows archived.
const ArchiveTable = "jetsapi.pipeline_execution_status_archive"

// fixedTables are deleted from by session_id after the domain tables, in this
// order. session_registry is not among them: it is deleted last of all, after the
// coordinator tables and the archive move. Each is guarded as a domain table is,
// so a database not yet migrated skips the tables it lacks.
var fixedTables = []string{
	"input_registry",
	"input_loader_status",
	"process_errors",
	"pipeline_execution_details",
	"pipeline_execution_channel_details",
	"cpipes_execution_status",
	"cpipes_execution_status_details",
	"report_execution_status",
	"compute_pipes_shard_registry",
	"compute_pipes_partitions_registry",
	"cpipes_results",
	"cpipes_metrics",
	"session_reservation",
	"pipeline_lock",
}

// statusColumns are pipeline_execution_status's columns, every one, in
// jets_schema.json's order. The archive has them all, with the same names and
// types; TestStatusColumnsAgree fails until this list, the two tables in
// jets_schema.json and the view pipeline_execution_history agree.
var statusColumns = []string{
	"key", "pipeline_config_key", "main_input_registry_key", "main_input_file_key",
	"merged_input_registry_keys", "client", "process_name", "main_object_type", "input_session_id", "request_id",
	"session_id", "source_period_key", "status", "failure_details", "user_email", "start_time", "last_update",
	"failure_class", "failure_source", "workspace_name", "workspace_version",
}

var archiveStmt = func() string {
	cols := strings.Join(statusColumns, ", ")
	return fmt.Sprintf(`INSERT INTO jetsapi.pipeline_execution_status_archive (%s, archive_reason)
		SELECT %s, $2 FROM jetsapi.pipeline_execution_status WHERE session_id = $1`, cols, cols)
}()

// table is one table deleted from by session_id.
type table struct {
	name  string // the key in Report.Rows
	ident string // quoted, for the statement and for to_regclass
}

// resolveTables returns the tables to delete from: every table_name registered in
// input_registry, then the fixed tables, each kept only if it resolves to a table
// with a session_id column. A name that does not is logged and skipped.
func resolveTables(ctx context.Context, db *pgxpool.Pool) ([]table, error) {
	var candidates []table
	rows, err := db.Query(ctx, `SELECT DISTINCT table_name FROM jetsapi.input_registry ORDER BY table_name`)
	if err != nil {
		return nil, fmt.Errorf("while reading the domain tables from input_registry: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		// An unqualified name, quoted as today's purge quoted it, so it resolves
		// through the search path to the same table.
		candidates = append(candidates, table{name: name, ident: pgx.Identifier{name}.Sanitize()})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, name := range fixedTables {
		candidates = append(candidates, table{name: "jetsapi." + name, ident: pgx.Identifier{"jetsapi", name}.Sanitize()})
	}

	idents := make([]string, len(candidates))
	for i := range candidates {
		idents[i] = candidates[i].ident
	}
	// to_regclass returns NULL for a name that does not resolve, rather than
	// raising, so one statement sorts the whole list.
	rows, err = db.Query(ctx, `SELECT t.ident
		FROM unnest($1::text[]) AS t(ident)
		JOIN pg_class c ON c.oid = to_regclass(t.ident) AND c.relkind IN ('r', 'p')
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'session_id' AND NOT a.attisdropped`, idents)
	if err != nil {
		return nil, fmt.Errorf("while resolving the tables to purge: %w", err)
	}
	found := map[string]bool{}
	for rows.Next() {
		var ident string
		if err := rows.Scan(&ident); err != nil {
			rows.Close()
			return nil, err
		}
		found[ident] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]table, 0, len(candidates))
	var skipped []string
	for _, t := range candidates {
		if found[t.ident] {
			out = append(out, t)
		} else {
			skipped = append(skipped, t.ident)
		}
	}
	if len(skipped) > 0 {
		log.Printf("sessionpurge: %d registered tables skipped, not a table or no session_id column: %s",
			len(skipped), strings.Join(skipped, ", "))
	}
	return out, nil
}

// Execute works through the pending and failed rows of session_deletion, oldest
// first, one session per transaction (see the package comment). It returns an
// error only when it cannot read its work; a session that fails is counted, logged
// and marked failed. With opts.DryRun it is ExecuteRequests with no requests.
func Execute(ctx context.Context, db *pgxpool.Pool, opts Options) (Report, error) {
	if opts.DryRun {
		return ExecuteRequests(ctx, db, nil, opts)
	}
	queued, err := queuedRequests(ctx, db, opts.Reasons)
	if err != nil {
		return newReport(), fmt.Errorf("while reading session_deletion: %w", err)
	}
	return run(ctx, db, queued, opts, true)
}

// ExecuteRequests is Execute for a dry run: it does what Queue(reqs) followed by
// Execute would do, on requests held in memory, and rolls every transaction back.
// The rows already pending or failed in session_deletion come first, as Execute
// would take them, and reqs are merged in by Queue's rule, so the report predicts
// a real run's. It refuses opts.DryRun false: a real run queues first.
func ExecuteRequests(ctx context.Context, db *pgxpool.Pool, reqs []Request, opts Options) (Report, error) {
	if !opts.DryRun {
		return newReport(), errors.New("sessionpurge.ExecuteRequests is for a dry run only; Queue the requests and Execute them")
	}
	for i := range reqs {
		if err := reqs[i].validate(); err != nil {
			return newReport(), err
		}
	}
	queued, err := queuedRequests(ctx, db, opts.Reasons)
	if err != nil {
		return newReport(), fmt.Errorf("while reading session_deletion: %w", err)
	}
	return run(ctx, db, merge(queued, reqs), opts, false)
}

// run works through reqs. A real run gives each session a transaction of its own,
// committed when the session is done. A dry run (opts.DryRun) runs every session
// in one transaction, each under a savepoint that is released when the session
// would be done and rolled back to when it fails or is refused, and rolls the
// whole transaction back at the end. So each session sees what the sessions
// before it deleted, as in a real run, and the report predicts a real run's for
// rows shared between sessions, such as a coordinator request's map row.
//
// The price is that a dry run holds the row locks it takes until it ends. They are
// the locks a real run would take one session at a time: on the rows of the
// sessions it deletes, and, by the coordinator rule in deleteRows, on no map row
// of a request that still has a live session.
//
// It holds the session locks (LockSession) to its end as well, and that is the
// one place the lock order every other taker keeps does not hold: a dry run takes
// many session locks, in queue order rather than sorted, and holds them while it
// takes row locks. Two consequences, both accepted:
//
//   - a pipeline start reading one of the sessions the dry run has reached waits
//     for the dry run to end (a start on a retention candidate, during the purge
//     lambda's dry run), then goes ahead, since a dry run queues nothing;
//   - a delete_sessions on sessions the dry run holds, or an executor whose
//     session shares a coordinator map row with one the dry run has deleted, can
//     form a cycle with it. PostgreSQL detects the deadlock and aborts one side:
//     delete_sessions answers 500 and the user tries again; an executor's session
//     is marked failed and retried; a dry run's session is counted failed.
func run(ctx context.Context, db *pgxpool.Pool, reqs []Request, opts Options, claim bool) (Report, error) {
	report := newReport()
	if len(reqs) == 0 {
		return report, nil
	}
	tables, err := resolveTables(ctx, db)
	if err != nil {
		return report, err
	}
	var dry pgx.Tx
	if opts.DryRun {
		claim = false
		if dry, err = db.Begin(ctx); err != nil {
			return report, fmt.Errorf("while starting the dry run's transaction: %w", err)
		}
		// Nothing a dry run did is ever committed.
		defer dry.Rollback(context.Background())
	}
	for i := range reqs {
		if err := ctx.Err(); err != nil {
			report.NotReached += len(reqs) - i
			return report, err
		}
		if !opts.Deadline.IsZero() && !time.Now().Before(opts.Deadline) {
			report.NotReached += len(reqs) - i
			log.Printf("sessionpurge: deadline %s passed, %d sessions left queued for the next run",
				opts.Deadline.Format(time.RFC3339), len(reqs)-i)
			break
		}
		r := &reqs[i]
		var res result
		if opts.DryRun {
			res, err = dryRunSession(ctx, dry, r, tables)
			if errors.Is(err, errDryRunBroken) {
				report.NotReached += len(reqs) - i
				return report, err
			}
		} else {
			res, err = deleteSession(ctx, db, r, tables, claim)
		}
		switch {
		case err != nil:
			report.Failed++
			report.FailedSessions = append(report.FailedSessions, r.SessionID)
			log.Printf("sessionpurge: session %s (%s) failed and was rolled back: %v", r.SessionID, r.Reason, err)
			if claim {
				markFailed(db, r.SessionID, err)
			}
		case res.held:
			report.Skipped++
		case res.refused:
			report.Refused++
			report.RefusedSessions = append(report.RefusedSessions, r.SessionID)
			log.Printf("sessionpurge: session %s (%s) refused, a run on it or reading it is pending or submitted",
				r.SessionID, r.Reason)
		default:
			report.Done++
			report.ByReason[r.Reason]++
			for name, n := range res.rows {
				report.Rows[name] += n
			}
		}
	}
	return report, nil
}

type result struct {
	held, refused bool
	rows          map[string]int64
}

// errDryRunBroken: the dry run's transaction cannot go on, so the run stops.
var errDryRunBroken = errors.New("the dry run's transaction cannot continue")

// deleteSession deletes one session in a transaction of its own, committed when
// the session is done and rolled back otherwise. With claim, the session's
// deletion row is locked first and marked done last; a row another executor holds
// is left to it.
func deleteSession(ctx context.Context, db *pgxpool.Pool, r *Request, tables []table, claim bool) (res result, err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return res, err
	}
	// After a commit this is a no-op; otherwise it is the rollback, on every path.
	defer tx.Rollback(context.Background())
	res, err = deleteRows(ctx, tx, r, tables, claim)
	if err != nil || res.held || res.refused {
		return res, err
	}
	if err = tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("while committing: %w", err)
	}
	return res, nil
}

// dryRunSession does one session's work under a savepoint of the dry run's
// transaction: released when the session would be done, so the sessions after it
// see its rows gone, and rolled back to otherwise, so a failure leaves the dry run
// going as it leaves a real run going.
func dryRunSession(ctx context.Context, dry pgx.Tx, r *Request, tables []table) (res result, err error) {
	sp, err := dry.Begin(ctx) // a SAVEPOINT
	if err != nil {
		return res, fmt.Errorf("%w: savepoint: %v", errDryRunBroken, err)
	}
	res, err = deleteRows(ctx, sp, r, tables, false)
	if err != nil || res.refused {
		if rbErr := sp.Rollback(context.Background()); rbErr != nil {
			return res, fmt.Errorf("%w: rolling back to the savepoint after %v: %v", errDryRunBroken, err, rbErr)
		}
		return res, err
	}
	if err = sp.Commit(ctx); err != nil { // RELEASE SAVEPOINT
		return res, fmt.Errorf("%w: releasing the savepoint: %v", errDryRunBroken, err)
	}
	return res, nil
}

// deleteRows does one session's work in tx, which the caller commits or rolls
// back: take the session lock (LockSession); refuse the session if a run on it or
// reading it is active; otherwise delete it from every table, archive its runs,
// delete session_registry last and, with claim, mark its deletion row done.
//
// The lock is taken first, before the claim and so before the refusal, in a real
// run and in a dry run alike. A pipeline start takes the same lock before it
// inserts its status row (jets/datatable, InsertPipelineExecutionStatus), so the
// two are ordered: a start that holds it commits its pending or submitted row
// before the refusal below reads, and a start that waits for it finds the session
// queued.
func deleteRows(ctx context.Context, tx pgx.Tx, r *Request, tables []table, claim bool) (res result, err error) {
	// The session lock first, before any row lock: see LockSession for the order
	// every taker keeps. A second executor on the same session waits here, and
	// then finds the deletion row done and counts the session held.
	if err = LockSession(ctx, tx, r.SessionID); err != nil {
		return res, err
	}
	if claim {
		var status string
		err = tx.QueryRow(ctx, `SELECT status FROM jetsapi.session_deletion
			WHERE session_id = $1 AND status <> 'done' FOR UPDATE SKIP LOCKED`, r.SessionID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			res.held = true
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("while claiming the deletion row: %w", err)
		}
	}

	var active bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status
		WHERE `+activeRunWhere("$1")+`)`, r.SessionID).Scan(&active)
	if err != nil {
		return res, fmt.Errorf("while checking for an active run: %w", err)
	}
	if active {
		res.refused = true
		return res, nil
	}

	res.rows = map[string]int64{}
	exec := func(name, stmt string, args ...any) error {
		tag, err := tx.Exec(ctx, stmt, args...)
		if err != nil {
			return fmt.Errorf("on %s: %w", name, err)
		}
		res.rows[name] += tag.RowsAffected()
		return nil
	}

	for _, t := range tables {
		if err = exec(t.name, "DELETE FROM "+t.ident+" WHERE session_id = $1", r.SessionID); err != nil {
			return res, err
		}
	}
	// The coordinator tables. The session's lock and item rows go; a request's map
	// row goes only when the last of its items has gone, so a request shared with
	// a live session keeps its map row, and one whose sessions are all deleted loses
	// it with the last of them.
	if err = exec("jetsapi.pipeline_coordinator_lock",
		`DELETE FROM jetsapi.pipeline_coordinator_lock WHERE session_id = $1`, r.SessionID); err != nil {
		return res, err
	}
	// The map rows of the session's requests are locked before its items go, in
	// request order, so that two executors deleting the last two sessions of one
	// request take them one after the other: the second waits here, and its
	// NOT EXISTS below, a new statement, sees the first one's items gone. Without
	// the lock each saw the other's item still there and both kept the map row.
	if _, err = tx.Exec(ctx, `SELECT m.request_id FROM jetsapi.pipeline_coordinator_map m
		WHERE m.request_id IN (SELECT i.request_id FROM jetsapi.pipeline_coordinator_map_items i WHERE i.session_id = $1)
		ORDER BY m.request_id FOR UPDATE OF m`, r.SessionID); err != nil {
		return res, fmt.Errorf("while locking the coordinator map rows: %w", err)
	}
	rows, err := tx.Query(ctx, `DELETE FROM jetsapi.pipeline_coordinator_map_items WHERE session_id = $1
		RETURNING request_id`, r.SessionID)
	if err != nil {
		return res, fmt.Errorf("on jetsapi.pipeline_coordinator_map_items: %w", err)
	}
	requests, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return res, fmt.Errorf("on jetsapi.pipeline_coordinator_map_items: %w", err)
	}
	res.rows["jetsapi.pipeline_coordinator_map_items"] += int64(len(requests))
	res.rows["jetsapi.pipeline_coordinator_map"] += 0 // reported, as every table is, when nothing goes
	if len(requests) > 0 {
		if err = exec("jetsapi.pipeline_coordinator_map", `DELETE FROM jetsapi.pipeline_coordinator_map m
			WHERE m.request_id = ANY($1::text[])
				AND NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_coordinator_map_items i WHERE i.request_id = m.request_id)`,
			requests); err != nil {
			return res, err
		}
	}
	// The session's runs move to the archive with their key.
	if err = exec(ArchiveTable, archiveStmt, r.SessionID, string(r.Reason)); err != nil {
		return res, err
	}
	if err = exec("jetsapi.pipeline_execution_status",
		`DELETE FROM jetsapi.pipeline_execution_status WHERE session_id = $1`, r.SessionID); err != nil {
		return res, err
	}
	// Last: once this row is gone the session is no longer a retention candidate,
	// so nothing may be left behind it.
	if err = exec("jetsapi.session_registry",
		`DELETE FROM jetsapi.session_registry WHERE session_id = $1`, r.SessionID); err != nil {
		return res, err
	}
	if claim {
		if _, err = tx.Exec(ctx, `UPDATE jetsapi.session_deletion
			SET status = 'done', completed_at = now(), error = NULL WHERE session_id = $1`, r.SessionID); err != nil {
			return res, fmt.Errorf("while marking the deletion done: %w", err)
		}
	}
	return res, nil
}

// markFailed records a session's failure in a transaction of its own, after its
// work was rolled back. It runs on a fresh context so a cancelled run still
// records why. A failure to record is logged: the row stays as it was and is
// retried.
func markFailed(db *pgxpool.Pool, sessionID string, cause error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := db.Exec(ctx, `UPDATE jetsapi.session_deletion
		SET status = 'failed', error = $2 WHERE session_id = $1 AND status <> 'done'`, sessionID, cause.Error())
	if err != nil {
		log.Printf("sessionpurge: while marking session %s failed: %v", sessionID, err)
	}
}
