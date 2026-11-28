package observe

import (
	"testing"
	"time"
)

// A registration a session deletion marked deleted is no delivery: when it is
// its file's first it is the file's arrival, so the file's later registrations
// stay re-processing, as a jetsapi.purged_delivery row makes them; otherwise it
// is dropped. No database needed.
func TestMarkedRegistrationIsEvidenceNotADelivery(t *testing.T) {
	at := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	reg := func(s, k string, d int) FeedDelivery {
		return FeedDelivery{SessionId: s, FileKey: k, Arrival: at(d)}
	}
	r := &FeedRegistrations{ByTable: map[string][]FeedDelivery{}, Unordered: map[string]int{}}
	r.Add("t", reg("1", "a", 1), true)  // a's first registration, marked
	r.Add("t", reg("2", "b", 2), false) // b's first, live
	r.Add("t", reg("3", "a", 3), false) // a again: re-processing
	r.Add("t", reg("4", "b", 4), true)  // b again, marked: dropped
	r.Add("t", reg("5", "", 5), true)   // no file key, marked: dropped
	r.Add("t", reg("6", "c", 6), false) // c's first, live

	ds := r.Deliveries("t", time.Time{})
	if len(ds) != 2 || ds[0].FileKey != "b" || ds[1].FileKey != "c" || ds[0].Ordinal != 1 || ds[1].Ordinal != 2 {
		t.Fatalf("deliveries %+v, want b then c", ds)
	}
	if len(ds[0].Reprocessed) != 0 {
		t.Errorf("b's re-processing %+v, want none: its marked registration is no registration", ds[0].Reprocessed)
	}
	ps := r.PurgedFiles("t", time.Time{})
	if len(ps) != 1 || ps[0].FileKey != "a" || !ps[0].FirstArrival.Equal(at(1)) ||
		len(ps[0].Reprocessed) != 1 || ps[0].Reprocessed[0].SessionId != "3" {
		t.Fatalf("purged files %+v, want a, first arrived on day 1, re-processed by session 3", ps)
	}

	// An earlier first registration recorded in purged_delivery is kept.
	r = &FeedRegistrations{ByTable: map[string][]FeedDelivery{}, Unordered: map[string]int{},
		Purged: map[string]map[string]time.Time{"t": {"a": at(1)}}}
	r.Add("t", reg("2", "a", 2), true)
	r.Add("t", reg("3", "a", 3), false)
	if ps := r.PurgedFiles("t", time.Time{}); len(ps) != 1 || !ps[0].FirstArrival.Equal(at(1)) {
		t.Errorf("purged files %+v, want a's recorded arrival, day 1", ps)
	}
}
