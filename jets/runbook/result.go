package runbook

import "time"

// ResultFormatVersion is the `format` of a `.rbr.json` result. It is
// versioned because a model reads it.
const ResultFormatVersion = 1

// Run statuses (runbook_run.status, Result.Status).
const (
	RunRunning   = "running"
	RunPaused    = "paused"
	RunCompleted = "completed"
	RunFailed    = "failed"
	RunAborted   = "aborted"
)

// Step statuses (runbook_step_run.status, StepRecord.Status).
const (
	StepPending              = "pending"
	StepRunning              = "running"
	StepWaiting              = "waiting"
	StepAwaitingConfirmation = "awaiting_confirmation"
	StepAwaitingInput        = "awaiting_input"
	StepCompleted            = "completed"
	StepFailed               = "failed"
	StepSkipped              = "skipped"
	StepDenied               = "denied"
	StepInterrupted          = "interrupted"
)

// Result is the `.rbr.json` export of one released run: every step's record,
// and the hashes of every revision of the files it ran under. It is written to
// a download (or an export prefix), never into the workspace.
type Result struct {
	Format      int            `json:"format"`
	Runbook     string         `json:"runbook"`
	Workspace   string         `json:"workspace"`
	RunKey      int64          `json:"run_key"`
	Status      string         `json:"status"`
	Parameters  map[string]any `json:"parameters"`
	Environment Environment    `json:"environment"`
	Revisions   []Revision     `json:"revisions"`
	StartedBy   string         `json:"started_by"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  *time.Time     `json:"finished_at"`
	ReleasedBy  string         `json:"released_by"`
	ReleasedAt  *time.Time     `json:"released_at"`
	Steps       []StepRecord   `json:"steps"`
}

// Environment is what the run ran against.
type Environment struct {
	JetstoreRelease  string `json:"jetstore_release"`
	WorkspaceVersion string `json:"workspace_version"`
}

// Revision is one version of the runbook's files a run executed under: the
// first at the start, one more for each edit taken at a pause.
type Revision struct {
	Revision int `json:"revision"`
	// Files maps each file, relative to the runbooks directory, to
	// "sha256:<hex>".
	Files map[string]string `json:"files"`
	// FromStep is the first step run under this revision.
	FromStep string `json:"from_step"`
}

// StepRecord is one execution of one step: a step inside a repeat has one
// record per iteration.
type StepRecord struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Iteration int    `json:"iteration"`
	Revision  int    `json:"revision"`
	Status    string `json:"status"`
	SQLHash   string `json:"sql_hash"`
	SQL       string `json:"sql"`
	// Bound is each bind parameter's name and value, as sent.
	Bound map[string]any `json:"bound"`
	// Preview is a ddl's previewed statements, run and rolled back.
	Preview  []PreviewStatement `json:"preview"`
	Decision *Decision          `json:"decision"`
	Outputs  map[string]any     `json:"outputs"`
	// Rows are the result's rows within the step's cap, each a column-to-value
	// object; RowCount is the full count and Truncated says whether rows were
	// cut.
	Rows       []map[string]any `json:"rows"`
	RowCount   int64            `json:"row_count"`
	Truncated  bool             `json:"truncated"`
	StartedAt  *time.Time       `json:"started_at"`
	FinishedAt *time.Time       `json:"finished_at"`
	Actor      string           `json:"actor"`
	Error      *string          `json:"error"`
}

// PreviewStatement is one statement of a previewed ddl.
type PreviewStatement struct {
	Statement    int    `json:"statement"` // 1-based, in the file's order
	CommandTag   string `json:"command_tag"`
	RowsAffected int64  `json:"rows_affected"`
}

// Decision is a person's answer to a step that waited for one: a ddl's
// Accept or Deny, or a confirmation.
type Decision struct {
	By       string    `json:"by"`
	At       time.Time `json:"at"`
	Accepted bool      `json:"accepted"`
	Comment  string    `json:"comment"`
}
