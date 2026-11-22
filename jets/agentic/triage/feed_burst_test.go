package triage

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// burstOf is a burst of the overdue detector on a_feed over the given sessions.
func burstOf(sessions ...string) *Burst {
	return &Burst{Feed: "a_feed", Detector: "feed_overdue@1",
		Day: time.Date(2023, 11, 14, 0, 0, 0, 0, time.UTC), Sessions: sessions}
}

// The session evidence feedSession builds is 1700000000000. A burst it leads is
// written here, and names the others; a burst another session leads is not.
func TestABurstIsWrittenOnItsLastDelivery(t *testing.T) {
	a := feedAnomaly("feed_overdue@1/a_feed/1700000000000", observe.SignalArrival)

	ev := feedSession(a)
	ev.Feed.Bursts = map[string]*Burst{a.AnomalyId: burstOf("1699999000000", "1699999500000", "1700000000000")}
	f := findingAt(Default().Classify(ev), LocusDeliveryOverdue)
	if f.Verdict != Present || len(f.GroupedInto) != 0 {
		t.Fatalf("the burst's last delivery: verdict %s, grouped into %v", f.Verdict, f.GroupedInto)
	}
	if !slices.Equal(f.BurstSessions, []string{"1699999000000", "1699999500000"}) {
		t.Errorf("burst sessions %v, want the two earlier deliveries", f.BurstSessions)
	}
	for _, want := range []string{"fired on 3 deliveries registered on 2023-11-14",
		"this incident stands for all of them"} {
		if !strings.Contains(f.Basis, want) {
			t.Errorf("the basis does not say %q:\n  %s", want, f.Basis)
		}
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}

	ev = feedSession(a)
	ev.Feed.Bursts = map[string]*Burst{a.AnomalyId: burstOf("1700000000000", "1700000500000")}
	f = findingAt(Default().Classify(ev), LocusDeliveryOverdue)
	if f.Verdict != Present {
		t.Fatalf("a delivery in another session's burst is %s: the anomaly is there, so it holds", f.Verdict)
	}
	if !slices.Equal(f.GroupedInto, []string{"1700000500000"}) || len(f.BurstSessions) != 0 {
		t.Errorf("grouped into %v, burst sessions %v; want grouped into the last delivery only",
			f.GroupedInto, f.BurstSessions)
	}
	if !strings.Contains(f.Basis, "written on its last delivery, session 1700000500000, and not here") {
		t.Errorf("the basis does not say where the incident is:\n  %s", f.Basis)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
}

// An anomaly in no burst, or in a burst of one, keeps the finding here, even
// beside an anomaly another session's burst holds.
func TestAnAnomalyOutsideABurstKeepsItsIncident(t *testing.T) {
	a := feedAnomaly("feed_overdue@1/a_feed/1700000000000", observe.SignalArrival)
	for name, bursts := range map[string]map[string]*Burst{
		"no burst read":  nil,
		"a burst of one": {a.AnomalyId: burstOf("1700000000000")},
	} {
		ev := feedSession(a)
		ev.Feed.Bursts = bursts
		f := findingAt(Default().Classify(ev), LocusDeliveryOverdue)
		if f.Verdict != Present || len(f.GroupedInto) != 0 || len(f.BurstSessions) != 0 {
			t.Errorf("%s: verdict %s, grouped into %v, burst sessions %v", name, f.Verdict, f.GroupedInto,
				f.BurstSessions)
		}
	}

	b := feedAnomaly("feed_overdue@2/a_feed/1700000000000", observe.SignalArrival)
	ev := feedSession(a, b)
	ev.Feed.Bursts = map[string]*Burst{a.AnomalyId: burstOf("1700000000000", "1700000500000")}
	f := findingAt(Default().Classify(ev), LocusDeliveryOverdue)
	if len(f.GroupedInto) != 0 {
		t.Errorf("a finding with an anomaly in no burst is grouped into %v", f.GroupedInto)
	}
}

// Validate refuses a grouping on a verdict that does not hold, and a finding
// that both leads a burst and is grouped into another.
func TestValidateRefusesAnImpossibleGrouping(t *testing.T) {
	base := Finding{SessionId: "1", Locus: LocusDeliveryOverdue, Basis: "b", ClassifierRef: "triage@1"}
	absent := base
	absent.Verdict, absent.GroupedInto = Absent, []string{"2"}
	if absent.Validate() == nil {
		t.Error("an absent verdict grouped into a burst validates")
	}
	both := base
	both.Verdict, both.GroupedInto, both.BurstSessions = Present, []string{"2"}, []string{"0"}
	if both.Validate() == nil {
		t.Error("a finding that leads a burst and is grouped into another validates")
	}
}
