package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The run record and the write-before-act contract over it (plan §7.2, task
// D.4).
//
// jetsapi.agent_run is the mutable summary of a run; jetsapi.agent_audit is the
// immutable record of how it got there. That division is what makes the
// contract worth having: a lost update to the run row costs a summary, while
// the trail behind it stays intact and append-only.
//
// **Write-before-act, concretely.** A run is made durable *before* the first
// model call, in one transaction that carries both the run row and its intent
// event. The commit is the acknowledgement: a process that dies immediately
// afterwards leaves both rows behind, which is the property the apiserver's zap
// audit logger cannot give, since its Sync is deferred to process exit.
//
// The intent event must ride the same transaction as the run row it records.
// Writing them separately would leave two ways to be half-done — an intent with
// no run, or a run nobody intended — and the audit store's whole claim is that
// its record and the thing it records cannot disagree.

// Run is one row of jetsapi.agent_run, mirroring the AgentRun entity. The
// column list is generated from the model, so a field added there is a compile
// error here rather than a silent omission.
type Run struct {
	RunId               string
	AgentId             string
	AgentVersion        string
	ModelId             string
	PromptVersion       string
	Tier                string
	StartedAt           time.Time
	DomainModelVersion  string
	IterationCap        int
	WallClockCapSeconds int
	// TriggeredBy is what started this run. For a candidate of a
	// generate-and-filter batch it is the parent run's id, which is how the
	// batch is reconstructed: each candidate keeps its own run and its own
	// hash chain, and this is the only thing joining them. Empty for a run
	// nothing else started.
	TriggeredBy string
}

// Beginner is the slice of pgx that StartRun needs: something that can open a
// transaction. Satisfied by *pgxpool.Pool and *pgx.Conn.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Execer is what FinishRun needs; same implementors, plus pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// StartRun makes a run durable before anything acts.
//
// It opens a transaction, inserts the run, appends the intent event on that
// same transaction — the DB interface is satisfied by pgx.Tx precisely so this
// is possible — and commits. Only after it returns may a caller make its first
// model call. The intent payload is the caller's description of what it is
// about to do, and must be non-empty valid JSON.
func StartRun(ctx context.Context, db Beginner, run *Run, intent []byte) error {
	if run == nil || run.RunId == "" {
		return fmt.Errorf("a run must carry an id before it can be recorded")
	}
	if len(intent) == 0 {
		return fmt.Errorf("the intent payload is empty; write-before-act records what is about to happen, not that something is")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("while opening the transaction for run %s: %w", run.RunId, err)
	}
	// Rollback is a no-op after a successful commit, so this is the safe
	// default path out of every early return below.
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO jetsapi.agent_run
		   (run_id, agent_id, agent_version, model_id, prompt_version, tier,
		    started_at, domain_model_version, iteration_cap, wall_clock_cap_seconds,
		    triggered_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''))`,
		run.RunId, run.AgentId, run.AgentVersion, run.ModelId, run.PromptVersion,
		run.Tier, run.StartedAt, run.DomainModelVersion, run.IterationCap,
		run.WallClockCapSeconds, run.TriggeredBy); err != nil {
		return fmt.Errorf("while recording run %s: %w", run.RunId, err)
	}

	if _, err := Append(ctx, tx, &Event{
		RunId:     run.RunId,
		EventType: EventIntent,
		Actor:     run.AgentId,
		Tier:      run.Tier,
		Payload:   intent,
	}); err != nil {
		return fmt.Errorf("while recording the intent for run %s: %w", run.RunId, err)
	}

	// The commit is the acknowledgement. Everything before it can be lost
	// without consequence; nothing after it may act until it has returned.
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("while committing the intent for run %s: %w", run.RunId, err)
	}
	return nil
}

// FinishRun records how a run ended and what it spent. It is an UPDATE, which
// agent_run permits and agent_audit does not — the outcome *event* is appended
// separately and is the durable record; this row is the summary a query reads.
func FinishRun(ctx context.Context, db Execer, runId, status string, tokenSpend int) error {
	if runId == "" {
		return fmt.Errorf("cannot finish a run with no id")
	}
	tag, err := db.Exec(ctx,
		`UPDATE jetsapi.agent_run
		    SET ended_at = now(), run_status = $2, token_spend = $3
		  WHERE run_id = $1`,
		runId, status, tokenSpend)
	if err != nil {
		return fmt.Errorf("while finishing run %s: %w", runId, err)
	}
	if tag.RowsAffected() == 0 {
		// A run that was never started cannot be finished, and silently
		// updating nothing would hide a caller that skipped StartRun — which
		// is exactly the write-before-act violation this package exists to
		// prevent.
		return fmt.Errorf("no run %s to finish; StartRun must have run first", runId)
	}
	return nil
}
