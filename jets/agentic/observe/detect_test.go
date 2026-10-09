// The two detectors against a real Postgres, over worker rows written by the
// cpipes worker's own writers.
//
// # What "over data a real pipeline wrote" means here, exactly
//
// Criterion 34 asks for two detectors emitting a persisted Anomaly over
// execution data a real pipeline wrote. Four of the five layers below are the
// production ones and the fifth is not, and the difference is stated here
// rather than left to be assumed:
//
//   - The execution tables are installed from jets/jets_schema.json by the same
//     jets/schema code `update_db -migrateDb` runs.
//   - jetsapi.anomaly is installed by audit.InstallSchema, the same call
//     update_db makes (jets/update_db/main.go:71).
//   - The worker rows are written by ComputePipesContext.InsertPipelineExecutionStatus
//     and UpdatePipelineExecutionStatus (jets/compute_pipes/actions_process_file.go:302,
//     :320), called with the arguments ProcessFilesAndReportStatus passes them
//     (:281) — so the columns every detector reads were produced by the code
//     that produces them in production, rather than by SQL this file composed.
//   - The configurations the confounder scan reads are JetStore's own shipped
//     pipeline assets, jets/workspace_assets/pipes_config/*.pc.json, unmodified.
//   - The run headers are composed here, because their writer is the apiserver
//     at submission; and no pipeline ran, because a pipeline needs S3, a
//     compiled workspace and a state machine. So the write *path* is real and
//     the *run* is not.
//
// The rows are backdated after the insert, because the worker's own insert
// takes start_time from the database's now() and a baseline needs history.
//
// Needs JETS_TEST_DSN, as extract_test.go does; skipped otherwise.
package observe

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	embedConfigPath  = "../../workspace_assets/pipes_config/embed_input_parts.pc.json"
	loaderConfigPath = "../../workspace_assets/pipes_config/jets_loader.pc.json"
)

// header writes one run header. Its production writer is the apiserver at
// submission, which is not reachable from a test.
func header(t *testing.T, pool *pgxpool.Pool, client, process, objectType, session, status string,
	start time.Time) int {
	t.Helper()
	var key int
	err := pool.QueryRow(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id,
		 source_period_key, status, user_email, start_time, last_update)
		VALUES (1, $1, $2, $3, $4, 1, $5, 'test@test', $6, $6) RETURNING key`,
		client, process, objectType, session, status, start).Scan(&key)
	if err != nil {
		t.Fatalf("inserting run header: %v", err)
	}
	return key
}

// workerRow writes one worker row the way a cpipes node does: the 'in progress'
// insert, then the update that carries the counts and the terminal status. Pass
// terminal == "" to leave the row as the insert left it, which is what a worker
// that never came back looks like and is where the six NULL counts come from
// (F99).
func workerRow(t *testing.T, pool *pgxpool.Pool, execKey int, client, process, session, stepId string,
	shard int, terminal string, in, out int, start time.Time) int64 {
	t.Helper()
	cpCtx := &compute_pipes.ComputePipesContext{
		ComputePipesArgs: compute_pipes.ComputePipesArgs{
			ComputePipesNodeArgs: compute_pipes.ComputePipesNodeArgs{
				NodeId:             shard,
				JetsPartitionLabel: "",
				PipelineExecKey:    execKey,
			},
			ComputePipesCommonArgs: compute_pipes.ComputePipesCommonArgs{
				Client:            client,
				ProcessName:       process,
				SessionId:         session,
				InputSessionId:    session,
				SourcePeriodKey:   1,
				PipelineConfigKey: 1,
				UserEmail:         "test@test",
				MainInputStepId:   stepId,
			},
		},
	}
	key, err := cpCtx.InsertPipelineExecutionStatus(pool)
	if err != nil {
		t.Fatalf("InsertPipelineExecutionStatus: %v", err)
	}
	if terminal != "" {
		// The argument order is ProcessFilesAndReportStatus's own
		// (actions_process_file.go:281): loaded, bad, size mb, files, rete
		// sessions, output.
		if err := cpCtx.UpdatePipelineExecutionStatus(pool, key,
			in, 0, 1, 1, 0, out, stepId, terminal, ""); err != nil {
			t.Fatalf("UpdatePipelineExecutionStatus: %v", err)
		}
	}
	// Backdate: the insert took now() from the database, and a baseline needs
	// a row that started before the window it is a baseline for.
	if _, err := pool.Exec(context.Background(),
		`UPDATE jetsapi.pipeline_execution_details SET start_time = $1, last_update = $1 WHERE key = $2`,
		start, key); err != nil {
		t.Fatalf("backdating worker row: %v", err)
	}
	return int64(key)
}

// storeConfig puts a shipped JetStore pipeline configuration where a run's
// configuration lives, so the confounder scan reads a real document.
func storeConfig(t *testing.T, pool *pgxpool.Pool, execKey int, session, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	// sharding_config_json and reducing_config_json are marked deleted in
	// jets_schema.json and jets/schema does not create them, so the columns the
	// table has are the columns the schema still declares.
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_config_json) VALUES ($1, $2, $3)`,
		execKey, session, string(raw)); err != nil {
		t.Fatalf("storing the config of session %s: %v", session, err)
	}
}

// Row 2 over rows the worker's own writers produced. The three cases are the
// whole of the predicate: a healthy step, a collapsed one, and one whose input
// is too small for a ratio to be evidence.
func TestVolumeCollapseOverRowsTheCpipesWorkerWrote(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-2 * time.Hour)
	session := client + "_run"

	ek := header(t, pool, client, "p", "obj", session, "completed", start)
	healthy := workerRow(t, pool, ek, client, "p", session, "reducing01", 0, StatusCompleted, 1000, 1000, start)
	collapsed := workerRow(t, pool, ek, client, "p", session, "reducing01", 1, StatusCompleted, 1000, 3, start)
	tiny := workerRow(t, pool, ek, client, "p", session, "reducing01", 2, StatusCompleted, 10, 0, start)

	ev, err := Gather(context.Background(), pool, Window{Since: start.Add(-time.Minute), Client: client}, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := DefaultVolumeCollapse().Detect(ev)
	if len(found) != 1 {
		t.Fatalf("got %d anomalies, want 1: %+v", len(found), found)
	}
	a := found[0]
	if a.SubjectRef != itoa(collapsed) {
		t.Errorf("subject is worker %s; healthy is %d and the sub-threshold one is %d",
			a.SubjectRef, healthy, tiny)
	}
	if a.SignalType != SignalVolume || a.SubjectType != SubjectWorker {
		t.Errorf("signal/subject = %s/%s", a.SignalType, a.SubjectType)
	}
	if a.ObservedValue != "3" {
		t.Errorf("observed value = %q, want the output count", a.ObservedValue)
	}
	// Row 2 is a within-run predicate and it does carry a range and a
	// magnitude, which is the opposite of what I-126 classified it as: the
	// expectation comes from the other column of the same row.
	if a.ExpectedMin == nil || *a.ExpectedMin != "500" {
		t.Errorf("expected minimum = %v, want half of the input count", a.ExpectedMin)
	}
	if a.ExpectedMax != nil {
		t.Errorf("expected maximum = %v; there is no upper bound at which an output count is anomalous", *a.ExpectedMax)
	}
	if a.DeviationMagnitude == nil || *a.DeviationMagnitude < 0.99 {
		t.Errorf("deviation magnitude = %v, want the shortfall", a.DeviationMagnitude)
	}
	// The channel table is not installed by this suite, and is deployed
	// nowhere, so nothing can say which edge lost the rows.
	if !contains(a.Confounders, ConfounderCrossStepJoinUnavailable) {
		t.Errorf("confounders %v do not say the DAG edge is unavailable", a.Confounders)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("the anomaly does not validate: %v", err)
	}
}

// F99: the six counts are NULL for exactly as long as the worker is running,
// and that is row 4's observation rather than row 2's. A coalesce written for
// tidiness would make every stalled worker a total collapse.
func TestVolumeCollapseIsSilentOnAWorkerStillRunning(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-2 * time.Hour)
	session := client + "_run"

	ek := header(t, pool, client, "p", "obj", session, "completed", start)
	workerRow(t, pool, ek, client, "p", session, "reducing01", 0, "", 0, 0, start)

	ev, err := Gather(context.Background(), pool, Window{Since: start.Add(-time.Minute), Client: client}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Workers.Rows) != 1 {
		t.Fatalf("got %d worker rows, want the one the insert wrote", len(ev.Workers.Rows))
	}
	if r := ev.Workers.Rows[0]; r.Status != StatusInProgress || r.InputRecords != nil || r.OutputRecords != nil {
		t.Fatalf("the insert alone should leave 'in progress' and NULL counts: %+v", r)
	}
	if found := DefaultVolumeCollapse().Detect(ev); len(found) != 0 {
		t.Errorf("a worker that never reported counts produced %d volume anomalies", len(found))
	}
	// And it is not invisible: it is row 4's, which the extraction already
	// answers and which N.4 did not build a detector for.
	if !ev.Workers.Rows[0].Stalled() {
		t.Error("the row is not reported as a stall either, so it is invisible to both")
	}
}

// A collapsed step whose failure is already reported by its status does not get
// a second signal beside it.
func TestVolumeCollapseIsSilentOnAFailedWorker(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-2 * time.Hour)
	session := client + "_run"

	ek := header(t, pool, client, "p", "obj", session, "failed", start)
	workerRow(t, pool, ek, client, "p", session, "reducing01", 0, StatusFailed, 1000, 0, start)

	ev, err := Gather(context.Background(), pool, Window{Since: start.Add(-time.Minute), Client: client}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if found := DefaultVolumeCollapse().Detect(ev); len(found) != 0 {
		t.Errorf("a failed worker produced %d volume anomalies beside its own status", len(found))
	}
}

// I-168's first honest answer: read the configuration and say what was found,
// with the limits of what the record kept.
func TestVolumeCollapseCarriesTheConfigurationsConfounders(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-2 * time.Hour)
	session := client + "_run"

	ek := header(t, pool, client, "p", "obj", session, "completed", start)
	workerRow(t, pool, ek, client, "p", session, "reducing01", 0, StatusCompleted, 1000, 3, start)
	storeConfig(t, pool, ek, session, embedConfigPath)

	ev, err := Gather(context.Background(), pool, Window{Since: start.Add(-time.Minute), Client: client}, 0)
	if err != nil {
		t.Fatal(err)
	}
	cc := ev.Configs[session]
	if cc == nil || !cc.Read {
		t.Fatalf("the shipped configuration was not read: %+v", cc)
	}
	found := DefaultVolumeCollapse().Detect(ev)
	if len(found) != 1 {
		t.Fatalf("got %d anomalies, want 1", len(found))
	}
	a := found[0]
	// embed_input_parts.pc.json writes through a csv_writer, so an output count
	// can be low without a row having been lost.
	if !contains(a.Confounders, ConfounderDeviceWriterOutput) {
		t.Errorf("confounders %v do not carry the device writer the config names", a.Confounders)
	}
	// Its on_error is "fail", not "drop", and the scan must not read the key's
	// presence as the behaviour.
	if contains(a.Confounders, ConfounderOnErrorDrop) {
		t.Errorf("confounders %v claim on_error: drop against a config that says fail", a.Confounders)
	}
	// parquet_input is in the configuration's namespace and does not bear on
	// this signal, so it is never on a volume anomaly whatever the config says.
	if contains(a.Confounders, ConfounderParquetInput) {
		t.Errorf("confounders %v carry an input-format qualifier on a volume signal", a.Confounders)
	}
	for _, c := range a.Confounders {
		if !slices.Contains(confounders, c) {
			t.Errorf("%q is not in the vocabulary", c)
		}
	}
}

// I-168's second honest answer, and the one it says must never be silent: a run
// that failed at sharding validation leaves no configuration at all, and the
// anomaly has to say the check was not made.
func TestAnUnreadConfigurationIsSaidRatherThanOmitted(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	start := time.Now().Add(-2 * time.Hour)
	session := client + "_run"

	ek := header(t, pool, client, "p", "obj", session, "completed", start)
	workerRow(t, pool, ek, client, "p", session, "reducing01", 0, StatusCompleted, 1000, 3, start)

	ev, err := Gather(context.Background(), pool, Window{Since: start.Add(-time.Minute), Client: client}, 0)
	if err != nil {
		t.Fatal(err)
	}
	cc := ev.Configs[session]
	if cc == nil {
		t.Fatal("a session with no configuration row produced no report at all, which is the silence I-168 forbids")
	}
	if cc.Read {
		t.Fatal("a session with no configuration row reports that it read one")
	}
	if cc.Note == "" {
		t.Fatal("the report says nothing about why")
	}
	found := DefaultVolumeCollapse().Detect(ev)
	if len(found) != 1 {
		t.Fatalf("got %d anomalies, want 1", len(found))
	}
	basis := found[0].ExpectedBasis
	if !containsSub(basis, "not checked") {
		t.Errorf("the basis does not say the configuration was not checked: %q", basis)
	}
	if contains(found[0].Confounders, ConfounderOnErrorDrop) ||
		contains(found[0].Confounders, ConfounderDeviceWriterOutput) {
		t.Errorf("an unread configuration produced a positive confounder: %v", found[0].Confounders)
	}
}

// The scan over the two configurations JetStore ships. No database: this is the
// half of the confounder question that is about reading a document.
func TestConfigConfounderScanOverShippedConfigs(t *testing.T) {
	cases := []struct {
		path     string
		want     []string
		wantNot  []string
		whatItIs string
	}{
		{
			path:     embedConfigPath,
			want:     []string{ConfounderDeviceWriterOutput},
			wantNot:  []string{ConfounderOnErrorDrop, ConfounderParquetInput, ConfounderMaxInputCount},
			whatItIs: "a csv_writer partition writer, on_error: fail, format: csv",
		},
		{
			path:     loaderConfigPath,
			want:     []string{ConfounderSamplingCap},
			wantNot:  []string{ConfounderDeviceWriterOutput, ConfounderOnErrorDrop, ConfounderParquetInput},
			whatItIs: "sampling_max_count 5000 on an input channel",
		},
	}
	for _, c := range cases {
		raw, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("reading %s: %v", c.path, err)
		}
		var doc any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parsing %s: %v", c.path, err)
		}
		got := scanConfigConfounders(doc)
		for _, w := range c.want {
			if !contains(got, w) {
				t.Errorf("%s (%s): %v does not carry %s", c.path, c.whatItIs, got, w)
			}
		}
		for _, w := range c.wantNot {
			if contains(got, w) {
				t.Errorf("%s (%s): %v wrongly carries %s", c.path, c.whatItIs, got, w)
			}
		}
	}
}

// Row 3, against a baseline that ends before the run it judges.
func TestStepRegressionNeedsAPriorSuccessAndAHistory(t *testing.T) {
	pool := testPool(t)
	client := uniq(t)
	windowOpens := time.Now().Add(-time.Hour)
	old := windowOpens.Add(-72 * time.Hour)

	// Four prior runs of the same source and step, all completed.
	for i := range 4 {
		s := client + "_ok" + itoa(int64(i))
		ek := header(t, pool, client, "p", "obj", s, "completed", old.Add(time.Duration(i)*time.Hour))
		workerRow(t, pool, ek, client, "p", s, "reducing01", 0, StatusCompleted, 100, 100,
			old.Add(time.Duration(i)*time.Hour))
	}
	// A step of the same source that has only ever failed: a broken
	// configuration rather than a regression.
	for i := range 4 {
		s := client + "_neverok" + itoa(int64(i))
		ek := header(t, pool, client, "p", "obj", s, "failed", old.Add(time.Duration(i)*time.Hour))
		workerRow(t, pool, ek, client, "p", s, "reducing09", 0, StatusFailed, 100, 0,
			old.Add(time.Duration(i)*time.Hour))
	}
	// And a step whose history is one run: not enough to have a normal.
	shallow := client + "_shallow"
	ek := header(t, pool, client, "p", "obj", shallow, "completed", old)
	workerRow(t, pool, ek, client, "p", shallow, "reducing05", 0, StatusCompleted, 100, 100, old)

	// The run being judged: all three steps fail.
	now := windowOpens.Add(10 * time.Minute)
	s := client + "_now"
	ekNow := header(t, pool, client, "p", "obj", s, "failed", now)
	regressed := workerRow(t, pool, ekNow, client, "p", s, "reducing01", 0, StatusFailed, 100, 0, now)
	workerRow(t, pool, ekNow, client, "p", s, "reducing09", 0, StatusFailed, 100, 0, now)
	workerRow(t, pool, ekNow, client, "p", s, "reducing05", 0, StatusFailed, 100, 0, now)

	ev, err := Gather(context.Background(), pool,
		Window{Since: windowOpens, Client: client}, 96*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	found, err := DefaultStepRegression().Detect(ev)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("got %d anomalies, want 1 (the step that had been working): %+v", len(found), found)
	}
	a := found[0]
	if a.SubjectRef != itoa(regressed) {
		t.Errorf("subject is worker %s, want %d", a.SubjectRef, regressed)
	}
	if a.SignalType != SignalStepRegression {
		t.Errorf("signal type = %q", a.SignalType)
	}
	if a.ObservedValue != StatusFailed {
		t.Errorf("observed value = %q", a.ObservedValue)
	}
	// A status compared against a history of statuses has no range to be
	// outside of and no distance to measure. I-126 made the three properties
	// optional for the within-run rows; this is a windowed row that needs the
	// same permission, and row 2 above is a within-run row that does not.
	if a.ExpectedMin != nil || a.ExpectedMax != nil || a.DeviationMagnitude != nil {
		t.Errorf("a categorical comparison invented a number: %v %v %v",
			a.ExpectedMin, a.ExpectedMax, a.DeviationMagnitude)
	}
	if !containsSub(a.ExpectedBasis, "completed") {
		t.Errorf("the basis does not say what the comparison was against: %q", a.ExpectedBasis)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("the anomaly does not validate: %v", err)
	}
}

// The non-overlap rule, which is enforced rather than documented: a baseline
// containing the run being judged makes a run its own precedent.
func TestStepRegressionRefusesABaselineContainingTheRun(t *testing.T) {
	now := time.Now()
	ev := &Evidence{
		Workers: &WorkerSet{Window: Window{Since: now.Add(-time.Hour)}},
		Prior:   &BaselineSet{Window: Window{Since: now.Add(-72 * time.Hour), Until: now}},
	}
	if _, err := DefaultStepRegression().Detect(ev); err == nil {
		t.Fatal("an overlapping baseline was accepted")
	}
}

// Gather is what makes that rule unfailable by accident.
func TestGatherClosesTheBaselineAtTheDetectionWindow(t *testing.T) {
	pool := testPool(t)
	since := time.Now().Add(-time.Hour)
	ev, err := Gather(context.Background(), pool,
		Window{Since: since, Client: uniq(t), SessionId: "one_run"}, 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !ev.Prior.Window.Until.Equal(since) {
		t.Errorf("baseline window ends at %s, want the detection window's start %s",
			ev.Prior.Window.Until, since)
	}
	if ev.Prior.Window.SessionId != "" {
		t.Errorf("the baseline inherited the session filter %q; a baseline restricted to the run "+
			"being judged is not a baseline", ev.Prior.Window.SessionId)
	}
}

// A scheduled detector runs over overlapping windows, so the identifier both
// detectors compose has to be what makes a re-run idempotent.
func TestInsertAnomalyIfNewIsIdempotentAcrossRuns(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	a := &Anomaly{
		AnomalyId:     uniq(t) + "_idem",
		SessionId:     "s",
		SubjectType:   SubjectWorker,
		SubjectRef:    "1",
		SignalType:    SignalVolume,
		ObservedValue: "0",
		ExpectedBasis: "b",
		DetectorRef:   "volume_collapse@1",
	}
	first, err := InsertAnomalyIfNew(ctx, pool, a)
	if err != nil || !first {
		t.Fatalf("first insert: new=%v err=%v", first, err)
	}
	second, err := InsertAnomalyIfNew(ctx, pool, a)
	if err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if second {
		t.Error("a re-run over the same rows wrote a second anomaly for the same worker")
	}
}

// Criterion 34, end to end: both detectors, over rows the worker's own writers
// produced, each emitting an Anomaly that persists and reads back.
func TestBothDetectorsEmitAPersistedAnomaly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	client := uniq(t)
	windowOpens := time.Now().Add(-time.Hour)
	old := windowOpens.Add(-72 * time.Hour)

	for i := range 4 {
		s := client + "_ok" + itoa(int64(i))
		ek := header(t, pool, client, "p", "obj", s, "completed", old.Add(time.Duration(i)*time.Hour))
		workerRow(t, pool, ek, client, "p", s, "reducing01", 0, StatusCompleted, 5000, 5000,
			old.Add(time.Duration(i)*time.Hour))
	}
	now := windowOpens.Add(10 * time.Minute)
	s := client + "_now"
	ek := header(t, pool, client, "p", "obj", s, "failed", now)
	storeConfig(t, pool, ek, s, loaderConfigPath)
	workerRow(t, pool, ek, client, "p", s, "reducing01", 0, StatusFailed, 5000, 0, now)
	workerRow(t, pool, ek, client, "p", s, "reducing02", 1, StatusCompleted, 5000, 4, now)

	ev, err := Gather(ctx, pool, Window{Since: windowOpens, Client: client}, 96*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	found := DefaultVolumeCollapse().Detect(ev)
	reg, err := DefaultStepRegression().Detect(ev)
	if err != nil {
		t.Fatal(err)
	}
	found = append(found, reg...)
	if len(found) != 2 {
		t.Fatalf("got %d anomalies, want one from each detector: %+v", len(found), found)
	}
	for i := range found {
		isNew, err := InsertAnomalyIfNew(ctx, pool, &found[i])
		if err != nil {
			t.Fatalf("persisting %s: %v", found[i].AnomalyId, err)
		}
		if !isNew {
			t.Errorf("%s was already recorded", found[i].AnomalyId)
		}
	}

	rows, err := pool.Query(ctx, `SELECT anomaly_signal_type, anomaly_detector_ref, anomaly_confounders,
		anomaly_expected_basis FROM jetsapi.anomaly WHERE anomaly_session_id = $1 ORDER BY anomaly_signal_type`, s)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var signals, refs []string
	for rows.Next() {
		var sig, ref, basis string
		var conf []string
		if err := rows.Scan(&sig, &ref, &conf, &basis); err != nil {
			t.Fatal(err)
		}
		signals = append(signals, sig)
		refs = append(refs, ref)
		if basis == "" {
			t.Errorf("%s round-tripped with no expected_basis", sig)
		}
		for _, c := range conf {
			if !slices.Contains(confounders, c) {
				t.Errorf("%s round-tripped with %q, outside the vocabulary", sig, c)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 2 || signals[0] != SignalStepRegression || signals[1] != SignalVolume {
		t.Fatalf("read back %v, want one step_regression and one volume", signals)
	}
	if refs[0] == refs[1] {
		t.Errorf("both anomalies name the same detector %q, so nothing distinguishes them", refs[0])
	}
	// The confounder the sampling cap in jets_loader.pc.json is there to
	// supply: the volume anomaly must say it could not rule out a capped read.
	var volumeConf []string
	for i := range found {
		if found[i].SignalType == SignalVolume {
			volumeConf = found[i].Confounders
		}
	}
	if !contains(volumeConf, ConfounderSamplingCap) {
		t.Errorf("the volume anomaly %v does not carry the sampling cap its run's config configures", volumeConf)
	}
}

// The deployment precondition, asked of a database where nothing has been
// migrated. Section 18.7 says ReadExtent "reports both tables' presence rather
// than assuming, which makes the precondition checkable at run time instead of
// diagnosable afterwards"; before N.4 that held only where the execution tables
// already existed, because the two to_regclass calls shared a statement with a
// count over pipeline_execution_status. On an unmigrated database the query
// failed on that relation and reported nothing at all — which is the failure
// the sentence promises to prevent, arriving one table earlier.
func TestReadExtentAnswersOnAnUnmigratedDatabase(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// A database of its own rather than a schema trick, so nothing this test
	// does can be seen by the rest of the suite.
	name := "unmigrated_" + uniq(t)
	if _, err := pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Skipf("cannot create a database to simulate an unmigrated deployment: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name) })

	cfg, err := pgxpool.ParseConfig(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	bare, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer bare.Close()

	e, err := ReadExtent(ctx, bare)
	if err != nil {
		t.Fatalf("ReadExtent could not answer on an unmigrated database: %v", err)
	}
	if e.ExecutionRecord || e.ChannelDetails || e.Anomalies {
		t.Errorf("an unmigrated database reported tables: %+v", e)
	}
	if e.Regime() != "unknown" {
		t.Errorf("Regime() = %q over no record at all", e.Regime())
	}
}

// I-127's extensions, confirmed by the only test there is: a caller that has to
// choose a value. Section A.2.6's ten signal types can name row 2's signal and
// cannot name row 3's, and neither of its six subject types is a worker.
func TestTheDetectorsUseTheExtendedVocabularies(t *testing.T) {
	appendixSignals := []string{"volume", "freshness", "schema", "distribution", "rule_breach",
		"cost", "duration", "rejection_rate", "cardinality", "referential"}
	appendixSubjects := []string{"feed", "pipeline", "stage", "run", "table", "column"}

	if !slices.Contains(appendixSignals, SignalVolume) {
		t.Error("row 2's signal is not one of the appendix's ten, which it should be")
	}
	if slices.Contains(appendixSignals, SignalStepRegression) {
		t.Error("row 3's signal is in the appendix's ten, so the extension was not needed")
	}
	if slices.Contains(appendixSubjects, SubjectWorker) {
		t.Error("the worker grain is in the appendix's six, so the extension was not needed")
	}
	// And both are in the vocabularies the generated CHECK constraints carry,
	// which TestVocabulariesMatchDDL asserts against the DDL itself.
	for _, v := range []string{SignalVolume, SignalStepRegression} {
		if !slices.Contains(signalTypes, v) {
			t.Errorf("%q is not in the signal vocabulary", v)
		}
	}
	if !slices.Contains(subjectTypes, SubjectWorker) {
		t.Error("the worker subject is not in the subject vocabulary")
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

func containsSub(s, sub string) bool { return strings.Contains(s, sub) }
