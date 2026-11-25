// The readers of the execution record after a session is deleted. A deleted
// session's run headers move to pipeline_execution_status_archive, and its
// worker rows are deleted, by jets/sessionpurge; these tests delete a session
// through that package, on a database built as update_db builds it (the archive
// and the view pipeline_execution_history included), and check what the record
// still says about the run.
//
// Needs JETS_TEST_DSN, as the rest of the package; run with -count=1.
package observe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/sessionpurge"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// purgeablePool is a fresh database with every jetsapi table, the history view
// and the agentic DDL.
func purgeablePool(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	pool := purgetest.DB(t, name)
	if err := audit.InstallSchema(context.Background(), pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// deleteSessions deletes the sessions as a user's Delete does: queued, then
// executed, one transaction each.
func deleteSessions(t *testing.T, pool *pgxpool.Pool, client string, sessions ...string) {
	t.Helper()
	ctx := context.Background()
	var reqs []sessionpurge.Request
	for _, s := range sessions {
		reqs = append(reqs, sessionpurge.Request{SessionID: s, Client: client,
			Reason: sessionpurge.ReasonUser, RequestedBy: "test@test"})
	}
	if _, err := sessionpurge.Queue(ctx, pool, reqs); err != nil {
		t.Fatalf("queueing the deletions: %v", err)
	}
	report, err := sessionpurge.Execute(ctx, pool, sessionpurge.Options{
		Reasons: []sessionpurge.Reason{sessionpurge.ReasonUser}})
	if err != nil {
		t.Fatalf("executing the deletions: %v", err)
	}
	if len(report.FailedSessions) > 0 || len(report.RefusedSessions) > 0 {
		t.Fatalf("deletions failed %v, refused %v", report.FailedSessions, report.RefusedSessions)
	}
}

// ReadExtent counts the archived headers with the live ones, so the record's
// oldest header is a deleted session's run when that is the oldest.
func TestReadExtentCountsTheArchivedRuns(t *testing.T) {
	pool := purgeablePool(t, "agentic_observe_extent_history")
	start := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	done := []worker{{stepId: "reducing00", status: StatusCompleted, in: i64(100), out: i64(100)}}
	insertRun(t, pool, "c1", "p", "obj", "s-old", "completed", false, start, done)
	insertRun(t, pool, "c1", "p", "obj", "s-new", "completed", false, start.Add(24*time.Hour), done)

	before, err := ReadExtent(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if !before.History || before.Headers != 2 || before.Archived != 0 || !before.OldestHeader.Equal(start) {
		t.Fatalf("before the deletion: %+v", before)
	}

	deleteSessions(t, pool, "c1", "s-old")
	e, err := ReadExtent(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if e.Headers != 2 || e.Archived != 1 {
		t.Errorf("after deleting one of two sessions: %d headers, %d archived; want 2 and 1", e.Headers, e.Archived)
	}
	if !e.OldestHeader.Equal(start) {
		t.Errorf("the oldest header is %s; the deleted session's run, at %s, is still the oldest in the history",
			e.OldestHeader, start)
	}
	if !e.OldestWorker.Equal(start.Add(24 * time.Hour)) {
		t.Errorf("the oldest worker row is %s; the deleted session's went with it, so want %s",
			e.OldestWorker, start.Add(24*time.Hour))
	}
	if e.Regime() != "header-outlives-detail" {
		t.Errorf("Regime() = %q with a deleted session's header older than any worker row", e.Regime())
	}
}

// A step baseline cannot hold a deleted session's run, whose worker rows are
// gone, but it reads the run's header from the history and says the baseline of
// that source is short. Another source in the same window is not qualified.
func TestStepBaselinesSayWhichSourceLostARun(t *testing.T) {
	pool := purgeablePool(t, "agentic_observe_baseline_history")
	ctx := context.Background()
	t0 := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	done := []worker{{stepId: "reducing00", status: StatusCompleted, in: i64(100), out: i64(100)}}
	for i, s := range []string{"a1", "a2", "a3", "a4"} {
		insertRun(t, pool, "c1", "p", "claims", s, "completed", false, t0.Add(time.Duration(i)*time.Hour), done)
	}
	insertRun(t, pool, "c2", "p", "claims", "b1", "completed", false, t0, done)
	insertRun(t, pool, "c2", "p", "claims", "b2", "completed", false, t0.Add(time.Hour), done)

	// The window opens at the oldest worker row that survives, so the only
	// shortfall is the deletion.
	w := Window{Since: t0, Until: t0.Add(24 * time.Hour)}
	deleteSessions(t, pool, "c1", "a2", "a3")

	set, err := StepBaselines(ctx, pool, w)
	if err != nil {
		t.Fatal(err)
	}
	var a, b *StepBaseline
	for i := range set.Baselines {
		switch set.Baselines[i].Client {
		case "c1":
			a = &set.Baselines[i]
		case "c2":
			b = &set.Baselines[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("baselines %+v", set.Baselines)
	}
	if a.Runs != 2 {
		t.Errorf("c1's baseline holds %d runs; its two deleted runs have no worker rows, so want 2", a.Runs)
	}
	if n := set.ArchivedFor(a); n != 2 {
		t.Errorf("ArchivedFor(c1) = %d, want the 2 runs read from the archive", n)
	}
	if !contains(a.Confounders, ConfounderHistoryTruncated) {
		t.Errorf("c1's baseline lost two runs and is not qualified: %v", a.Confounders)
	}
	if d := set.Describe(a); !strings.Contains(d, "2 runs of c1/p/claims in the window were deleted") {
		t.Errorf("Describe does not say what the baseline lost: %s", d)
	}
	if set.ArchivedFor(b) != 0 || contains(b.Confounders, ConfounderHistoryTruncated) {
		t.Errorf("c2 lost nothing and is qualified: archived %d, %v", set.ArchivedFor(b), b.Confounders)
	}
	if !set.Truncated() {
		t.Error("Truncated() is false over a window two of whose runs were deleted")
	}
}
