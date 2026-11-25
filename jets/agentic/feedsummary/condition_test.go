package feedsummary

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/feedcondition"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// week is the session id of the feed's delivery in week w of 2023.
func week(w int) string {
	return strconv.FormatInt(time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 7*w).UnixMilli(), 10)
}

func weekAt(w int) time.Time { return time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 7*w) }

func incOf(w int) string { return "inc_" + week(w) + "_" + triage.LocusDeliveryDepartsProfile }

// weeklyFeed builds a feed of n weekly deliveries whose findings at the
// departs-profile locus sit on the weeks named, each detected an hour after its
// delivery.
func weeklyFeed(n int, fired ...int) *feedcondition.Feed {
	h := &triage.FeedHistory{Feed: "acme_pharmacy", Registry: true}
	r := &observe.FeedRegistrations{ByTable: map[string][]observe.FeedDelivery{}}
	for w := 0; w < n; w++ {
		r.ByTable["acme_pharmacy"] = append(r.ByTable["acme_pharmacy"], observe.FeedDelivery{SessionId: week(w),
			FileKey: "k" + strconv.Itoa(w), Arrival: weekAt(w)})
	}
	h.Deliveries = r.Deliveries("acme_pharmacy", time.Time{})
	for _, w := range fired {
		h.Anomalies = append(h.Anomalies, observe.Anomaly{AnomalyId: "a" + strconv.Itoa(w), SessionId: week(w),
			DetectedAt: weekAt(w).Add(time.Hour), SubjectType: observe.SubjectFeed, SubjectRef: "acme_pharmacy",
			SignalType: observe.SignalVolume, DetectorRef: "delivery_size@1"})
	}
	return feedcondition.FromHistory(h)
}

func raise(f *feedcondition.Feed, w int, verdictAt time.Time) {
	inc := feedcondition.Incident{IncidentId: incOf(w), SessionId: week(w), Locus: triage.LocusDeliveryDepartsProfile,
		Status: audit.IncidentDiagnosed}
	if !verdictAt.IsZero() {
		inc.Verdict = &audit.IncidentTransition{ToStatus: audit.IncidentSuppressedAsBenign, Actor: "person@example.com",
			ActorKind: audit.ActorHuman, TransitionedAt: verdictAt}
	}
	f.AddIncident(inc)
}

// summarise renders departs() with the condition an incident on week w holds,
// and fails on anything the counter finds untrue of it.
func summarise(t *testing.T, f *feedcondition.Feed, w int) (*Condition, string) {
	t.Helper()
	c := conditionOf(f, incOf(w), week(w), triage.LocusDeliveryDepartsProfile)
	if c == nil {
		t.Fatalf("no condition holds week %d", w)
	}
	in := departs()
	in.IncidentID = incOf(w)
	in.Condition = c
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	if n := CountUntrue(s.Text, in); n.Untrue() != 0 {
		t.Errorf("the counter finds untrue statements in the render:\n%s\n%s", n, s.Text)
	}
	return c, strings.Join(strings.Fields(s.Text), " ")
}

func wantSaid(t *testing.T, flat string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(flat, want) {
			t.Errorf("the summary does not say %q:\n%s", want, flat)
		}
	}
}

func wantUnsaid(t *testing.T, flat string, nots ...string) {
	t.Helper()
	for _, not := range nots {
		if strings.Contains(flat, not) {
			t.Errorf("the summary says %q:\n%s", not, flat)
		}
	}
}

// The weekly repeat: one condition of eight deliveries, still open, its
// incident on the first and the seven later deliveries joining it.
func TestAMultiDeliveryConditionIsSummarisedAsOne(t *testing.T) {
	f := weeklyFeed(8, 0, 1, 2, 3, 4, 5, 6, 7)
	raise(f, 0, time.Time{})
	c, flat := summarise(t, f, 0)
	if c.Held != 8 || c.Total != 8 || c.End != EndOpen || c.Previous.Kind != PrevNone || len(c.Others) != 0 {
		t.Fatalf("%+v", c)
	}
	wantSaid(t, flat,
		"This incident stands for eight deliveries of the feed with a finding at this locus, registered from 2 January 2023 to 20 February 2023.",
		"The findings above are those of the delivery the incident is written on",
		"It is still open: the feed's latest delivery carries the finding.",
		"No earlier delivery of the feed carries a finding at this locus.",
		"give it one verdict")
	wantUnsaid(t, flat, "ended", "of the eight in its condition")
}

// An interrupted condition is two incidents. The first ended with a delivery
// with no finding; the second names the first's incident with its verdict, read
// when the summary is read, so a second condition raised before the first (a
// backfill triaged newest first) names the first's incident once it exists.
func TestAnInterruptedConditionNamesThePreviousIncidentLive(t *testing.T) {
	f := weeklyFeed(8, 0, 1, 2, 4, 5, 6)
	raise(f, 4, time.Time{})
	c, flat := summarise(t, f, 4)
	if c.Previous.Kind != PrevCondition || c.Previous.IncidentID != "" || c.Previous.Deliveries != 3 {
		t.Fatalf("before the first is raised: %+v", c.Previous)
	}
	wantSaid(t, flat, "The feed's previous condition at this locus, three deliveries registered from 2 January "+
		"2023 to 16 January 2023, has no incident yet.")

	raise(f, 0, time.Time{})
	c, flat = summarise(t, f, 4)
	if c.Held != 3 || c.End != EndNoFinding || !c.EndedAt.Equal(weekAt(7)) || c.Previous.IncidentID != incOf(0) {
		t.Fatalf("the second: %+v", c)
	}
	wantSaid(t, flat,
		"It ended with the feed's next delivery, registered on 20 February 2023, which carries no finding at this locus",
		"has incident "+incOf(0)+", with no verdict.")

	verdictAt := weekAt(3).Add(2 * time.Hour)
	f = weeklyFeed(8, 0, 1, 2, 4, 5, 6)
	raise(f, 0, verdictAt)
	raise(f, 4, time.Time{})
	_, flat = summarise(t, f, 4)
	wantSaid(t, flat, "has incident "+incOf(0)+", with a person's verdict, suppressed_as_benign on 23 January 2023.")

	c, flat = summarise(t, f, 0)
	if c.Held != 3 || c.End != EndNoFinding || !c.EndedAt.Equal(weekAt(3)) || c.Previous.Kind != PrevNone {
		t.Fatalf("the first: %+v", c)
	}
	wantSaid(t, flat, "registered from 2 January 2023 to 16 January 2023.",
		"It ended with the feed's next delivery, registered on 23 January 2023, which carries no finding")
	wantUnsaid(t, flat, "still open")
}

// A person's verdict mid-condition: deliveries found after it are new
// evidence, with no incident until triage reaches them and then one of their
// own, which names the judged incident and its verdict.
func TestAConditionOpenedAfterAVerdict(t *testing.T) {
	verdictAt := weekAt(2).Add(2 * time.Hour) // after week 2's detection, before week 3's
	f := weeklyFeed(6, 0, 1, 2, 3, 4, 5)
	raise(f, 0, verdictAt)
	c, flat := summarise(t, f, 0)
	if c.Held != 3 || c.Total != 6 || c.Unraised != 3 || c.End != EndVerdict || c.EndedInto != "" ||
		c.Verdict != audit.IncidentSuppressedAsBenign {
		t.Fatalf("before the new evidence is raised: %+v", c)
	}
	wantSaid(t, flat,
		"This incident stands for three deliveries of the feed with a finding at this locus, of the six in its condition",
		"It was closed by a person's verdict, suppressed_as_benign on 16 January 2023: the condition's next "+
			"delivery, registered on 23 January 2023, was found after the verdict, so it is new evidence and not "+
			"part of what was judged; it has no incident yet.",
		"Three deliveries of its condition have no incident yet")

	raise(f, 3, time.Time{})
	c, flat = summarise(t, f, 0)
	if c.EndedInto != incOf(3) || !slices.Equal(c.Others, []string{incOf(3)}) || c.Unraised != 0 {
		t.Fatalf("after: %+v", c)
	}
	wantSaid(t, flat, "it belongs to incident "+incOf(3)+".")

	c, flat = summarise(t, f, 3)
	if c.Held != 3 || c.End != EndOpen || c.Previous.Kind != PrevAfterVerdict || c.Previous.IncidentID != incOf(0) {
		t.Fatalf("the new incident: %+v", c)
	}
	wantSaid(t, flat,
		"registered from 23 January 2023 to 6 February 2023",
		"It is still open",
		"It was opened after a person's verdict on incident "+incOf(0)+", earlier in the same condition, "+
			"suppressed_as_benign on 16 January 2023")
}

// Incidents written one per delivery before conditions stay; each holds its
// own delivery, names the others, and says the condition goes on past it.
func TestIncidentsWrittenPerDeliveryEachHoldTheirOwn(t *testing.T) {
	f := weeklyFeed(4, 0, 1, 2, 3)
	for w := 0; w < 4; w++ {
		raise(f, w, time.Time{})
	}
	c, flat := summarise(t, f, 1)
	if c.Held != 1 || c.Total != 4 || c.End != EndNextIncident || c.EndedInto != incOf(2) ||
		!slices.Equal(c.Others, []string{incOf(0), incOf(2), incOf(3)}) || c.Previous.Kind != "" {
		t.Fatalf("%+v", c)
	}
	wantSaid(t, flat,
		"This incident stands for one delivery of the feed with a finding at this locus, of the four in its "+
			"condition, registered on 9 January 2023.",
		"The condition goes on past it: its next delivery, registered on 16 January 2023, belongs to incident "+incOf(2)+".",
		"Other deliveries of its condition belong to other incidents: "+incOf(0)+", "+incOf(2)+", "+incOf(3)+".")
	wantUnsaid(t, flat, "give it one verdict", "previous condition")
}

// A session whose id is not a time is a condition of one.
func TestADeliveryWithNoPlaceInTheOrderIsAConditionOfOne(t *testing.T) {
	f := weeklyFeed(2, 0)
	f.History.Anomalies = append(f.History.Anomalies, observe.Anomaly{AnomalyId: "x", SessionId: "manual",
		DetectedAt: weekAt(1), SubjectType: observe.SubjectFeed, SubjectRef: "acme_pharmacy",
		SignalType: observe.SignalVolume, DetectorRef: "delivery_size@1"})
	f = feedcondition.FromHistory(f.History)
	c := conditionOf(f, "inc_manual_"+triage.LocusDeliveryDepartsProfile, "manual", triage.LocusDeliveryDepartsProfile)
	if c == nil || c.End != EndUnordered || c.Held != 1 || c.Unregistered != 1 || c.Previous.Kind != "" {
		t.Fatalf("%+v", c)
	}
	in := departs()
	in.IncidentID = "inc_manual_" + triage.LocusDeliveryDepartsProfile
	in.Condition = c
	s, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(s.Text), " ")
	wantSaid(t, flat, "The delivery's session id is not a time, so it has no place in the feed's order")
	wantUnsaid(t, flat, "This incident stands for", "earlier delivery")
	if n := CountUntrue(s.Text, in); n.Untrue() != 0 {
		t.Errorf("the counter finds untrue statements in the render:\n%s", n)
	}
}

// The counter's three condition categories, and the condition's facts in the
// identifier and date categories: a draft's false claims about a condition are
// counted, and its true ones are not.
func TestTheCounterReadsTheCondition(t *testing.T) {
	open := departs() // three deliveries, 11 to 25 May, open; previous of two, suppressed as benign
	ended := departs()
	ended.Condition.End, ended.Condition.EndedAt = EndNoFinding, day(2026, 6, 1)
	noVerdict := departs()
	noVerdict.Condition.Previous = Previous{Kind: PrevNone}
	goesOn := departs()
	goesOn.Condition.End, goesOn.Condition.EndedInto = EndNextIncident, "inc_1748000000000_delivery_departs_profile"
	goesOn.Condition.EndedAt = day(2026, 6, 1)
	for _, tc := range []struct {
		name  string
		in    *Input
		draft string
		cat   string
		want  int
	}{
		// How many deliveries.
		{"right count", open, "The condition holds 3 deliveries.", CatDeliveries, 0},
		{"right count in words", open, "It stands for three weekly deliveries.", CatDeliveries, 0},
		{"previous count", open, "The previous condition had two deliveries.", CatDeliveries, 0},
		{"wrong count", open, "The condition holds 5 deliveries.", CatDeliveries, 1},
		{"wrong count in words", open, "It stands for eight consecutive weekly deliveries.", CatDeliveries, 1},
		{"one delivery, wrongly", open, "Only one delivery is affected.", CatDeliveries, 1},
		{"one of the deliveries", open, "One of the deliveries is late.", CatDeliveries, 0},
		// From when to when.
		{"right span", open, "Registered from 11 May 2026 to 25 May 2026.", CatDate, 0},
		{"wrong end date", open, "Registered from 11 May 2026 to 26 May 2026.", CatDate, 1},
		{"previous span", open, "The previous condition ran from 16 February 2026 to 23 February 2026.", CatDate, 0},
		// Open or ended.
		{"open, said open", open, "The condition is still open.", CatOpen, 0},
		{"open, said ended", open, "The condition has ended.", CatOpen, 1},
		{"open, said closed", open, "It is closed now.", CatOpen, 1},
		{"ended, said ended", ended, "It ended on 1 June 2026.", CatOpen, 0},
		{"ended, said open", ended, "The condition is still open.", CatOpen, 1},
		{"ended, said ongoing", ended, "This is an ongoing problem.", CatOpen, 1},
		{"goes on, either", goesOn, "It ended, and the condition is still open.", CatOpen, 0},
		// The previous verdict.
		{"the verdict given", open, "The previous incident was suppressed as benign.", CatVerdict, 0},
		{"the verdict by name", open, "Its verdict was suppressed_as_benign.", CatVerdict, 0},
		{"another verdict", open, "The previous incident was reclassified.", CatVerdict, 1},
		{"no verdict to give", noVerdict, "The previous incident was suppressed as benign.", CatVerdict, 1},
		{"verified, none given", noVerdict, "A person verified the previous incident.", CatVerdict, 1},
		// The previous incident.
		{"the previous incident", open, "See inc_1740000000000_delivery_departs_profile.", CatIdentifier, 0},
		{"a previous incident that is not", open, "See inc_1739000000000_delivery_departs_profile.", CatIdentifier, 1},
		{"the next incident", goesOn, "It goes on as inc_1748000000000_delivery_departs_profile.", CatIdentifier, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, cat := range CountUntrue(tc.draft, tc.in).Categories {
				if cat.Name == tc.cat {
					found = true
					if len(cat.Findings) != tc.want {
						t.Errorf("%q: %d findings %v, want %d", tc.draft, len(cat.Findings), cat.Findings, tc.want)
					}
				}
			}
			if !found {
				t.Errorf("no category %q", tc.cat)
			}
		})
	}
}
