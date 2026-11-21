package rca

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

// The anomaly-to-locus map, pinned as a table over (subject, signal).
//
// **It pins both halves of the change that keyed the map on the subject.** On a
// worker or an edge every signal lands where it landed when the map was keyed on
// the signal alone, which is what a regression in the run loci would break. On a
// feed the same words land at the two delivery loci, which is what the subject
// key is for: a feed's volume is a delivery of the wrong size, not a run that
// lost rows, and a feed's arrival is a delivery that never came, not a sink
// write that never landed.
//
// Every (subject, signal) pair in the two vocabularies is asserted, including the
// ones that map to nothing, so that a member added to either vocabulary is
// placed nowhere until somebody says where.
func TestLocusOfIsKeyedOnTheSubject(t *testing.T) {
	// The map as it stood when it was keyed on the signal alone.
	before := map[string]string{
		observe.SignalVolume:        triage.LocusRowsLostSilently,
		observe.SignalWorkerStall:   triage.LocusWorkerNotTerminated,
		observe.SignalArrival:       triage.LocusWrittenNotArrived,
		observe.SignalRejectionRate: triage.LocusPerRecordFailuresReported,
	}
	feed := map[string]string{
		observe.SignalArrival:      triage.LocusDeliveryOverdue,
		observe.SignalVolume:       triage.LocusDeliveryDepartsProfile,
		observe.SignalFreshness:    triage.LocusDeliveryDepartsProfile,
		observe.SignalDistribution: triage.LocusDeliveryDepartsProfile,
		observe.SignalCardinality:  triage.LocusDeliveryDepartsProfile,
		observe.SignalRuleBreach:   triage.LocusDeliveryDepartsProfile,
	}
	want := map[string]map[string]string{
		observe.SubjectWorker: before,
		observe.SubjectEdge:   before,
		observe.SubjectFeed:   feed,
	}

	subjects := []string{
		observe.SubjectFeed, observe.SubjectPipeline, observe.SubjectStage, observe.SubjectRun,
		observe.SubjectTable, observe.SubjectColumn, observe.SubjectWorker, observe.SubjectEdge,
	}
	signals := []string{
		observe.SignalVolume, observe.SignalFreshness, observe.SignalSchema,
		observe.SignalDistribution, observe.SignalRuleBreach, observe.SignalCost,
		observe.SignalDuration, observe.SignalRejectionRate, observe.SignalCardinality,
		observe.SignalReferential, observe.SignalStepRegression, observe.SignalWorkerStall,
		observe.SignalArrival,
	}
	for _, subject := range subjects {
		for _, signal := range signals {
			wantLocus, wantOK := want[subject][signal]
			got, ok := locusOf(subject, signal)
			if ok != wantOK || got != wantLocus {
				t.Errorf("locusOf(%s, %s) = (%q, %v), want (%q, %v)",
					subject, signal, got, ok, wantLocus, wantOK)
			}
		}
	}

	// The two lists above spell the anomaly vocabularies; this keeps them honest
	// against observe's own validation.
	for _, subject := range subjects {
		for _, signal := range signals {
			a := observe.Anomaly{
				AnomalyId: "a", SessionId: "s", SubjectType: subject, SubjectRef: "r",
				SignalType: signal, ObservedValue: "1", ExpectedBasis: "b", DetectorRef: "d@1",
			}
			if err := a.Validate(); err != nil {
				t.Fatalf("the test spells a vocabulary member observe refuses: %v", err)
			}
		}
	}
	if n := len(signalLocus); n != 2*len(before)+len(feed) {
		t.Errorf("signalLocus holds %d pairs, want %d: a pair outside the three subjects has been added",
			n, 2*len(before)+len(feed))
	}
}

// A feed anomaly reaches the ranking at a delivery locus and never at a run
// locus, end to end through Rank: the failure the subject key exists to prevent
// is a feed's volume filed as evidence that a run lost rows.
func TestAFeedAnomalyIsNotRunEvidence(t *testing.T) {
	in := &Input{
		Report: report("s-f", map[string]triage.Verdict{
			triage.LocusRowsLostSilently: triage.Present,
		}, nil),
		Anomalies: []observe.Anomaly{{
			AnomalyId: "an-feed", SessionId: "s-f", SubjectType: observe.SubjectFeed,
			SubjectRef: "a_feed", SignalType: observe.SignalVolume,
			ObservedValue: "12", ExpectedBasis: "declared size range",
			DetectorRef: "delivery_size@1",
		}},
	}
	got := Default().Rank(in)
	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses for a present run locus")
	}
	for i := range got.Hypotheses {
		for _, e := range got.Hypotheses[i].SupportingEvidence {
			if strings.Contains(e.SourceRef, "an-feed") {
				t.Errorf("hypothesis %s at %s cites the feed anomaly as supporting evidence",
					got.Hypotheses[i].CauseCategory, got.Hypotheses[i].Locus)
			}
		}
	}
}
