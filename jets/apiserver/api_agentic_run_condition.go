package main

// A run incident's condition: what get_incident, list_incidents and
// find_session_incidents read of it.
//
// A process that fails the same way run after run is one fault, and shadow
// triage raises one incident for it, on its first run: the consecutive live
// runs of one process for one client with a present finding at one run locus,
// and at per_record_failures_reported one error code, are a run condition, and
// each later run joins its incident and carries none of its own. A run incident
// written so carries its key and its position (jetsapi.incident's
// incident_client, incident_process_name, incident_anchor_start_time and
// incident_error_code), and is found by them, never by its session, so it is
// still its condition's incident after its own run is deleted.
//
// **None of these reads derives a join.** Which incident a run belongs to is
// the runcondition package's to say (History.Resolve and History.Members), from
// what the database holds; these reads only shape its answer for the screens,
// as api_agentic_condition.go does for a feed condition. All of them write
// nothing.
//
// **Nothing here reads a message.** A per-record failure is named by its error
// code, built from rule literals and JetStore constants, and counted by the
// rows jetsapi.run_finding stored for it; a message's text is read on the
// session's Process Errors rows, under the apiserver's own permissions.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/runcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
	"github.com/jackc/pgx/v5"
)

// RunIncidentKey is what a run-locus incident carries of its run condition.
type RunIncidentKey struct {
	// ErrorCode is the incident's error code at per_record_failures_reported,
	// "" elsewhere and on an incident written before error codes.
	ErrorCode string
	// Client and Process are the client and process of the run the incident is
	// written on, "" on an incident written before run conditions or on a run
	// with no live header.
	Client, Process string
	// Anchor is the start time of the run the incident is written on, its
	// position, kept after that run is deleted; zero when it carries no key.
	Anchor time.Time
}

// Keyed reports whether the incident carries a run condition's key, so a
// condition can find it.
func (k RunIncidentKey) Keyed() bool { return k.Client != "" && k.Process != "" && !k.Anchor.IsZero() }

// Key is the run condition's key at a locus.
func (k RunIncidentKey) Key(locus string) runcondition.Key {
	return runcondition.Key{Client: k.Client, Process: k.Process, Locus: locus, Code: k.ErrorCode}
}

// runIncidentKeyColumnsSQL asks whether jetsapi.incident has the four columns
// a run incident's key is read from; a database migrated before them has none.
const runIncidentKeyColumnsSQL = `SELECT to_regclass('jetsapi.incident') IS NOT NULL
   AND (SELECT count(*) FROM information_schema.columns
         WHERE table_schema = 'jetsapi' AND table_name = 'incident'
           AND column_name IN ('incident_error_code', 'incident_client', 'incident_process_name',
                               'incident_anchor_start_time')) = 4`

const readRunIncidentKeysSQL = `SELECT incident_id, coalesce(incident_error_code, ''),
       coalesce(incident_client, ''), coalesce(incident_process_name, ''), incident_anchor_start_time
  FROM jetsapi.incident
 WHERE incident_id = ANY($1::text[])`

// readRunIncidentKeys reads the key of each named incident. An incident with
// none of the four columns set is left out, as is every incident of a
// database that has not the columns.
func readRunIncidentKeys(ctx context.Context, db observe.DB, ids []string) (map[string]RunIncidentKey, error) {
	out := map[string]RunIncidentKey{}
	if len(ids) == 0 {
		return out, nil
	}
	var has bool
	if err := db.QueryRow(ctx, runIncidentKeyColumnsSQL).Scan(&has); err != nil {
		return nil, fmt.Errorf("while asking whether incidents carry a run condition's key: %w", err)
	}
	if !has {
		return out, nil
	}
	rows, err := db.Query(ctx, readRunIncidentKeysSQL, ids)
	if err != nil {
		return nil, fmt.Errorf("while reading the run condition keys of incidents %v: %w", ids, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var k RunIncidentKey
		var anchor *time.Time
		if err := rows.Scan(&id, &k.ErrorCode, &k.Client, &k.Process, &anchor); err != nil {
			return nil, fmt.Errorf("while scanning an incident's run condition key: %w", err)
		}
		if anchor != nil {
			k.Anchor = *anchor
		}
		if k != (RunIncidentKey{}) {
			out[id] = k
		}
	}
	return out, rows.Err()
}

// RunConditionRun is one live run of a run condition.
type RunConditionRun struct {
	SessionId string
	StartTime time.Time
	// StoredRows is, at per_record_failures_reported, the code's stored rows in
	// the run: a lower bound, because an operator stops storing rows at its
	// max_error_count. Nil at the other loci.
	StoredRows *int64
	// IncidentId is the incident the run is placed in.
	IncidentId string
}

// IncidentRunCondition is a run incident's condition, read live.
type IncidentRunCondition struct {
	Key runcondition.Key
	// FirstSeen is the incident's position, the start time of the run it is
	// written on, kept after that run is deleted, or an earlier member's when
	// one resolves to it. LastSeen is the start time of its latest member still
	// held, zero when none is.
	FirstSeen, LastSeen time.Time
	// Runs are the condition's live runs placed in this incident, in order:
	// the runs still held, which falls as retention deletes runs.
	Runs []RunConditionRun
	// ConditionRuns counts every live run of the condition, those placed in a
	// later incident (one raised after a person's verdict) included.
	ConditionRuns int
	// OwnRunHeld reports that the run the incident is written on is still live.
	OwnRunHeld bool
	Open       bool
	// EndedBy is the live run whose finding here is absent and that ended the
	// condition, "" while it is open.
	EndedBy string
}

// AloneOnItsRun reports that the condition holds the incident's own run and
// nothing else: what every run incident was before run conditions, which a
// screen need not restate.
func (c *IncidentRunCondition) AloneOnItsRun() bool {
	return c.OwnRunHeld && len(c.Runs) == 1 && c.ConditionRuns == 1
}

// RunCode is one error code a run's per-record failures were given, with its
// stored rows, from jetsapi.run_finding.
type RunCode struct {
	Code string
	// Verdict is present when the code reports and quiet when it does not
	// (below its threshold, or a debug code).
	Verdict    string
	StoredRows int64
}

// RunIncidentDetail is what get_incident adds to a run incident.
type RunIncidentDetail struct {
	Key RunIncidentKey
	// Codes are the error codes of the incident's run at
	// per_record_failures_reported, most rows first; empty at any other locus
	// and once the run is deleted.
	Codes []RunCode
	// Condition is the incident's run condition, nil when it has none; Note
	// then says why.
	Condition *IncidentRunCondition
	Note      string
}

// runConditionOf builds the incident's condition from its key's history: the
// members that resolve to it (History.Members), dated by start time. Nil when
// no condition holds it.
func runConditionOf(h *runcondition.History, incidentId string, key RunIncidentKey, session string) *IncidentRunCondition {
	c, members := h.Members(incidentId)
	if c == nil {
		return nil
	}
	out := &IncidentRunCondition{Key: h.Key, FirstSeen: key.Anchor, ConditionRuns: len(c.Members), Open: c.Open()}
	if c.EndedBy != nil {
		out.EndedBy = c.EndedBy.SessionId
	}
	for _, m := range members {
		out.Runs = append(out.Runs, RunConditionRun{SessionId: m.SessionId, StartTime: m.StartTime,
			StoredRows: m.StoredRows, IncidentId: incidentId})
		if m.StartTime.Before(out.FirstSeen) {
			out.FirstSeen = m.StartTime
		}
		if m.StartTime.After(out.LastSeen) {
			out.LastSeen = m.StartTime
		}
		if m.SessionId == session {
			out.OwnRunHeld = true
		}
	}
	return out
}

const readRunCodesSQL = `SELECT error_code, verdict, coalesce(stored_rows, 0)
  FROM jetsapi.run_finding
 WHERE session_id = $1 AND locus = $2 AND error_code <> ''
 ORDER BY stored_rows DESC NULLS LAST, error_code`

// readRunCodes reads the error codes of a run's per-record failures and their
// stored rows; none where jetsapi.run_finding is absent.
func readRunCodes(ctx context.Context, db observe.DB, session string) ([]RunCode, error) {
	var has bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.run_finding') IS NOT NULL`).Scan(&has); err != nil {
		return nil, fmt.Errorf("while asking whether run findings are stored: %w", err)
	}
	if !has {
		return nil, nil
	}
	rows, err := db.Query(ctx, readRunCodesSQL, session, triage.LocusPerRecordFailuresReported)
	if err != nil {
		return nil, fmt.Errorf("while reading the error codes of session %s: %w", session, err)
	}
	defer rows.Close()
	var out []RunCode
	for rows.Next() {
		var c RunCode
		if err := rows.Scan(&c.Code, &c.Verdict, &c.StoredRows); err != nil {
			return nil, fmt.Errorf("while scanning an error code of session %s: %w", session, err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// readRunIncident reads what get_incident adds to a run incident: its code,
// the codes of its run with their stored rows, and its run condition, read
// live through the runcondition package. Nil for a delivery locus.
func readRunIncident(ctx context.Context, db observe.DB, inc *audit.Incident) (*RunIncidentDetail, error) {
	if !runcondition.IsRunLocus(inc.Locus) {
		return nil, nil
	}
	keys, err := readRunIncidentKeys(ctx, db, []string{inc.IncidentId})
	if err != nil {
		return nil, err
	}
	d := &RunIncidentDetail{Key: keys[inc.IncidentId]}
	if inc.Locus == triage.LocusPerRecordFailuresReported {
		if d.Codes, err = readRunCodes(ctx, db, inc.SessionId); err != nil {
			return nil, err
		}
	}
	if !d.Key.Keyed() {
		d.Note = "it carries no run condition's key: it was written before run conditions, or on a run with no " +
			"live header, and stands for its own run alone"
		return d, nil
	}
	h, err := runcondition.Read(ctx, db, d.Key.Key(inc.Locus))
	if err != nil {
		return nil, err
	}
	if d.Condition = runConditionOf(h, inc.IncidentId, d.Key, inc.SessionId); d.Condition == nil {
		d.Note = fmt.Sprintf("no live run of %s holds it: the runs it stood for have been deleted, or their "+
			"findings are no longer stored", d.Key.Key(inc.Locus).Describe())
	}
	return d, nil
}

// runConditionPayload is a run incident's condition as the incident screen
// reads it.
func runConditionPayload(c *IncidentRunCondition) map[string]any {
	runs := make([]map[string]any, 0, len(c.Runs))
	for _, r := range c.Runs {
		run := map[string]any{"sessionId": r.SessionId, "startTime": timeOrEmpty(r.StartTime)}
		if r.StoredRows != nil {
			run["storedRows"] = *r.StoredRows
		}
		runs = append(runs, run)
	}
	out := map[string]any{
		"client":        c.Key.Client,
		"process":       c.Key.Process,
		"locus":         c.Key.Locus,
		"firstSeen":     timeOrEmpty(c.FirstSeen),
		"lastSeen":      timeOrEmpty(c.LastSeen),
		"runsHeld":      len(c.Runs),
		"conditionRuns": c.ConditionRuns,
		"ownRunHeld":    c.OwnRunHeld,
		"open":          c.Open,
		"endedBy":       c.EndedBy,
		"runs":          runs,
	}
	if c.Key.Code != "" {
		out["errorCode"] = c.Key.Code
	}
	return out
}

func runCodesPayload(cs []RunCode) []map[string]any {
	out := make([]map[string]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"code": c.Code, "verdict": c.Verdict, "storedRows": c.StoredRows})
	}
	return out
}

// addRunIncident adds a run incident's detail to get_incident's row: its code
// and its run's codes where it has them, and its condition, or why it has
// none. **A condition that holds the incident's own run alone adds nothing**,
// as the writer's basis adds nothing for one: it is what every run incident
// was before run conditions, and the list already counts it as one run.
func addRunIncident(row map[string]any, d *RunIncidentDetail) {
	if d.Key.ErrorCode != "" {
		row["errorCode"] = d.Key.ErrorCode
	}
	if len(d.Codes) > 0 {
		row["codes"] = runCodesPayload(d.Codes)
	}
	switch {
	case d.Condition != nil && !d.Condition.AloneOnItsRun():
		row["runCondition"] = runConditionPayload(d.Condition)
	case d.Condition == nil && d.Note != "":
		row["runConditionNote"] = d.Note
	}
}

// readRunSpans reads the span of each run incident of one list page from its
// run condition: first seen at its position, last seen at its latest member
// still held, and a count of the members still held, which falls as retention
// deletes runs. Each key's history is read once. A run incident with no key
// is left out, and keeps its own session's span.
func readRunSpans(ctx context.Context, db observe.DB, incs []audit.IncidentSummary) (map[string]IncidentSpan, error) {
	out := map[string]IncidentSpan{}
	var ids []string
	for _, inc := range incs {
		if runcondition.IsRunLocus(inc.Locus) {
			ids = append(ids, inc.IncidentId)
		}
	}
	keys, err := readRunIncidentKeys(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	histories := map[runcondition.Key]*runcondition.History{}
	for _, inc := range incs {
		k, ok := keys[inc.IncidentId]
		if !ok {
			continue
		}
		sp := IncidentSpan{ErrorCode: k.ErrorCode}
		if !k.Keyed() {
			// A code and no key: the run had no live header when it was
			// written. Its span is its own session's, which the caller sets.
			out[inc.IncidentId] = sp
			continue
		}
		key := k.Key(inc.Locus)
		h := histories[key]
		if h == nil {
			if h, err = runcondition.Read(ctx, db, key); err != nil {
				return nil, err
			}
			histories[key] = h
		}
		sp.FromRunCondition, sp.First = true, k.Anchor
		if c := runConditionOf(h, inc.IncidentId, k, inc.SessionId); c != nil {
			sp.First, sp.Last, sp.Count = c.FirstSeen, c.LastSeen, len(c.Runs)
		}
		out[inc.IncidentId] = sp
	}
	return out, nil
}

// runSessionMatches answers, for a session with a live run header, which run
// conditions' incidents its present run findings belong to, where it has no
// incident of its own at the finding's key. own holds the session's own run
// incidents by "<locus>/<code>"; a per-record incident written before codes
// ("<locus>/") stands for every code of its session.
func runSessionMatches(ctx context.Context, db observe.DB, session string, own map[string]bool) ([]SessionIncident, error) {
	var client, process string
	err := db.QueryRow(ctx, `SELECT client, process_name FROM jetsapi.pipeline_execution_status
		WHERE session_id = $1 ORDER BY key LIMIT 1`, session).Scan(&client, &process)
	if errors.Is(err, pgx.ErrNoRows) {
		// No run header: the session is no position of any run condition.
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("while reading the run header of session %s: %w", session, err)
	}
	rows, err := db.Query(ctx, `SELECT locus, error_code FROM jetsapi.run_finding
		WHERE session_id = $1 AND verdict = $2 ORDER BY locus, error_code`, session, runcondition.VerdictPresent)
	if err != nil {
		return nil, fmt.Errorf("while reading the run findings of session %s: %w", session, err)
	}
	type at struct{ locus, code string }
	var present []at
	coded := map[string]bool{}
	for rows.Next() {
		var a at
		if err := rows.Scan(&a.locus, &a.code); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning a run finding of session %s: %w", session, err)
		}
		if a.code != "" {
			coded[a.locus] = true
		}
		present = append(present, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the run findings of session %s: %w", session, err)
	}
	var out []SessionIncident
	for _, a := range present {
		// At per_record_failures_reported the codes are the keys, and the row
		// for the locus as a whole stands for them.
		if a.code == "" && coded[a.locus] {
			continue
		}
		if own[a.locus+"/"+a.code] || (a.locus == triage.LocusPerRecordFailuresReported && own[a.locus+"/"]) {
			continue
		}
		h, err := runcondition.Read(ctx, db, runcondition.Key{Client: client, Process: process, Locus: a.locus,
			Code: a.code})
		if err != nil {
			return nil, err
		}
		p := h.Resolve(session)
		m := SessionIncident{SessionId: session, Locus: a.locus, ErrorCode: a.code, Process: process, Client: client,
			Why: p.Why}
		switch p.Kind {
		case runcondition.Join, runcondition.AlreadyRaised:
			m.How, m.IncidentId, m.IncidentSessionId, m.Status = SessionIncidentCondition,
				p.Target.IncidentId, p.Target.SessionId, p.Target.Status
		default:
			m.How, m.AnchorSessionId = SessionIncidentNotRaised, p.Anchor
		}
		out = append(out, m)
	}
	return out, nil
}
