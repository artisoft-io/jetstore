package feedsummary

import (
	"fmt"
	"slices"
	"strings"
	"time"

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
	// Bursts are, per detector that fired here, the other deliveries of the
	// same feed registered the same day on which that detector also fired.
	// Empty when there are none.
	Bursts []Burst
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

// Burst is a detector that fired on more than one delivery of the feed
// registered the same day.
type Burst struct {
	Detector string
	// Day is the registration day, UTC.
	Day time.Time
	// Deliveries counts the deliveries of the feed registered that day on
	// which the detector fired, this one included.
	Deliveries int
	// Others are the incidents raised on the other deliveries, sorted: a
	// delivery of the burst keeps an incident of its own when another finding
	// at the locus, outside the burst, holds on it.
	Others []string
	// Unraised counts the other deliveries with no incident of their own,
	// which a burst raised as a single incident leaves all of them.
	Unraised int
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
	for _, b := range in.Bursts {
		if !slices.Contains(feedprofile.Detectors, b.Detector) {
			return fmt.Errorf("burst detector %q is not a feed detector", b.Detector)
		}
		for _, o := range b.Others {
			if !strings.HasPrefix(o, "inc_") || !strings.HasSuffix(o, "_"+in.Locus) {
				return fmt.Errorf("burst incident %q is not an incident at %s", o, in.Locus)
			}
		}
	}
	return nil
}
