package datatable

// The runbook executor: a small in-process service in the apiserver that drives
// workspace runbook runs from the state in jetsapi.runbook_run and
// jetsapi.runbook_step_run. Project runbooks, Phase 1, track AB (assessment
// §4.4, plan §3.1 and §3.3).
//
// # One step at a time, from the tables
//
// A goroutine wakes every few seconds, or when a person acts on a run, and
// drives every run whose status is `running`. Driving a run is a loop of one
// step each: read the run and its step rows, decide what the next thing is,
// do it, record it. Nothing is carried from one step to the next in memory, so
// a step's outputs reach a later step only through the row that recorded them.
//
// **Two apiserver tasks never drive one run.** Each step is driven on a
// connection of its own (taken out of the pool and closed afterwards) that
// first takes the session-level advisory lock
// pg_try_advisory_lock(runbookLockClass, run_key); a task that cannot take it
// leaves the run alone. The lock goes when the connection closes, which is also
// what happens when the process dies. Every change of state, the executor's and
// a person's, is made in a transaction that locks the run's row first, so a
// person's Abort and the executor's next step are ordered.
//
// The connection is closed after each step so that nothing a ddl does to its
// session (a SET without LOCAL) outlives the step, and it carries the
// application name `jetstore_runbook_<run_key>`, which is how Abort finds the
// statement to cancel (pg_cancel_backend) when the step runs in another task.
//
// # Restart
//
// A step that was `running` when the apiserver stopped may or may not have
// taken effect. The start-up sweep (SweepInterrupted) marks it `interrupted`
// and pauses its run; nothing reruns on its own. A person retries it or skips
// it. A `wait` between two checks is `waiting`, not running, with its next check
// time in the row, so a restart loses nothing there: the next start's driver
// makes the check when it is due. A ddl awaiting confirmation is not running
// either, and an accepted step that had not started is `pending` and runs.
//
// # Who a step runs as
//
// Each step runs as a person: the one who accepted it, for a step that waited
// for a confirmation; otherwise the run's acting user, who is the person who
// started the run or last resumed it (Continue, Skip, Retry, Accept, Record).
// The capability a step needs is read from the database when the step runs, so
// a person who lost a role since the run started is refused at the next step
// (criterion 7). A refusal fails the step and pauses the run, whatever its
// on_fail, so someone who holds the capability can retry it. There is no
// service identity, and the admin is not used unless the admin is the person.
//
// # What no step does
//
// No step calls a model, and none opens a connection other than to the
// database, except through the existing JetStore functions a person's button
// calls (StartPendingTasks, InsertRows, PutSchemaEventToS3, the workspace
// compile), behind RunbookJetStore (criterion 8).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/runbook"
	"github.com/artisoft-io/jetstore/jets/user"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// runbookLockClass is the first key of the per-run advisory lock ("RBK").
const runbookLockClass int32 = 0x52424b00

// runbookWorkspaceIDECapability is what a ddl and a compile need.
const runbookWorkspaceIDECapability = "workspace_ide"

// runbookStepCapability is the capability a step type needs when it runs,
// beside runbook_execute (assessment §4.2).
func runbookStepCapability(t runbook.StepType) string {
	switch t {
	case runbook.StepQuery, runbook.StepWait:
		return CapabilityReadData
	case runbook.StepDDL, runbook.StepCompile:
		return runbookWorkspaceIDECapability
	case runbook.StepStartPending, runbook.StepStartPipeline, runbook.StepPutSchemaEvent:
		return CapabilityRunPipelines
	}
	return ""
}

// RunbookJetStore is what the JetStore step types call: the functions the UI's
// buttons call. Behind an interface so the executor is testable without AWS.
type RunbookJetStore interface {
	// StartPending is Check Pending.
	StartPending(ctx context.Context, actor string) error
	// StartPipeline inserts a submitted pipeline_execution_status row and
	// returns its session id.
	StartPipeline(ctx context.Context, actor string, row map[string]any) (string, error)
	// PutSchemaEvent writes a schema event and returns the keys written.
	PutSchemaEvent(ctx context.Context, actor, event, key string) ([]string, error)
	// Compile compiles the workspace and returns when it has finished, or
	// when the deadline has passed.
	Compile(ctx context.Context, actor, workspaceName string, deadline time.Duration) error
}

// RunbookCapabilityChecker answers nil when the person holds the capability
// now, reading it from the database.
type RunbookCapabilityChecker func(ctx context.Context, email, capability string) error

// DatabaseCapabilityChecker reads the person's roles and capabilities from
// jetsapi.users and jetsapi.role_capability on every call.
func DatabaseCapabilityChecker(dbpool *pgxpool.Pool) RunbookCapabilityChecker {
	return func(ctx context.Context, email, capability string) error {
		u, err := user.GetUserByEmail(dbpool, email)
		if err != nil {
			return fmt.Errorf("user %s cannot be read: %v", email, err)
		}
		if !u.IsAdmin() && u.IsActive != 1 {
			return fmt.Errorf("user %s is not active", email)
		}
		if !u.HasCapability(capability) {
			return fmt.Errorf("user %s does not hold the %s capability", email, capability)
		}
		return nil
	}
}

// RunbookExecutor drives runbook runs. One per apiserver.
type RunbookExecutor struct {
	pool     *pgxpool.Pool
	jetstore RunbookJetStore
	caps     RunbookCapabilityChecker
	tick     time.Duration

	mu      sync.Mutex
	driving map[int64]bool
	cancels map[int64]context.CancelFunc
	wake    chan struct{}
}

// NewRunbookExecutor makes an executor; Start starts its driver.
func NewRunbookExecutor(dbpool *pgxpool.Pool, jetstore RunbookJetStore, caps RunbookCapabilityChecker) *RunbookExecutor {
	if caps == nil {
		caps = DatabaseCapabilityChecker(dbpool)
	}
	return &RunbookExecutor{
		pool:     dbpool,
		jetstore: jetstore,
		caps:     caps,
		tick:     2 * time.Second,
		driving:  map[int64]bool{},
		cancels:  map[int64]context.CancelFunc{},
		wake:     make(chan struct{}, 1),
	}
}

var (
	runbookExecutorMu sync.Mutex
	runbookExecutor   *RunbookExecutor
)

// SetRunbookExecutor installs the process's executor and returns the previous.
func SetRunbookExecutor(e *RunbookExecutor) *RunbookExecutor {
	runbookExecutorMu.Lock()
	defer runbookExecutorMu.Unlock()
	prev := runbookExecutor
	runbookExecutor = e
	return prev
}

// CurrentRunbookExecutor is the installed executor, nil before one is.
func CurrentRunbookExecutor() *RunbookExecutor {
	runbookExecutorMu.Lock()
	defer runbookExecutorMu.Unlock()
	return runbookExecutor
}

// Start runs the driver until ctx ends.
func (e *RunbookExecutor) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(e.tick)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-e.wake:
			}
			e.driveAll(ctx)
		}
	}()
}

// Wake makes the driver look at the runs now.
func (e *RunbookExecutor) Wake() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *RunbookExecutor) driveAll(ctx context.Context) {
	rows, err := e.pool.Query(ctx, `SELECT key FROM jetsapi.runbook_run
		WHERE status = $1 AND workspace_name = $2 ORDER BY key`, runbook.RunRunning, activeWorkspace())
	if err != nil {
		log.Printf("runbook executor: while listing running runs: %v", err)
		return
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		log.Printf("runbook executor: while listing running runs: %v", err)
		return
	}
	for _, k := range keys {
		e.mu.Lock()
		if e.driving[k] {
			e.mu.Unlock()
			continue
		}
		e.driving[k] = true
		e.mu.Unlock()
		go func(k int64) {
			defer func() {
				e.mu.Lock()
				delete(e.driving, k)
				e.mu.Unlock()
			}()
			e.DriveRun(ctx, k)
		}(k)
	}
}

// DriveRun drives one run until it has nothing to do now: it waits on a person,
// on a wait's next check, or has ended. A panic is recovered into a log line;
// the step stays running and the next start's sweep interrupts it.
func (e *RunbookExecutor) DriveRun(ctx context.Context, key int64) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("runbook run %d: panic in the executor: %v", key, r)
		}
	}()
	for range 10000 {
		progress, err := e.stepOnce(ctx, key)
		if err != nil {
			log.Printf("runbook run %d: %v", key, err)
			return
		}
		if !progress {
			return
		}
	}
}

// execConn takes a connection out of the pool for one step and the run's
// advisory lock on it. ok is false when another task holds the lock.
func (e *RunbookExecutor) execConn(ctx context.Context, key int64) (conn *pgx.Conn, ok bool, err error) {
	c, err := e.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("while acquiring a connection: %v", err)
	}
	conn = c.Hijack()
	var locked bool
	err = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::int, $2::int),
		set_config('application_name', $3, false)`, runbookLockClass, int32(key), runbookAppName(key)).Scan(&locked, new(string))
	if err != nil || !locked {
		conn.Close(context.Background())
		return nil, false, err
	}
	return conn, true, nil
}

func runbookAppName(key int64) string { return "jetstore_runbook_" + strconv.FormatInt(key, 10) }

// --- audit ---------------------------------------------------------------

// runbookAuditRunID is the agent_audit run_id a runbook run's events chain
// under. agent_audit's run_id is not a foreign key and its actor is an agent
// identity or a user email, so a person's runbook run is a run of that chain
// in its own right (Q-6).
func runbookAuditRunID(key int64) string { return "runbook_run:" + strconv.FormatInt(key, 10) }

// auditStep appends one event to the run's chain in the caller's transaction.
// The payload names the step and the transition and carries no value, row or
// parameter: the chain is not a copy of the record.
func auditStep(ctx context.Context, tx pgx.Tx, run *rbRun, s *rbStep, eventType, transition, actor, detail string) error {
	p := map[string]any{
		"subject":    "runbook",
		"workspace":  run.WorkspaceName,
		"runbook":    run.Runbook,
		"run_key":    run.Key,
		"transition": transition,
	}
	if s != nil {
		p["step_key"] = s.Key
		p["step_id"] = s.StepID
		p["iteration"] = s.Iteration
		p["type"] = s.Type
		p["revision"] = s.Revision
		if s.SQLHash != nil {
			p["sql_hash"] = *s.SQLHash
		}
	}
	if detail != "" {
		p["detail"] = detail
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = audit.Append(ctx, tx, &audit.Event{RunId: runbookAuditRunID(run.Key), EventType: eventType, Actor: actor, Payload: b})
	return err
}

// --- writing rows ------------------------------------------------------------

// saveRun writes the run's mutable columns; finished_at is set when the status
// becomes terminal.
func saveRun(ctx context.Context, tx pgx.Tx, r *rbRun) error {
	params, _ := json.Marshal(r.Parameters)
	revs, _ := json.Marshal(r.Revisions)
	files, _ := json.Marshal(r.RevisionFiles)
	_, err := tx.Exec(ctx, `UPDATE jetsapi.runbook_run SET status = $2, acting_user = $3, parameters = $4::jsonb,
		revisions = $5::jsonb, revision_files = $6::jsonb, last_error = $7, updated_at = now(),
		finished_at = CASE WHEN $2 IN ('completed', 'failed', 'aborted') THEN coalesce(finished_at, now()) ELSE NULL END
		WHERE key = $1`, r.Key, r.Status, r.ActingUser, string(params), string(revs), string(files), r.LastError)
	if err != nil {
		return fmt.Errorf("while saving run %d: %v", r.Key, err)
	}
	return nil
}

// historyEvent is the SQL that appends one event to a step row's history.
const historyEvent = `coalesce(history, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
	'at', now(), 'by', $HBY::text, 'event', $HEV::text, 'detail', $HDT::text))`

// setStep updates a step row: sets is a list of "column = $n" with args from
// $1, and the history event's three parameters follow.
func setStep(ctx context.Context, tx pgx.Tx, stepKey int64, sets string, args []any, by, event, detail string) error {
	n := len(args)
	h := strings.NewReplacer("$HBY", "$"+strconv.Itoa(n+2), "$HEV", "$"+strconv.Itoa(n+3),
		"$HDT", "$"+strconv.Itoa(n+4)).Replace(historyEvent)
	stmt := `UPDATE jetsapi.runbook_step_run SET ` + sets + `, history = ` + h + ` WHERE key = $` + strconv.Itoa(n+1)
	all := append(append([]any{}, args...), stepKey, by, event, detail)
	if _, err := tx.Exec(ctx, stmt, all...); err != nil {
		return fmt.Errorf("while updating step row %d: %v", stepKey, err)
	}
	return nil
}

// insertStep appends a step row to the run.
func insertStep(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook, step *runbook.Step,
	iteration int, status, by, event string) (*rbStep, error) {
	var sqlHash *string
	if step.Type.HasSQL() {
		if text, ok := lr.Files[step.SQLPath(lr.Runbook.Runbook)]; ok {
			h := sha256Of(text)
			sqlHash = &h
		}
	}
	var key int64
	var seq int
	err := tx.QueryRow(ctx, `INSERT INTO jetsapi.runbook_step_run
		(run_key, sequence, step_id, iteration, type, status, revision, def_hash, sql_hash, attempts, history,
		 finished_at)
		SELECT $1, coalesce(max(sequence), 0) + 1, $2, $3, $4, $5, $6, $7, $8, 0,
		       jsonb_build_array(jsonb_build_object('at', now(), 'by', $9::text, 'event', $10::text)),
		       CASE WHEN $5 = 'skipped' THEN now() END
		  FROM jetsapi.runbook_step_run WHERE run_key = $1
		RETURNING key, sequence`,
		run.Key, step.ID, iteration, string(step.Type), status, run.currentRevision(), stepDefHash(step), sqlHash,
		by, event).Scan(&key, &seq)
	if err != nil {
		return nil, fmt.Errorf("while recording step %s of run %d: %v", step.ID, run.Key, err)
	}
	return &rbStep{Key: key, RunKey: run.Key, Sequence: seq, StepID: step.ID, Iteration: iteration,
		Type: string(step.Type), Status: status, Revision: run.currentRevision(), DefHash: stepDefHash(step),
		SQLHash: sqlHash}, nil
}

// --- reading the run's files and values ----------------------------------------

// pinned is the runbook the run executes now: its current revision's files.
func (r *rbRun) pinned() (*loadedRunbook, error) {
	files := r.RevisionFiles[strconv.Itoa(r.currentRevision())]
	if files == nil {
		return nil, fmt.Errorf("run %d has no files for revision %d", r.Key, r.currentRevision())
	}
	lr := loadRunbookFiles(r.Runbook, files)
	if !lr.Valid() {
		return nil, fmt.Errorf("run %d: revision %d of runbook %s does not validate: %s",
			r.Key, r.currentRevision(), r.Runbook, lr.errorSummary())
	}
	return lr, nil
}

// buildEnv is what a step's conditions and bindings read: the run's parameters
// and the outputs of every completed step, the later of two records of one
// step (a retry, a repeat's iterations) winning.
func buildEnv(run *rbRun, rb *runbook.Runbook, steps []*rbStep) (runbook.MapEnv, error) {
	env := runbook.MapEnv{}
	for name, raw := range run.Parameters {
		par, ok := rb.Parameters[name]
		if !ok {
			continue
		}
		t, err := runbook.ParseType(par.Type)
		if err != nil {
			return nil, err
		}
		v, err := runbook.ParseValue(t, raw)
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %v", name, err)
		}
		env[runbook.ParamRef(name)] = v
	}
	for _, s := range steps {
		if s.Status != runbook.StepCompleted || len(s.Outputs) == 0 {
			continue
		}
		if err := addOutputs(env, s.StepID, s.Outputs, s.OutputTypes); err != nil {
			return nil, fmt.Errorf("outputs of step %s: %v", s.StepID, err)
		}
	}
	return env, nil
}

// addOutputs puts a step's recorded outputs in env, typed as recorded.
func addOutputs(env runbook.MapEnv, stepID string, outputs []byte, types map[string]string) error {
	vals := map[string]json.RawMessage{}
	if err := json.Unmarshal(outputs, &vals); err != nil {
		return err
	}
	for col, raw := range vals {
		t, err := runbook.ParseType(types[col])
		if err != nil {
			return fmt.Errorf("%s: %v", col, err)
		}
		v, err := runbook.ParseValue(t, raw)
		if err != nil {
			return fmt.Errorf("%s: %v", col, err)
		}
		env[runbook.OutputRef(stepID, col)] = v
	}
	return nil
}

// actorOf is who a step runs as: the person who accepted it, else the run's
// acting user.
func actorOf(run *rbRun, s *rbStep) string {
	if s.Decision != nil && s.Decision.Accepted && s.Decision.By != "" {
		return s.Decision.By
	}
	return run.ActingUser
}

// checkStepCapability answers nil when the person may run the step now.
func (e *RunbookExecutor) checkStepCapability(ctx context.Context, actor string, t runbook.StepType) error {
	if err := e.caps(ctx, actor, RunbookExecuteCapability); err != nil {
		return err
	}
	if c := runbookStepCapability(t); c != "" {
		return e.caps(ctx, actor, c)
	}
	return nil
}

// --- the position in the runbook ------------------------------------------------

// posOf finds a step: its top-level index, and its index in its repeat or -1.
func posOf(rb *runbook.Runbook, id string) (int, int) {
	for i, s := range rb.Steps {
		if s.ID == id {
			return i, -1
		}
		for j, n := range s.Steps {
			if n.ID == id {
				return i, j
			}
		}
	}
	return -1, -1
}

func latestRowOf(steps []*rbStep, id string) *rbStep {
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].StepID == id {
			return steps[i]
		}
	}
	return nil
}

// advance records what follows the last step recorded: the next step,
// pending; the next iteration of a repeat; a repeat's interval; or the end of
// the run. It changes run.Status when the run ends or a repeat fails; the
// caller saves the run. It returns the step row it inserted, if any.
func (e *RunbookExecutor) advance(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook,
	steps []*rbStep, by string) (*rbStep, error) {
	rb := lr.Runbook
	var last *rbStep
	top, nested := -1, -1
	for i := len(steps) - 1; i >= 0; i-- {
		if t, n := posOf(rb, steps[i].StepID); t >= 0 {
			last, top, nested = steps[i], t, n
			break
		}
	}
	insertTop := func(i int) (*rbStep, error) {
		if i >= len(rb.Steps) {
			run.Status = runbook.RunCompleted
			if err := auditStep(ctx, tx, run, nil, audit.EventOutcome, "run_completed", by, ""); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return insertStep(ctx, tx, run, lr, rb.Steps[i], 0, runbook.StepPending, by, "pending")
	}
	if last == nil {
		return insertTop(0)
	}
	if nested < 0 {
		return insertTop(top + 1)
	}
	rep := rb.Steps[top]
	if nested+1 < len(rep.Steps) {
		return insertStep(ctx, tx, run, lr, rep.Steps[nested+1], last.Iteration, runbook.StepPending, by, "pending")
	}
	// The end of an iteration.
	rrow := latestRowOf(steps, rep.ID)
	if rrow == nil || rrow.Status != runbook.StepRunning {
		return insertTop(top + 1)
	}
	env, err := buildEnv(run, rb, steps)
	if err != nil {
		return nil, err
	}
	failRepeat := func(msg string) (*rbStep, error) {
		if err := setStep(ctx, tx, rrow.Key, `status = $1, finished_at = now(), error = $2`,
			[]any{runbook.StepFailed, msg}, by, "failed", msg); err != nil {
			return nil, err
		}
		rrow.Status = runbook.StepFailed
		if err := auditStep(ctx, tx, run, rrow, audit.EventOutcome, "failed", by, msg); err != nil {
			return nil, err
		}
		return nil, e.failRun(ctx, tx, run, rep, rep.ID+": "+msg, by, false)
	}
	done := false
	if rep.Until != nil {
		ok, err := rep.Until.Eval(env)
		if err != nil {
			return failRepeat("until cannot be evaluated: " + err.Error())
		}
		done = ok
	}
	if done {
		if err := setStep(ctx, tx, rrow.Key, `status = $1, finished_at = now()`,
			[]any{runbook.StepCompleted}, by, "completed", fmt.Sprintf("until held after %d iteration(s)", rrow.Attempts)); err != nil {
			return nil, err
		}
		if err := auditStep(ctx, tx, run, rrow, audit.EventOutcome, "executed", by, ""); err != nil {
			return nil, err
		}
		return insertTop(top + 1)
	}
	maxCount := runbook.MaxRepeatCount
	if rep.MaxCount != nil {
		maxCount = *rep.MaxCount
	}
	if rrow.Attempts >= maxCount {
		return failRepeat(fmt.Sprintf("until did not hold after %d iteration(s)", rrow.Attempts))
	}
	if iv := rep.IntervalDuration(); iv > 0 {
		return nil, setStep(ctx, tx, rrow.Key, `status = $1, next_check_at = now() + make_interval(secs => $2)`,
			[]any{runbook.StepWaiting, iv.Seconds()}, by, "waiting", "")
	}
	return e.nextIteration(ctx, tx, run, lr, rep, rrow, by)
}

// nextIteration starts a repeat's next iteration.
func (e *RunbookExecutor) nextIteration(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook,
	rep *runbook.Step, rrow *rbStep, by string) (*rbStep, error) {
	iteration := rrow.Attempts
	if err := setStep(ctx, tx, rrow.Key, `status = $1, attempts = attempts + 1, next_check_at = NULL`,
		[]any{runbook.StepRunning}, by, "iteration", strconv.Itoa(iteration)); err != nil {
		return nil, err
	}
	if len(rep.Steps) == 0 {
		return nil, nil
	}
	return insertStep(ctx, tx, run, lr, rep.Steps[0], iteration, runbook.StepPending, by, "pending")
}

// failRun applies a failed step's on_fail to its run; refused is a capability
// refusal, which pauses the run whatever on_fail says. A run that ends failed
// records run_failed in its audit chain, as a run that ends completed records
// run_completed and an aborted one aborted, so the chain says how every run
// ended (the step's own failed event carries the error).
func (e *RunbookExecutor) failRun(ctx context.Context, tx pgx.Tx, run *rbRun, step *runbook.Step,
	msg, actor string, refused bool) error {
	run.LastError = &msg
	if refused || (step != nil && step.OnFailOrDefault() == runbook.OnFailPause) {
		run.Status = runbook.RunPaused
		return nil
	}
	run.Status = runbook.RunFailed
	return auditStep(ctx, tx, run, nil, audit.EventOutcome, "run_failed", actor, "")
}

// --- one step -------------------------------------------------------------------

// stepWork is what a step does after the transaction that decided it has
// committed: the part that takes time or reaches outside the database.
type stepWork func(ctx context.Context, conn *pgx.Conn) error

// stepOnce drives one step of one run. progress is false when the run has
// nothing to do now, or another task holds it.
func (e *RunbookExecutor) stepOnce(ctx context.Context, key int64) (bool, error) {
	conn, ok, err := e.execConn(ctx, key)
	if err != nil || !ok {
		return false, err
	}
	defer conn.Close(context.Background())

	tx, err := conn.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	progress, work, err := e.decide(ctx, tx, key)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	if work == nil {
		return progress, nil
	}
	sctx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancels[key] = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.cancels, key)
		e.mu.Unlock()
		cancel()
	}()
	return true, work(sctx, conn)
}

// decide reads the run under its row lock and makes the next transition. A
// step that executes is marked running here, and its execution is returned as
// work for after the commit.
func (e *RunbookExecutor) decide(ctx context.Context, tx pgx.Tx, key int64) (bool, stepWork, error) {
	run, err := loadRun(ctx, tx, key, true)
	if err != nil {
		return false, nil, err
	}
	if run.Status != runbook.RunRunning {
		return false, nil, nil
	}
	if run.WorkspaceName != activeWorkspace() {
		msg := fmt.Sprintf("workspace %s is no longer the active workspace %s", run.WorkspaceName, activeWorkspace())
		run.Status, run.LastError = runbook.RunPaused, &msg
		return true, nil, saveRun(ctx, tx, run)
	}
	steps, err := loadSteps(ctx, tx, key)
	if err != nil {
		return false, nil, err
	}
	lr, err := run.pinned()
	if err != nil {
		if err = e.failRun(ctx, tx, run, nil, err.Error(), run.ActingUser, false); err != nil {
			return false, nil, err
		}
		return true, nil, saveRun(ctx, tx, run)
	}
	act := activeStep(steps)
	if act == nil {
		if _, err = e.advance(ctx, tx, run, lr, steps, run.ActingUser); err != nil {
			return false, nil, err
		}
		return true, nil, saveRun(ctx, tx, run)
	}
	step := lr.Runbook.Step(act.StepID)
	if step == nil {
		msg := fmt.Sprintf("step %s is not in revision %d of the runbook", act.StepID, run.currentRevision())
		if err = setStep(ctx, tx, act.Key, `status = $1, finished_at = now(), error = $2`,
			[]any{runbook.StepSkipped, msg}, "", "skipped", msg); err != nil {
			return false, nil, err
		}
		return true, nil, saveRun(ctx, tx, run)
	}
	actor := actorOf(run, act)

	switch act.Status {
	case runbook.StepRunning:
		// Left running by a driver that is gone: this task holds the lock.
		return true, nil, e.interrupt(ctx, tx, run, act)
	case runbook.StepAwaitingConfirmation, runbook.StepAwaitingInput:
		run.Status = runbook.RunPaused
		return true, nil, saveRun(ctx, tx, run)
	case runbook.StepWaiting:
		var due bool
		if err = tx.QueryRow(ctx, `SELECT next_check_at IS NULL OR next_check_at <= now()
			FROM jetsapi.runbook_step_run WHERE key = $1`, act.Key).Scan(&due); err != nil {
			return false, nil, err
		}
		if !due {
			return false, nil, nil
		}
		if step.Type == runbook.StepRepeat {
			if _, err = e.nextIteration(ctx, tx, run, lr, step, act, actor); err != nil {
				return false, nil, err
			}
			return true, nil, saveRun(ctx, tx, run)
		}
		return e.startExecution(ctx, tx, run, lr, act, step, actor)
	}

	// pending
	env, err := buildEnv(run, lr.Runbook, steps)
	if err != nil {
		return true, nil, e.failStep(ctx, tx, run, act, step, actor, err.Error(), false)
	}
	if step.When != nil && act.Decision == nil {
		ok, err := step.When.Eval(env)
		if err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, "when cannot be evaluated: "+err.Error(), false)
		}
		if !ok {
			if err = setStep(ctx, tx, act.Key, `status = $1, finished_at = now()`,
				[]any{runbook.StepSkipped}, actor, "skipped", "when is false"); err != nil {
				return false, nil, err
			}
			act.Status = runbook.StepSkipped
			if err = auditStep(ctx, tx, run, act, audit.EventDecision, "skipped", actor, "when is false"); err != nil {
				return false, nil, err
			}
			return true, nil, saveRun(ctx, tx, run)
		}
	}

	switch step.Type {
	case runbook.StepAssert:
		ok, err := step.Condition.Eval(env)
		if err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, "condition cannot be evaluated: "+err.Error(), false)
		}
		if !ok {
			msg := step.Message
			if msg == "" {
				msg = "the condition does not hold"
			}
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, msg, false)
		}
		if err = setStep(ctx, tx, act.Key, `status = $1, started_at = now(), finished_at = now(), actor = $2`,
			[]any{runbook.StepCompleted, actor}, actor, "completed", ""); err != nil {
			return false, nil, err
		}
		act.Status = runbook.StepCompleted
		if err = auditStep(ctx, tx, run, act, audit.EventOutcome, "executed", actor, ""); err != nil {
			return false, nil, err
		}
		return true, nil, saveRun(ctx, tx, run)

	case runbook.StepPause, runbook.StepManual:
		if err = setStep(ctx, tx, act.Key, `status = $1, started_at = now()`,
			[]any{runbook.StepAwaitingInput}, actor, "awaiting_input", ""); err != nil {
			return false, nil, err
		}
		run.Status = runbook.RunPaused
		return true, nil, saveRun(ctx, tx, run)

	case runbook.StepRepeat:
		if err = setStep(ctx, tx, act.Key, `status = $1, started_at = now(), attempts = 1, actor = $2`,
			[]any{runbook.StepRunning, actor}, actor, "iteration", "0"); err != nil {
			return false, nil, err
		}
		if len(step.Steps) > 0 {
			if _, err = insertStep(ctx, tx, run, lr, step.Steps[0], 0, runbook.StepPending, actor, "pending"); err != nil {
				return false, nil, err
			}
		}
		return true, nil, saveRun(ctx, tx, run)
	}

	if step.ConfirmRequired() && (act.Decision == nil || !act.Decision.Accepted) {
		return e.prepareConfirmation(ctx, tx, run, lr, act, step, actor, env)
	}
	return e.startExecution(ctx, tx, run, lr, act, step, actor)
}

// failStep records a step as failed and applies its on_fail; refused is a
// capability refusal, which pauses the run whatever on_fail says.
func (e *RunbookExecutor) failStep(ctx context.Context, tx pgx.Tx, run *rbRun, act *rbStep, step *runbook.Step,
	actor, msg string, refused bool) error {
	if err := setStep(ctx, tx, act.Key, `status = $1, finished_at = now(), error = $2,
		started_at = coalesce(started_at, now()), actor = coalesce(actor, $3)`,
		[]any{runbook.StepFailed, msg, actor}, actor, "failed", msg); err != nil {
		return err
	}
	act.Status = runbook.StepFailed
	if err := auditStep(ctx, tx, run, act, audit.EventOutcome, "failed", actor, msg); err != nil {
		return err
	}
	if err := e.failRun(ctx, tx, run, step, step.ID+": "+msg, actor, refused); err != nil {
		return err
	}
	return saveRun(ctx, tx, run)
}

// interrupt marks a step that was running when its executor went away.
func (e *RunbookExecutor) interrupt(ctx context.Context, tx pgx.Tx, run *rbRun, act *rbStep) error {
	msg := "interrupted: the apiserver stopped while this step was running; it may or may not have taken effect. Retry it or skip it"
	if err := setStep(ctx, tx, act.Key, `status = $1, finished_at = now(), error = $2`,
		[]any{runbook.StepInterrupted, msg}, "", "interrupted", ""); err != nil {
		return err
	}
	act.Status = runbook.StepInterrupted
	actor := actorOf(run, act)
	if err := auditStep(ctx, tx, run, act, audit.EventError, "interrupted", actor, ""); err != nil {
		return err
	}
	if run.Status == runbook.RunRunning {
		run.Status = runbook.RunPaused
		m := act.StepID + ": " + msg
		run.LastError = &m
	}
	return saveRun(ctx, tx, run)
}

// startExecution checks the person's capability, marks the step running and
// returns its execution.
func (e *RunbookExecutor) startExecution(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook,
	act *rbStep, step *runbook.Step, actor string) (bool, stepWork, error) {
	if err := e.checkStepCapability(ctx, actor, step.Type); err != nil {
		return true, nil, e.failStep(ctx, tx, run, act, step, actor, "refused: "+err.Error(), true)
	}
	if err := setStep(ctx, tx, act.Key, `status = $1, started_at = coalesce(started_at, now()), actor = $2,
		attempts = attempts + 1, next_check_at = NULL`,
		[]any{runbook.StepRunning, actor}, actor, "running", ""); err != nil {
		return false, nil, err
	}
	// Write before act: the intent commits before the step does anything.
	if err := auditStep(ctx, tx, run, act, audit.EventIntent, "executing", actor, ""); err != nil {
		return false, nil, err
	}
	if err := saveRun(ctx, tx, run); err != nil {
		return false, nil, err
	}
	return true, func(ctx context.Context, conn *pgx.Conn) error {
		return e.execute(ctx, conn, run.Key, lr, act, step, actor)
	}, nil
}

// --- confirmation -------------------------------------------------------------

// prepareConfirmation stops a step for a person's Accept or Deny: a ddl's
// statements as they will be sent, every bound value and its preview; a
// start_pipeline's resolved row; a put_schema_event's event and key; a
// compile's workspace. The step stays pending while a preview runs (a preview
// rolls back, so a restart in the middle of one costs nothing but the preview).
func (e *RunbookExecutor) prepareConfirmation(ctx context.Context, tx pgx.Tx, run *rbRun, lr *loadedRunbook,
	act *rbStep, step *runbook.Step, actor string, env runbook.MapEnv) (bool, stepWork, error) {
	switch step.Type {
	case runbook.StepDDL:
		text, ok := lr.Files[step.SQLPath(lr.Runbook.Runbook)]
		if !ok {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, "its SQL file is missing", false)
		}
		stmts, err := runbook.Bind(text, step, env)
		if err == nil {
			err = checkStatements(stmts)
		}
		if err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, err.Error(), false)
		}
		bound, statements, sentText, err := boundJSON(stmts)
		if err != nil {
			return false, nil, err
		}
		sqlHash := sha256Of(text)
		if !step.Preview {
			return true, nil, e.awaitConfirmation(ctx, tx, run, act, actor, sentText, sqlHash, statements, bound, nil)
		}
		if err = e.checkStepCapability(ctx, actor, step.Type); err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, "refused: preview: "+err.Error(), true)
		}
		return true, func(ctx context.Context, conn *pgx.Conn) error {
			entries, perr := runDDL(ctx, conn, stmts, step.TimeoutOrDefault(), true, nil)
			return e.withRun(ctx, conn, run.Key, func(tx pgx.Tx, run *rbRun) error {
				if !stillPending(ctx, tx, act.Key) {
					return nil
				}
				if perr != nil {
					return e.failStep(ctx, tx, run, act, step, actor, "preview: "+pgErrorText(perr), false)
				}
				preview, _ := json.Marshal(entries)
				return e.awaitConfirmation(ctx, tx, run, act, actor, sentText, sqlHash, statements, bound, preview)
			})
		}, nil

	case runbook.StepStartPipeline:
		row, err := resolveRow(step, env)
		if err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, err.Error(), false)
		}
		bound, _ := json.Marshal(row)
		return true, nil, e.awaitConfirmation(ctx, tx, run, act, actor, "", "", nil, bound, nil)

	case runbook.StepPutSchemaEvent:
		event, err := resolveEvent(step, env)
		if err != nil {
			return true, nil, e.failStep(ctx, tx, run, act, step, actor, err.Error(), false)
		}
		bound, _ := json.Marshal(map[string]any{"event": event, "key": step.Key})
		return true, nil, e.awaitConfirmation(ctx, tx, run, act, actor, "", "", nil, bound, nil)

	case runbook.StepCompile:
		bound, _ := json.Marshal(map[string]any{"workspace": run.WorkspaceName,
			"deadline": step.DeadlineOrDefault().String()})
		return true, nil, e.awaitConfirmation(ctx, tx, run, act, actor, "", "", nil, bound, nil)
	}
	return true, nil, e.failStep(ctx, tx, run, act, step, actor,
		fmt.Sprintf("step type %s does not wait for a confirmation", step.Type), false)
}

func stillPending(ctx context.Context, tx pgx.Tx, stepKey int64) bool {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM jetsapi.runbook_step_run WHERE key = $1`, stepKey).Scan(&status)
	return err == nil && status == runbook.StepPending
}

func nilIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

func (e *RunbookExecutor) awaitConfirmation(ctx context.Context, tx pgx.Tx, run *rbRun, act *rbStep, actor,
	sentText, sqlHash string, statements, bound, preview []byte) error {
	var sqlText, hash any
	if sentText != "" {
		sqlText = sentText
	}
	if sqlHash != "" {
		hash = sqlHash
	}
	if err := setStep(ctx, tx, act.Key, `status = $1, sql = $2, sql_hash = coalesce($3, sql_hash),
		statements = $4::jsonb, bound = $5::jsonb, preview = $6::jsonb, started_at = coalesce(started_at, now())`,
		[]any{runbook.StepAwaitingConfirmation, sqlText, hash, nilIfEmpty(statements), nilIfEmpty(bound), nilIfEmpty(preview)},
		actor, "awaiting_confirmation", ""); err != nil {
		return err
	}
	run.Status = runbook.RunPaused
	return saveRun(ctx, tx, run)
}

// resolveRow is a start_pipeline's row with every ref resolved.
func resolveRow(step *runbook.Step, env runbook.MapEnv) (map[string]any, error) {
	row := make(map[string]any, len(step.Row))
	for col, v := range step.Row {
		if v.From != "" {
			val, ok := env.Lookup(v.From)
			if !ok {
				return nil, fmt.Errorf("row column %s: %s has no value", col, v.From)
			}
			row[col] = jsonSafe(val.V)
			continue
		}
		var lit any
		if len(v.Literal) > 0 {
			if err := json.Unmarshal(v.Literal, &lit); err != nil {
				return nil, fmt.Errorf("row column %s: %v", col, err)
			}
		}
		row[col] = lit
	}
	return row, nil
}

func resolveEvent(step *runbook.Step, env runbook.MapEnv) (string, error) {
	if step.Event == nil {
		return "", errors.New("the step has no event")
	}
	val, ok := env.Lookup(step.Event.From)
	if !ok {
		return "", fmt.Errorf("event: %s has no value", step.Event.From)
	}
	s, ok := val.V.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("event: %s is not a text holding the event's JSON", step.Event.From)
	}
	if !json.Valid([]byte(s)) {
		return "", fmt.Errorf("event: %s does not hold JSON", step.Event.From)
	}
	return s, nil
}

// --- execution ------------------------------------------------------------------

// withRun runs fn in a transaction holding the run's row lock.
func (e *RunbookExecutor) withRun(ctx context.Context, conn *pgx.Conn, key int64, fn func(tx pgx.Tx, run *rbRun) error) error {
	// The step's context may have been cancelled by Abort; the record is
	// written regardless.
	bg := context.Background()
	var tx pgx.Tx
	var err error
	if conn.IsClosed() {
		// pgx closes a connection whose query a cancelled context interrupted
		// (Abort). The record goes through the pool: it is guarded by the run's
		// row lock and by the step row still reading running.
		tx, err = e.pool.Begin(bg)
	} else {
		tx, err = conn.Begin(bg)
	}
	if err != nil {
		return err
	}
	defer tx.Rollback(bg)
	run, err := loadRun(bg, tx, key, true)
	if err != nil {
		return err
	}
	if err = fn(tx, run); err != nil {
		return err
	}
	return tx.Commit(bg)
}

// outcome is what an execution recorded.
type outcome struct {
	outputs   map[string]runbook.Value
	result    *queryResult
	preview   []previewEntry
	statement []byte
	bound     []byte
	sentText  string
	sqlHash   string
	// detail is the history event's detail: an executed ddl's command tags,
	// kept there so the preview the person was shown stays as shown.
	detail string
}

// execute runs a step marked running and records its outcome.
func (e *RunbookExecutor) execute(ctx context.Context, conn *pgx.Conn, key int64, lr *loadedRunbook,
	act *rbStep, step *runbook.Step, actor string) error {
	bg := context.Background()
	var run *rbRun
	var steps []*rbStep
	err := e.withRun(bg, conn, key, func(tx pgx.Tx, r *rbRun) error {
		run = r
		s, err := loadSteps(bg, tx, key)
		steps = s
		return err
	})
	if err != nil {
		return err
	}
	if run.Status != runbook.RunRunning {
		return e.recordFailure(conn, key, act, step, actor, "the run was "+run.Status+" before the step started")
	}
	env, err := buildEnv(run, lr.Runbook, steps)
	if err != nil {
		return e.recordFailure(conn, key, act, step, actor, err.Error())
	}

	switch step.Type {
	case runbook.StepQuery, runbook.StepWait:
		text, ok := lr.Files[step.SQLPath(lr.Runbook.Runbook)]
		if !ok {
			return e.recordFailure(conn, key, act, step, actor, "its SQL file is missing")
		}
		stmts, err := runbook.Bind(text, step, env)
		if err != nil {
			return e.recordFailure(conn, key, act, step, actor, err.Error())
		}
		bound, statements, sentText, err := boundJSON(stmts)
		if err != nil {
			return err
		}
		res, err := runReadOnlyQuery(ctx, conn, stmts, step.TimeoutOrDefault(), step.MaxRowsOrDefault())
		if err != nil {
			return e.recordFailureWith(conn, key, act, step, actor, pgErrorText(err),
				&outcome{statement: statements, bound: bound, sentText: sentText, sqlHash: sha256Of(text)})
		}
		outs, err := runbook.QueryOutputs(step, res.Columns, res.Rows)
		if err != nil {
			return e.recordFailureWith(conn, key, act, step, actor, err.Error(),
				&outcome{statement: statements, bound: bound, sentText: sentText, sqlHash: sha256Of(text), result: res})
		}
		o := &outcome{outputs: outs, result: res, statement: statements, bound: bound, sentText: sentText, sqlHash: sha256Of(text)}
		if step.Type == runbook.StepWait {
			return e.recordWaitCheck(conn, key, act, step, actor, env, o)
		}
		return e.recordSuccess(conn, key, act, step, actor, o)

	case runbook.StepDDL:
		return e.executeDDL(ctx, conn, key, lr, act, step, actor, env)

	case runbook.StepStartPending:
		if err := e.jetstore.StartPending(ctx, actor); err != nil {
			return e.recordFailure(conn, key, act, step, actor, err.Error())
		}
		return e.recordSuccess(conn, key, act, step, actor, &outcome{})

	case runbook.StepStartPipeline:
		row := map[string]any{}
		if act.Decision != nil && len(act.Bound) > 0 {
			// Accepted: exactly the row the person was shown.
			if err := json.Unmarshal(act.Bound, &row); err != nil {
				return e.recordFailure(conn, key, act, step, actor, err.Error())
			}
		} else if row, err = resolveRow(step, env); err != nil {
			return e.recordFailure(conn, key, act, step, actor, err.Error())
		}
		bound, _ := json.Marshal(row)
		sid, err := e.jetstore.StartPipeline(ctx, actor, row)
		if err != nil {
			return e.recordFailureWith(conn, key, act, step, actor, err.Error(), &outcome{bound: bound})
		}
		v, err := runbook.NewValue(runbook.Type{Base: runbook.TypeSessionID}, sid)
		if err != nil {
			return e.recordFailure(conn, key, act, step, actor, err.Error())
		}
		return e.recordSuccess(conn, key, act, step, actor,
			&outcome{outputs: map[string]runbook.Value{"session_id": v}, bound: bound})

	case runbook.StepPutSchemaEvent:
		var b struct {
			Event string `json:"event"`
			Key   string `json:"key"`
		}
		if act.Decision != nil && len(act.Bound) > 0 {
			if err := json.Unmarshal(act.Bound, &b); err != nil {
				return e.recordFailure(conn, key, act, step, actor, err.Error())
			}
		} else {
			if b.Event, err = resolveEvent(step, env); err != nil {
				return e.recordFailure(conn, key, act, step, actor, err.Error())
			}
			b.Key = step.Key
		}
		bound, _ := json.Marshal(b)
		if _, err := e.jetstore.PutSchemaEvent(ctx, actor, b.Event, b.Key); err != nil {
			return e.recordFailureWith(conn, key, act, step, actor, err.Error(), &outcome{bound: bound})
		}
		return e.recordSuccess(conn, key, act, step, actor, &outcome{bound: bound})

	case runbook.StepCompile:
		if err := e.jetstore.Compile(ctx, actor, run.WorkspaceName, step.DeadlineOrDefault()); err != nil {
			return e.recordFailure(conn, key, act, step, actor, err.Error())
		}
		return e.recordSuccess(conn, key, act, step, actor, &outcome{})
	}
	return e.recordFailure(conn, key, act, step, actor, fmt.Sprintf("step type %s does not execute", step.Type))
}

// executeDDL runs an accepted ddl: exactly the statements and values the person
// was shown, in one transaction with the step's record, as the person who
// accepted it.
func (e *RunbookExecutor) executeDDL(ctx context.Context, conn *pgx.Conn, key int64, lr *loadedRunbook,
	act *rbStep, step *runbook.Step, actor string, env runbook.MapEnv) error {
	if act.Decision == nil || !act.Decision.Accepted {
		// The executor's own check, beside the validator's: a ddl never runs
		// without a person's Accept.
		return e.recordFailure(conn, key, act, step, actor, "a ddl runs only once a person accepts it")
	}
	text, ok := lr.Files[step.SQLPath(lr.Runbook.Runbook)]
	if !ok {
		return e.recordFailure(conn, key, act, step, actor, "its SQL file is missing")
	}
	stmts, err := runbook.Bind(text, step, env)
	if err != nil {
		return e.recordFailure(conn, key, act, step, actor, err.Error())
	}
	bound, statements, sentText, err := boundJSON(stmts)
	if err != nil {
		return err
	}
	if act.SQL == nil || *act.SQL != sentText || !jsonEqual(bound, act.Bound) ||
		act.SQLHash == nil || *act.SQLHash != sha256Of(text) {
		return e.recordFailure(conn, key, act, step, actor,
			"what would run is not what was accepted (statements, bound values or SQL file changed); retry the step to see it again")
	}
	var recorded bool
	_, err = runDDL(ctx, conn, stmts, step.TimeoutOrDefault(), false, func(tx pgx.Tx, entries []previewEntry) error {
		// The record commits with the statements: a run whose step reads
		// completed is a run whose ddl took effect, and the reverse.
		run, err := loadRun(context.Background(), tx, key, true)
		if err != nil {
			return err
		}
		tags := make([]string, len(entries))
		for i, en := range entries {
			tags[i] = en.CommandTag
		}
		o := &outcome{statement: statements, bound: bound, sentText: sentText, sqlHash: sha256Of(text),
			detail: "executed: " + strings.Join(tags, "; ")}
		if err = e.writeSuccess(context.Background(), tx, run, act, step, actor, o); err != nil {
			return err
		}
		recorded = true
		return nil
	})
	if err != nil || !recorded {
		if err == nil {
			err = errors.New("the step's record was not written")
		}
		return e.recordFailureWith(conn, key, act, step, actor, pgErrorText(err),
			&outcome{statement: statements, bound: bound, sentText: sentText, sqlHash: sha256Of(text)})
	}
	return nil
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return string(xa) == string(ya)
}

// outcomeColumns is the SET clause and arguments recording an outcome's data.
func outcomeColumns(o *outcome, start int) (string, []any, error) {
	sets := ""
	var args []any
	add := func(col string, v any, cast string) {
		args = append(args, v)
		sets += fmt.Sprintf(", %s = $%d%s", col, start+len(args)-1, cast)
	}
	if o.outputs != nil {
		b, types, err := valuesJSON(o.outputs)
		if err != nil {
			return "", nil, err
		}
		tb, _ := json.Marshal(types)
		add("outputs", string(b), "::jsonb")
		add("output_types", string(tb), "::jsonb")
	}
	if o.result != nil {
		rows, cut, err := rowsJSON(o.result)
		if err != nil {
			return "", nil, err
		}
		cols, _ := json.Marshal(o.result.Columns)
		add("columns", string(cols), "::jsonb")
		add("rows", string(rows), "::jsonb")
		add("row_count", o.result.RowCount, "")
		add("truncated", o.result.Truncated || cut, "")
	}
	if o.preview != nil {
		b, _ := json.Marshal(o.preview)
		add("preview", string(b), "::jsonb")
	}
	if o.statement != nil {
		add("statements", string(o.statement), "::jsonb")
	}
	if o.bound != nil {
		add("bound", string(o.bound), "::jsonb")
	}
	if o.sentText != "" {
		add("sql", o.sentText, "")
	}
	if o.sqlHash != "" {
		add("sql_hash", o.sqlHash, "")
	}
	return sets, args, nil
}

// writeSuccess records a completed step in tx.
func (e *RunbookExecutor) writeSuccess(ctx context.Context, tx pgx.Tx, run *rbRun, act *rbStep,
	step *runbook.Step, actor string, o *outcome) error {
	extra, args, err := outcomeColumns(o, 3)
	if err != nil {
		return err
	}
	n := 3 + len(args)
	tag, err := tx.Exec(ctx, `UPDATE jetsapi.runbook_step_run SET status = $1, finished_at = now(), error = NULL`+
		extra+`, history = coalesce(history, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
		'at', now(), 'by', $2::text, 'event', 'completed', 'detail', $`+strconv.Itoa(n+1)+`::text))
		WHERE key = $`+strconv.Itoa(n)+` AND status = 'running'`,
		append(append([]any{runbook.StepCompleted, actor}, args...), act.Key, o.detail)...)
	if err != nil {
		return fmt.Errorf("while recording step %s: %v", step.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("step %s is no longer running", step.ID)
	}
	act.Status = runbook.StepCompleted
	if err = auditStep(ctx, tx, run, act, audit.EventOutcome, "executed", actor, ""); err != nil {
		return err
	}
	return saveRun(ctx, tx, run)
}

func (e *RunbookExecutor) recordSuccess(conn *pgx.Conn, key int64, act *rbStep, step *runbook.Step,
	actor string, o *outcome) error {
	return e.withRun(context.Background(), conn, key, func(tx pgx.Tx, run *rbRun) error {
		return e.writeSuccess(context.Background(), tx, run, act, step, actor, o)
	})
}

func (e *RunbookExecutor) recordFailure(conn *pgx.Conn, key int64, act *rbStep, step *runbook.Step, actor, msg string) error {
	return e.recordFailureWith(conn, key, act, step, actor, msg, &outcome{})
}

// recordFailureWith records a failed execution with what it had resolved, and
// applies on_fail to the run unless the run has meanwhile been aborted.
func (e *RunbookExecutor) recordFailureWith(conn *pgx.Conn, key int64, act *rbStep, step *runbook.Step,
	actor, msg string, o *outcome) error {
	ctx := context.Background()
	return e.withRun(ctx, conn, key, func(tx pgx.Tx, run *rbRun) error {
		extra, args, err := outcomeColumns(o, 3)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE jetsapi.runbook_step_run SET status = 'failed', finished_at = now(), error = $1`+
			extra+`, history = coalesce(history, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
			'at', now(), 'by', $2::text, 'event', 'failed', 'detail', $1::text)) WHERE key = $`+strconv.Itoa(3+len(args))+
			` AND status = 'running'`, append(append([]any{msg, actor}, args...), act.Key)...)
		if err != nil {
			return fmt.Errorf("while recording the failure of step %s: %v", step.ID, err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		act.Status = runbook.StepFailed
		if err = auditStep(ctx, tx, run, act, audit.EventOutcome, "failed", actor, msg); err != nil {
			return err
		}
		if run.Status == runbook.RunRunning {
			if err = e.failRun(ctx, tx, run, step, step.ID+": "+msg, actor, false); err != nil {
				return err
			}
		}
		return saveRun(ctx, tx, run)
	})
}

// recordWaitCheck records one check of a wait: completed when until holds,
// waiting for the next check otherwise, failed when its count or deadline is
// spent.
func (e *RunbookExecutor) recordWaitCheck(conn *pgx.Conn, key int64, act *rbStep, step *runbook.Step,
	actor string, env runbook.MapEnv, o *outcome) error {
	ctx := context.Background()
	for col, v := range o.outputs {
		env[runbook.OutputRef(step.ID, col)] = v
	}
	held, evalErr := false, error(nil)
	if step.Until != nil {
		held, evalErr = step.Until.Eval(env)
	}
	if evalErr != nil {
		return e.recordFailureWith(conn, key, act, step, actor, "until cannot be evaluated: "+evalErr.Error(), o)
	}
	if held {
		return e.recordSuccess(conn, key, act, step, actor, o)
	}
	var checks int
	var spent bool
	maxCount := runbook.MaxWaitCount
	if step.MaxCount != nil {
		maxCount = *step.MaxCount
	}
	interval := step.IntervalDuration()
	if interval < runbook.MinWaitInterval {
		interval = runbook.MinWaitInterval
	}
	err := conn.QueryRow(ctx, `SELECT attempts, now() + make_interval(secs => $2) > started_at + make_interval(secs => $3)
		FROM jetsapi.runbook_step_run WHERE key = $1`, act.Key, interval.Seconds(), step.DeadlineOrDefault().Seconds()).
		Scan(&checks, &spent)
	if err != nil {
		return err
	}
	if checks >= maxCount || spent {
		why := fmt.Sprintf("until did not hold after %d check(s)", checks)
		if spent {
			why += fmt.Sprintf(", and the next check would pass the %s deadline", step.DeadlineOrDefault())
		}
		return e.recordFailureWith(conn, key, act, step, actor, why, o)
	}
	return e.withRun(ctx, conn, key, func(tx pgx.Tx, run *rbRun) error {
		extra, args, err := outcomeColumns(o, 3)
		if err != nil {
			return err
		}
		n := 3 + len(args)
		_, err = tx.Exec(ctx, `UPDATE jetsapi.runbook_step_run SET status = $1,
			next_check_at = now() + make_interval(secs => $2)`+extra+`,
			history = coalesce(history, '[]'::jsonb) || jsonb_build_array(jsonb_build_object(
			'at', now(), 'by', $`+strconv.Itoa(n+1)+`::text, 'event', 'waiting'))
			WHERE key = $`+strconv.Itoa(n)+` AND status = 'running'`,
			append(append([]any{runbook.StepWaiting, interval.Seconds()}, args...), act.Key, actor)...)
		if err != nil {
			return err
		}
		return saveRun(ctx, tx, run)
	})
}

// --- the start-up sweep and abort ----------------------------------------------

// SweepInterrupted is run once when the apiserver starts, before its driver:
// a step left running by a stopped apiserver is marked interrupted and its run
// paused. A run another live task is driving (its advisory lock is held) is
// left alone. It returns the number of steps marked.
func (e *RunbookExecutor) SweepInterrupted(ctx context.Context) (int, error) {
	rows, err := e.pool.Query(ctx, `SELECT DISTINCT s.run_key FROM jetsapi.runbook_step_run s
		JOIN jetsapi.runbook_run r ON r.key = s.run_key
		WHERE s.status = $1 AND s.type <> 'repeat'`, runbook.StepRunning)
	if err != nil {
		return 0, fmt.Errorf("while looking for interrupted runbook steps: %v", err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return 0, err
	}
	n := 0
	for _, key := range keys {
		conn, ok, err := e.execConn(ctx, key)
		if err != nil {
			return n, err
		}
		if !ok {
			continue
		}
		err = e.withRun(ctx, conn, key, func(tx pgx.Tx, run *rbRun) error {
			steps, err := loadSteps(ctx, tx, key)
			if err != nil {
				return err
			}
			for _, s := range steps {
				if s.Status == runbook.StepRunning && s.Type != string(runbook.StepRepeat) {
					if err = e.interrupt(ctx, tx, run, s); err != nil {
						return err
					}
					n++
				}
			}
			return nil
		})
		conn.Close(context.Background())
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// cancelRunning cancels the statement a run's step is executing: by its
// context when it runs in this process, else by pg_cancel_backend on the
// connection that carries the run's application name.
func (e *RunbookExecutor) cancelRunning(ctx context.Context, key int64) {
	e.mu.Lock()
	cancel := e.cancels[key]
	e.mu.Unlock()
	if cancel != nil {
		cancel()
		return
	}
	_, err := e.pool.Exec(ctx, `SELECT pg_cancel_backend(pid) FROM pg_stat_activity
		WHERE application_name = $1 AND pid <> pg_backend_pid()`, runbookAppName(key))
	if err != nil {
		log.Printf("runbook run %d: while cancelling its statement: %v", key, err)
	}
}
