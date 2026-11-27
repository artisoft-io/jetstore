package claimset

// The summary's rules on a few records. That the summary is the row the feed
// detection computes from the table, field by field, is
// TestClaimSetSummariserMatchesTheStatement's, in jets/agentic/observe.

import (
	"fmt"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/keyset"
)

func testFeed(lag, span *int) feedprofile.Feed {
	return feedprofile.Feed{TableName: "t", Client: "c", Profile: feedprofile.Profile{
		ClaimSet: &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"id", "line"},
			ServiceDateColumn: "d", MemberColumn: "m"},
		MaxServiceDateLagDays: lag, MaxServiceDateSpanDays: span}}
}

func str(s string) *string { return &s }

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestSummaryOfNoRecordIsNil(t *testing.T) {
	if s := NewSummariser(testFeed(nil, nil), nil, 0).Summary("d", "s", nil, time.Now()); s != nil {
		t.Errorf("%+v", s)
	}
}

// A delivery of many members: exact dates, the lag and the span decided on
// days, the histogram's buckets, the present counts, and a fingerprint and a
// sketch that are keyset's over the distinct keys.
func TestSummaryOfALargeDelivery(t *testing.T) {
	lag, span := 30, 10
	s := NewSummariser(testFeed(&lag, &span), []string{"id", "m"}, 11)
	b := keyset.NewBuilder()
	newest := day(2026, 3, 31)
	for i := range 40 {
		key := []*string{str(fmt.Sprintf("c%d", i/2)), str("1")}
		b.AddText(keyset.KeyText(key))
		d := newest.AddDate(0, 0, -i)
		s.Add(Record{Key: key, Member: str(fmt.Sprintf("m%d", i)), ServiceDate: &d, Present: []bool{true, i%2 == 0}})
	}
	s.Add(Record{Key: []*string{nil, nil}, Present: []bool{false, false}})
	b.AddText(keyset.KeyText([]*string{nil, nil}))
	arrival := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	got := s.Summary("d", "s", &arrival, time.Now())
	want := b.Summary()
	if got.Rows != 41 || got.DistinctKeys != 21 || got.Members != 40 || got.DatedRows != 40 || got.DatesInMonths {
		t.Errorf("counts %+v", got)
	}
	if !got.Newest.Equal(*newest) || !got.Oldest.Equal(newest.AddDate(0, 0, -39)) || *got.SpanDays != 39 {
		t.Errorf("dates %v %v %v", got.Newest, got.Oldest, *got.SpanDays)
	}
	if !*got.LagExceeds || *got.OutOfSpan != 29 {
		t.Errorf("lag %v, out of span %d", *got.LagExceeds, *got.OutOfSpan)
	}
	// Behind the newest: 0 | 1 | 2-7 | 8-14 | 15-21 | 22-28 | 29-30 | 31 | 32-39.
	wantRows := []int64{1, 1, 6, 7, 7, 7, 2, 1, 8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	if fmt.Sprint(got.Histogram.Rows) != fmt.Sprint(wantRows) {
		t.Errorf("histogram %v, want %v", got.Histogram.Rows, wantRows)
	}
	if got.Present["id"] != 40 || got.Present["m"] != 20 {
		t.Errorf("present %v", got.Present)
	}
	if got.Fingerprint == nil || uint64(*got.Fingerprint) != want.Fingerprint || string(got.Sketch) != string(want.Sketch.Pack()) {
		t.Error("the fingerprint or the sketch is not keyset's")
	}
}

// A delivery of few members keeps its dates by month and its span in days, and
// one of few keys keeps no fingerprint or sketch; one with no date of service
// counts no row outside a declared span.
func TestSummaryOfASmallDelivery(t *testing.T) {
	span := 10
	s := NewSummariser(testFeed(nil, &span), nil, 11)
	for i := range 3 {
		s.Add(Record{Key: []*string{str(fmt.Sprint(i)), nil}, Member: str("m"), ServiceDate: day(2026, 3, 20+i)})
	}
	got := s.Summary("d", "s", nil, time.Now())
	if !got.DatesInMonths || !got.Newest.Equal(*day(2026, 3, 1)) || *got.SpanDays != 2 || got.Fingerprint != nil ||
		got.Sketch != nil || got.LagExceeds != nil {
		t.Errorf("%+v", got)
	}
	undated := NewSummariser(testFeed(nil, &span), nil, 11)
	undated.Add(Record{Key: []*string{str("1"), nil}})
	if got := undated.Summary("d", "s", nil, time.Now()); got.OutOfSpan == nil || *got.OutOfSpan != 0 ||
		got.Histogram != nil || got.Newest != nil || got.SpanDays != nil {
		t.Errorf("%+v", got)
	}
}
