package observe

// Unit tests of the delivery-grain feed detectors over inputs built in Go: the
// trailing band, which the fixture has no case for because every profiled
// fixture feed declares a size range, and the coverage cases the fixture does
// not hold. They need no database.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe/feedfixture"
	"github.com/artisoft-io/jetstore/jets/wsvalidate"
)

func feedDeliveryPtr[T any](v T) *T { return &v }

// feedDeliveryWeekly builds a weekly feed's deliveries, one a week from a
// Monday, each registered under its own week, with the sizes given; a
// negative size is a delivery with no staging row.
func feedDeliveryWeekly(sizes ...int64) []FeedDelivery {
	start := time.Date(2026, 1, 5, 6, 0, 0, 0, time.UTC)
	var out []FeedDelivery
	for i, s := range sizes {
		at := start.AddDate(0, 0, 7*i)
		m, w, d := feedfixture.Period(at)
		x := FeedDelivery{Ordinal: i + 1, SessionId: fmt.Sprint(at.UnixMilli()), Arrival: at,
			Client: "client_x", Org: "", FileKey: fmt.Sprintf("k/%d", i),
			Period: &FeedDeliveryPeriod{Date: at.Truncate(24 * time.Hour), Month: m, Week: w, Day: d}}
		if s >= 0 {
			x.SizeBytes = feedDeliveryPtr(s)
		}
		out = append(out, x)
	}
	return out
}

func feedDeliveryFeed(table string, p feedprofile.Profile) feedprofile.Feed {
	p.DeclaredBy, p.DeclaredAt = "someone", "2026-01-01"
	return feedprofile.Feed{Client: "client_x", TableName: table, ObjectType: "Claim", Profile: p}
}

func TestFeedDeliveryPeriodStartInvertsCalculatePeriod(t *testing.T) {
	for d := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC); d.Year() < 2027; d = d.AddDate(0, 0, 1) {
		m, w, dp := feedfixture.Period(d)
		for pt, n := range map[string]int{feedprofile.MonthPeriod: m, feedprofile.WeekPeriod: w,
			feedprofile.DayPeriod: dp} {
			start := FeedDeliveryPeriodStart(pt, n)
			next := FeedDeliveryPeriodStart(pt, n+1)
			if d.Before(start) || !d.Before(next) {
				t.Fatalf("%s %d runs %s to %s and does not hold %s", pt, n, start, next, d)
			}
		}
	}
	if wd := FeedDeliveryPeriodStart(feedprofile.WeekPeriod, 2940).Weekday(); wd != time.Thursday {
		t.Errorf("a week period starts on %s", wd)
	}
}

func TestFeedDeliveryBandDefaults(t *testing.T) {
	d := DefaultDeliverySize()
	// Too little history.
	if _, ok := d.Band([]int64{100, 100, 100}); ok {
		t.Error("three sizes made a band; the default needs four")
	}
	// Identical sizes: the MAD is zero and the floor sets the width.
	b, ok := d.Band([]int64{1000, 1000, 1000, 1000})
	if !ok || b.Min != 500 || b.Max != 1500 || b.MAD != 0 {
		t.Errorf("identical sizes give %+v", b)
	}
	// A spread wider than the floor: median 1000, MAD 200, 3 × 1.4826 × 200.
	b, _ = d.Band([]int64{600, 800, 1000, 1200, 1400})
	if b.Median != 1000 || b.MAD != 200 || b.Min != 110 || b.Max != 1890 {
		t.Errorf("a wide spread gives %+v", b)
	}
	// Only the last BandDeliveries count: an old outlier ages out.
	b, _ = d.Band([]int64{1_000_000, 10, 10, 10, 10, 10, 10, 10, 10})
	if b.N != 8 || b.Median != 10 {
		t.Errorf("the band reads %d sizes, median %g", b.N, b.Median)
	}
	// The lower bound never goes below zero.
	b, _ = d.Band([]int64{10, 10, 1000, 1000, 1000})
	if b.Min < 0 {
		t.Errorf("a negative lower bound: %+v", b)
	}
}

// A size of 0 is an empty file or a size never recorded, so the band leaves
// it out: half or more zeros no longer make a band of 0 to 0 bytes, and the
// band still reads the last BandDeliveries non-zero sizes.
func TestFeedDeliveryBandLeavesOutZeroSizes(t *testing.T) {
	d := DefaultDeliverySize()
	b, ok := d.Band([]int64{0, 1000, 0, 1000, 0, 0, 1000, 0, 1000, 0})
	if !ok || b.N != 4 || b.Zeros != 6 || b.Median != 1000 || b.Min != 500 || b.Max != 1500 {
		t.Errorf("half zeros give %+v, ok %v", b, ok)
	}
	// Fewer than MinBandDeliveries non-zero sizes: no band, however many zeros.
	if b, ok := d.Band([]int64{0, 0, 0, 0, 0, 0, 1000, 1000, 1000}); ok || b.N != 3 || b.Zeros != 6 {
		t.Errorf("three non-zero sizes among nine made a band: %+v", b)
	}
	if _, ok := d.Band([]int64{0, 0, 0, 0, 0, 0, 0, 0}); ok {
		t.Error("only zeros made a band")
	}
	// The window is the last eight non-zero sizes; zeros before the oldest of
	// them are not counted as left out.
	b, _ = d.Band([]int64{0, 0, 7, 10, 10, 10, 0, 10, 10, 10, 10, 10})
	if b.N != 8 || b.Zeros != 1 || b.Median != 10 {
		t.Errorf("the window over non-zero sizes: %+v", b)
	}
}

// A feed whose recorded sizes are mostly 0 is judged against its non-zero
// sizes. The fifth delivery, which fired against a band of 0 to 0 bytes when
// zeros were read into the band, is not judged: fewer than four non-zero sizes
// precede it, as they do every delivery up to the eleventh. The twelfth is
// silent, and the empty thirteenth fires below the band of the full ones.
func TestFeedDeliverySizeBandOverZeroSizes(t *testing.T) {
	f := feedDeliveryFeed("zero_feed", feedprofile.Profile{PeriodType: feedprofile.WeekPeriod})
	ds := feedDeliveryWeekly(0, 0, 0, 0, 1000, 0, 1000, 0, 1000, 0, 1000, 1000, 0)
	in := &FeedDeliveryInput{AsOf: ds[len(ds)-1].Arrival, Profiled: []feedprofile.Feed{f},
		Deliveries: map[string][]FeedDelivery{f.TableName: ds}}
	res, err := DefaultDeliverySize().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	short := "not_judged insufficient_history"
	want := []string{short, short, short, short, short, short, short, short, short, short, short,
		"silent ", "fires "}
	if len(res.Verdicts) != len(want) {
		t.Fatalf("%d verdicts", len(res.Verdicts))
	}
	for i, v := range res.Verdicts {
		if got := v.Verdict + " " + v.Reason; got != want[i] {
			t.Errorf("delivery %d: %s, want %s", i+1, got, want[i])
		}
	}
	if len(res.Anomalies) != 1 {
		t.Fatalf("%d anomalies", len(res.Anomalies))
	}
	a := res.Anomalies[0]
	if a.ObservedValue != "0" || *a.ExpectedMin != "500" || *a.ExpectedMax != "1500" ||
		a.DeviationMagnitude == nil || *a.DeviationMagnitude != 1 ||
		!strings.Contains(a.ExpectedBasis, "the median of the 5 preceding non-zero delivery sizes (7 preceding "+
			"sizes of 0 bytes, empty files or sizes never recorded, are left out), 1000 bytes") {
		t.Errorf("the empty delivery's anomaly: %+v %s", a, a.ExpectedBasis)
	}
	if err := a.Validate(); err != nil {
		t.Error(err)
	}
}

func TestFeedDeliveryBandParametersAreArguments(t *testing.T) {
	d := DeliverySize{Generation: 1, BandDeliveries: 3, MinBandDeliveries: 2, BandSpread: 1, MinBandWidth: 0.1}
	// The last three, 90, 100 and 120: median 100, MAD 10, half-width 14.8.
	b, ok := d.Band([]int64{9, 90, 100, 120})
	if !ok || b.N != 3 || b.Median != 100 || b.Min != 85 || b.Max != 115 {
		t.Errorf("the band ignores its arguments: %+v", b)
	}
}

// With no declared range, the size detector judges each delivery against the
// band of those before it, and says so; a delivery with too little history is
// not judged and carries history_truncated.
func TestFeedDeliverySizeTrailingBand(t *testing.T) {
	f := feedDeliveryFeed("band_feed", feedprofile.Profile{PeriodType: feedprofile.WeekPeriod})
	// A steady feed, a sudden halving, a delivery with no size, recovery, and a
	// cumulative-style slow growth that stays in the band.
	ds := feedDeliveryWeekly(1000, 1010, 990, 1005, 1000, 480, -1, 1000, 1030, 1060, 1090)
	in := &FeedDeliveryInput{AsOf: ds[len(ds)-1].Arrival, Profiled: []feedprofile.Feed{f},
		Deliveries: map[string][]FeedDelivery{f.TableName: ds}}
	res, err := DefaultDeliverySize().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"not_judged insufficient_history", "not_judged insufficient_history",
		"not_judged insufficient_history", "not_judged insufficient_history", "silent ", "fires ",
		"not_judged no_size", "silent ", "silent ", "silent ", "silent "}
	if len(res.Verdicts) != len(want) {
		t.Fatalf("%d verdicts", len(res.Verdicts))
	}
	for i, v := range res.Verdicts {
		if got := v.Verdict + " " + v.Reason; got != want[i] {
			t.Errorf("delivery %d: %s, want %s", i+1, got, want[i])
		}
		if v.Reason == FeedDeliveryShortHistory && (len(v.Confounders) != 1 || v.Confounders[0] != ConfounderHistoryTruncated) {
			t.Errorf("delivery %d has too little history and does not say history_truncated", i+1)
		}
	}
	if len(res.Anomalies) != 1 {
		t.Fatalf("%d anomalies", len(res.Anomalies))
	}
	a := res.Anomalies[0]
	if a.ObservedValue != "480" || *a.ExpectedMin != "500" || *a.ExpectedMax != "1500" ||
		!strings.Contains(a.ExpectedBasis, "declares no size range") ||
		!strings.Contains(a.ExpectedBasis, "trailing band") {
		t.Errorf("the band's anomaly: %+v", a)
	}
	if err := a.Validate(); err != nil {
		t.Error(err)
	}
	// The halved delivery joins the history after it is judged: the next
	// delivery is judged against a band that includes it.
	if res.Verdicts[7].Verdict != FeedDeliverySilent {
		t.Errorf("the delivery after the halving: %+v", res.Verdicts[7])
	}
}

func TestFeedDeliverySizeHalfOpenRange(t *testing.T) {
	f := feedDeliveryFeed("min_only", feedprofile.Profile{SizeBytes: &feedprofile.SizeRange{Min: feedDeliveryPtr[int64](500)}})
	ds := feedDeliveryWeekly(499, 500, 1_000_000)
	res, _ := DefaultDeliverySize().Detect(&FeedDeliveryInput{AsOf: ds[2].Arrival,
		Profiled: []feedprofile.Feed{f}, Deliveries: map[string][]FeedDelivery{f.TableName: ds}})
	if res.Verdicts[0].Verdict != FeedDeliveryFires || res.Verdicts[1].Verdict != FeedDeliverySilent ||
		res.Verdicts[2].Verdict != FeedDeliverySilent {
		t.Errorf("a minimum alone: %+v", res.Verdicts)
	}
	if a := res.Anomalies[0]; a.ExpectedMax != nil || *a.ExpectedMin != "500" ||
		!strings.Contains(a.ExpectedBasis, "at least 500") {
		t.Errorf("the anomaly under a minimum alone: %+v", a)
	}
}

// The coverage names each feed not judged with its reason: no profile, a
// refused profile with its findings, a missing field by name, and a profiled
// feed with no surviving delivery.
func TestFeedDeliveryCoverageReasons(t *testing.T) {
	refused := feedprofile.Feed{Client: "client_x", TableName: "refused",
		Findings: []wsvalidate.Finding{{Severity: wsvalidate.Error, Message: "period_type is not one of …"}}}
	bare := feedprofile.Feed{Client: "client_x", TableName: "bare"}
	noPeriod := feedDeliveryFeed("no_period_type", feedprofile.Profile{OverdueAfterDays: feedDeliveryPtr(3)})
	empty := feedDeliveryFeed("never_delivered", feedprofile.Profile{PeriodType: feedprofile.WeekPeriod,
		OverdueAfterDays: feedDeliveryPtr(3)})
	in := &FeedDeliveryInput{AsOf: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Profiled: []feedprofile.Feed{noPeriod, empty}, Unprofiled: []feedprofile.Feed{refused, bare},
		Deliveries: map[string][]FeedDelivery{}, Unordered: map[string]int{"never_delivered": 2}}
	res, err := DefaultFeedOverdue().Detect(in)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]FeedDeliveryNotJudgedFeed{}
	for _, n := range res.Coverage.NotJudged {
		got[n.TableName] = n
	}
	for table, reason := range map[string]string{"refused": FeedDeliveryProfileRefused, "bare": FeedDeliveryNoProfile,
		"no_period_type": "field_absent:period_type", "never_delivered": FeedDeliveryNoDelivery} {
		if got[table].Reason != reason {
			t.Errorf("%s: %q, want %q", table, got[table].Reason, reason)
		}
	}
	if len(got["refused"].Findings) != 1 {
		t.Error("the refused profile's findings are not carried")
	}
	if len(res.Coverage.Judged) != 0 || len(res.Anomalies) != 0 {
		t.Errorf("judged %v", res.Coverage.Judged)
	}
	line := res.Coverage.Line()
	for _, want := range []string{"feed_overdue: judged 0", "refused (profile_refused)",
		"no_period_type (field_absent:period_type)", "never_delivered (no_delivery)"} {
		if !strings.Contains(line, want) {
			t.Errorf("the coverage line lacks %q: %s", want, line)
		}
	}
	// The size detector needs no field, so the same profiles are judged by it.
	size, _ := DefaultDeliverySize().Detect(in)
	if len(size.Coverage.Judged) != 2 {
		t.Errorf("delivery_size judged %v", size.Coverage.Judged)
	}
}

// periods_per_delivery widens the interval: a feed due every two weeks is not
// overdue a week after its latest period.
func TestFeedDeliveryOverdueEveryTwoPeriods(t *testing.T) {
	ds := feedDeliveryWeekly(1, 1, 1)
	latest := ds[2].Period.Week
	for ppd, from := range map[int]time.Time{
		1: FeedDeliveryPeriodStart(feedprofile.WeekPeriod, latest+2).AddDate(0, 0, 3),
		2: FeedDeliveryPeriodStart(feedprofile.WeekPeriod, latest+3).AddDate(0, 0, 3),
	} {
		f := feedDeliveryFeed("biweekly", feedprofile.Profile{PeriodType: feedprofile.WeekPeriod,
			PeriodsPerDelivery: feedDeliveryPtr(ppd), OverdueAfterDays: feedDeliveryPtr(3)})
		if got := FeedOverdueFrom(f.Profile, latest); !got.Equal(from) {
			t.Errorf("every %d weeks: overdue from %s, want %s", ppd, got, from)
		}
		for _, c := range []struct {
			at   time.Time
			want string
		}{{from.Add(-time.Second), FeedDeliverySilent}, {from, FeedDeliveryFires}} {
			res, _ := DefaultFeedOverdue().Detect(&FeedDeliveryInput{AsOf: c.at, Profiled: []feedprofile.Feed{f},
				Deliveries: map[string][]FeedDelivery{f.TableName: ds}})
			if res.Verdicts[0].Verdict != c.want {
				t.Errorf("every %d weeks, at %s: %s", ppd, c.at, res.Verdicts[0].Verdict)
			}
		}
	}
}

// A delivery with no period is not judged and is nobody's latest; the first
// delivery with a period then has no predecessor.
func TestFeedDeliveryBackdatedSkipsAMissingPeriod(t *testing.T) {
	ds := feedDeliveryWeekly(1, 1, 1, 1)
	ds[0].Period = nil
	ds[3].Period = ds[1].Period // registered under an earlier week than ds[2]
	f := feedDeliveryFeed("gappy", feedprofile.Profile{PeriodType: feedprofile.WeekPeriod})
	res, _ := DefaultDeliveryBackdated().Detect(&FeedDeliveryInput{AsOf: ds[3].Arrival,
		Profiled: []feedprofile.Feed{f}, Deliveries: map[string][]FeedDelivery{f.TableName: ds}})
	want := []string{"not_judged no_period", "not_judged no_predecessor", "silent ", "fires "}
	for i, v := range res.Verdicts {
		if got := v.Verdict + " " + v.Reason; got != want[i] {
			t.Errorf("delivery %d: %s, want %s", i+1, got, want[i])
		}
	}
	if len(res.Anomalies) != 1 || *res.Anomalies[0].DeviationMagnitude != 1 {
		t.Errorf("anomalies: %+v", res.Anomalies)
	}
}
