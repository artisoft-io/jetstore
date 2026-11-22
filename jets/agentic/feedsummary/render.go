package feedsummary

import (
	_ "embed"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/template"
)

// Document is the template document, verbatim.
//
//go:embed feed_incident_summary.json
var Document []byte

// Key is the name the document registers under.
const Key = "feed_incident_summary"

var (
	once     sync.Once
	compiled *template.Template
	compErr  error
)

// Template returns the compiled document. A document that does not compile is
// a defect of this package and TestTheDocumentCompiles is what catches it; the
// error is still returned rather than panicking, so that a reader of an
// incident is shown the incident without its summary rather than nothing.
func Template() (*template.Template, error) {
	once.Do(func() {
		compiled, compErr = template.CompileJSON(Document)
		if compErr == nil && compiled.Key() != Key {
			compErr = fmt.Errorf("the summary template registers as %q, want %q", compiled.Key(), Key)
		}
	})
	return compiled, compErr
}

// Summary is a rendered summary and what the render could not say.
type Summary struct {
	Text string
	// Violations are the template's own require assertions that the input did
	// not satisfy. The document declares none today, so this is empty; it is
	// carried so that adding one is a change to the document only.
	Violations []string
	// RenderedBy names what produced Text. It is always the template.
	RenderedBy string
}

// RenderedBy is the value of Summary.RenderedBy.
const RenderedBy = "template " + Key

// Render renders the summary of one feed incident.
//
// It returns an error only when the input fails Validate or the document does
// not compile; the render itself cannot fail.
func Render(in *Input) (*Summary, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	t, err := Template()
	if err != nil {
		return nil, err
	}
	text, vs := t.Render(in.Document())
	out := &Summary{Text: text, RenderedBy: RenderedBy}
	for _, v := range vs {
		out.Violations = append(out.Violations, v.String())
	}
	return out, nil
}

const yes = "yes"

func flag(m map[string]any, key string, b bool) {
	if b {
		m[key] = yes
	}
}

func setDate(m map[string]any, key string, t time.Time) {
	if !t.IsZero() {
		m[key] = t.UTC().Format(time.DateOnly)
	}
}

// periodWord is the word a period type is counted in.
func periodWord(pt string) string {
	switch pt {
	case feedprofile.MonthPeriod:
		return "month"
	case feedprofile.WeekPeriod:
		return "week"
	case feedprofile.DayPeriod:
		return "day"
	}
	return ""
}

// formatCount writes a whole number with thousands separators.
func formatCount(x float64) string {
	n := int64(math.Round(x))
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// formatShare writes a share as a percentage to one decimal place.
func formatShare(x float64) string {
	return strconv.FormatFloat(x*100, 'f', 1, 64) + "%"
}

func formatIn(x float64, unit string) string {
	if unit == UnitShare {
		return formatShare(x)
	}
	return formatCount(x)
}

// othersShown caps the incidents a burst names; the rest are counted.
const othersShown = 10

// Document is the input as the template reads it: a decoded document whose
// property names are the template's paths. Numbers that a sentence prints are
// formatted here (a share as a percentage, a count with separators), and every
// word is the template's.
func (in *Input) Document() map[string]any {
	m := map[string]any{
		"Incident_Id": in.IncidentID,
		"Locus":       in.Locus,
		"Feed_Table":  in.Feed.Table,
	}
	setDate(m, "Detected_At", in.DetectedAt)
	if in.Feed.Configured {
		m["Feed_Configured"] = yes
		m["Feed_Client"] = in.Feed.Client
		if in.Feed.Org != "" {
			m["Feed_Org"] = in.Feed.Org
		}
		m["Feed_Object_Type"] = in.Feed.ObjectType
		flag(m, "Feed_Cumulative", in.Feed.Cumulative)
		if in.Feed.RegisteredUnder != "" {
			m["Registered_Under"] = in.Feed.RegisteredUnder
		}
	}
	d := in.Declaration
	pw := periodWord(d.PeriodType)
	if d.Readable {
		m["Declaration_Readable"] = yes
		m["Declared_By"] = d.DeclaredBy
		if !d.DeclaredAt.IsZero() {
			setDate(m, "Declared_At", d.DeclaredAt)
			// A summary is rendered at read time, so the profile it reads may
			// have been declared again after the incident was found.
			if d.AgeDays < 0 {
				m["Declared_After"] = yes
			} else {
				m["Declaration_Age_Days"] = d.AgeDays
			}
		}
		if pw != "" {
			m["Period_Word"] = pw
			m["Periods_Per_Delivery"] = d.PeriodsPerDelivery
		}
		if d.OverdueAfterDays != nil {
			m["Overdue_After_Days"] = *d.OverdueAfterDays
		}
	}

	findings := make([]any, 0, len(in.Findings))
	for _, f := range in.Findings {
		findings = append(findings, findingNode(f, pw, in.DetectedAt))
	}
	m["has_Findings"] = findings

	readings := make([]any, 0, len(in.Readings))
	for _, r := range in.Readings {
		readings = append(readings, map[string]any{
			"Rank":            r.Rank,
			"Cause":           r.Cause,
			"Confidence_Text": strconv.FormatFloat(r.Confidence, 'f', 2, 64),
			"Supporting":      r.Supporting,
			"Contradicting":   r.Contradicting,
			"Grade":           r.Evidenceability,
		})
	}
	m["has_Readings"] = readings

	bursts := make([]any, 0, len(in.Bursts))
	for _, b := range in.Bursts {
		n := map[string]any{"Detector": b.Detector, "Deliveries": b.Deliveries}
		setDate(n, "Day", b.Day)
		shown := b.Others
		if len(shown) > othersShown {
			n["Others_More"] = len(shown) - othersShown
			shown = shown[:othersShown]
		}
		if len(shown) > 0 {
			n["Others_Text"] = strings.Join(shown, ", ")
		}
		if b.Unraised > 0 {
			n["Unraised"] = b.Unraised
		}
		bursts = append(bursts, n)
	}
	m["has_Bursts"] = bursts
	if in.OtherFeeds > 0 {
		m["Other_Feeds"] = in.OtherFeeds
	}
	return m
}

// findingNode is one finding as the template reads it. Parsed is set when the
// numbers its detector's sentence needs are all present; otherwise the
// template says the check fired and what it measured could not be read.
func findingNode(f Finding, periodWord string, detected time.Time) map[string]any {
	n := map[string]any{"Detector": f.Detector}
	if periodWord != "" {
		n["Period_Word"] = periodWord
	}
	if f.Column != "" {
		n["Column"] = f.Column
	}
	if f.ObservedUnit != "" {
		n["Unit"] = f.ObservedUnit
	}
	if f.BoundUnit != "" {
		n["Bound_Unit"] = f.BoundUnit
	}
	if f.Observed != nil {
		n["Observed_Text"] = formatIn(*f.Observed, f.ObservedUnit)
	}
	if f.Bound != nil {
		n["Bound_Text"] = formatIn(*f.Bound, f.BoundUnit)
	}
	setDate(n, "Observed_Period", f.ObservedPeriod)
	setDate(n, "Bound_Period", f.BoundPeriod)
	parsed := false
	switch f.Detector {
	case feedprofile.FeedOverdue:
		parsed = !f.ObservedPeriod.IsZero()
		setDate(n, "Overdue_From", f.OverdueFrom)
		if !f.OverdueFrom.IsZero() && !detected.IsZero() && detected.After(f.OverdueFrom) {
			n["Days_Overdue"] = int(detected.Sub(f.OverdueFrom).Hours() / 24)
		}
	case feedprofile.DeliverySize:
		var lo, hi *float64
		if f.Bound != nil && f.BoundIsMin {
			lo, hi = f.Bound, f.Upper
		} else {
			hi = f.Bound
		}
		if lo != nil {
			n["Min_Text"] = formatCount(*lo)
		}
		if hi != nil {
			n["Max_Text"] = formatCount(*hi)
		}
		below := f.Observed != nil && lo != nil && *f.Observed < *lo
		above := f.Observed != nil && hi != nil && *f.Observed > *hi
		flag(n, "Below", below)
		flag(n, "Above", above)
		parsed = below || above
	case feedprofile.DeliveryBackdated:
		parsed = !f.ObservedPeriod.IsZero() && !f.BoundPeriod.IsZero()
		if f.Observed != nil && *f.Observed > 0 {
			n["Periods_Behind"] = int(math.Round(*f.Observed))
		}
	case feedprofile.ServiceDateStale, feedprofile.ServiceDateSpan:
		parsed = f.Observed != nil && f.Bound != nil
	case feedprofile.FieldCompleteness:
		parsed = f.Observed != nil && f.Bound != nil && f.Column != ""
	case feedprofile.DeliveryOverlap:
		parsed = f.Observed != nil && (f.ObservedUnit == UnitKeysAbsent || f.Bound != nil)
	case feedprofile.DeliveryUnchanged:
		parsed = f.Equal
	}
	flag(n, "Parsed", parsed)
	return n
}
