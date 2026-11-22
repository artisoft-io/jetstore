package feedproposal

import (
	"regexp"
	"strings"
	"testing"
)

func share(x float64) *float64 { return &x }

// deliveries builds a feed's history from a pattern: 'o' is an overlap share
// of 0.97, 'l' a low overlap share of 0.2, 'u' unchanged, '.' nothing.
func deliveries(pattern string) []Delivery {
	var out []Delivery
	for i, c := range pattern {
		d := Delivery{SessionID: string(rune('a' + i)), Ordinal: i + 1}
		switch c {
		case 'o':
			d.Share = share(0.97)
		case 'l':
			d.Share = share(0.2)
		case 'u':
			d.Unchanged = true
		}
		out = append(out, d)
	}
	return out
}

func TestTheRunOfEvidenceEndsAtTheLatestDelivery(t *testing.T) {
	for _, c := range []struct {
		pattern string
		want    int
	}{
		{".ooooooo", 7},  // the misdeclared feed: every delivery after the first
		{"........o", 1}, // one restated delivery
		{"..ooo....", 0}, // a cumulative stretch the feed has since left
		{"o.o.o.o.o", 1}, // restatements scattered through a history
		{"..oul", 0},     // a low share is not evidence, and ends the run
		{"..ouo", 3},     // unchanged counts as evidence
		{"", 0},
	} {
		got := trailingEvidence(deliveries(c.pattern), DefaultMinShare)
		if len(got) != c.want {
			t.Errorf("%q: a run of %d, want %d", c.pattern, len(got), c.want)
		}
		if len(got) > 0 && got[len(got)-1].Ordinal != len(c.pattern) {
			t.Errorf("%q: the run does not end at the latest delivery", c.pattern)
		}
	}
}

func TestOptionsRefuseARunOfOne(t *testing.T) {
	o := DefaultOptions("0.1.0")
	if err := o.check(); err != nil {
		t.Fatal(err)
	}
	o.MinStreak = 1
	if err := o.check(); err == nil {
		t.Error("a run of one is accepted, so one restated delivery would draft a proposal")
	}
	o = DefaultOptions("")
	if err := o.check(); err == nil {
		t.Error("an empty model version is accepted")
	}
}

func TestTheProposalIdIsStableAndNamesNothing(t *testing.T) {
	a := ProposalIDFor(CaseCumulativeUnderIncremental, "some_feed")
	if a != ProposalIDFor(CaseCumulativeUnderIncremental, "some_feed") {
		t.Error("the id is not deterministic")
	}
	if a == ProposalIDFor(CaseCumulativeUnderIncremental, "another_feed") ||
		a == ProposalIDFor("another_case", "some_feed") {
		t.Error("two (feed, case) pairs share an id")
	}
	if !regexp.MustCompile(`^chg_[a-z0-9]+$`).MatchString(a) || strings.Contains(a, "some_feed") {
		t.Errorf("id %q does not match the model's pattern or names the feed", a)
	}
}

func TestTheRationaleSaysWhatChangesAndWhatDoesNot(t *testing.T) {
	ds := deliveries(".ooo")
	ds[1].IncidentID, ds[1].IncidentStatus = "inc_b_delivery_departs_profile", "diagnosed"
	a := &Assessment{TableName: "some_feed", Client: "c", ObjectType: "Claim", Deliveries: ds,
		Streak: ds[1:], Qualifies: true, TriggerIncident: ds[1].IncidentID,
		SharedObjectType: 2, Undeclared: 1}
	o := DefaultOptions("0.1.0")
	p := Build(a, o)
	if p.ApprovalState != "draft" || p.Trigger != Trigger || p.TriggerRef != ds[1].IncidentID {
		t.Errorf("built as %s, %s %s", p.ApprovalState, p.Trigger, p.TriggerRef)
	}
	for _, s := range []string{
		"setting source_config.is_cumulative to 1", "only the latest delivery across the whole lookback",
		"overlap share 0.9700", "leaves the object type mixed", "Nothing has been applied",
		"stays in draft until a person moves it", "no org",
	} {
		if !strings.Contains(p.Rationale, s) {
			t.Errorf("the rationale does not say %q:\n%s", s, p.Rationale)
		}
	}
}
