package main

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/shadow"
	"github.com/artisoft-io/jetstore/jets/compute_pipes"
	"github.com/artisoft-io/jetstore/jets/schema"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The incident the supervision screens render, produced by triage rather than
// typed (task AC.3, criterion 50's fourth clause).
//
// **Criterion 50 asks for the screens to show a supervisor an incident and its
// ranked hypotheses "over an incident triage wrote", and until this file the
// screen tests rendered a hand-written row.** A hand-written fixture is a fixture
// its author agreed with by construction, which is I-147's shape at the wire: the
// stub is shaped like the caller, so it passes whatever the caller does.
//
// So the fixture is generated here — a real execution record, `shadow.Writer`
// classifying and ranking it, `getIncident` projecting the result — and committed
// where the browser suite imports it. **It is a golden test rather than a
// generator**: it fails when what triage writes stops matching what the screen is
// tested against, which is the only version of this that keeps working after the
// day it was written. Regenerate with UPDATE_GOLDEN=1 and read the diff.
//
// The bound, stated because the criterion's own text invites the confusion: this
// establishes that the *payload* the screen is tested against is one triage
// produced. **No browser has rendered it and no live apiserver has served it**,
// which is the same distance criterion 31 spent two phases crossing and which
// nothing here shortens.
const goldenIncidentPath = "../../jetsclient_ide/src/incidents/__fixtures__/triage_written_incident.json"

var goldenNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

func TestTheScreensFixtureIsAnIncidentTriageWrote(t *testing.T) {
	if os.Getenv("JETS_TEST_DSN") == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	ctx := context.Background()
	pool := goldenDB(t, "shadow_golden")
	goldenRecord(t, pool, "s-golden")

	w := shadow.DefaultWriter()
	w.Actor = "triage@1"
	w.Baseline = 0
	w.Now = func() time.Time { return goldenNow }
	res, err := w.Run(ctx, pool, "s-golden")
	if err != nil {
		t.Fatalf("shadow.Writer.Run: %v", err)
	}
	var subject string
	for _, wr := range res.Written {
		if wr.Status == audit.IncidentDiagnosed && wr.Hypotheses > 1 {
			subject = wr.IncidentId
			break
		}
	}
	if subject == "" {
		t.Fatalf("no incident reached `diagnosed` with more than one hypothesis; the fixture cannot "+
			"exercise a ranking. Written: %+v", res.Written)
	}

	// Through the endpoint's own projection, so the committed shape is the wire
	// shape and not a second statement of it.
	payload, code, err := agenticDispatch(ctx, pgOps{pool},
		&AgenticAction{Action: "get_incident", IncidentId: subject}, "supervisor@example.com",
		audit.DisclosePHI)
	if err != nil || code != 200 {
		t.Fatalf("get_incident returned %d: %v", code, err)
	}
	got, err := json.MarshalIndent(*payload, "", "  ")
	if err != nil {
		t.Fatalf("marshalling the payload: %v", err)
	}
	got = append(got, '\n')

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(goldenIncidentPath), 0o755); err != nil {
			t.Fatalf("creating the fixture directory: %v", err)
		}
		if err := os.WriteFile(goldenIncidentPath, got, 0o644); err != nil {
			t.Fatalf("writing %s: %v", goldenIncidentPath, err)
		}
		t.Logf("regenerated %s (%d bytes)", goldenIncidentPath, len(got))
		return
	}
	want, err := os.ReadFile(goldenIncidentPath)
	if err != nil {
		t.Fatalf("reading %s: %v — run with UPDATE_GOLDEN=1 to create it", goldenIncidentPath, err)
	}
	if string(got) != string(want) {
		t.Errorf("what triage writes no longer matches the fixture the browser suite renders.\n"+
			"Run with UPDATE_GOLDEN=1, read the diff, and check the screen tests still assert what "+
			"they mean to.\n--- got ---\n%s", got)
	}

	// The two things criterion 50's first clause is about, asserted on the
	// generated payload rather than on a fixture: the locus is there and the
	// cause is not claimed. The screen test asserts how they are rendered.
	var decoded map[string]any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("decoding the payload: %v", err)
	}
	inc := decoded["incident"].(map[string]any)
	if inc["locus"] == "" {
		t.Error("the generated incident carries no locus")
	}
	if inc["classification"] != "" {
		t.Errorf("the generated incident claims a cause (%v); triage evidences a locus and never a "+
			"cause (§9.5)", inc["classification"])
	}
	if n := len(decoded["transitions"].([]any)); n < 2 {
		t.Errorf("the payload carries %d transitions; a diagnosed incident has at least two, and "+
			"the ranking's basis rides the second (criterion 45)", n)
	}
}

func goldenDB(t *testing.T, name string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	u, err := url.Parse(os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("parsing JETS_TEST_DSN: %v", err)
	}
	admin, err := pgxpool.New(ctx, os.Getenv("JETS_TEST_DSN"))
	if err != nil {
		t.Fatalf("connecting to the maintenance database: %v", err)
	}
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		admin.Close()
		t.Fatalf("dropping %s: %v", name, err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("creating %s: %v", name, err)
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
	want := map[string]bool{"pipeline_execution_status": true, "pipeline_execution_details": true}
	for i := range defs {
		if want[defs[i].TableName] {
			if err := defs[i].UpdateTableSchema(pool, false); err != nil {
				t.Fatalf("installing %s: %v", defs[i].TableName, err)
			}
		}
	}
	if err := audit.InstallSchema(ctx, pool); err != nil {
		t.Fatalf("installing the agentic DDL: %v", err)
	}
	return pool
}

// goldenRecord writes one failed run.
//
// **The worker rows go in through the cpipes node's own two statements** rather
// than through SQL this test composed, which is F104's point: a payload derived
// from rows a test typed agrees with the test that typed them. The run header is
// composed, because its production writer is the apiserver at submission and is
// not reachable without a state machine.
func goldenRecord(t *testing.T, pool *pgxpool.Pool, session string) {
	t.Helper()
	ctx := context.Background()
	start := goldenNow.Add(-time.Hour)
	var execKey int
	if err := pool.QueryRow(ctx, `INSERT INTO jetsapi.pipeline_execution_status
		(pipeline_config_key, client, process_name, main_object_type, session_id,
		 source_period_key, status, failure_details, user_email, start_time, last_update)
		VALUES (1, 'cgt', 'loader', 'claims', $1, 1, 'failed', '', 'test@test', $2, $2) RETURNING key`,
		session, start).Scan(&execKey); err != nil {
		t.Fatalf("inserting the run header: %v", err)
	}
	for _, w := range []struct {
		shard    int
		terminal string
		in, out  int
		errMsg   string
	}{
		{0, "failed", 5000, 0, "boom,bang"},
		{1, "", 0, 0, ""},
		{2, "completed", 8000, 40, ""},
	} {
		cp := &compute_pipes.ComputePipesContext{
			ComputePipesArgs: compute_pipes.ComputePipesArgs{
				ComputePipesNodeArgs: compute_pipes.ComputePipesNodeArgs{
					NodeId: w.shard, PipelineExecKey: execKey,
				},
				ComputePipesCommonArgs: compute_pipes.ComputePipesCommonArgs{
					Client: "cgt", ProcessName: "loader", SessionId: session,
					InputSessionId: session, SourcePeriodKey: 1, PipelineConfigKey: 1,
					UserEmail: "test@test", MainInputStepId: "reducing01",
				},
			},
		}
		key, err := cp.InsertPipelineExecutionStatus(pool)
		if err != nil {
			t.Fatalf("InsertPipelineExecutionStatus: %v", err)
		}
		if w.terminal != "" {
			if err := cp.UpdatePipelineExecutionStatus(pool, key, w.in, 0, 1, 1, 0, w.out,
				"reducing01", w.terminal, w.errMsg); err != nil {
				t.Fatalf("UpdatePipelineExecutionStatus: %v", err)
			}
		}
		if _, err := pool.Exec(ctx,
			`UPDATE jetsapi.pipeline_execution_details SET start_time = $1, last_update = $1 WHERE key = $2`,
			start, key); err != nil {
			t.Fatalf("backdating the worker row: %v", err)
		}
	}
}
