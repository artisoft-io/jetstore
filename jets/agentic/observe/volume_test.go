// The volume detector's basis per step, against a real Postgres.
//
// Each step is judged against its own history, and the floor applies only to a
// step with too little of it. The cases are the whole of that rule: a reducing
// step whose ratio is low and steady fires nothing, a step whose ratio collapses
// against its own history fires even above the floor, and a step with no history
// falls back to the floor and says whether its history is short or purged.
//
// The worker rows are written by the cpipes worker's own writers (workerRow in
// detect_test.go). Needs JETS_TEST_DSN, as extract_test.go does; skipped
// otherwise. Run with -count=1: the suite reads jets_schema.json and
// jets_init_db.sql, which the test cache does not see.
package observe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// historyPool is testPool plus the archive table and the view
// pipeline_execution_history, which a deployment's migration installs beside the
// execution tables.
func historyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)
	for _, def := range purgetest.SchemaDefinitions(t) {
		if def.TableName != "pipeline_execution_status_archive" {
			continue
		}
		if err := def.UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("installing %s: %v", def.TableName, err)
		}
	}
	// Dropped first, as jets_init_db.sql does: on a database a test run with an
	// older view left behind, CREATE OR REPLACE cannot insert its new columns.
	if _, err := pool.Exec(context.Background(), `DROP VIEW IF EXISTS jetsapi.pipeline_execution_history`); err != nil {
		t.Fatalf("dropping pipeline_execution_history: %v", err)
	}
	if _, err := pool.Exec(context.Background(), purgetest.HistoryViewStatement(t)); err != nil {
		t.Fatalf("creating pipeline_execution_history: %v", err)
	}
	return pool
}

// run writes one completed run of one step: a header and one worker per
// (in, out) pair, all started at start. It returns the header key and the
// worker keys.
func run(t *testing.T, pool *pgxpool.Pool, client, process, step string, start time.Time,
	counts ...[2]int) (int, []int64) {
	t.Helper()
	session := uniq(t)
	ek := header(t, pool, client, process, "obj", session, "completed", start)
	var keys []int64
	for i, c := range counts {
		keys = append(keys, workerRow(t, pool, ek, client, process, session, step, i,
			StatusCompleted, c[0], c[1], start))
	}
	return ek, keys
}

// detectWithHistory gathers the window, reads the volume history and judges it.
func detectWithHistory(t *testing.T, pool *pgxpool.Pool, d VolumeCollapse, w Window) ([]Anomaly, *Evidence) {
	t.Helper()
	ctx := context.Background()
	ev, err := Gather(ctx, pool, w, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.ReadHistory(ctx, pool, ev); err != nil {
		t.Fatal(err)
	}
	return d.Detect(ev), ev
}

// A reducing step writes a small, steady fraction of what it reads. The floor
// alone flags every run of it; judged against its own history, a run at its
// usual ratio is not an anomaly.
func TestVolumeASteadyReducingStepFiresNothing(t *testing.T) {
	pool := historyPool(t)
	client := uniq(t)
	now := time.Now().UTC()
	for i := 5; i >= 1; i-- {
		run(t, pool, client, "review", "reducing01", now.Add(-time.Duration(i)*24*time.Hour),
			[2]int{200000, 3600 + 50*i})
	}
	start := now.Add(-time.Hour)
	run(t, pool, client, "review", "reducing01", start, [2]int{200000, 3500})

	w := Window{Since: start.Add(-time.Minute), Client: client}
	found, ev := detectWithHistory(t, pool, DefaultVolumeCollapse(), w)
	if len(found) != 0 {
		t.Fatalf("a run at its step's usual ratio fired %d anomalies: %s", len(found), found[0].ExpectedBasis)
	}
	if n := len(ev.Volume.Runs[volumeKey(client, "review", "reducing01")]); n != 6 {
		t.Errorf("history holds %d runs of the step, want the 5 before the window and the 1 in it", n)
	}
	// The contrast, which is the production evidence in one assertion: with no
	// history read, the same row is judged against the floor and fires.
	ev.Volume = nil
	if floor := DefaultVolumeCollapse().Detect(ev); len(floor) != 1 ||
		!strings.HasPrefix(floor[0].ExpectedBasis, "against the absolute floor") ||
		!strings.Contains(floor[0].ExpectedBasis, "No history was read") {
		t.Errorf("without history the steady step should fire once against the floor, got %+v", floor)
	}
}

// A step whose ratio falls well below its own median fires, even when the ratio
// is above the floor: an expanding step that suddenly writes fewer rows than it
// reads has lost most of its output, and the floor could never see it.
func TestVolumeAStepThatCollapsesAgainstItsHistoryFires(t *testing.T) {
	pool := historyPool(t)
	client := uniq(t)
	now := time.Now().UTC()
	for i := 4; i >= 1; i-- {
		run(t, pool, client, "expand", "explode01", now.Add(-time.Duration(i)*24*time.Hour),
			[2]int{10000, 20000}, [2]int{10000, 20000})
	}
	start := now.Add(-time.Hour)
	_, keys := run(t, pool, client, "expand", "explode01", start, [2]int{10000, 8000}, [2]int{10000, 20000})

	found, _ := detectWithHistory(t, pool, DefaultVolumeCollapse(), Window{Since: start.Add(-time.Minute), Client: client})
	if len(found) != 1 {
		t.Fatalf("got %d anomalies, want 1 for the shard that collapsed: %+v", len(found), found)
	}
	a := found[0]
	if a.SubjectRef != itoa(keys[0]) {
		t.Errorf("subject is worker %s, want %d", a.SubjectRef, keys[0])
	}
	if a.DetectorRef != "volume_collapse@2" {
		t.Errorf("detector_ref = %s, want generation 2", a.DetectorRef)
	}
	if !strings.HasPrefix(a.ExpectedBasis, "against the step's own history") ||
		!strings.Contains(a.ExpectedBasis, "over its 4 prior runs") ||
		!strings.Contains(a.ExpectedBasis, "is 2, and the detector fires below 50% of it") {
		t.Errorf("basis does not name the history it compared against: %s", a.ExpectedBasis)
	}
	// Half the median of 2.0, over 10,000 rows read.
	if a.ExpectedMin == nil || *a.ExpectedMin != "10000" {
		t.Errorf("expected minimum = %v, want 10000", a.ExpectedMin)
	}
	if a.DeviationMagnitude == nil || *a.DeviationMagnitude < 0.59 || *a.DeviationMagnitude > 0.61 {
		t.Errorf("deviation = %v, want 0.6, the shortfall from the step's own median", a.DeviationMagnitude)
	}
	if contains(a.Confounders, ConfounderHistoryTruncated) {
		t.Errorf("a step judged on its history carries history_truncated: %v", a.Confounders)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("the anomaly does not validate: %v", err)
	}
}

// A step with too little history is judged against the floor, and the basis
// says whether the history is short or was purged with its sessions.
func TestVolumeAStepWithNoHistoryFallsBackToTheFloor(t *testing.T) {
	pool := historyPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("short", func(t *testing.T) {
		client := uniq(t)
		run(t, pool, client, "fresh", "reducing01", now.Add(-48*time.Hour), [2]int{10000, 10000})
		start := now.Add(-time.Hour)
		run(t, pool, client, "fresh", "reducing01", start, [2]int{10000, 100})
		found, _ := detectWithHistory(t, pool, DefaultVolumeCollapse(), Window{Since: start.Add(-time.Minute), Client: client})
		if len(found) != 1 {
			t.Fatalf("got %d anomalies, want 1 against the floor", len(found))
		}
		b := found[0].ExpectedBasis
		if !strings.HasPrefix(b, "against the absolute floor") ||
			!strings.Contains(b, "has 1 prior runs with counts, fewer than the 3") ||
			!strings.Contains(b, "short rather than purged") {
			t.Errorf("basis does not say the floor was applied for a short history: %s", b)
		}
		if found[0].ExpectedMin == nil || *found[0].ExpectedMin != "5000" {
			t.Errorf("expected minimum = %v, want the floor over the input", found[0].ExpectedMin)
		}
		if contains(found[0].Confounders, ConfounderHistoryTruncated) {
			t.Errorf("a short history is not a truncated one: %v", found[0].Confounders)
		}
	})

	t.Run("purged", func(t *testing.T) {
		client := uniq(t)
		for i := 1; i <= 3; i++ {
			if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status_archive
				(key, pipeline_config_key, client, process_name, main_object_type, session_id,
				 source_period_key, status, user_email, start_time, last_update, archive_reason)
				VALUES ($1, 1, $2, 'purged', 'obj', $3, 1, 'completed', 'test@test', $4, $4, 'retention')`,
				700000000+time.Now().UnixNano()%100000000+int64(i), client, client+"_old"+itoa(int64(i)),
				now.Add(-time.Duration(30+i)*24*time.Hour)); err != nil {
				t.Fatalf("archiving a run: %v", err)
			}
		}
		start := now.Add(-time.Hour)
		run(t, pool, client, "purged", "reducing01", start, [2]int{10000, 100})
		found, _ := detectWithHistory(t, pool, DefaultVolumeCollapse(), Window{Since: start.Add(-time.Minute), Client: client})
		if len(found) != 1 {
			t.Fatalf("got %d anomalies, want 1 against the floor", len(found))
		}
		b := found[0].ExpectedBasis
		if !strings.HasPrefix(b, "against the absolute floor") ||
			!strings.Contains(b, "3 earlier runs of process purged are archived") {
			t.Errorf("basis does not say the history may have been purged: %s", b)
		}
		if !contains(found[0].Confounders, ConfounderHistoryTruncated) {
			t.Errorf("a step whose history may be purged does not carry history_truncated: %v", found[0].Confounders)
		}
	})
}

// The history is the step's own and nothing else: the run being judged is not
// its own precedent, a run that partly failed or read too little does not move
// the median, a later run in the window is not history for an earlier one, and
// only the last HistoryRuns runs count.
func TestVolumeHistoryIsTheStepsOwnPriorRuns(t *testing.T) {
	pool := historyPool(t)
	client := uniq(t)
	now := time.Now().UTC()

	// Ten old runs at ratio 0.9 that the window of three pushes out.
	for i := 20; i >= 11; i-- {
		run(t, pool, client, "p", "s1", now.Add(-time.Duration(i)*24*time.Hour), [2]int{10000, 9000})
	}
	// Three recent runs at ratio 0.02, which are the step's history.
	for i := 5; i >= 3; i-- {
		run(t, pool, client, "p", "s1", now.Add(-time.Duration(i)*24*time.Hour), [2]int{10000, 200})
	}
	// A partly failed run and a tiny one, both at ratio 0.9, which must not count.
	session := uniq(t)
	ek := header(t, pool, client, "p", "obj", session, "failed", now.Add(-48*time.Hour))
	workerRow(t, pool, ek, client, "p", session, "s1", 0, StatusCompleted, 10000, 9000, now.Add(-48*time.Hour))
	workerRow(t, pool, ek, client, "p", session, "s1", 1, StatusFailed, 10000, 0, now.Add(-48*time.Hour))
	run(t, pool, client, "p", "s1", now.Add(-30*time.Hour), [2]int{10, 9})
	// Another step of the same process, at 0.9, which is not this step's history.
	run(t, pool, client, "p", "s2", now.Add(-26*time.Hour), [2]int{10000, 9000})

	// Two runs in the window: the first at the history's ratio, the second at a
	// collapse of it. The first is history for the second; the second is not
	// history for the first.
	first := now.Add(-3 * time.Hour)
	run(t, pool, client, "p", "s1", first, [2]int{10000, 200})
	second := now.Add(-time.Hour)
	_, keys := run(t, pool, client, "p", "s1", second, [2]int{10000, 5})

	d := DefaultVolumeCollapse()
	d.HistoryRuns = 3
	found, ev := detectWithHistory(t, pool, d, Window{Since: first.Add(-time.Minute), Client: client})
	if n := len(ev.Volume.Runs[volumeKey(client, "p", "s1")]); n != 5 {
		t.Errorf("history holds %d runs of step s1, want the last 3 before the window and the 2 in it", n)
	}
	if len(found) != 1 || found[0].SubjectRef != itoa(keys[0]) {
		t.Fatalf("want one anomaly on the collapsed run's worker %d, got %+v", keys[0], found)
	}
	b := found[0].ExpectedBasis
	// The second run's three predecessors are two of the old recent runs and
	// the first run in the window, all at 0.02.
	if !strings.Contains(b, "over its 3 prior runs") || !strings.Contains(b, "is 0.02,") {
		t.Errorf("basis is not the median of the last three qualifying runs: %s", b)
	}
	if !strings.Contains(b, first.Truncate(time.Second).Format(time.RFC3339)) {
		t.Errorf("the run earlier in the window is not the latest of the history: %s", b)
	}
}

// Generation 2 re-judges what generation 1 judged: the anomaly_id carries the
// generation, so the same worker gets a new row rather than the old one's
// conflict.
func TestVolumeGenerationTwoReJudgesGenerationOne(t *testing.T) {
	pool := historyPool(t)
	ctx := context.Background()
	client := uniq(t)
	start := time.Now().UTC().Add(-time.Hour)
	run(t, pool, client, "p", "reducing01", start, [2]int{10000, 100})
	w := Window{Since: start.Add(-time.Minute), Client: client}

	old := DefaultVolumeCollapse()
	old.Generation = 1
	ev, err := Gather(ctx, pool, w, 0)
	if err != nil {
		t.Fatal(err)
	}
	g1 := old.Detect(ev)
	if len(g1) != 1 {
		t.Fatalf("generation 1 found %d, want 1", len(g1))
	}
	if isNew, err := InsertAnomalyIfNew(ctx, pool, &g1[0]); err != nil || !isNew {
		t.Fatalf("generation 1 insert: new=%v err=%v", isNew, err)
	}
	g2, _ := detectWithHistory(t, pool, DefaultVolumeCollapse(), w)
	if len(g2) != 1 {
		t.Fatalf("generation 2 found %d, want 1", len(g2))
	}
	if g2[0].AnomalyId == g1[0].AnomalyId {
		t.Fatalf("both generations compose anomaly_id %s", g2[0].AnomalyId)
	}
	if isNew, err := InsertAnomalyIfNew(ctx, pool, &g2[0]); err != nil || !isNew {
		t.Errorf("generation 2 did not write a row over a worker generation 1 judged: new=%v err=%v", isNew, err)
	}
}
