package feedprofile

import (
	"fmt"
	"slices"
)

// The eight feed detectors, by the names their anomalies and coverage lines
// carry. Three judge a delivery from its registration alone; five read the
// delivery's claims in the declared claim-set table.
const (
	FeedOverdue       = "feed_overdue"
	DeliverySize      = "delivery_size"
	DeliveryBackdated = "delivery_backdated"
	ServiceDateStale  = "service_date_stale"
	ServiceDateSpan   = "service_date_span"
	FieldCompleteness = "field_completeness"
	DeliveryOverlap   = "delivery_overlap"
	DeliveryUnchanged = "delivery_unchanged"
)

// Detectors lists the eight feed detectors in the order the documentation
// gives them.
var Detectors = []string{
	FeedOverdue, DeliverySize, DeliveryBackdated, ServiceDateStale,
	ServiceDateSpan, FieldCompleteness, DeliveryOverlap, DeliveryUnchanged,
}

// The profile fields a detector may need, by their JSON names.
const (
	FieldPeriodType             = "period_type"
	FieldOverdueAfterDays       = "overdue_after_days"
	FieldClaimSet               = "claim_set"
	FieldMaxServiceDateLagDays  = "max_service_date_lag_days"
	FieldMaxServiceDateSpanDays = "max_service_date_span_days"
	FieldMaxRedeliveredShare    = "max_redelivered_share"
	FieldRequiredFields         = "required_fields"
)

// requirement is one detector's row of the table: the fields it needs on every
// feed, the fields it needs on an incremental feed only, and whether it judges
// a cumulative feed at all.
//
// The fields are in the order a coverage line should name them: the
// detector's own threshold first, claim_set last, so that a profile lacking
// both is reported by the field that is specific to the detector.
type requirement struct {
	always      []string
	incremental []string
	cumulative  bool
}

// required is the table every feed detector and its coverage line read. A
// detector whose fields are absent does not judge the feed; it does not judge
// it with defaults.
//
// delivery_size needs no field: without size_bytes it judges a delivery
// against the feed's own trailing band of sizes, and its basis says so.
// periods_per_delivery is never required, because it defaults to 1.
var required = map[string]requirement{
	FeedOverdue:       {always: []string{FieldPeriodType, FieldOverdueAfterDays}, cumulative: true},
	DeliverySize:      {cumulative: true},
	DeliveryBackdated: {always: []string{FieldPeriodType}, cumulative: true},
	ServiceDateStale:  {always: []string{FieldMaxServiceDateLagDays, FieldClaimSet}, cumulative: true},
	ServiceDateSpan:   {always: []string{FieldMaxServiceDateSpanDays, FieldClaimSet}, cumulative: false},
	FieldCompleteness: {always: []string{FieldRequiredFields, FieldClaimSet}, cumulative: true},
	DeliveryOverlap: {always: []string{FieldClaimSet}, incremental: []string{FieldMaxRedeliveredShare},
		cumulative: true},
	DeliveryUnchanged: {always: []string{FieldClaimSet}, cumulative: true},
}

// Applies reports whether a detector judges a feed of this kind at all. Only
// service_date_span does not judge a cumulative feed, whose deliveries span its
// whole history by design. A detector that does not apply reports the feed as
// not judged for that reason, not for a missing field.
func Applies(detector string, cumulative bool) (bool, error) {
	r, ok := required[detector]
	if !ok {
		return false, unknown(detector)
	}
	return !cumulative || r.cumulative, nil
}

// Required returns the profile fields a detector needs to judge a feed, by
// their JSON names. Whether the feed is cumulative matters because
// delivery_overlap needs max_redelivered_share on an incremental feed only.
func Required(detector string, cumulative bool) ([]string, error) {
	r, ok := required[detector]
	if !ok {
		return nil, unknown(detector)
	}
	fields := slices.Clone(r.incremental)
	if cumulative {
		fields = nil
	}
	// The detector's own threshold first, then claim_set.
	out := make([]string, 0, len(r.always)+len(fields))
	for _, f := range r.always {
		if f != FieldClaimSet {
			out = append(out, f)
		}
	}
	out = append(out, fields...)
	if slices.Contains(r.always, FieldClaimSet) {
		out = append(out, FieldClaimSet)
	}
	return out, nil
}

// Missing returns the fields a detector needs and the profile lacks, by their
// JSON names, in the order Required gives them. An empty result means the
// detector can judge the feed. The first field is the one a coverage line
// names when it gives one reason.
func Missing(detector string, p Profile, cumulative bool) ([]string, error) {
	fields, err := Required(detector, cumulative)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range fields {
		if !present(f, p) {
			out = append(out, f)
		}
	}
	return out, nil
}

// present reports whether the profile declares a field. A pointer field is
// present when it is set; required_fields when it names at least one column.
func present(field string, p Profile) bool {
	switch field {
	case FieldPeriodType:
		return p.PeriodType != ""
	case FieldOverdueAfterDays:
		return p.OverdueAfterDays != nil
	case FieldClaimSet:
		return p.ClaimSet != nil
	case FieldMaxServiceDateLagDays:
		return p.MaxServiceDateLagDays != nil
	case FieldMaxServiceDateSpanDays:
		return p.MaxServiceDateSpanDays != nil
	case FieldMaxRedeliveredShare:
		return p.MaxRedeliveredShare != nil
	case FieldRequiredFields:
		return len(p.RequiredFields) > 0
	}
	panic(fmt.Sprintf("feedprofile: the required-field table names %q, which present does not know", field))
}

func unknown(detector string) error {
	return fmt.Errorf("%q is not a feed detector; the feed detectors are %v", detector, Detectors)
}
