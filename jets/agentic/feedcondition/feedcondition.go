// Package feedcondition says which incident a feed finding belongs to, and
// computes a feed's conditions with their incidents. It reads and never writes.
//
// # A condition and its incident
//
// A condition is a maximal run of consecutive positions of one feed that each
// carry a finding of the feed at one locus (triage's feed_condition.go). It is
// one fault that persists, and a person should read it once: so a condition
// has one incident, on its first delivery, and each later delivery of the
// condition joins it rather than raising another.
//
// # Where a finding goes: Resolve
//
// For a present finding at locus L on session S, of a feed whose condition at
// L holding S is C:
//
//  1. **S already has an incident at L**: already raised; nothing changes.
//  2. **Otherwise E is the incidents at L on C's positions.** The target is
//     the incident on the latest position at or before S, or failing one the
//     earliest after S. If a person has given the target a verdict, and S's
//     earliest anomaly at L was detected after that verdict, S's finding is
//     new evidence: it is written on S, naming the target. Otherwise S joins
//     the target and nothing is written.
//  3. **E is empty**: the anchor is C's first position. If S is the anchor, the
//     incident is written on S. Otherwise the anchor's incident is raised now,
//     classifying the anchor session and writing only its finding at L, and S
//     joins it.
//
// **A join is derived, not stored.** Which incident a delivery belongs to is
// computed from what the database holds (the feed's positions, its anomalies,
// the incidents on those positions and the verdicts on them), so a second pass
// derives the same joins and writes nothing, and nothing has to be retracted.
// An incident never moves: an incident already written on a later position of
// a condition, as a day's burst was written on its last delivery before
// conditions, stays, and the condition's other deliveries join it.
//
// **A person's verdict** is a human incident_event to `reclassified`,
// `suppressed_as_benign` or `verified`, the earliest one counting
// (audit.EarliestHumanVerdicts). A finding detected after it is new evidence and
// opens a new incident, which names the judged one; a finding detected before it
// was part of what was judged, and joins. Neither is suppressed. The rule's one
// hole is a detection run back-dated to before the verdict: its finding joins
// the judged incident.
//
// **Every new incident names the previous one**: after a verdict, the judged
// incident; otherwise the feed's previous condition at the locus and its
// incident, with its verdict or *no verdict*, or, when that condition's
// incident is not written yet (a backfill triaged newest first raises the
// later condition first), where it will be written.
//
// # Read-only, so it can be run against a database it must not change
//
// Read and ReadAll compute conditions from four tables, jetsapi.anomaly,
// input_registry (session_id, file_key, table_name, source_type and key only),
// incident and incident_event, and issue no write. A database where incidents
// were written per delivery before conditions keeps them; ReadAll shows what
// they are as conditions without moving any.
package feedcondition

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

// Incident is an incident at a delivery locus on one of a feed's positions.
type Incident struct {
	IncidentId string
	SessionId  string
	Locus      string
	Status     string
	// Verdict is the incident's first human verdict, or nil when a person has
	// given none.
	Verdict *audit.IncidentTransition
}

// VerdictSentence renders the incident's verdict for a basis.
func (i *Incident) VerdictSentence() string {
	if i == nil || i.Verdict == nil {
		return "no verdict"
	}
	return fmt.Sprintf("a person's verdict, %s by %s at %s", i.Verdict.ToStatus, i.Verdict.Actor,
		i.Verdict.TransitionedAt.UTC().Format(time.RFC3339))
}

// Feed is one feed's conditions, with the incidents on its positions.
type Feed struct {
	Feed string
	// History is what the conditions were computed from.
	History *triage.FeedHistory
	// Conditions are the feed's conditions at every locus its anomalies reach.
	Conditions []triage.Condition
	// Incidents are the incidents at a delivery locus on the feed's positions,
	// by locus and then by session.
	Incidents map[string]map[string]*Incident
	// IncidentsRead reports whether jetsapi.incident was there to read.
	IncidentsRead bool

	order map[string]int // session -> place in the feed's order; unordered sessions absent
}

// Read computes one feed's conditions and reads the incidents on its positions
// and their first human verdicts. It writes nothing.
func Read(ctx context.Context, db observe.DB, feed string) (*Feed, error) {
	var registry, incidents, events bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('jetsapi.input_registry') IS NOT NULL,
	       to_regclass('jetsapi.incident') IS NOT NULL, to_regclass('jetsapi.incident_event') IS NOT NULL`).
		Scan(&registry, &incidents, &events); err != nil {
		return nil, fmt.Errorf("while asking which tables feed %s's conditions can be read from: %w", feed, err)
	}
	h, err := triage.ReadFeedHistory(ctx, db, feed, registry)
	if err != nil {
		return nil, err
	}
	f := FromHistory(h)
	if !incidents {
		return f, nil
	}
	f.IncidentsRead = true
	sessions := make([]string, 0, len(f.order))
	for s := range f.order {
		sessions = append(sessions, s)
	}
	for i := range f.Conditions {
		if f.Conditions[i].Unordered {
			sessions = append(sessions, f.Conditions[i].First().SessionId)
		}
	}
	slices.Sort(sessions)
	rows, err := db.Query(ctx, `SELECT incident_id, incident_session_id, incident_locus, status
		  FROM jetsapi.incident
		 WHERE incident_locus = ANY($1::text[]) AND incident_session_id = ANY($2::text[])
		 ORDER BY incident_id`, triage.DeliveryLoci, sessions)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents on feed %s's deliveries: %w", feed, err)
	}
	var ids []string
	for rows.Next() {
		inc := &Incident{}
		if err := rows.Scan(&inc.IncidentId, &inc.SessionId, &inc.Locus, &inc.Status); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning an incident of feed %s: %w", feed, err)
		}
		f.add(inc)
		ids = append(ids, inc.IncidentId)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the incidents of feed %s: %w", feed, err)
	}
	if !events || len(ids) == 0 {
		return f, nil
	}
	verdicts, err := audit.EarliestHumanVerdicts(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for _, bySession := range f.Incidents {
		for _, inc := range bySession {
			if v, ok := verdicts[inc.IncidentId]; ok {
				inc.Verdict = &v
			}
		}
	}
	return f, nil
}

// FromHistory computes a feed's conditions with no incident read: the
// conditions alone, or a Feed to which a test adds incidents.
func FromHistory(h *triage.FeedHistory) *Feed {
	f := &Feed{Feed: h.Feed, History: h, Conditions: h.Conditions(),
		Incidents: map[string]map[string]*Incident{}, order: map[string]int{}}
	for i, p := range h.Positions() {
		f.order[p.SessionId] = i
	}
	return f
}

// AddIncident records an incident on one of the feed's positions, as Read
// does from the database. It is for a test, or for a caller that holds
// incidents read another way.
func (f *Feed) AddIncident(inc Incident) { f.add(&inc) }

func (f *Feed) add(inc *Incident) {
	if f.Incidents[inc.Locus] == nil {
		f.Incidents[inc.Locus] = map[string]*Incident{}
	}
	f.Incidents[inc.Locus][inc.SessionId] = inc
}

// IncidentsOn returns the incidents on a condition's members, in the
// condition's order.
func (f *Feed) IncidentsOn(c *triage.Condition) []*Incident {
	var out []*Incident
	for _, m := range c.Members {
		if inc := f.Incidents[c.Locus][m.SessionId]; inc != nil {
			out = append(out, inc)
		}
	}
	return out
}

// PreviousCondition returns the latest condition of the feed at the locus
// that ends before the session's position, or nil. A session with no place in
// the feed's order has none before it.
func (f *Feed) PreviousCondition(session, locus string) *triage.Condition {
	at, ok := f.order[session]
	if !ok {
		return nil
	}
	var best *triage.Condition
	bestAt := -1
	for i := range f.Conditions {
		c := &f.Conditions[i]
		if c.Locus != locus || c.Unordered {
			continue
		}
		if p, ok := f.order[c.Last().SessionId]; ok && p < at && p > bestAt {
			best, bestAt = c, p
		}
	}
	return best
}

// IncidentOf returns the latest incident on a condition's members, which is
// the incident its last delivery belongs to, or nil when none is written.
func (f *Feed) IncidentOf(c *triage.Condition) *Incident {
	if c == nil {
		return nil
	}
	incs := f.IncidentsOn(c)
	if len(incs) == 0 {
		return nil
	}
	return incs[len(incs)-1]
}

// Kind is where a finding goes.
type Kind string

const (
	// AlreadyRaised: the session has its own incident at the locus.
	AlreadyRaised Kind = "already_raised"
	// WriteHere: the incident is written on the session.
	WriteHere Kind = "write_here"
	// Join: the finding joins an incident already written on another position
	// of its condition, and nothing is written.
	Join Kind = "join"
	// RaiseAnchor: the condition has no incident yet and the session is not its
	// first position, so the first position's incident is raised and the
	// finding joins it.
	RaiseAnchor Kind = "raise_anchor"
)

// Placement is where one present finding goes, and why.
type Placement struct {
	Kind      Kind
	SessionId string
	Locus     string
	// Feed is the feed whose condition decided it, or "" when no single feed's
	// condition could (Why says why).
	Feed string
	// Condition is the condition at the locus holding the session, or nil.
	Condition *triage.Condition
	// Target is the incident joined (Join, AlreadyRaised), or nil.
	Target *Incident
	// Anchor is the session the condition's incident is to be written on
	// (WriteHere: the session itself; RaiseAnchor: the condition's first).
	Anchor string
	// Judged is, for WriteHere after a person's verdict, the judged incident
	// the finding is new evidence after.
	Judged *Incident
	// PreviousCondition is, for WriteHere and RaiseAnchor, the feed's latest
	// condition at the locus before the anchor's, and Previous its incident,
	// or, after a person's verdict, the judged incident. Either may be nil:
	// no earlier condition, or one whose incident is not written yet.
	PreviousCondition *triage.Condition
	Previous          *Incident
	// Why says how the placement was reached, in a sentence.
	Why string
}

// Resolve places a present finding at a locus on a session of the feed, by the
// rule in the package comment. It reads nothing.
func (f *Feed) Resolve(session, locus string) Placement {
	p := Placement{SessionId: session, Locus: locus, Feed: f.Feed}
	if inc := f.Incidents[locus][session]; inc != nil {
		p.Kind, p.Target = AlreadyRaised, inc
		p.Why = fmt.Sprintf("session %s already has incident %s at this locus", session, inc.IncidentId)
		return p
	}
	c := triage.ConditionAt(f.Conditions, session, locus)
	p.Condition = c
	if c == nil {
		p.Kind, p.Anchor = WriteHere, session
		p.Why = fmt.Sprintf("session %s carries no finding of feed %s at this locus, so no condition of the feed "+
			"holds it", session, f.Feed)
		return p
	}
	at := c.IndexOf(session)
	var target *Incident
	for i := at; i >= 0 && target == nil; i-- {
		target = f.Incidents[locus][c.Members[i].SessionId]
	}
	for i := at + 1; i < len(c.Members) && target == nil; i++ {
		target = f.Incidents[locus][c.Members[i].SessionId]
	}
	switch {
	case target != nil && target.Verdict != nil && c.Members[at].FirstDetected.After(target.Verdict.TransitionedAt):
		p.Kind, p.Anchor, p.Judged, p.Previous = WriteHere, session, target, target
		p.Why = fmt.Sprintf("its condition's incident %s carries %s, and this delivery's finding was detected at %s, "+
			"after it, so it is new evidence and is raised as an incident of its own rather than joined to the "+
			"judged one", target.IncidentId, target.VerdictSentence(),
			c.Members[at].FirstDetected.UTC().Format(time.RFC3339))
	case target != nil:
		p.Kind, p.Target = Join, target
		p.Why = fmt.Sprintf("it joins incident %s, written on session %s for %s", target.IncidentId,
			target.SessionId, c.Describe())
		if target.Verdict != nil {
			p.Why += fmt.Sprintf("; the incident carries %s, and this finding was detected before it, so it is part "+
				"of what was judged", target.VerdictSentence())
		}
	case at == 0:
		p.Kind, p.Anchor = WriteHere, session
		p.PreviousCondition = f.PreviousCondition(session, locus)
		p.Previous = f.IncidentOf(p.PreviousCondition)
		p.Why = "it is the first delivery of " + c.Describe()
	default:
		p.Kind, p.Anchor = RaiseAnchor, c.First().SessionId
		p.PreviousCondition = f.PreviousCondition(p.Anchor, locus)
		p.Previous = f.IncidentOf(p.PreviousCondition)
		p.Why = fmt.Sprintf("no delivery of %s has an incident yet, so it is raised on the condition's first "+
			"delivery, session %s, and this delivery joins it", c.Describe(), p.Anchor)
	}
	return p
}

// Basis is what the writer appends to the triage basis of an incident it
// writes for this placement (WriteHere, or the anchor of RaiseAnchor): the
// condition as known now, why the incident is written where it is, and the
// previous incident at the locus with its verdict.
func (p *Placement) Basis() string {
	var b strings.Builder
	c := p.Condition
	switch {
	case c == nil:
		b.WriteString(". " + strings.ToUpper(p.Why[:1]) + p.Why[1:])
	case p.Judged != nil:
		fmt.Fprintf(&b, ". This delivery is %d of %d in %s. Its incident is written here because %s",
			c.IndexOf(p.SessionId)+1, len(c.Members), c.Describe(), p.Why)
	default:
		fmt.Fprintf(&b, ". This incident is for %s. It is written on the condition's first delivery, and each "+
			"later delivery of the feed with a finding at this locus joins it without an incident of its own, "+
			"until a position of the feed carries none here or a person's verdict on it is followed by a new finding",
			c.Describe())
	}
	pc := p.PreviousCondition
	switch {
	case p.Judged != nil:
		fmt.Fprintf(&b, ". The previous incident at this locus is %s, on session %s, with %s", p.Judged.IncidentId,
			p.Judged.SessionId, p.Judged.VerdictSentence())
	case pc != nil && p.Previous != nil:
		fmt.Fprintf(&b, ". The previous condition of the feed at this locus, %d %s ending at session %s, has incident "+
			"%s, on session %s, with %s", len(pc.Members), plural(len(pc.Members), "delivery", "deliveries"),
			pc.Last().SessionId, p.Previous.IncidentId, p.Previous.SessionId, p.Previous.VerdictSentence())
	case pc != nil:
		fmt.Fprintf(&b, ". The previous condition of the feed at this locus, %d %s from session %s to session %s, has "+
			"no incident yet; when it is raised it is written on its first delivery, session %s",
			len(pc.Members), plural(len(pc.Members), "delivery", "deliveries"), pc.First().SessionId,
			pc.Last().SessionId, pc.First().SessionId)
	case p.Feed != "" && c != nil && !c.Unordered:
		b.WriteString(". No earlier delivery of the feed has a finding at this locus")
	}
	return b.String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ResolveSession places a present finding at a locus on a session, reading the
// feed whose anomaly put it there. A session carrying findings of two feeds at
// the locus is written on itself: an incident keyed on the session and the
// locus cannot sit in two feeds' conditions. It writes nothing.
func ResolveSession(ctx context.Context, db observe.DB, session, locus string) (Placement, *Feed, error) {
	as, err := observe.ReadAnomalies(ctx, db, session)
	if err != nil {
		return Placement{}, nil, err
	}
	var feeds []string
	for _, a := range as {
		if l, ok := triage.LocusOf(a.SubjectType, a.SignalType); ok && l == locus && a.SubjectType == observe.SubjectFeed &&
			!slices.Contains(feeds, a.SubjectRef) {
			feeds = append(feeds, a.SubjectRef)
		}
	}
	slices.Sort(feeds)
	switch len(feeds) {
	case 0:
		return Placement{Kind: WriteHere, SessionId: session, Locus: locus, Anchor: session,
			Why: "no feed anomaly on the session lands at this locus, so no feed's condition holds it"}, nil, nil
	case 1:
	default:
		return Placement{Kind: WriteHere, SessionId: session, Locus: locus, Anchor: session,
			Why: fmt.Sprintf("the session carries findings of %d feeds at this locus (%s), and one incident keyed "+
				"on the session and the locus cannot sit in two feeds' conditions, so it is written here",
				len(feeds), strings.Join(feeds, ", "))}, nil, nil
	}
	f, err := Read(ctx, db, feeds[0])
	if err != nil {
		return Placement{}, nil, err
	}
	return f.Resolve(session, locus), f, nil
}

// ReadAll computes the conditions of the named feeds, or of every feed
// carrying a feed anomaly when feeds is empty, with the incidents on their
// positions and those incidents' first human verdicts. It writes nothing, and
// needs only reads of jetsapi.anomaly, input_registry, incident and
// incident_event, so it can be run against a copy of those four tables.
func ReadAll(ctx context.Context, db observe.DB, feeds []string) ([]*Feed, error) {
	if len(feeds) == 0 {
		rows, err := db.Query(ctx, `SELECT DISTINCT anomaly_subject_ref FROM jetsapi.anomaly
			WHERE anomaly_subject_type = $1 ORDER BY 1`, observe.SubjectFeed)
		if err != nil {
			return nil, fmt.Errorf("while listing the feeds carrying an anomaly: %w", err)
		}
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				rows.Close()
				return nil, fmt.Errorf("while scanning a feed: %w", err)
			}
			feeds = append(feeds, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	out := make([]*Feed, 0, len(feeds))
	for _, t := range feeds {
		f, err := Read(ctx, db, t)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}
