package triage

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// # One fault, one incident: a feed's conditions
//
// A feed detector judges each delivery of a feed and writes an anomaly on the
// delivery that departs from the feed's profile. When a feed sends the same
// wrong file every week, each delivery carries its own correct finding, and
// together they are one fault that persists. Written one incident per
// delivery, eight weeks of it are eight incidents for a person to read.
//
// So the findings of a feed are grouped into conditions. **A condition is a
// maximal run of consecutive positions of one feed that each carry a finding
// of the feed at one locus**, a finding at a locus being a feed anomaly that
// LocusOf places there. It is keyed on the feed and the locus, not the
// detector: one delivery's findings at a locus are already one incident, so a
// key on the detector would split what is already together.
//
// **The positions of a feed** are its deliveries, as the feed detectors read
// them (observe.FeedRegistrations.Deliveries: each file at its first
// registration, in session-id then registration order), plus every session
// whose id is a number and that carries an anomaly of the feed without being
// one of its deliveries. A later registration of a file the feed already
// delivered is re-processing, not a delivery: it is not a position, so it
// neither breaks a condition nor extends one.
//
// **A position with no finding of the feed at the locus ends the condition**,
// whatever the reason: the delivery was judged and passed, it was not judged,
// or it has not been judged yet. The detectors store only what fired, so a
// clean judgement cannot be told from no judgement; splitting on both errs
// towards one incident too many, never one too few.
//
// Four edge rules:
//
//   - **A session whose id is not a number** has no position, so its finding
//     is a condition of one.
//   - **A session carrying findings of two feeds at one locus** is a condition
//     of one for each feed, and ends the conditions either side of it: an
//     incident is keyed on the session and the locus, so it cannot sit in two
//     feeds' conditions.
//   - **Where a feed's registrations no longer reach back**, the purge having
//     removed them while it keeps anomalies, its clean deliveries cannot be
//     seen. Before the feed's earliest surviving delivery, and everywhere when
//     input_registry is not deployed, two positions are consecutive only when
//     their session ids fall on one UTC day, which is the rule a day's burst
//     of one detector was read by before conditions.
//   - **A condition holds no verdict of its own.** Where its incident lives,
//     and whether a later finding joins it after a person judged it, is the
//     shadow writer's, through the feedcondition package; this file only
//     reads what the database holds.

// ConditionMember is one position of a condition: a session carrying a finding
// of the feed at the condition's locus.
type ConditionMember struct {
	SessionId string
	// Delivery reports whether the session is a delivery of the feed; false for
	// a session that carries a finding of the feed and no registration of it.
	Delivery bool
	// Arrival is the session id read as epoch milliseconds, or zero for a
	// session id that is not a number.
	Arrival time.Time
	// FirstDetected is the earliest detected_at among the feed's anomalies at
	// the locus on this session.
	FirstDetected time.Time
	// Anomalies are those anomalies' ids, oldest first, and Detectors their
	// detector refs, each once.
	Anomalies []string
	Detectors []string
}

// Why a condition ended.
const (
	// EndNoFinding: the feed's next position carries no finding of the feed at
	// the locus.
	EndNoFinding = "no_finding"
	// EndOtherFeed: the next position carries findings of two feeds at the
	// locus, and is a condition of one of its own.
	EndOtherFeed = "other_feed"
	// EndDayBoundary: the next position carries a finding, but both fall before
	// the feed's earliest surviving delivery and on different UTC days.
	EndDayBoundary = "day_boundary"
)

// Condition is a maximal run of consecutive positions of one feed that each
// carry a finding of the feed at one locus.
type Condition struct {
	Feed  string
	Locus string
	// Members are the condition's positions in the feed's order.
	Members []ConditionMember
	// EndedBy is the session of the feed's position after the last member, and
	// End why it ended the condition. Both are "" when the last member is the
	// feed's latest position: the condition is still open.
	EndedBy string
	End     string
	// SameDay reports that at least one link of the condition was judged by
	// the purged-era rule: consecutive only on one UTC day.
	SameDay bool
	// Unordered reports a condition of one on a session whose id is not a
	// number, and Shared one on a session carrying findings of another feed at
	// the same locus.
	Unordered bool
	Shared    bool
}

// Open reports whether the condition reaches the feed's latest position.
func (c *Condition) Open() bool { return c != nil && !c.Unordered && c.EndedBy == "" }

// First is the condition's first member.
func (c *Condition) First() ConditionMember { return c.Members[0] }

// Last is the condition's last member.
func (c *Condition) Last() ConditionMember { return c.Members[len(c.Members)-1] }

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

// Describe renders the condition as it stands, for an incident's basis.
func (c *Condition) Describe() string {
	var b strings.Builder
	n := len(c.Members)
	fmt.Fprintf(&b, "a condition of feed %s at locus %s: %d %s", c.Feed, c.Locus, n, plural(n, "delivery", "deliveries"))
	if unregistered := c.unregistered(); unregistered > 0 {
		fmt.Fprintf(&b, " (%d of them %s with no registration of the feed)", unregistered,
			plural(unregistered, "a session", "sessions"))
	}
	if first, last := c.First().Arrival, c.Last().Arrival; !first.IsZero() && !last.IsZero() {
		if n == 1 {
			fmt.Fprintf(&b, ", registered %s", first.Format(timeFmt))
		} else {
			fmt.Fprintf(&b, ", registered from %s to %s", first.Format(timeFmt), last.Format(timeFmt))
		}
	}
	fmt.Fprintf(&b, ", %s %s", plural(n, "session", "sessions"), strings.Join(c.Sessions(), ", "))
	switch {
	case c.Unordered:
		b.WriteString("; its session id is not a time, so it has no place in the feed's order and is a condition of one")
	case c.Shared:
		b.WriteString("; the session carries findings of another feed at this locus too, so it is a condition " +
			"of one, and one incident stands for both feeds' findings there")
	case c.EndedBy == "":
		b.WriteString("; it is open: the feed's latest position carries a finding here")
	case c.End == EndDayBoundary:
		fmt.Fprintf(&b, "; it ends before session %s, the next with a finding, because both fall before the feed's "+
			"earliest surviving registration and on different UTC days, so the deliveries between them cannot be seen",
			c.EndedBy)
	case c.End == EndOtherFeed:
		fmt.Fprintf(&b, "; it ends at session %s, which carries findings of another feed at this locus too", c.EndedBy)
	default:
		fmt.Fprintf(&b, "; it ended at session %s, the feed's next position, which carries no finding here "+
			"(judged clean, or not judged)", c.EndedBy)
	}
	if c.SameDay {
		b.WriteString(". Part of it falls before the feed's earliest surviving registration, where two findings are " +
			"consecutive only on one UTC day")
	}
	return b.String()
}

func (c *Condition) unregistered() int {
	n := 0
	for i := range c.Members {
		if !c.Members[i].Delivery {
			n++
		}
	}
	return n
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// FeedPosition is one place in a feed's order.
type FeedPosition struct {
	SessionId string
	Arrival   time.Time
	// Delivery reports whether it is a delivery of the feed.
	Delivery bool
}

// FeedHistory is what a feed's conditions are computed from: its deliveries
// and the feed anomalies on every session carrying one of its own.
type FeedHistory struct {
	Feed string
	// Registry reports whether jetsapi.input_registry was read. When it is
	// false no delivery is known, and every link is judged by the same-day rule.
	Registry bool
	// Deliveries are the feed's deliveries in registration order, as
	// observe.FeedRegistrations.Deliveries returns them.
	Deliveries []observe.FeedDelivery
	// Anomalies are the subject-feed anomalies on every session carrying one
	// of this feed's: this feed's own, and another feed's on the same sessions.
	Anomalies []observe.Anomaly
}

// position is a FeedPosition with its order key.
type position struct {
	FeedPosition
	ms int64
}

// sessionMillis reads a session id as epoch milliseconds, as the feed
// detectors do.
func sessionMillis(s string) (int64, bool) {
	ms, err := strconv.ParseInt(s, 10, 64)
	return ms, err == nil && ms > 0
}

// positions returns the feed's ordered positions and the sessions carrying an
// anomaly of the feed whose id is not a number.
func (h *FeedHistory) positions() ([]position, []string) {
	var out []position
	seen := map[string]bool{}
	for _, d := range h.Deliveries {
		if seen[d.SessionId] {
			continue // a session that delivered two files of the feed is one position
		}
		seen[d.SessionId] = true
		out = append(out, position{FeedPosition{SessionId: d.SessionId, Arrival: d.Arrival, Delivery: true},
			d.Arrival.UnixMilli()})
	}
	var unordered []string
	for _, a := range h.Anomalies {
		if a.SubjectRef != h.Feed || seen[a.SessionId] {
			continue
		}
		seen[a.SessionId] = true
		ms, ok := sessionMillis(a.SessionId)
		if !ok {
			unordered = append(unordered, a.SessionId)
			continue
		}
		out = append(out, position{FeedPosition{SessionId: a.SessionId, Arrival: time.UnixMilli(ms).UTC()}, ms})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ms != out[j].ms {
			return out[i].ms < out[j].ms
		}
		return out[i].SessionId < out[j].SessionId
	})
	sort.Strings(unordered)
	return out, unordered
}

// Positions returns the feed's positions in order: its deliveries and the
// numeric sessions carrying an anomaly of the feed with no registration of it.
func (h *FeedHistory) Positions() []FeedPosition {
	ps, _ := h.positions()
	out := make([]FeedPosition, len(ps))
	for i := range ps {
		out[i] = ps[i].FeedPosition
	}
	return out
}

// Conditions computes the feed's conditions at every locus its anomalies reach:
// by locus in Loci order, then in the feed's order, the conditions of one on a
// session id that is not a number last.
func (h *FeedHistory) Conditions() []Condition {
	ps, unordered := h.positions()

	// The earliest surviving delivery: before it, the same-day rule.
	var firstDelivery int64 = -1
	if h.Registry {
		for _, p := range ps {
			if p.Delivery {
				firstDelivery = p.ms
				break
			}
		}
	}
	purged := func(i int) bool { return firstDelivery < 0 || ps[i].ms < firstDelivery }
	sameDay := func(i, j int) bool { return ps[i].ms/86400000 == ps[j].ms/86400000 }

	// What each session carries, by locus: this feed's anomalies, and whether
	// another feed has one there.
	own := map[string]map[string][]observe.Anomaly{} // locus -> session -> anomalies
	other := map[string]map[string]bool{}            // locus -> session -> another feed fired
	var loci []string
	for _, a := range h.Anomalies {
		if a.SubjectType != observe.SubjectFeed {
			continue
		}
		l, ok := LocusOf(a.SubjectType, a.SignalType)
		if !ok {
			continue
		}
		if a.SubjectRef != h.Feed {
			if other[l] == nil {
				other[l] = map[string]bool{}
			}
			other[l][a.SessionId] = true
			continue
		}
		if own[l] == nil {
			own[l] = map[string][]observe.Anomaly{}
			loci = append(loci, l)
		}
		own[l][a.SessionId] = append(own[l][a.SessionId], a)
	}
	slices.SortFunc(loci, func(a, b string) int { return slices.Index(Loci, a) - slices.Index(Loci, b) })

	member := func(l, session string, p *position) ConditionMember {
		m := ConditionMember{SessionId: session}
		if p != nil {
			m.Delivery, m.Arrival = p.Delivery, p.Arrival
		}
		as := slices.Clone(own[l][session])
		slices.SortFunc(as, func(a, b observe.Anomaly) int {
			if c := a.DetectedAt.Compare(b.DetectedAt); c != 0 {
				return c
			}
			return strings.Compare(a.AnomalyId, b.AnomalyId)
		})
		for i, a := range as {
			if i == 0 {
				m.FirstDetected = a.DetectedAt
			}
			m.Anomalies = append(m.Anomalies, a.AnomalyId)
			if !slices.Contains(m.Detectors, a.DetectorRef) {
				m.Detectors = append(m.Detectors, a.DetectorRef)
			}
		}
		return m
	}

	var out []Condition
	for _, l := range loci {
		carries := func(i int) bool { return len(own[l][ps[i].SessionId]) > 0 }
		shared := func(i int) bool { return other[l][ps[i].SessionId] }
		for i := 0; i < len(ps); {
			if !carries(i) {
				i++
				continue
			}
			c := Condition{Feed: h.Feed, Locus: l, Shared: shared(i)}
			c.Members = append(c.Members, member(l, ps[i].SessionId, &ps[i]))
			j := i
			for !c.Shared && j+1 < len(ps) && carries(j+1) && !shared(j+1) {
				if purged(j) {
					if !sameDay(j, j+1) {
						break
					}
					c.SameDay = true
				}
				j++
				c.Members = append(c.Members, member(l, ps[j].SessionId, &ps[j]))
			}
			if j+1 < len(ps) {
				c.EndedBy = ps[j+1].SessionId
				switch {
				case !carries(j + 1):
					c.End = EndNoFinding
				case c.Shared || shared(j+1):
					c.End = EndOtherFeed
				default:
					c.End = EndDayBoundary
				}
			}
			out = append(out, c)
			i = j + 1
		}
		for _, s := range unordered {
			if len(own[l][s]) > 0 {
				out = append(out, Condition{Feed: h.Feed, Locus: l, Unordered: true, Shared: other[l][s],
					Members: []ConditionMember{member(l, s, nil)}})
			}
		}
	}
	return out
}

// ConditionAt returns the condition at the locus holding the session, or nil.
func ConditionAt(cs []Condition, session, locus string) *Condition {
	for i := range cs {
		if cs[i].Locus == locus && cs[i].IndexOf(session) >= 0 {
			return &cs[i]
		}
	}
	return nil
}

// feedRegistrationsSQL reads one feed's file registrations in registration
// order, observe.FeedRegistrationOrderSQL, the order the feed detectors read
// them in. It reads input_registry alone, the columns that order and identify
// a delivery, so that the conditions can be computed from an export of four
// tables (anomaly, input_registry, incident, incident_event).
const feedRegistrationsSQL = `SELECT ir.session_id, COALESCE(ir.file_key, '')
  FROM jetsapi.input_registry ir
 WHERE ir.table_name = $1 AND ir.source_type = 'file'
 ORDER BY ` + observe.FeedRegistrationOrderSQL

// feedAnomaliesSQL reads the subject-feed anomalies on every session carrying
// one of the feed's.
const feedAnomaliesSQL = `SELECT anomaly_id, detected_at, anomaly_session_id, anomaly_subject_type,
       anomaly_subject_ref, anomaly_signal_type, anomaly_detector_ref
  FROM jetsapi.anomaly
 WHERE anomaly_subject_type = $1
   AND anomaly_session_id IN (SELECT anomaly_session_id FROM jetsapi.anomaly
                               WHERE anomaly_subject_type = $1 AND anomaly_subject_ref = $2)
 ORDER BY detected_at, anomaly_id`

// RegistryDeployedSQL asks whether jetsapi.input_registry exists.
const RegistryDeployedSQL = `SELECT to_regclass('jetsapi.input_registry') IS NOT NULL`

// ReadFeedHistory reads what a feed's conditions are computed from. It writes
// nothing. registry says whether jetsapi.input_registry exists; when it does
// not, no delivery is read.
func ReadFeedHistory(ctx context.Context, db observe.DB, feed string, registry bool) (*FeedHistory, error) {
	h := &FeedHistory{Feed: feed, Registry: registry}
	if registry {
		rows, err := db.Query(ctx, feedRegistrationsSQL, feed)
		if err != nil {
			return nil, fmt.Errorf("while reading the registrations of feed %s: %w", feed, err)
		}
		regs := &observe.FeedRegistrations{ByTable: map[string][]observe.FeedDelivery{}, Unordered: map[string]int{}}
		for rows.Next() {
			var d observe.FeedDelivery
			if err := rows.Scan(&d.SessionId, &d.FileKey); err != nil {
				rows.Close()
				return nil, fmt.Errorf("while scanning a registration of feed %s: %w", feed, err)
			}
			ms, ok := sessionMillis(d.SessionId)
			if !ok {
				regs.Unordered[feed]++
				continue
			}
			d.Arrival = time.UnixMilli(ms).UTC()
			regs.ByTable[feed] = append(regs.ByTable[feed], d)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("while reading the registrations of feed %s: %w", feed, err)
		}
		h.Deliveries = regs.Deliveries(feed, time.Time{})
	}
	rows, err := db.Query(ctx, feedAnomaliesSQL, observe.SubjectFeed, feed)
	if err != nil {
		return nil, fmt.Errorf("while reading the anomalies of feed %s: %w", feed, err)
	}
	defer rows.Close()
	for rows.Next() {
		var a observe.Anomaly
		if err := rows.Scan(&a.AnomalyId, &a.DetectedAt, &a.SessionId, &a.SubjectType, &a.SubjectRef,
			&a.SignalType, &a.DetectorRef); err != nil {
			return nil, fmt.Errorf("while scanning an anomaly of feed %s: %w", feed, err)
		}
		h.Anomalies = append(h.Anomalies, a)
	}
	return h, rows.Err()
}
