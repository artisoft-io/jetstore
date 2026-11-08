package delegate

// The session purge as the lambda runs it, against a real PostgreSQL: skipped
// unless JETS_TEST_DSN names one, and rebuilding the database srg_purge_delegate.
// See jets/sessionpurge/purgetest for how to run it.

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Only the value "false" makes deletions real.
func TestIsDryRun(t *testing.T) {
	for value, want := range map[string]bool{
		"": true, "true": true, "yes": true, "0": true, "False": true, "FALSE": true, "no": true,
		"false": false, " false ": false,
	} {
		if got := IsDryRun(value); got != want {
			t.Errorf("IsDryRun(%q) = %v, want %v", value, got, want)
		}
	}
	t.Setenv("PURGE_DRY_RUN", "")
	if !IsDryRun("") {
		t.Error("unset PURGE_DRY_RUN is not a dry run")
	}
}

func daysAgo(n int) time.Time { return time.Now().AddDate(0, 0, -n) }

// seedPurge writes, with a 30-day retention in mind: an expired session, an
// expired session with a run still submitted, a recent session with a status row
// past 180 days, an orphan, an archived run past 180 days and a retention
// tombstone past 180 days.
func seedPurge(t *testing.T, pool *pgxpool.Pool) {
	purgetest.Seed(t, pool, purgetest.Session{ID: "expired", Client: "c1", Registered: true, LastUpdate: daysAgo(40),
		DomainTables: []string{"DomainA"}, Runs: []string{"completed", "failed"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "busy", Client: "c1", Registered: true, LastUpdate: daysAgo(40),
		Runs: []string{"submitted"}})
	purgetest.Seed(t, pool, purgetest.Session{ID: "recent", Client: "c1", Registered: true, LastUpdate: daysAgo(1),
		DomainTables: []string{"DomainA"}})
	purgetest.Exec(t, pool, `UPDATE jetsapi.pipeline_execution_status SET last_update = now() - interval '200 days'
		WHERE session_id = 'recent'`)
	purgetest.Seed(t, pool, purgetest.Session{ID: "orphan", Client: "c1", LastUpdate: daysAgo(40), DomainTables: []string{"DomainA"}})
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.pipeline_execution_status_archive
		(key, pipeline_config_key, client, process_name, main_object_type, session_id, source_period_key,
		 status, user_email, last_update, archive_reason)
		VALUES (9999, 1, 'c1', 'p', 'o', 'gone', 1, 'completed', 'u', now() - interval '200 days', 'retention')`)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.session_deletion
		(session_id, client, reason, requested_by, status, completed_at)
		VALUES ('gone', 'c1', 'retention', 'purge', 'done', now() - interval '200 days')`)
}

// A dry run changes no row count in any table, whatever it finds.
func TestPurgeSessionsDryRunWritesNothing(t *testing.T) {
	pool := purgetest.DB(t, "srg_purge_delegate")
	seedPurge(t, pool)
	before := purgetest.RowCounts(t, pool)
	now := time.Now()
	err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, DryRun: true,
		Now: now, Deadline: now.Add(runBudget)})
	if err != nil {
		t.Fatal(err)
	}
	if after := purgetest.RowCounts(t, pool); !reflect.DeepEqual(after, before) {
		for k := range after {
			if after[k] != before[k] {
				t.Errorf("dry run changed %s: %d rows, was %d", k, after[k], before[k])
			}
		}
	}
}

// A real run whose deadline has already passed queues its candidates and leaves
// every one pending, with none of its rows deleted.
func TestPurgeSessionsDeadlinePassed(t *testing.T) {
	pool := purgetest.DB(t, "srg_purge_delegate")
	seedPurge(t, pool)
	rows := map[string]map[string]int64{}
	for _, id := range []string{"expired", "busy", "orphan"} {
		rows[id] = purgetest.SessionRows(t, pool, id)
	}
	now := time.Now()
	err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(-time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range rows {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, want) {
			t.Errorf("%s changed: %v, was %v", id, got, want)
		}
		var status, reason string
		if err := pool.QueryRow(context.Background(), `SELECT status, reason FROM jetsapi.session_deletion WHERE session_id = $1`,
			id).Scan(&status, &reason); err != nil || status != "pending" {
			t.Errorf("%s: deletion row %q (%v), want pending", id, status, err)
		}
	}
}

// A real run deletes the expired session and the orphan, refuses the busy one,
// keeps the recent one with its old status row (the age-based status delete is
// gone), and purges the old archived run and tombstone.
func TestPurgeSessionsRealRun(t *testing.T) {
	pool := purgetest.DB(t, "srg_purge_delegate")
	seedPurge(t, pool)
	keep := map[string]map[string]int64{}
	for _, id := range []string{"busy", "recent"} {
		keep[id] = purgetest.SessionRows(t, pool, id)
	}
	now := time.Now()
	if err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, id := range []string{"expired", "orphan"} {
		if left := purgetest.SessionRows(t, pool, id); len(left) > 0 {
			t.Errorf("rows of %s left: %v", id, left)
		}
	}
	for id, want := range keep {
		if got := purgetest.SessionRows(t, pool, id); !reflect.DeepEqual(got, want) {
			t.Errorf("%s changed: %v, was %v", id, got, want)
		}
	}
	for id, want := range map[string]string{"expired": "done retention", "orphan": "done orphan", "busy": "pending retention"} {
		var got string
		_ = pool.QueryRow(ctx, `SELECT status || ' ' || reason FROM jetsapi.session_deletion WHERE session_id = $1`, id).Scan(&got)
		if got != want {
			t.Errorf("%s deletion row %q, want %q", id, got, want)
		}
	}
	var archived, oldArchive, tombstone int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive WHERE session_id IN ('expired', 'orphan')`).Scan(&archived)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.pipeline_execution_status_archive WHERE session_id = 'gone'`).Scan(&oldArchive)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.session_deletion WHERE session_id = 'gone'`).Scan(&tombstone)
	if archived != 3 || oldArchive != 0 || tombstone != 0 {
		t.Errorf("archive holds %d runs of the deleted sessions (want 3) and %d old ones (want 0); %d old tombstones (want 0)",
			archived, oldArchive, tombstone)
	}
}
