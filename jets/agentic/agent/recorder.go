package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The loop's persistence, kept behind two small interfaces so the loop can be
// tested without Postgres and so the ordering the contract depends on is
// visible in the loop rather than hidden in a type assertion.

// Recorder makes a run durable before it acts and records how it ended.
//
// Start must return before the first model call. That is the whole of
// write-before-act: the commit inside it is the acknowledgement, and a process
// dying immediately afterwards leaves a durable record of what it was about to
// do. A loop with no Recorder does not persist and says so by having none —
// which is what the tests use, and what a dry run would.
type Recorder interface {
	Start(ctx context.Context, intent []byte) error
	Finish(ctx context.Context, outcome Outcome, tokenSpend int) error
}

// PgDB is what the Postgres recorder needs: enough to open a transaction, to
// exec, and to query one row. Satisfied by *pgxpool.Pool.
type PgDB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PgRecorder is the real implementation: jetsapi.agent_run for the summary,
// jetsapi.agent_audit for the trail.
type PgRecorder struct {
	DB  PgDB
	Run audit.Run
}

func (r *PgRecorder) Start(ctx context.Context, intent []byte) error {
	if r.Run.StartedAt.IsZero() {
		r.Run.StartedAt = time.Now().UTC()
	}
	return audit.StartRun(ctx, r.DB, &r.Run, intent)
}

func (r *PgRecorder) Finish(ctx context.Context, outcome Outcome, tokenSpend int) error {
	return audit.FinishRun(ctx, r.DB, r.Run.RunId, string(outcome), tokenSpend)
}

// Append satisfies Auditor, so one PgRecorder serves as both: the events of a
// run and the run itself belong to the same store and the same connection.
func (r *PgRecorder) Append(ctx context.Context, ev *audit.Event) error {
	_, err := audit.Append(ctx, r.DB, ev)
	return err
}

// intentPayload describes what a run is about to do. It is written before the
// first model call, so it can name the task and the budget but nothing about
// the outcome — that is the point.
func intentPayload(task *Task, budget Budget) []byte {
	b, _ := json.Marshal(map[string]any{
		"instruction":    task.Instruction,
		"verifier":       task.Verifier,
		"max_iterations": budget.MaxIterations,
	})
	return b
}
