package sessionpurge

import (
	"context"
	"fmt"
	"strings"
)

// archiveAllStmt moves every pipeline_execution_status row to the archive, key
// included, and deletes the per-run detail rows of the sessions moved. One
// statement, so a run inserted while it executes is either moved with the rest or
// left untouched in the live table: never deleted without being archived, as an
// INSERT ... SELECT followed by a DELETE could do.
var archiveAllStmt = func() string {
	cols := strings.Join(statusColumns, ", ")
	return fmt.Sprintf(`WITH moved AS (
			DELETE FROM jetsapi.pipeline_execution_status RETURNING %s
		), details AS (
			DELETE FROM jetsapi.pipeline_execution_details
			WHERE session_id IN (SELECT session_id FROM moved)
		), channels AS (
			DELETE FROM jetsapi.pipeline_execution_channel_details
			WHERE session_id IN (SELECT session_id FROM moved)
		)
		INSERT INTO jetsapi.pipeline_execution_status_archive (%s, archive_reason)
		SELECT %s, $1 FROM moved`, cols, cols, cols)
}()

// ArchiveAllRuns is the archive step of a session's deletion applied to every
// session at once, for a reset of the stack's domain data (the apiserver's
// /purgeData reset_domain_tables). Every pipeline_execution_status row is copied,
// key included, to pipeline_execution_status_archive with archive_reason reason,
// and deleted; the pipeline_execution_details and
// pipeline_execution_channel_details rows of those runs' sessions are deleted, as
// Execute deletes them. It returns the number of runs archived.
//
// Unlike Execute it refuses nothing: a reset drops the domain tables under any
// run in flight, so a pending or submitted run is archived with the rest. db is
// the caller's transaction, so the reset's other statements commit with it.
func ArchiveAllRuns(ctx context.Context, db Querier, reason Reason) (int64, error) {
	switch reason {
	case ReasonRetention, ReasonOrphan, ReasonUser:
	default:
		return 0, fmt.Errorf("ArchiveAllRuns: unknown reason %q", reason)
	}
	tag, err := db.Exec(ctx, archiveAllStmt, string(reason))
	if err != nil {
		return 0, fmt.Errorf("while archiving pipeline_execution_status: %w", err)
	}
	return tag.RowsAffected(), nil
}
