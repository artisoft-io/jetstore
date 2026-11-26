package retention

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The audit tables PurgeAudit deletes from, the keys of its report.
const (
	AgentAuditTable    = "jetsapi.agent_audit"
	AgentRunTable      = "jetsapi.agent_run"
	ApprovalEventTable = "jetsapi.approval_event"
)

// AuditReport says what PurgeAudit deleted, or for a dry run would delete.
type AuditReport struct {
	// Rows is the rows deleted per table, for the tables that are there.
	Rows map[string]int64
	// Notes say what was left alone and why: a table not installed, the append-
	// only audit table when its guard cannot be lifted.
	Notes []string
}

// PurgeAudit deletes the agent's audit trail older than before, by each row's
// own timestamp. Nothing calls it unless a site sets a retention for the audit
// tables: by default no purge touches them.
//
//   - jetsapi.agent_audit, by run: a run's rows go when its latest row was
//     written before before, so every chain left is whole. The table is
//     append-only by a trigger, which is disabled for the deletion and enabled
//     again in the same transaction; a database user that cannot (not the
//     table's owner) leaves the table as it is, and the report says so.
//   - jetsapi.agent_run, by the run's end, or its start when it has none, and
//     only once no audit row of the run is left.
//   - jetsapi.approval_event, by decided_at.
//
// **jetsapi.change_proposal is not among them: it carries no timestamp to judge
// a row's age by**, and a proposal's age read from its approval events would
// delete a proposal nobody has decided yet. It is left as it is.
//
// One transaction, committed unless dryRun; a dry run counts what a real run
// would delete and deletes nothing.
func PurgeAudit(ctx context.Context, db DB, before time.Time, dryRun bool) (AuditReport, error) {
	r := AuditReport{Rows: map[string]int64{}}
	var audit, runs, approvals, guarded bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.agent_audit') IS NOT NULL,
	       to_regclass('jetsapi.agent_run') IS NOT NULL, to_regclass('jetsapi.approval_event') IS NOT NULL,
	       EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'agent_audit_immutable'
	                  AND tgrelid = to_regclass('jetsapi.agent_audit'))`).Scan(&audit, &runs, &approvals,
		&guarded); err != nil {
		return r, fmt.Errorf("while asking which audit tables are installed: %w", err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return r, err
	}
	defer tx.Rollback(context.Background())

	if audit {
		n, note, err := purgeAgentAudit(ctx, tx, before, dryRun, guarded)
		if err != nil {
			return r, err
		}
		if note != "" {
			r.Notes = append(r.Notes, note)
		} else {
			r.Rows[AgentAuditTable] = n
		}
	} else {
		r.Notes = append(r.Notes, AgentAuditTable+" is not installed")
	}

	if runs {
		// A run row goes only once its audit trail has: in a dry run, which
		// deletes nothing, the rows a real run would leave are those written on
		// or after before.
		trail := ""
		switch {
		case audit && dryRun:
			trail = ` AND NOT EXISTS (SELECT 1 FROM jetsapi.agent_audit a WHERE a.run_id = r.run_id
				AND a.created_at >= $1::timestamptz)`
		case audit:
			trail = ` AND NOT EXISTS (SELECT 1 FROM jetsapi.agent_audit a WHERE a.run_id = r.run_id)`
		}
		n, err := deleteOrCount(ctx, tx, dryRun, "jetsapi.agent_run r",
			`coalesce(r.ended_at, r.started_at) < $1::timestamptz`+trail, before)
		if err != nil {
			return r, fmt.Errorf("on %s: %w", AgentRunTable, err)
		}
		r.Rows[AgentRunTable] = n
	} else {
		r.Notes = append(r.Notes, AgentRunTable+" is not installed")
	}

	if approvals {
		n, err := deleteOrCount(ctx, tx, dryRun, "jetsapi.approval_event e", `e.decided_at < $1::timestamptz`, before)
		if err != nil {
			return r, fmt.Errorf("on %s: %w", ApprovalEventTable, err)
		}
		r.Rows[ApprovalEventTable] = n
	} else {
		r.Notes = append(r.Notes, ApprovalEventTable+" is not installed")
	}
	r.Notes = append(r.Notes, "jetsapi.change_proposal is kept: it has no timestamp to judge a row's age by")

	if dryRun {
		return r, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return r, fmt.Errorf("while committing the audit retention: %w", err)
	}
	return r, nil
}

// deleteOrCount deletes the rows of from matching where, or counts them in a
// dry run.
func deleteOrCount(ctx context.Context, tx pgx.Tx, dryRun bool, from, where string, args ...any) (int64, error) {
	if dryRun {
		var n int64
		err := tx.QueryRow(ctx, `SELECT count(*) FROM `+from+` WHERE `+where, args...).Scan(&n)
		return n, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM `+from+` WHERE `+where, args...)
	return tag.RowsAffected(), err
}

// purgeAgentAudit deletes, or counts, the agent_audit rows of the runs whose
// latest row was written before before. Lifting the append-only guard is done
// under a savepoint, so a user that may not do it leaves the table alone and
// the rest of the retention goes on; note says so.
func purgeAgentAudit(ctx context.Context, tx pgx.Tx, before time.Time, dryRun, guarded bool) (int64, string, error) {
	const old = `run_id IN (SELECT run_id FROM jetsapi.agent_audit GROUP BY run_id
		HAVING max(created_at) < $1::timestamptz)`
	if dryRun {
		var n int64
		err := tx.QueryRow(ctx, `SELECT count(*) FROM jetsapi.agent_audit WHERE `+old, before).Scan(&n)
		if err != nil {
			return 0, "", fmt.Errorf("on %s: %w", AgentAuditTable, err)
		}
		return n, "", nil
	}
	var n int64
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM jetsapi.agent_audit WHERE `+old, before).Scan(&n); err != nil {
		return 0, "", fmt.Errorf("on %s: %w", AgentAuditTable, err)
	}
	if n == 0 {
		return 0, "", nil
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return 0, "", err
	}
	if guarded {
		if _, err := sp.Exec(ctx, `ALTER TABLE jetsapi.agent_audit DISABLE TRIGGER agent_audit_immutable`); err != nil {
			_ = sp.Rollback(context.Background())
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				return 0, fmt.Sprintf("%s is kept: its append-only guard cannot be lifted by this database user (%s)",
					AgentAuditTable, pgErr.Message), nil
			}
			return 0, "", fmt.Errorf("on %s: %w", AgentAuditTable, err)
		}
	}
	tag, err := sp.Exec(ctx, `DELETE FROM jetsapi.agent_audit WHERE `+old, before)
	if err != nil {
		_ = sp.Rollback(context.Background())
		return 0, "", fmt.Errorf("on %s: %w", AgentAuditTable, err)
	}
	if guarded {
		if _, err := sp.Exec(ctx, `ALTER TABLE jetsapi.agent_audit ENABLE TRIGGER agent_audit_immutable`); err != nil {
			_ = sp.Rollback(context.Background())
			return 0, "", fmt.Errorf("while restoring the append-only guard of %s: %w", AgentAuditTable, err)
		}
	}
	if err := sp.Commit(ctx); err != nil {
		return 0, "", err
	}
	return tag.RowsAffected(), "", nil
}
