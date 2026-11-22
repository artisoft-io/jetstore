package triage

import (
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

func intp(n int) *int { return &n }

// feedSession is a registration session of feed a_feed, whose source
// configuration names client client_a, registered under client Any.
func feedSession(anomalies ...observe.Anomaly) *Evidence {
	d := &DeclaredFeed{TableName: "a_feed", Configured: true, Rows: 1, Profiled: true,
		Feed: feedprofile.Feed{Client: "client_a", Org: "org_a", ObjectType: "Claim", TableName: "a_feed",
			Profile: feedprofile.Profile{PeriodType: feedprofile.WeekPeriod, OverdueAfterDays: intp(3),
				SizeBytes:  &feedprofile.SizeRange{},
				DeclaredBy: "someone@example.com", DeclaredAt: "2026-03-01"}}}
	return &Evidence{SessionId: "1700000000000", Feed: &FeedEvidence{
		Read:          true,
		Anomalies:     anomalies,
		Registrations: []Registration{{TableName: "a_feed", Client: "Any", Org: ""}},
		Feeds:         map[string]*DeclaredFeed{"a_feed": d},
	}}
}

func feedAnomaly(id, signal string) observe.Anomaly {
	return observe.Anomaly{AnomalyId: id, SessionId: "1700000000000", SubjectType: observe.SubjectFeed,
		SubjectRef: "a_feed", SignalType: signal, ObservedValue: "2026-03-30",
		ExpectedBasis: "the declared cadence", DetectorRef: "feed_overdue@1",
		DetectedAt: time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC), Confounders: []string{}}
}

func findingAt(r *Report, locus string) Finding {
	for _, f := range r.Findings {
		if f.Locus == locus {
			return f
		}
	}
	return Finding{}
}

// A delivery locus holds on the anomaly that lands there, and its basis names
// the feed by its source configuration: the feed's client, never the client the
// delivery was registered under.
func TestADeliveryLocusHoldsOnItsAnomalyAndNamesTheFeedsOwnClient(t *testing.T) {
	r := Default().Classify(feedSession(feedAnomaly("feed_overdue@1/a_feed/1700000000000", observe.SignalArrival)))
	f := findingAt(r, LocusDeliveryOverdue)
	if f.Verdict != Present {
		t.Fatalf("delivery_overdue is %s, want present: %s", f.Verdict, f.Basis)
	}
	if err := f.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"client client_a", "registered under client Any", "the one after which nothing came",
		"feed_overdue@1"} {
		if !strings.Contains(f.Basis, want) {
			t.Errorf("the basis does not say %q:\n  %s", want, f.Basis)
		}
	}
	if strings.Contains(f.Basis, "of client Any") {
		t.Errorf("the basis names the registration's client as the feed's:\n  %s", f.Basis)
	}
	if len(f.Subjects) != 1 || !strings.Contains(f.Subjects[0], "feed_overdue@1/a_feed/") {
		t.Errorf("subjects are %v, want the anomaly", f.Subjects)
	}
	// The other delivery locus was asked of a delivery of a profiled feed and
	// carries no departure: absent, not unasked.
	if g := findingAt(r, LocusDeliveryDepartsProfile); g.Verdict != Absent {
		t.Errorf("delivery_departs_profile is %s, want absent: %s", g.Verdict, g.Basis)
	}
	// And no run locus fired on a session that has no run.
	for _, g := range r.Fired() {
		if g.Locus != LocusDeliveryOverdue {
			t.Errorf("run locus %s fired on a registration session", g.Locus)
		}
	}
}

// A departure lands at delivery_departs_profile, whatever its signal, and never
// at delivery_overdue.
func TestADepartureLandsAtDeliveryDepartsProfile(t *testing.T) {
	a := feedAnomaly("delivery_size@1/a_feed/1700000000000", observe.SignalVolume)
	a.DetectorRef = "delivery_size@1"
	r := Default().Classify(feedSession(a))
	if f := findingAt(r, LocusDeliveryDepartsProfile); f.Verdict != Present {
		t.Errorf("delivery_departs_profile is %s, want present", f.Verdict)
	}
	if f := findingAt(r, LocusDeliveryOverdue); f.Verdict != Absent {
		t.Errorf("delivery_overdue is %s, want absent: the feed declares a cadence and no arrival anomaly "+
			"hangs here", f.Verdict)
	}
}

// Not asked is not absent: a session that is not a delivery of any profiled
// feed, which is every run's own session, and a database with no anomaly table.
func TestADeliveryLocusIsNotEvaluableWhereNothingWasAsked(t *testing.T) {
	run := &Evidence{SessionId: "s-run", Feed: &FeedEvidence{Read: true, Registrations: []Registration{},
		Feeds: map[string]*DeclaredFeed{}}}
	r := Default().Classify(run)
	for _, l := range DeliveryLoci {
		if f := findingAt(r, l); f.Verdict != NotEvaluable {
			t.Errorf("%s on a run's session is %s, want not_evaluable", l, f.Verdict)
		}
	}
	none := &Evidence{SessionId: "s-run", Feed: &FeedEvidence{Note: "jetsapi.anomaly is not deployed"}}
	r = Default().Classify(none)
	for _, l := range DeliveryLoci {
		if f := findingAt(r, l); f.Verdict != NotEvaluable || !strings.Contains(f.Basis, "not deployed") {
			t.Errorf("%s with no anomaly table is %s (%s), want not_evaluable naming the table", l, f.Verdict, f.Basis)
		}
	}
	// A profiled feed that declares no cadence is not asked delivery_overdue.
	// It is still asked delivery_departs_profile, because the size check judges
	// every profiled feed against its own trailing band and needs no field.
	ev := feedSession()
	ev.Feed.Feeds["a_feed"].Feed.Profile = feedprofile.Profile{DeclaredBy: "x", DeclaredAt: "2026-03-01"}
	r = Default().Classify(ev)
	if f := findingAt(r, LocusDeliveryOverdue); f.Verdict != NotEvaluable {
		t.Errorf("delivery_overdue on a feed that declares no cadence is %s, want not_evaluable", f.Verdict)
	}
	if f := findingAt(r, LocusDeliveryDepartsProfile); f.Verdict != Absent {
		t.Errorf("delivery_departs_profile on a profiled feed is %s, want absent", f.Verdict)
	}
}

// A feed anomaly whose table no source configuration names still holds the
// locus, and the basis says no client can be named rather than naming one.
func TestAnUnconfiguredFeedIsNamedAsSuch(t *testing.T) {
	ev := feedSession(feedAnomaly("feed_overdue@1/other/1700000000000", observe.SignalArrival))
	ev.Feed.Anomalies[0].SubjectRef = "other"
	f := findingAt(Default().Classify(ev), LocusDeliveryOverdue)
	if f.Verdict != Present || !strings.Contains(f.Basis, "no client can be named") {
		t.Errorf("got %s: %s", f.Verdict, f.Basis)
	}
}
