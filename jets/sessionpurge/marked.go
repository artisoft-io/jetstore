package sessionpurge

// The end of a marked registration.
//
// A session's deletion marks its jetsapi.input_registry rows deleted rather than
// deleting them (deleteRows), so that a run that read the session still resolves
// its input: the schema provider of its main input, its source for the run
// conditions and the step baselines, and the arrival of a file it re-processed.
// What refers to a registration is a run: its input_session_id names the
// registration's session, its main_input_registry_key the registration's key, or
// its merged_input_registry_keys hold it. Runs are live in
// pipeline_execution_status until their own session is deleted, then archived in
// pipeline_execution_status_archive until PurgeArchive deletes them.
//
// PurgeMarkedRegistrations deletes a marked registration once no run, live or
// archived, refers to it, so the table does not grow for ever. A live
// registration is never touched.
//
// It cannot race a run starting from the registration. A run is started from a
// live registration only: the start reads jetsapi.input_registry_live, and a
// resubmission is refused when its main input is not live (CheckResubmittable in
// jets/datatable). A registration is marked only when its session's deletion
// commits, under the session lock a start takes before it inserts its status
// row (GuardInputSession), and the start then finds the session's
// session_deletion row and is refused. So no run inserted after a registration
// was marked refers to it, and the one statement below, whose snapshot sees every
// run committed before it, keeps every registration a run refers to.

import (
	"context"
	"fmt"

	"github.com/artisoft-io/jetstore/jets/inputregistry"
)

// purgeMarkedSQL deletes the marked registrations no run refers to, %s being
// the archive's half of the runs' union, or nothing where the archive does not
// exist.
const purgeMarkedSQL = `WITH runs AS (
		SELECT input_session_id, main_input_registry_key, merged_input_registry_keys
		FROM jetsapi.pipeline_execution_status%s
	), sessions AS (
		SELECT DISTINCT input_session_id AS session_id FROM runs WHERE input_session_id IS NOT NULL
	), keys AS (
		SELECT main_input_registry_key AS key FROM runs WHERE main_input_registry_key IS NOT NULL
		UNION
		SELECT unnest(merged_input_registry_keys) FROM runs
	)
	DELETE FROM jetsapi.input_registry ir
	WHERE ir.deleted_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM sessions s WHERE s.session_id = ir.session_id)
		AND NOT EXISTS (SELECT 1 FROM keys k WHERE k.key = ir.key)`

const purgeMarkedArchiveSQL = `
		UNION ALL
		SELECT input_session_id, main_input_registry_key, merged_input_registry_keys
		FROM jetsapi.pipeline_execution_status_archive`

const archiveExistsSQL = `SELECT to_regclass('jetsapi.pipeline_execution_status_archive') IS NOT NULL`

// PurgeMarkedRegistrations deletes, in db, the input_registry rows marked
// deleted that no run refers to: no row of pipeline_execution_status or
// pipeline_execution_status_archive names the row's session_id in
// input_session_id, or its key in main_input_registry_key or in
// merged_input_registry_keys. It returns how many it deleted. exists is false,
// and nothing is deleted, where input_registry has no deleted_at column, as on a
// database not yet migrated, where nothing is marked. db is a pool or the
// caller's transaction: a dry run passes one it rolls back.
func PurgeMarkedRegistrations(ctx context.Context, db Querier) (n int64, exists bool, err error) {
	if exists, err = inputregistry.HasDeletedAt(ctx, db); err != nil || !exists {
		return 0, exists, err
	}
	var archive bool
	if err = db.QueryRow(ctx, archiveExistsSQL).Scan(&archive); err != nil {
		return 0, true, fmt.Errorf("while looking for jetsapi.pipeline_execution_status_archive: %w", err)
	}
	union := ""
	if archive {
		union = purgeMarkedArchiveSQL
	}
	tag, err := db.Exec(ctx, fmt.Sprintf(purgeMarkedSQL, union))
	if err != nil {
		return 0, true, fmt.Errorf("while purging the marked rows of jetsapi.input_registry: %w", err)
	}
	return tag.RowsAffected(), true, nil
}
