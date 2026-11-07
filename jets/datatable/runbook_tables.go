package datatable

// The runbook executor's two tables, jetsapi.runbook_run and
// jetsapi.runbook_step_run (jets_schema.json), and how a row reads and how it is
// shown. Project runbooks, Phase 1, track AB.
//
// **The tables are the whole of a run's state.** The executor holds nothing in
// memory that it cannot rebuild from them: which step comes next is derived from
// the step rows, the outputs a later step binds are read back from them, and the
// files a run executes are pinned in the run row by revision. That is what lets
// an apiserver restart lose nothing but the step that was executing, which the
// start-up sweep marks interrupted (runbook_executor.go).
//
// Times are timestamptz and every one is written by the database's clock
// (now()), so a run's timings do not depend on two clocks agreeing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
	"github.com/jackc/pgx/v5"
)

// RunbookExecuteCapability is required by every action on /runbook. Each step
// also requires its own capability when it runs (runbookStepCapability).
const RunbookExecuteCapability = "runbook_execute"

// RunbookError is a refusal with the HTTP status it answers, and the findings
// of a runbook that does not validate when that is the reason.
type RunbookError struct {
	Status   int
	Message  string
	Findings []wsvalidate.Finding
}

func (e *RunbookError) Error() string { return e.Message }

func rbErr(status int, format string, args ...any) *RunbookError {
	return &RunbookError{Status: status, Message: fmt.Sprintf(format, args...)}
}

// RunbookErrorStatus is the HTTP status an error from this file's operations
// answers: a RunbookError's own, 500 otherwise.
func RunbookErrorStatus(err error) int {
	var re *RunbookError
	if errors.As(err, &re) {
		return re.Status
	}
	return http.StatusInternalServerError
}

// rbRun is one row of jetsapi.runbook_run.
type rbRun struct {
	Key           int64
	WorkspaceName string
	Runbook       string
	Status        string
	Parameters    map[string]json.RawMessage
	Revisions     []runbook.Revision
	// RevisionFiles maps a revision number ("1") to its files' content, by
	// path relative to runbooks/. It is what the executor runs, so an edit on
	// disk takes effect only when a person resumes the run (§4.4).
	RevisionFiles map[string]map[string]string
	Environment   runbook.Environment
	StartedBy     string
	ActingUser    string
	StartedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
	ReleasedBy    *string
	ReleasedAt    *time.Time
	LastError     *string
}

// currentRevision is the run's last revision number.
func (r *rbRun) currentRevision() int {
	if len(r.Revisions) == 0 {
		return 0
	}
	return r.Revisions[len(r.Revisions)-1].Revision
}

// rbStep is one row of jetsapi.runbook_step_run.
type rbStep struct {
	Key         int64
	RunKey      int64
	Sequence    int
	StepID      string
	Iteration   int
	Type        string
	Status      string
	Revision    int
	DefHash     string
	SQLHash     *string
	SQL         *string
	Statements  []byte
	Bound       []byte
	Outputs     []byte
	OutputTypes map[string]string
	Columns     []string
	Rows        []byte
	RowCount    *int64
	Truncated   bool
	Preview     []byte
	Actor       *string
	Decision    *runbook.Decision
	History     []byte
	NextCheckAt *time.Time
	Attempts    int
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Error       *string
}

const rbRunColumns = `key, workspace_name, runbook, status, parameters, revisions, revision_files,
	environment, started_by, acting_user, started_at, updated_at, finished_at, released_by, released_at, last_error`

const rbStepColumns = `key, run_key, sequence, step_id, iteration, type, status, revision, def_hash,
	sql_hash, sql, statements, bound, outputs, output_types, columns, rows, row_count, truncated, preview,
	actor, decision, history, next_check_at, attempts, started_at, finished_at, error`

type rbQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanRun(row pgx.Row) (*rbRun, error) {
	var r rbRun
	var params, revisions, files, env []byte
	err := row.Scan(&r.Key, &r.WorkspaceName, &r.Runbook, &r.Status, &params, &revisions, &files,
		&env, &r.StartedBy, &r.ActingUser, &r.StartedAt, &r.UpdatedAt, &r.FinishedAt, &r.ReleasedBy,
		&r.ReleasedAt, &r.LastError)
	if err != nil {
		return nil, err
	}
	if err = unmarshalIf(params, &r.Parameters); err != nil {
		return nil, fmt.Errorf("run %d parameters: %v", r.Key, err)
	}
	if err = unmarshalIf(revisions, &r.Revisions); err != nil {
		return nil, fmt.Errorf("run %d revisions: %v", r.Key, err)
	}
	if err = unmarshalIf(files, &r.RevisionFiles); err != nil {
		return nil, fmt.Errorf("run %d revision files: %v", r.Key, err)
	}
	if err = unmarshalIf(env, &r.Environment); err != nil {
		return nil, fmt.Errorf("run %d environment: %v", r.Key, err)
	}
	return &r, nil
}

func unmarshalIf(b []byte, dst any) error {
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, dst)
}

// loadRun reads one run; forUpdate locks its row for the transaction, which is
// how every state change on a run is serialised.
func loadRun(ctx context.Context, q rbQuerier, key int64, forUpdate bool) (*rbRun, error) {
	stmt := `SELECT ` + rbRunColumns + ` FROM jetsapi.runbook_run WHERE key = $1`
	if forUpdate {
		stmt += ` FOR UPDATE`
	}
	r, err := scanRun(q.QueryRow(ctx, stmt, key))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, rbErr(http.StatusNotFound, "runbook run %d does not exist", key)
	}
	return r, err
}

func scanStep(rows pgx.Rows) (*rbStep, error) {
	var s rbStep
	var outTypes, columns, decision []byte
	err := rows.Scan(&s.Key, &s.RunKey, &s.Sequence, &s.StepID, &s.Iteration, &s.Type, &s.Status, &s.Revision,
		&s.DefHash, &s.SQLHash, &s.SQL, &s.Statements, &s.Bound, &s.Outputs, &outTypes, &columns, &s.Rows,
		&s.RowCount, &s.Truncated, &s.Preview, &s.Actor, &decision, &s.History, &s.NextCheckAt, &s.Attempts,
		&s.StartedAt, &s.FinishedAt, &s.Error)
	if err != nil {
		return nil, err
	}
	if err = unmarshalIf(outTypes, &s.OutputTypes); err != nil {
		return nil, err
	}
	if err = unmarshalIf(columns, &s.Columns); err != nil {
		return nil, err
	}
	if len(decision) > 0 && string(decision) != "null" {
		s.Decision = &runbook.Decision{}
		if err = json.Unmarshal(decision, s.Decision); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

// loadSteps reads a run's step rows in sequence.
func loadSteps(ctx context.Context, q rbQuerier, runKey int64) ([]*rbStep, error) {
	rows, err := q.Query(ctx, `SELECT `+rbStepColumns+` FROM jetsapi.runbook_step_run
		WHERE run_key = $1 ORDER BY sequence`, runKey)
	if err != nil {
		return nil, fmt.Errorf("while reading the steps of run %d: %v", runKey, err)
	}
	defer rows.Close()
	var out []*rbStep
	for rows.Next() {
		s, err := scanStep(rows)
		if err != nil {
			return nil, fmt.Errorf("while reading a step of run %d: %v", runKey, err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Step row statuses that are not yet final. A repeat's own row is `running`
// for as long as its iterations are, without executing anything itself.
func rbStepActive(s *rbStep) bool {
	switch s.Status {
	case runbook.StepPending, runbook.StepWaiting, runbook.StepAwaitingConfirmation, runbook.StepAwaitingInput:
		return true
	case runbook.StepRunning:
		return s.Type != string(runbook.StepRepeat)
	}
	return false
}

// rbStepExecuted reports whether a step row may have taken effect: completed,
// or interrupted while it ran. Its definition may not change in a later
// revision (§4.4: history cannot be rewritten).
func rbStepExecuted(s *rbStep) bool {
	return s.Status == runbook.StepCompleted || s.Status == runbook.StepInterrupted
}

// activeStep is the step row the run is at, when one is not final: what the
// executor does next, or what a person must answer.
func activeStep(steps []*rbStep) *rbStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if rbStepActive(steps[i]) {
			return steps[i]
		}
	}
	return nil
}

// frontierStep is the step a person's control acts on: the active one, else
// the last one recorded. It is the screen's "current step" (the last step that
// is not pending).
func frontierStep(steps []*rbStep) *rbStep {
	if a := activeStep(steps); a != nil {
		return a
	}
	if len(steps) == 0 {
		return nil
	}
	return steps[len(steps)-1]
}

// rawOrNull is b as JSON, or null when empty.
func rawOrNull(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("null")
	}
	return json.RawMessage(b)
}

func timeOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// runView is a run as /runbook answers it, field names as the columns name
// them. revision_files is not shown: the files are the workspace's, and the
// hashes in revisions identify them.
func runView(r *rbRun, steps []*rbStep) map[string]any {
	revisions := r.Revisions
	if revisions == nil {
		revisions = []runbook.Revision{}
	}
	params := r.Parameters
	if params == nil {
		params = map[string]json.RawMessage{}
	}
	v := map[string]any{
		"run_key":        r.Key,
		"workspace_name": r.WorkspaceName,
		"runbook":        r.Runbook,
		"status":         r.Status,
		"parameters":     params,
		"revisions":      revisions,
		"environment":    r.Environment,
		"started_by":     r.StartedBy,
		"acting_user":    r.ActingUser,
		"started_at":     r.StartedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":     r.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"finished_at":    timeOrNil(r.FinishedAt),
		"released_by":    r.ReleasedBy,
		"released_at":    timeOrNil(r.ReleasedAt),
		"last_error":     r.LastError,
	}
	if steps != nil {
		out := make([]map[string]any, 0, len(steps))
		for _, s := range steps {
			out = append(out, stepView(s))
		}
		v["steps"] = out
	}
	return v
}

// stepView is a step row as /runbook answers it: every column of §3.3, plus
// `statements` (each statement as sent, with the parameter name each $n
// stands for), `columns` (the result's column order) and `history`.
func stepView(s *rbStep) map[string]any {
	var decision any
	if s.Decision != nil {
		decision = s.Decision
	}
	return map[string]any{
		"step_key":      s.Key,
		"run_key":       s.RunKey,
		"sequence":      s.Sequence,
		"step_id":       s.StepID,
		"iteration":     s.Iteration,
		"type":          s.Type,
		"status":        s.Status,
		"revision":      s.Revision,
		"sql_hash":      s.SQLHash,
		"sql":           s.SQL,
		"statements":    rawOrNull(s.Statements),
		"bound":         rawOrNull(s.Bound),
		"outputs":       rawOrNull(s.Outputs),
		"columns":       s.Columns,
		"rows":          rawOrNull(s.Rows),
		"row_count":     s.RowCount,
		"truncated":     s.Truncated,
		"preview":       rawOrNull(s.Preview),
		"actor":         s.Actor,
		"decision":      decision,
		"history":       rawOrNull(s.History),
		"next_check_at": timeOrNil(s.NextCheckAt),
		"attempts":      s.Attempts,
		"started_at":    timeOrNil(s.StartedAt),
		"finished_at":   timeOrNil(s.FinishedAt),
		"error":         s.Error,
	}
}

// resultOf is the `.rbr.json` document of a run (§3.5).
func resultOf(r *rbRun, steps []*rbStep) (*runbook.Result, error) {
	res := &runbook.Result{
		Format:      runbook.ResultFormatVersion,
		Runbook:     r.Runbook,
		Workspace:   r.WorkspaceName,
		RunKey:      r.Key,
		Status:      r.Status,
		Parameters:  map[string]any{},
		Environment: r.Environment,
		Revisions:   r.Revisions,
		StartedBy:   r.StartedBy,
		StartedAt:   r.StartedAt.UTC(),
		FinishedAt:  r.FinishedAt,
		ReleasedAt:  r.ReleasedAt,
		Steps:       []runbook.StepRecord{},
	}
	if r.ReleasedBy != nil {
		res.ReleasedBy = *r.ReleasedBy
	}
	for k, v := range r.Parameters {
		res.Parameters[k] = v
	}
	if res.Revisions == nil {
		res.Revisions = []runbook.Revision{}
	}
	for _, s := range steps {
		rec := runbook.StepRecord{
			ID:         s.StepID,
			Type:       s.Type,
			Iteration:  s.Iteration,
			Revision:   s.Revision,
			Status:     s.Status,
			Decision:   s.Decision,
			Truncated:  s.Truncated,
			StartedAt:  s.StartedAt,
			FinishedAt: s.FinishedAt,
			Error:      s.Error,
		}
		if s.SQLHash != nil {
			rec.SQLHash = *s.SQLHash
		}
		if s.SQL != nil {
			rec.SQL = *s.SQL
		}
		if s.Actor != nil {
			rec.Actor = *s.Actor
		}
		if s.RowCount != nil {
			rec.RowCount = *s.RowCount
		}
		if err := unmarshalIf(s.Bound, &rec.Bound); err != nil {
			return nil, err
		}
		if err := unmarshalIf(s.Outputs, &rec.Outputs); err != nil {
			return nil, err
		}
		if err := unmarshalIf(s.Rows, &rec.Rows); err != nil {
			return nil, err
		}
		if err := unmarshalIf(s.Preview, &rec.Preview); err != nil {
			return nil, err
		}
		res.Steps = append(res.Steps, rec)
	}
	return res, nil
}
