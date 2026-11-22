package audit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The locus CHECKs follow the vocabulary onto a table that already exists.
//
// **`CREATE TABLE IF NOT EXISTS` leaves an existing table's constraints alone**,
// so when the locus vocabulary gained the two delivery loci, a database installed
// before that kept a nine-member incident_locus_ck and hypothesis_locus_ck, and
// the first incident written at a delivery locus would be refused there — on a
// deployment that has been running detection for weeks, which is every
// deployment the new loci are for. The generated DDL carries a block per CHECK
// that compares the installed members with the vocabulary and replaces the
// constraint when they differ; this is that block run against a real database.
//
// It installs twice, as a deployment does: once over a database holding the
// nine-member constraints and a row at an old locus, and once more over the
// result. The first install must widen both constraints and keep the row; the
// second must leave them alone.
func TestInstallSchemaWidensTheLocusChecksOnAnExistingTable(t *testing.T) {
	db := freshDatabase(t, "locus_premigration")
	ctx := context.Background()

	// The constraints as a database installed before the widening holds them,
	// written out rather than derived: a fixture generated from today's model
	// could not represent yesterday's.
	const nine = `'run_not_started', 'step_never_started', 'worker_not_terminated',
		'worker_failed', 'sink_failed_under_completed_worker', 'rows_lost_silently',
		'per_record_failures_reported', 'per_record_failures_unreportable', 'written_not_arrived'`
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing the schema: %v", err)
	}
	for _, stmt := range []string{
		`ALTER TABLE jetsapi.incident DROP CONSTRAINT incident_locus_ck,
		   ADD CONSTRAINT incident_locus_ck CHECK (incident_locus IN (` + nine + `))`,
		`ALTER TABLE jetsapi.hypothesis DROP CONSTRAINT hypothesis_locus_ck,
		   ADD CONSTRAINT hypothesis_locus_ck CHECK (hypothesis_locus IN (` + nine + `))`,
	} {
		if _, err := db.Exec(ctx, stmt); err != nil {
			t.Fatalf("building the earlier constraints: %v", err)
		}
	}
	mustInsertLocusIncident(t, db, "inc_old", "worker_failed")
	mustInsertLocusHypothesis(t, db, "hyp_old", "inc_old", "worker_failed")
	if err := insertLocusIncident(db, "inc_probe", "delivery_overdue"); err == nil {
		t.Fatal("the fixture admits a delivery locus already; this test would prove nothing")
	}

	// The install a deployment runs after upgrading.
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing over the earlier constraints: %v", err)
	}
	for _, locus := range []string{LocusDeliveryOverdue, LocusDeliveryDepartsProfile} {
		id := "inc_" + locus
		if err := insertLocusIncident(db, id, locus); err != nil {
			t.Errorf("an incident at %s was refused after the install: %v", locus, err)
		}
		if err := insertLocusHypothesis(db, "hyp_"+locus, id, locus); err != nil {
			t.Errorf("a hypothesis at %s was refused after the install: %v", locus, err)
		}
	}
	var kept int
	if err := db.QueryRow(ctx,
		`SELECT count(*) FROM jetsapi.incident WHERE incident_id = 'inc_old'`).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != 1 {
		t.Errorf("the row written under the old constraint is gone (%d rows)", kept)
	}
	// The vocabulary did not drift into an admit-anything constraint.
	if err := insertLocusIncident(db, "inc_bogus", "delivery_late"); err == nil {
		t.Error("the widened constraint admits a locus outside the vocabulary")
	}

	// The second install finds the members equal and does nothing: the
	// constraint is the same catalogue row, not a replacement of it.
	before := constraintOids(t, db)
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if after := constraintOids(t, db); after != before {
		t.Errorf("the second install replaced a locus CHECK that already matched the vocabulary "+
			"(oids %s, then %s); an up-to-date database should not take the table lock", before, after)
	}
}

// A narrowing that existing rows violate fails the install by name rather than
// leaving the wider constraint in place. Nothing narrows the vocabulary today;
// this pins the direction the comparison was written to fail in.
func TestInstallSchemaRefusesToNarrowALocusCheckOverRowsItWouldReject(t *testing.T) {
	db := freshDatabase(t, "locus_narrowing")
	ctx := context.Background()
	if err := InstallSchema(ctx, db); err != nil {
		t.Fatalf("installing the schema: %v", err)
	}
	if _, err := db.Exec(ctx, `ALTER TABLE jetsapi.incident DROP CONSTRAINT incident_locus_ck`); err != nil {
		t.Fatal(err)
	}
	mustInsertLocusIncident(t, db, "inc_retired", "a_retired_locus")
	err := InstallSchema(ctx, db)
	if err == nil {
		t.Fatal("the install succeeded over a row at a locus outside the vocabulary")
	}
	if !strings.Contains(err.Error(), "incident_locus_ck") {
		t.Errorf("the failure does not name the constraint: %v", err)
	}
}

// freshDatabase creates a database of its own on the server JETS_TEST_DSN names,
// so a test that builds an earlier schema does not alter the one the other tests
// share, and drops it afterwards.
func freshDatabase(t *testing.T, prefix string) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	name := fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	cfg, err := pgxpool.ParseConfig(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := pool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name); err != nil {
			t.Logf("dropping %s: %v", name, err)
		}
	})
	return db
}

func insertLocusIncident(db *pgxpool.Pool, id, locus string) error {
	_, err := db.Exec(context.Background(),
		`INSERT INTO jetsapi.incident (incident_id, incident_session_id, incident_detected_at,
		   incident_locus, severity, status, incident_confounders, incident_model_version)
		 VALUES ($1, 's1', now(), $2, 'low', 'detected', '{}', 'test')`, id, locus)
	return err
}

func mustInsertLocusIncident(t *testing.T, db *pgxpool.Pool, id, locus string) {
	t.Helper()
	if err := insertLocusIncident(db, id, locus); err != nil {
		t.Fatalf("inserting incident %s at %s: %v", id, locus, err)
	}
}

func insertLocusHypothesis(db *pgxpool.Pool, id, incident, locus string) error {
	_, err := db.Exec(context.Background(),
		`INSERT INTO jetsapi.hypothesis (hypothesis_id, hypothesis_incident_ref, cause, confidence,
		   rank, supporting_evidence, contradicting_evidence, hypothesis_locus, basis)
		 VALUES ($1, $2, 'a cause', 0.5, 1, '[]', '[]', $3, '{}')`, id, incident, locus)
	return err
}

func mustInsertLocusHypothesis(t *testing.T, db *pgxpool.Pool, id, incident, locus string) {
	t.Helper()
	if err := insertLocusHypothesis(db, id, incident, locus); err != nil {
		t.Fatalf("inserting hypothesis %s at %s: %v", id, locus, err)
	}
}

func constraintOids(t *testing.T, db *pgxpool.Pool) string {
	t.Helper()
	var oids string
	if err := db.QueryRow(context.Background(),
		`SELECT string_agg(oid::text, ',' ORDER BY conname) FROM pg_constraint
		  WHERE conname IN ('incident_locus_ck', 'hypothesis_locus_ck')`).Scan(&oids); err != nil {
		t.Fatal(err)
	}
	return oids
}
