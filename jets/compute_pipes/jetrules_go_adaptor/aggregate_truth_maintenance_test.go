package jetrules_go_adaptor

// Does an aggregate over an entity property see a child linked after the
// aggregate was first computed?
//
// It does not, and this file is the demonstration. `min_of`, `max_of` and
// `sum_values` register their truth-maintenance callback on
// `jets:value_property` and never on `jets:entity_property`
// (`MinMaxOp.RegisterCallback`, `jets/jetrules/rete/expr_operator_math_minmax.go`,
// and `registerCallback4MinMaxOf`, `jets/rete/expr_op_arithmetics.h`). So a
// change to an already-linked child's value is recomputed and **a newly linked
// child is not**.
//
// `CE_RxDateRange10` is where that costs something real. It aggregates a
// pharmacy event's date range and days supply over `hasPharmacyClaim`, and the
// claims are linked to the event *by a rule* rather than being present when the
// session starts, so the aggregate is computed against however many claims
// happened to be linked at the moment it first fired.
//
// **These tests fail until agentic_ai I-535 is fixed.** That is deliberate: they
// assert what the rules mean, not what the engine currently does. Inverting them
// to assert 30-where-90-is-right would make the suite green and lock in the
// defect.
//
// Run them the way the sibling completeness tests are run:
//
//	WORKSPACES_HOME=<...>/workspaces WORKSPACE=jets_ws \
//	  go test -count=1 ./jets/compute_pipes/jetrules_go_adaptor/
//
// The `-count=1` is not optional: the corpus lives outside the Go module, so a
// cached `ok` survives a workspace change and is indistinguishable from a pass.

import (
	"testing"

	"github.com/artisoft-io/jetstore/jets/jetrules/rdf"
)

// theThreeFillEvent returns the pharmacy event carrying the fixture's three
// lisinopril fills. `AM_PCreateEvent20` keys the event index by NDC, so three
// claims for one NDC are one event with three `hasPharmacyClaim` links.
func theThreeFillEvent(t *testing.T, s *rdf.RdfSession, rm *rdf.ResourceManager) *rdf.Node {
	t.Helper()
	root := oneSubjectOfType(t, s, rm, "am:AnalysisRoot")
	profile := object(t, s, rm, root, "am:hasPatientProfile")
	for _, ev := range objects(s, rm, profile, "cintel:has_Pharmacy_Events") {
		if len(objects(s, rm, ev, "_0:hasPharmacyClaim")) == 3 {
			return ev
		}
	}
	t.Fatal("no pharmacy event with three linked claims; the fixture no longer " +
		"carries three fills of one NDC, so these tests are measuring nothing")
	return nil
}

// The control, and it is what makes the three failures below mean something.
//
// A union check over the whole profile would pass while every aggregate was
// wrong. What this asserts is narrower and load-bearing: **all three claims are
// linked to the event, and a rule that iterates over them sees all three.**
// `AM_PCreateEvent50` binds `?clm01` in its antecedent so it fires once per
// claim; `CE_RxDateRange10` aggregates and fires once. Same session, same links,
// same instant — one is right and the others are not.
//
// So a failure below cannot be explained by "the claims were never linked".
func TestAllThreeFillsReachTheEventAndAPerClaimRuleSeesThem(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	ev := theThreeFillEvent(t, s, rm)

	if got := len(objects(s, rm, ev, "_0:hasPharmacyClaim")); got != 3 {
		t.Fatalf("_0:hasPharmacyClaim links = %d, want 3", got)
	}
	if got := values(s, rm, ev, "cintel:Fill_Dates"); len(got) != 3 {
		t.Errorf("cintel:Fill_Dates = %v (%d values), want 3 — the per-claim rule "+
			"should see every linked claim", got, len(got))
	}
}

// sum_values: three fills of 30 days supply is 90, and the engine reports the
// days supply of however many claims were linked when the rule first fired.
func TestSumValuesSeesEveryLinkedClaim(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	ev := theThreeFillEvent(t, s, rm)

	got := values(s, rm, ev, "cintel:Total_Days_Supply")
	if len(got) != 1 {
		t.Fatalf("cintel:Total_Days_Supply = %v, want exactly one value", got)
	}
	if got[0] != "90" {
		t.Errorf("cintel:Total_Days_Supply = %s, want 90 (three fills of 30). "+
			"sum_values registered no callback on hasPharmacyClaim, so claims "+
			"linked after the first do not recompute it — agentic_ai I-535", got[0])
	}
}

// min_of and max_of, asserted together and with the shape assertion first.
//
// `Date_From == Date_To` is impossible for three claims with distinct service
// dates, and it is the assertion that does not depend on which claim happened to
// win the race. The exact-value checks below it say which way it went.
func TestMinOfAndMaxOfSpanEveryLinkedClaim(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	ev := theThreeFillEvent(t, s, rm)

	from := values(s, rm, ev, "cintel:Date_From")
	to := values(s, rm, ev, "cintel:Date_To")
	if len(from) != 1 || len(to) != 1 {
		t.Fatalf("cintel:Date_From = %v, cintel:Date_To = %v, want one value each", from, to)
	}
	if from[0] == to[0] {
		t.Errorf("cintel:Date_From == cintel:Date_To == %s, but the event has three "+
			"claims with distinct service dates. min_of and max_of aggregated a "+
			"subset — agentic_ai I-535", from[0])
	}
	if from[0] != "2025-06-15" {
		t.Errorf("cintel:Date_From = %s, want 2025-06-15 (min_of)", from[0])
	}
	if to[0] != "2025-08-24" {
		t.Errorf("cintel:Date_To = %s, want 2025-08-24 (max_of)", to[0])
	}
}

// And what it costs downstream, which is where the measurement got worse than the
// diagnosis.
//
// **Measured 2026-09-09: Span_Days is 66.** That is not what a one-claim
// aggregate would give (31), and it is not the truth (101). It is
// `((2025-08-24 + 30) - 2025-07-20) + 1` — the value `CE_RxNbrDays10` computes if
// it reads `Date_From` as 2025-07-20 while the test reads it, afterwards, as
// 2025-08-24.
//
// **So the aggregate is not merely incomplete, it is unstable within one
// session**: it changed after a downstream rule had already consumed it, and two
// consumers of the same property in the same run saw different values. A rule
// that reads an aggregate gets whatever it held at the moment that rule fired.
//
// That also disposes of the tidier reading that the first-linked claim wins.
// `min_of` finishes at **2025-08-24, the maximum of the three service dates** —
// so it is not returning a minimum over the linked set at all.
//
// `Span_Days` and `Adherence_Ratio` are computed from the two aggregates above,
// so a wrong span produces a plausible ratio rather than an obviously broken one:
// 90/101 = 0.891 is the truth and 30/31 = 0.968 is what a one-claim aggregate
// yields. **A member with a two-month gap in therapy scores 0.97**, which reads
// as excellent adherence, and it errs in the direction that reassures.
func TestAdherenceIsComputedOverTheWholeTherapy(t *testing.T) {
	s, rm := ruleSessionOverTheFixture(t)
	ev := theThreeFillEvent(t, s, rm)

	span := values(s, rm, ev, "cintel:Span_Days")
	if len(span) != 1 {
		t.Fatalf("cintel:Span_Days = %v, want exactly one value", span)
	}
	if span[0] != "101" {
		t.Errorf("cintel:Span_Days = %s, want 101 — 2025-06-15 to 2025-08-24 is 70 "+
			"days, plus the last fill's 30 days supply, plus 1 (CE_RxNbrDays10)", span[0])
	}
}
