// Package runcondition says which incident a run finding belongs to, and
// computes a process's run conditions with their incidents. It reads and never
// writes, except for the rows it describes and the shadow writer stores
// (jetsapi.run_finding).
//
// # A run condition and its incident
//
// A process that fails the same way run after run is one fault that persists,
// and a person should read it once. So the run findings of one process for one
// client at one run locus are grouped into conditions, and a condition has one
// incident, on its first run; each later run of the condition joins it rather
// than raising another.
//
// **The key** is (client, process name, locus), and at
// per_record_failures_reported also the error code: one run can report several
// codes, each with its own threshold and its own verdict, so each code has
// conditions of its own.
//
// **A job's failures are run conditions too**, at locus job_failed with the
// job's name as the process: a job writes a row only when it fails, so its
// condition holds every failure from the first, and ends only at a person's
// verdict (IsRunLocus).
//
// **The positions** are the process's live runs for that client, the rows of
// jetsapi.pipeline_execution_status, in start_time then session order. **A
// deleted run is no position**: it neither ends a condition nor joins one, and
// the archive of deleted runs is not read.
//
// **A run's verdict at the key** is read from jetsapi.run_finding, which the
// shadow writer fills for every run it classifies (see RowsOf for what a row
// says). A condition is a maximal stretch of runs whose verdict is present,
// skipping the runs whose verdict is quiet (rows below the code's threshold, or
// a debug code), not evaluable, or that triage never classified. **A condition
// ends only at a run whose verdict at the key is absent.** A skipped run says
// nothing about the key, so it neither joins nor ends a condition.
//
// # Where a finding goes: Resolve
//
// A run incident carries its key and its position: the client, the process
// name and the start time of the run it is written on (jetsapi.incident's
// incident_client, incident_process_name and incident_anchor_start_time, with
// incident_locus and incident_error_code). **A condition's incidents are found
// by key and position, never by session**: the incidents at its key whose
// position falls after the absent run before the condition and before the absent
// run that ends it. So an incident whose first run was deleted is still its
// condition's incident.
//
// For a present finding at key K on run S, in condition C:
//
//  1. **S already has an incident at K**: already raised; nothing changes.
//  2. **Otherwise E is C's incidents.** The target is the latest of them at or
//     before S, or failing one the earliest after S. If a person has given the
//     target a verdict and S's finding was stored after it, S's finding is new
//     evidence: it is written on S, naming the target. Otherwise S joins the
//     target and nothing is written.
//  3. **E is empty**: the anchor is C's first run. If S is the anchor, the
//     incident is written on S. Otherwise the anchor's incident is raised now,
//     and S joins it.
//
// **A join is derived, not stored**, as for a feed condition (the
// feedcondition package): which incident a run belongs to is computed from what
// the database holds, so a second pass derives the same joins and writes
// nothing. An incident never moves, and the writer never retracts one: a run
// incident written before run conditions carries no key, belongs to no
// condition, and stays as written.
package runcondition

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The verdicts jetsapi.run_finding stores, its run_finding_verdict_ck.
const (
	VerdictPresent      = "present"
	VerdictQuiet        = "quiet"
	VerdictAbsent       = "absent"
	VerdictNotEvaluable = "not_evaluable"
)

// Verdicts is the vocabulary of run_finding.verdict, in the DDL's order.
var Verdicts = []string{VerdictPresent, VerdictQuiet, VerdictAbsent, VerdictNotEvaluable}

// Rank orders the verdicts by how much they say about a run: not evaluable,
// then absent, then quiet, then present, and -1 for a run never classified. A
// stored verdict is replaced only by one of a higher rank, so a later
// classification that can see less never turns a member into a gap.
func Rank(v string) int {
	switch v {
	case VerdictNotEvaluable:
		return 0
	case VerdictAbsent:
		return 1
	case VerdictQuiet:
		return 2
	case VerdictPresent:
		return 3
	}
	return -1
}

// IsRunLocus reports whether a locus is one a run condition can be keyed on:
// the nine positions in a run's execution record, and job_failed, a job's
// failure, rather than a position in a feed's deliveries.
//
// **A job's failures are grouped as a run's are**, by client, process name and
// locus, so a job that fails on five days is one condition of five runs with
// one incident. A job writes a row only when it fails, so its runs all read
// present at job_failed and nothing reads absent: its condition ends only at a
// person's verdict, after which a later failure raises a new incident naming
// the judged one, as for any run condition.
func IsRunLocus(locus string) bool {
	return slices.Contains(triage.Loci, locus) && !slices.Contains(triage.DeliveryLoci, locus)
}

// Row is one row of jetsapi.run_finding.
type Row struct {
	SessionId string
	Locus     string
	// ErrorCode is "" at a locus with no code, and, at
	// per_record_failures_reported, on the row that stands for the locus as a
	// whole.
	ErrorCode string
	Verdict   string
	// StoredRows is a code's stored rows in the run, nil on a row with no code.
	StoredRows    *int64
	ClassifierRef string
	ClassifiedAt  time.Time
}

// RowsOf turns a report's findings at the run loci into the rows the shadow
// writer stores, classified at the given time. The delivery loci have none.
//
//   - At a locus with no code: one row, error code "", the finding's verdict.
//   - At per_record_failures_reported: the row with error code "" carries the
//     locus's verdict as a whole, and each code of the run's rows has a row of
//     its own, present when it reports and quiet when it does not (below its
//     threshold, or a debug code), with its stored rows. **A code with no row on
//     a run whose "" row is present or absent is absent for that run**; a run
//     whose "" row is not evaluable is not evaluable for every code.
//
// **Only the loci that apply to the session are stored**: on a job's row
// (Report.Job) the job_failed row alone, and on any other session the nine run
// loci and no job_failed row, which would say only that a pipeline run is not a
// job. A run with no row at a locus is one triage never classified there, so
// it neither joins nor ends a condition, which is what either row would have
// said.
func RowsOf(r *triage.Report, at time.Time) []Row {
	var out []Row
	for i := range r.Findings {
		f := &r.Findings[i]
		if !IsRunLocus(f.Locus) || r.Job != (f.Locus == triage.LocusJobFailed) {
			continue
		}
		out = append(out, Row{SessionId: r.SessionId, Locus: f.Locus, Verdict: string(f.Verdict),
			ClassifierRef: f.ClassifierRef, ClassifiedAt: at})
		if f.Locus != triage.LocusPerRecordFailuresReported || f.Verdict == triage.NotEvaluable {
			continue
		}
		for _, c := range f.Codes {
			v := VerdictQuiet
			if c.Reports() {
				v = VerdictPresent
			}
			n := c.StoredRows
			out = append(out, Row{SessionId: r.SessionId, Locus: f.Locus, ErrorCode: c.Code, Verdict: v,
				StoredRows: &n, ClassifierRef: f.ClassifierRef, ClassifiedAt: at})
		}
	}
	return out
}

// VerdictAt is the verdict at a locus and code that a run's rows say, by the
// rule RowsOf states, and the row it was read from; "" and nil when the rows
// say nothing (the run was not classified at the locus).
func VerdictAt(rows []Row, locus, code string) (string, *Row) {
	var base, own *Row
	for i := range rows {
		if rows[i].Locus != locus {
			continue
		}
		switch rows[i].ErrorCode {
		case "":
			base = &rows[i]
		case code:
			own = &rows[i]
		}
	}
	return verdictOf(code, own, base)
}

func verdictOf(code string, own, base *Row) (string, *Row) {
	switch {
	case code == "":
		if base == nil {
			return "", nil
		}
		return base.Verdict, base
	case own != nil:
		return own.Verdict, own
	case base == nil:
		return "", nil
	case base.Verdict == VerdictNotEvaluable:
		return VerdictNotEvaluable, base
	}
	return VerdictAbsent, base
}

// Key is what a run condition is keyed on.
type Key struct {
	Client  string
	Process string
	Locus   string
	// Code is the error code at per_record_failures_reported, "" elsewhere.
	Code string
}

// Describe renders the key for a basis.
func (k Key) Describe() string {
	s := fmt.Sprintf("process %s for client %s at locus %s", k.Process, k.Client, k.Locus)
	if k.Code != "" {
		s += fmt.Sprintf(", error code %q", k.Code)
	}
	return s
}

// Run is one position: a live run of the key's process for its client.
type Run struct {
	SessionId string
	StartTime time.Time
	// Verdict is the run's verdict at the key, "" when triage never classified
	// it there.
	Verdict string
	// ClassifiedAt is when the verdict was stored: for a present verdict, when
	// the finding was first seen as present.
	ClassifiedAt time.Time
	// StoredRows is, for a code, the code's stored rows in the run.
	StoredRows *int64
}

func before(at time.Time, session string, than time.Time, thanSession string) bool {
	if !at.Equal(than) {
		return at.Before(than)
	}
	return session < thanSession
}

// Incident is a run incident at the key.
type Incident struct {
	IncidentId string
	SessionId  string
	Status     string
	// Position is the start time of the run the incident is written on, kept
	// on the incident after that run is deleted.
	Position time.Time
	// Verdict is the incident's first human verdict, or nil.
	Verdict *audit.IncidentTransition
}

func (i *Incident) before(at time.Time, session string) bool {
	return before(i.Position, i.SessionId, at, session)
}

// VerdictSentence renders the incident's verdict for a basis.
func (i *Incident) VerdictSentence() string {
	if i == nil || i.Verdict == nil {
		return "no verdict"
	}
	return fmt.Sprintf("a person's verdict, %s by %s at %s", i.Verdict.ToStatus, i.Verdict.Actor,
		i.Verdict.TransitionedAt.UTC().Format(time.RFC3339))
}

// Condition is a maximal stretch of a process's live runs whose verdict at the
// key is present, skipping the runs that say nothing about it.
type Condition struct {
	Key Key
	// Members are the runs whose verdict is present, in order.
	Members []Run
	// Skipped counts the runs between the first member and the last whose
	// verdict is quiet, not evaluable, or that were never classified.
	Skipped int
	// After is the run with an absent verdict before the first member, nil when
	// none is; EndedBy the one after the last member, nil when the condition is
	// open.
	After, EndedBy *Run
}

// Open reports whether no run with an absent verdict follows the condition.
func (c *Condition) Open() bool { return c != nil && c.EndedBy == nil }

// First is the condition's first member.
func (c *Condition) First() Run { return c.Members[0] }

// Last is the condition's last member.
func (c *Condition) Last() Run { return c.Members[len(c.Members)-1] }

// IndexOf returns the position of a session among the members, or -1.
func (c *Condition) IndexOf(session string) int {
	if c == nil {
		return -1
	}
	for i := range c.Members {
		if c.Members[i].SessionId == session {
			return i
		}
	}
	return -1
}

// Sessions returns the members' sessions, in order.
func (c *Condition) Sessions() []string {
	out := make([]string, len(c.Members))
	for i := range c.Members {
		out[i] = c.Members[i].SessionId
	}
	return out
}

// Holds reports whether an incident's position falls in the condition: after
// the absent run before it, and before the absent run that ends it.
func (c *Condition) Holds(inc *Incident) bool {
	if c.After != nil && !before(c.After.StartTime, c.After.SessionId, inc.Position, inc.SessionId) {
		return false
	}
	return c.EndedBy == nil || inc.before(c.EndedBy.StartTime, c.EndedBy.SessionId)
}

const timeFmt = "2006-01-02 15:04 UTC"

// Describe renders the condition as it stands, for an incident's basis.
func (c *Condition) Describe() string {
	var b strings.Builder
	n := len(c.Members)
	fmt.Fprintf(&b, "a condition of %s: %d %s", c.Key.Describe(), n, plural(n, "run", "runs"))
	if n == 1 {
		fmt.Fprintf(&b, ", started %s", c.First().StartTime.UTC().Format(timeFmt))
	} else {
		fmt.Fprintf(&b, ", started from %s to %s", c.First().StartTime.UTC().Format(timeFmt),
			c.Last().StartTime.UTC().Format(timeFmt))
	}
	fmt.Fprintf(&b, ", %s %s", plural(n, "session", "sessions"), strings.Join(c.Sessions(), ", "))
	if c.Skipped > 0 {
		fmt.Fprintf(&b, " (%d %s between them %s nothing here: below the threshold, not evaluable, or not "+
			"classified)", c.Skipped, plural(c.Skipped, "run", "runs"), plural(c.Skipped, "says", "say"))
	}
	if c.EndedBy == nil {
		b.WriteString("; it is open: no later run of the process has an absent verdict here")
	} else {
		fmt.Fprintf(&b, "; it ended at session %s, a later run of the process whose finding here is absent",
			c.EndedBy.SessionId)
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// History is what a key's conditions are computed from: the process's live
// runs with their verdicts at the key, and the incidents at the key.
type History struct {
	Key Key
	// Runs are the positions in order.
	Runs []Run
	// Incidents are the incidents at the key, by position.
	Incidents []*Incident
	// FindingsRead reports whether jetsapi.run_finding was there to read; when
	// it was not, no run has a verdict and there is no condition.
	FindingsRead bool
}

// Overlay sets a run's verdict at the key as a classification not yet stored
// would, by the stored rows' rule: only a verdict that says more replaces the
// one held. It is how a writer places a finding before (or, in a dry run,
// without) storing it. A session that is no position is ignored.
func (h *History) Overlay(session, verdict string, at time.Time) {
	for i := range h.Runs {
		r := &h.Runs[i]
		if r.SessionId == session && Rank(verdict) > Rank(r.Verdict) {
			r.Verdict, r.ClassifiedAt = verdict, at
		}
	}
}

// RunOf returns the position of a session, or nil.
func (h *History) RunOf(session string) *Run {
	for i := range h.Runs {
		if h.Runs[i].SessionId == session {
			return &h.Runs[i]
		}
	}
	return nil
}

// Conditions computes the key's conditions, in order.
func (h *History) Conditions() []Condition {
	var out []Condition
	var cur *Condition
	var lastAbsent *Run
	pending := 0
	for i := range h.Runs {
		r := &h.Runs[i]
		switch r.Verdict {
		case VerdictPresent:
			if cur == nil {
				cur = &Condition{Key: h.Key, After: lastAbsent}
			} else {
				cur.Skipped += pending
			}
			pending = 0
			cur.Members = append(cur.Members, *r)
		case VerdictAbsent:
			if cur != nil {
				cur.EndedBy = r
				out = append(out, *cur)
				cur = nil
			}
			lastAbsent, pending = r, 0
		default:
			if cur != nil {
				pending++
			}
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// ConditionAt returns the condition holding the session, or nil.
func (h *History) ConditionAt(session string) *Condition {
	cs := h.Conditions()
	for i := range cs {
		if cs[i].IndexOf(session) >= 0 {
			return &cs[i]
		}
	}
	return nil
}

// IncidentsOf returns the condition's incidents, by position.
func (h *History) IncidentsOf(c *Condition) []*Incident {
	var out []*Incident
	for _, inc := range h.Incidents {
		if c.Holds(inc) {
			out = append(out, inc)
		}
	}
	return out
}

// PreviousIncident returns the latest incident at the key positioned before
// the condition, at or before the absent run that precedes it, or nil.
func (h *History) PreviousIncident(c *Condition) *Incident {
	if c.After == nil {
		return nil
	}
	var best *Incident
	for _, inc := range h.Incidents {
		if inc.before(c.After.StartTime, c.After.SessionId) || inc.SessionId == c.After.SessionId {
			best = inc
		}
	}
	return best
}

// Kind is where a finding goes.
type Kind string

const (
	// AlreadyRaised: the session has its own incident at the key.
	AlreadyRaised Kind = "already_raised"
	// WriteHere: the incident is written on the session.
	WriteHere Kind = "write_here"
	// Join: the finding joins an incident of its condition, and nothing is
	// written.
	Join Kind = "join"
	// RaiseAnchor: the condition has no incident yet and the session is not its
	// first run, so the first run's incident is raised and the finding joins it.
	RaiseAnchor Kind = "raise_anchor"
)

// Placement is where one present run finding goes, and why.
type Placement struct {
	Kind      Kind
	SessionId string
	Key       Key
	// Condition is the condition holding the session, or nil when the session
	// is no position of the key's process.
	Condition *Condition
	// Target is the incident joined (Join, AlreadyRaised), or nil.
	Target *Incident
	// Anchor is the session the condition's incident is to be written on
	// (WriteHere: the session itself; RaiseAnchor: the condition's first run).
	Anchor string
	// Judged is, for WriteHere after a person's verdict, the judged incident the
	// finding is new evidence after.
	Judged *Incident
	// Previous is, for WriteHere and RaiseAnchor, the latest incident at the key
	// before the condition, or the judged incident; nil when there is none.
	Previous *Incident
	// Why says how the placement was reached, in a sentence.
	Why string
}

// Resolve places a present finding at the key on a session, by the rule in the
// package comment. It reads nothing: a caller placing a finding not yet stored
// overlays it first (Overlay).
func (h *History) Resolve(session string) Placement {
	p := Placement{SessionId: session, Key: h.Key}
	run := h.RunOf(session)
	if run == nil {
		p.Kind, p.Anchor = WriteHere, session
		p.Why = fmt.Sprintf("session %s is no live run of process %s for client %s, so no condition holds it "+
			"and the incident is written here", session, h.Key.Process, h.Key.Client)
		return p
	}
	c := h.ConditionAt(session)
	p.Condition = c
	if c == nil {
		p.Kind, p.Anchor = WriteHere, session
		p.Why = fmt.Sprintf("session %s's stored verdict here is not present, so no condition holds it and the "+
			"incident is written here", session)
		return p
	}
	incs := h.IncidentsOf(c)
	for _, inc := range incs {
		if inc.SessionId == session {
			p.Kind, p.Target = AlreadyRaised, inc
			p.Why = fmt.Sprintf("session %s already has incident %s at this key", session, inc.IncidentId)
			return p
		}
	}
	// The latest incident at or before the run (one on the run itself is
	// already raised, above), else the earliest after it.
	var target *Incident
	for _, inc := range incs {
		if inc.before(run.StartTime, run.SessionId) {
			target = inc
		}
	}
	if target == nil {
		for _, inc := range incs {
			if !inc.before(run.StartTime, run.SessionId) {
				target = inc
				break
			}
		}
	}
	at := c.IndexOf(session)
	member := c.Members[at]
	switch {
	case target != nil && target.Verdict != nil && member.ClassifiedAt.After(target.Verdict.TransitionedAt):
		p.Kind, p.Anchor, p.Judged, p.Previous = WriteHere, session, target, target
		p.Why = fmt.Sprintf("its condition's incident %s carries %s, and this run's finding was found at %s, "+
			"after it, so it is new evidence and is raised as an incident of its own rather than joined to the "+
			"judged one", target.IncidentId, target.VerdictSentence(), member.ClassifiedAt.UTC().Format(time.RFC3339))
	case target != nil:
		p.Kind, p.Target = Join, target
		p.Why = fmt.Sprintf("it joins incident %s, written on session %s for %s", target.IncidentId,
			target.SessionId, c.Describe())
		if target.Verdict != nil {
			p.Why += fmt.Sprintf("; the incident carries %s, and this finding was found before it, so it is part "+
				"of what was judged", target.VerdictSentence())
		}
	case at == 0:
		p.Kind, p.Anchor = WriteHere, session
		p.Previous = h.PreviousIncident(c)
		p.Why = "it is the first run of " + c.Describe()
	default:
		p.Kind, p.Anchor = RaiseAnchor, c.First().SessionId
		p.Previous = h.PreviousIncident(c)
		p.Why = fmt.Sprintf("no run of %s has an incident yet, so it is raised on the condition's first run, "+
			"session %s, and this run joins it", c.Describe(), p.Anchor)
	}
	return p
}

// Basis is what the writer appends to the triage basis of an incident it
// writes for this placement (WriteHere, or the anchor of RaiseAnchor): the
// condition as known now, why the incident is written where it is, and the
// previous incident at the key with its verdict.
//
// **The ordinary case appends nothing**: a run that is the only run of its
// condition so far, with no earlier incident at the key, is what every run
// incident was before run conditions, and a sentence saying so would tell a
// reader nothing the incident does not. The sentence is written where it says
// something: a condition already longer than the run, an earlier incident, or
// a person's verdict.
func (p *Placement) Basis() string {
	var b strings.Builder
	c := p.Condition
	if p.Kind == WriteHere && c != nil && len(c.Members) == 1 && p.Judged == nil && p.Previous == nil {
		return ""
	}
	switch {
	case c == nil:
		b.WriteString(". " + strings.ToUpper(p.Why[:1]) + p.Why[1:])
	case p.Judged != nil:
		fmt.Fprintf(&b, ". This run is %d of %d in %s. Its incident is written here because %s",
			c.IndexOf(p.SessionId)+1, len(c.Members), c.Describe(), p.Why)
	default:
		fmt.Fprintf(&b, ". This incident is for %s. It is written on the condition's first run, and each later "+
			"run of the process with this finding joins it without an incident of its own, until a run's finding "+
			"here is absent or a person's verdict on it is followed by a new finding", c.Describe())
	}
	switch {
	case p.Judged != nil:
		fmt.Fprintf(&b, ". The previous incident at this key is %s, on session %s, with %s", p.Judged.IncidentId,
			p.Judged.SessionId, p.Judged.VerdictSentence())
	case p.Previous != nil:
		fmt.Fprintf(&b, ". The previous incident at this key is %s, on session %s, with %s", p.Previous.IncidentId,
			p.Previous.SessionId, p.Previous.VerdictSentence())
	case c != nil:
		b.WriteString(". No earlier run of the process has an incident at this key")
	}
	return b.String()
}

// Members returns the condition holding an incident and the members that
// resolve to it: those whose target (Resolve's step 2) is the incident, with
// the incident's own run when it is live. It is what a list showing one row per
// incident reads as the incident's span: first seen at the incident's
// position, last seen at its latest member, a count of the members still held.
// Nil when the incident is not at the key or no condition holds it.
func (h *History) Members(incidentId string) (*Condition, []Run) {
	var inc *Incident
	for _, i := range h.Incidents {
		if i.IncidentId == incidentId {
			inc = i
		}
	}
	if inc == nil {
		return nil, nil
	}
	cs := h.Conditions()
	for i := range cs {
		c := &cs[i]
		if !c.Holds(inc) {
			continue
		}
		var out []Run
		for _, m := range c.Members {
			p := h.Resolve(m.SessionId)
			switch {
			case p.Kind == AlreadyRaised && p.Target == inc,
				p.Kind == Join && p.Target == inc:
				out = append(out, m)
			}
		}
		return c, out
	}
	return nil, nil
}

// readRunsSQL reads a process's live runs for a client, in order, with the
// run_finding rows at the locus that say its verdict at the key: the code's
// own row and the row for the locus as a whole.
const readRunsSQL = `SELECT s.session_id, s.start_time,
       own.verdict, own.classified_at, own.stored_rows,
       base.verdict, base.classified_at
  FROM jetsapi.pipeline_execution_status s
  LEFT JOIN jetsapi.run_finding own
         ON own.session_id = s.session_id AND own.locus = $3 AND own.error_code = $4 AND $4 <> ''
  LEFT JOIN jetsapi.run_finding base
         ON base.session_id = s.session_id AND base.locus = $3 AND base.error_code = ''
 WHERE s.client = $1 AND s.process_name = $2
 ORDER BY s.start_time, s.session_id`

// readRunsNoFindingsSQL is readRunsSQL where jetsapi.run_finding is absent.
const readRunsNoFindingsSQL = `SELECT s.session_id, s.start_time
  FROM jetsapi.pipeline_execution_status s
 WHERE s.client = $1 AND s.process_name = $2
 ORDER BY s.start_time, s.session_id`

// readIncidentsSQL reads the run incidents at a key, by position.
const readIncidentsSQL = `SELECT incident_id, incident_session_id, status, incident_anchor_start_time
  FROM jetsapi.incident
 WHERE incident_client = $1 AND incident_process_name = $2 AND incident_locus = $3
   AND coalesce(incident_error_code, '') = $4 AND incident_anchor_start_time IS NOT NULL
 ORDER BY incident_anchor_start_time, incident_session_id, incident_id`

const tablesSQL = `SELECT to_regclass('jetsapi.run_finding') IS NOT NULL,
       to_regclass('jetsapi.incident') IS NOT NULL,
       to_regclass('jetsapi.incident_event') IS NOT NULL,
       EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'jetsapi' AND table_name = 'incident' AND column_name = 'incident_client')`

// Read reads what a key's conditions are computed from: the process's live
// runs with their verdicts at the key, the incidents at the key, and their
// first human verdicts. It writes nothing.
func Read(ctx context.Context, db observe.DB, key Key) (*History, error) {
	var findings, incidents, events, keyed bool
	if err := db.QueryRow(ctx, tablesSQL).Scan(&findings, &incidents, &events, &keyed); err != nil {
		return nil, fmt.Errorf("while asking which tables %s's conditions can be read from: %w", key.Describe(), err)
	}
	h := &History{Key: key, FindingsRead: findings}
	stmt, args := readRunsNoFindingsSQL, []any{key.Client, key.Process}
	if findings {
		stmt, args = readRunsSQL, []any{key.Client, key.Process, key.Locus, key.Code}
	}
	rows, err := db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("while reading the runs of %s: %w", key.Describe(), err)
	}
	seen := map[string]bool{}
	for rows.Next() {
		var r Run
		var ownVerdict, baseVerdict *string
		var ownAt, baseAt *time.Time
		var ownRows *int64
		dest := []any{&r.SessionId, &r.StartTime}
		if findings {
			dest = append(dest, &ownVerdict, &ownAt, &ownRows, &baseVerdict, &baseAt)
		}
		if err := rows.Scan(dest...); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning a run of %s: %w", key.Describe(), err)
		}
		if seen[r.SessionId] {
			continue // a session with two headers is one position, its first
		}
		seen[r.SessionId] = true
		var own, base *Row
		if ownVerdict != nil {
			own = &Row{Verdict: *ownVerdict, ClassifiedAt: *ownAt, StoredRows: ownRows}
		}
		if baseVerdict != nil {
			base = &Row{Verdict: *baseVerdict, ClassifiedAt: *baseAt}
		}
		if v, from := verdictOf(key.Code, own, base); from != nil {
			r.Verdict, r.ClassifiedAt, r.StoredRows = v, from.ClassifiedAt, from.StoredRows
		}
		h.Runs = append(h.Runs, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the runs of %s: %w", key.Describe(), err)
	}
	if !incidents || !keyed {
		return h, nil
	}
	rows, err = db.Query(ctx, readIncidentsSQL, key.Client, key.Process, key.Locus, key.Code)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents of %s: %w", key.Describe(), err)
	}
	var ids []string
	for rows.Next() {
		inc := &Incident{}
		if err := rows.Scan(&inc.IncidentId, &inc.SessionId, &inc.Status, &inc.Position); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning an incident of %s: %w", key.Describe(), err)
		}
		h.Incidents = append(h.Incidents, inc)
		ids = append(ids, inc.IncidentId)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the incidents of %s: %w", key.Describe(), err)
	}
	if !events || len(ids) == 0 {
		return h, nil
	}
	verdicts, err := audit.EarliestHumanVerdicts(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for _, inc := range h.Incidents {
		if v, ok := verdicts[inc.IncidentId]; ok {
			inc.Verdict = &v
		}
	}
	return h, nil
}
