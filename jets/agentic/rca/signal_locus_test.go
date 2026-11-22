package rca

import (
	"strings"
	"testing"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
	"github.com/artisoft-io/jetstore/jets/agentic/triage"
)

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
