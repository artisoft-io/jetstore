package rca

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artisoft-io/jetstore/jets/agentic/feedprofile"
	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

func intPtr(n int) *int           { return &n }
func floatPtr(x float64) *float64 { return &x }

// feedInput is one registration session of feed a_feed with the given feed
// anomalies, ranked through triage's own classifier so that the verdicts are
// the ones shadow mode would write.
func feedInput(anomalies ...observe.Anomaly) *Input {
	d := &triage.DeclaredFeed{TableName: "a_feed", Configured: true, Rows: 1, Profiled: true,
		Feed: feedprofile.Feed{Client: "client_a", Org: "org_a", ObjectType: "Claim", TableName: "a_feed",
			Profile: feedprofile.Profile{PeriodType: feedprofile.WeekPeriod, OverdueAfterDays: intPtr(3),
				ClaimSet: &feedprofile.ClaimSet{Table: "claims", KeyColumns: []string{"claim_id"},
					ServiceDateColumn: "service_date", MemberColumn: "member_id"},
				RequiredFields: []feedprofile.RequiredField{{Column: "paid_amount", MinPresentShare: floatPtr(0.95)}},
				DeclaredBy:     "someone@example.com", DeclaredAt: "2026-03-01"}}}
	ev := &triage.Evidence{SessionId: "1700000000000", Feed: &triage.FeedEvidence{
		Read:          true,
		Anomalies:     anomalies,
		Registrations: []triage.Registration{{TableName: "a_feed", Client: "Any"}},
		Feeds:         map[string]*triage.DeclaredFeed{"a_feed": d},
	}}
	return &Input{Report: triage.Default().Classify(ev), Evidence: ev, Anomalies: anomalies}
}

func feedAnomalyOf(detector, signal, suffix string) observe.Anomaly {
	return observe.Anomaly{AnomalyId: detector + "@1/a_feed/1700000000000" + suffix, SessionId: "1700000000000",
		SubjectType: observe.SubjectFeed, SubjectRef: "a_feed", SignalType: signal, ObservedValue: "x",
		ExpectedBasis: "the profile", DetectorRef: detector + "@1",
		DetectedAt: time.Date(2026, 6, 1, 5, 0, 0, 0, time.UTC), Confounders: []string{}}
}

func hypothesisOf(r *Ranking, locus, class string) *Hypothesis {
	for i := range r.Hypotheses {
		if r.Hypotheses[i].Locus == locus && r.Hypotheses[i].CauseCategory == class {
			return &r.Hypotheses[i]
		}
	}
	return nil
}

func anyStatement(es []Evidence, parts ...string) bool {
	for _, e := range es {
		ok := true
		for _, p := range parts {
			ok = ok && strings.Contains(e.Statement, p)
		}
		if ok {
			return true
		}
	}
	return false
}

// The criterion in one test: a source_delivery_failure hypothesis on
// delivery_overdue carries the declared cadence as evidence for it, and every
// feed hypothesis carries the declaration's date and author as evidence
// against.
func TestAFeedHypothesisCarriesItsDeclarationOnBothSides(t *testing.T) {
	got := Default().Rank(feedInput(feedAnomalyOf(feedprofile.FeedOverdue, observe.SignalArrival, "")))
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	var classes []string
	for _, h := range got.Hypotheses {
		classes = append(classes, h.CauseCategory)
	}
	if want := []string{CauseSourceDeliveryFailure, CauseBenignVariation}; !slices.Equal(classes, want) {
		t.Fatalf("delivery_overdue ranked %v, want %v", classes, want)
	}
	sdf := hypothesisOf(got, triage.LocusDeliveryOverdue, CauseSourceDeliveryFailure)
	if !anyStatement(sdf.SupportingEvidence, "declares a delivery every 1 week_period with 3 days' grace") {
		t.Errorf("source_delivery_failure does not carry the declared cadence for it: %+v", sdf.SupportingEvidence)
	}
	// The condition its grade names is met, so the table's "conditional" item
	// is not charged against it a second time.
	if anyStatement(sdf.ContradictingEvidence, "ability to evidence") {
		t.Errorf("a met condition is still charged as conditional: %+v", sdf.ContradictingEvidence)
	}
	for _, h := range got.Hypotheses {
		if !anyStatement(h.ContradictingEvidence, "declared by someone@example.com on 2026-03-01",
			"92 days before the finding was detected", "client client_a") {
			t.Errorf("%s at %s does not carry the declaration's author, date and age against it: %+v",
				h.CauseCategory, h.Locus, h.ContradictingEvidence)
		}
		for _, side := range [][]Evidence{h.SupportingEvidence, h.ContradictingEvidence} {
			if anyStatement(side, "of client Any") {
				t.Errorf("%s names the registration's client as the feed's", h.CauseCategory)
			}
		}
	}
	// No run locus was charged: a registration session is not asked about a
	// run, and a run locus's verdict is no evidence about a delivery.
	for _, h := range got.Hypotheses {
		if anyStatement(h.ContradictingEvidence, "run_not_started") {
			t.Errorf("%s at %s was charged with a run locus", h.CauseCategory, h.Locus)
		}
	}
	if sdf.Rank != 1 {
		t.Errorf("source_delivery_failure ranks %d on an overdue feed with a declared cadence, want 1", sdf.Rank)
	}
}

// A departure offers three classes, and source_content_change is evidenced at
// column grain only where the profile names the column the check compared.
func TestADepartureOffersThreeClassesAndColumnGrainOnlyWhereAColumnIsNamed(t *testing.T) {
	got := Default().Rank(feedInput(feedAnomalyOf(feedprofile.FieldCompleteness, observe.SignalRuleBreach,
		"/paid_amount")))
	for _, c := range []string{CauseSourceContentChange, CauseTransformationDefect, CauseBenignVariation} {
		h := hypothesisOf(got, triage.LocusDeliveryDepartsProfile, c)
		if h == nil {
			t.Fatalf("no %s hypothesis at delivery_departs_profile", c)
		}
		if !anyStatement(h.ContradictingEvidence, "declared by someone@example.com on 2026-03-01") {
			t.Errorf("%s does not carry the declaration against it", c)
		}
	}
	scc := hypothesisOf(got, triage.LocusDeliveryDepartsProfile, CauseSourceContentChange)
	if !anyStatement(scc.SupportingEvidence, "column grain", `required field "paid_amount"`) {
		t.Errorf("a required-field departure is not evidenced at column grain: %+v", scc.SupportingEvidence)
	}
	if g := EvidenceabilityAt(scc.CauseCategory, scc.Locus); g != Conditional {
		t.Errorf("source_content_change at delivery_departs_profile is graded %q", g)
	}

	// A size departure names no column, so the condition is not met and the
	// table's own reason is charged against the class.
	got = Default().Rank(feedInput(feedAnomalyOf(feedprofile.DeliverySize, observe.SignalVolume, "")))
	scc = hypothesisOf(got, triage.LocusDeliveryDepartsProfile, CauseSourceContentChange)
	if anyStatement(scc.SupportingEvidence, "column grain") {
		t.Errorf("a size departure claims column grain")
	}
	if !anyStatement(scc.ContradictingEvidence, `is "conditional"`, "names the column") {
		t.Errorf("a size departure does not carry the conditional grade against it: %+v", scc.ContradictingEvidence)
	}
}

// A feed whose profile cannot be re-read still carries a case against: it says
// the declaration cannot be weighed, rather than losing the item.
func TestAnUnreadableDeclarationIsStillACaseAgainst(t *testing.T) {
	in := feedInput(feedAnomalyOf(feedprofile.FeedOverdue, observe.SignalArrival, ""))
	in.Evidence.Feed.Feeds["a_feed"].Profiled = false
	got := Default().Rank(in)
	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses")
	}
	for _, h := range got.Hypotheses {
		if !anyStatement(h.ContradictingEvidence, "cannot be re-read") {
			t.Errorf("%s does not say its declaration cannot be re-read", h.CauseCategory)
		}
	}
	sdf := hypothesisOf(got, triage.LocusDeliveryOverdue, CauseSourceDeliveryFailure)
	if anyStatement(sdf.SupportingEvidence, "declared cadence") {
		t.Errorf("a cadence nobody can read is cited for the class")
	}
}

// Onset order: on a session that is a delivery and a run, a delivery locus
// breaks a tie ahead of a run locus, and the nine run loci keep their order.
func TestTheDeliveryLociBreakATieFirst(t *testing.T) {
	if locusOrder(triage.LocusDeliveryOverdue) >= locusOrder(triage.LocusRunNotStarted) {
		t.Errorf("delivery_overdue does not precede run_not_started in onset order")
	}
	if locusOrder(triage.LocusDeliveryOverdue) >= locusOrder(triage.LocusDeliveryDepartsProfile) {
		t.Errorf("delivery_overdue does not precede delivery_departs_profile in onset order")
	}
	prev := -1
	for _, l := range triage.Loci {
		if slices.Contains(triage.DeliveryLoci, l) {
			continue
		}
		if o := locusOrder(l); o <= prev {
			t.Errorf("run locus %s moved in onset order", l)
		} else {
			prev = o
		}
	}
}
