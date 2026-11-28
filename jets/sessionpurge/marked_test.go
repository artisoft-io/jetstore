package sessionpurge

// A deletion marks a session's input_registry rows deleted and keeps them, and
// PurgeMarkedRegistrations deletes them once no run refers to them. Against a
// real PostgreSQL, skipped unless JETS_TEST_DSN names one.

import (
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/inputregistry"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// registration writes one input_registry row of session id and returns its key.
func registration(t *testing.T, pool *pgxpool.Pool, id, table string) int {
	t.Helper()
	var key int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
		(client, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ('c1', 'ObjectA', 'k', 1, $1, 'file', $2, 'system') RETURNING key`, table, id).Scan(&key); err != nil {
		t.Fatal(err)
	}
	return key
}

func liveCount(t *testing.T, pool *pgxpool.Pool, id string) int64 {
	t.Helper()
	return count(t, pool, `SELECT count(*) FROM jetsapi.input_registry_live WHERE session_id = $1`, id)
}

// A deleted session's registrations are marked, not deleted: the rows stay in
// the table with deleted_at set, leave the live view, and the report counts them
// as marked.
func TestDeletionMarksRegistrations(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA", "DomainB"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "s2", Client: "c1", Registered: true, DomainTables: []string{"DomainA"}})
	queue(t, pool, retention("s1"))
	report, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Marked[registryTable] != 2 || report.Rows[registryTable] != 0 {
		t.Errorf("report marks %d and deletes %d input_registry rows, want 2 marked and none deleted",
			report.Marked[registryTable], report.Rows[registryTable])
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.input_registry WHERE session_id = 's1' AND deleted_at IS NOT NULL`); n != 2 {
		t.Errorf("%d rows of s1 marked in the table, want 2", n)
	}
	if n := liveCount(t, pool, "s1"); n != 0 {
		t.Errorf("%d rows of s1 in input_registry_live, want none", n)
	}
	if n := liveCount(t, pool, "s2"); n != 1 {
		t.Errorf("%d rows of s2 in input_registry_live, want its 1", n)
	}
	// Deleting the session again (a user's request after the purge's) marks
	// nothing more and keeps the first deleted_at.
	var first time.Time
	if err := pool.QueryRow(ctx, `SELECT min(deleted_at) FROM jetsapi.input_registry WHERE session_id = 's1'`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	purgetest.Exec(t, pool, `DELETE FROM jetsapi.session_deletion WHERE session_id = 's1'`)
	queue(t, pool, Request{SessionID: "s1", Client: "c1", Reason: ReasonUser, RequestedBy: "u"})
	report, err = Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Marked[registryTable] != 0 {
		t.Errorf("second deletion %+v, want done with nothing marked", report)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.input_registry WHERE session_id = 's1' AND deleted_at = $1`, first); n != 2 {
		t.Errorf("%d rows keep their first deleted_at, want 2", n)
	}
}

// A session whose registrations are all marked is neither a retention nor an
// orphan candidate once its session_deletion tombstone is gone, so it is not
// queued, counted or deleted again; a dry run does not count it either.
func TestMarkedSessionNotSelectedAgain(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "1700000000000", Client: "c1", Registered: true,
		LastUpdate: days(200), DomainTables: []string{"DomainA"}})
	queue(t, pool, retention("1700000000000"))
	if _, err := Execute(ctx, pool, Options{}); err != nil {
		t.Fatal(err)
	}
	if n := purgetest.MarkedRows(t, pool, "1700000000000"); n != 1 {
		t.Fatalf("%d rows marked, want 1", n)
	}
	// The tombstone ages out.
	if _, err := PurgeTombstones(ctx, pool, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if deletion(t, pool, "1700000000000") != nil {
		t.Fatal("the tombstone is still there")
	}
	retained, err := RetentionCandidates(ctx, pool, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	orphans, err := OrphanCandidates(ctx, pool, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(retained) != 0 || len(orphans) != 0 {
		t.Errorf("candidates %v and orphans %v, want none: the session's only rows are marked", retained, orphans)
	}
	report, err := ExecuteRequests(ctx, pool, append(retained, orphans...), Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 0 || len(report.Marked) != 0 {
		t.Errorf("dry run %+v, want nothing", report)
	}
	// A live registration of an unregistered session is still an orphan.
	registration(t, pool, "1700000000001", "DomainA")
	orphans, err = OrphanCandidates(ctx, pool, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 1 || orphans[0].SessionID != "1700000000001" {
		t.Errorf("orphans %v, want the live one", orphans)
	}
}

// A dry run counts the rows it would mark and marks none.
func TestDryRunMarksNothing(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA", "DomainB"}})
	report, err := ExecuteRequests(ctx, pool, []Request{retention("s1")}, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Marked[registryTable] != 2 {
		t.Errorf("dry run %+v, want s1 done with 2 rows marked", report)
	}
	if n := purgetest.MarkedRows(t, pool, "s1"); n != 0 {
		t.Errorf("%d rows marked by a dry run", n)
	}
	if n := liveCount(t, pool, "s1"); n != 2 {
		t.Errorf("%d live rows of s1 after a dry run, want 2", n)
	}
}

// dropMarking makes the database one the migration has not reached: no view,
// no deleted_at. The next test rebuilds the database.
func dropMarking(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	purgetest.Exec(t, pool, `DROP VIEW jetsapi.input_registry_live; ALTER TABLE jetsapi.input_registry DROP COLUMN deleted_at`)
	inputregistry.Forget()
	t.Cleanup(inputregistry.Forget)
}

// On a database without deleted_at the session's registrations are deleted as
// they were before, the orphans are read from the table, and the purge of
// marked rows does nothing.
func TestUnmigratedDatabaseDeletes(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, DomainTables: []string{"DomainA", "DomainB"}})
	dropMarking(t, pool)
	if rel, err := inputregistry.LiveRelation(ctx, pool); err != nil || rel != inputregistry.Table {
		t.Fatalf("live relation %q, %v; want the table", rel, err)
	}
	report, err := ExecuteRequests(ctx, pool, []Request{retention("s1")}, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[registryTable] != 2 || len(report.Marked) != 0 {
		t.Errorf("dry run deletes %d and marks %v, want 2 deleted", report.Rows[registryTable], report.Marked)
	}
	queue(t, pool, retention("s1"))
	report, err = Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Done != 1 || report.Rows[registryTable] != 2 || len(report.Marked) != 0 {
		t.Errorf("report %+v, want s1 done with its 2 registrations deleted", report)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.input_registry WHERE session_id = 's1'`); n != 0 {
		t.Errorf("%d registrations of s1 left", n)
	}
	registration(t, pool, "1700000000001", "DomainA")
	orphans, err := OrphanCandidates(ctx, pool, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 1 {
		t.Errorf("orphans %v, want the one registration's session", orphans)
	}
	n, exists, err := PurgeMarkedRegistrations(ctx, pool)
	if err != nil || exists || n != 0 {
		t.Errorf("purge of marked rows: %d, %v, %v; want nothing, no column", n, exists, err)
	}
}

// A marked registration goes once no run, live or archived, refers to it by
// its session (input_session_id), its key (main_input_registry_key) or among
// merged_input_registry_keys. A live registration never goes.
func TestPurgeMarkedRegistrations(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	mark := func(keys ...int) {
		for _, k := range keys {
			purgetest.Exec(t, pool, `UPDATE jetsapi.input_registry SET deleted_at = now() WHERE key = $1`, k)
		}
	}
	run := func(table, inputSession string, mainKey *int, merged []int, lastUpdate time.Time) {
		if merged == nil {
			merged = []int{}
		}
		archive := ""
		if table == ArchiveTable {
			archive = ", 999 + (SELECT count(*) FROM jetsapi.pipeline_execution_status_archive), 'retention'"
		}
		cols := "pipeline_config_key, client, process_name, main_object_type, input_session_id, main_input_registry_key, " +
			"merged_input_registry_keys, session_id, source_period_key, status, user_email, last_update"
		if archive != "" {
			cols += ", key, archive_reason"
		}
		purgetest.Exec(t, pool, `INSERT INTO `+table+` (`+cols+`)
			VALUES (1, 'c1', 'proc', 'ObjectA', $1, $2, $3, 'run-'||$1, 1, 'completed', 'u', $4::timestamptz`+archive+`)`,
			inputSession, mainKey, merged, lastUpdate)
	}
	live := "jetsapi.pipeline_execution_status"
	bySession := registration(t, pool, "by-session", "T")
	byMain := registration(t, pool, "by-main", "T")
	byMerged := registration(t, pool, "by-merged", "T")
	byArchive := registration(t, pool, "by-archive", "T")
	byArchiveMerged := registration(t, pool, "by-archive-merged", "T")
	unreferenced := registration(t, pool, "unreferenced", "T")
	liveUnreferenced := registration(t, pool, "live-unreferenced", "T")
	liveReferenced := registration(t, pool, "live-referenced", "T")
	mark(bySession, byMain, byMerged, byArchive, byArchiveMerged, unreferenced)

	run(live, "by-session", nil, nil, time.Now())
	run(live, "other", &byMain, nil, time.Now())
	run(live, "other2", nil, []int{12345, byMerged}, time.Now())
	run(live, "live-referenced", &liveReferenced, nil, time.Now())
	run(ArchiveTable, "other3", &byArchive, nil, days(200))
	run(ArchiveTable, "other4", nil, []int{byArchiveMerged}, days(10))

	keys := func() map[int]bool {
		out := map[int]bool{}
		rows, err := pool.Query(ctx, `SELECT key FROM jetsapi.input_registry`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var k int
			_ = rows.Scan(&k)
			out[k] = true
		}
		return out
	}

	// A dry run: the caller's transaction, rolled back.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n, exists, err := PurgeMarkedRegistrations(ctx, tx)
	if err != nil || !exists || n != 1 {
		t.Errorf("dry run purges %d, %v, %v; want the 1 unreferenced", n, exists, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if !keys()[unreferenced] {
		t.Error("the dry run's purge was not rolled back")
	}

	n, _, err = PurgeMarkedRegistrations(ctx, pool)
	if err != nil || n != 1 {
		t.Fatalf("purged %d, %v; want 1", n, err)
	}
	got := keys()
	for name, k := range map[string]int{"by-session": bySession, "by-main": byMain, "by-merged": byMerged,
		"by-archive": byArchive, "by-archive-merged": byArchiveMerged, "live-unreferenced": liveUnreferenced,
		"live-referenced": liveReferenced} {
		if !got[k] {
			t.Errorf("registration %s (%d) was purged", name, k)
		}
	}
	if got[unreferenced] {
		t.Error("the unreferenced marked registration is still there")
	}

	// The archive's purge frees the registration only its purged run referred to.
	if _, err := PurgeArchive(ctx, pool, days(180)); err != nil {
		t.Fatal(err)
	}
	if n, _, err = PurgeMarkedRegistrations(ctx, pool); err != nil || n != 1 {
		t.Fatalf("after the archive purge, purged %d, %v; want 1", n, err)
	}
	got = keys()
	if got[byArchive] || !got[byArchiveMerged] || !got[liveUnreferenced] {
		t.Errorf("after the archive purge: by-archive kept %v, by-archive-merged kept %v, live kept %v; "+
			"want false, true, true", got[byArchive], got[byArchiveMerged], got[liveUnreferenced])
	}
}
