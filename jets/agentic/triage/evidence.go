package triage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/jackc/pgx/v5"
)

// The reads the nine predicates need. Three of them are observe's already —
// the worker rows, the step baseline and the run's stored configuration — and
// are called rather than reimplemented. Three are not, and they are here
// because observe's boundary is the *detection* substrate: it reads what a
// comparison needs, and a comparison never needed the run header's own
// columns, process_errors or the DAG edge.

// Extent is observe's, plus the two tables triage reads that no detector does.
// It is composed rather than added to observe.Extent for the reason above; the
// embedded pointer keeps Regime() and the three deployment flags reachable
// without a second name for them.
type Extent struct {
	*observe.Extent
	// ProcessErrors reports whether jetsapi.process_errors exists. It is a
	// base table installed from jets_schema.json rather than by
	// audit.InstallSchema, so it is present wherever update_db has ever run —
	// but locus per_record_failures_reported is an *absence* claim over it, and
	// an absence claim over a table that might not be there is exactly the
	// conflation NotEvaluable exists to prevent.
	ProcessErrors bool
	// CpipesStatus reports whether jetsapi.cpipes_execution_status exists. It
	// is where a run's configuration survives the run and is the only substrate
	// for locus per_record_failures_unreportable.
	CpipesStatus bool
}

const triageTablesSQL = `SELECT
  to_regclass('jetsapi.process_errors') IS NOT NULL,
  to_regclass('jetsapi.cpipes_execution_status') IS NOT NULL`

// ReadExtent reports what observe.ReadExtent does plus the two tables above.
// to_regclass returns NULL for an absent relation rather than raising, which is
// what lets this answer on a database update_db has never touched — the case
// N.4 found ReadExtent could not report on and fixed (F107).
func ReadExtent(ctx context.Context, db observe.DB) (*Extent, error) {
	base, err := observe.ReadExtent(ctx, db)
	if err != nil {
		return nil, err
	}
	e := &Extent{Extent: base}
	if err := db.QueryRow(ctx, triageTablesSQL).Scan(&e.ProcessErrors, &e.CpipesStatus); err != nil {
		return nil, fmt.Errorf("while reading which triage tables exist: %w", err)
	}
	return e, nil
}

// RunHeader is jetsapi.pipeline_execution_status for one run. Only four of its
// seventeen columns are read: three keys and the status. failure_details is
// read because it is the only evidence locus run_not_started has, and F197/F198
// are why it is carried as prose rather than parsed — the state machine's six
// live catches all route to one error-status task, so the column says a failure
// was caught and not where, and the one machine-readable class the platform
// computes is discarded before the database.
type RunHeader struct {
	Key            int64
	SessionId      string
	Client         string
	ProcessName    string
	MainObjectType string
	Status         string
	FailureDetails string
	StartTime      time.Time
	LastUpdate     time.Time
}

// The run header's terminal statuses. The list is WorkerRow.Stalled's
// (observe/extract.go:204), which is the only other place in the tree that
// asks this question; TestTerminalRunStatusesAgreeWithObserve pins the two
// together so a change to either fails the suite rather than silently making
// the two disagree about what a finished run is.
var terminalRunStatuses = []string{
	"completed", "failed", "timed_out", "recovered", "interrupted", "errors",
}

// Terminal reports whether the run reached a terminal state.
func (h *RunHeader) Terminal() bool {
	for _, s := range terminalRunStatuses {
		if h.Status == s {
			return true
		}
	}
	return false
}

const headerSQL = `SELECT key, session_id, client, process_name, main_object_type,
    status, coalesce(failure_details, ''), start_time, last_update
  FROM jetsapi.pipeline_execution_status WHERE session_id = $1`

// ReadHeader returns the run header, or nil when there is none. Nil is two
// different things — a run that was never submitted and a run whose header the
// six-month purge has taken (F54) — and the caller cannot tell them apart,
// which is why every predicate that needs the header reports NotEvaluable
// rather than Absent when it is missing.
func ReadHeader(ctx context.Context, db observe.DB, sessionId string) (*RunHeader, error) {
	var h RunHeader
	err := db.QueryRow(ctx, headerSQL, sessionId).Scan(
		&h.Key, &h.SessionId, &h.Client, &h.ProcessName, &h.MainObjectType,
		&h.Status, &h.FailureDetails, &h.StartTime, &h.LastUpdate)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("while reading the run header of session %q: %w", sessionId, err)
	}
	return &h, nil
}

// ErrorSummary is what jetsapi.process_errors holds for one run. It is a
// summary rather than the rows because the rows carry nothing to group by that
// a diagnosis wants: F190 says the table has no cpipes_step_id and no
// pipeline_execution_details_key, so an error row joins to one row per step of
// the run and the join is worthless. What is countable is how many, over how
// many shards, and how many carry each of the two columns a single operator
// family writes.
type ErrorSummary struct {
	SessionId string
	Rows      int64
	Shards    int64
	// WithInputColumn counts rows whose input_column is set. Only the inference
	// operator family writes it (F185), so this is the fraction of the run's
	// per-record errors that can say which column failed.
	WithInputColumn int64
	// WithReteSession counts rows carrying a saved rete session, which is the
	// jetrules family's own column.
	WithReteSession int64
	First, Last     time.Time
}

const errorSummarySQL = `SELECT count(*), count(DISTINCT shard_id),
    count(*) FILTER (WHERE coalesce(input_column, '') <> ''),
    count(*) FILTER (WHERE coalesce(rete_session_saved, '') <> ''),
    min(last_update), max(last_update)
  FROM jetsapi.process_errors WHERE session_id = $1`

// ReadErrorSummary counts the run's per-record error rows.
func ReadErrorSummary(ctx context.Context, db observe.DB, sessionId string) (*ErrorSummary, error) {
	s := &ErrorSummary{SessionId: sessionId}
	var first, last *time.Time
	err := db.QueryRow(ctx, errorSummarySQL, sessionId).Scan(
		&s.Rows, &s.Shards, &s.WithInputColumn, &s.WithReteSession, &first, &last)
	if err != nil {
		return nil, fmt.Errorf("while summarising process_errors for session %q: %w", sessionId, err)
	}
	if first != nil {
		s.First = *first
	}
	if last != nil {
		s.Last = *last
	}
	return s, nil
}

// Edge is one row of jetsapi.pipeline_execution_channel_details, plus its
// parent worker's status, which is the whole of what locus
// sink_failed_under_completed_worker compares.
type Edge struct {
	Key          int64
	ParentKey    int64
	SessionId    string
	InputChannel string
	OutputChan   string
	OutputType   string
	// OutputEntity is blank when SinksCount > 1, because the row folds several
	// sinks and names none of them (compute_pipes_results.go:193).
	OutputEntity   string
	OutputLocation string
	SinksCount     int64
	OutputRecords  *int64
	ErrorMessage   string
	// ParentStatus is the worker row's status, or empty when the parent row is
	// gone. An edge whose parent is missing cannot say whether its sink failed
	// *under a completed worker*, which is the whole of the locus.
	ParentStatus string
	ParentStepId string
	ParentShard  *int64
}

const edgesSQL = `SELECT c.key, c.pipeline_execution_details_key, c.session_id,
    c.input_channel, c.output_channel, c.output_type, c.output_entity,
    c.output_location, c.output_sinks_count, c.output_records_count,
    coalesce(c.error_message, ''),
    coalesce(d.status, ''), coalesce(d.cpipes_step_id, ''), d.shard_id
  FROM jetsapi.pipeline_execution_channel_details c
  LEFT JOIN jetsapi.pipeline_execution_details d ON d.key = c.pipeline_execution_details_key
 WHERE c.session_id = $1
 ORDER BY c.key`

// ReadEdges returns the run's DAG edges. The caller must have established that
// the table exists; this is not checked here because the check is a deployment
// question and belongs with the other three in Extent.
//
// The join to the parent is LEFT for observe's reason one table over: the two
// tables are purged on the same session clock, but a row whose parent is absent
// is a different observation from no row, and an inner join would erase it.
func ReadEdges(ctx context.Context, db observe.DB, sessionId string) ([]Edge, error) {
	rows, err := db.Query(ctx, edgesSQL, sessionId)
	if err != nil {
		return nil, fmt.Errorf("while reading the channel details of session %q: %w", sessionId, err)
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.Key, &e.ParentKey, &e.SessionId,
			&e.InputChannel, &e.OutputChan, &e.OutputType, &e.OutputEntity,
			&e.OutputLocation, &e.SinksCount, &e.OutputRecords, &e.ErrorMessage,
			&e.ParentStatus, &e.ParentStepId, &e.ParentShard); err != nil {
			return nil, fmt.Errorf("while scanning a channel detail row: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading channel detail rows: %w", err)
	}
	return out, nil
}

// The five operator types whose failures can reach process_errors, from
// errorChannelConfig (jets/compute_pipes/actions_start_common.go:1091). Any
// other operator's row-level failure has nowhere to go by construction and is
// not this locus.
//
// **Five, not the four F188 measured.** The vllm arm was added by AG.1 on the
// same day AA.1 measured the corpus, so F188's "four operator types" is right
// at its own pin (baee0c47) and is one short at this one. The corpus figures it
// reports are unaffected: there is no vllm instance in any configuration yet.
var errorChannelOperators = []string{"map_record", "jetrules", "ollama", "embed", "vllm"}

// OperatorInstances is what one run's stored configuration says about the
// operators that could report per-record errors.
type OperatorInstances struct {
	// WithChannel and WithoutChannel count instances of the five types above,
	// by whether an error_channel with a name was found in the instance's own
	// subtree.
	WithChannel    map[string]int
	WithoutChannel map[string]int
	// Others counts instances of every other operator type, so a report can say
	// what fraction of the step this survey is about.
	Others int
}

// Total returns the number of instances of the five reporting types.
func (o *OperatorInstances) Total() int {
	n := 0
	for _, c := range o.WithChannel {
		n += c
	}
	for _, c := range o.WithoutChannel {
		n += c
	}
	return n
}

// Unreportable returns the number of instances with no error channel.
func (o *OperatorInstances) Unreportable() int {
	n := 0
	for _, c := range o.WithoutChannel {
		n += c
	}
	return n
}

// ConfigSurvey is the run's configuration read for locus
// per_record_failures_unreportable. It is a report rather than a count for
// observe.ConfigConfounders' reason: an empty survey under Read == false and an
// empty survey under Read == true are different claims, and only the second has
// asked the question.
type ConfigSurvey struct {
	SessionId string
	Read      bool
	Operators OperatorInstances
	// Note says what the read covers or why it failed, in the words a triage
	// verdict's basis needs.
	Note string
}

const configSQL = `SELECT cpipes_config_json FROM jetsapi.cpipes_execution_status
  WHERE session_id = $1`

// ReadConfigSurvey reads the run's stored configuration and counts the
// operator instances that could report per-record errors.
//
// Three limits come with the source and every one of them bounds the locus
// rather than the read (F194, F195, and config.go's own header):
//
//   - cpipes_config_json holds at most ONE step's config — the sharding step's,
//     overwritten by each reducing step — so a survey of it is a survey of the
//     last step to have started and never of the run.
//   - A run that failed at sharding validation leaves no row at all, so the
//     locus is NotEvaluable exactly where locus run_not_started is most likely
//     to be Present.
//   - The row is purged by neither the RETENTION_DAYS clock nor the header's,
//     so a config can outlive the worker rows it describes.
func ReadConfigSurvey(ctx context.Context, db observe.DB, sessionId string) (*ConfigSurvey, error) {
	s := &ConfigSurvey{SessionId: sessionId}
	s.Operators.WithChannel = map[string]int{}
	s.Operators.WithoutChannel = map[string]int{}
	if sessionId == "" {
		s.Note = "no session id, so no configuration was read"
		return s, nil
	}
	var raw string
	err := db.QueryRow(ctx, configSQL, sessionId).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		s.Note = fmt.Sprintf("no cpipes_execution_status row for session %q, which is what a run that "+
			"failed at sharding validation leaves; no operator could be surveyed", sessionId)
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("while reading the configuration of session %q: %w", sessionId, err)
	}
	if raw == "" || raw == "{}" {
		s.Note = fmt.Sprintf("session %q has an empty cpipes_config_json; no operator could be surveyed",
			sessionId)
		return s, nil
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		s.Note = fmt.Sprintf("session %q has a cpipes_config_json that does not parse (%v); no operator "+
			"could be surveyed", sessionId, err)
		return s, nil
	}
	s.Read = true
	s.Operators = surveyOperators(doc)
	s.Note = fmt.Sprintf("configuration read from cpipes_execution_status for session %q; it holds at "+
		"most one step's config — the sharding step's, overwritten by each reducing step — so operators "+
		"of the run's other steps are not in this survey", sessionId)
	return s, nil
}

// surveyOperators walks the parsed configuration for objects carrying a "type"
// in errorChannelOperators and reports, per type, how many declare an error
// channel and how many do not.
//
// It walks the document rather than unmarshalling into ComputePipesConfig, for
// observe.scanConfigConfounders' reason: that type gains a field whenever the
// engine gains an operator, and a triage step that fails to build because a new
// operator landed is worse than one that walks a document. It also means the
// two shapes cpipes_config_json can hold — a sharding config and a reducing
// config — need no discrimination here.
//
// **The error channel is looked for anywhere in the instance's own subtree**,
// which includes a conditional_config arm whose `when` may not have fired. That
// over-reports *reportable*, and over-reporting reportable is the safe
// direction for this locus: the locus asserts that failures had nowhere to go,
// so counting a conditional channel as present makes the classifier less likely
// to assert it, not more.
func surveyOperators(doc any) OperatorInstances {
	out := OperatorInstances{
		WithChannel:    map[string]int{},
		WithoutChannel: map[string]int{},
	}
	var walk func(node any)
	walk = func(node any) {
		switch n := node.(type) {
		case map[string]any:
			if t, ok := n["type"].(string); ok {
				if isErrorChannelOperator(t) {
					if hasNamedErrorChannel(n) {
						out.WithChannel[t]++
					} else {
						out.WithoutChannel[t]++
					}
				} else if _, hasOutput := n["output_channel"]; hasOutput {
					// A "type" beside an output_channel is a transformation
					// rather than a PipeSpec or a schema fragment that happens
					// to carry the word. It is counted only so a report can say
					// what fraction of the step the survey covers.
					out.Others++
				}
			}
			for _, v := range n {
				walk(v)
			}
		case []any:
			for i := range n {
				walk(n[i])
			}
		}
	}
	walk(doc)
	return out
}

func isErrorChannelOperator(t string) bool {
	for _, o := range errorChannelOperators {
		if o == t {
			return true
		}
	}
	return false
}

// hasNamedErrorChannel looks for an error_channel with a non-empty name
// anywhere below node. validateErrorChannels
// (actions_start_common.go:1247) treats a channel with an empty name as no
// channel at all, and so does this.
func hasNamedErrorChannel(node any) bool {
	found := false
	var walk func(any)
	walk = func(n any) {
		if found {
			return
		}
		switch v := n.(type) {
		case map[string]any:
			if ec, ok := v["error_channel"].(map[string]any); ok {
				if name, ok := ec["name"].(string); ok && name != "" {
					found = true
					return
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for i := range v {
				walk(v[i])
			}
		}
	}
	walk(node)
	return found
}

// Evidence is one run's record, and what the read could not see. It is one
// session by construction: Q-23 settled that an incident's grain is the
// session, because session_id is the only key all seven evidence-bearing
// tables carry (F202), and a classifier keyed above that grain would produce
// findings whose evidence sets cannot be assembled by a join.
type Evidence struct {
	SessionId string
	Extent    *Extent

	// Header is nil when the run has none — never submitted, or purged. The
	// two are indistinguishable and every predicate that needs it says so.
	Header *RunHeader

	Workers *observe.WorkerSet
	// Prior is the step history this run is judged against, over a window that
	// ends where this run started. Nil when no baseline was asked for, which
	// makes locus step_never_started NotEvaluable rather than Absent.
	Prior *observe.BaselineSet

	Errors *ErrorSummary
	// Edges is nil when jetsapi.pipeline_execution_channel_details is not
	// deployed, which is the ordinary case (I-132).
	Edges []Edge

	// Config is observe's confounder read, reused so that a finding's
	// qualifiers are the ones a detector would have written.
	Config *observe.ConfigConfounders
	// Survey is the operator census locus per_record_failures_unreportable
	// needs. It is a second read of the same column because the two questions
	// are different: observe asks which of five confounders the config mentions,
	// this asks which operators it configures without an error channel.
	Survey *ConfigSurvey
}

// Gather reads everything the nine predicates need for one run, in one call.
//
// baselineFor is how far back before the run started the step history reaches;
// zero means no baseline is read, which is legitimate — eight of the nine loci
// need none — and leaves Prior nil.
//
// The baseline window is closed at the run's own start by construction rather
// than by the caller's arithmetic, which is the property Gather gives
// observe.StepRegression and which a caller composing windows by hand can fail
// by accident. Where the header is gone the run's start is taken from the
// earliest worker row, and where there are no worker rows either there is
// nothing to bound a baseline with and none is read.
func Gather(ctx context.Context, db observe.DB, sessionId string, baselineFor time.Duration) (*Evidence, error) {
	ev := &Evidence{SessionId: sessionId}
	var err error
	if ev.Extent, err = ReadExtent(ctx, db); err != nil {
		return nil, err
	}
	if !ev.Extent.ExecutionRecord {
		// Nothing below can be read at all. The Extent alone is the answer, and
		// Classify turns it into nine NotEvaluable verdicts rather than nine
		// absent ones.
		return ev, nil
	}
	if ev.Header, err = ReadHeader(ctx, db, sessionId); err != nil {
		return nil, err
	}
	if ev.Workers, err = observe.Workers(ctx, db, observe.Window{
		Since:     time.Time{},
		SessionId: sessionId,
	}); err != nil {
		return nil, err
	}
	if ev.Extent.ProcessErrors {
		if ev.Errors, err = ReadErrorSummary(ctx, db, sessionId); err != nil {
			return nil, err
		}
	}
	if ev.Extent.ChannelDetails {
		if ev.Edges, err = ReadEdges(ctx, db, sessionId); err != nil {
			return nil, err
		}
	}
	if ev.Extent.CpipesStatus {
		if ev.Config, err = observe.ReadConfigConfounders(ctx, db, sessionId); err != nil {
			return nil, err
		}
		if ev.Survey, err = ReadConfigSurvey(ctx, db, sessionId); err != nil {
			return nil, err
		}
	}
	if baselineFor > 0 {
		start, ok := ev.runStart()
		if ok {
			prior := observe.Window{
				Since: start.Add(-baselineFor),
				Until: start,
			}
			if ev.Header != nil {
				prior.Client = ev.Header.Client
				prior.ProcessName = ev.Header.ProcessName
			}
			if ev.Prior, err = observe.StepBaselines(ctx, db, prior); err != nil {
				return nil, err
			}
		}
	}
	return ev, nil
}

// runStart is when this run began: the header's start_time, or the earliest
// worker row's when the header is gone.
func (ev *Evidence) runStart() (time.Time, bool) {
	if ev.Header != nil {
		return ev.Header.StartTime, true
	}
	if ev.Workers == nil || len(ev.Workers.Rows) == 0 {
		return time.Time{}, false
	}
	earliest := ev.Workers.Rows[0].StartTime
	for i := range ev.Workers.Rows {
		if ev.Workers.Rows[i].StartTime.Before(earliest) {
			earliest = ev.Workers.Rows[i].StartTime
		}
	}
	return earliest, true
}

// workerRows is the rows, or an empty slice when nothing was read.
func (ev *Evidence) workerRows() []observe.WorkerRow {
	if ev.Workers == nil {
		return nil
	}
	return ev.Workers.Rows
}
