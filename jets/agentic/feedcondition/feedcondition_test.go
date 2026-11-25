package feedcondition

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/audit"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

const locus = triage.LocusDeliveryDepartsProfile

// week is the session id of the feed's delivery in week w of 2023.
func week(w int) string {
	return strconv.FormatInt(time.Date(2023, 1, 2, 12, 0, 0, 0, time.UTC).AddDate(0, 0, 7*w).UnixMilli(), 10)
}

func at(session string) time.Time {
	ms, _ := strconv.ParseInt(session, 10, 64)
	return time.UnixMilli(ms).UTC()
}

// feed builds a feed of n weekly deliveries whose findings at the locus sit on
// the weeks named, each detected an hour after its delivery.
func feed(n int, fired ...int) *Feed {
	h := &triage.FeedHistory{Feed: "f", Registry: true}
	r := &observe.FeedRegistrations{ByTable: map[string][]observe.FeedDelivery{}}
	for w := 0; w < n; w++ {
		r.ByTable["f"] = append(r.ByTable["f"], observe.FeedDelivery{SessionId: week(w),
			FileKey: "k" + strconv.Itoa(w), Arrival: at(week(w))})
	}
	h.Deliveries = r.Deliveries("f", time.Time{})
	for _, w := range fired {
		h.Anomalies = append(h.Anomalies, observe.Anomaly{AnomalyId: "a" + strconv.Itoa(w), SessionId: week(w),
			DetectedAt: at(week(w)).Add(time.Hour), SubjectType: observe.SubjectFeed, SubjectRef: "f",
			SignalType: observe.SignalVolume, DetectorRef: "delivery_size@1"})
	}
	return FromHistory(h)
}

func incidentOn(f *Feed, w int, verdictAt time.Time) {
	inc := Incident{IncidentId: "inc_" + week(w), SessionId: week(w), Locus: locus, Status: audit.IncidentDiagnosed}
	if !verdictAt.IsZero() {
		inc.Verdict = &audit.IncidentTransition{ToStatus: audit.IncidentSuppressedAsBenign, Actor: "person@example.com",
			ActorKind: audit.ActorHuman, TransitionedAt: verdictAt}
	}
	f.AddIncident(inc)
}

func TestAConditionWithNoIncidentIsRaisedOnItsFirstDelivery(t *testing.T) {
	f := feed(6, 1, 2, 3, 4)
	if p := f.Resolve(week(1), locus); p.Kind != WriteHere || p.Anchor != week(1) {
		t.Errorf("the first delivery: %s on %s, want written here", p.Kind, p.Anchor)
	}
	p := f.Resolve(week(4), locus)
	if p.Kind != RaiseAnchor || p.Anchor != week(1) {
		t.Fatalf("the last delivery: %s on %s, want the first delivery's incident raised", p.Kind, p.Anchor)
	}
	b := p.Basis()
	for _, want := range []string{"4 deliveries", "It is written on the condition's first delivery",
		"No earlier delivery of the feed has a finding at this locus"} {
		if !strings.Contains(b, want) {
			t.Errorf("the basis does not say %q:\n  %s", want, b)
		}
	}

	incidentOn(f, 1, time.Time{})
	if p := f.Resolve(week(1), locus); p.Kind != AlreadyRaised {
		t.Errorf("the first delivery after its incident: %s", p.Kind)
	}
	for _, w := range []int{2, 3, 4} {
		if p := f.Resolve(week(w), locus); p.Kind != Join || p.Target.IncidentId != "inc_"+week(1) {
			t.Errorf("week %d: %s, want joined to the first delivery's incident", w, p.Kind)
		}
	}
}

// An incident already written on a later delivery, as a day's burst was
// written on its last before conditions, is never moved: the condition's
// earlier deliveries join it, and a later one joins the latest at or before it.
func TestAnIncidentNeverMoves(t *testing.T) {
	f := feed(6, 1, 2, 3, 4, 5)
	incidentOn(f, 3, time.Time{})
	for w, want := range map[int]string{1: week(3), 2: week(3), 4: week(3), 5: week(3)} {
		if p := f.Resolve(week(w), locus); p.Kind != Join || p.Target.SessionId != want {
			t.Errorf("week %d: %s into %v, want joined to week 3's", w, p.Kind, p.Target)
		}
	}
	incidentOn(f, 5, time.Time{})
	if p := f.Resolve(week(4), locus); p.Target.SessionId != week(3) {
		t.Errorf("week 4 joins %s, want the latest at or before it, week 3's", p.Target.SessionId)
	}
}

// After a person's verdict, a finding detected later is new evidence and is
// written on its own delivery, naming the judged incident; one detected before
// it joins.
func TestAVerdictClosesTheConditionForLaterEvidence(t *testing.T) {
	f := feed(6, 0, 1, 2, 3)
	verdict := at(week(2)).Add(30 * time.Minute) // after week 2 arrived, before its finding was detected
	incidentOn(f, 0, verdict)

	if p := f.Resolve(week(1), locus); p.Kind != Join {
		t.Errorf("a finding detected before the verdict: %s, want joined to the judged incident", p.Kind)
	}
	for _, w := range []int{2, 3} {
		p := f.Resolve(week(w), locus)
		if p.Kind != WriteHere || p.Judged == nil || p.Previous == nil || p.Previous.IncidentId != "inc_"+week(0) {
			t.Fatalf("week %d after the verdict: %s, judged %v, previous %v", w, p.Kind, p.Judged, p.Previous)
		}
		for _, want := range []string{"is new evidence", "suppressed_as_benign by person@example.com",
			"The previous incident at this locus is inc_" + week(0)} {
			if !strings.Contains(p.Basis(), want) {
				t.Errorf("week %d: the basis does not say %q:\n  %s", w, want, p.Basis())
			}
		}
	}
	// Once week 2 is written, week 3 joins it: the latest incident at or
	// before it carries no verdict.
	incidentOn(f, 2, time.Time{})
	if p := f.Resolve(week(3), locus); p.Kind != Join || p.Target.SessionId != week(2) {
		t.Errorf("week 3 after week 2's incident: %s into %v", p.Kind, p.Target)
	}
}

// A condition after a break names the previous condition's incident and its
// verdict, or, before that incident is written, where it will be.
func TestANewConditionNamesThePreviousIncident(t *testing.T) {
	f := feed(8, 1, 2, 4, 5)
	p := f.Resolve(week(5), locus)
	if p.PreviousCondition == nil || p.Previous != nil ||
		!strings.Contains(p.Basis(), "has no incident yet; when it is raised it is written on its first delivery, "+
			"session "+week(1)) {
		t.Errorf("before the previous condition is raised:\n  %s", p.Basis())
	}
	incidentOn(f, 1, time.Time{})
	p = f.Resolve(week(5), locus)
	if p.Kind != RaiseAnchor || p.Anchor != week(4) || p.Previous == nil || p.Previous.SessionId != week(1) {
		t.Fatalf("%s on %s, previous %v", p.Kind, p.Anchor, p.Previous)
	}
	if !strings.Contains(p.Basis(), "inc_"+week(1)+", on session "+week(1)+", with no verdict") {
		t.Errorf("the basis does not name the previous incident:\n  %s", p.Basis())
	}
}
