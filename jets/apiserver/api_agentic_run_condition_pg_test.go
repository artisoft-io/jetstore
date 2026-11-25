package main

// A run incident's condition through the endpoint, against a real Postgres:
// get_incident, list_incidents and find_session_incidents read it as the
// shadow writer placed the runs, and none of them carries a message.
//
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:55892/postgres \
//	  go test -count=1 -run 'RunIncident|RunCondition|IncidentList' ./jets/apiserver/

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/errorcode"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// A workspace code with a threshold of 3 stored rows per run.
const runCodeAmount = "value is not numeric"

var runConditionStart = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// runConditionAPIDB is a database with the execution record, the catalogue
// holding runCodeAmount, and the agentic tables, and nothing written yet.
func runConditionAPIDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	for _, q := range []string{"DROP DATABASE IF EXISTS " + name + " WITH (FORCE)", "CREATE DATABASE " + name} {
		if _, err := admin.Exec(ctx, q); err != nil {
			admin.Close()
			t.Fatalf("%s: %v", q, err)
		}
	}
	admin.Close()
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	f, err := os.Open("../jets_schema.json")
	if err != nil {
		t.Fatalf("opening jets_schema.json: %v", err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatalf("decoding jets_schema.json: %v", err)
	}
	want := map[string]bool{"pipeline_execution_status": true, "pipeline_execution_details": true,
		"pipeline_execution_channel_details": true, "process_errors": true, "cpipes_execution_status": true,
		"error_code_catalogue": true}
	found := 0
	for i := range defs {
		if want[defs[i].TableName] {
			found++
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("installing %s: %v", defs[i].TableName, err)
			}
		}
	}
	if found != len(want) {
		t.Fatalf("found %d of the %d tables in jets_schema.json", found, len(want))
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.error_code_catalogue
		(operator_type, match_prefix, kind, report_threshold, source, source_ref)
		VALUES ('jetrules', $1, 'error', 3, 'rules', 'rules.jr value_rule')`, runCodeAmount); err != nil {
		t.Fatal(err)
	}
	return pool
}

// processError is one row of jetsapi.process_errors.
type processError struct{ op, inputColumn, msg string }

// plantProcessRun writes one failed run of a process for a client, starting at
// start: its header, one worker that failed, and its per-record errors, the
// worker rows through the cpipes node's own two statements.
func plantProcessRun(t *testing.T, pool *pgxpool.Pool, client, process, session string, start time.Time,
	errs []processError) {
	t.Helper()
	ctx := context.Background()
	var execKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id,
		 source_period_key, status, failure_details, user_email, start_time, last_update)
		VALUES (1, $1, $2, 'claims', $3, 1, 'failed', '', 'test@test', $4, $4) RETURNING key`,
		client, process, session, start).Scan(&execKey); err != nil {
		t.Fatalf("inserting the run header: %v", err)
	}
	cp := &compute_pipes.ComputePipesContext{
		ComputePipesArgs: compute_pipes.ComputePipesArgs{
			ComputePipesNodeArgs: compute_pipes.ComputePipesNodeArgs{NodeId: 0, PipelineExecKey: execKey},
			ComputePipesCommonArgs: compute_pipes.ComputePipesCommonArgs{
				Client: client, ProcessName: process, SessionId: session, InputSessionId: session,
				SourcePeriodKey: 1, PipelineConfigKey: 1, UserEmail: "test@test", MainInputStepId: "reducing01",
			},
		},
	}
	key, err := cp.InsertPipelineExecutionStatus(pool)
	if err != nil {
		t.Fatalf("InsertPipelineExecutionStatus: %v", err)
	}
	if err := cp.UpdatePipelineExecutionStatus(pool, key, 1000, 0, 1, 1, 0, 0, "reducing01", "failed", "boom"); err != nil {
		t.Fatalf("UpdatePipelineExecutionStatus: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE jetsapi.pipeline_execution_details SET start_time = $1, last_update = $1 WHERE key = $2`,
		start, key); err != nil {
		t.Fatalf("backdating the worker row: %v", err)
	}
	for i, e := range errs {
		var op, col any
		if e.op != "" {
			op = e.op
		}
		if e.inputColumn != "" {
			col = e.inputColumn
		}
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, input_column, error_message, shard_id, operator_type)
			VALUES ($1, $2, $3, $4, $5, 0, $6)`, execKey, session, fmt.Sprintf("k%d", i), col, e.msg, op); err != nil {
			t.Fatal(err)
		}
	}
}

// deleteProcessRun removes a run's session as the session purge does: its
// header, its worker rows, its per-record errors and its stored verdicts. Its
// incidents stay.
func deleteProcessRun(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	for _, table := range []string{"run_finding", "process_errors", "pipeline_execution_details",
		"pipeline_execution_status"} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM jetsapi."+table+" WHERE session_id = $1", session); err != nil {
			t.Fatal(err)
		}
	}
}

// triageRuns runs the shadow writer over the sessions, in order.
func triageRuns(t *testing.T, pool *pgxpool.Pool, sessions ...string) {
	t.Helper()
	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return runConditionStart.Add(30 * 24 * time.Hour) }
	for _, s := range sessions {
		if _, err := w.Run(context.Background(), pool, s); err != nil {
			t.Fatalf("shadow.Writer.Run %s: %v", s, err)
		}
	}
}

// plantRunCondition writes three consecutive failed runs of one process for
// one client, a day apart, each with two rows of JetStore's fixed mapping
// code, and triages them: one worker_failed condition and one mapping code
// condition, each one incident on the first run, which the later two join.
// It returns the three sessions and their start times.
func plantRunCondition(t *testing.T, pool *pgxpool.Pool, client, process string) ([]string, []time.Time) {
	t.Helper()
	var sessions []string
	var starts []time.Time
	for i := range 3 {
		s := fmt.Sprintf("%s-%s-run-%d", client, process, i+1)
		start := runConditionStart.Add(time.Duration(i) * 24 * time.Hour)
		plantProcessRun(t, pool, client, process, s, start, []processError{
			{"map_record", "", errorcode.MapRecordPrefix + "error mapping column col1 at output position 1: bad"},
			{"map_record", "", errorcode.MapRecordPrefix + "error mapping column col2 at output position 2: bad"},
		})
		sessions, starts = append(sessions, s), append(starts, start)
	}
	triageRuns(t, pool, sessions...)
	return sessions, starts
}

func listedIncident(t *testing.T, page map[string]any, id string) map[string]any {
	t.Helper()
	for _, r := range page["incidents"].([]any) {
		if row := r.(map[string]any); row["incidentId"] == id {
			return row
		}
	}
	t.Fatalf("incident %s is not on the list", id)
	return nil
}

func rfc(at time.Time) string { return at.UTC().Format(time.RFC3339) }

// A run incident's span on the list is its run condition's: first seen at its
// position, last seen at its latest member still held, and a count of the
// members still held. Deleting the condition's first run keeps the first seen
// (the incident's stored position) and lowers the count; a run incident with
// no key keeps its own session's count of one. get_incident carries the same
// condition, and a later run's session finds the condition's incident.
func TestARunIncidentsSpanIsItsRunCondition(t *testing.T) {
	pool := runConditionAPIDB(t, "apiserver_run_condition_span")
	ctx := context.Background()
	sessions, starts := plantRunCondition(t, pool, "client_r", "loader")
	wf := shadow.IncidentIdFor(sessions[0], triage.LocusWorkerFailed)
	pr := shadow.IncidentIdForCode(sessions[0], triage.LocusPerRecordFailuresReported, errorcode.CodeMappingColumn)

	page := listIncidentsPage(t, pool, AgenticAction{})
	if n := int(page["total"].(float64)); n != 2 {
		t.Fatalf("three runs of one condition at two keys raise %d incidents, want 2: %v", n, incidentIds(page))
	}
	for _, id := range []string{wf, pr} {
		row := listedIncident(t, page, id)
		if row["count"].(float64) != 3 || row["firstSeen"] != rfc(starts[0]) || row["lastSeen"] != rfc(starts[2]) ||
			row["fromRunCondition"] != true || row["fromCondition"] != false {
			t.Errorf("incident %s: count %v, %v to %v, fromRunCondition %v; want 3 runs, %s to %s", id,
				row["count"], row["firstSeen"], row["lastSeen"], row["fromRunCondition"], rfc(starts[0]), rfc(starts[2]))
		}
	}
	if code := listedIncident(t, page, pr)["errorCode"]; code != errorcode.CodeMappingColumn {
		t.Errorf("the per-record incident's errorCode is %v", code)
	}
	if code, ok := listedIncident(t, page, wf)["errorCode"]; ok {
		t.Errorf("the worker_failed incident carries errorCode %v; it has none", code)
	}

	// get_incident: the condition, the code and the run's codes.
	inc := readIncident(t, pool, pr)
	c, ok := inc["runCondition"].(map[string]any)
	if !ok {
		t.Fatalf("the per-record incident carries no run condition: %v", inc)
	}
	runs := c["runs"].([]any)
	if c["runsHeld"].(float64) != 3 || len(runs) != 3 || c["process"] != "loader" || c["client"] != "client_r" ||
		c["errorCode"] != errorcode.CodeMappingColumn || c["open"] != true || c["ownRunHeld"] != true {
		t.Errorf("the run condition: %v", c)
	}
	for i, r := range runs {
		r := r.(map[string]any)
		if r["sessionId"] != sessions[i] || r["startTime"] != rfc(starts[i]) || r["storedRows"].(float64) != 2 {
			t.Errorf("run %d of the condition: %v", i+1, r)
		}
	}
	codes := inc["codes"].([]any)
	if len(codes) != 1 || codes[0].(map[string]any)["code"] != errorcode.CodeMappingColumn ||
		codes[0].(map[string]any)["storedRows"].(float64) != 2 || codes[0].(map[string]any)["verdict"] != "present" {
		t.Errorf("the run's codes: %v", codes)
	}

	// The middle run's session finds both incidents on the first.
	ms := findIncidents(t, pool, sessions[1])["matches"].([]any)
	if len(ms) != 2 {
		t.Fatalf("the middle run matches %d incidents, want 2: %v", len(ms), ms)
	}
	for _, m := range ms {
		m := m.(map[string]any)
		if m["how"] != SessionIncidentCondition || m["incidentSessionId"] != sessions[0] || m["process"] != "loader" ||
			m["client"] != "client_r" {
			t.Errorf("the middle run's match: %v", m)
		}
		if m["locus"] == triage.LocusPerRecordFailuresReported &&
			(m["incidentId"] != pr || m["errorCode"] != errorcode.CodeMappingColumn) {
			t.Errorf("the middle run's per-record match: %v", m)
		}
	}
	own := findIncidents(t, pool, sessions[0])["matches"].([]any)
	if len(own) != 2 {
		t.Errorf("the first run matches %v, want its two own incidents", own)
	}
	for _, m := range own {
		m := m.(map[string]any)
		if m["how"] != SessionIncidentOwn {
			t.Errorf("the first run's match: %v", m)
		}
		if _, has := m["process"]; has {
			t.Errorf("an own incident's match names a process: %v", m)
		}
	}

	// The first run is deleted: the incidents keep their position as first
	// seen, and count the two runs still held.
	deleteProcessRun(t, pool, sessions[0])
	page = listIncidentsPage(t, pool, AgenticAction{})
	for _, id := range []string{wf, pr} {
		row := listedIncident(t, page, id)
		if row["count"].(float64) != 2 || row["firstSeen"] != rfc(starts[0]) || row["lastSeen"] != rfc(starts[2]) {
			t.Errorf("incident %s after its first run is deleted: count %v, %v to %v; want 2 runs, %s to %s", id,
				row["count"], row["firstSeen"], row["lastSeen"], rfc(starts[0]), rfc(starts[2]))
		}
	}
	c = readIncident(t, pool, pr)["runCondition"].(map[string]any)
	if c["runsHeld"].(float64) != 2 || c["ownRunHeld"] != false || c["firstSeen"] != rfc(starts[0]) {
		t.Errorf("the run condition after its first run is deleted: %v", c)
	}
	if _, has := readIncident(t, pool, pr)["codes"]; has {
		t.Error("the incident names codes of a run that is deleted")
	}

	// A run incident with no key: written before run conditions.
	if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version)
		VALUES ('inc_legacy_worker_failed', 'legacy', $1, 'worker_failed', 'medium', 'diagnosed', '{}', 'old')`,
		runConditionStart); err != nil {
		t.Fatal(err)
	}
	legacy := listedIncident(t, listIncidentsPage(t, pool, AgenticAction{}), "inc_legacy_worker_failed")
	if legacy["count"].(float64) != 1 || legacy["firstSeen"] != legacy["lastSeen"] || legacy["fromRunCondition"] != false {
		t.Errorf("an incident with no key: %v", legacy)
	}
	if note, _ := readIncident(t, pool, "inc_legacy_worker_failed")["runConditionNote"].(string); !strings.Contains(note,
		"before run conditions") {
		t.Errorf("the incident with no key says %q", note)
	}
}

// No canary planted in a rule exception's variable tail reaches the
// get_incident, list_incidents or find_session_incidents payload, while the
// payloads do name the code the canary rows carry and the run condition the
// two runs make.
func TestARuleExceptionTailReachesNoIncidentPayload(t *testing.T) {
	pool := runConditionAPIDB(t, "apiserver_run_condition_canary")
	const canary = "CANARY7731ZQ"
	var sessions []string
	for i := range 2 {
		s := fmt.Sprintf("run-canary-%d", i+1)
		plantProcessRun(t, pool, "client_c", "loader", s, runConditionStart.Add(time.Duration(i)*24*time.Hour),
			[]processError{
				{"jetrules", canary, errorcode.RuleExceptionPrefix + runCodeAmount + ": " + canary},
				{"jetrules", canary, errorcode.RuleExceptionPrefix + runCodeAmount + " " + canary},
				{"jetrules", canary, errorcode.RuleExceptionPrefix + runCodeAmount + canary},
				{"jetrules", canary, errorcode.RuleExceptionPrefix + "nothing catalogues this: " + canary},
				{"", canary, errorcode.RuleExceptionPrefix + canary},
				{"map_record", canary, errorcode.MapRecordPrefix + "error mapping column " + canary +
					" at output position 2: " + canary},
			})
		sessions = append(sessions, s)
	}
	triageRuns(t, pool, sessions...)

	payloads := map[string]string{}
	marshal := func(where string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		payloads[where] = string(b)
	}
	page := listIncidentsPage(t, pool, AgenticAction{})
	marshal("list_incidents", page)
	ids := incidentIds(page)
	if len(ids) < 3 {
		t.Fatalf("the canary runs raised %v; want per-record incidents to search", ids)
	}
	conditions := 0
	for _, id := range ids {
		// With protected health information disclosed, so the search covers
		// every evidence statement and not only what a redacted read shows.
		payload, code, err := agenticDispatch(context.Background(), pgOps{pool},
			&AgenticAction{Action: "get_incident", IncidentId: id}, "supervisor@example.com", audit.DisclosePHI)
		if err != nil || code != 200 {
			t.Fatalf("get_incident %s returned %d: %v", id, code, err)
		}
		marshal("get_incident "+id, *payload)
		inc := decode(t, payload)["incident"].(map[string]any)
		if _, ok := inc["runCondition"]; ok {
			conditions++
		}
	}
	if conditions == 0 {
		t.Error("no incident carries a run condition, so the search proves nothing there")
	}
	for _, s := range sessions {
		marshal("find_session_incidents "+s, findIncidents(t, pool, s))
	}
	named := false
	for where, text := range payloads {
		if strings.Contains(text, canary) {
			t.Errorf("the %s payload carries the canary", where)
		}
		if strings.Contains(text, `"errorCode":`) && strings.Contains(text, runCodeAmount) {
			named = true
		}
	}
	if !named {
		t.Errorf("no payload names the catalogued code the canary rows carry: %v", payloads["list_incidents"])
	}
}
