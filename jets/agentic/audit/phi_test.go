// Making the data_classification marker load-bearing (task AE.2, I-311), and
// the additive half of a migration (task AB.4, I-337).
//
// **What is worth testing here is that the marker and its enforcement cannot
// drift apart**, which is the whole defect: the marker was correct, travelled
// the whole toolchain, and was read by nothing. A test that only checked "the
// statement is withheld" would pass a second marked property being silently
// disclosed.
//
// The two schema tests need JETS_TEST_DSN like the rest of this package; the
// coverage test does not and runs everywhere.
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

// The generated manifest against the hand-written redactors, in both directions
// — `_assert_tables_agree`'s discipline for a second kind of disagreement.
//
// A marked property with no redactor is the original defect returning under a
// new name; a redactor for a property nothing marks is code defending a field
// the model does not classify, which is the same disagreement from the other
// side and is the half that would otherwise be invisible.
func TestEveryClassifiedPropertyHasARedactor(t *testing.T) {
	if len(DataClassifiedProperties) == 0 {
		t.Fatal("the generated manifest is empty; either the model marks nothing " +
			"or `jets-agentic generate` did not run")
	}
	unhandled, unknown := PHIRedactorCoverage()
	if len(unhandled) > 0 {
		t.Errorf("classified with no redactor: %v — a marker nothing reads is the defect "+
			"I-311 was raised for, arriving again", unhandled)
	}
	if len(unknown) > 0 {
		t.Errorf("redactors for properties the model does not classify: %v — either the "+
			"marker was removed from model.py or this map names the wrong property", unknown)
	}
}

// The one marked property today, pinned by name. **This is not redundant with
// the coverage test**: that one says the two sides agree, and this one says what
// they agree about — so a change that removed the marker *and* the redactor
// together would pass the first and fail here, which is the shape of a silent
// weakening.
func TestTheClassifiedPropertyIsEvidenceStatement(t *testing.T) {
	if len(DataClassifiedProperties) != 1 {
		t.Fatalf("the model marks %d properties; if that is deliberate, extend phiRedactors "+
			"and this test together: %v", len(DataClassifiedProperties), DataClassifiedProperties)
	}
	got := DataClassifiedProperties[0]
	if got.Entity != "Evidence" || got.Property != "statement" || got.Classification != ClassificationPHI {
		t.Errorf("the classified property is %+v, want Evidence.statement (PHI)", got)
	}
}

// The control itself, against a real database: the statement does not leave the
// read path unless the caller said DisclosePHI.
//
// **Both directions in one test on purpose.** The withheld half alone would be
// satisfied by a read that always returned nothing, which is a deletion rather
// than a control; the disclosed half is what makes the first one about a policy.
func TestPHIIsWithheldFromTheReadUnlessDisclosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := fmt.Sprintf("inc_phi_%d", time.Now().UnixNano())
	step := "reducing00"
	var shard int64
	insertIncident(t, pool, id, id+"_s", LocusWorkerFailed, "", "triaged",
		&step, &shard, []string{"step_label_ambiguous"}, time.Now().UTC())
	insertHypothesis(t, pool, id+"_h1", id, "a step regressed", "", 1, 0.7,
		`[{"statement":"member 12345 arrived twice","source":"run_telemetry"}]`,
		`[{"statement":"a sampling cap was set","source":"detector_confounder"}]`)

	redacted, err := ReadIncident(ctx, pool, id, RedactPHI)
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range [][]Evidence{
		redacted.Hypotheses[0].SupportingEvidence,
		redacted.Hypotheses[0].ContradictingEvidence,
	} {
		if side[0].Statement != "" {
			t.Errorf("a PHI-classified statement left the read path as %q", side[0].Statement)
		}
		if !side[0].StatementRedacted {
			t.Error("the statement was withheld and nothing says so; a caller cannot tell " +
				"it from an agent that cited a source and said nothing")
		}
		// Everything else on the item is untouched: the marker is on one
		// property, and redacting the source would be this control deciding
		// something the model did not say.
		if side[0].Source == "" {
			t.Error("the source was redacted; only the marked property should be")
		}
	}

	disclosed, err := ReadIncident(ctx, pool, id, DisclosePHI)
	if err != nil {
		t.Fatal(err)
	}
	got := disclosed.Hypotheses[0].SupportingEvidence[0]
	if got.Statement != "member 12345 arrived twice" {
		t.Errorf("the disclosed statement is %q", got.Statement)
	}
	if got.StatementRedacted {
		t.Error("a disclosed statement is flagged as withheld")
	}
}

// HypothesesFor is the other entry point and takes the same decision, so a
// caller that reached the hypotheses directly could not sidestep it.
func TestHypothesesForTakesThePHIDecisionToo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := fmt.Sprintf("inc_phi2_%d", time.Now().UnixNano())
	insertIncident(t, pool, id, id+"_s", LocusWorkerFailed, "", "triaged",
		nil, nil, []string{}, time.Now().UTC())
	insertHypothesis(t, pool, id+"_h1", id, "a step regressed", "", 1, 0.7,
		`[{"statement":"a patient identifier","source":"run_telemetry"}]`, `[]`)

	hs, err := HypothesesFor(ctx, pool, id, RedactPHI)
	if err != nil {
		t.Fatal(err)
	}
	if hs[0].SupportingEvidence[0].Statement != "" {
		t.Error("HypothesesFor disclosed a PHI-classified statement to a redacted caller")
	}
}

// AB.4, I-337 — the additive half of a migration.
//
// **`CREATE TABLE IF NOT EXISTS` does nothing to a table that already exists**,
// so before this a column added to the domain model reached a fresh database and
// no migrated one, silently: InstallSchema ran clean either way and the missing
// column surfaced as a 42703 in whatever query touched it next. Every deployment
// that installed the schema between AB.1 and AB.4 is in exactly that state.
//
// It builds a database holding jetsapi.incident at its pre-AB.4 shape, which is
// what such a deployment looks like, and asserts that InstallSchema adds the
// column rather than leaving it.
func TestInstallSchemaAddsANullableColumnToAnAlreadyInstalledTable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	name := fmt.Sprintf("ab4_premigration_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name); err != nil {
			t.Logf("dropping %s: %v", name, err)
		}
	})
	cfg, err := pgxpool.ParseConfig(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	old, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	defer old.Close()

	// The pre-AB.4 shape, written out rather than derived: a fixture generated
	// from today's model could not represent yesterday's.
	for _, stmt := range []string{
		"CREATE SCHEMA IF NOT EXISTS jetsapi",
		`CREATE TABLE jetsapi.incident (
		   incident_id            text PRIMARY KEY,
		   incident_session_id    text NOT NULL,
		   incident_detected_at   timestamp with time zone NOT NULL,
		   incident_locus         text NOT NULL,
		   classification         text,
		   severity               text NOT NULL,
		   status                 text NOT NULL,
		   incident_step_ref      text,
		   incident_shard_ref     bigint,
		   incident_confounders   text[] NOT NULL,
		   incident_model_version text NOT NULL)`,
	} {
		if _, err := old.Exec(ctx, stmt); err != nil {
			t.Fatalf("building the pre-AB.4 schema: %v", err)
		}
	}
	if hasIncidentRunRef(t, old) {
		t.Fatal("the fixture already has the column; this test would prove nothing")
	}

	if err := InstallSchema(ctx, old); err != nil {
		t.Fatalf("installing over the pre-AB.4 schema: %v", err)
	}
	if !hasIncidentRunRef(t, old) {
		t.Fatal("InstallSchema left jetsapi.incident without incident_run_ref; a column " +
			"added to the model reaches a fresh database and not a migrated one (I-337)")
	}
	// Idempotent: the second run is what a deployment actually does, every
	// `update_db -migrateDb`.
	if err := InstallSchema(ctx, old); err != nil {
		t.Fatalf("second install: %v", err)
	}
}

func hasIncidentRunRef(t *testing.T, db *pgxpool.Pool) bool {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(),
		`SELECT count(*) FROM information_schema.columns
		  WHERE table_schema = 'jetsapi' AND table_name = 'incident'
		    AND column_name = 'incident_run_ref'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

// The generated DDL must carry the ALTER for the column this phase added, which
// is a cheaper check than the one above and fails for a different reason: that
// one fails when the statements do not run, this one when the emitter stops
// producing them.
func TestTheGeneratedDDLCarriesTheAdditiveMigration(t *testing.T) {
	want := "ALTER TABLE jetsapi.incident ADD COLUMN IF NOT EXISTS incident_run_ref text;"
	if !strings.Contains(schemaSQL, want) {
		t.Errorf("the generated DDL does not contain %q", want)
	}
	// Never for a NOT NULL column: adding one to a populated table needs a
	// default, which is a decision no emitter is entitled to take.
	if strings.Contains(schemaSQL, "ADD COLUMN IF NOT EXISTS incident_session_id") {
		t.Error("the emitter produced an ALTER for a NOT NULL column")
	}
}
