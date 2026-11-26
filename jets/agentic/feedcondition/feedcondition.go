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
// # A condition's incidents: by key and position
//
// **A feed incident carries its key and its position**: the feed
// (incident_feed) and the arrival of the delivery it is written on, its
// session id read as a time (incident_anchor_start_time), the value a feed's
// positions are ordered by. With the locus that is the condition's key. So a
// condition's incidents are found by them, not by their sessions: the
// incidents at the feed and the locus positioned after the feed's position
// before the condition's first member, and before the position that ends it
// (Feed.IncidentsOn). **A delivery deleted with its session is no position**:
// its registration and its anomalies are gone, so it neither ends a condition
// nor joins one, and an incident written on it is still found by its position,
// so the condition's later deliveries join it rather than raising a second.
//
// Where nothing lies between a condition and the next one at the locus (the
// purged-era day boundary), a deleted delivery between them is the later
// condition's. Next to a session carrying another feed's finding at the locus
// too, it is the neighbouring condition's on its side of that session, since a
// shared session's condition of one holds only its own incident, which is no
// one feed's. **An incident written before feed incidents carried a key** is
// found by its session, as every feed incident was: it is its condition's
// while its session is a position, and no condition's once it is not.
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

// Incident is an incident at a delivery locus of a feed: on one of its
// positions, or carrying the feed's key and a position that is no longer one.
type Incident struct {
	IncidentId string
	SessionId  string
	Locus      string
	Status     string
	// Feed is the feed the incident carries as its key, "" for an incident
	// written before feed incidents carried one.
	Feed string
	// Position is the arrival of the delivery the incident is written on, kept
	// on the incident after that delivery is deleted: the stored position for
	// an incident with a key, the session id read as a time for one without,
	// and zero for a session id that is not a time.
	Position time.Time
	// Verdict is the incident's first human verdict, or nil when a person has
	// given none.
	Verdict *audit.IncidentTransition
}

// Keyed reports whether the incident carries a feed condition's key and a
// position, so a condition finds it after its session is deleted.
func (i *Incident) Keyed() bool { return i != nil && i.Feed != "" && !i.Position.IsZero() }

// before orders an incident against a position as a feed orders its positions:
// by arrival, then session id.
func (i *Incident) before(at time.Time, session string) bool {
	return before(i.Position, i.SessionId, at, session)
}

func before(at time.Time, session string, than time.Time, thanSession string) bool {
	if !at.Equal(than) {
		return at.Before(than)
	}
	return session < thanSession
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
	// Incidents are the incidents at a delivery locus read for the feed, by
	// locus and then by their own session: those on its positions, and those
	// carrying its key, whose session may no longer be a position.
	Incidents map[string]map[string]*Incident
	// IncidentsRead reports whether jetsapi.incident was there to read.
	IncidentsRead bool

	order     map[string]int // session -> place in the feed's order; unordered sessions absent
	positions []triage.FeedPosition
}

// readIncidentsSQL reads the incidents at the delivery loci read for a feed:
// those on its positions ($2), and those carrying its key ($3), wherever they
// sit.
const readIncidentsSQL = `SELECT incident_id, incident_session_id, incident_locus, status,
       coalesce(incident_feed, ''), incident_anchor_start_time
  FROM jetsapi.incident
 WHERE incident_locus = ANY($1::text[]) AND incident_session_id = ANY($2::text[])
UNION
SELECT incident_id, incident_session_id, incident_locus, status,
       incident_feed, incident_anchor_start_time
  FROM jetsapi.incident
 WHERE incident_feed = $3 AND incident_locus = ANY($1::text[]) AND incident_anchor_start_time IS NOT NULL
 ORDER BY 1`

// readIncidentsNoKeySQL is readIncidentsSQL on a database whose incidents
// carry no feed key, migrated before incident_feed.
const readIncidentsNoKeySQL = `SELECT incident_id, incident_session_id, incident_locus, status,
       '', CAST(NULL AS timestamp with time zone)
  FROM jetsapi.incident
 WHERE incident_locus = ANY($1::text[]) AND incident_session_id = ANY($2::text[])
 ORDER BY incident_id`

const tablesSQL = `SELECT to_regclass('jetsapi.input_registry') IS NOT NULL,
       to_regclass('jetsapi.incident') IS NOT NULL, to_regclass('jetsapi.incident_event') IS NOT NULL,
       EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'jetsapi' AND table_name = 'incident' AND column_name = 'incident_feed')`

// Read computes one feed's conditions and reads its incidents, those on its
// positions and those carrying its key, with their first human verdicts. It
// writes nothing.
func Read(ctx context.Context, db observe.DB, feed string) (*Feed, error) {
	var registry, incidents, events, keyed bool
	if err := db.QueryRow(ctx, tablesSQL).Scan(&registry, &incidents, &events, &keyed); err != nil {
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
	stmt, args := readIncidentsNoKeySQL, []any{triage.DeliveryLoci, sessions}
	if keyed {
		stmt, args = readIncidentsSQL, []any{triage.DeliveryLoci, sessions, feed}
	}
	rows, err := db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, fmt.Errorf("while reading the incidents on feed %s's deliveries: %w", feed, err)
	}
	var ids []string
	for rows.Next() {
		inc := &Incident{}
		var at *time.Time
		if err := rows.Scan(&inc.IncidentId, &inc.SessionId, &inc.Locus, &inc.Status, &inc.Feed, &at); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning an incident of feed %s: %w", feed, err)
		}
		if at != nil && inc.Feed != "" {
			inc.Position = at.UTC()
		} else {
			inc.Feed = ""
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

// FeedsOf returns the feed each of the named incidents carries as its key,
// for those that carry one: the feed whose condition finds the incident after
// the delivery it is written on is deleted. A database whose incidents carry
// no feed key returns none.
func FeedsOf(ctx context.Context, db observe.DB, incidentIds []string) (map[string]string, error) {
	out := map[string]string{}
	if len(incidentIds) == 0 {
		return out, nil
	}
	var keyed bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'jetsapi' AND table_name = 'incident' AND column_name = 'incident_feed')`).Scan(&keyed); err != nil {
		return nil, fmt.Errorf("while asking whether incidents carry a feed key: %w", err)
	}
	if !keyed {
		return out, nil
	}
	rows, err := db.Query(ctx, `SELECT incident_id, incident_feed FROM jetsapi.incident
		WHERE incident_id = ANY($1::text[]) AND incident_feed IS NOT NULL AND incident_anchor_start_time IS NOT NULL`,
		incidentIds)
	if err != nil {
		return nil, fmt.Errorf("while reading the feed keys of %d incidents: %w", len(incidentIds), err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, feed string
		if err := rows.Scan(&id, &feed); err != nil {
			return nil, fmt.Errorf("while scanning an incident's feed key: %w", err)
		}
		out[id] = feed
	}
	return out, rows.Err()
}

// ReadKeyed reads the feed an incident carries as its key, and returns it with
// the incident as the feed holds it; both nil when the incident carries no
// key. It is how a reader finds the condition of an incident whose own
// delivery is deleted, which ResolveSession cannot: that delivery's anomalies
// went with it.
func ReadKeyed(ctx context.Context, db observe.DB, incidentId string) (*Feed, *Incident, error) {
	feeds, err := FeedsOf(ctx, db, []string{incidentId})
	if err != nil || feeds[incidentId] == "" {
		return nil, nil, err
	}
	f, err := Read(ctx, db, feeds[incidentId])
	if err != nil {
		return nil, nil, err
	}
	inc := f.IncidentById(incidentId)
	if inc == nil {
		return nil, nil, nil
	}
	return f, inc, nil
}

// FromHistory computes a feed's conditions with no incident read: the
// conditions alone, or a Feed to which a test adds incidents.
func FromHistory(h *triage.FeedHistory) *Feed {
	f := &Feed{Feed: h.Feed, History: h, Conditions: h.Conditions(),
		Incidents: map[string]map[string]*Incident{}, order: map[string]int{}, positions: h.Positions()}
	for i, p := range f.positions {
		f.order[p.SessionId] = i
	}
	return f
}

// AddIncident records an incident of the feed, as Read does from the
// database: on one of its positions or, carrying the feed's key and a
// position, anywhere. It is for a test, or for a caller that holds incidents
// read another way. An incident with no key is given its session's arrival as
// its position, as Read gives one.
func (f *Feed) AddIncident(inc Incident) { f.add(&inc) }

func (f *Feed) add(inc *Incident) {
	if !inc.Keyed() {
		inc.Feed, inc.Position = "", time.Time{}
		if at, ok := triage.SessionArrival(inc.SessionId); ok {
			inc.Position = at
		}
	}
	if f.Incidents[inc.Locus] == nil {
		f.Incidents[inc.Locus] = map[string]*Incident{}
	}
	f.Incidents[inc.Locus][inc.SessionId] = inc
}

// IncidentsOn returns the incidents a condition holds, by position: those on
// its members, and those carrying the feed's key whose position lies between
// the condition's bounding ends (holds).
func (f *Feed) IncidentsOn(c *triage.Condition) []*Incident {
	var out []*Incident
	for _, inc := range f.Incidents[c.Locus] {
		if f.holds(c, inc) {
			out = append(out, inc)
		}
	}
	slices.SortFunc(out, func(a, b *Incident) int {
		switch {
		case a.before(b.Position, b.SessionId):
			return -1
		case b.before(a.Position, a.SessionId):
			return 1
		}
		return strings.Compare(a.IncidentId, b.IncidentId)
	})
	return out
}

// holds reports whether a condition holds an incident at its locus. An
// incident on one of its members is its own, keyed or not. Otherwise only an
// incident carrying the feed's key, on a session that is no position of the
// feed (a delivery deleted with its session), is held, by its position: after
// the feed's position before the condition's first member, and before the
// position that ends the condition. Where that position carries a finding at
// the locus too, a day boundary, the deleted deliveries between them are the
// next condition's, so the condition holds only those before its last member;
// next to a session shared with another feed's finding they stay this one's,
// since a shared session's condition holds no incident but its own. A
// condition of one on an unordered or shared session holds only its own.
func (f *Feed) holds(c *triage.Condition, inc *Incident) bool {
	if inc.Locus != c.Locus {
		return false
	}
	if c.IndexOf(inc.SessionId) >= 0 {
		return true
	}
	if c.Unordered || c.Shared || inc.Feed != f.Feed || !inc.Keyed() {
		return false
	}
	if _, ok := f.order[inc.SessionId]; ok {
		return false // a position that is not a member is another condition's, or ends this one
	}
	if at, ok := f.order[c.First().SessionId]; ok && at > 0 {
		lo := f.positions[at-1]
		if !before(lo.Arrival, lo.SessionId, inc.Position, inc.SessionId) {
			return false
		}
	}
	switch c.End {
	case "":
		return true
	case triage.EndDayBoundary:
		last := c.Last()
		return inc.before(last.Arrival, last.SessionId)
	default:
		at, ok := f.order[c.EndedBy]
		if !ok {
			return false
		}
		hi := f.positions[at]
		return inc.before(hi.Arrival, hi.SessionId)
	}
}

// ConditionOf returns the condition holding an incident of the feed: the one
// its session is a member of, or, for an incident carrying the feed's key on a
// session that is no position (a delivery deleted with its session), the one
// that holds it by its position (holds). Nil when no condition holds it.
func (f *Feed) ConditionOf(inc *Incident) *triage.Condition {
	if inc == nil {
		return nil
	}
	if c := triage.ConditionAt(f.Conditions, inc.SessionId, inc.Locus); c != nil {
		return c
	}
	for i := range f.Conditions {
		if c := &f.Conditions[i]; f.holds(c, inc) {
			return c
		}
	}
	return nil
}

// ConditionOfOrNil is ConditionOf on a feed that may be nil, as ReadKeyed
// returns for an incident with no key.
func (f *Feed) ConditionOfOrNil(inc *Incident) *triage.Condition {
	if f == nil {
		return nil
	}
	return f.ConditionOf(inc)
}

// Members returns the condition holding an incident and the members Resolve
// places in it: those it joins, and its own delivery while that is a
// position. It is what says whether an incident still stands for a delivery
// the database holds. Nil when no condition holds the incident.
func (f *Feed) Members(inc *Incident) (*triage.Condition, []triage.ConditionMember) {
	c := f.ConditionOf(inc)
	if c == nil {
		return nil, nil
	}
	var out []triage.ConditionMember
	for _, m := range c.Members {
		p := f.Resolve(m.SessionId, c.Locus)
		if (p.Kind == Join || p.Kind == AlreadyRaised) && p.Target != nil && p.Target.IncidentId == inc.IncidentId {
			out = append(out, m)
		}
	}
	return c, out
}

// IncidentBefore returns the latest incident a condition holds that is
// positioned before inc, or nil: an earlier incident of the same condition,
// found by position, so it is found after the delivery it is written on is
// deleted.
func (f *Feed) IncidentBefore(c *triage.Condition, inc *Incident) *Incident {
	if c == nil || inc == nil {
		return nil
	}
	var out *Incident
	for _, o := range f.IncidentsOn(c) {
		if o.IncidentId != inc.IncidentId && o.before(inc.Position, inc.SessionId) {
			out = o
		}
	}
	return out
}

// IncidentById returns the incident of the feed with this id, or nil.
func (f *Feed) IncidentById(id string) *Incident {
	for _, bySession := range f.Incidents {
		for _, inc := range bySession {
			if inc.IncidentId == id {
				return inc
			}
		}
	}
	return nil
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
	// The latest incident the condition holds at or before the session (one on
	// the session itself is already raised, above), else the earliest after it.
	at := c.IndexOf(session)
	m := c.Members[at]
	incs := f.IncidentsOn(c)
	var target *Incident
	for _, inc := range incs {
		if inc.before(m.Arrival, m.SessionId) {
			target = inc
		}
	}
	for _, inc := range incs {
		if target != nil {
			break
		}
		if !inc.before(m.Arrival, m.SessionId) {
			target = inc
		}
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
	p, feed := placeByFeeds(session, locus, feedsAt(as, locus))
	if feed == "" {
		return p, nil, nil
	}
	f, err := Read(ctx, db, feed)
	if err != nil {
		return Placement{}, nil, err
	}
	return f.Resolve(session, locus), f, nil
}

// feedsAt names the feeds whose anomalies land at the locus, sorted.
func feedsAt(as []observe.Anomaly, locus string) []string {
	var feeds []string
	for _, a := range as {
		if l, ok := triage.LocusOf(a.SubjectType, a.SignalType); ok && l == locus && a.SubjectType == observe.SubjectFeed &&
			!slices.Contains(feeds, a.SubjectRef) {
			feeds = append(feeds, a.SubjectRef)
		}
	}
	slices.Sort(feeds)
	return feeds
}

// placeByFeeds is ResolveSession's rule for which feed's condition can hold a
// session at a locus: exactly one feed, named in the second return. With none
// or two, the placement is WriteHere and the feed is "".
func placeByFeeds(session, locus string, feeds []string) (Placement, string) {
	switch len(feeds) {
	case 0:
		return Placement{Kind: WriteHere, SessionId: session, Locus: locus, Anchor: session,
			Why: "no feed anomaly on the session lands at this locus, so no feed's condition holds it"}, ""
	case 1:
		return Placement{}, feeds[0]
	default:
		return Placement{Kind: WriteHere, SessionId: session, Locus: locus, Anchor: session,
			Why: fmt.Sprintf("the session carries findings of %d feeds at this locus (%s), and one incident keyed "+
				"on the session and the locus cannot sit in two feeds' conditions, so it is written here",
				len(feeds), strings.Join(feeds, ", "))}, ""
	}
}

// SessionLocus names a finding's place: a session and a delivery locus.
type SessionLocus struct{ Session, Locus string }

// Resolved is ResolveSession's answer for one SessionLocus: the placement, and
// the feed whose condition holds it, nil when none does.
type Resolved struct {
	Placement Placement
	Feed      *Feed
}

// ResolveSessions is ResolveSession for many places at once, by the same rule,
// for a list that shows one row per incident: it reads the anomalies of every
// session in one statement and each feed once, however many of the places it
// holds. It writes nothing.
func ResolveSessions(ctx context.Context, db observe.DB, places []SessionLocus) (map[SessionLocus]Resolved, error) {
	out := make(map[SessionLocus]Resolved, len(places))
	if len(places) == 0 {
		return out, nil
	}
	sessions := make([]string, 0, len(places))
	for _, pl := range places {
		if !slices.Contains(sessions, pl.Session) {
			sessions = append(sessions, pl.Session)
		}
	}
	rows, err := db.Query(ctx, `SELECT DISTINCT anomaly_session_id, anomaly_subject_type, anomaly_subject_ref,
	       anomaly_signal_type
	  FROM jetsapi.anomaly
	 WHERE anomaly_session_id = ANY($1::text[]) AND anomaly_subject_type = $2`, sessions, observe.SubjectFeed)
	if err != nil {
		return nil, fmt.Errorf("while reading the feed anomalies of %d sessions: %w", len(sessions), err)
	}
	bySession := map[string][]observe.Anomaly{}
	for rows.Next() {
		var a observe.Anomaly
		if err := rows.Scan(&a.SessionId, &a.SubjectType, &a.SubjectRef, &a.SignalType); err != nil {
			rows.Close()
			return nil, fmt.Errorf("while scanning a feed anomaly: %w", err)
		}
		bySession[a.SessionId] = append(bySession[a.SessionId], a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("while reading the feed anomalies of %d sessions: %w", len(sessions), err)
	}
	feeds := map[string]*Feed{}
	for _, pl := range places {
		p, name := placeByFeeds(pl.Session, pl.Locus, feedsAt(bySession[pl.Session], pl.Locus))
		if name == "" {
			out[pl] = Resolved{Placement: p}
			continue
		}
		f, ok := feeds[name]
		if !ok {
			if f, err = Read(ctx, db, name); err != nil {
				return nil, err
			}
			feeds[name] = f
		}
		out[pl] = Resolved{Placement: f.Resolve(pl.Session, pl.Locus), Feed: f}
	}
	return out, nil
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
