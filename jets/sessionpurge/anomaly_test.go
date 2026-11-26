package sessionpurge

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/sessionpurge/purgetest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedAnomalies writes n detector readings on a session, as the detectors write
// them: by anomaly_session_id.
func seedAnomalies(t *testing.T, pool *pgxpool.Pool, id string, n int) {
	t.Helper()
	for i := range n {
		purgetest.Exec(t, pool, `INSERT INTO jetsapi.anomaly
			(anomaly_id, detected_at, anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
			 anomaly_signal_type, anomaly_observed_value, anomaly_expected_basis, anomaly_confounders,
			 anomaly_detector_ref)
			VALUES ($1, now(), $2, 'feed', 'feed_a', 'arrival', 'x', 'profile', '{}', 'feed_overdue@1')`,
			"a-"+id+"-"+strconv.Itoa(i), id)
	}
}

// seedIncident writes one incident on a session.
func seedIncident(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	inc := "inc_" + id + "_worker_failed"
	purgetest.Exec(t, pool, `INSERT INTO jetsapi.incident
		(incident_id, incident_session_id, incident_detected_at, incident_locus, severity, status,
		 incident_confounders, incident_model_version)
		VALUES ($1, $2, now(), 'worker_failed', 'high', 'diagnosed', '{}', '0.1.0')`, inc, id)
}

func agenticDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := purgetest.DB(t, testDB)
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// A session's anomalies and run verdicts are deleted with it, by the purge's
// retention deletion and by a user's, which take the same executor; the report
// counts them; another session's are kept, and so are the session's incidents,
// which the agentic retention sweep decides.
func TestAnomaliesDeletedWithTheirSession(t *testing.T) {
	pool := agenticDB(t)
	for _, id := range []string{"ret", "usr", "keep"} {
		purgetest.Seed(t, pool, purgetest.Session{ID: id, Client: "c1", Registered: true, LastUpdate: days(200)})
		seedAnomalies(t, pool, id, 3)
		seedRunFindings(t, pool, id)
		seedIncident(t, pool, id)
	}

	queue(t, pool, retention("ret"))
	queue(t, pool, Request{SessionID: "usr", Client: "c1", Reason: ReasonUser, RequestedBy: "someone@example.com"})
	// A user's Delete, as the apiserver executes it: user rows only.
	user, err := Execute(ctx, pool, Options{Reasons: []Reason{ReasonUser}})
	if err != nil {
		t.Fatal(err)
	}
	// The purge's run: what is left in the queue.
	purge, err := Execute(ctx, pool, Options{Deadline: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	for name, r := range map[string]Report{"user": user, "retention": purge} {
		if r.Done != 1 || r.Rows["jetsapi.anomaly"] != 3 || r.Rows["jetsapi.run_finding"] != 3 {
			t.Errorf("the %s deletion: %+v, want one session with 3 anomalies and 3 run verdicts", name, r)
		}
	}
	for id, want := range map[string]int64{"ret": 0, "usr": 0, "keep": 3} {
		if n := count(t, pool, `SELECT count(*) FROM jetsapi.anomaly WHERE anomaly_session_id = $1`, id); n != want {
			t.Errorf("%d anomalies left on %s, want %d", n, id, want)
		}
		if n := count(t, pool, `SELECT count(*) FROM jetsapi.run_finding WHERE session_id = $1`, id); n != want {
			t.Errorf("%d run verdicts left on %s, want %d", n, id, want)
		}
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.incident`); n != 3 {
		t.Errorf("%d incidents left, want the 3 seeded: the session purge deletes none", n)
	}
}

// A database without the anomaly table skips it, as it skips any fixed table it
// lacks, and the session is deleted from everything else.
func TestNoAnomalyTableIsSkipped(t *testing.T) {
	pool := purgetest.DB(t, testDB)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, LastUpdate: days(200)})
	queue(t, pool, retention("s1"))
	r, err := Execute(ctx, pool, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Rows["jetsapi.anomaly"]; ok || r.Done != 1 || r.Failed != 0 {
		t.Errorf("report %+v, want s1 done and no anomaly table reported", r)
	}
}

// A session found only by its anomalies, its registration gone, is an orphan
// once its id is older than the retention, and its anomalies go with it.
func TestAnomalyOnlySessionIsAnOrphan(t *testing.T) {
	pool := agenticDB(t)
	old := strconv.FormatInt(days(100).UnixMilli(), 10)
	recent := strconv.FormatInt(days(1).UnixMilli(), 10)
	seedAnomalies(t, pool, old, 2)
	seedAnomalies(t, pool, recent, 2)
	orph, err := OrphanCandidates(ctx, pool, days(30))
	if err != nil {
		t.Fatal(err)
	}
	if len(orph) != 1 || orph[0].SessionID != old || orph[0].Client != ClientUnknown {
		t.Fatalf("orphan candidates %+v, want %s alone", orph, old)
	}
	queue(t, pool, orph...)
	if r, err := Execute(ctx, pool, Options{}); err != nil || r.Done != 1 || r.Rows["jetsapi.anomaly"] != 2 {
		t.Errorf("report %+v, %v", r, err)
	}
}

// AfterDryRun sees the dry run's deletions, is called when there is nothing to
// delete, and is rolled back with the rest; a real run never calls it.
func TestAfterDryRun(t *testing.T) {
	pool := agenticDB(t)
	purgetest.Seed(t, pool, purgetest.Session{ID: "s1", Client: "c1", Registered: true, LastUpdate: days(200)})
	seedAnomalies(t, pool, "s1", 2)
	var seen []int64
	after := func(ctx_ context.Context, tx pgx.Tx) error {
		var n int64
		if err := tx.QueryRow(ctx_, `SELECT count(*) FROM jetsapi.anomaly WHERE anomaly_session_id = 's1'`).Scan(&n); err != nil {
			return err
		}
		seen = append(seen, n)
		_, err := tx.Exec(ctx_, `INSERT INTO jetsapi.process_errors (session_id, error_message) VALUES ('hook', 'x')`)
		return err
	}
	if _, err := ExecuteRequests(ctx, pool, []Request{retention("s1")}, Options{DryRun: true, AfterDryRun: after}); err != nil {
		t.Fatal(err)
	}
	if _, err := ExecuteRequests(ctx, pool, nil, Options{DryRun: true, AfterDryRun: after}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != 0 || seen[1] != 2 {
		t.Errorf("the hook saw %v anomalies of s1, want 0 after the dry run's deletion, then 2 with none", seen)
	}
	if n := count(t, pool, `SELECT count(*) FROM jetsapi.process_errors WHERE session_id = 'hook'`); n != 0 {
		t.Errorf("the hook's write survived the dry run: %d rows", n)
	}
	queue(t, pool, retention("s1"))
	if _, err := Execute(ctx, pool, Options{AfterDryRun: after}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Errorf("a real run called the hook")
	}
}
