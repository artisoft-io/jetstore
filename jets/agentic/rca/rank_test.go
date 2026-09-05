package rca

import (
	"fmt"
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The floor's behaviour, over triage reports built here rather than read from a
// database. rank_pg_test.go is the same ranker over reports the nine predicates
// produced against a real Postgres; these are the properties that do not need
// one, and they are the ones that say what the package guarantees.

// report builds a nine-verdict triage report. Loci not named are Absent, which
// is the ordinary case and is deliberately not the zero value of anything: an
// unnamed locus in this helper is one that was evaluated and did not hold.
func report(session string, verdicts map[string]triage.Verdict,
	confounders map[string][]string) *triage.Report {

	r := &triage.Report{SessionId: session}
	for _, l := range triage.Loci {
		v, ok := verdicts[l]
		if !ok {
			v = triage.Absent
		}
		f := triage.Finding{
			SessionId:     session,
			Locus:         l,
			Verdict:       v,
			Basis:         fmt.Sprintf("fixture basis for %s at verdict %s", l, v),
			Confounders:   confounders[l],
			ClassifierRef: "triage@1",
		}
		if err := f.Validate(); err != nil {
			panic("the fixture built an invalid finding: " + err.Error())
		}
		if v != triage.NotEvaluable {
			r.Evaluable++
		}
		r.Findings = append(r.Findings, f)
	}
	return r
}

// The one thing criterion 45 asks for, asserted over every hypothesis the floor
// can emit rather than over a chosen example.
//
// The census is deliberate: it fires every locus §9.5 maps, so every one of the
// eight emittable classes is produced at least once and each is checked for
// both sides.
func TestEveryHypothesisCarriesBothSidesOfTheCase(t *testing.T) {
	all := map[string]triage.Verdict{}
	for _, l := range triage.Loci {
		all[l] = triage.Present
	}
	got := Default().Rank(&Input{Report: report("s-all", all, nil)})
	if err := got.Validate(); err != nil {
		t.Fatalf("the ranking does not validate: %v", err)
	}
	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses over a report in which every locus is present")
	}
	seen := map[string]bool{}
	withNothingAgainst := 0
	for i := range got.Hypotheses {
		h := &got.Hypotheses[i]
		seen[h.CauseCategory] = true
		if len(h.SupportingEvidence) == 0 {
			t.Errorf("%s at %s has no supporting evidence", h.CauseCategory, h.Locus)
		}
		if h.ContradictingEvidence == nil {
			t.Errorf("%s at %s has nil contradicting evidence, which Validate should have refused",
				h.CauseCategory, h.Locus)
		}
		if len(h.ContradictingEvidence) == 0 {
			withNothingAgainst++
			t.Logf("%s at %s has an empty case against it", h.CauseCategory, h.Locus)
		}
		if h.Basis == "" || !strings.Contains(h.Basis, "ranked") {
			t.Errorf("%s at %s has no readable ranking basis: %q", h.CauseCategory, h.Locus, h.Basis)
		}
	}
	t.Logf("%d hypotheses over %d present loci; %d classes reached; %d with an empty case against",
		len(got.Hypotheses), len(triage.Loci), len(seen), withNothingAgainst)

	// The two classes §9.5 attaches to no locus can never be emitted, whatever
	// fired. That is I-262 as a property of the output rather than of the table.
	for _, never := range []string{CauseDependencyFailure, CauseCapacityOrCostDeviation} {
		if seen[never] {
			t.Errorf("class %s was emitted; §9.5 attaches it to no locus, so nothing in the record can "+
				"put it in front of a reader", never)
		}
	}
}

// **The negative control for the whole package.** A hypothesis whose
// contradicting evidence was never considered must not be a value this package
// will pass on, and nil rather than empty is what "never considered" looks like
// in Go.
func TestNilContradictingEvidenceIsRefused(t *testing.T) {
	h := Hypothesis{
		Cause:              "something broke",
		CauseCategory:      CauseParseFailure,
		Locus:              triage.LocusPerRecordFailuresReported,
		Confidence:         0.9,
		Rank:               1,
		Basis:              "because I said so",
		RankerRef:          "test@1",
		SupportingEvidence: []Evidence{{Statement: "a thing", Source: SourceRunTelemetry}},
		// ContradictingEvidence deliberately left nil.
	}
	err := h.Validate()
	if err == nil {
		t.Fatal("a hypothesis with nil contradicting evidence validated; the calibration control is not " +
			"enforced and §A.2.8's required column is decorative")
	}
	if !strings.Contains(err.Error(), "calibration control") {
		t.Errorf("the message does not say why the field is required: %v", err)
	}
	// And an explicitly empty list is accepted, because asserting that nothing
	// was found against a hypothesis is a claim an agent is allowed to make.
	h.ContradictingEvidence = []Evidence{}
	if err := h.Validate(); err != nil {
		t.Errorf("an empty contradicting list was refused: %v", err)
	}
}

// A hypothesis must say which of the two vocabularies its claim is in, which is
// Q-27's answer as a field rather than as a convention. AD.1 scores per locus
// and reports cause-class denominators beside it; a hypothesis with no locus is
// countable in neither.
func TestAHypothesisWithoutALocusIsRefused(t *testing.T) {
	h := Hypothesis{
		Cause:                 "something broke",
		CauseCategory:         CauseParseFailure,
		Confidence:            0.5,
		Rank:                  1,
		Basis:                 "b",
		RankerRef:             "test@1",
		SupportingEvidence:    []Evidence{{Statement: "a thing", Source: SourceRunTelemetry}},
		ContradictingEvidence: []Evidence{},
	}
	if err := h.Validate(); err == nil {
		t.Fatal("a hypothesis with no locus validated")
	}
	h.Locus = triage.LocusPerRecordFailuresReported
	if err := h.Validate(); err != nil {
		t.Errorf("a hypothesis naming its locus was refused: %v", err)
	}
	// An invented source is refused too, which matters here more than it would
	// on triage.Finding: the evidence columns are jsonb and Postgres will
	// accept anything.
	h.SupportingEvidence[0].Source = "the_internet"
	if err := h.Validate(); err == nil {
		t.Fatal("an evidence source outside the vocabulary validated; nothing else in the stack " +
			"would have caught it, the columns being jsonb")
	}
}

// **A run in which nothing fired produces no hypothesis, and the basis says the
// ranking asserts nothing rather than asserting health.** This is the negative
// control for the failure a ranker is most likely to have: a default answer.
func TestNoPresentLocusYieldsNoHypothesis(t *testing.T) {
	got := Default().Rank(&Input{Report: report("s-clean", nil, nil)})
	if len(got.Hypotheses) != 0 {
		t.Fatalf("%d hypotheses over a report in which no locus is present: %v",
			len(got.Hypotheses), got.Hypotheses[0].CauseCategory)
	}
	if !strings.Contains(got.Basis, "asserts nothing about the run") {
		t.Errorf("the basis reads as though the run were healthy: %s", got.Basis)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("an empty ranking does not validate: %v", err)
	}
}

// A locus §9.5 maps to no class is reported rather than dropped. A caller
// counting hypotheses per locus otherwise cannot tell a locus that produced
// none from one that was never asked, which is the same conflation
// triage.NotEvaluable exists to prevent, one level up.
func TestAnUnmappedLocusIsReportedRatherThanDropped(t *testing.T) {
	got := Default().Rank(&Input{Report: report("s-2", map[string]triage.Verdict{
		triage.LocusStepNeverStarted: triage.Present,
	}, nil)})
	if len(got.Hypotheses) != 0 {
		t.Errorf("locus step_never_started produced %d hypotheses; §9.5 lists it in no row",
			len(got.Hypotheses))
	}
	if len(got.UnmappedLoci) != 1 || got.UnmappedLoci[0] != triage.LocusStepNeverStarted {
		t.Errorf("UnmappedLoci is %v, want [step_never_started]", got.UnmappedLoci)
	}
	if !strings.Contains(got.Basis, "map to no cause class") {
		t.Errorf("the ranking's basis does not say a locus fired and produced nothing: %s", got.Basis)
	}
}

// **The confounder split, which is the one piece of judgement in the floor.**
// A configured behaviour is the case *for* benign variation and against every
// other class; a limit on what the record could see is against all ten.
func TestAConfiguredBehaviourSupportsBenignAndContradictsTheRest(t *testing.T) {
	in := &Input{Report: report("s-6", map[string]triage.Verdict{
		triage.LocusRowsLostSilently: triage.Present,
	}, map[string][]string{
		triage.LocusRowsLostSilently: {observe.ConfounderOnErrorDrop, observe.ConfounderHistoryTruncated},
	})}
	got := Default().Rank(in)
	found := 0
	for i := range got.Hypotheses {
		h := &got.Hypotheses[i]
		forDrop := citesConfounder(h.SupportingEvidence, observe.ConfounderOnErrorDrop)
		againstDrop := citesConfounder(h.ContradictingEvidence, observe.ConfounderOnErrorDrop)
		againstHistory := citesConfounder(h.ContradictingEvidence, observe.ConfounderHistoryTruncated)
		if !againstHistory {
			t.Errorf("%s at %s does not carry history_truncated against it; a bounded comparison "+
				"weakens every class", h.CauseCategory, h.Locus)
		}
		switch h.CauseCategory {
		case CauseBenignVariation:
			found++
			if !forDrop || againstDrop {
				t.Errorf("on_error_drop is for=%v against=%v on benign_variation; a configured drop is "+
					"the case for benign rather than a doubt about it", forDrop, againstDrop)
			}
		default:
			if forDrop || !againstDrop {
				t.Errorf("on_error_drop is for=%v against=%v on %s; a configured drop is an alternative "+
					"explanation to this class", forDrop, againstDrop, h.CauseCategory)
			}
		}
	}
	if found != 1 {
		t.Fatalf("benign_variation was emitted %d times over a present rows_lost_silently", found)
	}
}

func citesConfounder(items []Evidence, name string) bool {
	for _, e := range items {
		if e.Source == SourceDetectorConfounder && strings.Contains(e.Statement, name) {
			return true
		}
	}
	return false
}

// A class whose other evidence positions could not be evaluated has a bounded
// case rather than a complete one, and the wording distinguishes that from a
// position that was asked and said no. Both are contradicting evidence and they
// are not the same claim.
func TestAnUnaskedSiblingLocusBoundsTheCaseRatherThanRefutingIt(t *testing.T) {
	got := Default().Rank(&Input{Report: report("s-t", map[string]triage.Verdict{
		// transport_failure sits at loci 1, 4, 5 and 9. Fire 4, make 5
		// unanswerable — the ordinary state, its table being deployed nowhere
		// measured — and leave 1 and 9 absent.
		triage.LocusWorkerFailed:                   triage.Present,
		triage.LocusSinkFailedUnderCompletedWorker: triage.NotEvaluable,
		triage.LocusWrittenNotArrived:              triage.NotEvaluable,
	}, nil)})

	var transport *Hypothesis
	for i := range got.Hypotheses {
		if got.Hypotheses[i].CauseCategory == CauseTransportFailure {
			transport = &got.Hypotheses[i]
		}
	}
	if transport == nil {
		t.Fatal("no transport_failure hypothesis over a present worker_failed")
	}
	bounded, refuted := 0, 0
	for _, e := range transport.ContradictingEvidence {
		if strings.Contains(e.Statement, "bounded rather than complete") {
			bounded++
		}
		if strings.Contains(e.Statement, "was evaluated and does not hold") {
			refuted++
		}
	}
	if bounded != 2 {
		t.Errorf("%d bounded-case items, want 2 (loci 5 and 9 are not evaluable)", bounded)
	}
	if refuted != 1 {
		t.Errorf("%d refuting items, want 1 (locus 1 was evaluated and is absent)", refuted)
	}
	if len(got.UnaskedLoci) != 2 {
		t.Errorf("UnaskedLoci is %v, want the two not-evaluable loci", got.UnaskedLoci)
	}
}

// Locus per_record_failures_unreportable maps to no cause class and is not
// therefore useless: §9.4 row 8's *cannot see* is that the record cannot
// distinguish it from a clean run, so a run in that state cannot be called
// benign on the record's silence. It is the only cross-locus rule in the floor.
func TestAnUnreportableOperatorContradictsBenign(t *testing.T) {
	with := Default().Rank(&Input{Report: report("s-b1", map[string]triage.Verdict{
		triage.LocusRowsLostSilently:              triage.Present,
		triage.LocusPerRecordFailuresUnreportable: triage.Present,
	}, nil)})
	without := Default().Rank(&Input{Report: report("s-b2", map[string]triage.Verdict{
		triage.LocusRowsLostSilently: triage.Present,
	}, nil)})

	benignOf := func(r *Ranking) *Hypothesis {
		for i := range r.Hypotheses {
			if r.Hypotheses[i].CauseCategory == CauseBenignVariation {
				return &r.Hypotheses[i]
			}
		}
		return nil
	}
	a, b := benignOf(with), benignOf(without)
	if a == nil || b == nil {
		t.Fatal("benign_variation was not emitted in one of the two rankings")
	}
	if len(a.ContradictingEvidence) != len(b.ContradictingEvidence)+1 {
		t.Errorf("an unreportable operator added %d items against benign, want 1",
			len(a.ContradictingEvidence)-len(b.ContradictingEvidence))
	}
	if a.Confidence >= b.Confidence {
		t.Errorf("benign is no less confident with an unreportable operator present: %.3f vs %.3f",
			a.Confidence, b.Confidence)
	}
}

// Two rankings over the same evidence must be identical, including the order.
// The cheapest way that breaks is a map iteration reaching the sort, which is
// triage.TestClassificationIsStable's reasoning one package over.
func TestRankingIsStable(t *testing.T) {
	mk := func() *Ranking {
		return Default().Rank(&Input{Report: report("s-s", map[string]triage.Verdict{
			triage.LocusRunNotStarted:                  triage.Present,
			triage.LocusWorkerFailed:                   triage.Present,
			triage.LocusRowsLostSilently:               triage.Present,
			triage.LocusPerRecordFailuresReported:      triage.Present,
			triage.LocusSinkFailedUnderCompletedWorker: triage.NotEvaluable,
		}, map[string][]string{
			triage.LocusRowsLostSilently: {observe.ConfounderSamplingCap, observe.ConfounderHistoryTruncated},
		})})
	}
	first := mk()
	for i := 0; i < 5; i++ {
		next := mk()
		if len(first.Hypotheses) != len(next.Hypotheses) {
			t.Fatalf("run %d produced %d hypotheses, run 0 produced %d", i+1,
				len(next.Hypotheses), len(first.Hypotheses))
		}
		for j := range first.Hypotheses {
			a, b := &first.Hypotheses[j], &next.Hypotheses[j]
			if a.HypothesisId != b.HypothesisId || a.Rank != b.Rank || a.Confidence != b.Confidence ||
				a.Basis != b.Basis || len(a.SupportingEvidence) != len(b.SupportingEvidence) ||
				len(a.ContradictingEvidence) != len(b.ContradictingEvidence) {
				t.Fatalf("run %d disagrees with run 0 at rank %d:\n  %+v\n  %+v", i+1, j+1, a, b)
			}
		}
		if first.Basis != next.Basis {
			t.Fatalf("run %d's ranking basis differs from run 0's", i+1)
		}
	}
}

// **Criterion 45's last clause, asserted rather than asserted about.** A reader
// must be able to check the ranking by counting the two lists, so the basis has
// to name both counts and the ratio it made of them.
func TestTheRankingBasisIsCheckableByCounting(t *testing.T) {
	got := Default().Rank(&Input{Report: report("s-r", map[string]triage.Verdict{
		triage.LocusRunNotStarted:    triage.Present,
		triage.LocusWorkerFailed:     triage.Present,
		triage.LocusRowsLostSilently: triage.Present,
	}, nil)})
	if len(got.Hypotheses) < 2 {
		t.Fatalf("expected several hypotheses, got %d", len(got.Hypotheses))
	}
	for i := range got.Hypotheses {
		h := &got.Hypotheses[i]
		sup, con := len(h.SupportingEvidence), len(h.ContradictingEvidence)
		want := fmt.Sprintf("%d/%d = %.2f", sup, sup+con, h.Confidence)
		if !strings.Contains(h.Basis, want) {
			t.Errorf("rank %d's basis does not carry the arithmetic %q:\n  %s", h.Rank, want, h.Basis)
		}
		if !strings.Contains(h.Basis, h.Locus) {
			t.Errorf("rank %d's basis does not name the locus it came from:\n  %s", h.Rank, h.Basis)
		}
	}
	// And the ranking is in confidence order *within* §9.5's evidenceability
	// tier, with every class the record cannot evidence below every class it
	// can. Asserting plain confidence order here would pass the version of this
	// package that ranked source_delivery_failure first on a run that never
	// started, which is the defect the tier exists for.
	seenUnevidenceable := false
	for i := range got.Hypotheses {
		h := &got.Hypotheses[i]
		un := evidenceabilityOf(h.CauseCategory) == None
		if un {
			seenUnevidenceable = true
		} else if seenUnevidenceable {
			t.Errorf("rank %d (%s) is a class the record can evidence and sits below one it cannot",
				h.Rank, h.CauseCategory)
		}
		if i > 0 {
			p := &got.Hypotheses[i-1]
			if (evidenceabilityOf(p.CauseCategory) == None) == un && p.Confidence < h.Confidence {
				t.Errorf("rank %d is less confident than rank %d within one tier", p.Rank, h.Rank)
			}
		}
	}
	t.Logf("ranking basis: %s", got.Basis)
	for i := range got.Hypotheses {
		t.Logf("  %d. %-28s at %-34s conf %.2f  (+%d/-%d)", got.Hypotheses[i].Rank,
			got.Hypotheses[i].CauseCategory, got.Hypotheses[i].Locus, got.Hypotheses[i].Confidence,
			len(got.Hypotheses[i].SupportingEvidence), len(got.Hypotheses[i].ContradictingEvidence))
	}
}

// An anomaly's confounders reach the case against, which is §9.8's instruction
// in one assertion: the contradicting side is populated from Anomaly.Confounders
// before any model is asked for one. An anomaly whose signal §9.4 has no locus
// for reaches no hypothesis and is named in the ranking's basis instead.
func TestAnomalyConfoundersReachTheCaseAgainst(t *testing.T) {
	in := &Input{
		Report: report("s-a", map[string]triage.Verdict{
			triage.LocusRowsLostSilently: triage.Present,
		}, nil),
		Anomalies: []observe.Anomaly{
			{
				AnomalyId: "an-1", SessionId: "s-a", SubjectType: observe.SubjectWorker,
				SubjectRef: "reducing01/0", SignalType: observe.SignalVolume,
				ObservedValue: "0.01", ExpectedBasis: "within-run input against output",
				Confounders: []string{observe.ConfounderMergeRowCountUnknown},
				DetectorRef: "volume_collapse@1",
			},
			{
				AnomalyId: "an-2", SessionId: "s-a", SubjectType: observe.SubjectStage,
				SubjectRef: "reducing01", SignalType: observe.SignalStepRegression,
				ObservedValue: "failed", ExpectedBasis: "30 days of prior runs",
				Confounders: []string{observe.ConfounderHistoryTruncated},
				DetectorRef: "step_regression@1",
			},
		},
	}
	got := Default().Rank(in)
	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses")
	}
	for i := range got.Hypotheses {
		h := &got.Hypotheses[i]
		if !citesConfounder(h.ContradictingEvidence, observe.ConfounderMergeRowCountUnknown) {
			t.Errorf("%s does not carry the volume detector's confounder against it", h.CauseCategory)
		}
		if citesConfounder(h.ContradictingEvidence, observe.ConfounderHistoryTruncated) {
			t.Errorf("%s carries a confounder from an anomaly whose signal maps to no locus", h.CauseCategory)
		}
	}
	if !strings.Contains(got.Basis, "an-2 (step_regression)") {
		t.Errorf("the ranking's basis does not name the anomaly that reached no hypothesis: %s", got.Basis)
	}
}
