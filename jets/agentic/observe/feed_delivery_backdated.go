package observe

import (
	"fmt"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// DeliveryBackdated is the delivery_backdated detector: a delivery registered
// under a period that precedes the latest period of any delivery registered
// before it, at the profile's period type.
//
// It reads registration only, so it judges a feed whose claims are never
// loaded. It is not the span check: a delivery registered under a current
// period whose dates of service reach back is service_date_span's, and this
// detector stays silent on it. Whether a back-dated registration is a fault or
// a deliberate backfill is not knowable from the registration, and the basis
// does not say which.
//
// A delivery whose predecessors carry no period is not judged, and carries
// history_truncated: the purge may have removed what came before it.
type DeliveryBackdated struct {
	Generation int
}

// DefaultDeliveryBackdated is the first generation.
func DefaultDeliveryBackdated() DeliveryBackdated { return DeliveryBackdated{Generation: 1} }

// Ref is the detector_ref: delivery_backdated@<generation>.
func (d DeliveryBackdated) Ref() string {
	return fmt.Sprintf("%s@%d", feedprofile.DeliveryBackdated, d.Generation)
}

// Detect judges every delivery of every profiled feed that declares a period
// type, each against the deliveries registered before it.
func (d DeliveryBackdated) Detect(in *FeedDeliveryInput) (FeedDeliveryResult, error) {
	judged, cov, err := feedDeliveryJudgeable(feedprofile.DeliveryBackdated, in)
	res := FeedDeliveryResult{Detector: feedprofile.DeliveryBackdated}
	if err != nil {
		return res, err
	}
	for _, f := range judged {
		pt := f.Profile.PeriodType
		latest, latestSession := 0, ""
		for _, x := range in.Deliveries[f.TableName] {
			v := FeedDeliveryVerdict{TableName: f.TableName, Ordinal: x.Ordinal, SessionId: x.SessionId}
			switch {
			case x.Period == nil:
				v.Verdict, v.Reason = FeedDeliveryNotJudged, FeedDeliveryNoPeriod
			case latest == 0:
				v.Verdict, v.Reason, v.Confounders = FeedDeliveryNotJudged, FeedDeliveryNoPredecessor,
					feedDeliveryTruncated()
			case x.Period.Of(pt) < latest:
				v.Verdict = FeedDeliveryFires
				v.AnomalyId = feedDeliveryAnomalyId(d.Ref(), f.TableName, x.SessionId)
				behind := float64(latest - x.Period.Of(pt))
				expected := FeedDeliveryPeriodStart(pt, latest).Format("2006-01-02")
				res.Anomalies = append(res.Anomalies, Anomaly{
					AnomalyId:          v.AnomalyId,
					DetectedAt:         in.AsOf,
					SessionId:          x.SessionId,
					SubjectType:        SubjectFeed,
					SubjectRef:         f.TableName,
					SignalType:         SignalDistribution,
					ObservedValue:      FeedDeliveryPeriodStart(pt, x.Period.Of(pt)).Format("2006-01-02"),
					ExpectedMin:        &expected,
					ExpectedBasis:      d.basis(f, x, latest, latestSession),
					DeviationMagnitude: &behind,
					Confounders:        []string{},
					DetectorRef:        d.Ref(),
				})
			default:
				v.Verdict = FeedDeliverySilent
			}
			res.Verdicts = append(res.Verdicts, v)
			if x.Period != nil && x.Period.Of(pt) > latest {
				latest, latestSession = x.Period.Of(pt), x.SessionId
			}
		}
	}
	return res.withCoverage(cov), nil
}

func (d DeliveryBackdated) basis(f feedprofile.Feed, x FeedDelivery, latest int, latestSession string) string {
	pt := f.Profile.PeriodType
	return fmt.Sprintf("%s counts deliveries in %s. This delivery, registered %s, names %s (source period "+
		"dated %s), %d %s(s) before %s, the latest period of any delivery registered before it (session %s). "+
		"Registration alone does not say whether it is a fault or a deliberate backfill",
		feedDeliveryDeclaration(f), pt, x.Arrival.Format("2006-01-02T15:04:05Z07:00"),
		feedDeliveryPeriodName(pt, x.Period.Of(pt)), x.Period.Date.Format("2006-01-02"),
		latest-x.Period.Of(pt), pt, feedDeliveryPeriodName(pt, latest), latestSession) +
		feedDeliveryRegistration(f, x)
}
