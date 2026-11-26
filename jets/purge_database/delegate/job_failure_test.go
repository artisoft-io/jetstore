package delegate

// The purge records its own failures as a job's failure
// (datatable.RecordJobFailure): one row per run that went wrong, under a session
// of its own. Against a real PostgreSQL, skipped unless JETS_TEST_DSN names one;
// see jets/sessionpurge/purgetest.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/datatable"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const jobFailureDB = "br_purge_job_failure"

// jobFailureTestDB is purgetest.DB with the source_period key sequence moved
// past the row purgetest writes with an explicit key, so the record can add
// today's period.
func jobFailureTestDB(t *testing.T) *pgxpool.Pool {
	pool := purgetest.DB(t, jobFailureDB)
	purgetest.Exec(t, pool, `SELECT setval(pg_get_serial_sequence('jetsapi.source_period', 'key'), 100)`)
	return pool
}

// purgeFailure is a recorded failure of the purge job.
type purgeFailure struct {
	session, details, sourceType, class, source string
	configKey                                   int
}

func purgeFailures(t *testing.T, pool *pgxpool.Pool) []purgeFailure {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT pe.session_id, pe.failure_details, coalesce(sr.source_type, ''),
			pe.failure_class, pe.failure_source, pe.pipeline_config_key
		FROM jetsapi.pipeline_execution_status pe
			LEFT JOIN jetsapi.session_registry sr ON sr.session_id = pe.session_id
		WHERE pe.process_name = $1 ORDER BY pe.key`, datatable.JobPurgeData)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []purgeFailure
	for rows.Next() {
		var f purgeFailure
		if err := rows.Scan(&f.session, &f.details, &f.sourceType, &f.class, &f.source, &f.configKey); err != nil {
			t.Fatal(err)
		}
		out = append(out, f)
	}
	return out
}

func checkJobRow(t *testing.T, f purgeFailure) {
	t.Helper()
	if f.configKey != 0 || f.sourceType != datatable.SessionSourceTypeJob ||
		f.class != datatable.FailureClassJobFailed || f.source != datatable.FailureSourceJob {
		t.Errorf("the purge's failure %+v is not a job's row under a registered job session", f)
	}
}

// A run that stops with an error records it once, even though its context is
// what failed; the run still returns the error.
func TestPurgeRecordsARunThatFails(t *testing.T) {
	pool := jobFailureTestDB(t)
	seedPurge(t, pool)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	now := time.Now()
	err := purgeSessions(ctx, pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)})
	if err == nil {
		t.Fatal("a run on a cancelled context returned no error")
	}
	got := purgeFailures(t, pool)
	if len(got) != 1 {
		t.Fatalf("%d failures recorded, want 1: %+v", len(got), got)
	}
	checkJobRow(t, got[0])
	if !strings.HasPrefix(got[0].details, "the purge stopped: ") || !strings.Contains(got[0].details, err.Error()) {
		t.Errorf("failure_details %q, want the run's error %q", got[0].details, err)
	}
}

// A run that fails to delete a session goes on, and records one failure naming
// the session and its error; a dry run records the same, marked as a dry run.
func TestPurgeRecordsTheSessionsItFailsToDelete(t *testing.T) {
	for _, dry := range []bool{true, false} {
		name := "real run"
		if dry {
			name = "dry run"
		}
		t.Run(name, func(t *testing.T) {
			pool := jobFailureTestDB(t)
			purgetest.Seed(t, pool, purgetest.Session{ID: "stuck", Client: "c1", Registered: true, LastUpdate: daysAgo(40),
				DomainTables: []string{"RefuseTable"}})
			purgetest.Seed(t, pool, purgetest.Session{ID: "fine", Client: "c1", Registered: true, LastUpdate: daysAgo(40),
				DomainTables: []string{"DomainA"}})
			purgetest.Exec(t, pool, `CREATE FUNCTION public.refuse() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN RAISE EXCEPTION 'delete refused by test trigger'; END $$;
				CREATE TRIGGER refuse BEFORE DELETE ON public."RefuseTable" FOR EACH ROW EXECUTE FUNCTION public.refuse()`)
			now := time.Now()
			if err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, DryRun: dry,
				Now: now, Deadline: now.Add(runBudget)}); err != nil {
				t.Fatalf("a run with a failed session returned %v, want it to go on", err)
			}
			got := purgeFailures(t, pool)
			if len(got) != 1 {
				t.Fatalf("%d failures recorded, want 1: %+v", len(got), got)
			}
			checkJobRow(t, got[0])
			d := got[0].details
			if !strings.Contains(d, "1 sessions failed to delete") || !strings.Contains(d, "stuck (") ||
				!strings.Contains(d, "delete refused by test trigger") || strings.Contains(d, "fine") {
				t.Errorf("failure_details %q, want the stuck session and its error only", d)
			}
			if dry != strings.HasPrefix(d, "dry run, nothing was deleted") {
				t.Errorf("failure_details %q: dry run %v", d, dry)
			}
		})
	}
}

// A run that goes right records nothing.
func TestPurgeRecordsNothingWhenNothingFails(t *testing.T) {
	pool := jobFailureTestDB(t)
	seedPurge(t, pool)
	for _, dry := range []bool{true, false} {
		now := time.Now()
		if err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, DryRun: dry,
			Now: now, Deadline: now.Add(runBudget)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := purgeFailures(t, pool); len(got) != 0 {
		t.Errorf("a run that went right recorded %+v", got)
	}
}

// The record of a purge's failure has no client, and once past retention it is
// purged like any session: the empty client does not stop the run.
func TestPurgeDeletesAnOldFailureOfItsOwn(t *testing.T) {
	pool := jobFailureTestDB(t)
	sessionId, err := datatable.RecordJobFailure(context.Background(), pool,
		datatable.JobFailure{Job: datatable.JobPurgeData, Details: "an old failure"})
	if err != nil {
		t.Fatal(err)
	}
	purgetest.Exec(t, pool, `UPDATE jetsapi.session_registry SET last_update = now() - interval '40 days'
		WHERE session_id = $1`, sessionId)
	now := time.Now()
	if err := purgeSessions(context.Background(), pool, purgeConfig{RetentionDays: 30, Now: now,
		Deadline: now.Add(runBudget)}); err != nil {
		t.Fatalf("the purge stopped on a session with no client: %v", err)
	}
	if left := purgetest.SessionRows(t, pool, sessionId); len(left) > 0 {
		t.Errorf("the old failure's rows are left: %v", left)
	}
	if got := purgeFailures(t, pool); len(got) != 0 {
		t.Errorf("failures after the purge: %+v, want none", got)
	}
}

// A record that cannot be written is logged and changes nothing about the run.
func TestPurgeGoesOnWhenItsRecordFails(t *testing.T) {
	saved := recordFailure
	recordFailure = func(context.Context, *pgxpool.Pool, datatable.JobFailure) (string, error) {
		return "", errors.New("database unavailable")
	}
	t.Cleanup(func() { recordFailure = saved })
	runErr := errors.New("while reading the retention candidates: boom")
	recordRun(context.Background(), nil, purgeConfig{}, runErr, nil) // must not panic
}
