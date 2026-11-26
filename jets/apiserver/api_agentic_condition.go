package main

// Which incident a session belongs to, and what a feed incident's condition
// holds: the two reads behind a grouped delivery's link to its incident.
//
// A feed that sends the same faulty delivery week after week is one condition,
// and shadow triage raises one incident for it, on its first delivery. Every
// later delivery of the condition joins that incident and carries none of its
// own, so a person holding one of those deliveries' sessions (from a pipeline
// run on Home, or from the feed's own records) cannot find its incident in the
// incident list by session. `find_session_incidents` answers that, and
// `get_incident` carries the condition so the incident's page lists every
// delivery it stands for.
//
// **Neither read derives the join itself.** Which incident a delivery belongs
// to is the feedcondition package's to say (`Resolve`), from what the database
// holds, and nothing records it. Both reads call that resolver and only shape
// its answer for the screens, so the screens, the summary and the writer
// cannot disagree about where a delivery went. Both write nothing.

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// How a session belongs to an incident.
const (
	// SessionIncidentOwn: the incident is written on the session.
	SessionIncidentOwn = "own"
	// SessionIncidentCondition: the session is a delivery of a feed condition
	// whose incident is written on another of its deliveries.
	SessionIncidentCondition = "condition"
	// SessionIncidentNotRaised: the session carries a feed finding whose
	// condition has no incident yet, because shadow triage has not reached it.
	SessionIncidentNotRaised = "not_raised"
)

// SessionIncident is one incident a session belongs to, or one finding on it
// whose incident is not raised yet.
type SessionIncident struct {
	// SessionId is the session the answer is about: the one asked for, or the
	// input session of a pipeline run asked for.
	SessionId string
	Locus     string
	How       string
	// IncidentId and IncidentSessionId are the incident and the session it is
	// written on; both "" for SessionIncidentNotRaised.
	IncidentId        string
	IncidentSessionId string
	Status            string
	// Feed is the feed whose condition placed the session, "" for an own
	// incident.
	Feed string
	// AnchorSessionId is, for SessionIncidentNotRaised, the session the
	// condition's incident will be written on.
	AnchorSessionId string
	// Why is the resolver's sentence for a condition's placement.
	Why string
	// ErrorCode is the error code at per_record_failures_reported, "" at any
	// other locus and on an incident written before error codes.
	ErrorCode string
	// Process and Client are, for a run condition's placement, the process and
	// client whose runs the condition is of; "" otherwise.
	Process, Client string
}

// SessionIncidents is what find_session_incidents answers.
//
// A run condition places its findings as a feed condition does: a later run of
// a process failing the same way has no incident of its own, and belongs to the
// incident on the condition's first run, which runcondition.Resolve names.
type SessionIncidents struct {
	SessionId string
	// InputSessions are the input sessions of the pipeline runs whose own
	// session is SessionId: a run's incident is on its own session, and the
	// incident of the delivery it read is on the input session.
	InputSessions []string
	Matches       []SessionIncident
}

// readSessionIncidents answers which incidents a session belongs to. It reads
// the incidents on the session and on the input sessions of the runs that
// carry it, then, at each run locus and error code where a session's run has a
// present finding and no incident of its own, asks runcondition where it went,
// and at each delivery locus where a session has no incident of its own, asks
// feedcondition.
func readSessionIncidents(ctx context.Context, db pgxPool, session string) (*SessionIncidents, error) {
	out := &SessionIncidents{SessionId: session, InputSessions: []string{}, Matches: []SessionIncident{}}
	var runs, anomalies, findings bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL,
	       to_regclass('jetsapi.anomaly') IS NOT NULL,
	       to_regclass('jetsapi.run_finding') IS NOT NULL`).Scan(&runs, &anomalies, &findings); err != nil {
		return nil, fmt.Errorf("while asking which tables a session's incidents can be read from: %w", err)
	}
	if runs {
		rows, err := db.Query(ctx, `SELECT DISTINCT input_session_id FROM jetsapi.pipeline_execution_status
			WHERE session_id = $1 AND input_session_id IS NOT NULL AND input_session_id <> $1
			ORDER BY 1`, session)
		if err != nil {
			return nil, fmt.Errorf("while reading the input sessions of the runs of session %s: %w", session, err)
		}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return nil, fmt.Errorf("while scanning an input session: %w", err)
			}
			out.InputSessions = append(out.InputSessions, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading the input sessions of the runs of session %s: %w", session, err)
		}
	}
	sessions := append([]string{session}, out.InputSessions...)

	own, err := audit.IncidentsOnSessions(ctx, db, sessions)
	if err != nil {
		return nil, err
	}
	var ownIds []string
	for _, inc := range own {
		ownIds = append(ownIds, inc.IncidentId)
	}
	keys, err := readRunIncidentKeys(ctx, db, ownIds)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}               // session/locus with an incident of its own
	hasRun := map[string]map[string]bool{} // session -> locus/code of its own run incidents
	for _, inc := range own {
		has[inc.SessionId+"/"+inc.Locus] = true
		code := keys[inc.IncidentId].ErrorCode
		if hasRun[inc.SessionId] == nil {
			hasRun[inc.SessionId] = map[string]bool{}
		}
		hasRun[inc.SessionId][inc.Locus+"/"+code] = true
		out.Matches = append(out.Matches, SessionIncident{SessionId: inc.SessionId, Locus: inc.Locus,
			How: SessionIncidentOwn, IncidentId: inc.IncidentId, IncidentSessionId: inc.SessionId,
			Status: inc.Status, ErrorCode: code})
	}
	if runs && findings {
		for _, s := range sessions {
			ms, err := runSessionMatches(ctx, db, s, hasRun[s])
			if err != nil {
				return nil, err
			}
			out.Matches = append(out.Matches, ms...)
		}
	}
	if !anomalies {
		return out, nil
	}
	for _, s := range sessions {
		for _, locus := range triage.DeliveryLoci {
			if has[s+"/"+locus] {
				continue
			}
			p, f, err := feedcondition.ResolveSession(ctx, db, s, locus)
			if err != nil {
				return nil, err
			}
			if f == nil || p.Condition == nil {
				// No finding of a feed at the locus, or findings of two feeds,
				// whose incident is the session's own once it is written.
				continue
			}
			m := SessionIncident{SessionId: s, Locus: locus, Feed: p.Feed, Why: p.Why}
			switch p.Kind {
			case feedcondition.Join, feedcondition.AlreadyRaised:
				m.How, m.IncidentId, m.IncidentSessionId, m.Status = SessionIncidentCondition,
					p.Target.IncidentId, p.Target.SessionId, p.Target.Status
			default:
				// WriteHere or RaiseAnchor: the finding's incident is not written
				// yet, on this session or on the condition's first delivery.
				m.How, m.AnchorSessionId = SessionIncidentNotRaised, p.Anchor
			}
			out.Matches = append(out.Matches, m)
		}
	}
	return out, nil
}

// ConditionDelivery is one position of a condition, with the incident the
// resolver places it in.
type ConditionDelivery struct {
	SessionId string
	// FileKey is the file the feed delivered in the session, "" for a session
	// that carries a finding of the feed and no registration of it.
	FileKey    string
	Registered bool
	// Arrival is the session id read as a time, zero when it is not one.
	Arrival       time.Time
	FirstDetected time.Time
	Detectors     []string
	// Placement is the resolver's Kind for the delivery's finding, and
	// IncidentId the incident it is in, "" when that incident is not written.
	Placement  string
	IncidentId string
}

// IncidentCondition is a feed incident's condition, read live.
type IncidentCondition struct {
	Feed       string
	Locus      string
	Deliveries []ConditionDelivery
	// StandsFor counts the deliveries placed in this incident.
	StandsFor int
	Open      bool
	// EndedBy and End say what ended the condition; both "" while it is open.
	EndedBy   string
	End       string
	SameDay   bool
	Unordered bool
	Shared    bool
	Described string
	// Previous is the latest incident at the locus on an earlier delivery of
	// the feed, nil when none is written; PreviousInSame says it is on an earlier
	// delivery of this condition (an incident judged before this one was
	// raised), and PreviousCondition is the feed's previous condition at the
	// locus otherwise.
	Previous          *feedcondition.Incident
	PreviousInSame    bool
	PreviousCondition *triage.Condition
}

// readIncidentCondition computes a feed incident's condition, live: its
// deliveries with the incident the resolver places each in, how it ends, and
// the previous incident at the locus with its verdict. It returns nil with a
// reason when no single feed's condition holds the incident's session.
//
// **An incident carrying its feed's key is found by it when its own delivery
// is no position**: that delivery deleted with its session took its anomalies
// with it, so ResolveSession finds no feed on it, and the condition the
// incident still stands for is the one holding it by its position
// (feedcondition.Feed.ConditionOf).
func readIncidentCondition(ctx context.Context, db observe.DB, inc *audit.Incident) (*IncidentCondition, string, error) {
	p, f, err := feedcondition.ResolveSession(ctx, db, inc.SessionId, inc.Locus)
	if err != nil {
		return nil, "", err
	}
	var c *triage.Condition
	if f != nil {
		c = triage.ConditionAt(f.Conditions, inc.SessionId, inc.Locus)
	}
	if c == nil {
		kf, kinc, err := feedcondition.ReadKeyed(ctx, db, inc.IncidentId)
		if err != nil {
			return nil, "", err
		}
		if kc := kf.ConditionOfOrNil(kinc); kc != nil {
			f, c = kf, kc
		}
	}
	if f == nil {
		return nil, p.Why, nil
	}
	if c == nil {
		return nil, fmt.Sprintf("no finding of feed %s at this locus is on session %s now", f.Feed, inc.SessionId), nil
	}
	fileKeys := map[string]string{}
	if f.History != nil {
		for _, d := range f.History.Deliveries {
			if _, ok := fileKeys[d.SessionId]; !ok {
				fileKeys[d.SessionId] = d.FileKey
			}
		}
	}
	out := &IncidentCondition{Feed: c.Feed, Locus: c.Locus, Open: c.Open(), EndedBy: c.EndedBy, End: c.End,
		SameDay: c.SameDay, Unordered: c.Unordered, Shared: c.Shared, Described: c.Describe()}
	for _, m := range c.Members {
		kind, placedIn := placement(f, c, m.SessionId)
		d := ConditionDelivery{SessionId: m.SessionId, FileKey: fileKeys[m.SessionId], Registered: m.Delivery,
			Arrival: m.Arrival, FirstDetected: m.FirstDetected, Detectors: slices.Clone(m.Detectors),
			Placement: string(kind), IncidentId: placedIn}
		if d.IncidentId == inc.IncidentId {
			out.StandsFor++
		}
		out.Deliveries = append(out.Deliveries, d)
	}
	// The previous incident: the latest this condition holds before this one,
	// by position, which is a judged incident this one was raised after, or
	// else the incident of the feed's previous condition at the locus.
	if prev := f.IncidentBefore(c, selfOf(f, inc)); prev != nil {
		out.Previous, out.PreviousInSame = prev, true
	}
	if out.Previous == nil && !c.Unordered {
		out.PreviousCondition = f.PreviousCondition(c.First().SessionId, c.Locus)
		out.Previous = f.IncidentOf(out.PreviousCondition)
	}
	return out, "", nil
}

// selfOf is the incident as the feed holds it, or, when the feed did not read
// it, the incident positioned at its session's arrival, as the feed would.
func selfOf(f *feedcondition.Feed, inc *audit.Incident) *feedcondition.Incident {
	if self := f.IncidentById(inc.IncidentId); self != nil {
		return self
	}
	self := &feedcondition.Incident{IncidentId: inc.IncidentId, SessionId: inc.SessionId, Locus: inc.Locus,
		Status: inc.Status}
	if at, ok := triage.SessionArrival(inc.SessionId); ok {
		self.Position = at
	}
	return self
}

// placement is where the resolver places a member's finding, and the incident
// it is in, "" when that incident is not written. It is the one reading of a
// delivery's incident that the incident's page and the list both use.
func placement(f *feedcondition.Feed, c *triage.Condition, session string) (feedcondition.Kind, string) {
	q := f.Resolve(session, c.Locus)
	if q.Target != nil && (q.Kind == feedcondition.Join || q.Kind == feedcondition.AlreadyRaised) {
		return q.Kind, q.Target.IncidentId
	}
	return q.Kind, ""
}

// IncidentSpan is when an incident was first and last seen, and how many
// deliveries or runs it stands for, as the incident list shows them.
type IncidentSpan struct {
	First, Last time.Time
	Count       int
	// FromCondition reports that the span was read from the incident's feed
	// condition: the deliveries placed in the incident, by arrival.
	FromCondition bool
	// FromRunCondition reports that the span was read from the incident's run
	// condition: first seen at its position, last seen at its latest member
	// still held, Count the members still held, which falls as retention
	// deletes runs. Last is zero when none is held.
	//
	// With neither, the incident stands for its own session alone.
	FromRunCondition bool
	// ErrorCode is the incident's error code, "" when it has none.
	ErrorCode string
	// Job is, on a job_failed incident, the name of the job that failed; ""
	// at any other locus.
	Job string
}

// readIncidentSpans reads the span of each incident of one list page.
//
// **A feed incident's span is its condition's**, read through the resolver
// as get_incident's condition is: Count is the deliveries placed in this
// incident (StandsFor), so a delivery placed in a later incident of the same
// condition is not counted, and First and Last are the earliest and latest of
// their arrivals, the session id read as a time, as the condition orders them.
// A member with no arrival (an id that is not a time) is dated by when it was
// first detected.
//
// **A run incident's span is its run condition's** (readRunSpans): First is
// its stored position, the start time of the run it is written on, kept after
// that run is deleted; Last is the start time of its latest member still held;
// Count is the members still held, which falls as retention deletes runs.
//
// **Any other incident stands for one session**, and so does a run incident
// written before run conditions, which carries no key: Count is 1, and First
// and Last are that session's arrival, or the incident's detection time when
// its session id is not a time. So every kind is dated by when the delivery or
// the run happened, not by when triage got to it.
//
// The feed conditions are read with feedcondition.ResolveSessions: one
// statement for the page's anomalies, and each feed once; the run conditions
// with runcondition.Read, each key once.
func readIncidentSpans(ctx context.Context, db pgxPool, incs []audit.IncidentSummary) (map[string]IncidentSpan, error) {
	out := make(map[string]IncidentSpan, len(incs))
	var places []feedcondition.SessionLocus
	for _, inc := range incs {
		if slices.Contains(triage.DeliveryLoci, inc.Locus) {
			places = append(places, feedcondition.SessionLocus{Session: inc.SessionId, Locus: inc.Locus})
		}
	}
	resolved := map[feedcondition.SessionLocus]feedcondition.Resolved{}
	if len(places) > 0 {
		var anomalies bool
		if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.anomaly') IS NOT NULL`).Scan(&anomalies); err != nil {
			return nil, fmt.Errorf("while asking whether the anomaly table exists: %w", err)
		}
		if anomalies {
			var err error
			if resolved, err = feedcondition.ResolveSessions(ctx, db, places); err != nil {
				return nil, err
			}
		}
	}
	runSpans, err := readRunSpans(ctx, db, incs)
	if err != nil {
		return nil, err
	}
	keyed, err := keyedConditions(ctx, db, incs, resolved)
	if err != nil {
		return nil, err
	}
	for _, inc := range incs {
		own := inc.DetectedAt
		if t, ok := triage.SessionArrival(inc.SessionId); ok {
			own = t
		}
		span := IncidentSpan{First: own, Last: own, Count: 1}
		if rs, ok := runSpans[inc.IncidentId]; ok {
			if rs.FromRunCondition {
				span = rs
			} else {
				span.ErrorCode, span.Job = rs.ErrorCode, rs.Job
			}
		}
		r := resolved[feedcondition.SessionLocus{Session: inc.SessionId, Locus: inc.Locus}]
		f, c := r.Feed, (*triage.Condition)(nil)
		if f != nil {
			c = triage.ConditionAt(f.Conditions, inc.SessionId, inc.Locus)
		}
		if c == nil {
			if k, ok := keyed[inc.IncidentId]; ok {
				f, c = k.feed, k.condition
			}
		}
		if c != nil {
			if cs := conditionSpan(f, c, inc.IncidentId); cs.Count > 0 {
				span = cs
			}
		}
		out[inc.IncidentId] = span
	}
	return out, nil
}

// conditionSpan is the span of the deliveries of a condition placed in an
// incident, by arrival, FromCondition set; Count 0 when none is.
func conditionSpan(f *feedcondition.Feed, c *triage.Condition, incidentId string) IncidentSpan {
	var cs IncidentSpan
	for _, m := range c.Members {
		if _, in := placement(f, c, m.SessionId); in != incidentId {
			continue
		}
		at := m.Arrival
		if at.IsZero() {
			at = m.FirstDetected
		}
		if cs.Count == 0 || at.Before(cs.First) {
			cs.First = at
		}
		if cs.Count == 0 || at.After(cs.Last) {
			cs.Last = at
		}
		cs.Count++
	}
	cs.FromCondition = cs.Count > 0
	return cs
}

// keyedCondition is the feed and condition holding an incident by its key.
type keyedCondition struct {
	feed      *feedcondition.Feed
	condition *triage.Condition
}

// keyedConditions finds, for the page's feed incidents that no condition
// holds by their session (their delivery deleted with its session), the
// condition holding each by its feed key and position, reading each feed once.
func keyedConditions(ctx context.Context, db observe.DB, incs []audit.IncidentSummary,
	resolved map[feedcondition.SessionLocus]feedcondition.Resolved) (map[string]keyedCondition, error) {
	out := map[string]keyedCondition{}
	var ids []string
	for _, inc := range incs {
		if !slices.Contains(triage.DeliveryLoci, inc.Locus) {
			continue
		}
		r := resolved[feedcondition.SessionLocus{Session: inc.SessionId, Locus: inc.Locus}]
		if r.Feed == nil || triage.ConditionAt(r.Feed.Conditions, inc.SessionId, inc.Locus) == nil {
			ids = append(ids, inc.IncidentId)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	var table bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.anomaly') IS NOT NULL`).Scan(&table); err != nil || !table {
		return out, err
	}
	feeds, err := feedcondition.FeedsOf(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	read := map[string]*feedcondition.Feed{}
	for _, id := range ids {
		name, ok := feeds[id]
		if !ok {
			continue
		}
		f, ok := read[name]
		if !ok {
			if f, err = feedcondition.Read(ctx, db, name); err != nil {
				return nil, err
			}
			read[name] = f
		}
		if c := f.ConditionOf(f.IncidentById(id)); c != nil {
			out[id] = keyedCondition{feed: f, condition: c}
		}
	}
	return out, nil
}

// conditionPayload is the condition as the incident screen reads it.
func conditionPayload(c *IncidentCondition) map[string]any {
	deliveries := make([]map[string]any, 0, len(c.Deliveries))
	for _, d := range c.Deliveries {
		deliveries = append(deliveries, map[string]any{
			"sessionId":       d.SessionId,
			"fileKey":         d.FileKey,
			"registered":      d.Registered,
			"arrival":         timeOrEmpty(d.Arrival),
			"firstDetectedAt": timeOrEmpty(d.FirstDetected),
			"detectors":       nonEmptyStrings(d.Detectors),
			"placement":       d.Placement,
			"incidentId":      d.IncidentId,
		})
	}
	out := map[string]any{
		"feed":       c.Feed,
		"locus":      c.Locus,
		"deliveries": deliveries,
		"standsFor":  c.StandsFor,
		"open":       c.Open,
		"endedBy":    c.EndedBy,
		"end":        c.End,
		"sameDay":    c.SameDay,
		"unordered":  c.Unordered,
		"shared":     c.Shared,
		"described":  c.Described,
		"previous":   nil,
	}
	if c.Previous != nil || c.PreviousCondition != nil {
		prev := map[string]any{"inSameCondition": c.PreviousInSame, "incident": nil, "condition": nil}
		if c.Previous != nil {
			var verdict any
			if v := c.Previous.Verdict; v != nil {
				verdict = map[string]any{
					"toStatus":       v.ToStatus,
					"actor":          v.Actor,
					"transitionedAt": timeOrEmpty(v.TransitionedAt),
				}
			}
			prev["incident"] = map[string]any{
				"incidentId": c.Previous.IncidentId,
				"sessionId":  c.Previous.SessionId,
				"status":     c.Previous.Status,
				"verdict":    verdict,
			}
		}
		if pc := c.PreviousCondition; pc != nil {
			prev["condition"] = map[string]any{
				"deliveries":     len(pc.Members),
				"firstSessionId": pc.First().SessionId,
				"lastSessionId":  pc.Last().SessionId,
			}
		}
		out["previous"] = prev
	}
	return out
}

// findSessionIncidents is the find_session_incidents action.
func findSessionIncidents(ctx context.Context, ops agenticOps, a *AgenticAction) (*map[string]any, int, error) {
	if a.SessionId == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("find_session_incidents needs a sessionId")
	}
	r, err := ops.SessionIncidents(ctx, a.SessionId)
	if err != nil {
		return nil, incidentErrorCode(err), err
	}
	matches := make([]map[string]any, 0, len(r.Matches))
	for _, m := range r.Matches {
		match := map[string]any{
			"sessionId":         m.SessionId,
			"locus":             m.Locus,
			"how":               m.How,
			"incidentId":        m.IncidentId,
			"incidentSessionId": m.IncidentSessionId,
			"status":            m.Status,
			"feed":              m.Feed,
			"anchorSessionId":   m.AnchorSessionId,
			"why":               m.Why,
		}
		// Only where set, so a feed's answer is as it was.
		if m.ErrorCode != "" {
			match["errorCode"] = m.ErrorCode
		}
		if m.Process != "" {
			match["process"], match["client"] = m.Process, m.Client
		}
		matches = append(matches, match)
	}
	return &map[string]any{
		"sessionId":     r.SessionId,
		"inputSessions": nonEmptyStrings(r.InputSessions),
		"matches":       matches,
	}, http.StatusOK, nil
}
