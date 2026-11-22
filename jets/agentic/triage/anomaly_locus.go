package triage

import "github.com/artisoft-io/jetstore/jets/agentic/observe"

// Which locus a detector's anomaly lands at, keyed on its subject as well as its
// signal.
//
// **There is one table, and it is here rather than in rca.** It was rca's for
// as long as only the ranker placed anomalies: it cites an anomaly at the locus
// triage found present. The delivery loci are the first whose verdict is itself
// read from an anomaly, so triage has to place a feed anomaly to decide whether
// the locus holds, and rca has to place the same anomaly to cite it. Two copies
// of the table would be two answers to one question; rca calls LocusOf.
//
// observe.Anomaly's signal types are keyed to the detector table's rows
// (Anomaly.anomaly_signal_type's own comment: "rejection_rate is row 1, volume
// is rows 2 and 5, step_regression row 3, worker_stall row 4, arrival row 10"),
// and the loci cite the same rows: locus rows_lost_silently is row 2,
// worker_not_terminated is row 4, written_not_arrived is row 10, and
// per_record_failures_reported is where a rejection rate is counted.
//
// **Keyed on the subject because a signal alone does not say where the evidence
// sits.** A feed detector emits volume when a delivery is the wrong size and
// arrival when no delivery came — the same words as a run that lost rows and a
// sink write that never landed, about a different thing. Keyed on the signal
// alone, a feed's volume would be filed as evidence that a run lost rows, with
// nothing in the ranking to say otherwise. So the run loci are reached from the
// two subjects an execution-record detector writes, worker and edge; a feed
// reaches the two delivery loci; and any other subject reaches nothing, and the
// ranking's basis names it, rather than being placed by a guess at what its
// signal meant on a subject nobody has mapped.
//
// **On a feed, arrival is delivery_overdue and every departure from the profile
// is delivery_departs_profile.** Which departure — stale, restated, incomplete,
// too large, back-dated — is the anomaly's signal and detector, and travels
// with it; the locus says only that a delivery departs from what was declared
// for it.
//
// **step_regression maps to no locus and that is not an omission.** The nine run
// loci are positions in one run's record; a step that regressed is a comparison
// across runs, and the gate's table files it under transformation_defect's
// evidence rather than under a locus.
type anomalyKey struct {
	Subject string
	Signal  string
}

// runSignalLocus is the map as it stood before it was keyed on the subject. It
// is applied to worker and edge alike.
var runSignalLocus = map[string]string{
	observe.SignalVolume:        LocusRowsLostSilently,
	observe.SignalWorkerStall:   LocusWorkerNotTerminated,
	observe.SignalArrival:       LocusWrittenNotArrived,
	observe.SignalRejectionRate: LocusPerRecordFailuresReported,
}

// feedSignalLocus is the map for subject feed: the six signals the feed
// detectors emit.
var feedSignalLocus = map[string]string{
	observe.SignalArrival:      LocusDeliveryOverdue,
	observe.SignalVolume:       LocusDeliveryDepartsProfile,
	observe.SignalFreshness:    LocusDeliveryDepartsProfile,
	observe.SignalDistribution: LocusDeliveryDepartsProfile,
	observe.SignalCardinality:  LocusDeliveryDepartsProfile,
	observe.SignalRuleBreach:   LocusDeliveryDepartsProfile,
}

var signalLocus = func() map[anomalyKey]string {
	m := map[anomalyKey]string{}
	for _, subject := range []string{observe.SubjectWorker, observe.SubjectEdge} {
		for signal, locus := range runSignalLocus {
			m[anomalyKey{subject, signal}] = locus
		}
	}
	for signal, locus := range feedSignalLocus {
		m[anomalyKey{observe.SubjectFeed, signal}] = locus
	}
	return m
}()

// LocusOf returns the locus an anomaly with this subject and signal lands at,
// and false where the table has none.
func LocusOf(subject, signal string) (string, bool) {
	l, ok := signalLocus[anomalyKey{subject, signal}]
	return l, ok
}
