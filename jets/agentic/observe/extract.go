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

	// Source, when set, keeps the step baselines to the runs of one main input
	// source (StepBaselines): a run is compared with its own source's runs, as
	// triage compares the one run it judges. It is read only where
	// jetsapi.input_registry exists (Extent.Sources); elsewhere every run has
	// the empty source, and a filter on the empty source keeps them all.
	Source *Source
	// BySource, with Source nil, keeps every source but groups the step
	// baselines by source as well (StepBaselines), each StepBaseline one
	// source's and carrying it in Source. It is what a detector over many runs
	// needs, the step-regression detector's Gather: each run is judged against
	// the baseline of its own source, read in one statement for all of them.
	// With neither set, the runs of every source of a process are one baseline,
	// which StepRegression refuses where the database records sources.
	BySource bool
	// RecentRuns, when above 0 and with Client and ProcessName set, has
	// StepBaselines also read, for each step, how many of the source's last
	// RecentRuns runs in the window had it (StepBaseline.Recent). A step the
	// process stopped running is then told from one it still runs. It is not
	// read for a window grouped by source (BySource with Source nil).
	RecentRuns int
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

// Extent is what a deployment's own retention has left of the record. It is the
// code form of the first of the extraction's six queries.
//
// # The regimes, against the session purge
//
// A session is deleted whole, by one path (jets/sessionpurge): by the scheduled
// purge once its session_registry.last_update is RETENTION_DAYS old
// (RetentionCandidates, jets/sessionpurge/sessionpurge.go:174), as an orphan by
// the same age (OrphanCandidates, jets/sessionpurge/sessionpurge.go:217), or by
// a user's Delete at any age, together with every session derived from it. One
// transaction deletes the session's pipeline_execution_details,
// pipeline_execution_channel_details, process_errors and cpipes_execution_status
// rows, and moves its pipeline_execution_status rows, key included, to
// pipeline_execution_status_archive (deleteRows,
// jets/sessionpurge/execute.go:356). An archived run is kept until 180 days
// after its own last_update (archiveDays,
// jets/purge_database/delegate/delegate.go:34), and the view
// pipeline_execution_history is the union of the live table and the archive. A
// run's header no longer ages out while its session lives.
//
// RETENTION_DAYS is a site-global environment variable of the purge lambda
// with no default (DoPurgeSessions, jets/purge_database/delegate/delegate.go:49):
// unset, the purge deletes nothing, and unless PURGE_DRY_RUN is exactly "false"
// it only reports. So which regime a deployment is in is read off the
// deployment, not off this repository. Regime reads it off the keys that
// join a worker row to its header, not off timestamps: a run's header is
// written when the run is submitted and its first worker row later, so the
// oldest header in a live, aligned record is always older than the oldest
// worker row, and a comparison of the two times cannot tell that record from
// one whose detail has gone.
//
//   - header-outlives-detail is a header older than the oldest worker row with
//     no worker row of its own (BareHeaders). The history holds such headers
//     where RETENTION_DAYS is below 180 (the archive keeps a session deleted
//     for retention until its runs are 180 days old) and, at any
//     RETENTION_DAYS, for every session a user deleted. The second kind lies
//     inside the window rather than at its start, so it is not counted here;
//     Archived counts it.
//   - detail-outlives-header is a worker row whose header is in neither the
//     live table nor the archive (HeaderlessWorkers). It is a legacy of the
//     purge before the session purge, which deleted every header at six
//     months while the worker rows followed RETENTION_DAYS. Those headers were
//     deleted, not archived. The worker rows they left drain as their sessions
//     reach RETENTION_DAYS, and never at a site whose purge has not run for
//     real.
//   - mixed is both at once: a site with the legacy rows still draining whose
//     archive also keeps runs older than its worker record.
//   - aligned is neither. Where RETENTION_DAYS is 180 or more, the purge run
//     that archives a session past retention also purges the archived runs
//     older than 180 days, so the archive keeps nothing older than the live
//     record.
type Extent struct {
	// Headers counts the run headers in the record: the live table's and, where
	// the history view exists, the archive's. OldestHeader is the earliest
	// start_time among them, zero when there are none.
	Headers      int64
	OldestHeader time.Time
	OldestWorker time.Time // zero when there are none

	// History reports whether jetsapi.pipeline_execution_history exists. Where
	// it does not, the database predates the archive, and Headers and
	// OldestHeader are the live table's.
	History bool
	// Archived counts the headers in the archive: runs whose session was
	// deleted. Each has lost its worker rows, so a reader of the worker grain
	// cannot see it, and a baseline that should have held it is shorter than
	// its window says. Zero where History is false.
	Archived int64

	// HeaderlessWorkers counts the worker rows whose run header is in neither
	// the live table nor the archive (the live table alone where History is
	// false). BareHeaders counts the headers in the record that are older than
	// the oldest worker row and have no worker row of their own. Regime is
	// read from these two, by key; see Extent.
	HeaderlessWorkers int64
	BareHeaders       int64

	// ExecutionRecord reports whether jetsapi.pipeline_execution_status and
	// jetsapi.pipeline_execution_details both exist. When it is false the three
	// numbers above are zero because there was nothing to count, not because
	// the record is empty, and no detector can run at all.
	//
	// It was added 2026-08-27 because the first caller found the
	// design's claim to be narrower than written: this function reported
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
	// Sources reports whether jetsapi.input_registry exists, which a run's
	// main input source is read from (Source). Where it does not, every run
	// has the empty source.
	Sources bool
}

// Regime names which regime the deployment is in, from the keys that join a
// worker row to its header; see Extent. It is a description rather than a
// setting: nothing here can read RETENTION_DAYS, which lives in the purge
// lambda's environment.
func (e *Extent) Regime() string {
	switch {
	case e.OldestWorker.IsZero() || e.OldestHeader.IsZero():
		return "unknown"
	case e.HeaderlessWorkers > 0 && e.BareHeaders > 0:
		return "mixed"
	case e.HeaderlessWorkers > 0:
		// Detail outlives its header: a join from detail to header drops rows.
		return "detail-outlives-header"
	case e.BareHeaders > 0:
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
  to_regclass('jetsapi.anomaly') IS NOT NULL,
  to_regclass('jetsapi.pipeline_execution_history') IS NOT NULL,
  to_regclass('jetsapi.input_registry') IS NOT NULL`

// extentSQL is for a database that predates the archive. The last two
// columns are HeaderlessWorkers and BareHeaders; both anti-joins are on
// indexed keys (the header's primary key, and
// pipeline_execution_details_pipeline_execution_status_key_idx).
const extentSQL = `SELECT
  (SELECT count(*) FROM jetsapi.pipeline_execution_status),
  0::bigint,
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_status),
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_details),
  (SELECT count(*) FROM jetsapi.pipeline_execution_details d
    WHERE NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_status h
                       WHERE h.key = d.pipeline_execution_status_key)),
  (SELECT count(*) FROM jetsapi.pipeline_execution_status h
    WHERE h.start_time < (SELECT min(start_time) FROM jetsapi.pipeline_execution_details)
      AND NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_details d
                       WHERE d.pipeline_execution_status_key = h.key))`

// extentHistorySQL counts the headers in the history, live and archived.
const extentHistorySQL = `SELECT
  (SELECT count(*) FROM jetsapi.pipeline_execution_history),
  (SELECT count(*) FROM jetsapi.pipeline_execution_history WHERE archived_at IS NOT NULL),
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_history),
  (SELECT min(start_time) FROM jetsapi.pipeline_execution_details),
  (SELECT count(*) FROM jetsapi.pipeline_execution_details d
    WHERE NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_history h
                       WHERE h.key = d.pipeline_execution_status_key)),
  (SELECT count(*) FROM jetsapi.pipeline_execution_history h
    WHERE h.start_time < (SELECT min(start_time) FROM jetsapi.pipeline_execution_details)
      AND NOT EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_details d
                       WHERE d.pipeline_execution_status_key = h.key))`

// ReadExtent reports which of the tables exist and what the deployment's
// retention has left of the record. The two questions are two statements
// because the second cannot be asked of a database that has not been migrated,
// and reporting the first is the whole point of the function.
func ReadExtent(ctx context.Context, db DB) (*Extent, error) {
	var e Extent
	if err := db.QueryRow(ctx, tablesSQL).Scan(
		&e.ExecutionRecord, &e.ChannelDetails, &e.Anomalies, &e.History, &e.Sources); err != nil {
		return nil, fmt.Errorf("while reading which execution tables exist: %w", err)
	}
	if !e.ExecutionRecord {
		return &e, nil
	}
	stmt := extentSQL
	if e.History {
		stmt = extentHistorySQL
	}
	var oldestHeader, oldestWorker *time.Time
	err := db.QueryRow(ctx, stmt).Scan(&e.Headers, &e.Archived, &oldestHeader, &oldestWorker,
		&e.HeaderlessWorkers, &e.BareHeaders)
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
	StepId             string // cpipes_step_id; legitimately empty
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
	// Source is the run's main input source, read through the header where
	// jetsapi.input_registry exists (Extent.Sources). It is the zero Source
	// where that table does not exist, where the run has no main input
	// registration, and where the header is gone.
	Source Source
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
//
// %[1]s is the run's source, SourceColumns where jetsapi.input_registry exists
// and three empty strings where it does not, and %[2]s its join, through the
// header (empty where the table does not exist).
const workersSQL = `SELECT
    d.key, d.pipeline_execution_status_key, d.session_id, d.client, d.process_name,
    d.source_period_key, d.shard_id, d.jets_partition, coalesce(d.cpipes_step_id, ''),
    d.status, coalesce(d.error_message, ''),
    d.input_records_count, d.input_bad_records_count, d.input_files_count,
    d.input_files_size_mb, d.rete_sessions_count, d.output_records_count,
    d.start_time, d.last_update,
    h.key IS NOT NULL, coalesce(h.main_object_type, ''), coalesce(h.status, ''),
    %[1]s
  FROM jetsapi.pipeline_execution_details d
  LEFT JOIN jetsapi.pipeline_execution_status h ON h.key = d.pipeline_execution_status_key
  %[2]s
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
	// Whether the runs' sources can be read, asked on its own because a
	// statement naming an absent relation fails rather than reading nothing.
	var sources bool
	if err := db.QueryRow(ctx,
		`SELECT to_regclass('jetsapi.input_registry') IS NOT NULL`).Scan(&sources); err != nil {
		return nil, fmt.Errorf("while checking for jetsapi.input_registry: %w", err)
	}
	cols, join := noSourceColumns, ""
	if sources {
		cols, join = SourceColumns, SourceJoin("h")
	}
	sql := fmt.Sprintf(workersSQL, cols, join)
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
			&r.HasHeader, &r.MainObjectType, &r.RunStatus,
			&r.Source.Client, &r.Source.Org, &r.Source.ObjectType); err != nil {
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

	// Source is the main input source the baseline was read for, the zero
	// Source when the window kept every source (Window.Source nil).
	Source Source
	// Recent is how many of its source's last Window.RecentRuns runs in the
	// window had the step: 0 for a step the process stopped running that
	// long ago. Meaningful only where BaselineSet.RecentRead.
	Recent int64

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
//
// %[1]s is the source's join and %[2]s its predicate, both empty when the
// window keeps every source (sourceClauses); %[3]s the source's three columns
// and %[4]s their places in the grouping, which are read wherever the join is
// (baselineSourceClauses).
const stepBaselineSQL = `SELECT
    d.client, d.process_name, h.main_object_type, coalesce(d.cpipes_step_id, ''),
    %[3]s,
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
  %[1]s
 WHERE d.start_time >= $1 AND d.start_time < $2
   AND ($3 = '' OR d.client = $3)
   AND ($4 = '' OR d.process_name = $4)
   %[2]s
 GROUP BY 1, 2, 3, 4%[4]s
 ORDER BY 1, 2, 3, 4%[4]s`

// recentRunsSQL reads, for one process of one client (and, with %[2]s, one
// source), the last $5 runs in the window of each main object type: the runs
// with a worker row in the window, latest first. recentStepsSQL counts, for
// each step, how many of them had it; recentCountSQL how many there are.
const recentRunsSQL = `WITH runs AS (
  SELECT h.key, h.main_object_type,
         row_number() OVER (PARTITION BY h.main_object_type ORDER BY h.start_time DESC, h.key DESC) AS rn
    FROM jetsapi.pipeline_execution_status h
    %[1]s
   WHERE h.client = $3 AND h.process_name = $4
     %[2]s
     AND EXISTS (SELECT 1 FROM jetsapi.pipeline_execution_details w
                  WHERE w.pipeline_execution_status_key = h.key AND w.start_time >= $1 AND w.start_time < $2)
), recent AS (SELECT key, main_object_type FROM runs WHERE rn <= $5)`

const recentStepsSQL = recentRunsSQL + `
SELECT r.main_object_type, coalesce(d.cpipes_step_id, ''), count(DISTINCT r.key)
  FROM recent r JOIN jetsapi.pipeline_execution_details d ON d.pipeline_execution_status_key = r.key
 GROUP BY 1, 2`

const recentCountSQL = recentRunsSQL + `
SELECT main_object_type, count(*) FROM recent GROUP BY 1`

// sourceClauses returns the join and the predicate that keep a window's runs
// to its source, in the header alias h, with the source's three values at
// placeholders from first on, and the arguments; all empty when the window
// keeps every source or the database records none.
func sourceClauses(w Window, e *Extent, first int) (join, match string, args []any) {
	if w.Source == nil || e == nil || !e.Sources {
		return "", "", nil
	}
	return SourceJoin("h"), "AND " + SourceMatch(first, first+1, first+2),
		[]any{w.Source.Client, w.Source.Org, w.Source.ObjectType}
}

// baselineSourceClauses is sourceClauses for stepBaselineSQL, which also
// reads each baseline's source wherever the database records sources and the
// window asks for them, a filter (Source) or a grouping (BySource): the join,
// the predicate (a filter's only), the three columns, their grouping, and the
// arguments. Elsewhere the columns are empty strings and there is no join.
func baselineSourceClauses(w Window, e *Extent, first int) (join, match, cols, group string, args []any) {
	if e == nil || !e.Sources || (w.Source == nil && !w.BySource) {
		return "", "", noSourceColumns, "", nil
	}
	cols, group = SourceColumns, ", 5, 6, 7"
	if w.Source == nil {
		return SourceJoin("h"), "", cols, group, nil
	}
	join, match, args = sourceClauses(w, e, first)
	return join, match, cols, group, args
}

const headerlessSQL = `SELECT count(*)
  FROM jetsapi.pipeline_execution_details d
  LEFT JOIN jetsapi.pipeline_execution_status h ON h.key = d.pipeline_execution_status_key
 WHERE h.key IS NULL
   AND d.start_time >= $1 AND d.start_time < $2
   AND ($3 = '' OR d.client = $3)
   AND ($4 = '' OR d.process_name = $4)`

// The runs in the window whose session was deleted, by source. Their worker
// rows were deleted with the session, so no baseline above can hold them; their
// headers survive in the history and say which source lost a run. A header's
// start_time is the run's submission, a little before its first worker's, so a
// run submitted just before the window opens is not counted; it is the same
// bound the worker rows are read with, applied to the header.
// With a source, only that source's runs are counted (%[1]s and %[2]s, as in
// stepBaselineSQL).
const archivedInWindowSQL = `SELECT h.client, h.process_name, h.main_object_type, count(*)
  FROM jetsapi.pipeline_execution_history h
  %[1]s
 WHERE h.archived_at IS NOT NULL
   AND h.start_time >= $1 AND h.start_time < $2
   AND ($3 = '' OR h.client = $3)
   AND ($4 = '' OR h.process_name = $4)
   %[2]s
 GROUP BY 1, 2, 3`

// BaselineSet is the result of StepBaselines: the aggregate, and the numbers
// that say how much of the window it could not see.
type BaselineSet struct {
	Window    Window
	Baselines []StepBaseline
	// BySource reports whether each baseline is one main input source's: the
	// window kept one source (Window.Source) or grouped by source
	// (Window.BySource), or the database records no sources, so that every run
	// has the empty source. False only where sources are recorded and the
	// window read every source of a process as one baseline.
	BySource bool
	// Headerless is the count of worker rows in the window whose header has
	// been purged. They are excluded from every baseline above, because a
	// source identity needs main_object_type and it is only on the header.
	// The session purge deletes a session's worker rows with its header, so
	// these are left by the purge before it, which deleted headers at six
	// months on their own clock.
	Headerless int64
	// Archived counts the runs in the window whose session was deleted, keyed
	// by archivedKey(client, process, main_object_type). Their worker rows
	// went with the session, so every baseline of that source is over fewer
	// runs than the window held, and says so with history_truncated. Empty
	// where the history view does not exist. A set grouped by source
	// (Window.BySource) counts them for every source of the object type: a
	// deleted session's main input registration may have been deleted with
	// it, so its run can no longer be put under one source, and a baseline is
	// then said to be truncated rather than wrongly said to be whole.
	Archived map[string]int64
	// Extent is the deployment's retention state, read in the same call so a
	// caller can tell a genuinely quiet window from a purged one.
	Extent *Extent
	// RecentRead reports whether each baseline's Recent was read
	// (Window.RecentRuns), and RecentRuns, keyed by main object type, how many
	// of the source's last Window.RecentRuns runs the window holds: fewer than
	// asked when the window holds fewer runs.
	RecentRead bool
	RecentRuns map[string]int64
}

// Truncated reports whether the set is over less history than it was asked
// for: the window reaches further back than the record does, or runs in it
// were deleted with their sessions. A caller with one source in the window,
// as triage has, reads it whole; a detector over many sources reads
// ArchivedFor per baseline instead.
func (s *BaselineSet) Truncated() bool {
	return s.reachesPastRecord() || len(s.Archived) > 0
}

func (s *BaselineSet) reachesPastRecord() bool {
	if s.Extent == nil || s.Extent.OldestWorker.IsZero() {
		return false
	}
	return s.Window.Since.Before(s.Extent.OldestWorker)
}

func archivedKey(client, process, objectType string) string {
	return client + "\x00" + process + "\x00" + objectType
}

// ArchivedFor is how many runs of b's source in the window were deleted with
// their sessions. The source is (client, process_name, main_object_type): the
// step is on the worker rows only, and they are gone.
func (s *BaselineSet) ArchivedFor(b *StepBaseline) int64 {
	return s.Archived[archivedKey(b.Client, b.ProcessName, b.MainObjectType)]
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
	set := &BaselineSet{Window: w, Extent: extent,
		BySource: w.Source != nil || w.BySource || !extent.Sources}

	if err := db.QueryRow(ctx, headerlessSQL,
		w.Since, w.until(), w.Client, w.ProcessName).Scan(&set.Headerless); err != nil {
		return nil, fmt.Errorf("while counting worker rows with no header: %w", err)
	}
	if extent.History {
		if set.Archived, err = readArchivedInWindow(ctx, db, w, extent); err != nil {
			return nil, err
		}
	}

	join, match, cols, group, srcArgs := baselineSourceClauses(w, extent, 5)
	args := append([]any{w.Since, w.until(), w.Client, w.ProcessName}, srcArgs...)
	var source Source
	if w.Source != nil {
		source = *w.Source
	}
	rows, err := db.Query(ctx, fmt.Sprintf(stepBaselineSQL, join, match, cols, group), args...)
	if err != nil {
		return nil, fmt.Errorf("while extracting step baselines: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b StepBaseline
		var read Source
		if err := rows.Scan(
			&b.Client, &b.ProcessName, &b.MainObjectType, &b.StepId,
			&read.Client, &read.Org, &read.ObjectType,
			&b.Runs, &b.Workers, &b.Completed, &b.Failed, &b.InProgress,
			&b.FirstSeen, &b.LastSeen, &b.LastSuccess, &b.LastFailure); err != nil {
			return nil, fmt.Errorf("while scanning a step baseline: %w", err)
		}
		// The filter's source where one was set, as before; the row's own
		// where the baselines are grouped by source.
		b.Source = source
		if w.Source == nil && w.BySource {
			b.Source = read
		}
		b.Confounders = baselineConfounders(&b, set)
		set.Baselines = append(set.Baselines, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading step baselines: %w", err)
	}
	rows.Close()
	// Recent is read for one source or for a process whose runs are one
	// baseline, not for a set grouped by source, whose recent runs would be
	// those of every source together.
	if w.RecentRuns > 0 && w.Client != "" && w.ProcessName != "" && (w.Source != nil || !w.BySource) {
		if err := readRecent(ctx, db, w, extent, set); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// readRecent reads, for each baseline, how many of its source's last
// w.RecentRuns runs in the window had the step, and how many such runs there
// are.
func readRecent(ctx context.Context, db DB, w Window, extent *Extent, set *BaselineSet) error {
	join, match, srcArgs := sourceClauses(w, extent, 6)
	args := append([]any{w.Since, w.until(), w.Client, w.ProcessName, w.RecentRuns}, srcArgs...)
	steps := map[string]int64{}
	rows, err := db.Query(ctx, fmt.Sprintf(recentStepsSQL, join, match), args...)
	if err != nil {
		return fmt.Errorf("while reading the steps of the source's recent runs: %w", err)
	}
	for rows.Next() {
		var objectType, step string
		var n int64
		if err := rows.Scan(&objectType, &step, &n); err != nil {
			rows.Close()
			return fmt.Errorf("while scanning a step of the source's recent runs: %w", err)
		}
		steps[objectType+"\x00"+step] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("while reading the steps of the source's recent runs: %w", err)
	}
	set.RecentRuns = map[string]int64{}
	rows, err = db.Query(ctx, fmt.Sprintf(recentCountSQL, join, match), args...)
	if err != nil {
		return fmt.Errorf("while counting the source's recent runs: %w", err)
	}
	for rows.Next() {
		var objectType string
		var n int64
		if err := rows.Scan(&objectType, &n); err != nil {
			rows.Close()
			return fmt.Errorf("while scanning the source's recent runs: %w", err)
		}
		set.RecentRuns[objectType] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("while counting the source's recent runs: %w", err)
	}
	for i := range set.Baselines {
		b := &set.Baselines[i]
		b.Recent = steps[b.MainObjectType+"\x00"+b.StepId]
	}
	set.RecentRead = true
	return nil
}

func readArchivedInWindow(ctx context.Context, db DB, w Window, extent *Extent) (map[string]int64, error) {
	join, match, srcArgs := sourceClauses(w, extent, 5)
	args := append([]any{w.Since, w.until(), w.Client, w.ProcessName}, srcArgs...)
	rows, err := db.Query(ctx, fmt.Sprintf(archivedInWindowSQL, join, match), args...)
	if err != nil {
		return nil, fmt.Errorf("while counting the archived runs in the window: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var client, process, objectType string
		var n int64
		if err := rows.Scan(&client, &process, &objectType, &n); err != nil {
			return nil, fmt.Errorf("while scanning the archived runs in the window: %w", err)
		}
		out[archivedKey(client, process, objectType)] = n
	}
	return out, rows.Err()
}

// baselineConfounders sets only what this baseline's own rows establish. A
// confounder the record cannot see is not set here and is not silently
// assumed absent — see the package comment.
func baselineConfounders(b *StepBaseline, set *BaselineSet) []string {
	var out []string
	if b.StepId == "" {
		// cpipes_step_id is a stage location and ten reducing steps in
		// the corpus carry an empty one, so this baseline may be two steps.
		out = append(out, ConfounderStepLabelAmbiguous)
	}
	if set.Headerless > 0 || set.reachesPastRecord() || set.ArchivedFor(b) > 0 {
		out = append(out, ConfounderHistoryTruncated)
	}
	if set.Extent != nil && !set.Extent.ChannelDetails {
		out = append(out, ConfounderCrossStepJoinUnavailable)
	}
	return out
}

// Describe renders a window for an Anomaly's expected_basis, which the
// anomaly design asks to be human-readable and which is the field a triage step reads
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
	if n := s.ArchivedFor(b); n > 0 {
		fmt.Fprintf(&sb, "; %d runs of %s/%s/%s in the window were deleted with their sessions, "+
			"and their worker rows with them, so they are not in this count", n, b.Client, b.ProcessName,
			b.MainObjectType)
	}
	return sb.String()
}
