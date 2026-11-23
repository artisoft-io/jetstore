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
// BandDeliveries deliveries registered before it that have a non-zero size:
// their median, plus or minus the larger of BandSpread scaled median absolute
// deviations and MinBandWidth of the median. A delivery with fewer than
// MinBandDeliveries such predecessors is not judged and carries
// history_truncated. The four parameters are arguments, and their defaults are
// unsourced: no feed's size distribution has been measured to set them.
//
// A size of 0 is left out of the band because it does not say what a feed
// delivers: file_key_staging.file_size is NOT NULL DEFAULT 0, so a 0 is an
// empty file or a size never recorded. Read into the band, a feed whose
// recorded sizes are half or more 0 had a median and a MAD of 0, a band of 0
// to 0 bytes, and every non-empty delivery fired against a maximum of 0 bytes;
// two feeds in production did exactly that. A delivery whose own size is 0 is
// still judged, against the band of the non-zero sizes before it, so an empty
// delivery from a feed that is normally full fires below the band. The basis
// says how many zero sizes the band left out.
//
// The band follows the feed, so it accepts a change that arrives slowly: a
// cumulative feed growing by a few per cent a delivery stays inside it, and a
// feed that halves at once does not. The floor on the width keeps a feed whose
// sizes never vary from firing on every change of a few bytes.
type DeliverySize struct {
	Generation int
	// BandDeliveries is how many preceding non-zero sizes the band reads, at
	// most.
	BandDeliveries int
	// MinBandDeliveries is how many non-zero sizes it needs before it judges.
	MinBandDeliveries int
	// BandSpread is the band's half-width in scaled median absolute
	// deviations (1.4826 times the MAD, a standard deviation for normal data).
	BandSpread float64
	// MinBandWidth is the least half-width, as a share of the median.
	MinBandWidth float64
}

// DefaultDeliverySize is the first generation, with the band's defaults:
// eight preceding non-zero sizes, at least four, three scaled MADs, and at
// least half the median either side. The defaults are unsourced.
//
// Leaving the sizes of 0 out of the band did not start a new generation: it
// changes no verdict on a feed that never records a 0, and on one that does it
// moves the band onto the sizes the feed delivers, withdrawing the findings
// against a band of 0 bytes, which said nothing about the feed. A new generation would derive a second identifier for every finding
// already written, on every feed.
func DefaultDeliverySize() DeliverySize {
	return DeliverySize{Generation: 1, BandDeliveries: 8, MinBandDeliveries: 4, BandSpread: 3, MinBandWidth: 0.5}
}

// Ref is the detector_ref: delivery_size@<generation>.
func (d DeliverySize) Ref() string {
	return fmt.Sprintf("%s@%d", feedprofile.DeliverySize, d.Generation)
}

// FeedDeliveryBand is the trailing band over prior non-zero sizes: its
// bounds, inclusive, the median and median absolute deviation it was built
// from, and how many sizes it read. Zeros counts the sizes of 0 it left out:
// those registered after the oldest size it reads, or all of them when it
// reads every non-zero size there is. It is defined for at least one size.
type FeedDeliveryBand struct {
	Min, Max    int64
	Median, MAD float64
	N           int
	Zeros       int
}

// Band computes the trailing band over the sizes given, which are the
// preceding deliveries' in registration order. It leaves out the sizes of 0
// and reads the last BandDeliveries of the rest. ok is false when fewer than
// MinBandDeliveries non-zero sizes remain.
func (d DeliverySize) Band(prior []int64) (b FeedDeliveryBand, ok bool) {
	var xs []float64
	zeros := 0
	for i := len(prior) - 1; i >= 0; i-- {
		if d.BandDeliveries > 0 && len(xs) == d.BandDeliveries {
			break
		}
		if prior[i] == 0 {
			zeros++
			continue
		}
		xs = append(xs, float64(prior[i]))
	}
	slices.Reverse(xs)
	if len(xs) == 0 || len(xs) < d.MinBandDeliveries {
		return FeedDeliveryBand{N: len(xs), Zeros: zeros}, false
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
		Median: med, MAD: mad, N: len(xs), Zeros: zeros,
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
		zeros := ""
		switch band.Zeros {
		case 0:
		case 1:
			zeros = " (1 preceding size of 0 bytes, an empty file or a size never recorded, is left out)"
		default:
			zeros = fmt.Sprintf(" (%d preceding sizes of 0 bytes, empty files or sizes never recorded, are left out)",
				band.Zeros)
		}
		s = fmt.Sprintf("%s declares no size range, so the delivery is judged against the feed's trailing band: "+
			"the median of the %d preceding non-zero delivery sizes%s, %.0f bytes, plus or minus the larger of %g "+
			"scaled median absolute deviations (MAD %.0f) and %g of the median, giving %d to %d bytes. "+
			"The band's parameters are defaults nobody has measured",
			feedDeliveryDeclaration(f), band.N, zeros, band.Median, d.BandSpread, band.MAD, d.MinBandWidth,
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
