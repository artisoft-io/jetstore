package retention

import (
	"reflect"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedAuditTrail writes three runs' audit rows (one old, one with an old row and
// a recent one, one recent), their run rows, two approvals and a proposal.
func seedAuditTrail(t *testing.T, pool *pgxpool.Pool, old, recent time.Time) {
	t.Helper()
	for _, r := range []struct {
		run string
		at  time.Time
	}{{"old", old}, {"old", old.Add(time.Minute)}, {"mixed", old}, {"mixed", recent}, {"new", recent}} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.agent_audit (run_id, event_type, actor, payload, created_at)
			VALUES ($1, 'intent', 'agent@1', '{}', $2)`, r.run, r.at)
	}
	for _, r := range []struct {
		run   string
		start time.Time
		end   *time.Time
	}{{"old", old, &old}, {"mixed", old, nil}, {"new", recent, nil}, {"lone", old, nil}} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.agent_run
			(run_id, agent_id, agent_version, model_id, prompt_version, tier, started_at, ended_at,
			 domain_model_version, iteration_cap, wall_clock_cap_seconds)
			VALUES ($1, 'a', '1', 'm', 'p', 'T2', $2, $3, '0.1.0', 10, 60)`, r.run, r.start, r.end)
	}
	for id, at := range map[string]time.Time{"ap-old": old, "ap-new": recent} {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.approval_event
			(approval_event_id, run_ref, subject_ref, from_state, to_state, actor, tier_at_event, decided_at)
			VALUES ($1, 'old', 'p1', 'awaiting_human_approval', 'approved', 'u@example.com', 'T2', $2)`, id, at)
	}
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.change_proposal
		(proposal_id, trigger, affected_pipelines, generated_tests, impact_affected_assets,
		 impact_clinical_relevance_touched, approval_state, proposal_model_version)
		VALUES ('p1', 'incident', '{}', '{}', '{}', false, 'approved', '0.1.0')`)
}

func auditCounts(t *testing.T, pool *pgxpool.Pool) map[string]int64 {
	t.Helper()
	return map[string]int64{
		AgentAuditTable:           count(t, pool, `SELECT count(*) FROM jetsapi.agent_audit`),
		AgentRunTable:             count(t, pool, `SELECT count(*) FROM jetsapi.agent_run`),
		ApprovalEventTable:        count(t, pool, `SELECT count(*) FROM jetsapi.approval_event`),
		"jetsapi.change_proposal": count(t, pool, `SELECT count(*) FROM jetsapi.change_proposal`),
	}
}

// The audit retention deletes whole runs of the audit trail older than it, the
// run rows whose trail has gone, and the approvals decided before it; it keeps
// every proposal, which has no timestamp; its dry run counts the same and
// deletes nothing; and the append-only guard is back afterwards.
func TestPurgeAudit(t *testing.T) {
	pool := db(t)
	now := time.Now().UTC()
	old, recent := now.AddDate(0, 0, -400), now.AddDate(0, 0, -10)
	seedAuditTrail(t, pool, old, recent)
	before := auditCounts(t, pool)
	cutoff := now.AddDate(0, 0, -365)

	want := map[string]int64{AgentAuditTable: 2, AgentRunTable: 2, ApprovalEventTable: 1}
	dry, err := PurgeAudit(ctx, pool, cutoff, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dry.Rows, want) {
		t.Errorf("the dry run counts %v, want %v", dry.Rows, want)
	}
	if got := auditCounts(t, pool); !reflect.DeepEqual(got, before) {
		t.Errorf("the dry run changed the audit tables: %v, was %v", got, before)
	}

	real, err := PurgeAudit(ctx, pool, cutoff, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(real.Rows, want) {
		t.Errorf("the real run deleted %v, want %v (notes %v)", real.Rows, want, real.Notes)
	}
	after := map[string]int64{AgentAuditTable: 3, AgentRunTable: 2, ApprovalEventTable: 1, "jetsapi.change_proposal": 1}
	if got := auditCounts(t, pool); !reflect.DeepEqual(got, after) {
		t.Errorf("after the retention: %v, want %v", got, after)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.agent_run WHERE run_id IN ('old', 'lone')`); n != 0 {
		t.Errorf("%d old run rows with no trail left", n)
	}
	// The mixed run keeps its whole chain, and the guard is back.
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.agent_audit WHERE run_id = 'mixed'`); n != 2 {
		t.Errorf("the mixed run's chain has %d rows, want both", n)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM jetsapi.agent_audit WHERE run_id = 'new'`); err == nil {
		t.Error("agent_audit accepted a DELETE after the retention: its append-only guard was not restored")
	}
}
