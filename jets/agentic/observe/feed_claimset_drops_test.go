package observe

// The stale-claim and span checks count the records a run's retain window
// dropped before they reached the claim-set table. Judged over aggregates
// built in Go, which needs no database.

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
)

func claimSetDropsFeed(lag, span int, share *float64) feedprofile.Feed {
	return feedprofile.Feed{Client: "client_x", TableName: "drops_feed", ObjectType: "Claim",
		Profile: feedprofile.Profile{
			ClaimSet: &feedprofile.ClaimSet{Table: "t", KeyColumns: []string{"k"}, ServiceDateColumn: "d",
				MemberColumn: "m"},
			MaxServiceDateLagDays: feedDeliveryPtr(lag), MaxServiceDateSpanDays: feedDeliveryPtr(span),
			MaxOutOfSpanShare: share, DeclaredBy: "someone", DeclaredAt: "2026-01-01",
		}}
}

func claimSetDropsJudge(det string, f feedprofile.Feed, a *ClaimSetAggregates) ClaimSetVerdict {
	arrival := time.UnixMilli(1767225600000).UTC()
	d := claimSetDelivery{session: "1767225600000", ordinal: 1, arrival: &arrival}
	v := ClaimSetVerdict{Detector: det, SessionID: d.session, Aggregates: a}
	claimSetJudge(&v, det, f, d, nil, "", claimSetReadable{}, a, DefaultClaimSetOptions())
	return v
}

func dropsI64(n int64) *int64 { return &n }

func bptr(b bool) *bool { return &b }

// rowsWithDates are the aggregates of a delivery with rows in the table: 100
// rows with a date of service, outside of them outside the span, spanning
// spanDays, and a lag of lag days.
func rowsWithDates(outside, spanDays, lag int64, exceeds bool) *ClaimSetAggregates {
	run := "P1"
	return &ClaimSetAggregates{Rows: 100, Members: 50, DistinctKeys: 100, DatedRows: 100,
		OutOfSpanRows: dropsI64(outside), SpanDays: dropsI64(spanDays), Lag: dropsI64(lag), LagExceeds: bptr(exceeds), Run: &run}
}

func withDrops(a *ClaimSetAggregates, before, after, daysBefore, daysAfter int64) *ClaimSetAggregates {
	a.Dropped = &ClaimSetDropped{Column: "service_date", DaysBefore: daysBefore, DaysAfter: daysAfter,
		NoDate: 4, BeforeWindow: before, AfterWindow: after}
	return a
}

// Without a window, and with a window that dropped records while rows with a
// date reached the table, the stale check judges the newest date as before:
// the drops change its detail and not its verdict.
func TestClaimSetStaleWithRowsJudgesTheNewest(t *testing.T) {
	f := claimSetDropsFeed(45, 30, nil)
	for _, c := range []struct {
		lag     int64
		exceeds bool
		want    string
	}{{10, false, ClaimSetSilent}, {90, true, ClaimSetFires}} {
		plain := claimSetDropsJudge(ClaimSetServiceDateStale, f, rowsWithDates(0, 20, c.lag, c.exceeds))
		dropped := claimSetDropsJudge(ClaimSetServiceDateStale, f,
			withDrops(rowsWithDates(0, 20, c.lag, c.exceeds), 25, 0, 60, 0))
		if plain.Verdict != c.want || dropped.Verdict != c.want {
			t.Errorf("lag %d: %s without drops and %s with, want %s", c.lag, plain.Verdict, dropped.Verdict, c.want)
		}
		if strings.Contains(plain.Detail, "retain window") {
			t.Errorf("lag %d without a window: detail %q", c.lag, plain.Detail)
		}
		if !strings.Contains(dropped.Detail, "dropped 29 records with its retain window on \"service_date\"") ||
			!strings.Contains(dropped.Detail, "the 25 before it are stale") {
			t.Errorf("lag %d with drops: detail %q", c.lag, dropped.Detail)
		}
		if c.want == ClaimSetFires && (plain.Anomalies[0].ObservedValue != dropped.Anomalies[0].ObservedValue ||
			plain.Anomalies[0].AnomalyId != dropped.Anomalies[0].AnomalyId) {
			t.Errorf("lag %d: the anomaly moved, %+v to %+v", c.lag, plain.Anomalies[0], dropped.Anomalies[0])
		}
	}
}

// A delivery whose window dropped every record with a date as before it fires
// the stale check, when the declared lag is at most the window; otherwise, or
// when some were dropped after the window, it is not judged.
func TestClaimSetStaleWhenEveryDatedRecordWasDropped(t *testing.T) {
	allDropped := func(before, after int64) *ClaimSetAggregates {
		run := "P1"
		a := &ClaimSetAggregates{Run: &run}
		return withDrops(a, before, after, 60, 0)
	}
	v := claimSetDropsJudge(ClaimSetServiceDateStale, claimSetDropsFeed(45, 30, nil), allDropped(500, 0))
	if v.Verdict != ClaimSetFires || len(v.Anomalies) != 1 {
		t.Fatalf("every dated record dropped before the window: %s %s %q", v.Verdict, v.Reason, v.Detail)
	}
	an := v.Anomalies[0]
	if an.ObservedValue != "more than 60 days" || *an.ExpectedMax != "45 days" || an.DeviationMagnitude != nil ||
		!strings.Contains(an.ExpectedBasis, "none of the 500 records of delivery 1767225600000 with a date of service") ||
		!strings.Contains(an.ExpectedBasis, "session P1") {
		t.Errorf("the anomaly %+v", an)
	}
	if err := an.Validate(); err != nil {
		t.Error(err)
	}
	if an.AnomalyId != "service_date_stale@1/drops_feed/1767225600000" {
		t.Errorf("the identifier %q", an.AnomalyId)
	}
	for _, c := range []struct {
		name   string
		f      feedprofile.Feed
		a      *ClaimSetAggregates
		reason string
		detail string
	}{
		{"a lag wider than the window", claimSetDropsFeed(90, 30, nil), allDropped(500, 0), ClaimSetNoServiceDates,
			"reaches further back than the window"},
		{"some dropped after the window", claimSetDropsFeed(45, 30, nil), allDropped(500, 3), ClaimSetNoServiceDates,
			"3 after it"},
		{"none dropped before the window", claimSetDropsFeed(45, 30, nil), allDropped(0, 3), ClaimSetNoServiceDates,
			"0 before the window"},
	} {
		v := claimSetDropsJudge(ClaimSetServiceDateStale, c.f, c.a)
		if v.Verdict != ClaimSetNotJudged || v.Reason != c.reason || !strings.Contains(v.Detail, c.detail) {
			t.Errorf("%s: %s %s %q", c.name, v.Verdict, v.Reason, v.Detail)
		}
	}
	// With no window and no rows, as before: not evaluable.
	run := "P1"
	v = claimSetDropsJudge(ClaimSetServiceDateStale, claimSetDropsFeed(45, 30, nil), &ClaimSetAggregates{Run: &run})
	if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoClaimRows || strings.Contains(v.Detail, "retain window") {
		t.Errorf("no rows and no window: %s %s %q", v.Verdict, v.Reason, v.Detail)
	}
	// The checks that do not read drops stay not evaluable, and say why.
	v = claimSetDropsJudge(ClaimSetDeliveryOverlap, claimSetDropsFeed(45, 30, nil), allDropped(500, 0))
	if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoClaimRows ||
		!strings.Contains(v.Detail, "dropped 504 records with its retain window") {
		t.Errorf("overlap with every record dropped: %s %s %q", v.Verdict, v.Reason, v.Detail)
	}
}

// The span check counts the records dropped before or after the window as
// outside the span, with the rows with a date, when the declared span is at
// most the window's width; and quotes the run's Input Span and Core Span.
func TestClaimSetSpanCountsTheDroppedRecords(t *testing.T) {
	five := 0.05
	for _, c := range []struct {
		name              string
		f                 feedprofile.Feed
		a                 *ClaimSetAggregates
		verdict, observed string
		detail            []string
	}{
		{"a share, no window: 2 of 100", claimSetDropsFeed(45, 30, &five), rowsWithDates(2, 400, 1, false),
			ClaimSetSilent, "", []string{"2 of 100 rows"}},
		{"a share, 10 dropped before the window: 12 of 110", claimSetDropsFeed(45, 30, &five),
			withDrops(rowsWithDates(2, 400, 1, false), 10, 0, 60, 2), ClaimSetFires, "0.1091",
			[]string{"12 of 110 (10.9%) are outside the span", "the 10 outside it are outside the span"}},
		{"a share, a span wider than the window", claimSetDropsFeed(45, 90, &five),
			withDrops(rowsWithDates(2, 400, 1, false), 10, 0, 60, 2), ClaimSetSilent, "",
			[]string{"is wider than the window's 62 days"}},
		{"no share, the rows inside, records dropped after the window", claimSetDropsFeed(45, 30, nil),
			withDrops(rowsWithDates(0, 25, 1, false), 0, 6, 60, 2), ClaimSetFires, "6 records outside",
			[]string{"6 of 106 (5.7%) are outside the span"}},
		{"no share, the rows outside too", claimSetDropsFeed(45, 30, nil),
			withDrops(rowsWithDates(3, 400, 1, false), 5, 0, 60, 2), ClaimSetFires, "400 days", nil},
		{"no share, a window that dropped nothing outside it", claimSetDropsFeed(45, 30, nil),
			withDrops(rowsWithDates(0, 25, 1, false), 0, 0, 60, 2), ClaimSetSilent, "",
			[]string{"dropped 4 records with its retain window"}},
	} {
		v := claimSetDropsJudge(ClaimSetServiceDateSpan, c.f, c.a)
		if v.Verdict != c.verdict {
			t.Errorf("%s: %s %s %q", c.name, v.Verdict, v.Reason, v.Detail)
			continue
		}
		for _, want := range c.detail {
			if !strings.Contains(v.Detail, want) {
				t.Errorf("%s: detail %q lacks %q", c.name, v.Detail, want)
			}
		}
		if c.verdict != ClaimSetFires {
			continue
		}
		an := v.Anomalies[0]
		if an.ObservedValue != c.observed {
			t.Errorf("%s: observed %q, want %q", c.name, an.ObservedValue, c.observed)
		}
		if c.a.Dropped != nil && (c.a.Dropped.BeforeWindow+c.a.Dropped.AfterWindow) > 0 &&
			!strings.Contains(an.ExpectedBasis, "session P1") {
			t.Errorf("%s: the basis does not name the run: %q", c.name, an.ExpectedBasis)
		}
		if err := an.Validate(); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// Every dated record dropped outside the window: the share outside is 1.
func TestClaimSetSpanWhenEveryDatedRecordWasDropped(t *testing.T) {
	five := 0.05
	run := "P1"
	a := withDrops(&ClaimSetAggregates{Run: &run, InputSpanDays: dropsI64(1496), CoreSpanDays: dropsI64(965)}, 40, 2, 60, 2)
	v := claimSetDropsJudge(ClaimSetServiceDateSpan, claimSetDropsFeed(45, 30, &five), a)
	if v.Verdict != ClaimSetFires || v.Anomalies[0].ObservedValue != "1.0000" ||
		!strings.Contains(v.Anomalies[0].ExpectedBasis, "an Input Span of 1496 days and a Core Span of 965 days") {
		t.Errorf("every dated record dropped: %s %q %+v", v.Verdict, v.Detail, v.Anomalies)
	}
	// A span wider than the window: nothing with a date is left to judge.
	v = claimSetDropsJudge(ClaimSetServiceDateSpan, claimSetDropsFeed(45, 90, &five), a)
	if v.Verdict != ClaimSetNotJudged || v.Reason != ClaimSetNoServiceDates {
		t.Errorf("a span wider than the window: %s %s %q", v.Verdict, v.Reason, v.Detail)
	}
}

// The span check quotes the run's Input Span and Core Span in its basis when
// the run recorded them, and its verdict without a window is the one it
// reached before.
func TestClaimSetSpanQuotesTheRunsSpans(t *testing.T) {
	a := rowsWithDates(3, 400, 1, false)
	a.InputSpanDays, a.CoreSpanDays = dropsI64(1496), dropsI64(965)
	v := claimSetDropsJudge(ClaimSetServiceDateSpan, claimSetDropsFeed(45, 30, nil), a)
	plain := claimSetDropsJudge(ClaimSetServiceDateSpan, claimSetDropsFeed(45, 30, nil), rowsWithDates(3, 400, 1, false))
	if v.Verdict != ClaimSetFires || plain.Verdict != ClaimSetFires ||
		v.Anomalies[0].ObservedValue != plain.Anomalies[0].ObservedValue ||
		!strings.HasPrefix(v.Anomalies[0].ExpectedBasis, plain.Anomalies[0].ExpectedBasis) {
		t.Fatalf("with spans %+v, without %+v", v.Anomalies, plain.Anomalies)
	}
	want := "The run that read it, session P1, recorded an Input Span of 1496 days and a Core Span of 965 days"
	if !strings.Contains(v.Anomalies[0].ExpectedBasis, want) || !strings.Contains(v.Detail, want) ||
		strings.Contains(plain.Anomalies[0].ExpectedBasis, "Input Span") {
		t.Errorf("basis %q, detail %q", v.Anomalies[0].ExpectedBasis, v.Detail)
	}
}
