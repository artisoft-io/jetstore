package triage

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// day is the epoch milliseconds of noon UTC on day d of November 2023.
func day(d int) int64 {
	return time.Date(2023, 11, d, 12, 0, 0, 0, time.UTC).UnixMilli()
}

func sid(ms int64) string { return strconv.FormatInt(ms, 10) }

// registrations builds a feed's deliveries as observe reads them: each
// (session, file key) in registration order, a repeated file key being
// re-processing.
func registrations(feed string, regs ...[2]string) []observe.FeedDelivery {
	r := &observe.FeedRegistrations{ByTable: map[string][]observe.FeedDelivery{}}
	for _, x := range regs {
		ms, _ := strconv.ParseInt(x[0], 10, 64)
		r.ByTable[feed] = append(r.ByTable[feed], observe.FeedDelivery{SessionId: x[0], FileKey: x[1],
			Arrival: time.UnixMilli(ms).UTC()})
	}
	return r.Deliveries(feed, time.Time{})
}

// finding is a feed anomaly of feed on session with the signal, detected at
// the session's own time.
func finding(feed, session, signal string) observe.Anomaly {
	at := time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC)
	if ms, ok := sessionMillis(session); ok {
		at = time.UnixMilli(ms).UTC().Add(time.Hour)
	}
	return observe.Anomaly{AnomalyId: "a/" + feed + "/" + session + "/" + signal, DetectedAt: at, SessionId: session,
		SubjectType: observe.SubjectFeed, SubjectRef: feed, SignalType: signal, DetectorRef: "delivery_size@1"}
}

func departs(feed, session string) observe.Anomaly {
	return finding(feed, session, observe.SignalVolume)
}

// shape renders conditions as "s1,s2|s3" per locus, for a compact comparison.
func shape(cs []Condition, locus string) string {
	var parts []string
	for _, c := range cs {
		if c.Locus == locus {
			parts = append(parts, strings.Join(c.Sessions(), ","))
		}
	}
	return strings.Join(parts, "|")
}

// Consecutive deliveries with a finding are one condition, a delivery with none
// ends it, and a session that carries a finding and no registration is a
// position in its place.
func TestAConditionIsARunOfConsecutivePositions(t *testing.T) {
	d1, d2, d3, d4, d5 := sid(day(1)), sid(day(8)), sid(day(15)), sid(day(22)), sid(day(29))
	u := sid(day(30))
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}, [2]string{d3, "k3"},
			[2]string{d4, "k4"}, [2]string{d5, "k5"}),
		Anomalies: []observe.Anomaly{departs("f", d1), departs("f", d2), departs("f", d4), departs("f", d5),
			departs("f", u)}}
	cs := h.Conditions()
	if got, want := shape(cs, LocusDeliveryDepartsProfile), d1+","+d2+"|"+d4+","+d5+","+u; got != want {
		t.Fatalf("conditions %s, want %s", got, want)
	}
	first, second := cs[0], cs[1]
	if first.EndedBy != d3 || first.End != EndNoFinding || first.Open() {
		t.Errorf("the first condition ended by %q (%s), open %v; want ended by the delivery with no finding, %s",
			first.EndedBy, first.End, first.Open(), d3)
	}
	if !second.Open() || second.Last().Delivery || !second.First().Delivery {
		t.Errorf("the second condition: open %v, its last a delivery %v; want open, ending at the unregistered session",
			second.Open(), second.Last().Delivery)
	}
	for _, want := range []string{"a condition of feed f at locus delivery_departs_profile: 3 deliveries",
		"1 of them a session with no registration of the feed", "it is open"} {
		if !strings.Contains(second.Describe(), want) {
			t.Errorf("Describe does not say %q:\n  %s", want, second.Describe())
		}
	}
	if !strings.Contains(first.Describe(), "it ended at session "+d3) {
		t.Errorf("Describe does not say where it ended:\n  %s", first.Describe())
	}
	if c := ConditionAt(cs, d5, LocusDeliveryDepartsProfile); c == nil || c.IndexOf(d5) != 1 {
		t.Errorf("ConditionAt does not find %s second in its condition", d5)
	}
}

// A re-registration of a file the feed already delivered is re-processing: it
// is not a position, so it neither breaks a condition nor extends one.
func TestAReRegistrationNeitherBreaksNorExtendsACondition(t *testing.T) {
	d1, d2, again, d3 := sid(day(1)), sid(day(8)), sid(day(9)), sid(day(15))
	remap := sid(day(20))
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}, [2]string{again, "k1"},
			[2]string{d3, "k3"}, [2]string{remap, "k2"}),
		Anomalies: []observe.Anomaly{departs("f", d2), departs("f", d3)}}
	cs := h.Conditions()
	if got, want := shape(cs, LocusDeliveryDepartsProfile), d2+","+d3; got != want {
		t.Fatalf("conditions %s, want %s: the re-registration between them is not a position", got, want)
	}
	if !cs[0].Open() {
		t.Errorf("the condition ended at %s: a re-map after the latest delivery does not end it", cs[0].EndedBy)
	}
	if ps := h.Positions(); len(ps) != 3 {
		t.Errorf("%d positions, want the three deliveries", len(ps))
	}
}

// A delivery that was not judged carries no finding, so it ends a condition as
// a delivery judged clean does: one incident too many rather than one too few.
func TestADeliveryNotJudgedEndsACondition(t *testing.T) {
	d1, d2, notJudged, d4 := sid(day(1)), sid(day(8)), sid(day(15)), sid(day(22))
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}, [2]string{notJudged, "k3"},
			[2]string{d4, "k4"}),
		Anomalies: []observe.Anomaly{departs("f", d1), departs("f", d2), departs("f", d4)}}
	if got, want := shape(h.Conditions(), LocusDeliveryDepartsProfile), d1+","+d2+"|"+d4; got != want {
		t.Errorf("conditions %s, want %s", got, want)
	}
}

// Before the feed's earliest surviving registration, and everywhere with no
// input_registry, two positions are consecutive only on one UTC day.
func TestPurgedDeliveriesFallBackToOneUTCDay(t *testing.T) {
	a, b := sid(day(2)), sid(day(2)+60_000) // one day
	c := sid(day(3))                        // the next day: the deliveries between cannot be seen
	d1 := sid(day(3) + 3_600_000)           // the earliest surviving delivery, the same day as c
	d2 := sid(day(10))
	anomalies := []observe.Anomaly{departs("f", a), departs("f", b), departs("f", c), departs("f", d1),
		departs("f", d2)}
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}), Anomalies: anomalies}
	cs := h.Conditions()
	if got, want := shape(cs, LocusDeliveryDepartsProfile), a+","+b+"|"+c+","+d1+","+d2; got != want {
		t.Fatalf("conditions %s, want %s", got, want)
	}
	if !cs[0].SameDay || cs[0].End != EndDayBoundary || cs[0].EndedBy != c {
		t.Errorf("the purged condition: same day %v, ended by %s (%s); want ended at the day boundary by %s",
			cs[0].SameDay, cs[0].EndedBy, cs[0].End, c)
	}
	if !strings.Contains(cs[0].Describe(), "earliest surviving registration") {
		t.Errorf("Describe does not say why it ended:\n  %s", cs[0].Describe())
	}

	// No input_registry: every link is judged by the day.
	none := &FeedHistory{Feed: "f", Registry: false, Anomalies: anomalies}
	if got, want := shape(none.Conditions(), LocusDeliveryDepartsProfile), a+","+b+"|"+c+","+d1+"|"+d2; got != want {
		t.Errorf("with no registry, conditions %s, want %s", got, want)
	}
}

// A session carrying findings of two feeds at one locus is a condition of one
// for each, and ends the conditions either side of it.
func TestATwoFeedSessionIsAConditionOfOne(t *testing.T) {
	d1, d2, d3 := sid(day(1)), sid(day(8)), sid(day(15))
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}, [2]string{d3, "k3"}),
		Anomalies:  []observe.Anomaly{departs("f", d1), departs("f", d2), departs("g", d2), departs("f", d3)}}
	cs := h.Conditions()
	if got, want := shape(cs, LocusDeliveryDepartsProfile), d1+"|"+d2+"|"+d3; got != want {
		t.Fatalf("conditions %s, want %s", got, want)
	}
	if !cs[1].Shared || cs[0].End != EndOtherFeed || cs[1].Open() {
		t.Errorf("shared %v, first ended %s, shared open %v", cs[1].Shared, cs[0].End, cs[1].Open())
	}
	// Another feed's finding at another locus is no matter.
	h.Anomalies[2] = finding("g", d2, observe.SignalArrival)
	if got, want := shape(h.Conditions(), LocusDeliveryDepartsProfile), d1+","+d2+","+d3; got != want {
		t.Errorf("with the other feed at another locus, conditions %s, want %s", got, want)
	}
}

// A session whose id is not a number has no position: its finding is a
// condition of one, which is never open.
func TestANonNumericSessionIsAConditionOfOne(t *testing.T) {
	d1 := sid(day(1))
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}),
		Anomalies:  []observe.Anomaly{departs("f", d1), departs("f", "manual-load"), departs("f", "manual-load-2")}}
	cs := h.Conditions()
	if got, want := shape(cs, LocusDeliveryDepartsProfile), d1+"|manual-load|manual-load-2"; got != want {
		t.Fatalf("conditions %s, want %s", got, want)
	}
	if !cs[1].Unordered || cs[1].Open() || !strings.Contains(cs[1].Describe(), "is not a time") {
		t.Errorf("unordered %v, open %v: %s", cs[1].Unordered, cs[1].Open(), cs[1].Describe())
	}
}

// The key is the feed and the locus, not the detector: two detectors' findings
// on one delivery are one member, and the two delivery loci are two conditions.
func TestAConditionIsKeyedOnTheLocus(t *testing.T) {
	d1, d2 := sid(day(1)), sid(day(8))
	other := departs("f", d2)
	other.AnomalyId, other.DetectorRef, other.SignalType = "a/other", "delivery_backdated@1", observe.SignalFreshness
	other.DetectedAt = other.DetectedAt.Add(-time.Minute)
	h := &FeedHistory{Feed: "f", Registry: true,
		Deliveries: registrations("f", [2]string{d1, "k1"}, [2]string{d2, "k2"}),
		Anomalies: []observe.Anomaly{departs("f", d1), departs("f", d2), other,
			finding("f", d2, observe.SignalArrival)}}
	cs := h.Conditions()
	if got := shape(cs, LocusDeliveryDepartsProfile); got != d1+","+d2 {
		t.Errorf("departs-profile conditions %s, want one of both deliveries", got)
	}
	if got := shape(cs, LocusDeliveryOverdue); got != d2 {
		t.Errorf("overdue conditions %s, want %s alone", got, d2)
	}
	m := ConditionAt(cs, d2, LocusDeliveryDepartsProfile).Members[1]
	if !slices.Equal(m.Detectors, []string{"delivery_backdated@1", "delivery_size@1"}) ||
		!m.FirstDetected.Equal(other.DetectedAt) || len(m.Anomalies) != 2 {
		t.Errorf("member %+v: want both detectors, the earliest detection first", m)
	}
}

// The condition reader orders a feed's registrations as the feed detectors do,
// and reads input_registry alone, so the conditions can be computed from an
// export of four tables.
func TestTheConditionReaderOrdersAsTheDetectorsDo(t *testing.T) {
	if !strings.HasSuffix(feedRegistrationsSQL, "ORDER BY "+observe.FeedRegistrationOrderSQL) {
		t.Errorf("feedRegistrationsSQL does not end in registration order: %s", feedRegistrationsSQL)
	}
	if strings.Count(feedRegistrationsSQL, "jetsapi.") != 1 || !strings.Contains(feedRegistrationsSQL, "jetsapi.input_registry ir") {
		t.Errorf("feedRegistrationsSQL reads more than input_registry: %s", feedRegistrationsSQL)
	}
}
