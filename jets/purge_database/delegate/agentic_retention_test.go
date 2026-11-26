package delegate

// The purge's agentic half, against a real PostgreSQL (see the header of
// e2e_test.go): a deleted session's anomalies and run verdicts go with it, the
// incidents no session they stand for is held go after it with their rows, and
// the agent's audit trail is kept unless its own retention is set. Each test
// rebuilds the database bq_purge_agentic.

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

const agenticDB = "bq_purge_agentic"

func agenticPurgeDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := purgetest.DB(t, agenticDB)
	if err := audit.InstallSchema(context.Background(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// purgeLogged runs purgeSessions with cfg and returns its log.
func purgeLogged(t *testing.T, pool *pgxpool.Pool, cfg purgeConfig) string {
	t.Helper()
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	err := purgeSessions(context.Background(), pool, cfg)
	log.SetFlags(flags)
	log.SetOutput(out)
	if err != nil {
		t.Fatalf("purgeSessions: %v\n%s", err, buf.String())
	}
	return buf.String()
}

func n(t *testing.T, pool *pgxpool.Pool, stmt string, args ...any) int64 {
	t.Helper()
	var c int64
	if err := pool.QueryRow(context.Background(), stmt, args...).Scan(&c); err != nil {
		t.Fatalf("%v\n%s", err, stmt)
	}
	return c
}

// seedAgentic writes, on a session, two anomalies, the run verdicts at one
// locus, and an incident with three hypotheses, two events and a remediation.
func seedAgentic(t *testing.T, pool *pgxpool.Pool, session string) string {
	t.Helper()
	for _, id := range []string{"a1", "a2"} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.anomaly
			(anomaly_id, detected_at, anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
			 anomaly_signal_type, anomaly_observed_value, anomaly_expected_basis, anomaly_confounders,
			 anomaly_detector_ref)
			VALUES ($1, now(), $2, 'run', 'proc', 'duration', 'x', 'history', '{}', 'duration@1')`,
			id+"-"+session, session)
	}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.run_finding
		(session_id, locus, error_code, verdict, classifier_ref, classified_at)
		VALUES ($1, 'worker_failed', '', 'present', 'triage@1', now())`, session)
	inc := "inc_" + session + "_worker_failed"
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version)
		VALUES ($1, $2, now(), 'worker_failed', 'high', 'diagnosed', '{}', '0.1.0')`, inc, session)
	for _, h := range []string{"h1", "h2", "h3"} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.hypothesis
			(hypothesis_id, hypothesis_incident_ref, cause, confidence, rank, supporting_evidence,
			 contradicting_evidence, hypothesis_locus, basis)
			VALUES ($1, $2, 'c', 0.5, 1, '[]', '[]', 'worker_failed', '{}')`, h+"-"+inc, inc)
	}
	for _, e := range []string{"e1", "e2"} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident_event
			(incident_event_id, event_incident_ref, from_status, to_status, event_actor, event_actor_kind,
			 transitioned_at)
			VALUES ($1, $2, 'detected', 'triaged', 'triage@1', 'agent', now())`, e+"-"+inc, inc)
	}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.remediation
		(remediation_id, incident_ref, autonomy_tier_required, reversibility, remediation_approval_state)
		VALUES ($1, $2, 'T2', 'fully_reversible', 'draft')`, "r-"+inc, inc)
	return inc
}

func agenticRows(t *testing.T, pool *pgxpool.Pool, session string) map[string]int64 {
	t.Helper()
	inc := "inc_" + session + "_worker_failed"
	return map[string]int64{
		"anomaly":        n(t, pool, `SELECT count(*) FROM jetsapi.anomaly WHERE anomaly_session_id = $1`, session),
		"run_finding":    n(t, pool, `SELECT count(*) FROM jetsapi.run_finding WHERE session_id = $1`, session),
		"incident":       n(t, pool, `SELECT count(*) FROM jetsapi.incident WHERE incident_id = $1`, inc),
		"hypothesis":     n(t, pool, `SELECT count(*) FROM jetsapi.hypothesis WHERE hypothesis_incident_ref = $1`, inc),
		"incident_event": n(t, pool, `SELECT count(*) FROM jetsapi.incident_event WHERE event_incident_ref = $1`, inc),
		"remediation":    n(t, pool, `SELECT count(*) FROM jetsapi.remediation WHERE incident_ref = $1`, inc),
	}
}

func wantLines(t *testing.T, logged string, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(logged, l+"\n") {
			t.Errorf("the log has no line %q:\n%s", l, logged)
		}
	}
}

// The retention purge deletes an expired session's anomalies and run verdicts
// with it, and then its incident with its hypotheses, events and remediation,
// in the same run; a recent session keeps all of them. The dry run reports
// both deletions and deletes nothing.
func TestThePurgeDeletesAnomaliesAndSweepsIncidents(t *testing.T) {
	pool := agenticPurgeDB(t)
	now := time.Now()
	purgetest.Seed(t, pool, purgetest.Session{ID: "expired", Client: "c1", Registered: true, LastUpdate: daysAgo(40)})
	purgetest.Seed(t, pool, purgetest.Session{ID: "recent", Client: "c1", Registered: true, LastUpdate: daysAgo(1)})
	inc := seedAgentic(t, pool, "expired")
	seedAgentic(t, pool, "recent")
	full := map[string]int64{"anomaly": 2, "run_finding": 1, "incident": 1, "hypothesis": 3, "incident_event": 2,
		"remediation": 1}

	dry := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, DryRun: true, Now: now, Deadline: now.Add(runBudget)})
	wantLines(t, dry,
		"   DRY RUN, nothing written: 2 rows from jetsapi.anomaly",
		"   DRY RUN, nothing written: 1 rows from jetsapi.run_finding",
		"Incident retention: DRY RUN, nothing written: 1 incidents whose own session is no longer held: 1 deleted, "+
			"0 kept (a delivery or run still held is placed in them), 0 failed, 0 not reached before the deadline",
		"   incident retention: DRY RUN, nothing written: 3 rows from jetsapi.hypothesis",
		"   incident retention: DRY RUN, nothing written: 2 rows from jetsapi.incident_event",
		"   incident retention: DRY RUN, nothing written: 1 rows from jetsapi.remediation",
		"   incident retention: DRY RUN, nothing written: 1 rows from jetsapi.incident",
		"Agent audit retention: AGENT_AUDIT_RETENTION_DAYS is not set, the audit trail is kept")
	for _, s := range []string{"expired", "recent"} {
		if got := agenticRows(t, pool, s); !mapsEqual(got, full) {
			t.Errorf("the dry run changed %s's rows: %v", s, got)
		}
	}

	real := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)})
	wantLines(t, real,
		"   2 rows from jetsapi.anomaly",
		"Incident retention: 1 incidents whose own session is no longer held: 1 deleted, "+
			"0 kept (a delivery or run still held is placed in them), 0 failed, 0 not reached before the deadline",
		"   incident retention: 3 rows from jetsapi.hypothesis")
	for k, v := range agenticRows(t, pool, "expired") {
		if v != 0 {
			t.Errorf("%d %s rows of the expired session (incident %s) left", v, k, inc)
		}
	}
	if got := agenticRows(t, pool, "recent"); !mapsEqual(got, full) {
		t.Errorf("the recent session's rows changed: %v", got)
	}
}

func mapsEqual(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// seedAudit writes one run's old audit trail: two audit rows, its run row and
// one approval, all 400 days old, and one proposal.
func seedAudit(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	old := time.Now().AddDate(0, 0, -400)
	for range 2 {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.agent_audit (run_id, event_type, actor, payload, created_at)
			VALUES ('run-old', 'intent', 'agent@1', '{}', $1)`, old)
	}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.agent_run
		(run_id, agent_id, agent_version, model_id, prompt_version, tier, started_at, ended_at,
		 domain_model_version, iteration_cap, wall_clock_cap_seconds)
		VALUES ('run-old', 'a', '1', 'm', 'p', 'T2', $1, $1, '0.1.0', 10, 60)`, old)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.approval_event
		(approval_event_id, run_ref, subject_ref, from_state, to_state, actor, tier_at_event, decided_at)
		VALUES ('ap-old', 'run-old', 'p1', 'awaiting_human_approval', 'approved', 'u@example.com', 'T2', $1)`, old)
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.change_proposal
		(proposal_id, trigger, affected_pipelines, generated_tests, impact_affected_assets,
		 impact_clinical_relevance_touched, approval_state, proposal_model_version)
		VALUES ('p1', 'incident', '{}', '{}', '{}', false, 'approved', '0.1.0')`)
}

func auditRows(t *testing.T, pool *pgxpool.Pool) [4]int64 {
	t.Helper()
	return [4]int64{n(t, pool, `SELECT count(*) FROM jetsapi.agent_audit`),
		n(t, pool, `SELECT count(*) FROM jetsapi.agent_run`),
		n(t, pool, `SELECT count(*) FROM jetsapi.approval_event`),
		n(t, pool, `SELECT count(*) FROM jetsapi.change_proposal`)}
}

// No purge touches the audit trail while AGENT_AUDIT_RETENTION_DAYS is unset,
// however old it is; set, the trail older than it goes (its dry run first,
// which deletes nothing), and the proposals, which carry no timestamp, stay.
func TestTheAuditTrailIsKeptUnlessItsRetentionIsSet(t *testing.T) {
	pool := agenticPurgeDB(t)
	seedAudit(t, pool)
	now := time.Now()
	all := [4]int64{2, 1, 1, 1}

	purgeLogged(t, pool, purgeConfig{RetentionDays: 30, Now: now, Deadline: now.Add(runBudget)})
	if got := auditRows(t, pool); got != all {
		t.Errorf("with no audit retention the purge left %v, want %v", got, all)
	}

	dry := purgeLogged(t, pool, purgeConfig{RetentionDays: 30, AuditRetentionDays: 365, DryRun: true, Now: now,
		Deadline: now.Add(runBudget)})
	wantLines(t, dry, "   agent audit retention: DRY RUN, nothing written: 2 rows from jetsapi.agent_audit",
		"   agent audit retention: DRY RUN, nothing written: 1 rows from jetsapi.agent_run",
		"   agent audit retention: DRY RUN, nothing written: 1 rows from jetsapi.approval_event",
		"   agent audit retention: jetsapi.change_proposal is kept: it has no timestamp to judge a row's age by")
	if got := auditRows(t, pool); got != all {
		t.Errorf("the dry run left %v, want %v", got, all)
	}

	purgeLogged(t, pool, purgeConfig{RetentionDays: 30, AuditRetentionDays: 365, Now: now,
		Deadline: now.Add(runBudget)})
	if got, want := auditRows(t, pool), [4]int64{0, 0, 0, 1}; got != want {
		t.Errorf("with a 365-day audit retention the purge left %v, want %v", got, want)
	}
}

func TestAuditRetentionDays(t *testing.T) {
	for v, want := range map[string]int{"": 0, "  ": 0, "365": 365, " 1 ": 1} {
		if got, err := AuditRetentionDays(v); err != nil || got != want {
			t.Errorf("AuditRetentionDays(%q) = %d, %v; want %d", v, got, err, want)
		}
	}
	for _, v := range []string{"0", "-5", "a year", "1.5"} {
		if _, err := AuditRetentionDays(v); err == nil {
			t.Errorf("AuditRetentionDays(%q) accepted", v)
		}
	}
}
