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
// profile, and the other deliveries of the feed registered the same day.
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
	if len(at) == 0 {
		return nil, fmt.Errorf("incident %s has no feed anomaly at %s on session %s", inc.IncidentId, inc.Locus,
			inc.SessionId)
	}
	tables := []string{}
	for _, a := range at {
		if !slices.Contains(tables, a.SubjectRef) {
			tables = append(tables, a.SubjectRef)
		}
	}
	sort.Strings(tables)
	table := tables[0]
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

	bursts, err := readBursts(ctx, db, in, table)
	if err != nil {
		return nil, err
	}
	in.Bursts = bursts
	if err := in.Validate(); err != nil {
		return nil, fmt.Errorf("incident %s: %w", inc.IncidentId, err)
	}
	return in, nil
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

// burstSQL reads, for one feed and one detector, the sessions registered on a
// day that carry that detector's finding, and the incident each raised at the
// locus, if any. The registration day is the session id read as epoch
// milliseconds, which is how RegisterFileKeys seeds it and how the feed
// detectors read a delivery's arrival.
const burstSQL = `SELECT DISTINCT a.anomaly_session_id, i.incident_id
  FROM jetsapi.anomaly a
  LEFT JOIN jetsapi.incident i
         ON i.incident_session_id = a.anomaly_session_id AND i.incident_locus = $4
 WHERE a.anomaly_subject_type = 'feed' AND a.anomaly_subject_ref = $1
   AND split_part(a.anomaly_detector_ref, '@', 1) = $2
   AND a.anomaly_session_id ~ '^[0-9]{1,18}$'
   AND (to_timestamp(a.anomaly_session_id::bigint / 1000.0) AT TIME ZONE 'UTC')::date = $3::date
 ORDER BY 1`

// sessionDay reads a session id as the registration day it encodes.
func sessionDay(session string) (time.Time, bool) {
	if len(session) == 0 || len(session) > 18 {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(session, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	t := time.UnixMilli(ms).UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
}

// readBursts finds, for every detector that fired on this delivery, the other
// deliveries of the feed registered the same day on which it also fired, and
// which of them carry an incident of their own. Shadow triage raises such a
// burst as a single incident, so usually none does. It changes nothing.
func readBursts(ctx context.Context, db observe.DB, in *Input, table string) ([]Burst, error) {
	session := strings.TrimSuffix(strings.TrimPrefix(in.IncidentID, "inc_"), "_"+in.Locus)
	day, ok := sessionDay(session)
	if !ok {
		return nil, nil
	}
	var out []Burst
	seen := map[string]bool{}
	for _, f := range in.Findings {
		if seen[f.Detector] {
			continue
		}
		seen[f.Detector] = true
		rows, err := db.Query(ctx, burstSQL, table, f.Detector, day.Format(time.DateOnly), in.Locus)
		if err != nil {
			return nil, fmt.Errorf("while reading the deliveries of feed %s registered on %s: %w", table,
				day.Format(time.DateOnly), err)
		}
		b := Burst{Detector: f.Detector, Day: day}
		for rows.Next() {
			var s string
			var id *string
			if err := rows.Scan(&s, &id); err != nil {
				rows.Close()
				return nil, err
			}
			b.Deliveries++
			if s == session {
				continue
			}
			if id == nil {
				b.Unraised++
			} else {
				b.Others = append(b.Others, *id)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if b.Deliveries > 1 {
			sort.Strings(b.Others)
			out = append(out, b)
		}
	}
	return out, nil
}
