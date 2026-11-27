package audit

import (
	"context"
	"testing"
)

// The empty main input locus reaches a database installed before it, as the
// job locus did, and the incident's three source columns are added to it: the
// install compares each locus CHECK with the vocabulary and replaces it when
// they differ, on incident, hypothesis and run_finding alike, keeping the rows
// written under the earlier constraint, which read NULL in the new columns.
func TestInstallSchemaAdmitsTheEmptyInputLocusOnAnExistingTable(t *testing.T) {
	db := freshDatabase(t, "locus_empty_input_premigration")
	ctx := context.Background()

	// The twelve members a database held before main_input_empty, written out
	// rather than derived from today's model.
	const twelve = `'run_not_started', 'step_never_started', 'worker_not_terminated',
		'worker_failed', 'sink_failed_under_completed_worker', 'rows_lost_silently',
		'per_record_failures_reported', 'per_record_failures_unreportable', 'written_not_arrived',
		'delivery_overdue', 'delivery_departs_profile', 'job_failed'`
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing the schema: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE jetsapi.incident DROP CONSTRAINT incident_locus_ck,
		   ADD CONSTRAINT incident_locus_ck CHECK (incident_locus IN (` + twelve + `))`,
		`ALTER TABLE jetsapi.hypothesis DROP CONSTRAINT hypothesis_locus_ck,
		   ADD CONSTRAINT hypothesis_locus_ck CHECK (hypothesis_locus IN (` + twelve + `))`,
		`ALTER TABLE jetsapi.run_finding DROP CONSTRAINT run_finding_locus_ck,
		   ADD CONSTRAINT run_finding_locus_ck CHECK (locus IN (` + twelve + `))`,
	} {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Fatalf("building the earlier constraints: %v", err)
		}
	}
	if _, err := db.Exec(ctx, `ALTER TABLE jetsapi.incident DROP COLUMN incident_source_client,
		DROP COLUMN incident_source_org, DROP COLUMN incident_source_object_type`); err != nil {
		t.Fatalf("building the earlier incident table: %v", err)
	}
	mustInsertLocusIncident(t, db, "inc_old", LocusDeliveryOverdue)
	if err := insertLocusIncident(db, "inc_probe", LocusMainInputEmpty); err == nil {
		t.Fatal("the fixture admits main_input_empty already; this test would prove nothing")
	}
	insertFinding := func(session string) error {
		_, err := db.Exec(ctx, `INSERT INTO jetsapi.run_finding
			(session_id, locus, verdict, classifier_ref, classified_at)
			VALUES ($1, $2, 'present', 'triage@1', now())`, session, LocusMainInputEmpty)
		return err
	}
	if err := insertFinding("s_probe"); err == nil {
		t.Fatal("the fixture's run_finding admits main_input_empty already; this test would prove nothing")
	}

	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing over the earlier constraints: %v", err)
	}
	if err := insertLocusIncident(db, "inc_empty", LocusMainInputEmpty); err != nil {
		t.Errorf("an incident at main_input_empty was refused after the install: %v", err)
	}
	if err := insertLocusHypothesis(db, "hyp_empty", "inc_empty", LocusMainInputEmpty); err != nil {
		t.Errorf("a hypothesis at main_input_empty was refused after the install: %v", err)
	}
	if err := insertFinding("s_empty"); err != nil {
		t.Errorf("a run finding at main_input_empty was refused after the install: %v", err)
	}
	var kept int
	if err := db.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.incident WHERE incident_id = 'inc_old'`).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != 1 {
		t.Errorf("the row written under the old constraint is gone (%d rows)", kept)
	}
	var nulls bool
	if err := db.QueryRow(ctx, `SELECT incident_source_client IS NULL AND incident_source_org IS NULL
		AND incident_source_object_type IS NULL FROM jetsapi.incident WHERE incident_id = 'inc_old'`).
		Scan(&nulls); err != nil || !nulls {
		t.Errorf("the earlier incident's source columns are not NULL after the install (%v)", err)
	}
	if err := insertLocusIncident(db, "inc_bogus", "input_late"); err == nil {
		t.Error("the widened constraint admits a locus outside the vocabulary")
	}
}
