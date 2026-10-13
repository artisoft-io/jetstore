// Package detectrun records each run of the agentic detection commands,
// anomaly_detect and shadow_triage, in jetsapi.agentic_detection_run, so that
// a person can see that the detectors ran, when, on whose request, and what
// they found, without reading CloudWatch (agentic_ai I-861).
//
// # One row per command per task
//
// The daily detection task runs both commands, one container each
// (cdk/jetstore_one/stack/build_agentic_detect.go), so a run of the task is two
// rows sharing a task_ref, the ECS task ARN. The pair (task_ref, command) is
// unique, and that is what lets the two writers of a row agree on it:
//
//   - the apiserver, when a person presses *Run detection now*, inserts both
//     rows as `requested` with the ARN ECS returned (Request);
//   - each command, when it starts, moves its row to `running`, inserting it if
//     the apiserver did not (Begin) -- a scheduled run has no requester -- and
//     moves it to `ok` or `failed` when it ends (Finish).
//
// A command run outside ECS has no task ARN and records under a local
// reference instead, so a run from a workstation is visible too and is never
// mistaken for the task.
//
// # A record that cannot be written does not stop the detectors
//
// Begin and Finish log a failure to write and carry on. The detectors' output
// is the anomaly and incident rows, and refusing to produce them because the
// bookkeeping failed would trade the thing for its record. The cost is that a
// run can be missing from the screen; the command's log still has it.
package detectrun

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The two commands, as they are recorded.
const (
	CommandAnomalyDetect = "anomaly_detect"
	CommandShadowTriage  = "shadow_triage"
)

// Commands is the order the detection task runs them in.
var Commands = []string{CommandAnomalyDetect, CommandShadowTriage}

// Triggers. TriggerVariable carries one into the container: the daily rule sets
// it to schedule and the apiserver's run action to manual. A command started any
// other way -- `aws ecs run-task` by hand, a workstation -- records TriggerCommand.
const (
	TriggerVariable = "JETS_AGENTIC_DETECT_TRIGGER"
	TriggerSchedule = "schedule"
	TriggerManual   = "manual"
	TriggerCommand  = "command"
)

// RequestedByVariable carries the requesting user's email into the container on
// a manual run, so a row the command inserts itself still names who asked.
const RequestedByVariable = "JETS_AGENTIC_DETECT_REQUESTED_BY"

// Statuses.
const (
	StatusRequested = "requested"
	StatusRunning   = "running"
	StatusOk        = "ok"
	StatusFailed    = "failed"
)

// DB is the slice of a pgx pool used here.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Run is one row.
type Run struct {
	Key          int64           `json:"key"`
	TaskRef      string          `json:"taskRef"`
	Command      string          `json:"command"`
	Trigger      string          `json:"trigger"`
	RequestedBy  string          `json:"requestedBy"`
	Status       string          `json:"status"`
	Since        string          `json:"since"`
	RequestedAt  *time.Time      `json:"requestedAt"`
	StartedAt    *time.Time      `json:"startedAt"`
	FinishedAt   *time.Time      `json:"finishedAt"`
	Summary      json.RawMessage `json:"summary"`
	ErrorMessage string          `json:"errorMessage"`
}

// Request records a manual run the apiserver has just started: one `requested`
// row per command, under the task ARN ECS returned.
func Request(ctx context.Context, db DB, taskRef, requestedBy, since string) error {
	for _, c := range Commands {
		_, err := db.Exec(ctx, `INSERT INTO jetsapi.agentic_detection_run
			(task_ref, command, trigger, requested_by, status, since_window, requested_at, last_update)
			VALUES ($1, $2, $3, $4, $5, $6, now(), now())
			ON CONFLICT (task_ref, command) DO NOTHING`,
			taskRef, c, TriggerManual, requestedBy, StatusRequested, since)
		if err != nil {
			return fmt.Errorf("while recording the requested detection run: %w", err)
		}
	}
	return nil
}

// Window writes a -since duration the way a person and the -since flag both read
// it: whole hours as `26h`, anything finer as Go prints it.
func Window(d time.Duration) string {
	if d > 0 && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int64(d/time.Hour))
	}
	return d.String()
}

// Recorder is one command's row, from Begin to Finish.
type Recorder struct {
	db      DB
	TaskRef string
	Command string
	start   time.Time
}

// Begin moves the command's row to `running`, inserting it when nobody requested
// the run. It never returns nil, so a caller can Finish whatever happened.
func Begin(ctx context.Context, db DB, command, since string) *Recorder {
	r := &Recorder{db: db, TaskRef: TaskRef(ctx), Command: command, start: time.Now()}
	trigger := os.Getenv(TriggerVariable)
	if trigger == "" {
		trigger = TriggerCommand
	}
	_, err := db.Exec(ctx, `INSERT INTO jetsapi.agentic_detection_run
		(task_ref, command, trigger, requested_by, status, since_window, started_at, last_update)
		VALUES ($1, $2, $3, $4, $5, $6, now(), now())
		ON CONFLICT (task_ref, command) DO UPDATE SET
		  status = EXCLUDED.status, started_at = EXCLUDED.started_at,
		  since_window = EXCLUDED.since_window, last_update = now()`,
		r.TaskRef, command, trigger, os.Getenv(RequestedByVariable), StatusRunning, since)
	if err != nil {
		log.Printf("warning: the run of %s could not be recorded in jetsapi.agentic_detection_run, "+
			"so it will not show on the Detection screen: %v", command, err)
	}
	return r
}

// Finish records how the command ended: `ok` with its summary when runErr is
// nil, `failed` with the error otherwise. summary may be nil.
func (r *Recorder) Finish(ctx context.Context, summary any, runErr error) {
	status, msg := StatusOk, ""
	if runErr != nil {
		status, msg = StatusFailed, runErr.Error()
	}
	var doc []byte
	if summary != nil {
		var err error
		if doc, err = json.Marshal(summary); err != nil {
			log.Printf("warning: the summary of %s could not be encoded: %v", r.Command, err)
			doc = nil
		}
	}
	_, err := r.db.Exec(ctx, `UPDATE jetsapi.agentic_detection_run
		SET status = $3, finished_at = now(), summary = $4, error_message = $5, last_update = now()
		WHERE task_ref = $1 AND command = $2`,
		r.TaskRef, r.Command, status, nullIfEmpty(doc), msg)
	if err != nil {
		log.Printf("warning: the end of the run of %s could not be recorded: %v", r.Command, err)
	}
}

func nullIfEmpty(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// List returns the most recent rows first, at most limit (default 50, at most 500).
func List(ctx context.Context, db DB, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := db.Query(ctx, `SELECT key, task_ref, command, trigger, coalesce(requested_by, ''),
		  status, coalesce(since_window, ''), requested_at, started_at, finished_at,
		  coalesce(summary, ''), coalesce(error_message, '')
		FROM jetsapi.agentic_detection_run
		ORDER BY coalesce(started_at, requested_at, last_update) DESC, key DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("while listing detection runs: %w", err)
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var run Run
		var summary string
		if err := rows.Scan(&run.Key, &run.TaskRef, &run.Command, &run.Trigger, &run.RequestedBy,
			&run.Status, &run.Since, &run.RequestedAt, &run.StartedAt, &run.FinishedAt,
			&summary, &run.ErrorMessage); err != nil {
			return nil, fmt.Errorf("while reading a detection run: %w", err)
		}
		if summary != "" && json.Valid([]byte(summary)) {
			run.Summary = json.RawMessage(summary)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// metadataTimeout bounds the call to the ECS task metadata endpoint, which is
// local to the task and answers in milliseconds when it exists at all.
const metadataTimeout = 2 * time.Second

// TaskRef is the ECS task ARN when the process runs in an ECS task, read from
// the task metadata endpoint, and a local reference otherwise.
func TaskRef(ctx context.Context) string {
	if uri := os.Getenv("ECS_CONTAINER_METADATA_URI_V4"); uri != "" {
		if arn, err := taskArn(ctx, uri); err == nil && arn != "" {
			return arn
		} else if err != nil {
			log.Printf("warning: the ECS task metadata could not be read, recording under a local reference: %v", err)
		}
	}
	host, _ := os.Hostname()
	return fmt.Sprintf("local:%s:%d:%d", host, os.Getpid(), time.Now().UnixMilli())
}

func taskArn(ctx context.Context, uri string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, metadataTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(uri, "/")+"/task", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("task metadata endpoint answered %s", resp.Status)
	}
	var doc struct {
		TaskARN string `json:"TaskARN"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", err
	}
	return doc.TaskARN, nil
}
