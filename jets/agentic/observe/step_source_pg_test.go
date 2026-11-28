// The step-regression detector against the baseline of each run's own main
// input source: one process reading two sources whose runs reach different
// steps.
//
// Needs JETS_TEST_DSN, as the rest of the package; run with -count=1.
package observe

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// registerRunSource registers a run's main input under src and points the
// run's header at it, as a run started from a registered file is.
func registerRunSource(t *testing.T, pool *pgxpool.Pool, execKey int, src Source, session string) {
	t.Helper()
	ctx := context.Background()
	var irKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.input_registry
		(client, org, object_type, file_key, source_period_key, table_name, source_type, session_id, user_email)
		VALUES ($1, $2, $3, $4, 1, $5, 'file', $6, 'test@test') RETURNING key`,
		src.Client, src.Org, src.ObjectType, "in/"+session, src.Org+"_"+src.ObjectType, "reg-"+session).
		Scan(&irKey); err != nil {
		t.Fatalf("registering the main input of %s: %v", session, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE jetsapi.pipeline_execution_status SET main_input_registry_key = $1
		WHERE key = $2`, irKey, execKey); err != nil {
		t.Fatalf("pointing %s at its main input: %v", session, err)
	}
}

// sourceStepRun writes one run of process p for client c1 at start, with one
// worker row per step and the status given for it; src nil leaves the run with
// no main input registration. It returns the worker keys by step.
func sourceStepRun(t *testing.T, pool *pgxpool.Pool, src *Source, session string, start time.Time,
	steps map[string]string) map[string]int64 {
	t.Helper()
	status := "completed"
	for _, s := range steps {
		if s == StatusFailed {
			status = "failed"
		}
	}
	ek := header(t, pool, "c1", "p", "obj", session, status, start)
	if src != nil {
		registerRunSource(t, pool, ek, *src, session)
	}
	keys := map[string]int64{}
	shard := 0
	for step, s := range steps {
		keys[step] = workerRow(t, pool, ek, "c1", "p", session, step, shard, s, 100, 100, start)
		shard++
	}
	return keys
}

var (
	stepSourceA = Source{Client: "c1", Org: "vendor_a", ObjectType: "obj"}
	stepSourceB = Source{Client: "c1", Org: "vendor_b", ObjectType: "obj"}
)

// The two sources' histories, four runs each, interleaved: source A's files
// reach step reducing01 only, source B's reach reducing01 and reducing02.
func twoSourceHistory(t *testing.T, pool *pgxpool.Pool, a, b *Source, old time.Time) {
	t.Helper()
	for i := range 4 {
		at := old.Add(time.Duration(2*i) * time.Hour)
		sourceStepRun(t, pool, a, "a"+itoa(int64(i)), at, map[string]string{"reducing01": StatusCompleted})
		sourceStepRun(t, pool, b, "b"+itoa(int64(i)), at.Add(time.Hour), map[string]string{
			"reducing01": StatusCompleted, "reducing02": StatusCompleted})
	}
}

// A failed step is a regression only against its own source's runs. Judged
// against the process as a whole, a source A run failing reducing02 would be a
// regression, because source B's runs completed it four times; source A's runs
// never ran it, so it is not one. A source B run failing it is, and so is a
// source A run failing the step its own runs completed.
func TestStepRegressionJudgesEachRunAgainstItsOwnSource(t *testing.T) {
	pool := purgeablePool(t, "agentic_observe_step_source")
	ctx := context.Background()
	windowOpens := time.Now().UTC().Add(-time.Hour)
	twoSourceHistory(t, pool, &stepSourceA, &stepSourceB, windowOpens.Add(-72*time.Hour))

	now := windowOpens.Add(10 * time.Minute)
	aNew := sourceStepRun(t, pool, &stepSourceA, "a-new-step", now, map[string]string{
		"reducing01": StatusCompleted, "reducing02": StatusFailed})
	bFailed := sourceStepRun(t, pool, &stepSourceB, "b-failed", now.Add(time.Minute), map[string]string{
		"reducing01": StatusCompleted, "reducing02": StatusFailed})
	aFailed := sourceStepRun(t, pool, &stepSourceA, "a-failed", now.Add(2*time.Minute), map[string]string{
		"reducing01": StatusFailed})

	ev, err := Gather(ctx, pool, Window{Since: windowOpens, Client: "c1"}, 96*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Extent.Sources || !ev.Prior.BySource {
		t.Fatalf("sources recorded %v, baseline by source %v; want both", ev.Extent.Sources, ev.Prior.BySource)
	}
	// Three baselines, one per (source, step) the history holds: A has no
	// reducing02.
	got := map[string]int64{}
	for _, b := range ev.Prior.Baselines {
		got[b.Source.Org+"/"+b.StepId] = b.Completed
	}
	want := map[string]int64{"vendor_a/reducing01": 4, "vendor_b/reducing01": 4, "vendor_b/reducing02": 4}
	if len(got) != len(want) {
		t.Fatalf("baselines %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("baseline %s completed %d times, want %d (all baselines %v)", k, got[k], n, got)
		}
	}
	for _, r := range ev.Workers.Rows {
		if strings.HasPrefix(r.SessionId, "a-") && r.Source != stepSourceA ||
			strings.HasPrefix(r.SessionId, "b-") && r.Source != stepSourceB {
			t.Errorf("worker row %d of %s has source %+v", r.Key, r.SessionId, r.Source)
		}
	}

	found, err := DefaultStepRegression().Detect(ev)
	if err != nil {
		t.Fatal(err)
	}
	subjects := map[string]Anomaly{}
	for _, a := range found {
		subjects[a.SubjectRef] = a
	}
	if _, ok := subjects[itoa(aNew["reducing02"])]; ok {
		t.Errorf("a source A run failing a step only source B's runs completed was called a regression")
	}
	for _, k := range []int64{bFailed["reducing02"], aFailed["reducing01"]} {
		if _, ok := subjects[itoa(k)]; !ok {
			t.Errorf("worker %d failed a step its own source completed four times, and raised nothing", k)
		}
	}
	if len(found) != 2 {
		t.Errorf("got %d anomalies, want 2: %+v", len(found), found)
	}
	if a, ok := subjects[itoa(bFailed["reducing02"])]; ok {
		if !strings.Contains(a.ExpectedBasis, stepSourceB.Describe()) {
			t.Errorf("the basis does not name the run's own source: %q", a.ExpectedBasis)
		}
		if a.DetectorRef != "step_regression@2" {
			t.Errorf("detector ref %q, want step_regression@2", a.DetectorRef)
		}
		if err := a.Validate(); err != nil {
			t.Errorf("the anomaly does not validate: %v", err)
		}
	}

	// A baseline that read every source of the process as one is refused on a
	// database that records sources: matching it is the pooling above.
	pooled, err := StepBaselines(ctx, pool, Window{Since: ev.Prior.Window.Since, Until: ev.Prior.Window.Until,
		Client: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	if pooled.BySource {
		t.Fatal("a baseline read with neither Source nor BySource says it is by source")
	}
	if _, err := DefaultStepRegression().Detect(&Evidence{Workers: ev.Workers, Prior: pooled}); err == nil {
		t.Error("a baseline pooling the sources of a process was accepted")
	}

	// A baseline kept to one source judges only that source's runs.
	one, err := StepBaselines(ctx, pool, Window{Since: ev.Prior.Window.Since, Until: ev.Prior.Window.Until,
		Client: "c1", Source: &stepSourceB})
	if err != nil {
		t.Fatal(err)
	}
	onlyB, err := DefaultStepRegression().Detect(&Evidence{Workers: ev.Workers, Prior: one})
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyB) != 1 || onlyB[0].SubjectRef != itoa(bFailed["reducing02"]) {
		t.Errorf("against source B's baseline alone, got %+v; want the source B run's failure only", onlyB)
	}
}

// Where jetsapi.input_registry does not exist, every run has the empty source
// and the process has one baseline, as before the source was part of the key:
// the run failing reducing02 is a regression against the runs that completed
// it.
func TestStepRegressionWithoutInputRegistryKeepsOneBaseline(t *testing.T) {
	pool := purgeablePool(t, "agentic_observe_step_no_source")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DROP TABLE jetsapi.input_registry CASCADE`); err != nil {
		t.Fatal(err)
	}
	windowOpens := time.Now().UTC().Add(-time.Hour)
	twoSourceHistory(t, pool, nil, nil, windowOpens.Add(-72*time.Hour))
	judged := sourceStepRun(t, pool, nil, "a-new-step", windowOpens.Add(10*time.Minute), map[string]string{
		"reducing01": StatusCompleted, "reducing02": StatusFailed})

	ev, err := Gather(ctx, pool, Window{Since: windowOpens, Client: "c1"}, 96*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Extent.Sources || !ev.Prior.BySource {
		t.Fatalf("sources recorded %v, baseline by source %v; want false and true", ev.Extent.Sources,
			ev.Prior.BySource)
	}
	if len(ev.Prior.Baselines) != 2 {
		t.Fatalf("got %d baselines, want one per step: %+v", len(ev.Prior.Baselines), ev.Prior.Baselines)
	}
	for _, b := range ev.Prior.Baselines {
		if !b.Source.IsZero() {
			t.Errorf("baseline of %s has source %+v with no input_registry", b.StepId, b.Source)
		}
	}
	found, err := DefaultStepRegression().Detect(ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].SubjectRef != itoa(judged["reducing02"]) {
		t.Fatalf("got %+v, want the one failure of reducing02", found)
	}
	if !strings.Contains(found[0].ExpectedBasis, "jetsapi.input_registry does not exist") {
		t.Errorf("the basis does not say the runs have no recorded source: %q", found[0].ExpectedBasis)
	}
}
