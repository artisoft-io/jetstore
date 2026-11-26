package feedsummary

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// Summarise reads a feed incident's input and renders its summary. It is what
// the incident read calls, at read time: the summary is not stored.
func Summarise(ctx context.Context, db observe.DB, inc *audit.Incident) (*Summary, error) {
	in, err := Read(ctx, db, inc)
	if err != nil {
		return nil, err
	}
	return Render(in)
}

// IsFeedIncident reports whether an incident sits at a delivery locus, which
// is what makes it a feed incident and gives it a summary.
func IsFeedIncident(locus string) bool { return slices.Contains(triage.DeliveryLoci, locus) }

// Read builds an incident's Input from what the database holds: the incident
// and its hypotheses (already read by the caller), the session's feed
// anomalies at the incident's locus, the feed's source configuration and
// profile, and the feed's condition at the locus with the incidents on it,
// read through feedcondition.
//
// It reads the same rows triage read to place the incident
// (triage.ReadFeedEvidence), so the summary describes the finding triage
// classified and not a re-derivation of it.
func Read(ctx context.Context, db observe.DB, inc *audit.Incident) (*Input, error) {
	if inc == nil {
		return nil, fmt.Errorf("no incident")
	}
	if !IsFeedIncident(inc.Locus) {
		return nil, fmt.Errorf("incident %s is at %s, which is not a delivery locus", inc.IncidentId, inc.Locus)
	}
	ext, err := triage.ReadExtent(ctx, db)
	if err != nil {
		return nil, err
	}
	ev, err := triage.ReadFeedEvidence(ctx, db, inc.SessionId, ext)
	if err != nil {
		return nil, err
	}
	if !ev.Read {
		return nil, fmt.Errorf("the feed findings of incident %s cannot be read: %s", inc.IncidentId, ev.Note)
	}
	at := ev.At(inc.Locus)
	var fc *feedcondition.Feed
	if len(at) == 0 {
		// The delivery the incident is written on was deleted with its session,
		// and its anomalies with it. An incident carrying its feed's key still
		// stands for the deliveries of its condition placed in it, and is
		// summarised from the earliest of them.
		var from string
		if fc, from, err = keyedEvidence(ctx, db, inc); err != nil {
			return nil, err
		}
		if fc == nil {
			return nil, fmt.Errorf("incident %s has no feed anomaly at %s on session %s", inc.IncidentId, inc.Locus,
				inc.SessionId)
		}
		if ev, err = triage.ReadFeedEvidence(ctx, db, from, ext); err != nil {
			return nil, err
		}
		if !ev.Read {
			return nil, fmt.Errorf("the feed findings of incident %s cannot be read: %s", inc.IncidentId, ev.Note)
		}
		at = nil
		for _, a := range ev.At(inc.Locus) {
			if a.SubjectRef == fc.Feed {
				at = append(at, a)
			}
		}
		if len(at) == 0 {
			return nil, fmt.Errorf("incident %s has no feed anomaly at %s on session %s, nor on session %s of its "+
				"condition", inc.IncidentId, inc.Locus, inc.SessionId, from)
		}
	}
	tables := []string{}
	for _, a := range at {
		if !slices.Contains(tables, a.SubjectRef) {
			tables = append(tables, a.SubjectRef)
		}
	}
	sort.Strings(tables)
	table := tables[0]
	if fc != nil {
		table = fc.Feed
	}
	in := &Input{
		IncidentID: inc.IncidentId,
		Locus:      inc.Locus,
		DetectedAt: inc.DetectedAt.UTC(),
		OtherFeeds: len(tables) - 1,
	}
	declared := ev.Feed(table)
	in.Feed, in.Declaration = feedOf(table, declared, ev, in.DetectedAt)

	var required []string
	if declared != nil && declared.Profiled {
		for _, rf := range declared.Feed.Profile.RequiredFields {
			required = append(required, rf.Column)
		}
	}
	for _, a := range at {
		if a.SubjectRef != table {
			continue
		}
		f, ok := parseFinding(a, required)
		if ok {
			in.Findings = append(in.Findings, f)
		}
	}
	sort.SliceStable(in.Findings, func(i, j int) bool {
		return detectorOrder(in.Findings[i].Detector) < detectorOrder(in.Findings[j].Detector)
	})

	hs := slices.Clone(inc.Hypotheses)
	sort.SliceStable(hs, func(i, j int) bool { return hs[i].Rank < hs[j].Rank })
	for _, h := range hs {
		in.Readings = append(in.Readings, Reading{
			Rank: int(h.Rank), Cause: h.CauseCategory, Confidence: h.Confidence,
			Supporting: h.Basis.SupportingCount, Contradicting: h.Basis.ContradictingCount,
			Evidenceability: h.Basis.Evidenceability,
		})
	}

	if fc == nil {
		if fc, err = feedcondition.Read(ctx, db, table); err != nil {
			return nil, err
		}
	}
	in.Condition = conditionOf(fc, inc.IncidentId, inc.SessionId, inc.Locus)
	if err := in.Validate(); err != nil {
		return nil, fmt.Errorf("incident %s: %w", inc.IncidentId, err)
	}
	return in, nil
}

// keyedEvidence finds, for an incident carrying its feed's key, the feed and
// the earliest delivery of its condition still placed in it, whose findings
// are the condition's; a nil feed when the incident carries no key or no
// delivery is placed in it.
func keyedEvidence(ctx context.Context, db observe.DB, inc *audit.Incident) (*feedcondition.Feed, string, error) {
	f, kinc, err := feedcondition.ReadKeyed(ctx, db, inc.IncidentId)
	if err != nil || f == nil {
		return nil, "", err
	}
	_, members := f.Members(kinc)
	if len(members) == 0 {
		return nil, "", nil
	}
	return f, members[0].SessionId, nil
}

func feedOf(table string, d *triage.DeclaredFeed, ev *triage.FeedEvidence, detected time.Time) (Feed, Declaration) {
	f := Feed{Table: table}
	var decl Declaration
	if d == nil || !d.Configured {
		return f, decl
	}
	f.Configured = true
	f.Client, f.Org, f.ObjectType, f.Cumulative = d.Feed.Client, d.Feed.Org, d.Feed.ObjectType, d.Feed.Cumulative
	for _, r := range ev.Registrations {
		if r.TableName == table && r.Client != d.Feed.Client {
			f.RegisteredUnder = r.Client
			break
		}
	}
	if !d.Profiled {
		return f, decl
	}
	p := d.Feed.Profile
	decl.Readable = true
	decl.DeclaredBy = p.DeclaredBy
	if t, ok := d.DeclaredAt(); ok {
		decl.DeclaredAt = t
		decl.AgeDays = int(detected.Sub(t).Hours() / 24)
	}
	decl.PeriodType = p.PeriodType
	if p.PeriodType != "" {
		decl.PeriodsPerDelivery = p.DeliveryPeriods()
	}
	decl.OverdueAfterDays = p.OverdueAfterDays
	return f, decl
}

func detectorOrder(d string) int {
	if i := slices.Index(feedprofile.Detectors, d); i >= 0 {
		return i
	}
	return len(feedprofile.Detectors)
}

// splitRef reads a detector reference, `name@generation`.
func splitRef(ref string) (string, int) {
	name, gen, ok := strings.Cut(ref, "@")
	if !ok {
		return ref, 0
	}
	n, _ := strconv.Atoi(gen)
	return name, n
}

var quantity = regexp.MustCompile(`^(-?[0-9]+(?:\.[0-9]+)?)(?: (days|calendar months|keys absent))?$`)

// parseQuantity reads a number with an optional unit from the closed list the
// detectors write. A value of any other shape is not a quantity, and nothing
// of it is kept.
func parseQuantity(s string) (*float64, string, bool) {
	m := quantity.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return nil, "", false
	}
	x, err := strconv.ParseFloat(m[1], 64)
	if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
		return nil, "", false
	}
	return &x, m[2], true
}

func parseDay(s string) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(s))
	return t, err == nil
}

func parseInstant(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	return t.UTC(), err == nil
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// parseFinding reads one anomaly into numbers, by what its detector writes.
// The detector is the anomaly's own reference and the column, for a
// completeness finding, is the anomaly identifier's last segment, kept only
// when the profile names it as a required field. Anything that does not parse
// is left unset rather than guessed, and the template then says less.
func parseFinding(a observe.Anomaly, required []string) (Finding, bool) {
	det, gen := splitRef(a.DetectorRef)
	if !slices.Contains(feedprofile.Detectors, det) {
		return Finding{}, false
	}
	f := Finding{Detector: det, Generation: gen, Signal: a.SignalType}
	if a.DeviationMagnitude != nil && !math.IsNaN(*a.DeviationMagnitude) && !math.IsInf(*a.DeviationMagnitude, 0) {
		m := *a.DeviationMagnitude
		f.Magnitude = &m
	}
	switch det {
	case feedprofile.FeedOverdue:
		if t, ok := parseDay(a.ObservedValue); ok {
			f.ObservedPeriod = t
		}
		if t, ok := parseInstant(deref(a.ExpectedMax)); ok {
			f.OverdueFrom = t
		}
	case feedprofile.DeliveryBackdated:
		if t, ok := parseDay(a.ObservedValue); ok {
			f.ObservedPeriod = t
		}
		if t, ok := parseDay(deref(a.ExpectedMin)); ok {
			f.BoundPeriod = t
		}
		if f.Magnitude != nil {
			n := *f.Magnitude
			f.Observed, f.ObservedUnit = &n, UnitPeriods
		}
	case feedprofile.DeliverySize:
		if x, u, ok := parseQuantity(a.ObservedValue); ok && u == "" {
			f.Observed, f.ObservedUnit = x, UnitBytes
		}
		lo, _, okLo := parseQuantity(deref(a.ExpectedMin))
		hi, _, okHi := parseQuantity(deref(a.ExpectedMax))
		switch {
		case okLo && okHi:
			f.Bound, f.BoundIsMin, f.Upper, f.BoundUnit = lo, true, hi, UnitBytes
		case okLo:
			f.Bound, f.BoundIsMin, f.BoundUnit = lo, true, UnitBytes
		case okHi:
			f.Bound, f.BoundUnit = hi, UnitBytes
		}
	case feedprofile.ServiceDateStale:
		if x, u, ok := parseQuantity(a.ObservedValue); ok && u != "" {
			f.Observed, f.ObservedUnit = x, u
		}
		if x, u, ok := parseQuantity(deref(a.ExpectedMax)); ok && u != "" {
			f.Bound, f.BoundUnit = x, u
		}
	case feedprofile.ServiceDateSpan:
		// The span in days against the declared span, or, where the profile
		// declares max_out_of_span_share, the share of dated rows outside the
		// span against that share. The observed value and the bound are in
		// one unit or the finding is left unread.
		x, u, okX := parseQuantity(a.ObservedValue)
		b, bu, okB := parseQuantity(deref(a.ExpectedMax))
		switch {
		case okX && okB && u == UnitDays && bu == UnitDays:
			f.Observed, f.ObservedUnit, f.Bound, f.BoundUnit = x, u, b, bu
		case okX && okB && u == "" && bu == "":
			f.Observed, f.ObservedUnit, f.Bound, f.BoundUnit = x, UnitShare, b, UnitShare
		}
	case feedprofile.FieldCompleteness:
		col := a.AnomalyId[strings.LastIndex(a.AnomalyId, "/")+1:]
		if !slices.Contains(required, col) {
			return Finding{}, false
		}
		f.Column = col
		if x, u, ok := parseQuantity(a.ObservedValue); ok && u == "" {
			f.Observed, f.ObservedUnit = x, UnitShare
		}
		if x, u, ok := parseQuantity(deref(a.ExpectedMin)); ok && u == "" {
			f.Bound, f.BoundUnit, f.BoundIsMin = x, UnitShare, true
		}
	case feedprofile.DeliveryOverlap:
		if x, u, ok := parseQuantity(a.ObservedValue); ok {
			switch u {
			case "":
				f.Observed, f.ObservedUnit = x, UnitShare
				if b, bu, ok := parseQuantity(deref(a.ExpectedMax)); ok && bu == "" {
					f.Bound, f.BoundUnit = b, UnitShare
				}
			case UnitKeysAbsent:
				f.Observed, f.ObservedUnit = x, UnitKeysAbsent
			}
		}
	case feedprofile.DeliveryUnchanged:
		f.Equal = strings.TrimSpace(a.ObservedValue) == "equal"
	}
	return f, true
}

// conditionOf reads the part of the feed's condition at the locus that an
// incident holds, and the incident before it, from a feed read by
// feedcondition. Which incident each position belongs to is feedcondition's
// Resolve, the rule shadow triage writes by, applied to every position of the
// condition: so the summary names the deliveries the writer joined to this
// incident and no other reading of them. It reads nothing, and returns nil when
// no condition of the feed holds the session at the locus.
//
// **An incident carrying the feed's key is found by it** when its own session
// is no position, its delivery deleted with its session: the condition is the
// one holding it by its position, and the incident before it the latest that
// condition holds before it, by position.
func conditionOf(f *feedcondition.Feed, incident, session, locus string) *Condition {
	self := f.IncidentById(incident)
	c := triage.ConditionAt(f.Conditions, session, locus)
	if c == nil && self != nil {
		c = f.ConditionOf(self)
	}
	if c == nil {
		return nil
	}
	if self == nil {
		self = &feedcondition.Incident{IncidentId: incident, SessionId: session, Locus: locus}
		if at, ok := triage.SessionArrival(session); ok {
			self.Position = at
		}
	}
	out := &Condition{Total: len(c.Members), SameDay: c.SameDay, Shared: c.Shared}
	if c.Unordered {
		out.Held, out.End = 1, EndUnordered
		if !c.First().Delivery {
			out.Unregistered = 1
		}
		return out
	}

	// The incident each position belongs to, or "" for none yet.
	owner := make([]string, len(c.Members))
	others := map[string]bool{}
	var held []int
	for i, m := range c.Members {
		p := f.Resolve(m.SessionId, locus)
		if (p.Kind == feedcondition.AlreadyRaised || p.Kind == feedcondition.Join) && p.Target != nil {
			owner[i] = p.Target.IncidentId
		}
		switch {
		case owner[i] == incident || m.SessionId == session:
			// The incident's own session is its own, whatever the incident
			// table says of it.
			owner[i] = incident
			held = append(held, i)
		case owner[i] != "":
			others[owner[i]] = true
		default:
			out.Unraised++
		}
	}
	for o := range others {
		out.Others = append(out.Others, o)
	}
	slices.Sort(out.Others)
	if len(held) == 0 {
		return nil
	}
	out.Held = len(held)
	for _, i := range held {
		if !c.Members[i].Delivery {
			out.Unregistered++
		}
	}
	fi, li := held[0], held[len(held)-1]
	out.First, out.Last = c.Members[fi].Arrival, c.Members[li].Arrival

	switch {
	case li == len(c.Members)-1 && c.Open():
		out.End = EndOpen
	case li == len(c.Members)-1:
		out.End = c.End
		for _, p := range f.History.Positions() {
			if p.SessionId == c.EndedBy {
				out.EndedAt = p.Arrival
				break
			}
		}
	default:
		next := c.Members[li+1]
		out.EndedAt, out.EndedInto = next.Arrival, owner[li+1]
		out.End = EndNextIncident
		if self.Verdict != nil && next.FirstDetected.After(self.Verdict.TransitionedAt) {
			out.End = EndVerdict
			out.Verdict, out.VerdictAt = self.Verdict.ToStatus, self.Verdict.TransitionedAt.UTC()
		}
	}

	// The incident before this one: an earlier incident of the same
	// condition, by position, or the feed's previous condition at the locus.
	before := f.IncidentBefore(c, self)
	prev := &out.Previous
	switch {
	case before != nil:
		// An earlier incident of the same condition is named as the previous
		// one when this incident is new evidence after a person's verdict on
		// it; otherwise it is one written before conditions, among Others.
		if before.Verdict != nil && c.Members[fi].FirstDetected.After(before.Verdict.TransitionedAt) {
			prev.Kind, prev.IncidentID = PrevAfterVerdict, before.IncidentId
			prev.Verdict, prev.VerdictAt = before.Verdict.ToStatus, before.Verdict.TransitionedAt.UTC()
		}
	default:
		pc := f.PreviousCondition(c.First().SessionId, locus)
		if pc == nil {
			prev.Kind = PrevNone
			break
		}
		prev.Kind, prev.Deliveries = PrevCondition, len(pc.Members)
		prev.First, prev.Last = pc.First().Arrival, pc.Last().Arrival
		if pi := f.IncidentOf(pc); pi != nil {
			prev.IncidentID = pi.IncidentId
			if pi.Verdict != nil {
				prev.Verdict, prev.VerdictAt = pi.Verdict.ToStatus, pi.Verdict.TransitionedAt.UTC()
			}
		}
	}
	return out
}
