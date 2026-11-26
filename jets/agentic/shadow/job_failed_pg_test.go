package shadow

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A job's failures, against a real Postgres: JetStore's jobs and a workspace's
// custom Lambdas record a failure as one pipeline_execution_status row with no
// pipeline configuration, no input session and status failed, under a session
// reserved and registered for it with source type job. The rows here are
// planted in that shape rather than written by the helper that writes them, so
// this suite tests triage's reading of the row and nothing else.

var jobTables = append(slices.Clone(migratedTables), "session_registry")

var jobDay = time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)

// plantJobFailure writes a job's failure row and its session's registration,
// days after jobDay.
func plantJobFailure(t *testing.T, pool *pgxpool.Pool, session, client, process string, days int) {
	t.Helper()
	ctx := context.Background()
	start := jobDay.AddDate(0, 0, days)
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.session_registry
		(session_id, source_type, client, source_period_key, month_period, week_period, day_period, last_update)
		VALUES ($1, 'job', $2, 1, 1, 1, 1, $3)`, session, client, start); err != nil {
		t.Fatalf("registering job session %s: %v", session, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, main_input_registry_key, main_input_file_key, merged_input_registry_keys,
		 client, process_name, main_object_type, input_session_id, session_id, source_period_key,
		 status, failure_details, failure_class, failure_source, user_email, start_time, last_update)
		VALUES (0, NULL, NULL, '{}', $1, $2, '', NULL, $3, 1,
		        'failed', 'the purge failed: connection reset', 'job_failed', 'job', 'system', $4, $4)`,
		client, process, session, start); err != nil {
		t.Fatalf("inserting job row %s: %v", session, err)
	}
}

// plantSentinelFailure writes a failure on the legacy shared session, as a
// workspace's custom Lambdas did before a job's failure had a session of its own.
func plantSentinelFailure(t *testing.T, pool *pgxpool.Pool, process string, start time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, main_input_registry_key, merged_input_registry_keys, client, process_name,
		 main_object_type, input_session_id, session_id, source_period_key, status, failure_details,
		 user_email, start_time, last_update)
		VALUES (0, 0, '{}', 'client2', $1, 'claims', $2, $2, 1, 'failed', 'no data files', 'system', $3, $3)`,
		process, triage.SentinelSession, start); err != nil {
		t.Fatalf("inserting a sentinel row: %v", err)
	}
}

func incidentsAtJobFailed(t *testing.T, pool *pgxpool.Pool) []string {
	return incidentsAt(t, pool, triage.LocusJobFailed, "")
}

// One failure is one incident at job_failed, and the nine run loci say
// nothing about a job: run_not_started, which a terminal header with no worker
// row reads as, does not fire.
func TestAJobFailureRaisesOneJobFailedIncident(t *testing.T) {
	pool := freshDB(t, "shadow_job_one", jobTables, true)
	ctx := context.Background()
	w := testWriter()
	plantJobFailure(t, pool, "job-01", "", "purge_data", 0)

	res := run(t, pool, &w, "job-01")
	want := IncidentIdFor("job-01", triage.LocusJobFailed)
	if len(res.Written) != 1 || res.Written[0].IncidentId != want {
		t.Fatalf("a job's failure wrote %v; want one incident, %s", res.Written, want)
	}
	if !res.Report.Job {
		t.Error("the report of a job's row is not marked Job")
	}
	for _, f := range res.Report.Findings {
		switch {
		case f.Locus == triage.LocusJobFailed && f.Verdict != triage.Present:
			t.Errorf("job_failed is %s: %s", f.Verdict, f.Basis)
		case triage.IsPipelineRunLocus(f.Locus) && f.Verdict != triage.NotEvaluable:
			t.Errorf("run locus %s on a job's row is %s: %s", f.Locus, f.Verdict, f.Basis)
		}
	}
	if got := incidentsAt(t, pool, triage.LocusRunNotStarted, ""); len(got) != 0 {
		t.Errorf("a job's failure raised run_not_started incidents %v", got)
	}
	// The incident carries the job's key, an empty client included, and names
	// the job in its basis; no hypothesis, because no cause class maps to it.
	var client, process, status string
	if err := pool.QueryRow(ctx, `SELECT incident_client, incident_process_name, status
		FROM jetsapi.incident WHERE incident_id = $1`, want).Scan(&client, &process, &status); err != nil ||
		client != "" || process != "purge_data" || status != audit.IncidentTriaged {
		t.Errorf("incident %s carries %q/%q at %s (%v)", want, client, process, status, err)
	}
	if b := basisOf(t, pool, want); !strings.Contains(b, "job purge_data") {
		t.Errorf("the incident's basis does not name the job:\n%s", b)
	}
	// Only the job's own locus is stored for its session.
	var loci []string
	rows, err := pool.Query(ctx, `SELECT locus FROM jetsapi.run_finding WHERE session_id = 'job-01'`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var l string
		_ = rows.Scan(&l)
		loci = append(loci, l)
	}
	rows.Close()
	if !slices.Equal(loci, []string{triage.LocusJobFailed}) {
		t.Errorf("the job's stored findings are at %v, want job_failed alone", loci)
	}

	// A second pass writes nothing.
	if res = run(t, pool, &w, "job-01"); len(res.Written) != 0 {
		t.Errorf("a second pass wrote %v", res.Written)
	}
}

// A job failing on three days is one condition of three runs and one
// incident; after a person's verdict, a later failure is a new incident naming
// the judged one. Another job, or the same job for another client, is a
// condition of its own.
func TestAJobFailingOnThreeDaysIsOneIncident(t *testing.T) {
	pool := freshDB(t, "shadow_job_condition", jobTables, true)
	ctx := context.Background()
	w := testWriter()
	for i, s := range []string{"job-d1", "job-d2", "job-d3"} {
		plantJobFailure(t, pool, s, "", "purge_data", i)
	}
	plantJobFailure(t, pool, "reg-d2", "client2", "register_keys", 1)

	first := IncidentIdFor("job-d1", triage.LocusJobFailed)
	if res := run(t, pool, &w, "job-d1"); len(res.Written) != 1 || res.Written[0].IncidentId != first {
		t.Fatalf("the first failure wrote %v; want %s", res.Written, first)
	}
	for _, s := range []string{"job-d2", "job-d3"} {
		res := run(t, pool, &w, s)
		if len(res.Written) != 0 || !grouped(res, first) {
			t.Errorf("%s: written %v, grouped %v; want joined to %s", s, res.Written, res.Grouped, first)
		}
	}
	if res := run(t, pool, &w, "reg-d2"); len(res.Written) != 1 ||
		res.Written[0].IncidentId != IncidentIdFor("reg-d2", triage.LocusJobFailed) {
		t.Errorf("another job's failure wrote %v; want an incident of its own", res.Written)
	}
	if got := incidentsAtJobFailed(t, pool); len(got) != 2 {
		t.Errorf("incidents at job_failed: %v, want two (one per job)", got)
	}

	h, err := runcondition.Read(ctx, pool, runcondition.Key{Client: "", Process: "purge_data",
		Locus: triage.LocusJobFailed})
	if err != nil {
		t.Fatal(err)
	}
	cs := h.Conditions()
	if len(cs) != 1 || len(cs[0].Members) != 3 || !cs[0].Open() {
		t.Fatalf("the purge's conditions are %+v; want one open condition of three runs", cs)
	}
	if c, members := h.Members(first); c == nil || len(members) != 3 {
		t.Errorf("incident %s stands for %d runs, want 3", first, len(members))
	}

	// A person's verdict on the condition's incident, then a fourth failure:
	// a job records no success, so nothing else ends its condition.
	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, first).Scan(&status)
	verdictAt := fixtureNow.Add(time.Hour)
	if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
		IncidentEventId: "ie_person_job_benign", IncidentRef: first, FromStatus: status,
		ToStatus: audit.IncidentSuppressedAsBenign, Actor: "reviewer@example.com", ActorKind: audit.ActorHuman,
		TransitionedAt: verdictAt, Rationale: "a known setting, being fixed",
	}); err != nil {
		t.Fatal(err)
	}
	w.Now = func() time.Time { return verdictAt.Add(time.Hour) }
	plantJobFailure(t, pool, "job-d4", "", "purge_data", 3)
	fourth := IncidentIdFor("job-d4", triage.LocusJobFailed)
	res := run(t, pool, &w, "job-d4")
	if len(res.Written) != 1 || res.Written[0].IncidentId != fourth {
		t.Fatalf("a failure after the verdict wrote %v, grouped %v; want %s", res.Written, res.Grouped, fourth)
	}
	b := basisOf(t, pool, fourth)
	for _, want := range []string{"is new evidence", "The previous incident at this key is " + first,
		"suppressed_as_benign by reviewer@example.com"} {
		if !strings.Contains(b, want) {
			t.Errorf("the incident after the verdict does not say %q:\n%s", want, b)
		}
	}
}

// A header on the legacy shared session raises nothing, and is counted: the
// run selection leaves it out and says how many it left, and the writer
// skips it when it is named.
func TestTheSentinelSessionIsSkippedAndCounted(t *testing.T) {
	pool := freshDB(t, "shadow_job_sentinel", jobTables, true)
	ctx := context.Background()
	w := testWriter()
	plantSentinelFailure(t, pool, "loader", fixtureNow.Add(-2*time.Hour))
	plantSentinelFailure(t, pool, "deid", fixtureNow.Add(-time.Hour))
	failedWorkerSession(t, pool, "run-ordinary")

	sel, err := SelectSessions(ctx, pool, fixtureNow.Add(-24*time.Hour), "", "", 100, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(sel.Sessions, triage.SentinelSession) {
		t.Errorf("the selection includes the shared session: %v", sel.Sessions)
	}
	if !slices.Contains(sel.Sessions, "run-ordinary") {
		t.Errorf("the selection lost an ordinary run: %v", sel.Sessions)
	}
	if sel.Sentinel != 2 {
		t.Errorf("the selection counted %d headers on the shared session, want 2", sel.Sentinel)
	}
	if sel, err = SelectSessions(ctx, pool, fixtureNow.Add(-24*time.Hour), "", "deid", 100, false, false); err != nil ||
		sel.Sentinel != 1 {
		t.Errorf("with a process filter the selection counted %d (%v), want 1", sel.Sentinel, err)
	}

	res := run(t, pool, &w, triage.SentinelSession)
	if !res.Sentinel || len(res.Written) != 0 || len(res.Report.Findings) != 0 {
		t.Errorf("the shared session was not skipped: sentinel %v, written %v, %d findings",
			res.Sentinel, res.Written, len(res.Report.Findings))
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.incident WHERE incident_session_id = $1`,
		triage.SentinelSession).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d incidents on the shared session (%v)", n, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.run_finding WHERE session_id = $1`,
		triage.SentinelSession).Scan(&n); err != nil || n != 0 {
		t.Errorf("%d findings stored for the shared session (%v)", n, err)
	}
}

// A pipeline run beside a job's failure is read as before: job_failed is not
// evaluable on it and stores no finding, and its own loci fire as they did.
func TestAPipelineRunIsNotAJob(t *testing.T) {
	pool := freshDB(t, "shadow_job_pipeline", jobTables, true)
	ctx := context.Background()
	w := testWriter()
	failedWorkerSession(t, pool, "run-01")
	plantJobFailure(t, pool, "job-01", "client2", "loader", 0)

	res := run(t, pool, &w, "run-01")
	if res.Report.Job {
		t.Error("a pipeline run's report is marked Job")
	}
	if !slices.Contains(res.Report.Loci(), triage.LocusWorkerFailed) ||
		slices.Contains(res.Report.Loci(), triage.LocusJobFailed) {
		t.Errorf("the pipeline run fired %v", res.Report.Loci())
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM jetsapi.run_finding WHERE session_id = 'run-01'
		AND locus = $1`, triage.LocusJobFailed).Scan(&n); err != nil || n != 0 {
		t.Errorf("a pipeline run stored %d findings at job_failed (%v)", n, err)
	}
	// The job shares the run's client and process name and is still a
	// condition of its own locus: it joins no run incident.
	res = run(t, pool, &w, "job-01")
	if len(res.Written) != 1 || res.Written[0].Locus != triage.LocusJobFailed || len(res.Grouped) != 0 {
		t.Errorf("the job wrote %v, grouped %v; want one job_failed incident of its own", res.Written, res.Grouped)
	}
}
