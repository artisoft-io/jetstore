package shadow

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/errorcode"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run conditions, against a real Postgres: the consecutive runs of one process
// for one client with a finding at one run locus, and at
// per_record_failures_reported one error code, are one incident.

var runConditionTables = append(slices.Clone(migratedTables), "error_code_catalogue")

// A workspace code with a threshold of 3 stored rows per run, and JetStore's
// fixed mapping code, which reports at 1.
const (
	codeAmount  = "error amount is not numeric"
	codeMapping = errorcode.CodeMappingColumn
)

var runStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// plantRun writes one run of process loader for client2 starting hours after
// runStart: a worker that failed or completed, and the given rows of each code.
func plantRun(t *testing.T, pool *pgxpool.Pool, session string, hours float64, failed bool, amount, mapping int) {
	t.Helper()
	start := runStart.Add(time.Duration(hours * float64(time.Hour)))
	status, worker, out, msg := "completed", "completed", 1000, ""
	if failed {
		status, worker, out, msg = "failed", "failed", 0, "boom"
	}
	k := header(t, pool, "client2", "loader", "claims", session, status, start)
	workerRow(t, pool, k, "client2", "loader", session, "reducing01", 0, worker, 1000, out, msg, start)
	ctx := context.Background()
	for i := range amount {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, error_message, shard_id, operator_type)
			VALUES ($1, $2, $3, $4, 0, 'jetrules')`, k, session, fmt.Sprintf("a%d", i),
			errorcode.RuleExceptionPrefix+codeAmount+fmt.Sprintf(": %d.5", i)); err != nil {
			t.Fatal(err)
		}
	}
	for i := range mapping {
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, error_message, shard_id, operator_type)
			VALUES ($1, $2, $3, $4, 0, 'map_record')`, k, session, fmt.Sprintf("m%d", i),
			errorcode.MapRecordPrefix+fmt.Sprintf("error mapping column col%d at output position %d: bad", i, i)); err != nil {
			t.Fatal(err)
		}
	}
}

func runConditionDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	pool := freshDB(t, name, runConditionTables, true)
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.error_code_catalogue
		(operator_type, match_prefix, kind, report_threshold, source, source_ref)
		VALUES ('jetrules', $1, 'error', 3, 'rules', 'rules.jr amount_rule')`, codeAmount); err != nil {
		t.Fatal(err)
	}
	return pool
}

// deleteRun removes a run's session as jets/sessionpurge does: its header, its
// worker rows, its per-record errors and its stored verdicts. The incidents
// stay.
func deleteRun(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	for _, table := range []string{"run_finding", "process_errors", "pipeline_execution_details",
		"pipeline_execution_status"} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM jetsapi."+table+" WHERE session_id = $1", session); err != nil {
			t.Fatal(err)
		}
	}
}

func run(t *testing.T, pool *pgxpool.Pool, w *Writer, session string) *SessionResult {
	t.Helper()
	res, err := w.Run(context.Background(), pool, session)
	if err != nil {
		t.Fatalf("Run %s: %v", session, err)
	}
	return res
}

// atKeys keeps the incidents written at the three keys the fixture is about.
func atKeys(ws []IncidentWritten) []string {
	var out []string
	for _, w := range ws {
		if w.Locus == triage.LocusWorkerFailed || w.Locus == triage.LocusPerRecordFailuresReported {
			out = append(out, w.IncidentId)
		}
	}
	slices.Sort(out)
	return out
}

func grouped(res *SessionResult, into string) bool {
	return slices.ContainsFunc(res.Grouped, func(g string) bool { return strings.Contains(g, "(into "+into+")") })
}

func incidentsAt(t *testing.T, pool *pgxpool.Pool, locus, code string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT incident_id FROM jetsapi.incident
		WHERE incident_locus = $1 AND coalesce(incident_error_code, '') = $2 ORDER BY incident_id`, locus, code)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

func TestARunConditionIsOneIncident(t *testing.T) {
	pool := runConditionDB(t, "shadow_run_condition")
	ctx := context.Background()
	w := testWriter()
	wf := triage.LocusWorkerFailed
	pr := triage.LocusPerRecordFailuresReported

	// r1..r4 fail with both codes, r2's amount rows below its threshold;
	// r5 is clean; r6 and r7 fail again.
	plantRun(t, pool, "run-01", 1, true, 3, 2)
	plantRun(t, pool, "run-02", 2, true, 1, 1)
	plantRun(t, pool, "run-03", 3, true, 3, 1)
	plantRun(t, pool, "run-04", 4, true, 4, 1)
	plantRun(t, pool, "run-05", 5, false, 0, 0)
	plantRun(t, pool, "run-06", 6, true, 3, 1)
	plantRun(t, pool, "run-07", 7, true, 3, 1)

	i1 := IncidentIdFor("run-01", wf)
	i1a := IncidentIdForCode("run-01", pr, codeAmount)
	i1m := IncidentIdForCode("run-01", pr, codeMapping)
	if i1a != "inc_run-01_per_record_failures_reported_error_amount_is_not_numeric" ||
		i1m != "inc_run-01_per_record_failures_reported_map_record_error_mapping_column" {
		t.Fatalf("the per-record ids are %s and %s", i1a, i1m)
	}
	res := run(t, pool, &w, "run-01")
	if got := atKeys(res.Written); !slices.Equal(got, []string{i1a, i1m, i1}) {
		t.Fatalf("the first run wrote %v; want one incident per key: %v", got, []string{i1a, i1m, i1})
	}
	// The second code on the same run is a second incident, each naming its code.
	var code string
	if err := pool.QueryRow(ctx, `SELECT incident_error_code FROM jetsapi.incident WHERE incident_id = $1`, i1a).
		Scan(&code); err != nil || code != codeAmount {
		t.Errorf("incident %s carries code %q (%v)", i1a, code, err)
	}
	var client, process string
	var start time.Time
	if err := pool.QueryRow(ctx, `SELECT incident_client, incident_process_name, incident_anchor_start_time
		FROM jetsapi.incident WHERE incident_id = $1`, i1).Scan(&client, &process, &start); err != nil ||
		client != "client2" || process != "loader" || !start.Equal(runStart.Add(time.Hour)) {
		t.Errorf("incident %s carries key %s/%s at %v (%v)", i1, client, process, start, err)
	}

	// r2 joins at both of its reporting keys; its amount code is quiet and is
	// placed nowhere. r3 and r4 join at all three.
	res = run(t, pool, &w, "run-02")
	if len(atKeys(res.Written)) != 0 || !grouped(res, i1) || !grouped(res, i1m) || grouped(res, i1a) {
		t.Errorf("run-02: written %v, grouped %v; want joined to %s and %s only", res.Written, res.Grouped, i1, i1m)
	}
	for _, s := range []string{"run-03", "run-04"} {
		res = run(t, pool, &w, s)
		if len(atKeys(res.Written)) != 0 || !grouped(res, i1) || !grouped(res, i1a) || !grouped(res, i1m) {
			t.Errorf("%s: written %v, grouped %v; want joined to the run-01 incidents", s, res.Written, res.Grouped)
		}
	}
	// r5 has no finding at any of them; r6 opens three new incidents, each
	// naming the previous one.
	if res = run(t, pool, &w, "run-05"); len(atKeys(res.Written)) != 0 || len(res.Grouped) != 0 {
		t.Errorf("run-05, clean: written %v, grouped %v", res.Written, res.Grouped)
	}
	i6 := IncidentIdFor("run-06", wf)
	i6a := IncidentIdForCode("run-06", pr, codeAmount)
	res = run(t, pool, &w, "run-06")
	if got := atKeys(res.Written); len(got) != 3 || !slices.Contains(got, i6) || !slices.Contains(got, i6a) {
		t.Fatalf("run-06, after the clean run: wrote %v; want three new incidents", got)
	}
	if b := basisOf(t, pool, i6); !strings.Contains(b, "The previous incident at this key is "+i1) ||
		!strings.Contains(b, "with no verdict") {
		t.Errorf("run-06's incident does not name the previous one:\n%s", b)
	}

	// The quiet run is in no condition and ends none: the amount condition is
	// r1, r3, r4, with r2 skipped.
	h, err := runcondition.Read(ctx, pool, runcondition.Key{Client: "client2", Process: "loader", Locus: pr, Code: codeAmount})
	if err != nil {
		t.Fatal(err)
	}
	c, members := h.Members(i1a)
	if c == nil || !slices.Equal(c.Sessions(), []string{"run-01", "run-03", "run-04"}) || c.Skipped != 1 ||
		c.EndedBy == nil || c.EndedBy.SessionId != "run-05" || len(members) != 3 {
		t.Errorf("the amount condition: %+v, %d members", c, len(members))
	}
	var verdict string
	var stored int64
	if err := pool.QueryRow(ctx, `SELECT verdict, stored_rows FROM jetsapi.run_finding
		WHERE session_id = 'run-02' AND locus = $1 AND error_code = $2`, pr, codeAmount).Scan(&verdict, &stored); err != nil ||
		verdict != runcondition.VerdictQuiet || stored != 1 {
		t.Errorf("run-02's amount row is %s with %d rows (%v); want quiet with 1", verdict, stored, err)
	}

	// A person's verdict on r6's worker incident: r7's finding, found after
	// it, is a new incident naming it, and its other two keys join r6's.
	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM jetsapi.incident WHERE incident_id = $1`, i6).Scan(&status)
	verdictAt := fixtureNow.Add(time.Hour)
	if _, err := audit.RecordIncidentTransition(ctx, pool, &audit.IncidentTransition{
		IncidentEventId: "ie_person_benign_run", IncidentRef: i6, FromStatus: status,
		ToStatus: audit.IncidentSuppressedAsBenign, Actor: "reviewer@example.com", ActorKind: audit.ActorHuman,
		TransitionedAt: verdictAt, Rationale: "expected",
	}); err != nil {
		t.Fatal(err)
	}
	w.Now = func() time.Time { return verdictAt.Add(time.Hour) }
	i7 := IncidentIdFor("run-07", wf)
	res = run(t, pool, &w, "run-07")
	if got := atKeys(res.Written); !slices.Equal(got, []string{i7}) || !grouped(res, i6a) {
		t.Fatalf("run-07, after the verdict: wrote %v, grouped %v; want %s and the codes joined", got, res.Grouped, i7)
	}
	b := basisOf(t, pool, i7)
	for _, want := range []string{"is new evidence", "The previous incident at this key is " + i6,
		"suppressed_as_benign by reviewer@example.com"} {
		if !strings.Contains(b, want) {
			t.Errorf("run-07's incident basis does not say %q:\n%s", want, b)
		}
	}

	// A backfilled run between r2 and r3, classified last, joins the
	// condition it falls in, and its finding stored after the verdict on
	// another condition's incident changes nothing here.
	plantRun(t, pool, "run-02b", 2.5, true, 3, 1)
	if res = run(t, pool, &w, "run-02b"); len(atKeys(res.Written)) != 0 || !grouped(res, i1) || !grouped(res, i1a) {
		t.Errorf("the backfilled run: written %v, grouped %v; want joined to the run-01 incidents", res.Written, res.Grouped)
	}

	// A run deleted inside the condition neither ends nor splits it.
	deleteRun(t, pool, "run-03")
	if res = run(t, pool, &w, "run-04"); len(atKeys(res.Written)) != 0 || !grouped(res, i1) {
		t.Errorf("after deleting run-03, run-04: written %v, grouped %v; want joined to %s", res.Written, res.Grouped, i1)
	}

	// A condition whose first run is deleted keeps its incident: its runs,
	// and a new run inside it, join the incident and no second one is raised.
	deleteRun(t, pool, "run-01")
	plantRun(t, pool, "run-04b", 4.5, true, 3, 1)
	for _, s := range []string{"run-02", "run-04b"} {
		if res = run(t, pool, &w, s); len(atKeys(res.Written)) != 0 || !grouped(res, i1) {
			t.Errorf("after deleting run-01, %s: written %v, grouped %v; want joined to %s", s, res.Written, res.Grouped, i1)
		}
	}
	if got := incidentsAt(t, pool, wf, ""); !slices.Equal(got, []string{i1, i6, i7}) {
		t.Errorf("worker_failed incidents %v, want %v", got, []string{i1, i6, i7})
	}
	if got := incidentsAt(t, pool, pr, codeAmount); len(got) != 2 {
		t.Errorf("amount incidents %v, want two, run-01's and run-06's", got)
	}
	h, err = runcondition.Read(ctx, pool, runcondition.Key{Client: "client2", Process: "loader", Locus: wf})
	if err != nil {
		t.Fatal(err)
	}
	if c, members := h.Members(i1); c == nil || len(members) != 4 {
		t.Errorf("%s stands for %d live runs, want run-02, run-02b, run-04 and run-04b", i1, len(members))
	}

	// Idempotent: a second pass over every live run writes nothing.
	for _, s := range []string{"run-02", "run-02b", "run-04", "run-04b", "run-05", "run-06", "run-07"} {
		if res = run(t, pool, &w, s); len(res.Written) != 0 {
			t.Errorf("a second pass over %s wrote %v", s, res.Written)
		}
	}
	if a, err := Attest(ctx, pool); err != nil || !a.Holds() {
		t.Errorf("the attestation: %v\n%s", err, a.Report())
	}
}

// Only a condition's latest run triaged: the condition's incident is raised on
// its first run, which a dry run reports and does not write.
func TestOnlyTheLastRunTriagedRaisesTheFirst(t *testing.T) {
	pool := runConditionDB(t, "shadow_run_condition_anchor")
	ctx := context.Background()
	w := testWriter()
	wf := triage.LocusWorkerFailed
	plantRun(t, pool, "run-01", 1, true, 0, 0)
	plantRun(t, pool, "run-02", 2, true, 0, 0)
	plantRun(t, pool, "run-03", 3, true, 0, 0)
	// The earlier runs' verdicts are stored, as a run that classified them
	// without writing would; then only the last run is triaged for real.
	for _, s := range []string{"run-01", "run-02"} {
		r := w.Classifier
		ev, err := r.Gather(ctx, pool, s, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := storeRunFindings(ctx, pool, runcondition.RowsOf(r.Classify(ev), fixtureNow)); err != nil {
			t.Fatal(err)
		}
	}
	dry := testWriter()
	dry.DryRun = true
	res := run(t, pool, &dry, "run-03")
	i1 := IncidentIdFor("run-01", wf)
	if got := atKeys(res.Written); !slices.Equal(got, []string{i1}) || !grouped(res, i1) {
		t.Errorf("dry run over the last run: reports %v, grouped %v; want %s raised and joined", got, res.Grouped, i1)
	}
	if got := incidentsAt(t, pool, wf, ""); len(got) != 0 {
		t.Errorf("the dry run wrote %v", got)
	}
	res = run(t, pool, &w, "run-03")
	if got := atKeys(res.Written); !slices.Equal(got, []string{i1}) || !grouped(res, i1) {
		t.Errorf("the last run: wrote %v, grouped %v; want %s raised on the first run", got, res.Grouped, i1)
	}
	if b := basisOf(t, pool, i1); !strings.Contains(b, "It was raised when session run-03") {
		t.Errorf("the anchor's basis does not say what raised it:\n%s", b)
	}
	if res = run(t, pool, &w, "run-01"); len(res.Written) != 0 {
		t.Errorf("the first run, triaged after, wrote %v", res.Written)
	}
}

// A per-record incident written before codes stands for every code of its
// session, and run incidents written before run conditions stay as written.
func TestIncidentsWrittenBeforeCodesAreNeverRetracted(t *testing.T) {
	pool := runConditionDB(t, "shadow_run_condition_legacy")
	ctx := context.Background()
	w := testWriter()
	plantRun(t, pool, "run-01", 1, true, 3, 1)
	legacy := IncidentIdFor("run-01", triage.LocusPerRecordFailuresReported)
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version)
		VALUES ($1, 'run-01', $2, 'per_record_failures_reported', 'info', 'detected', '{}', '0.1.0')`,
		legacy, fixtureNow); err != nil {
		t.Fatal(err)
	}
	res := run(t, pool, &w, "run-01")
	for _, wr := range res.Written {
		if wr.Locus == triage.LocusPerRecordFailuresReported {
			t.Errorf("a per-record incident was written beside the one written before codes: %s", wr.IncidentId)
		}
	}
	n := 0
	for _, a := range res.AlreadyRaised {
		if strings.Contains(a, legacy) {
			n++
		}
	}
	if n != 2 {
		t.Errorf("the legacy incident stands for %d codes, want both: %v", n, res.AlreadyRaised)
	}
}

// No canary planted in a per-record message's variable tail reaches an
// incident, its events, its hypotheses, a stored verdict, any proposal, or the
// writer's own result.
func TestARuleExceptionTailReachesNoIncident(t *testing.T) {
	pool := runConditionDB(t, "shadow_run_condition_canary")
	ctx := context.Background()
	w := testWriter()
	const canary = "CANARY7731ZQ"
	start := runStart.Add(time.Hour)
	k := header(t, pool, "client2", "loader", "claims", "run-canary", "failed", start)
	workerRow(t, pool, k, "client2", "loader", "run-canary", "reducing01", 0, "failed", 1000, 0, "boom", start)
	for i, m := range []struct{ op, msg string }{
		{"jetrules", errorcode.RuleExceptionPrefix + codeAmount + ": " + canary},
		{"jetrules", errorcode.RuleExceptionPrefix + codeAmount + " " + canary},
		{"jetrules", errorcode.RuleExceptionPrefix + codeAmount + canary},
		{"jetrules", errorcode.RuleExceptionPrefix + "nothing catalogues this: " + canary},
		{"", errorcode.RuleExceptionPrefix + canary},
		{"map_record", errorcode.MapRecordPrefix + "error mapping column " + canary + " at output position 2: " + canary},
	} {
		var op any
		if m.op != "" {
			op = m.op
		}
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, input_column, error_message, shard_id, operator_type)
			VALUES ($1, 'run-canary', $2, $3, $4, 0, $5)`, k, fmt.Sprintf("k%d", i), canary, m.msg, op); err != nil {
			t.Fatal(err)
		}
	}
	res := run(t, pool, &w, "run-canary")
	if len(atKeys(res.Written)) < 3 {
		t.Fatalf("the canary run wrote %v; want per-record incidents to search", res.Written)
	}
	out, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	places := map[string]string{"the writer's result": string(out)}
	for _, table := range []string{"incident", "incident_event", "hypothesis", "remediation", "run_finding",
		"change_proposal", "approval_event", "agent_audit", "anomaly"} {
		rows, err := pool.Query(ctx, "SELECT row_to_json(t)::text FROM jetsapi."+table+" t")
		if err != nil {
			t.Fatalf("reading %s: %v", table, err)
		}
		var b strings.Builder
		n := 0
		for rows.Next() {
			var s string
			_ = rows.Scan(&s)
			b.WriteString(s)
			n++
		}
		rows.Close()
		places["jetsapi."+table] = b.String()
		if table == "incident" || table == "incident_event" || table == "run_finding" {
			if n == 0 {
				t.Errorf("jetsapi.%s holds no row, so the search proves nothing there", table)
			}
		}
	}
	for where, text := range places {
		if strings.Contains(text, canary) {
			t.Errorf("%s carries the canary", where)
		}
	}
	if !strings.Contains(places["jetsapi.incident"], codeAmount) {
		t.Error("the incidents do not name the catalogued code the canary rows carry")
	}
}
