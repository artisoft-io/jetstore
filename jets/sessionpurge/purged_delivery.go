package sessionpurge

// The record of the deliveries a deletion removes.
//
// A file's delivery is its first registration: the input_registry row of its
// file key, with source_type 'file', that arrived first, a registration's
// arrival being its session id read as Unix milliseconds (then its key). A
// later registration of the same file key is re-processing, and the feed
// detectors never judge it (jets/agentic/observe, feed_delivery_registrations.go).
//
// Deleting a session deleted its registrations until they were marked deleted
// instead (input_registry.deleted_at), so when it held a file's first
// registration a later registration of the same file would become the first
// that survives, and be read as the file's delivery: a file sent again would
// be judged as new, against an arrival long after its data. So before a
// session's registrations go, every one of them that is its file's first
// registration is recorded in jetsapi.purged_delivery: the table, the file key
// and the session, which is the arrival. The feed detectors read the record
// and keep any later registration of the file as re-processing.
//
// Both deletion paths come here, the retention purge and a user's Delete, so
// both leave the record.
//
// Since registrations are marked rather than deleted, a marked first
// registration is itself the evidence the record holds, and the feed reading
// (jets/agentic/observe, FeedRegistrations) reads it as such. The record is kept
// for what the marks cannot cover: the first registrations deleted before the
// column existed, those deleted by a release that does not mark while another
// that does is deployed beside it, and a database without the column. It is truncated with input_registry by a reset of the
// domain tables.
//
// A row is kept for twice RETENTION_DAYS from first_arrival, the file's arrival
// as the feed detectors read it: the retention purge deletes the older ones
// (PurgeDeliveriesBefore, called by jets/purge_database). The age is counted
// from first_arrival and not from purged_at, which the ON CONFLICT update below
// rewrites. The first registration itself is deleted about RETENTION_DAYS after
// the arrival, so the record outlives it by about one retention period. Once the
// record goes, the file's earliest surviving registration, if any, is read as
// its delivery, so a file registered again more than twice RETENTION_DAYS after
// its first delivery is judged a new delivery. Without a bound the table grew by
// one row per purged file for the life of the installation, about 0.5 KB a row
// with its unique index (measured on PostgreSQL 16: 507 MB for a million rows
// with file keys of about 120 characters).

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const purgedDeliveryExistsStmt = `SELECT to_regclass('jetsapi.purged_delivery') IS NOT NULL`

// recordPurgedDeliveryStmt records the session's registrations that are their
// file's first: no registration of the same table and file key arrived before
// it, among the registrations still held, marked deleted or not (a deletion
// marks a session's registrations where input_registry has deleted_at, so an
// earlier one deleted since is still read here, and was recorded when it was
// deleted). A file whose earlier first
// registration is already recorded keeps the earlier one. A registration whose
// session id is not a time has no arrival and is nobody's delivery, so it is
// neither recorded nor counted as an earlier one.
const recordPurgedDeliveryStmt = `INSERT INTO jetsapi.purged_delivery
		(table_name, file_key, first_session_id, first_arrival, deletion_reason)
	SELECT ir.table_name, ir.file_key, ir.session_id,
		'epoch'::timestamptz + ir.session_id::bigint * interval '1 millisecond', $2
	FROM jetsapi.input_registry ir
	WHERE ir.session_id = $1 AND ir.source_type = 'file' AND COALESCE(ir.file_key, '') <> ''
		AND ir.session_id ~ '^[0-9]{1,18}$'
		AND NOT EXISTS (SELECT 1 FROM jetsapi.input_registry e
			WHERE e.table_name = ir.table_name AND e.file_key = ir.file_key AND e.source_type = 'file'
				AND e.session_id ~ '^[0-9]{1,18}$'
				AND (e.session_id::bigint, e.key) < (ir.session_id::bigint, ir.key))
	ON CONFLICT (table_name, file_key) DO UPDATE
		SET first_session_id = EXCLUDED.first_session_id, first_arrival = EXCLUDED.first_arrival,
			deletion_reason = EXCLUDED.deletion_reason, purged_at = now()
		WHERE EXCLUDED.first_arrival < jetsapi.purged_delivery.first_arrival`

// recordPurgedDeliveries records, in the deletion's transaction, the first
// registrations the session holds, and returns how many it recorded
// (Report.PurgedDeliveries). Where the table does not exist, as on a database
// not yet migrated, nothing is recorded.
func recordPurgedDeliveries(ctx context.Context, tx pgx.Tx, r *Request) (int64, error) {
	var exists bool
	if err := tx.QueryRow(ctx, purgedDeliveryExistsStmt).Scan(&exists); err != nil {
		return 0, fmt.Errorf("while looking for jetsapi.purged_delivery: %w", err)
	}
	if !exists {
		return 0, nil
	}
	tag, err := tx.Exec(ctx, recordPurgedDeliveryStmt, r.SessionID, string(r.Reason))
	if err != nil {
		return 0, fmt.Errorf("on jetsapi.purged_delivery: %w", err)
	}
	return tag.RowsAffected(), nil
}

// purgeOldDeliveriesStmt deletes the records of files that first arrived before
// $1. first_arrival is a timestamptz, so $1 is compared as an instant.
const purgeOldDeliveriesStmt = `DELETE FROM jetsapi.purged_delivery WHERE first_arrival < $1::timestamptz`

// PurgeDeliveriesBefore deletes, in tx, the jetsapi.purged_delivery rows whose
// file first arrived before before, and returns how many it deleted. exists is
// false, and nothing is deleted, where the table does not exist, as on a
// database not yet migrated. The caller sets before (twice RETENTION_DAYS
// before the run's start) and decides whether tx commits: a dry run rolls it
// back and reports the count.
func PurgeDeliveriesBefore(ctx context.Context, tx pgx.Tx, before time.Time) (n int64, exists bool, err error) {
	if err = tx.QueryRow(ctx, purgedDeliveryExistsStmt).Scan(&exists); err != nil {
		return 0, false, fmt.Errorf("while looking for jetsapi.purged_delivery: %w", err)
	}
	if !exists {
		return 0, false, nil
	}
	tag, err := tx.Exec(ctx, purgeOldDeliveriesStmt, before)
	if err != nil {
		return 0, true, fmt.Errorf("while purging jetsapi.purged_delivery: %w", err)
	}
	return tag.RowsAffected(), true, nil
}
