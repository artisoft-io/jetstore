package rca

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// A feed hypothesis's case rests on a declaration, and says so on both sides.
//
// **For.** Two of the floor's classes are graded Conditional on a feed's
// profile, and where the condition holds the profile itself is the evidence:
//
//   - source_delivery_failure at delivery_overdue is evidenced by the declared
//     cadence. The period type, the periods per delivery and the grace are what
//     was due, which is exactly the record the table said nothing kept.
//   - source_content_change at delivery_departs_profile is evidenced at column
//     grain by a column the profile names, where the check that fired compared
//     that column: a required field, the service date, the business key.
//
// **Against, on every feed hypothesis.** Every detector judges a delivery
// against what a person wrote down, and a profile written once and never
// revisited goes stale the day the feed legitimately changes. So every
// hypothesis at a delivery locus, whatever its class, carries the
// declaration's author and date, and how old it was when the finding was
// detected: the judgement is only as good as the declaration. A feed whose
// profile can no longer be read carries that instead, because a case against
// that silently disappeared with the profile would read as a stronger case.
//
// The feed is named by its source configuration, client included, never by the
// client a delivery was registered under (triage.DeclaredFeed.Name).

// declared is one hypothesis's declaration evidence.
type declared struct {
	supporting, against []Evidence
	// conditionMet reports that the class is Conditional at this locus and the
	// case for has shown its condition to hold, so the table's generic
	// "conditional" item would be a second, stale item against.
	conditionMet bool
}

// declarationEvidence builds the declaration evidence for one hypothesis at a
// delivery locus, from the anomalies placed there and the feeds triage read.
// It returns nothing at a run locus.
func declarationEvidence(in *Input, locus string, c CauseClass, placed []observe.Anomaly) declared {
	var out declared
	if !slices.Contains(triage.DeliveryLoci, locus) {
		return out
	}
	var fe *triage.FeedEvidence
	if in.Evidence != nil {
		fe = in.Evidence.Feed
	}
	if len(placed) == 0 {
		// A caller that supplied triage's evidence and not the anomaly rows
		// still gets the declaration: the anomalies triage placed are the same.
		placed = fe.At(locus)
	}
	for _, table := range tablesOf(placed) {
		d := fe.Feed(table)
		if d == nil {
			d = &triage.DeclaredFeed{TableName: table}
		}
		ref := fmt.Sprintf("jetsapi.source_config.feed_profile_json for table %s", table)
		if !d.Profiled {
			out.against = append(out.against, Evidence{
				Statement: fmt.Sprintf("the finding was judged against a declaration for %s that cannot be "+
					"re-read now, so neither its author nor its date can be weighed: the judgement is only as "+
					"good as a declaration this ranking cannot see", d.Name()),
				Source:    SourceProfile,
				SourceRef: ref,
			})
			continue
		}
		p := d.Feed.Profile
		out.against = append(out.against, Evidence{
			Statement: declarationStatement(d, latestDetection(placed, table)),
			Source:    SourceProfile,
			SourceRef: ref,
		})
		switch {
		case c.Name == CauseSourceDeliveryFailure && locus == triage.LocusDeliveryOverdue:
			if p.PeriodType == "" || p.OverdueAfterDays == nil {
				continue
			}
			out.conditionMet = true
			out.supporting = append(out.supporting, Evidence{
				Statement: fmt.Sprintf("the declared cadence is what was due: the profile of %s declares a "+
					"delivery every %d %s with %d days' grace, and the overdue check compared the feed's "+
					"registrations with it", d.Name(), p.DeliveryPeriods(), p.PeriodType, *p.OverdueAfterDays),
				Source:    SourceProfile,
				SourceRef: ref,
			})
		case c.Name == CauseSourceContentChange && locus == triage.LocusDeliveryDepartsProfile:
			for _, a := range placed {
				if a.SubjectRef != table {
					continue
				}
				if cols := namedColumns(p, a); len(cols) > 0 {
					out.conditionMet = true
					out.supporting = append(out.supporting, Evidence{
						Statement: fmt.Sprintf("the change is evidenced at column grain: the profile of %s "+
							"names %s, and %s compared the delivery on it", d.Name(),
							strings.Join(cols, ", "), detectorOf(a)),
						Source:    SourceProfile,
						SourceRef: fmt.Sprintf("%s; jetsapi.anomaly %s", ref, a.AnomalyId),
					})
				}
			}
		}
	}
	return out
}

// declarationStatement is the item against: who declared, when, and how old the
// declaration was when the finding was detected.
func declarationStatement(d *triage.DeclaredFeed, detected time.Time) string {
	p := d.Feed.Profile
	age := ""
	if at, ok := d.DeclaredAt(); ok && !detected.IsZero() {
		age = fmt.Sprintf(", %d days before the finding was detected", int(detected.Sub(at).Hours()/24))
	}
	return fmt.Sprintf("the judgement is only as good as the declaration it compares against: the profile of %s "+
		"was declared by %s on %s%s. A departure from a declaration that has gone stale is not a fault in "+
		"the feed", d.Name(), p.DeclaredBy, p.DeclaredAt, age)
}

// namedColumns returns the profile's columns the anomaly's detector compared,
// quoted, or nothing when the detector compares no column the profile names.
func namedColumns(p feedprofile.Profile, a observe.Anomaly) []string {
	q := func(s string) string { return fmt.Sprintf("%q", s) }
	switch detectorOf(a) {
	case feedprofile.FieldCompleteness:
		// The anomaly's identifier ends in the column it judged.
		for _, f := range p.RequiredFields {
			if strings.HasSuffix(a.AnomalyId, "/"+f.Column) {
				return []string{"required field " + q(f.Column)}
			}
		}
	case feedprofile.ServiceDateStale, feedprofile.ServiceDateSpan:
		if p.ClaimSet != nil && p.ClaimSet.ServiceDateColumn != "" {
			return []string{"service date column " + q(p.ClaimSet.ServiceDateColumn)}
		}
	case feedprofile.DeliveryOverlap, feedprofile.DeliveryUnchanged:
		if p.ClaimSet != nil && len(p.ClaimSet.KeyColumns) > 0 {
			keys := make([]string, len(p.ClaimSet.KeyColumns))
			for i, k := range p.ClaimSet.KeyColumns {
				keys[i] = q(k)
			}
			return []string{"business key " + strings.Join(keys, ", ")}
		}
	}
	return nil
}

// detectorOf is the detector name of an anomaly: its detector_ref before the
// generation.
func detectorOf(a observe.Anomaly) string {
	name, _, _ := strings.Cut(a.DetectorRef, "@")
	return name
}

// tablesOf returns the feeds the anomalies concern, sorted, once each.
func tablesOf(as []observe.Anomaly) []string {
	var out []string
	for _, a := range as {
		if a.SubjectType == observe.SubjectFeed && !slices.Contains(out, a.SubjectRef) {
			out = append(out, a.SubjectRef)
		}
	}
	slices.Sort(out)
	return out
}

// latestDetection is when the latest of a feed's anomalies was detected.
func latestDetection(as []observe.Anomaly, table string) time.Time {
	var t time.Time
	for _, a := range as {
		if a.SubjectRef == table && a.DetectedAt.After(t) {
			t = a.DetectedAt
		}
	}
	return t
}
