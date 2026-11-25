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
}

// SessionIncidents is what find_session_incidents answers.
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
// carry it, then, at each delivery locus where a session has no incident of
// its own, asks feedcondition where its finding went.
func readSessionIncidents(ctx context.Context, db pgxPool, session string) (*SessionIncidents, error) {
	out := &SessionIncidents{SessionId: session, InputSessions: []string{}, Matches: []SessionIncident{}}
	var runs, anomalies bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.pipeline_execution_status') IS NOT NULL,
	       to_regclass('jetsapi.anomaly') IS NOT NULL`).Scan(&runs, &anomalies); err != nil {
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
	has := map[string]bool{} // session/locus with an incident of its own
	for _, inc := range own {
		has[inc.SessionId+"/"+inc.Locus] = true
		out.Matches = append(out.Matches, SessionIncident{SessionId: inc.SessionId, Locus: inc.Locus,
			How: SessionIncidentOwn, IncidentId: inc.IncidentId, IncidentSessionId: inc.SessionId,
			Status: inc.Status})
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
func readIncidentCondition(ctx context.Context, db observe.DB, inc *audit.Incident) (*IncidentCondition, string, error) {
	p, f, err := feedcondition.ResolveSession(ctx, db, inc.SessionId, inc.Locus)
	if err != nil {
		return nil, "", err
	}
	if f == nil {
		return nil, p.Why, nil
	}
	c := triage.ConditionAt(f.Conditions, inc.SessionId, inc.Locus)
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
		q := f.Resolve(m.SessionId, c.Locus)
		d := ConditionDelivery{SessionId: m.SessionId, FileKey: fileKeys[m.SessionId], Registered: m.Delivery,
			Arrival: m.Arrival, FirstDetected: m.FirstDetected, Detectors: slices.Clone(m.Detectors),
			Placement: string(q.Kind)}
		if q.Target != nil && (q.Kind == feedcondition.Join || q.Kind == feedcondition.AlreadyRaised) {
			d.IncidentId = q.Target.IncidentId
		}
		if d.IncidentId == inc.IncidentId {
			out.StandsFor++
		}
		out.Deliveries = append(out.Deliveries, d)
	}
	// The previous incident: the latest on an earlier delivery of this
	// condition, which is a judged incident this one was raised after, or else
	// the incident of the feed's previous condition at the locus.
	for i := c.IndexOf(inc.SessionId) - 1; i >= 0 && out.Previous == nil; i-- {
		if prev := f.Incidents[c.Locus][c.Members[i].SessionId]; prev != nil {
			out.Previous, out.PreviousInSame = prev, true
		}
	}
	if out.Previous == nil && !c.Unordered {
		out.PreviousCondition = f.PreviousCondition(c.First().SessionId, c.Locus)
		out.Previous = f.IncidentOf(out.PreviousCondition)
	}
	return out, "", nil
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
		matches = append(matches, map[string]any{
			"sessionId":         m.SessionId,
			"locus":             m.Locus,
			"how":               m.How,
			"incidentId":        m.IncidentId,
			"incidentSessionId": m.IncidentSessionId,
			"status":            m.Status,
			"feed":              m.Feed,
			"anchorSessionId":   m.AnchorSessionId,
			"why":               m.Why,
		})
	}
	return &map[string]any{
		"sessionId":     r.SessionId,
		"inputSessions": nonEmptyStrings(r.InputSessions),
		"matches":       matches,
	}, http.StatusOK, nil
}
