// The floor end to end against a real Postgres: the nine predicates over rows
// the cpipes writers wrote, the detectors' anomalies read back through
// observe.ReadAnomalies, and the hypotheses written into jetsapi.hypothesis and
// read out again.
//
// # What is production here and what is not
//
//   - The execution tables are installed from jets/jets_schema.json by the same
//     jets/schema code `update_db -migrateDb` runs, and jetsapi.anomaly,
//     jetsapi.incident and jetsapi.hypothesis by audit.InstallSchema, which is
//     the same call update_db makes (jets/update_db/main.go:71).
//   - Worker rows are written by ComputePipesContext.InsertPipelineExecutionStatus
//     and UpdatePipelineExecutionStatus; DAG edges by AggregateChannelResults and
//     InsertChannelExecutionDetails. This is triage's arrangement and the reason
//     is theirs: the fold that blanks output_entity at output_sinks_count > 1 is
//     a real fold rather than a blank string a test typed.
//   - Anomalies are written by observe.InsertAnomaly, the detectors' own writer,
//     and read by observe.ReadAnomalies rather than by a query composed here.
//     **That is deliberate and it is I-147's lesson**: a fixture shaped like the
//     caller agrees with the caller by construction, and the read path this
//     package depends on had no consumer before this task.
//   - Run headers, process_errors rows and cpipes_execution_status rows are
//     composed here, for triage's reason: their production writers are the
//     apiserver at submission, a channel-fed table writer and the sharding
//     start, none of which is reachable without S3, a compiled workspace and a
//     state machine.
//
// Needs JETS_TEST_DSN; skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5461:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5461/postgres go test ./jets/agentic/rca/
//
// Use `go test -p 1` when running this beside jets/agentic/{observe,triage,audit}
// against one DSN: they all call audit.InstallSchema and two sessions issuing the
// same CREATE TABLE / CREATE TRIGGER sequence deadlock on catalogue locks (I-175,
// I-291). This package's own database is created per run, which removes it here
// and not across packages.
package rca

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
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaPath = "../../jets_schema.json"

var migratedTables = []string{
	"pipeline_execution_status", "pipeline_execution_details",
	"pipeline_execution_channel_details", "process_errors", "cpipes_execution_status",
}

var now = time.Now().UTC().Truncate(time.Second)

func freshDB(t *testing.T, dbName string) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()

	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)"); err != nil {
		admin.Close()
		t.Fatalf("dropping %s: %v", dbName, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+dbName); err != nil {
		admin.Close()
		t.Fatalf("creating %s: %v", dbName, err)
	}
	admin.Close()

	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	u.Path = "/" + dbName
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("connecting to %s: %v", dbName, err)
	}
	t.Cleanup(pool.Close)

	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("opening %s: %v", schemaPath, err)
	}
	defer f.Close()
	var defs []schema.TableDefinition
	if err := json.NewDecoder(f).Decode(&defs); err != nil {
		t.Fatalf("decoding %s: %v", schemaPath, err)
	}
	want := map[string]bool{}
	for _, n := range migratedTables {
		want[n] = true
	}
	found := 0
	for i := range defs {
		if !want[defs[i].TableName] {
			continue
		}
		found++
		if err := defs[i].UpdateTableSchema(pool, false); err != nil {
			t.Fatalf("installing %s: %v", defs[i].TableName, err)
		}
	}
	if found != len(migratedTables) {
		t.Fatalf("found %d of the %d requested tables in %s", found, len(migratedTables), schemaPath)
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

func header(t *testing.T, pool *pgxpool.Pool, client, process, objectType, session, status string,
	start time.Time) int {
	t.Helper()
	var key int
	if err := pool.QueryRow(context.Background(), `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id,
		 source_period_key, status, failure_details, user_email, start_time, last_update)
		VALUES (1, $1, $2, $3, $4, 1, $5, $6, 'test@test', $7, $7) RETURNING key`,
		client, process, objectType, session, status, "ECS task stopped: OutOfMemoryError", start).
		Scan(&key); err != nil {
		t.Fatalf("inserting run header: %v", err)
	}
	return key
}

func workerRow(t *testing.T, pool *pgxpool.Pool, execKey int, client, process, session, stepId string,
	shard int, terminal string, in, bad, out int, errMsg string, start time.Time) int64 {
	t.Helper()
	cpCtx := &compute_pipes.ComputePipesContext{
		ComputePipesArgs: compute_pipes.ComputePipesArgs{
			ComputePipesNodeArgs: compute_pipes.ComputePipesNodeArgs{
				NodeId:          shard,
				PipelineExecKey: execKey,
			},
			ComputePipesCommonArgs: compute_pipes.ComputePipesCommonArgs{
				Client:            client,
				ProcessName:       process,
				SessionId:         session,
				InputSessionId:    session,
				SourcePeriodKey:   1,
				PipelineConfigKey: 1,
				UserEmail:         "test@test",
				MainInputStepId:   stepId,
			},
		},
	}
	key, err := cpCtx.InsertPipelineExecutionStatus(pool)
	if err != nil {
		t.Fatalf("InsertPipelineExecutionStatus: %v", err)
	}
	if terminal != "" {
		if err := cpCtx.UpdatePipelineExecutionStatus(pool, key,
			in, bad, 1, 1, 0, out, stepId, terminal, errMsg); err != nil {
			t.Fatalf("UpdatePipelineExecutionStatus: %v", err)
		}
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE jetsapi.pipeline_execution_details SET start_time = $1, last_update = $1 WHERE key = $2`,
		start, key); err != nil {
		t.Fatalf("backdating worker row: %v", err)
	}
	return int64(key)
}

func processErrors(t *testing.T, pool *pgxpool.Pool, execKey int, session string, n, withColumn int) {
	t.Helper()
	for i := 0; i < n; i++ {
		col := ""
		if i < withColumn {
			col = "member_dob"
		}
		if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.process_errors
			(pipeline_execution_status_key, session_id, row_jets_key, input_column, error_message, shard_id)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			execKey, session, fmt.Sprintf("k%d", i), col, "value is not a valid date", i%2); err != nil {
			t.Fatalf("inserting process_errors row: %v", err)
		}
	}
}

// A configuration with one map_record instance and no error channel, which is
// what 243 of the corpus's 243 instances look like (F188).
const configNoErrorChannel = `{"pipes_config":[{"type":"fan_out","input_channel":{"name":"input"},
  "transformation_pipes":[{"type":"map_record","column_evaluators":[],
    "output_channel":{"name":"output"}}]}], "on_error":"drop"}`

func storeConfig(t *testing.T, pool *pgxpool.Pool, execKey int, session, config string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `INSERT INTO jetsapi.cpipes_execution_status
		(pipeline_execution_status_key, session_id, cpipes_config_json) VALUES ($1, $2, $3)`,
		execKey, session, config); err != nil {
		t.Fatalf("inserting cpipes_execution_status row: %v", err)
	}
}

// rank gathers, classifies and ranks one session the way a caller would.
func rank(t *testing.T, pool *pgxpool.Pool, session string) (*triage.Report, *Ranking) {
	t.Helper()
	ctx := context.Background()
	ev, err := triage.Gather(ctx, pool, session, 0)
	if err != nil {
		t.Fatalf("triage.Gather: %v", err)
	}
	rep := triage.Default().Classify(ev)
	anomalies, err := observe.ReadAnomalies(ctx, pool, session)
	if err != nil {
		t.Fatalf("observe.ReadAnomalies: %v", err)
	}
	r := Default().Rank(&Input{Report: rep, Evidence: ev, Anomalies: anomalies})
	if err := r.Validate(); err != nil {
		t.Fatalf("the ranking does not validate: %v", err)
	}
	return rep, r
}

// The end-to-end case: a run that failed in the ordinary way, classified by the
// nine predicates over the cpipes writers' own rows, ranked over the gate's
// table, with a detector's anomaly read back from the table it was written to.
func TestRankingOverARealRecord(t *testing.T) {
	pool := freshDB(t, "rca_e2e")
	ctx := context.Background()

	k := header(t, pool, "cgt", "loader", "claims", "s-multi", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-multi", "reducing01", 0, "failed", 5000, 12, 0,
		"download failed,rules pool exhausted", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-multi", "reducing01", 1, "", 0, 0, 0, "",
		now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-multi", "reducing01", 2, "completed", 8000, 3, 40, "",
		now.Add(-time.Hour))
	processErrors(t, pool, k, "s-multi", 20, 6)
	storeConfig(t, pool, k, "s-multi", configNoErrorChannel)

	// A detector's own output, written by the detectors' writer.
	if err := observe.InsertAnomaly(ctx, pool, &observe.Anomaly{
		AnomalyId: "an-1", DetectedAt: now, SessionId: "s-multi",
		SubjectType: observe.SubjectWorker, SubjectRef: "reducing01/2",
		SignalType: observe.SignalVolume, ObservedValue: "0.005",
		ExpectedBasis: "within-run input against output on the worker row",
		Confounders:   []string{observe.ConfounderOnErrorDrop, observe.ConfounderMergeRowCountUnknown},
		DetectorRef:   "volume_collapse@1",
	}); err != nil {
		t.Fatalf("InsertAnomaly: %v", err)
	}

	rep, r := rank(t, pool, "s-multi")
	t.Logf("triage: %d of 9 evaluable, loci present: %v", rep.Evaluable, rep.Loci())
	t.Logf("ranking basis: %s", r.Basis)
	for i := range r.Hypotheses {
		h := &r.Hypotheses[i]
		t.Logf("  %2d. %-26s at %-34s conf %.2f (+%d/-%d)", h.Rank, h.CauseCategory, h.Locus,
			h.Confidence, len(h.SupportingEvidence), len(h.ContradictingEvidence))
	}
	if len(r.Hypotheses) == 0 {
		t.Fatal("a run with a failed worker, a stalled worker, a volume collapse and 20 per-record " +
			"errors produced no hypothesis")
	}

	// Every hypothesis carries both sides, and the detector's confounders are on
	// the side the split puts them: on_error_drop is the case *for* benign and
	// against the rest; merge_row_count_unknown is against all.
	benignSeen := false
	for i := range r.Hypotheses {
		h := &r.Hypotheses[i]
		if len(h.SupportingEvidence) == 0 || h.ContradictingEvidence == nil {
			t.Errorf("%s at %s: +%d/-%v", h.CauseCategory, h.Locus, len(h.SupportingEvidence),
				h.ContradictingEvidence)
		}
		if h.Locus != LocusOf(h) {
			t.Errorf("%s carries locus %q", h.CauseCategory, h.Locus)
		}
		if h.Locus == triage.LocusRowsLostSilently {
			if !citesConfounder(h.ContradictingEvidence, observe.ConfounderMergeRowCountUnknown) {
				t.Errorf("%s at %s does not carry the detector's merge_row_count_unknown against it",
					h.CauseCategory, h.Locus)
			}
			if h.CauseCategory == CauseBenignVariation {
				benignSeen = true
				if !citesConfounder(h.SupportingEvidence, observe.ConfounderOnErrorDrop) {
					t.Error("benign_variation does not cite the configured on_error_drop as its case")
				}
			}
		}
	}
	if !benignSeen {
		t.Error("no benign_variation hypothesis over a present rows_lost_silently")
	}

	// The corroborators §9.5 names as the record's instruments are read off the
	// evidence rather than asserted: input_bad_records_count for parse_failure,
	// the rete-session and input_column counts for the two classes that have
	// one, failure_details for infrastructure_failure.
	wantCorroborator := map[string]string{
		CauseParseFailure:          "input_bad_records_count sums to 15",
		CauseSourceContentChange:   "6 of 20 process_errors rows name an input_column",
		CauseInfrastructureFailure: "characters of failure_details",
	}
	for class, want := range wantCorroborator {
		found := false
		for i := range r.Hypotheses {
			if r.Hypotheses[i].CauseCategory != class {
				continue
			}
			for _, e := range r.Hypotheses[i].SupportingEvidence {
				if strings.Contains(e.Statement, want) {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("no %s hypothesis carries the corroborator %q that §9.5 names as its instrument",
				class, want)
		}
	}
}

// LocusOf is a test helper that re-derives a hypothesis's locus from its id, so
// that the field and the identity cannot silently disagree.
func LocusOf(h *Hypothesis) string {
	parts := strings.Split(h.HypothesisId, "/")
	if len(parts) != 3 {
		return ""
	}
	return parts[1]
}

// **The evidence a hypothesis carries must be writable into jetsapi.hypothesis,
// and readable back as the same three fields.** This is triage's
// TestFindingConfoundersAreAcceptedByTheIncidentTable one entity over, and it
// bears on a column the two Go types cannot check between them: both evidence
// columns are jsonb rather than text[], because an Evidence is a value object
// with three fields and text[] — the emitter's unmapped-type fall-through —
// would have stored three fields as one opaque string per item (I-287).
//
// **Writing rows here is not AC.3.** Nothing in this package writes; the test
// composes the insert to establish that what the ranker emits fits the column,
// which is the F68 lesson applied to a payload rather than to a table name.
func TestHypothesesAreWritableAndReadBackWholeAlbeitByATest(t *testing.T) {
	pool := freshDB(t, "rca_writeback")
	ctx := context.Background()

	k := header(t, pool, "usi", "qc", "members", "s-w", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "usi", "qc", "s-w", "reducing01", 0, "failed", 100, 4, 0, "boom",
		now.Add(-time.Hour))
	storeConfig(t, pool, k, "s-w", configNoErrorChannel)
	_, r := rank(t, pool, "s-w")
	if len(r.Hypotheses) == 0 {
		t.Fatal("no hypotheses to write")
	}

	for i := range r.Hypotheses {
		h := &r.Hypotheses[i]
		sup, err := json.Marshal(evidenceJSON(h.SupportingEvidence))
		if err != nil {
			t.Fatalf("marshalling supporting evidence: %v", err)
		}
		con, err := json.Marshal(evidenceJSON(h.ContradictingEvidence))
		if err != nil {
			t.Fatalf("marshalling contradicting evidence: %v", err)
		}
		// hypothesis_locus and basis are columns as of AC.3 (Q-46, answered by
		// the user 2026-09-04). This test wrote its rows before they existed;
		// it now writes the locus the ranker actually derived from and a basis
		// computed from the two slices being marshalled above, which is what
		// the shadow writer does one package over.
		basis, err := json.Marshal(map[string]any{
			"supporting_count":    len(h.SupportingEvidence),
			"contradicting_count": len(h.ContradictingEvidence),
			"evidenceability":     string(EvidenceabilityOf(h.CauseCategory)),
		})
		if err != nil {
			t.Fatalf("marshalling basis: %v", err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO jetsapi.hypothesis
			(hypothesis_id, hypothesis_incident_ref, cause, cause_category, confidence, rank,
			 supporting_evidence, contradicting_evidence, hypothesis_locus, basis)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			h.HypothesisId, "incident-for-"+r.SessionId, h.Cause, h.CauseCategory, h.Confidence,
			h.Rank, sup, con, h.Locus, basis); err != nil {
			t.Fatalf("inserting hypothesis %s: %v", h.HypothesisId, err)
		}
	}

	// Read them back in the order the index exists for, and check one item's
	// three fields survived the jsonb round trip.
	rows, err := pool.Query(ctx, `SELECT hypothesis_id, cause_category, rank,
		supporting_evidence, contradicting_evidence FROM jetsapi.hypothesis
		WHERE hypothesis_incident_ref = $1 ORDER BY rank`, "incident-for-"+r.SessionId)
	if err != nil {
		t.Fatalf("reading hypotheses back: %v", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id string
		var category *string
		var rank int64
		var sup, con []map[string]any
		if err := rows.Scan(&id, &category, &rank, &sup, &con); err != nil {
			t.Fatalf("scanning: %v", err)
		}
		n++
		if int(rank) != n {
			t.Errorf("row %d has rank %d; the index reads them in rank order", n, rank)
		}
		if len(sup) == 0 {
			t.Errorf("%s came back with no supporting evidence", id)
		}
		for _, e := range sup {
			if e["statement"] == nil || e["source"] == nil {
				t.Errorf("%s: an evidence item lost a field in the round trip: %v", id, e)
			}
		}
		if con == nil {
			t.Errorf("%s: contradicting_evidence came back null rather than as a list", id)
		}
	}
	if n != len(r.Hypotheses) {
		t.Errorf("wrote %d hypotheses and read %d back", len(r.Hypotheses), n)
	}
	t.Logf("%d hypotheses written to jetsapi.hypothesis and read back with their evidence whole", n)
}

func evidenceJSON(items []Evidence) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, e := range items {
		out = append(out, map[string]any{
			"statement": e.Statement, "source": e.Source, "source_ref": e.SourceRef,
		})
	}
	return out
}

// **The negative control the deployment supplies for free.** On a database that
// has not been migrated, four of the nine loci report not_evaluable, and a
// ranker that treated an unanswerable locus as an absent one would emit the
// same hypotheses as on a migrated one — silently, with no signal that half the
// evidence positions were never read.
func TestAnUnaskedLocusIsNotAnAbsentOne(t *testing.T) {
	pool := freshDB(t, "rca_unmigrated")
	ctx := context.Background()

	k := header(t, pool, "cgt", "loader", "claims", "s-u", "failed", now.Add(-time.Hour))
	workerRow(t, pool, k, "cgt", "loader", "s-u", "reducing01", 0, "failed", 100, 0, 0, "boom",
		now.Add(-time.Hour))

	// Take away the two tables an ordinary deployment lacks, after the rows are
	// written, so this is the same run seen by two deployments.
	fullRep, full := rank(t, pool, "s-u")
	if _, err := pool.Exec(ctx,
		`DROP TABLE jetsapi.pipeline_execution_channel_details, jetsapi.process_errors`); err != nil {
		t.Fatalf("dropping the two tables: %v", err)
	}
	_, thin := rank(t, pool, "s-u")

	if len(fullRep.Loci()) == 0 {
		t.Fatal("the migrated classification fired nothing")
	}
	if len(thin.UnaskedLoci) < 3 {
		t.Errorf("only %d loci are unaskable on the thin deployment: %v", len(thin.UnaskedLoci),
			thin.UnaskedLoci)
	}
	bounded := 0
	for i := range thin.Hypotheses {
		for _, e := range thin.Hypotheses[i].ContradictingEvidence {
			if strings.Contains(e.Statement, "bounded rather than complete") {
				bounded++
			}
		}
	}
	if bounded == 0 {
		t.Error("no hypothesis on the thin deployment says its case is bounded; an unasked evidence " +
			"position was read as an absent one")
	}
	if !strings.Contains(thin.Basis, "could not be evaluated") {
		t.Errorf("the ranking's basis does not report the unasked loci: %s", thin.Basis)
	}
	t.Logf("migrated: %d hypotheses, %d unasked loci; thin: %d hypotheses, %d unasked loci, "+
		"%d bounded-case items", len(full.Hypotheses), len(full.UnaskedLoci),
		len(thin.Hypotheses), len(thin.UnaskedLoci), bounded)
}
