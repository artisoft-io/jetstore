// Criterion 16 of the agentic_ai Phase 1 plan, tested against a real Postgres:
// one run produces intent, tool_call, decision and a terminal outcome with the
// hash chain intact, and the intent row survives a kill between the commit and
// the first model call.
//
// Needs JETS_TEST_DSN, the same throwaway database the audit suite uses;
// skipped otherwise. Locally:
//
//	docker run -d --rm -e POSTGRES_PASSWORD=pw -p 5455:5432 postgres:16-alpine
//	JETS_TEST_DSN=postgres://postgres:pw@localhost:5455/postgres go test ./jets/agentic/agent/
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/infer"
	"github.com/artisoft-io/jetstore/jets/agentic/tools"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("JETS_TEST_DSN")
	if dsn == "" {
		t.Skip("JETS_TEST_DSN not set; needs a throwaway Postgres")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := audit.InstallSchema(context.Background(), pool); err != nil {
		t.Fatalf("installing schema: %v", err)
	}
	return pool
}

func newRun(t *testing.T, pool *pgxpool.Pool) *PgRecorder {
	t.Helper()
	id := fmt.Sprintf("run-%d", time.Now().UnixNano())
	return &PgRecorder{DB: pool, Run: audit.Run{
		RunId: id, AgentId: "agent:test", AgentVersion: "0.1.0",
		ModelId: "test-model", PromptVersion: "p1", Tier: "T1",
		DomainModelVersion: "0.1.0", IterationCap: 3, WallClockCapSeconds: 60,
	}}
}

func eventTypes(t *testing.T, pool *pgxpool.Pool, runId string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`select event_type from jetsapi.agent_audit where run_id = $1 order by seq`, runId)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// Criterion 16, first clause: a whole run is on the transcript, in order.
func TestCriterion16_RunIsAuditableEndToEnd(t *testing.T) {
	pool := testPool(t)
	rec := newRun(t, pool)

	in := &stubInfer{answers: []string{`{"a":1}`}, tokens: 7}
	reg := &stubRegistry{reports: []any{&tools.ValidationReport{Valid: true}}}
	l := &Loop{
		Infer: in, Registry: reg, Audit: rec, Recorder: rec,
		Budget: Budget{MaxIterations: 3},
		RunId:  rec.Run.RunId, Actor: "agent:test", Tier: "T1",
	}

	res, err := l.Run(context.Background(), task())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != OutcomeSucceeded {
		t.Fatalf("outcome = %q", res.Outcome)
	}

	got := eventTypes(t, pool, rec.Run.RunId)
	want := []string{audit.EventIntent, audit.EventDecision, audit.EventToolCall, audit.EventOutcome}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %q, want %q", i, got[i], want[i])
		}
	}

	// The run row carries what it spent, which is what criterion 17 will read.
	var status string
	var spend int
	if err := pool.QueryRow(context.Background(),
		`select run_status, token_spend from jetsapi.agent_run where run_id = $1`,
		rec.Run.RunId).Scan(&status, &spend); err != nil {
		t.Fatalf("reading the run row: %v", err)
	}
	if status != string(OutcomeSucceeded) || spend != 14 {
		t.Errorf("run row has (%s, %d), want (succeeded, 14)", status, spend)
	}
}

// Criterion 16, second clause and the point of the contract: the intent is
// durable before anything acts. The model call is what "acting" means here, so
// a client that panics on its first call stands in for the process dying at
// exactly the worst moment.
func TestCriterion16_IntentSurvivesAKillBeforeTheFirstCall(t *testing.T) {
	pool := testPool(t)
	rec := newRun(t, pool)

	l := &Loop{
		Infer: panicInfer{}, Registry: &stubRegistry{}, Audit: rec, Recorder: rec,
		Budget: Budget{MaxIterations: 3},
		RunId:  rec.Run.RunId, Actor: "agent:test", Tier: "T1",
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected the stand-in for a dying process to panic")
			}
		}()
		_, _ = l.Run(context.Background(), task())
	}()

	got := eventTypes(t, pool, rec.Run.RunId)
	if len(got) != 1 || got[0] != audit.EventIntent {
		t.Fatalf("events = %v; the intent must be durable before the first model call", got)
	}
	// And the run it records is there too, from the same transaction. An
	// intent with no run would mean the two could disagree, which is the thing
	// one transaction exists to prevent.
	var n int
	if err := pool.QueryRow(context.Background(),
		`select count(*) from jetsapi.agent_run where run_id = $1`, rec.Run.RunId).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("found %d run rows, want 1 committed alongside the intent", n)
	}
}

// A run that cannot be recorded must not act. The alternative — proceeding
// unrecorded — is the one failure mode write-before-act exists to prevent, so
// it is a hard error rather than a logged one.
func TestStart_FailureStopsTheRunBeforeAnyCall(t *testing.T) {
	in := &stubInfer{answers: []string{`{"a":1}`}}
	l := &Loop{
		Infer: in, Registry: &stubRegistry{}, Recorder: failingRecorder{},
		Budget: Budget{MaxIterations: 2}, RunId: "run-x",
	}
	if _, err := l.Run(context.Background(), task()); err == nil {
		t.Fatal("expected the run to be refused")
	}
	if len(in.seen) != 0 {
		t.Errorf("made %d model calls after failing to record the intent; none is permitted", len(in.seen))
	}
}

// Finishing a run nobody started is an error rather than a silent no-op: it is
// how a caller that skipped StartRun would otherwise go unnoticed.
func TestFinishRun_WithoutStartIsAnError(t *testing.T) {
	pool := testPool(t)
	err := audit.FinishRun(context.Background(), pool, "run-never-started", "succeeded", 0)
	if err == nil {
		t.Fatal("expected finishing an unstarted run to fail")
	}
}

// StartRun refuses an empty intent: recording that something happened, without
// recording what was about to happen, is not the contract.
func TestStartRun_RequiresAnIntent(t *testing.T) {
	pool := testPool(t)
	run := newRun(t, pool).Run
	if err := audit.StartRun(context.Background(), pool, &run, nil); err == nil {
		t.Error("expected an empty intent to be refused")
	}
}

type panicInfer struct{}

func (panicInfer) Chat(context.Context, *infer.Request) (*infer.Response, error) {
	panic("the process died on its first model call")
}

type failingRecorder struct{}

func (failingRecorder) Start(context.Context, []byte) error {
	return errors.New("the database is unreachable")
}
func (failingRecorder) Finish(context.Context, Outcome, int) error { return nil }
