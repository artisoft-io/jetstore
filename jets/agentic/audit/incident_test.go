package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The incident read path (task AE.1).
//
// **Three of these need no database and one needs a database with *no* schema**,
// which is the unusual one and the reason it is here. Every other test in this
// package installs the schema before it asks anything, so nothing in the suite
// could ever have observed what a query does on a database where the migration
// has not been run — which is the state every existing deployment is in, because
// `jetsapi.incident` arrived at AB.1 and reaches a database only through
// `update_db -migrateDb` (P3 I-169). That is P3 F107's lesson repeated
// deliberately rather than rediscovered: a test that arranges the world before
// asking about the world cannot test a report about the world.

// The Go vocabulary and the generated CHECK must agree, on
// TestApprovalStatesMatchTheGeneratedCheck's argument: a value this package
// offers as a filter that the CHECK does not have is a filter that matches
// nothing, and it fails silently rather than loudly.
func TestIncidentLociMatchTheGeneratedCheck(t *testing.T) {
	assertVocabularyMatchesCheck(t, `incident_locus_ck CHECK \(incident_locus IN \(([^)]*)\)\)`,
		IncidentLoci, "IncidentLoci")
}

func TestIncidentStatusesMatchTheGeneratedCheck(t *testing.T) {
	assertVocabularyMatchesCheck(t, `incident_status_ck CHECK \(status IN \(([^)]*)\)\)`,
		IncidentStatuses, "IncidentStatuses")
}

func assertVocabularyMatchesCheck(t *testing.T, pattern string, vocab []string, name string) {
	t.Helper()
	sql, err := os.ReadFile("agent_audit.sql")
	if err != nil {
		t.Fatalf("reading agent_audit.sql: %v", err)
	}
	m := regexp.MustCompile(pattern).FindSubmatch(sql)
	if m == nil {
		t.Fatalf("could not find the CHECK matching %q; this test is stale", pattern)
	}
	inCheck := map[string]bool{}
	for _, lit := range strings.Split(string(m[1]), ",") {
		inCheck[strings.Trim(strings.TrimSpace(lit), "'")] = true
	}
	if len(inCheck) != len(vocab) {
		t.Errorf("the CHECK has %d values and %s has %d", len(inCheck), name, len(vocab))
	}
	for _, v := range vocab {
		if !inCheck[v] {
			t.Errorf("%q is in %s and not in the CHECK", v, name)
		}
		delete(inCheck, v)
	}
	for v := range inCheck {
		t.Errorf("%q is in the CHECK and not in %s", v, name)
	}
}

// A filter naming a status that does not exist is refused rather than silently
// matching nothing. An empty list and "there are no incidents" look identical on
// a screen, and only one of them is true.
func TestListIncidentsRefusesAnUnknownStatus(t *testing.T) {
	_, err := ListIncidents(context.Background(), nil, []string{"detected", "on_fire"}, 0)
	if err == nil {
		t.Fatal("an unknown status was accepted")
	}
	if !strings.Contains(err.Error(), "on_fire") {
		t.Errorf("the error should name the offending value; got %v", err)
	}
}

func TestReadIncidentRequiresAnId(t *testing.T) {
	if _, err := ReadIncident(context.Background(), nil, "", DisclosePHI); err == nil {
		t.Fatal("an empty incident id was accepted")
	}
}

// insertIncident writes one incident directly. The writer is AC.3's and does not
// exist, so the fixture is SQL — and this test says so rather than implying the
// rows came from anywhere else, which is criterion 43's fourth clause and the
// reason it is graded not met.
func insertIncident(t *testing.T, pool *pgxpool.Pool, id, session, locus, classification, status string,
	stepRef *string, shard *int64, confounders []string, detected time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO jetsapi.incident
		   (incident_id, incident_session_id, incident_detected_at, incident_locus,
		    classification, severity, status, incident_step_ref, incident_shard_ref,
		    incident_confounders, incident_model_version)
		 VALUES ($1,$2,$3,$4,NULLIF($5,''),'high',$6,$7,$8,$9,'0.1.0')`,
		id, session, detected, locus, classification, status, stepRef, shard, confounders)
	if err != nil {
		t.Fatalf("inserting incident %s: %v", id, err)
	}
}

func insertHypothesis(t *testing.T, pool *pgxpool.Pool, id, incidentRef, cause, category string,
	rank int64, confidence float64, supporting, contradicting string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		// hypothesis_locus and basis are columns as of AC.3 (Q-46). The fixture
		// writes a locus from the same nine-value vocabulary the CHECK admits and
		// a basis whose counts are those of the two arrays above, so a read-path
		// test still exercises a row shaped like one the writer produces.
		`INSERT INTO jetsapi.hypothesis
		   (hypothesis_id, hypothesis_incident_ref, cause, cause_category, confidence, rank,
		    supporting_evidence, contradicting_evidence, hypothesis_locus, basis)
		 VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7::jsonb,$8::jsonb,$9,$10::jsonb)`,
		id, incidentRef, cause, category, confidence, rank, supporting, contradicting,
		LocusWorkerFailed, basisFor(supporting, contradicting, category))
	if err != nil {
		t.Fatalf("inserting hypothesis %s: %v", id, err)
	}
}

// The round trip, and the four distinctions the screen depends on surviving it:
// a NULL classification is not a claim, a NULL shard is not shard 0, an empty
// contradicting array is not an absent one, and the hypotheses come back in rank
// order rather than insertion order.
func TestReadIncidentCarriesBothVocabulariesAndBothEvidenceSides(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	id := fmt.Sprintf("inc_rt_%d", time.Now().UnixNano())
	session := id + "_s"
	step := "reducing00"
	var shard int64
	insertIncident(t, pool, id, session, LocusWorkerFailed, "", "triaged",
		&step, &shard, []string{"step_label_ambiguous", "history_truncated"}, time.Now().UTC())
	// Inserted in the wrong order on purpose: rank is what orders them.
	insertHypothesis(t, pool, id+"_h2", id, "the upstream feed changed", "source_content_change", 2, 0.2,
		`[{"statement":"volume down 90%","source":"run_telemetry"}]`, `[]`)
	insertHypothesis(t, pool, id+"_h1", id, "a step regressed", "", 1, 0.7,
		`[{"statement":"step 3 slower","source":"run_telemetry","source_ref":"sess/3"}]`,
		`[{"statement":"sampling cap was set","source":"detector_confounder","source_ref":"sampling_cap"}]`)

	// DisclosePHI: this test is about the round trip, and a redacted read
	// would assert nothing about whether the statement survived the jsonb.
	inc, err := ReadIncident(ctx, pool, id, DisclosePHI)
	if err != nil {
		t.Fatalf("reading %s: %v", id, err)
	}
	if inc.Locus != LocusWorkerFailed {
		t.Errorf("locus %q, want %q", inc.Locus, LocusWorkerFailed)
	}
	// The one that matters most: a locus with no cause claimed is a legitimate
	// state, and it must not arrive as a cause (I-289, R-27).
	if inc.Classification != "" {
		t.Errorf("classification %q, want empty — nothing claimed one", inc.Classification)
	}
	if inc.ShardRef == nil || *inc.ShardRef != 0 {
		t.Errorf("shard ref %v, want a pointer to 0 — shard 0 is a shard", inc.ShardRef)
	}
	if len(inc.Confounders) != 2 {
		t.Errorf("confounders %v, want two", inc.Confounders)
	}
	if inc.HypothesisCount != 2 || len(inc.Hypotheses) != 2 {
		t.Fatalf("got %d hypotheses, want 2", len(inc.Hypotheses))
	}
	if inc.Hypotheses[0].HypothesisId != id+"_h1" {
		t.Errorf("hypotheses are not in rank order: first is %s", inc.Hypotheses[0].HypothesisId)
	}
	if got := inc.Hypotheses[0].ContradictingEvidence; len(got) != 1 || got[0].Source != "detector_confounder" {
		t.Errorf("contradicting evidence %v, want the one detector_confounder item", got)
	}
	if got := inc.Hypotheses[0].SupportingEvidence; len(got) != 1 || got[0].SourceRef != "sess/3" {
		t.Errorf("supporting evidence %v, want source_ref sess/3", got)
	}
	// Empty is not absent: A.2.8 calls contradicting evidence a calibration
	// control, so "the agent asserts none" has to be distinguishable from "the
	// agent was never asked".
	if inc.Hypotheses[1].ContradictingEvidence == nil {
		t.Error("an empty contradicting_evidence array came back as nil")
	}
	if len(inc.Hypotheses[1].ContradictingEvidence) != 0 {
		t.Errorf("contradicting evidence %v, want empty", inc.Hypotheses[1].ContradictingEvidence)
	}
	if inc.Hypotheses[1].CauseCategory != "source_content_change" {
		t.Errorf("cause category %q", inc.Hypotheses[1].CauseCategory)
	}
	if inc.Hypotheses[0].CauseCategory != "" {
		t.Errorf("cause category %q, want empty", inc.Hypotheses[0].CauseCategory)
	}
}

func TestListIncidentsFiltersByStatusAndCountsHypotheses(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	open := fmt.Sprintf("inc_ls_open_%d", stamp)
	closed := fmt.Sprintf("inc_ls_closed_%d", stamp)
	insertIncident(t, pool, open, open+"_s", LocusRowsLostSilently, "benign_variation", "detected",
		nil, nil, []string{"sampling_cap"}, time.Now().UTC())
	insertIncident(t, pool, closed, closed+"_s", LocusWrittenNotArrived, "", "closed",
		nil, nil, []string{}, time.Now().UTC().Add(-time.Hour))
	insertHypothesis(t, pool, open+"_h", open, "sampling cap", "benign_variation", 1, 0.5, `[]`, `[]`)

	rows, err := ListIncidents(ctx, pool, []string{"detected"}, 0)
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	var seenOpen, seenClosed bool
	for _, r := range rows {
		if r.IncidentId == open {
			seenOpen = true
			if r.HypothesisCount != 1 {
				t.Errorf("hypothesis count %d, want 1", r.HypothesisCount)
			}
			if r.ShardRef != nil {
				t.Errorf("shard ref %v, want nil — this incident localises to no shard", *r.ShardRef)
			}
			if r.Classification != "benign_variation" {
				t.Errorf("classification %q", r.Classification)
			}
		}
		if r.IncidentId == closed {
			seenClosed = true
		}
	}
	if !seenOpen {
		t.Error("the detected incident is not in a list filtered to detected")
	}
	if seenClosed {
		t.Error("a closed incident appeared in a list filtered to detected")
	}
}

// The report about the world, on a world that has not been migrated.
//
// It builds a database with no schema in it, which is what every JetStore
// deployment older than AB.1 looks like, and asserts the error a screen can act
// on rather than the string a screen would otherwise print.
func TestListIncidentsReportsAnUnmigratedDatabase(t *testing.T) {
	pool := testPool(t) // skips without a DSN, and installs the schema in the *other* database
	ctx := context.Background()
	name := fmt.Sprintf("ae1_nomigration_%d", time.Now().UnixNano())
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
	bare, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to %s: %v", name, err)
	}
	defer bare.Close()

	_, err = ListIncidents(ctx, bare, nil, 0)
	var notDeployed *ErrTablesNotDeployed
	if !errors.As(err, &notDeployed) {
		t.Fatalf("want ErrTablesNotDeployed on an unmigrated database, got %v", err)
	}
	if !strings.Contains(err.Error(), "migrateDb") {
		t.Errorf("the error should name the remedy; got %v", err)
	}
	// The detail read screen reaches the same wall and must report the same
	// thing: a stale link and an unmigrated database are different failures.
	_, err = ReadIncident(ctx, bare, "inc_1", DisclosePHI)
	if !errors.As(err, &notDeployed) {
		t.Fatalf("ReadIncident: want ErrTablesNotDeployed, got %v", err)
	}
}

// basisFor builds the `basis` column for a fixture from the two evidence arrays
// it is written beside, so the counts describe the row rather than being typed
// (AC.3, Q-46). The evidenceability tier is left `none` for a fixture, which is
// the honest value for a hand-composed row: nothing here consulted plan §9.5.
func basisFor(supporting, contradicting, category string) string {
	count := func(raw string) int {
		var items []Evidence
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return 0
		}
		return len(items)
	}
	b, err := json.Marshal(HypothesisBasis{
		SupportingCount:    count(supporting),
		ContradictingCount: count(contradicting),
		Evidenceability:    "none",
	})
	if err != nil {
		return `{"supporting_count":0,"contradicting_count":0,"evidenceability":"none"}`
	}
	return string(b)
}
