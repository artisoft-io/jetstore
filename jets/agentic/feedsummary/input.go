package feedsummary

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/rca"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// Input is everything a feed incident's summary is made from, and everything a
// model drafting the same summary would be given.
//
// # No field can carry a row
//
// Every field is a number, a date, a boolean, or a string drawn from one of
// three places: a closed vocabulary (the locus, a detector, a signal, a cause,
// a grade, a unit), the feed's source configuration (its table, client, org
// and object type), or the feed's profile (who declared it, its period type,
// the name of a required field). None is drawn from a delivery's rows. That is
// a property of the type rather than a filter on its contents:
// TestTheInputHasNoFieldThatCanCarryARow walks it by reflection and fails on
// any interface, map, byte slice or unlisted string.
//
// In particular the anomaly's own expected basis, which is prose the detector
// wrote, is not carried. Its numbers are, parsed into typed fields; the words
// are the template's.
type Input struct {
	// IncidentID is the incident's identifier: inc_<session>_<locus>.
	IncidentID string
	// Locus is one of triage.DeliveryLoci.
	Locus string
	// DetectedAt is when the incident was detected.
	DetectedAt time.Time

	Feed        Feed
	Declaration Declaration
	// Findings are the anomalies at this incident's locus on this feed,
	// in detector order.
	Findings []Finding
	// Readings are the incident's ranked hypotheses, best first.
	Readings []Reading
	// Condition is the feed's condition this incident is the incident of: the
	// deliveries it holds, from when to when, how it ended or that it is still
	// open, and the previous incident at the locus with its verdict. Read
	// through jets/agentic/feedcondition, the resolver shadow triage places a
	// finding with, so the summary and the writer cannot disagree about which
	// deliveries an incident holds. Nil when no condition of the feed holds
	// the incident's session.
	Condition *Condition
	// OtherFeeds counts the further feeds whose findings sit at this locus on
	// the same session. The summary describes one feed and says how many it
	// leaves out.
	OtherFeeds int
}

// Feed is the feed as its source configuration names it.
type Feed struct {
	// Table is source_config.table_name, the feed's identity.
	Table string
	// Configured reports whether a source_config row names the table. When it
	// is false the other identity fields are empty.
	Configured bool
	Client     string
	Org        string
	ObjectType string
	// Cumulative is source_config.is_cumulative.
	Cumulative bool
	// RegisteredUnder is the client a registration of this session names when
	// it is not the feed's own, as when a delivery is registered under a
	// placeholder client; "" otherwise.
	RegisteredUnder string
}

// Declaration is the feed's profile, as far as the summary reads it.
type Declaration struct {
	// Readable reports whether the stored profile parses and validates. When it
	// is false nothing else here is set, and the summary says the declaration
	// cannot be re-read.
	Readable bool
	// DeclaredBy and DeclaredAt are the profile's declared_by and declared_at.
	DeclaredBy string
	DeclaredAt time.Time
	// AgeDays is how old the declaration was when the incident was detected,
	// in whole days; meaningful only when DeclaredAt is set.
	AgeDays int
	// PeriodType is the profile's period_type, one of feedprofile.PeriodTypes,
	// or "" when the profile declares none.
	PeriodType string
	// PeriodsPerDelivery is the declared number of periods one delivery
	// covers; 0 when no period type is declared.
	PeriodsPerDelivery int
	// OverdueAfterDays is the declared grace; nil when none is declared.
	OverdueAfterDays *int
}

// Finding is one anomaly, parsed into numbers.
type Finding struct {
	// Detector is the detector's name without its generation, one of
	// feedprofile.Detectors.
	Detector   string
	Generation int
	// Signal is the anomaly's signal type, one of observe's signal vocabulary.
	Signal string
	// Column is the required field a completeness finding is about, as the
	// profile names it; "" for every other detector.
	Column string

	// Observed is what was measured, in ObservedUnit. Nil when the detector
	// measures a period rather than a quantity, or when the value did not parse.
	Observed     *float64
	ObservedUnit string
	// ObservedPeriod is the start of a period the detector observed (the
	// latest period delivered for an overdue feed, a back-dated delivery's own
	// period); zero otherwise.
	ObservedPeriod time.Time

	// Bound is the declared or expected limit Observed was compared with, in
	// BoundUnit. BoundIsMin says which side it bounds.
	Bound      *float64
	BoundUnit  string
	BoundIsMin bool
	// Upper is the second side of a two-sided band, in BoundUnit; nil when the
	// bound is one-sided.
	Upper *float64
	// BoundPeriod is the start of the period a back-dated delivery fell behind.
	BoundPeriod time.Time
	// OverdueFrom is when an overdue feed's next delivery became overdue.
	OverdueFrom time.Time

	// Equal reports a delivery whose key set equals its predecessor's.
	Equal bool
	// Magnitude is the anomaly's deviation magnitude; nil when it has none.
	Magnitude *float64
}

// Reading is one ranked hypothesis.
type Reading struct {
	Rank int
	// Cause is the hypothesis's cause class, one of rca's ten.
	Cause      string
	Confidence float64
	// Supporting and Contradicting count the evidence items on each side.
	Supporting    int
	Contradicting int
	// Evidenceability is the class's grade at this locus.
	Evidenceability string
}

// How the deliveries an incident holds end. A closed list, so that an end is
// never a value copied from a record.
const (
	// EndOpen: the incident holds its condition's last position, and that is
	// the feed's latest: the condition is still open.
	EndOpen = "open"
	// EndNoFinding, EndOtherFeed and EndDayBoundary are the condition's own
	// ends (triage.EndNoFinding and its siblings).
	EndNoFinding   = triage.EndNoFinding
	EndOtherFeed   = triage.EndOtherFeed
	EndDayBoundary = triage.EndDayBoundary
	// EndVerdict: the condition goes on, and its next delivery was found after
	// a person's verdict on this incident, so it is new evidence and not this
	// incident's.
	EndVerdict = "verdict"
	// EndNextIncident: the condition goes on, and its next delivery belongs to
	// another incident, one written before conditions, which is never moved.
	EndNextIncident = "next_incident"
	// EndUnordered: the session id is not a time, so the delivery has no place
	// in the feed's order and is a condition of one.
	EndUnordered = "unordered"
)

var ends = []string{EndOpen, EndNoFinding, EndOtherFeed, EndDayBoundary, EndVerdict, EndNextIncident, EndUnordered}

// What the summary names as the incident before this one at the locus.
const (
	// PrevNone: no earlier position of the feed carries a finding at the locus.
	PrevNone = "none"
	// PrevAfterVerdict: an earlier delivery of the same condition carries an
	// incident a person gave a verdict, and this incident's first delivery was
	// found after it.
	PrevAfterVerdict = "after_verdict"
	// PrevCondition: the feed's previous condition at the locus, with its
	// incident, or with none when it has not been raised yet.
	PrevCondition = "previous_condition"
)

var previousKinds = []string{"", PrevNone, PrevAfterVerdict, PrevCondition}

// Condition is the part of a feed condition one incident holds, as
// feedcondition resolves it when the summary is read.
//
// A condition is a maximal run of consecutive positions of one feed that each
// carry a finding of the feed at one locus (triage's feed_condition.go). Its
// incident is written on its first delivery and the later ones join it, so an
// incident usually holds the whole condition. It holds less when a person's
// verdict closed it and later deliveries are new evidence, or when incidents
// written one per delivery or per day before conditions sit on other
// positions of it: those are never moved, and each delivery joins the latest
// incident at or before it.
type Condition struct {
	// Held counts the condition's positions that belong to this incident: its
	// own delivery and those that join it. Unregistered counts those of them
	// that are sessions with no registration of the feed.
	Held         int
	Unregistered int
	// Total counts the condition's positions, held or not.
	Total int
	// First and Last are the arrivals of the first and last positions held,
	// the session ids read as epoch milliseconds; zero when End is
	// EndUnordered.
	First, Last time.Time
	// End says how the positions held end, one of the End constants.
	// EndedAt is the arrival of the position after the last one held, which
	// ended them; zero when End is EndOpen or EndUnordered. EndedInto is the
	// incident that position belongs to, for EndVerdict and EndNextIncident,
	// or "" when it has none yet.
	End       string
	EndedAt   time.Time
	EndedInto string
	// Verdict and VerdictAt are this incident's first verdict by a person, for
	// EndVerdict; "" and zero otherwise.
	Verdict   string
	VerdictAt time.Time
	// SameDay reports that part of the condition falls before the feed's
	// earliest surviving registration, where two findings are consecutive
	// only on one UTC day. Shared reports a condition of one on a session
	// carrying another feed's finding at the locus too.
	SameDay bool
	Shared  bool
	// Others are the other incidents the condition's positions belong to,
	// sorted; Unraised counts its positions that belong to no incident yet,
	// each found after a person's verdict on the incident before it, so new
	// evidence that triage has not reached.
	Others   []string
	Unraised int
	// Previous is the incident before this one at the locus.
	Previous Previous
}

// Previous is the incident at the locus on an earlier position of the feed,
// read when the summary is read: a condition raised before its predecessor,
// as a backfill triaged newest first raises it, names the predecessor's
// incident once it is written, which the incident's own basis cannot.
type Previous struct {
	// Kind is one of the Prev constants, or "" when the summary names none:
	// the delivery has no place in the feed's order, or the incident before
	// it in the same condition carries no verdict and is among Others.
	Kind string
	// IncidentID is the previous incident, or "" for a previous condition
	// that has none yet.
	IncidentID string
	// Verdict and VerdictAt are the previous incident's first verdict by a
	// person; "" and zero when it has none.
	Verdict   string
	VerdictAt time.Time
	// Deliveries, First and Last describe the previous condition, for
	// PrevCondition: how many positions it holds, and the arrivals of its
	// first and last.
	Deliveries  int
	First, Last time.Time
}

// Units the parser emits. A closed list, so that a unit is never a value
// copied from the anomaly.
const (
	UnitBytes          = "bytes"
	UnitDays           = "days"
	UnitCalendarMonths = "calendar months"
	UnitShare          = "share"
	UnitKeysAbsent     = "keys absent"
	UnitPeriods        = "periods"
)

var units = []string{"", UnitBytes, UnitDays, UnitCalendarMonths, UnitShare, UnitKeysAbsent, UnitPeriods}

var causes = []string{
	rca.CauseSourceDeliveryFailure, rca.CauseSourceContentChange, rca.CauseTransportFailure,
	rca.CauseParseFailure, rca.CauseValidationBreach, rca.CauseTransformationDefect,
	rca.CauseInfrastructureFailure, rca.CauseDependencyFailure, rca.CauseCapacityOrCostDeviation,
	rca.CauseBenignVariation,
}

var grades = []string{
	string(rca.Evidenced), string(rca.Conditional), string(rca.Coarse), string(rca.Asymmetric), string(rca.None),
}

var signals = []string{
	observe.SignalVolume, observe.SignalFreshness, observe.SignalSchema, observe.SignalDistribution,
	observe.SignalRuleBreach, observe.SignalCost, observe.SignalDuration, observe.SignalRejectionRate,
	observe.SignalCardinality, observe.SignalReferential, observe.SignalArrival,
}

// Validate checks every closed vocabulary. An Input that fails it is not
// rendered: a value outside a closed list is a value that came from somewhere
// it should not have.
func (in *Input) Validate() error {
	if in == nil {
		return fmt.Errorf("no input")
	}
	if !slices.Contains(triage.DeliveryLoci, in.Locus) {
		return fmt.Errorf("locus %q is not a delivery locus", in.Locus)
	}
	if want := "inc_"; !strings.HasPrefix(in.IncidentID, want) || !strings.HasSuffix(in.IncidentID, "_"+in.Locus) {
		return fmt.Errorf("incident id %q is not inc_<session>_%s", in.IncidentID, in.Locus)
	}
	if in.Declaration.PeriodType != "" && !slices.Contains(feedprofile.PeriodTypes, in.Declaration.PeriodType) {
		return fmt.Errorf("period type %q is not in the vocabulary", in.Declaration.PeriodType)
	}
	for _, f := range in.Findings {
		if !slices.Contains(feedprofile.Detectors, f.Detector) {
			return fmt.Errorf("detector %q is not a feed detector", f.Detector)
		}
		if !slices.Contains(signals, f.Signal) {
			return fmt.Errorf("signal %q is not in the vocabulary", f.Signal)
		}
		if !slices.Contains(units, f.ObservedUnit) || !slices.Contains(units, f.BoundUnit) {
			return fmt.Errorf("a unit of %s is not in the vocabulary", f.Detector)
		}
		if (f.Column != "") != (f.Detector == feedprofile.FieldCompleteness) {
			return fmt.Errorf("a column is carried by a completeness finding and by nothing else")
		}
	}
	for _, r := range in.Readings {
		if !slices.Contains(causes, r.Cause) {
			return fmt.Errorf("cause %q is not in the vocabulary", r.Cause)
		}
		if !slices.Contains(grades, r.Evidenceability) {
			return fmt.Errorf("grade %q is not in the vocabulary", r.Evidenceability)
		}
	}
	if c := in.Condition; c != nil {
		if !slices.Contains(ends, c.End) {
			return fmt.Errorf("condition end %q is not in the vocabulary", c.End)
		}
		if !slices.Contains(previousKinds, c.Previous.Kind) {
			return fmt.Errorf("previous incident kind %q is not in the vocabulary", c.Previous.Kind)
		}
		for _, v := range []string{c.Verdict, c.Previous.Verdict} {
			if v != "" && !slices.Contains(audit.VerdictStatuses, v) {
				return fmt.Errorf("verdict %q is not a verdict status", v)
			}
		}
		ids := append(slices.Clone(c.Others), c.EndedInto, c.Previous.IncidentID)
		for _, o := range ids {
			if o != "" && (!strings.HasPrefix(o, "inc_") || !strings.HasSuffix(o, "_"+in.Locus)) {
				return fmt.Errorf("condition incident %q is not an incident at %s", o, in.Locus)
			}
		}
	}
	return nil
}
