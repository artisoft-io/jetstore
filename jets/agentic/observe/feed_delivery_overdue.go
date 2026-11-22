package observe

import (
	"fmt"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// FeedOverdue is the feed_overdue detector: no delivery registered for the
// period a delivery was due in, past the declared grace.
//
// It counts from the latest period the feed has delivered, the greatest period
// over its deliveries at the profile's period type, not from the period of the
// last registration: a late registration for an old period does not make a
// feed that is up to date look overdue. The next delivery is due in that
// period plus periods_per_delivery, and the feed is overdue from the end of
// the due period plus overdue_after_days.
//
// It judges a feed at the as-of time, so it reaches one verdict per feed, and
// the finding hangs on the session of the feed's last registered delivery: the
// one after which nothing came. Its identifier is that session's, so a daily
// run over a feed that stays overdue writes one anomaly, and a feed that
// resumes and stops again writes a second.
type FeedOverdue struct {
	// Generation is the trailing number of the detector_ref.
	Generation int
}

// DefaultFeedOverdue is the first generation.
func DefaultFeedOverdue() FeedOverdue { return FeedOverdue{Generation: 1} }

// Ref is the detector_ref: feed_overdue@<generation>.
func (d FeedOverdue) Ref() string { return fmt.Sprintf("%s@%d", feedprofile.FeedOverdue, d.Generation) }

// FeedOverdueFrom is the first instant a feed whose latest delivered period is
// latest is overdue, under its profile.
func FeedOverdueFrom(p feedprofile.Profile, latest int) time.Time {
	grace := 0
	if p.OverdueAfterDays != nil {
		grace = *p.OverdueAfterDays
	}
	return FeedDeliveryPeriodStart(p.PeriodType, latest+p.DeliveryPeriods()+1).AddDate(0, 0, grace)
}

// Detect judges every profiled feed that declares a period type and a grace.
func (d FeedOverdue) Detect(in *FeedDeliveryInput) (FeedDeliveryResult, error) {
	judged, cov, err := feedDeliveryJudgeable(feedprofile.FeedOverdue, in)
	res := FeedDeliveryResult{Detector: feedprofile.FeedOverdue}
	if err != nil {
		return res, err
	}
	var still []string
	for _, f := range judged {
		ds := in.Deliveries[f.TableName]
		pt := f.Profile.PeriodType
		latest, latestAt := 0, -1
		for i, x := range ds {
			if x.Period != nil && x.Period.Of(pt) > latest {
				latest, latestAt = x.Period.Of(pt), i
			}
		}
		if len(ds) == 0 || latestAt < 0 {
			reason := FeedDeliveryNoDelivery
			if len(ds) > 0 {
				reason = FeedDeliveryNoPeriod
			}
			// Nothing to count from, and no session to hang a finding on. The
			// purge may have taken the feed's history: it is not judged rather
			// than judged overdue.
			cov.NotJudged = append(cov.NotJudged, FeedDeliveryNotJudgedFeed{TableName: f.TableName,
				Client: f.Client, Org: f.Org, Reason: reason})
			continue
		}
		still = append(still, f.TableName)
		last := ds[len(ds)-1]
		from := FeedOverdueFrom(f.Profile, latest)
		v := FeedDeliveryVerdict{TableName: f.TableName, Ordinal: last.Ordinal, SessionId: last.SessionId,
			Verdict: FeedDeliverySilent}
		if in.AsOf.Before(from) {
			res.Verdicts = append(res.Verdicts, v)
			continue
		}
		v.Verdict = FeedDeliveryFires
		v.AnomalyId = feedDeliveryAnomalyId(d.Ref(), f.TableName, last.SessionId)
		res.Verdicts = append(res.Verdicts, v)

		days := in.AsOf.Sub(from).Hours() / 24
		owed := from.Format(time.RFC3339)
		res.Anomalies = append(res.Anomalies, Anomaly{
			AnomalyId:          v.AnomalyId,
			DetectedAt:         in.AsOf,
			SessionId:          last.SessionId,
			SubjectType:        SubjectFeed,
			SubjectRef:         f.TableName,
			SignalType:         SignalArrival,
			ObservedValue:      FeedDeliveryPeriodStart(pt, latest).Format(time.DateOnly),
			ExpectedMax:        &owed,
			ExpectedBasis:      d.basis(f, latest, ds[latestAt], last, from, in.AsOf),
			DeviationMagnitude: &days,
			Confounders:        []string{},
			DetectorRef:        d.Ref(),
		})
	}
	cov.Judged = still
	return res.withCoverage(cov), nil
}

func (d FeedOverdue) basis(f feedprofile.Feed, latest int, latestDelivery, last FeedDelivery,
	from, asOf time.Time) string {
	p := f.Profile
	s := fmt.Sprintf("%s says a delivery is due every %d %s with %d days' grace. "+
		"The latest period delivered is %s (session %s), so the next was due in %s and the feed is "+
		"overdue from %s; at %s nothing later had been registered. Its last registration is session %s, "+
		"registered %s",
		feedDeliveryDeclaration(f), p.DeliveryPeriods(), p.PeriodType, *p.OverdueAfterDays,
		feedDeliveryPeriodName(p.PeriodType, latest), latestDelivery.SessionId,
		feedDeliveryPeriodName(p.PeriodType, latest+p.DeliveryPeriods()),
		from.Format(time.RFC3339), asOf.Format(time.RFC3339), last.SessionId, last.Arrival.Format(time.RFC3339))
	if last.Period != nil && last.Period.Of(p.PeriodType) < latest {
		s += fmt.Sprintf(", under the older %s", feedDeliveryPeriodName(p.PeriodType, last.Period.Of(p.PeriodType)))
	}
	return s + feedDeliveryRegistration(f, last)
}
