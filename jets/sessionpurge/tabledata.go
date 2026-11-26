package sessionpurge

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/detectrun"
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
// generator), so what depends on these rows is a report run again later, a
// person reading them, and the agentic layer's claim-set summary, which is
// computed from them once and judged from thereafter (claimSetHold).

// ProcessErrorsTable is the key in TableDataReport.Rows for jetsapi.process_errors.
const ProcessErrorsTable = "jetsapi.process_errors"

// TableDataReport says what a table-data purge did, or for a dry run what it
// would have done.
type TableDataReport struct {
	// Candidates is the sessions selected: registered, past the table-data clock,
	// not past the session's own, not queued for deletion and not yet purged.
	// Held sessions are among them.
	Candidates int
	// Held is the sessions whose claim rows a claim-set summary may still need
	// (claimSetHold); they are left unmarked and selected again by the next run.
	Held int
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
	Skipped                                       int
	RefusedSessions, FailedSessions, HeldSessions []string
	FailedErrors                                  map[string]string
	// Rows is the rows deleted per table: domain tables by their
	// input_registry.table_name, process_errors as ProcessErrorsTable.
	Rows map[string]int64
}

func newTableDataReport() TableDataReport {
	return TableDataReport{Rows: map[string]int64{}, FailedErrors: map[string]string{}}
}

// Summary is the report in a line, for a log.
func (r *TableDataReport) Summary() string {
	return fmt.Sprintf("%d sessions' table data deleted of %d due, %d refused (a run pending or submitted), "+
		"%d held (claim rows not yet summarised), %d failed, %d not reached before the deadline, "+
		"%d deleted or queued meanwhile", r.Done, r.Candidates, r.Refused, r.Held, r.Failed, r.NotReached, r.Skipped)
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
// deletion queued or failed. Oldest first. Beside each it reads the hold, a
// boolean expression of sr (claimSetHold), which may bind $3 to $5.
func tableDataCandidatesStmt(hold string) string {
	return `SELECT sr.session_id, (` + hold + `) FROM jetsapi.session_registry sr
	WHERE sr.last_update <= $1::timestamptz AND sr.last_update > $2::timestamptz
		AND sr.table_data_purged_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM jetsapi.session_deletion d
			WHERE d.session_id = sr.session_id AND d.status <> 'done')
	ORDER BY sr.last_update, sr.session_id`
}

// claimSetHoldPrerequisitesStmt says whether the claim-set summary is deployed:
// its table, and the record of the feed_detect runs that write it.
const claimSetHoldPrerequisitesStmt = `SELECT to_regclass('jetsapi.claim_set_summary') IS NOT NULL,
	to_regclass('jetsapi.agentic_detection_run') IS NOT NULL`

// claimSetTablesStmt reads the claim tables the feed profiles name, read
// through to_jsonb of the row as jets/agentic/feedprofile reads it, so it runs
// against a database whose source_config has no feed_profile_json.
const claimSetTablesStmt = `SELECT DISTINCT to_jsonb(sc) -> 'feed_profile_json' -> 'claim_set' ->> 'table'
	FROM jetsapi.source_config sc
	WHERE jsonb_typeof(to_jsonb(sc) -> 'feed_profile_json' -> 'claim_set') = 'object'
		AND (to_jsonb(sc) -> 'feed_profile_json' -> 'claim_set' ->> 'table') <> ''
	ORDER BY 1`

// claimSetDryRunPattern matches the summary a dry run of feed_detect records,
// as encoding/json writes observe.FeedProcessSummary: a dry run writes no
// summary, so it releases nothing.
const claimSetDryRunPattern = `%"dryRun":true%`

// claimSetHold returns the hold of the table-data purge, as a boolean
// expression of the candidate sr for tableDataCandidatesStmt, with the values
// it binds from $3. A session is held, its table data kept, when all of:
//
//  1. it has rows in a claim table, a table some source_config.feed_profile_json
//     names as claim_set.table (a bare name in public, "schema.table" naming
//     both, as observe.ClaimSetTable reads it), that exists and has a
//     session_id column;
//  2. no jetsapi.claim_set_summary row has its session_id, so feed_detect has not
//     summarised the rows yet;
//  3. no feed_detect run recorded in jetsapi.agentic_detection_run started after
//     the session's last_update and ended ok, a dry run aside. A run that did
//     and wrote no summary of the session never will (a pipeline run that read
//     a delivery another run had loaded, or a feed with no valid profile), so
//     the session is released.
//
// Deleting a held session's claim rows would leave its delivery with no summary
// to be judged from, ever. The hold never outlasts RETENTION_DAYS: the session
// deletion, which deletes the session whole, claim rows and summary with it, is
// unchanged, and the candidates are only sessions it has not reached.
//
// Where claim_set_summary or agentic_detection_run does not exist, the feature
// is not deployed and nothing is held; nor where no profile names a claim table.
// Each table is an identifier quoted by pgx.Identifier and guarded by
// to_regclass; one that does not resolve, or has no session_id, is skipped and
// logged. Each is indexed on session_id, as the purge already assumes.
func claimSetHold(ctx context.Context, db Querier) (string, []any, error) {
	var summaries, runs bool
	if err := db.QueryRow(ctx, claimSetHoldPrerequisitesStmt).Scan(&summaries, &runs); err != nil {
		return "", nil, fmt.Errorf("while looking for the claim-set summary: %w", err)
	}
	if !summaries || !runs {
		return "false", nil, nil
	}
	rows, err := db.Query(ctx, claimSetTablesStmt)
	if err != nil {
		return "", nil, fmt.Errorf("while reading the claim tables of the feed profiles: %w", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", nil, fmt.Errorf("while reading the claim tables of the feed profiles: %w", err)
	}
	var idents []string
	var skipped []string
	for _, name := range names {
		parts := strings.Split(name, ".")
		switch len(parts) {
		case 1:
			idents = append(idents, pgx.Identifier{"public", parts[0]}.Sanitize())
		case 2:
			idents = append(idents, pgx.Identifier{parts[0], parts[1]}.Sanitize())
		default:
			skipped = append(skipped, name)
		}
	}
	found := map[string]bool{}
	if len(idents) > 0 {
		rows, err = db.Query(ctx, `SELECT t.ident FROM unnest($1::text[]) AS t(ident)
			JOIN pg_class c ON c.oid = to_regclass(t.ident) AND c.relkind IN ('r', 'p')
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'session_id' AND NOT a.attisdropped`, idents)
		if err != nil {
			return "", nil, fmt.Errorf("while resolving the claim tables: %w", err)
		}
		resolved, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return "", nil, fmt.Errorf("while resolving the claim tables: %w", err)
		}
		for _, ident := range resolved {
			found[ident] = true
		}
	}
	var has []string
	for _, ident := range idents {
		if found[ident] {
			has = append(has, `EXISTS (SELECT 1 FROM `+ident+` x WHERE x.session_id = sr.session_id)`)
		} else {
			skipped = append(skipped, ident)
		}
	}
	if len(skipped) > 0 {
		log.Printf("sessionpurge: %d claim tables named by a feed profile skipped, not a table or no session_id: %s",
			len(skipped), strings.Join(skipped, ", "))
	}
	if len(has) == 0 {
		return "false", nil, nil
	}
	hold := `(` + strings.Join(has, `
			OR `) + `)
		AND NOT EXISTS (SELECT 1 FROM jetsapi.claim_set_summary s WHERE s.session_id = sr.session_id)
		AND NOT EXISTS (SELECT 1 FROM jetsapi.agentic_detection_run r
			WHERE r.command = $3 AND r.status = $4 AND r.started_at > sr.last_update
				AND coalesce(r.summary, '') NOT LIKE $5)`
	return hold, []any{detectrun.CommandFeedDetect, detectrun.StatusOk, claimSetDryRunPattern}, nil
}

// PurgeTableData deletes the table data of the sessions registered on or before
// before and after wholeBefore, which is the session deletion's own cutoff, so
// the sessions it deletes whole are left to it. Each session is one transaction
// of db: its rows in every domain table and in process_errors are deleted, then
// its session_registry row is marked. A session with a run pending or submitted
// on it or reading it is refused, as the session deletion refuses it, and left
// unmarked. A session whose claim rows a claim-set summary may still need is
// held (claimSetHold): counted, not touched, left unmarked, and selected again
// by the next run. A session that fails is rolled back, counted and logged; it
// is selected again by the next run. The deadline and the context are checked
// before each session.
//
// In a real run (opts.DryRun false) db is the pool, and each session takes the
// session lock (LockSession) first, so it is ordered against a pipeline start
// and against a deletion of the same session. In a dry run db is the dry run's
// transaction, read after its session deletions: each session is a savepoint of
// it, no lock is taken, and the caller rolls the whole of it back.
//
// It returns an error only when it cannot read its work: the tables, the hold
// or the candidates, as on a database whose session_registry has no
// table_data_purged_at yet.
func PurgeTableData(ctx context.Context, db TxBeginner, before, wholeBefore time.Time, opts Options) (TableDataReport, error) {
	report := newTableDataReport()
	hold, holdArgs, err := claimSetHold(ctx, db)
	if err != nil {
		return report, err
	}
	rows, err := db.Query(ctx, tableDataCandidatesStmt(hold), append([]any{before, wholeBefore}, holdArgs...)...)
	if err != nil {
		return report, fmt.Errorf("while reading the sessions whose table data is due: %w", err)
	}
	type candidate struct {
		id   string
		held bool
	}
	cands, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
		var c candidate
		err := row.Scan(&c.id, &c.held)
		return c, err
	})
	if err != nil {
		return report, fmt.Errorf("while reading the sessions whose table data is due: %w", err)
	}
	report.Candidates = len(cands)
	var ids []string
	for _, c := range cands {
		if c.held {
			report.Held++
			report.HeldSessions = append(report.HeldSessions, c.id)
		} else {
			ids = append(ids, c.id)
		}
	}
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
