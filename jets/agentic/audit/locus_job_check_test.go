package audit

import (
	"context"
	"testing"
)

// The job locus reaches a database installed before it, as the delivery loci
// did: the install compares each locus CHECK with the vocabulary and replaces
// it when they differ, on incident, hypothesis and run_finding alike, keeping
// the rows written under the earlier constraint.
func TestInstallSchemaAdmitsTheJobLocusOnAnExistingTable(t *testing.T) {
	db := freshDatabase(t, "locus_job_premigration")
	ctx := context.Background()

	// The eleven members a database held before job_failed, written out rather
	// than derived from today's model.
	const eleven = `'run_not_started', 'step_never_started', 'worker_not_terminated',
		'worker_failed', 'sink_failed_under_completed_worker', 'rows_lost_silently',
		'per_record_failures_reported', 'per_record_failures_unreportable', 'written_not_arrived',
		'delivery_overdue', 'delivery_departs_profile'`
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing the schema: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE jetsapi.incident DROP CONSTRAINT incident_locus_ck,
		   ADD CONSTRAINT incident_locus_ck CHECK (incident_locus IN (` + eleven + `))`,
		`ALTER TABLE jetsapi.hypothesis DROP CONSTRAINT hypothesis_locus_ck,
		   ADD CONSTRAINT hypothesis_locus_ck CHECK (hypothesis_locus IN (` + eleven + `))`,
		`ALTER TABLE jetsapi.run_finding DROP CONSTRAINT run_finding_locus_ck,
		   ADD CONSTRAINT run_finding_locus_ck CHECK (locus IN (` + eleven + `))`,
	} {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Fatalf("building the earlier constraints: %v", err)
		}
	}
	mustInsertLocusIncident(t, db, "inc_old", LocusDeliveryOverdue)
	if err := insertLocusIncident(db, "inc_probe", LocusJobFailed); err == nil {
		t.Fatal("the fixture admits job_failed already; this test would prove nothing")
	}
	insertFinding := func(session string) error {
		_, err := db.Exec(ctx, `INSERT INTO jetsapi.run_finding
			(session_id, locus, verdict, classifier_ref, classified_at)
			VALUES ($1, $2, 'present', 'triage@1', now())`, session, LocusJobFailed)
		return err
	}
	if err := insertFinding("s_probe"); err == nil {
		t.Fatal("the fixture's run_finding admits job_failed already; this test would prove nothing")
	}

	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing over the earlier constraints: %v", err)
	}
	if err := insertLocusIncident(db, "inc_job", LocusJobFailed); err != nil {
		t.Errorf("an incident at job_failed was refused after the install: %v", err)
	}
	if err := insertLocusHypothesis(db, "hyp_job", "inc_job", LocusJobFailed); err != nil {
		t.Errorf("a hypothesis at job_failed was refused after the install: %v", err)
	}
	if err := insertFinding("s_job"); err != nil {
		t.Errorf("a run finding at job_failed was refused after the install: %v", err)
	}
	var kept int
	if err := db.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.incident WHERE incident_id = 'inc_old'`).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != 1 {
		t.Errorf("the row written under the old constraint is gone (%d rows)", kept)
	}
	if err := insertLocusIncident(db, "inc_bogus", "job_late"); err == nil {
		t.Error("the widened constraint admits a locus outside the vocabulary")
	}
}
