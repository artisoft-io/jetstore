package observe

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

// DeliverySize is the delivery_size detector: one delivery's size outside the
// range its feed declares, bounds inclusive, or, with no range declared,
// outside the feed's own trailing band. The basis says which.
//
// The size is file_key_staging.file_size for the delivery's file key. A
// delivery with no staging row is not judged: the sync that rebuilds that
// table lists what is in storage now, not what was registered.
//
// # The trailing band
//
// With no size_bytes, a delivery is judged against the sizes of the last
// BandDeliveries deliveries registered before it that have a size: their
// median, plus or minus the larger of BandSpread scaled median absolute
// deviations and MinBandWidth of the median. A delivery with fewer than
// MinBandDeliveries such predecessors is not judged and carries
// history_truncated. The four parameters are arguments, and their defaults are
// unsourced: no feed's size distribution has been measured to set them.
//
// The band follows the feed, so it accepts a change that arrives slowly: a
// cumulative feed growing by a few per cent a delivery stays inside it, and a
// feed that halves at once does not. The floor on the width keeps a feed whose
// sizes never vary from firing on every change of a few bytes.
type DeliverySize struct {
	Generation int
	// BandDeliveries is how many preceding sizes the band reads, at most.
	BandDeliveries int
	// MinBandDeliveries is how many it needs before it judges.
	MinBandDeliveries int
	// BandSpread is the band's half-width in scaled median absolute
	// deviations (1.4826 times the MAD, a standard deviation for normal data).
	BandSpread float64
	// MinBandWidth is the least half-width, as a share of the median.
	MinBandWidth float64
}

// DefaultDeliverySize is the first generation, with the band's defaults:
// eight preceding deliveries, at least four, three scaled MADs, and at least
// half the median either side. The defaults are unsourced.
func DefaultDeliverySize() DeliverySize {
	return DeliverySize{Generation: 1, BandDeliveries: 8, MinBandDeliveries: 4, BandSpread: 3, MinBandWidth: 0.5}
}

// Ref is the detector_ref: delivery_size@<generation>.
func (d DeliverySize) Ref() string {
	return fmt.Sprintf("%s@%d", feedprofile.DeliverySize, d.Generation)
}

// FeedDeliveryBand is the trailing band over prior sizes: its bounds,
// inclusive, and the median and median absolute deviation it was built from.
// It is defined for at least one size.
type FeedDeliveryBand struct {
	Min, Max    int64
	Median, MAD float64
	N           int
}

// Band computes the trailing band over the sizes given, which are the
// preceding deliveries' in registration order; it reads the last
// BandDeliveries of them. ok is false when there are fewer than
// MinBandDeliveries.
func (d DeliverySize) Band(prior []int64) (b FeedDeliveryBand, ok bool) {
	if d.BandDeliveries > 0 && len(prior) > d.BandDeliveries {
		prior = prior[len(prior)-d.BandDeliveries:]
	}
	if len(prior) == 0 || len(prior) < d.MinBandDeliveries {
		return FeedDeliveryBand{N: len(prior)}, false
	}
	xs := make([]float64, len(prior))
	for i, s := range prior {
		xs[i] = float64(s)
	}
	med := feedDeliveryMedian(xs)
	dev := make([]float64, len(xs))
	for i, x := range xs {
		dev[i] = math.Abs(x - med)
	}
	mad := feedDeliveryMedian(dev)
	half := math.Max(d.BandSpread*1.4826*mad, d.MinBandWidth*med)
	return FeedDeliveryBand{
		Min:    int64(math.Max(0, math.Floor(med-half))),
		Max:    int64(math.Ceil(med + half)),
		Median: med, MAD: mad, N: len(prior),
	}, true
}

func feedDeliveryMedian(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// Detect judges every delivery of every profiled feed.
func (d DeliverySize) Detect(in *FeedDeliveryInput) (FeedDeliveryResult, error) {
	judged, cov, err := feedDeliveryJudgeable(feedprofile.DeliverySize, in)
	res := FeedDeliveryResult{Detector: feedprofile.DeliverySize}
	if err != nil {
		return res, err
	}
	for _, f := range judged {
		declared := f.Profile.SizeBytes
		var prior []int64
		for _, x := range in.Deliveries[f.TableName] {
			v := FeedDeliveryVerdict{TableName: f.TableName, Ordinal: x.Ordinal, SessionId: x.SessionId,
				Verdict: FeedDeliverySilent}
			if x.SizeBytes == nil {
				v.Verdict, v.Reason = FeedDeliveryNotJudged, FeedDeliveryNoSize
				res.Verdicts = append(res.Verdicts, v)
				continue
			}
			size := *x.SizeBytes
			var lo, hi *int64
			var band FeedDeliveryBand
			if declared != nil {
				lo, hi = declared.Min, declared.Max
			} else {
				var ok bool
				band, ok = d.Band(prior)
				if !ok {
					v.Verdict, v.Reason, v.Confounders = FeedDeliveryNotJudged, FeedDeliveryShortHistory,
						feedDeliveryTruncated()
				}
				lo, hi = &band.Min, &band.Max
			}
			prior = append(prior, size)
			if v.Verdict == FeedDeliveryNotJudged {
				res.Verdicts = append(res.Verdicts, v)
				continue
			}
			var magnitude float64
			switch {
			case lo != nil && size < *lo:
				magnitude = 1
				if *lo > 0 {
					magnitude = float64(*lo-size) / float64(*lo)
				}
			case hi != nil && size > *hi:
				magnitude = float64(size - *hi)
				if *hi > 0 {
					magnitude /= float64(*hi)
				}
			default:
				res.Verdicts = append(res.Verdicts, v)
				continue
			}
			v.Verdict = FeedDeliveryFires
			v.AnomalyId = feedDeliveryAnomalyId(d.Ref(), f.TableName, x.SessionId)
			res.Verdicts = append(res.Verdicts, v)
			a := Anomaly{
				AnomalyId:          v.AnomalyId,
				DetectedAt:         in.AsOf,
				SessionId:          x.SessionId,
				SubjectType:        SubjectFeed,
				SubjectRef:         f.TableName,
				SignalType:         SignalVolume,
				ObservedValue:      strconv.FormatInt(size, 10),
				ExpectedBasis:      d.basis(f, x, band),
				DeviationMagnitude: &magnitude,
				Confounders:        []string{},
				DetectorRef:        d.Ref(),
			}
			if lo != nil {
				s := strconv.FormatInt(*lo, 10)
				a.ExpectedMin = &s
			}
			if hi != nil {
				s := strconv.FormatInt(*hi, 10)
				a.ExpectedMax = &s
			}
			res.Anomalies = append(res.Anomalies, a)
		}
	}
	return res.withCoverage(cov), nil
}

func (d DeliverySize) basis(f feedprofile.Feed, x FeedDelivery, band FeedDeliveryBand) string {
	var s string
	if r := f.Profile.SizeBytes; r != nil {
		s = fmt.Sprintf("%s gives one delivery's size as %s bytes, bounds inclusive", feedDeliveryDeclaration(f),
			feedDeliveryRange(r.Min, r.Max))
	} else {
		s = fmt.Sprintf("%s declares no size range, so the delivery is judged against the feed's trailing band: "+
			"the median of the %d preceding delivery sizes, %.0f bytes, plus or minus the larger of %g scaled "+
			"median absolute deviations (MAD %.0f) and %g of the median, giving %d to %d bytes. "+
			"The band's parameters are defaults nobody has measured",
			feedDeliveryDeclaration(f), band.N, band.Median, d.BandSpread, band.MAD, d.MinBandWidth,
			band.Min, band.Max)
	}
	return s + fmt.Sprintf(". The delivery of session %s, registered %s, is file %s of %d bytes",
		x.SessionId, x.Arrival.Format("2006-01-02T15:04:05Z07:00"), x.FileKey, *x.SizeBytes) +
		feedDeliveryRegistration(f, x)
}

func feedDeliveryRange(lo, hi *int64) string {
	switch {
	case lo != nil && hi != nil:
		return fmt.Sprintf("%d to %d", *lo, *hi)
	case lo != nil:
		return fmt.Sprintf("at least %d", *lo)
	default:
		return fmt.Sprintf("at most %d", *hi)
	}
}
