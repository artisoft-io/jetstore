// Package sessionpurge is the one path by which a session is deleted from the
// JetStore database.
//
// A deletion is a row of jetsapi.session_deletion. Queue writes the rows, from
// the purge's candidates (RetentionCandidates, OrphanCandidates) or from a user;
// Execute works through them, one session per transaction:
//
//   - a session with a run on it, or reading it, that is pending or submitted is
//     refused and stays queued;
//   - otherwise its rows are deleted from every domain table registered in
//     input_registry, from the fixed jetsapi tables and from the pipeline
//     coordinator tables; its pipeline_execution_status rows are copied, key
//     included, to pipeline_execution_status_archive and deleted; its
//     session_registry row is deleted last; and its deletion row is marked done;
//   - any error rolls the whole session back, so no session is half deleted, and
//     marks its deletion row failed with the error, in a transaction of its own.
//     A failed row is retried by the next Execute.
//
// Table names are quoted as identifiers and session ids are bound, so no value
// is built into a statement. A registered table that does not exist, or has no
// session_id column, is skipped and logged; it is not an error.
//
// ExecuteRequests is the dry run: the same work on requests held in memory, every
// transaction rolled back. PurgeArchive and PurgeTombstones remove archived runs
// and done deletion rows by age.
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

// Reason says why a session is deleted.
type Reason string

const (
	// ReasonRetention: the session is older than the site's retention.
	ReasonRetention Reason = "retention"
	// ReasonOrphan: rows carry the session id and the session was never
	// registered, or its registration is gone.
	ReasonOrphan Reason = "orphan"
	// ReasonUser: a user deleted the session.
	ReasonUser Reason = "user"
)

// RequestedByPurge is the requested_by of the purge's own requests.
const RequestedByPurge = "purge"

// The status of a deletion row.
const (
	StatusPending = "pending"
	StatusDone    = "done"
	StatusFailed  = "failed"
)

// Request is one session to delete.
type Request struct {
	SessionID, Client, SourceType string
	MonthPeriod                   *int
	Reason                        Reason
	RequestedBy                   string
}

func (r *Request) validate() error {
	switch {
	case r.SessionID == "":
		return errors.New("a deletion request needs a session id")
	case r.Client == "":
		return fmt.Errorf("deletion request for session %s: no client", r.SessionID)
	case r.RequestedBy == "":
		return fmt.Errorf("deletion request for session %s: no requested_by", r.SessionID)
	}
	switch r.Reason {
	case ReasonRetention, ReasonOrphan, ReasonUser:
		return nil
	}
	return fmt.Errorf("deletion request for session %s: unknown reason %q", r.SessionID, r.Reason)
}

// merge applies Queue's rule to reqs in memory: the first request for a session
// wins, except that a user request replaces one that is not. A replaced request
// moves to the end, as Queue's new requested_at moves it. queued are rows already
// in session_deletion and not done, in the order Execute reads them.
func merge(queued, reqs []Request) []Request {
	out := make([]Request, 0, len(queued)+len(reqs))
	at := make(map[string]int, len(queued)+len(reqs))
	add := func(r Request) {
		if i, ok := at[r.SessionID]; ok {
			if r.Reason != ReasonUser || out[i].Reason == ReasonUser {
				return
			}
			out[i].SessionID = "" // replaced; dropped below
		}
		at[r.SessionID] = len(out)
		out = append(out, r)
	}
	for _, r := range queued {
		add(r)
	}
	for _, r := range reqs {
		add(r)
	}
	kept := out[:0]
	for _, r := range out {
		if r.SessionID != "" {
			kept = append(kept, r)
		}
	}
	return kept
}

// Queue inserts pending rows. A session already queued keeps its row, except that
// a ReasonUser request replaces a retention or orphan one that is not yet done.
// It returns the number of rows inserted or replaced. db is a pool, or a
// transaction of the caller's: delete_sessions queues inside the transaction that
// holds the sessions' locks.
func Queue(ctx context.Context, db Querier, reqs []Request) (int, error) {
	if len(reqs) == 0 {
		return 0, nil
	}
	for i := range reqs {
		if err := reqs[i].validate(); err != nil {
			return 0, err
		}
	}
	// One statement cannot update the same row twice, so apply the rule within
	// reqs first.
	reqs = merge(nil, reqs)
	n := len(reqs)
	ids, clients, reasons, by := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	sourceTypes, months := make([]*string, n), make([]*int, n)
	for i, r := range reqs {
		ids[i], clients[i], reasons[i], by[i] = r.SessionID, r.Client, string(r.Reason), r.RequestedBy
		if r.SourceType != "" {
			st := r.SourceType
			sourceTypes[i] = &st
		}
		months[i] = r.MonthPeriod
	}
	tag, err := db.Exec(ctx, `INSERT INTO jetsapi.session_deletion
			(session_id, client, source_type, month_period, reason, requested_by, status)
		SELECT u.session_id, u.client, u.source_type, u.month_period, u.reason, u.requested_by, 'pending'
		FROM unnest($1::text[], $2::text[], $3::text[], $4::int[], $5::text[], $6::text[])
			AS u(session_id, client, source_type, month_period, reason, requested_by)
		ON CONFLICT (session_id) DO UPDATE SET
			client = EXCLUDED.client,
			source_type = COALESCE(EXCLUDED.source_type, session_deletion.source_type),
			month_period = COALESCE(EXCLUDED.month_period, session_deletion.month_period),
			reason = EXCLUDED.reason,
			requested_by = EXCLUDED.requested_by,
			requested_at = now(),
			status = 'pending',
			completed_at = NULL,
			error = NULL
		WHERE EXCLUDED.reason = 'user'
			AND session_deletion.reason <> 'user'
			AND session_deletion.status <> 'done'`,
		ids, clients, sourceTypes, months, reasons, by)
	if err != nil {
		return 0, fmt.Errorf("while queueing %d session deletions: %w", n, err)
	}
	return int(tag.RowsAffected()), nil
}

// RetentionCandidates reads, without writing, the sessions whose
// session_registry.last_update is on or before before, by session id.
func RetentionCandidates(ctx context.Context, db *pgxpool.Pool, before time.Time) ([]Request, error) {
	// before is cast to timestamptz because last_update is a timestamp without
	// time zone, and pgx would otherwise encode it by its wall clock in its own
	// location.
	return readRequests(ctx, db, `SELECT session_id, client, source_type, month_period, $2::text, $3::text
		FROM jetsapi.session_registry
		WHERE last_update <= $1::timestamptz
		ORDER BY session_id`, before, ReasonRetention, RequestedByPurge)
}

// ClientUnknown is the client of an orphan found only in tables that carry no
// client, such as session_reservation and the coordinator tables.
const ClientUnknown = "(unknown)"

// orphanSourceTables are the jetsapi tables OrphanCandidates reads session ids
// from: every table the executor deletes from by session_id, session_registry
// aside. The order ranks them as sources of an orphan's client and source period:
// input_registry first, then pipeline_execution_status, then the rest.
func orphanSourceTables() []string {
	out := []string{"input_registry", "pipeline_execution_status"}
	for _, t := range fixedTables {
		if t != "input_registry" {
			out = append(out, t)
		}
	}
	return append(out, "pipeline_coordinator_map_items", "pipeline_coordinator_lock")
}

// OrphanCandidates reads, without writing, the session ids found in any jetsapi
// table the executor deletes from (orphanSourceTables) that are in neither
// session_registry nor session_deletion, and whose age is on or before before, by
// session id.
//
// A session's age is the latest last_update among its rows in the tables that
// have one. Failing that, when the id is all digits, it is the time the id
// encodes: session ids are epoch milliseconds when reserved. An id that is not
// numeric and appears only in tables with no last_update cannot be dated, is never
// a candidate, and is counted in the log.
//
// The client and month period are taken from the highest-ranked table that has
// them (input_registry, then pipeline_execution_status, then the rest), and the
// source type from input_registry. An orphan seen only where no table carries a
// client gets ClientUnknown.
func OrphanCandidates(ctx context.Context, db *pgxpool.Pool, before time.Time) ([]Request, error) {
	sources := orphanSourceTables()
	rows, err := db.Query(ctx, `SELECT table_name, array_agg(column_name::text)
		FROM information_schema.columns
		WHERE table_schema = 'jetsapi' AND table_name = ANY($1::text[])
		GROUP BY table_name`, sources)
	if err != nil {
		return nil, fmt.Errorf("while reading the columns of the session tables: %w", err)
	}
	columns := map[string]map[string]bool{}
	for rows.Next() {
		var table string
		var cols []string
		if err := rows.Scan(&table, &cols); err != nil {
			rows.Close()
			return nil, err
		}
		columns[table] = map[string]bool{}
		for _, c := range cols {
			columns[table][c] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// One aggregate per table, so a large table is reduced to its sessions before
	// the union.
	var parts []string
	for rank, table := range sources {
		cols := columns[table]
		if !cols["session_id"] {
			continue // not migrated yet, or no session_id: nothing to read
		}
		pick := func(col, agg, null string) string {
			if cols[col] {
				return agg + "(" + col + ")"
			}
			return null
		}
		sourceType := "NULL::text"
		if table == "input_registry" {
			sourceType = `CASE WHEN bool_or(source_type = 'file') THEN 'file'
				WHEN bool_or(source_type = 'domain_table') THEN 'domain_table' ELSE min(source_type) END`
		}
		parts = append(parts, fmt.Sprintf(`SELECT session_id, %s AS last_update, %s AS client, %s AS source_type,
				%s AS source_period_key, %d AS rank
			FROM %s GROUP BY session_id`,
			pick("last_update", "max", "NULL::timestamp"), pick("client", "min", "NULL::text"), sourceType,
			pick("source_period_key", "min", "NULL::int"), rank, pgx.Identifier{"jetsapi", table}.Sanitize()))
	}
	if len(parts) == 0 {
		return nil, nil
	}
	stmt := `WITH s AS (` + strings.Join(parts, "\n\t\tUNION ALL\n\t\t") + `
		), ids AS (
			SELECT session_id,
				(array_agg(client ORDER BY rank) FILTER (WHERE client IS NOT NULL))[1] AS client,
				min(source_type) AS source_type,
				(array_agg(source_period_key ORDER BY rank) FILTER (WHERE source_period_key IS NOT NULL))[1] AS source_period_key,
				max(last_update) AS last_update
			FROM s
			GROUP BY session_id
		), dated AS (
			SELECT ids.*, COALESCE(ids.last_update::timestamptz,
				CASE WHEN ids.session_id ~ '^[0-9]{1,18}$' THEN to_timestamp(ids.session_id::bigint / 1000.0) END) AS aged_at
			FROM ids
			WHERE NOT EXISTS (SELECT 1 FROM jetsapi.session_registry r WHERE r.session_id = ids.session_id)
				AND NOT EXISTS (SELECT 1 FROM jetsapi.session_deletion d WHERE d.session_id = ids.session_id)
		)
		SELECT dated.session_id, dated.client, dated.source_type, sp.month_period, dated.aged_at IS NULL
		FROM dated LEFT JOIN jetsapi.source_period sp ON sp.key = dated.source_period_key
		WHERE dated.aged_at IS NULL OR dated.aged_at <= $1::timestamptz
		ORDER BY dated.session_id`
	rows, err = db.Query(ctx, stmt, before)
	if err != nil {
		return nil, fmt.Errorf("while reading the orphan candidates: %w", err)
	}
	defer rows.Close()
	var out []Request
	undated := 0
	for rows.Next() {
		var client, sourceType *string
		var r Request
		var noAge bool
		if err := rows.Scan(&r.SessionID, &client, &sourceType, &r.MonthPeriod, &noAge); err != nil {
			return nil, err
		}
		if noAge {
			undated++
			continue
		}
		r.Client = ClientUnknown
		if client != nil && *client != "" {
			r.Client = *client
		}
		if sourceType != nil {
			r.SourceType = *sourceType
		}
		r.Reason, r.RequestedBy = ReasonOrphan, RequestedByPurge
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if undated > 0 {
		log.Printf("sessionpurge: %d unregistered session ids are not numeric and appear only in tables with no"+
			" last_update; they cannot be dated and are not orphan candidates", undated)
	}
	return out, nil
}

// queuedRequests reads the deletion rows Execute works through: pending and
// failed, oldest first, and only those with one of reasons when it is not empty.
func queuedRequests(ctx context.Context, db *pgxpool.Pool, reasons []Reason) ([]Request, error) {
	// Never nil: pgx sends a nil slice as NULL, and cardinality(NULL) is NULL.
	only := make([]string, 0, len(reasons))
	for _, r := range reasons {
		switch r {
		case ReasonRetention, ReasonOrphan, ReasonUser:
			only = append(only, string(r))
		default:
			return nil, fmt.Errorf("unknown deletion reason %q in Options.Reasons", r)
		}
	}
	return readRequests(ctx, db, `SELECT session_id, client, source_type, month_period, reason, requested_by
		FROM jetsapi.session_deletion
		WHERE status IN ('pending', 'failed')
			AND (cardinality($1::text[]) = 0 OR reason = ANY($1::text[]))
		ORDER BY requested_at, session_id`, only)
}

func readRequests(ctx context.Context, db *pgxpool.Pool, stmt string, args ...any) ([]Request, error) {
	rows, err := db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Request
	for rows.Next() {
		var r Request
		var sourceType *string
		var reason string
		if err := rows.Scan(&r.SessionID, &r.Client, &sourceType, &r.MonthPeriod, &reason, &r.RequestedBy); err != nil {
			return nil, err
		}
		if sourceType != nil {
			r.SourceType = *sourceType
		}
		r.Reason = Reason(reason)
		out = append(out, r)
	}
	return out, rows.Err()
}

// PurgeArchive deletes archived runs whose own last_update is on or before
// before, so an archived run is kept as long as it would have been in the live
// table, whenever its session was deleted.
func PurgeArchive(ctx context.Context, db *pgxpool.Pool, before time.Time) (int64, error) {
	tag, err := db.Exec(ctx, `DELETE FROM jetsapi.pipeline_execution_status_archive
		WHERE last_update <= $1::timestamptz`, before)
	if err != nil {
		return 0, fmt.Errorf("while purging pipeline_execution_status_archive: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PurgeTombstones deletes done deletion rows: retention and orphan ones completed
// on or before ageOut, user ones completed on or before userAgeOut. A user
// deletion is kept longer because a lookback read skips the sessions a user
// deleted. Pending and failed rows are never removed here.
func PurgeTombstones(ctx context.Context, db *pgxpool.Pool, ageOut, userAgeOut time.Time) (int64, error) {
	tag, err := db.Exec(ctx, `DELETE FROM jetsapi.session_deletion
		WHERE status = 'done' AND (
			(reason IN ('retention', 'orphan') AND completed_at <= $1::timestamptz)
			OR (reason = 'user' AND completed_at <= $2::timestamptz))`, ageOut, userAgeOut)
	if err != nil {
		return 0, fmt.Errorf("while purging session_deletion: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Summary is the report in a few lines, for a log.
func (r *Report) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d sessions deleted", r.Done)
	if len(r.ByReason) > 0 {
		b.WriteString(" (")
		first := true
		for _, reason := range []Reason{ReasonRetention, ReasonOrphan, ReasonUser} {
			if n := r.ByReason[reason]; n > 0 {
				if !first {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s %d", reason, n)
				first = false
			}
		}
		b.WriteString(")")
	}
	fmt.Fprintf(&b, ", %d refused (a run pending or submitted), %d failed, %d not reached before the deadline, %d held by another executor",
		r.Refused, r.Failed, r.NotReached, r.Skipped)
	fmt.Fprintf(&b, "; %d pipeline_execution_status rows archived", r.Rows[ArchiveTable])
	return b.String()
}
