package observe

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Window bounds an extraction. Every field is optional except the time bounds,
// and an empty string filter means "no filter" rather than "match the empty
// string" — cpipes_step_id is legitimately empty for ten reducing steps in the
// corpus, so a filter that could not be omitted would be a trap.
type Window struct {
	// Since and Until bound pipeline_execution_details.start_time, which is
	// set at the insert and left alone by the update
	// (UpdatePipelineExecutionStatus, actions_process_file.go:320), so it is
	// the moment the worker started rather than the moment it finished.
	Since time.Time
	Until time.Time // zero means now

	Client      string // optional
	ProcessName string // optional
	SessionId   string // optional: one run, for a within-run predicate

	// Limit caps the rows returned; 0 means no cap. A capped extraction sets
	// ConfounderHistoryTruncated on the result, because a detector cannot
	// tell a short window from a truncated one by looking at the rows.
	Limit int
}

func (w Window) until() time.Time {
	if w.Until.IsZero() {
		// UTC, because the bound is compared against
		// pipeline_execution_details.start_time, which is `timestamp without
		// time zone` holding UTC. A local clock west of Greenwich puts the
		// bound behind rows that have just been written, and the detectors go
		// blind for exactly the UTC offset -- on a deployment at -04:00, the
		// four hours a scheduled run is most likely to be asked about. Every
		// site that *writes* a timestamp here already uses .UTC(); these two
		// reads were the exception. Found 2026-09-01 against a real run.
		return time.Now().UTC()
	}
	return w.Until
}

// Extent is what a deployment's own retention settings have left of the
// record. It is the code form of the first of N.1's six queries, and it exists
// because RETENTION_DAYS is a site-global environment variable with no default
// (DoPurgeSessions, jets/purge_database/delegate/delegate.go:26): which of its
// three regimes a deployment is in has to be read off the deployment, not off
// this repository.
type Extent struct {
	Headers      int64
	OldestHeader time.Time // zero when there are none
	OldestWorker time.Time // zero when there are none

	// ExecutionRecord reports whether jetsapi.pipeline_execution_status and
	// jetsapi.pipeline_execution_details both exist. When it is false the three
	// numbers above are zero because there was nothing to count, not because
	// the record is empty, and no detector can run at all.
	//
	// It was added at N.4 (2026-08-27) because the first caller found the
	// claim in section 18.7 to be narrower than written: this function reported
	// the two tables below and did so from the same statement that counted the
	// header rows, so on a database where nothing had been migrated the query
	// failed on the missing header table and reported neither. The precondition
	// is only checkable if the check cannot itself fail on a missing relation,
	// and to_regclass is the one thing here that cannot.
	ExecutionRecord bool
	// ChannelDetails reports whether jetsapi.pipeline_execution_channel_details
	// exists. It is created by `update_db -migrateDb` from jets_schema.json and
	// was absent from all four production environments measured on 2026-08-25,
	// so a detector reading the DAG edge must check rather than assume.
	ChannelDetails bool
	// Anomalies reports whether jetsapi.anomaly exists. It arrives by the same
	// update_db run — audit.InstallSchema is called from
	// jets/update_db/main.go:71 — so the write target of every detector shares
	// the deployment precondition of the table above.
	Anomalies bool
}

// Regime names which of the three purge regimes the deployment is in, from the
// two oldest timestamps. It is a description rather than a setting: nothing
// here can read RETENTION_DAYS, which lives in the purge lambda's environment.
func (e *Extent) Regime() string {
	switch {
	case e.OldestWorker.IsZero() || e.OldestHeader.IsZero():
		return "unknown"
	case e.OldestWorker.Before(e.OldestHeader):
		// Detail outlives its header: a join from detail to header drops rows.
		return "detail-outlives-header"
	case e.OldestWorker.After(e.OldestHeader):
		// Headers outlive their detail: a run with zero detail rows is not a
		// run that produced nothing.
		return "header-outlives-detail"
	default:
		return "aligned"
	}
}

// The tables, asked first and separately. to_regclass returns NULL for an
// absent relation rather than raising, so this statement answers on any
// database with a jetsapi schema — including one where update_db has never
// run, which is the case it exists for.
const tablesSQL = `SELECT
  to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL
    AND to_regclass('jetsapi.pipeline_execution_details') IS NOT NULL,
  to_regclass('jetsapi.pipeline_execution_channel_details') IS NOT NULL,
  to_regclass('jetsapi.anomaly') IS NOT NULL`

const extentSQL = `SELECT
  (SELECT count(*) FROM jetsapi.pipeline_execution_status),
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_status),
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_details)`

// ReadExtent reports which of the four tables exist and what the deployment's
// retention has left of the record. The two questions are two statements
// because the second cannot be asked of a database that has not been migrated,
// and reporting the first is the whole point of the function.
func ReadExtent(ctx context.Context, db DB) (*Extent, error) {
	var e Extent
	if err := db.QueryRow(ctx, tablesSQL).Scan(
		&e.ExecutionRecord, &e.ChannelDetails, &e.Anomalies); err != nil {
		return nil, fmt.Errorf("while reading which execution tables exist: %w", err)
	}
	if !e.ExecutionRecord {
		return &e, nil
	}
	var oldestHeader, oldestWorker *time.Time
	err := db.QueryRow(ctx, extentSQL).Scan(&e.Headers, &oldestHeader, &oldestWorker)
	if err != nil {
		return nil, fmt.Errorf("while reading the execution record's extent: %w", err)
	}
	if oldestHeader != nil {
		e.OldestHeader = *oldestHeader
	}
	if oldestWorker != nil {
		e.OldestWorker = *oldestWorker
	}
	return &e, nil
}

// WorkerRow is one row of jetsapi.pipeline_execution_details, plus the two
// header columns that are not on it, when the header survived.
//
// The six counts are pointers because the column is nullable and is NULL for
// exactly as long as the worker is running: the insert names eleven columns
// and none of them is a count, and only UpdatePipelineExecutionStatus
// (actions_process_file.go:320) sets them. So *nil* here is the same
// observation as Status == StatusInProgress, and arithmetic on the counts of a
// stalled worker yields nothing rather than zero.
type WorkerRow struct {
	Key                int64
	ExecutionStatusKey int64
	SessionId          string
	Client             string
	ProcessName        string
	SourcePeriodKey    int
	ShardId            int
	JetsPartition      string
	StepId             string // cpipes_step_id; legitimately empty (F52)
	Status             string
	ErrorMessage       string

	InputRecords     *int64
	InputBadRecords  *int64
	InputFilesCount  *int64
	InputFilesSizeMb *int64
	ReteSessions     *int64
	OutputRecords    *int64

	StartTime  time.Time
	LastUpdate time.Time

	// HasHeader is false when the run header has been purged out from under
	// this row. MainObjectType and RunStatus are then empty, and the row
	// cannot be assigned to a source identity.
	HasHeader      bool
	MainObjectType string
	RunStatus      string
}

// The worker statuses. The worker's own switch produces three terminal values
// over the 'in progress' the insert wrote
// (ProcessFilesAndReportStatus, jets/compute_pipes/actions_process_file.go:267).
const (
	StatusInProgress  = "in progress"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusInterrupted = "interrupted"
)

// Stalled reports section 12.2 row 4's predicate: a worker that never reached
// a terminal state under a run header that did. It is false where the header
// is gone, because the predicate is not evaluable without it — which is a
// different answer from "no stall" and is why the count of headerless rows
// travels with the result.
func (w *WorkerRow) Stalled() bool {
	if !w.HasHeader || w.Status != StatusInProgress {
		return false
	}
	switch w.RunStatus {
	case "completed", "failed", "timed_out", "recovered", "interrupted", "errors":
		return true
	}
	return false
}

// The LEFT JOIN is the load-bearing token in this statement. An inner join
// silently drops every worker row whose header has been purged, which in three
// of four measured environments is the older part of the record — the part a
// baseline most wants.
const workersSQL = `SELECT
    d.key, d.pipeline_execution_status_key, d.session_id, d.client, d.process_name,
    d.source_period_key, d.shard_id, d.jets_partition, coalesce(d.cpipes_step_id, ''),
    d.status, coalesce(d.error_message, ''),
    d.input_records_count, d.input_bad_records_count, d.input_files_count,
    d.input_files_size_mb, d.rete_sessions_count, d.output_records_count,
    d.start_time, d.last_update,
    h.key IS NOT NULL, coalesce(h.main_object_type, ''), coalesce(h.status, '')
  FROM jetsapi.pipeline_execution_details d
  LEFT JOIN jetsapi.pipeline_execution_status h ON h.key = d.pipeline_execution_status_key
 WHERE d.start_time >= $1 AND d.start_time < $2
   AND ($3 = '' OR d.client = $3)
   AND ($4 = '' OR d.process_name = $4)
   AND ($5 = '' OR d.session_id = $5)
 ORDER BY d.start_time, d.key`

// WorkerSet is the result of Workers: the rows, and what the extraction had to
// leave out.
type WorkerSet struct {
	Window Window
	Rows   []WorkerRow

	// Headerless counts rows in Rows whose header has been purged. They are
	// returned rather than dropped — a within-run predicate does not need the
	// header — but they cannot carry a source identity.
	Headerless int
	// Truncated is set when Limit cut the result, so a caller cannot mistake a
	// capped read for an exhausted one.
	Truncated bool
}

// Workers extracts the worker grain: the substrate for section 12.2's row 2
// (an output count that collapsed relative to its input) and row 4 (a worker
// that never reached a terminal state). Neither needs a history, so neither
// needs a baseline window — a single run's session id is a legitimate Window.
func Workers(ctx context.Context, db DB, w Window) (*WorkerSet, error) {
	sql := workersSQL
	if w.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", w.Limit+1)
	}
	rows, err := db.Query(ctx, sql, w.Since, w.until(), w.Client, w.ProcessName, w.SessionId)
	if err != nil {
		return nil, fmt.Errorf("while extracting worker rows: %w", err)
	}
	defer rows.Close()

	set := &WorkerSet{Window: w}
	for rows.Next() {
		var r WorkerRow
		if err := rows.Scan(
			&r.Key, &r.ExecutionStatusKey, &r.SessionId, &r.Client, &r.ProcessName,
			&r.SourcePeriodKey, &r.ShardId, &r.JetsPartition, &r.StepId,
			&r.Status, &r.ErrorMessage,
			&r.InputRecords, &r.InputBadRecords, &r.InputFilesCount,
			&r.InputFilesSizeMb, &r.ReteSessions, &r.OutputRecords,
			&r.StartTime, &r.LastUpdate,
			&r.HasHeader, &r.MainObjectType, &r.RunStatus); err != nil {
			return nil, fmt.Errorf("while scanning a worker row: %w", err)
		}
		if !r.HasHeader {
			set.Headerless++
		}
		set.Rows = append(set.Rows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading worker rows: %w", err)
	}
	if w.Limit > 0 && len(set.Rows) > w.Limit {
		set.Truncated = true
		set.Rows = set.Rows[:w.Limit]
		// Headerless was counted over the read rows; recount over the kept ones.
		set.Headerless = 0
		for i := range set.Rows {
			if !set.Rows[i].HasHeader {
				set.Headerless++
			}
		}
	}
	return set, nil
}

// StepBaseline is one step's history within the window, keyed the way section
// 12.2 row 3 says a source has to be keyed: (client, process_name,
// main_object_type, cpipes_step_id) rather than by the input file key, which
// for a periodic feed differs every run.
type StepBaseline struct {
	Client         string
	ProcessName    string
	MainObjectType string
	StepId         string

	Runs       int64 // distinct run headers in the window
	Workers    int64
	Completed  int64
	Failed     int64
	InProgress int64

	FirstSeen   time.Time
	LastSeen    time.Time
	LastSuccess *time.Time
	LastFailure *time.Time

	// Confounders are the ones this baseline's own rows show. They are the
	// subset of the Anomaly vocabulary the record can establish; the rest are
	// the detector's to add from the configuration.
	Confounders []string
}

// EverSucceeded is row 3's precondition: the step has a success to have
// regressed from. A step that has only ever failed is a broken configuration
// rather than a regression, and the two want different remedies.
func (b *StepBaseline) EverSucceeded() bool { return b.Completed > 0 }

// A worker row whose header is gone cannot be assigned a main_object_type, so
// it is excluded from the aggregate and counted separately. Grouping it under
// an empty object type would silently invent a fourth source.
const stepBaselineSQL = `SELECT
    d.client, d.process_name, h.main_object_type, coalesce(d.cpipes_step_id, ''),
    count(DISTINCT d.pipeline_execution_status_key),
    count(*),
    count(*) FILTER (WHERE d.status = 'completed'),
    count(*) FILTER (WHERE d.status = 'failed'),
    count(*) FILTER (WHERE d.status = 'in progress'),
    min(d.start_time), max(d.start_time),
    max(d.start_time) FILTER (WHERE d.status = 'completed'),
    max(d.start_time) FILTER (WHERE d.status = 'failed')
  FROM jetsapi.pipeline_execution_details d
  JOIN jetsapi.pipeline_execution_status h ON h.key = d.pipeline_execution_status_key
 WHERE d.start_time >= $1 AND d.start_time < $2
   AND ($3 = '' OR d.client = $3)
   AND ($4 = '' OR d.process_name = $4)
 GROUP BY 1, 2, 3, 4
 ORDER BY 1, 2, 3, 4`

const headerlessSQL = `SELECT count(*)
  FROM jetsapi.pipeline_execution_details d
  LEFT JOIN jetsapi.pipeline_execution_status h ON h.key = d.pipeline_execution_status_key
 WHERE h.key IS NULL
   AND d.start_time >= $1 AND d.start_time < $2
   AND ($3 = '' OR d.client = $3)
   AND ($4 = '' OR d.process_name = $4)`

// BaselineSet is the result of StepBaselines: the aggregate, and the two
// numbers that say how much of the window it could not see.
type BaselineSet struct {
	Window    Window
	Baselines []StepBaseline
	// Headerless is the count of worker rows in the window whose header has
	// been purged. They are excluded from every baseline above, because a
	// source identity needs main_object_type and it is only on the header.
	Headerless int64
	// Extent is the deployment's retention state, read in the same call so a
	// caller can tell a genuinely quiet window from a purged one.
	Extent *Extent
}

// Truncated reports whether the window reaches further back than the record
// does, in which case every baseline in the set is over less history than it
// was asked for.
func (s *BaselineSet) Truncated() bool {
	if s.Extent == nil || s.Extent.OldestWorker.IsZero() {
		return false
	}
	return s.Window.Since.Before(s.Extent.OldestWorker)
}

// StepBaselines extracts the windowed aggregate that section 12.2's row 3
// compares a run against: per (client, process_name, main_object_type,
// cpipes_step_id), how the step has been going.
//
// This is the whole of the "SQL" in "SQL plus a Go emitter" for row 3 — one
// GROUP BY with four FILTER clauses. The alternative shapes exist to carry
// these same rows to an engine that would compute this again.
func StepBaselines(ctx context.Context, db DB, w Window) (*BaselineSet, error) {
	extent, err := ReadExtent(ctx, db)
	if err != nil {
		return nil, err
	}
	set := &BaselineSet{Window: w, Extent: extent}

	if err := db.QueryRow(ctx, headerlessSQL,
		w.Since, w.until(), w.Client, w.ProcessName).Scan(&set.Headerless); err != nil {
		return nil, fmt.Errorf("while counting worker rows with no header: %w", err)
	}

	rows, err := db.Query(ctx, stepBaselineSQL, w.Since, w.until(), w.Client, w.ProcessName)
	if err != nil {
		return nil, fmt.Errorf("while extracting step baselines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b StepBaseline
		if err := rows.Scan(
			&b.Client, &b.ProcessName, &b.MainObjectType, &b.StepId,
			&b.Runs, &b.Workers, &b.Completed, &b.Failed, &b.InProgress,
			&b.FirstSeen, &b.LastSeen, &b.LastSuccess, &b.LastFailure); err != nil {
			return nil, fmt.Errorf("while scanning a step baseline: %w", err)
		}
		b.Confounders = baselineConfounders(&b, set)
		set.Baselines = append(set.Baselines, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading step baselines: %w", err)
	}
	return set, nil
}

// baselineConfounders sets only what this baseline's own rows establish. A
// confounder the record cannot see is not set here and is not silently
// assumed absent — see the package comment.
func baselineConfounders(b *StepBaseline, set *BaselineSet) []string {
	var out []string
	if b.StepId == "" {
		// F52: cpipes_step_id is a stage location and ten reducing steps in
		// the corpus carry an empty one, so this baseline may be two steps.
		out = append(out, ConfounderStepLabelAmbiguous)
	}
	if set.Headerless > 0 || set.Truncated() {
		out = append(out, ConfounderHistoryTruncated)
	}
	if set.Extent != nil && !set.Extent.ChannelDetails {
		out = append(out, ConfounderCrossStepJoinUnavailable)
	}
	return out
}

// Describe renders a window for an Anomaly's expected_basis, which section
// A.2.6 asks to be human-readable and which is the field a triage step reads
// to know what the comparison was against.
func (s *BaselineSet) Describe(b *StepBaseline) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d runs of %s/%s/%s step %q between %s and %s: %d completed, %d failed",
		b.Runs, b.Client, b.ProcessName, b.MainObjectType, b.StepId,
		b.FirstSeen.Format(time.RFC3339), b.LastSeen.Format(time.RFC3339),
		b.Completed, b.Failed)
	if s.Headerless > 0 {
		fmt.Fprintf(&sb, "; %d worker rows in the window have no run header and are excluded", s.Headerless)
	}
	return sb.String()
}
