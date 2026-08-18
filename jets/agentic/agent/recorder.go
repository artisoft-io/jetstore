package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	// Propose records what a successful run produced. It is called before
	// Finish and only on success, and it writes a row — never to git. Staged
	// branch writes arrive with the Phase-2 approval screens, because a
	// copilot that can commit before anyone can review the commit has the
	// supervision layer in the wrong order.
	Propose(ctx context.Context, artifact json.RawMessage) (string, error)
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

// Propose writes the run's output as a draft proposal. Draft is not a
// placeholder: Appendix A.2.10 requires tests, affected pipelines and an
// impact analysis, and an authoring run has none of them, so the state says
// truthfully that this is not yet a proposal anyone can approve.
func (r *PgRecorder) Propose(ctx context.Context, artifact json.RawMessage) (string, error) {
	id := fmt.Sprintf("chg_%s", strings.ToLower(strings.ReplaceAll(r.Run.RunId, "-", "")))
	p := &audit.Proposal{
		ProposalId:    id,
		Trigger:       "authoring_run",
		TriggerRef:    r.Run.RunId,
		Rationale:     string(artifact),
		ApprovalState: approvalDraft,
		ModelVersion:  r.Run.DomainModelVersion,
	}
	if err := audit.RecordProposal(ctx, r.DB, p); err != nil {
		return "", err
	}
	return id, nil
}

// approvalDraft mirrors the model's ApprovalState.draft. It is spelled out here
// rather than imported because the vocabulary lives in Python; the DDL's CHECK
// constraint is what actually enforces it, so a typo fails the insert rather
// than passing silently.
const approvalDraft = "draft"

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
