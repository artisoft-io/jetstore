// Package triage is the deterministic half of the incident agent: it
// classifies one run into nine loci, from the execution record and
// nothing else.
//
// # Two more loci, and they are not positions in a run
//
// The vocabulary has eleven members. The last two, delivery_overdue and
// delivery_departs_profile, are positions in a feed's deliveries: a feed
// detector compares a delivery with the feed's declared profile and writes an
// anomaly with subject feed, and the locus is a predicate over that anomaly row
// rather than over the execution record. LocusOf is the one table that places
// an anomaly at a locus, and it is this package's because a delivery locus's
// verdict is read through it; rca calls it to cite the same anomaly. The
// session such a finding hangs on is a registration session, which may have no
// run, so a session can be classified on its feed anomalies alone. Classify
// reports both on every session, so that a report still has one verdict per
// member, and reports them NotEvaluable on a session that is not a delivery of
// a profiled feed and carries no feed anomaly, which is every run's own
// session: a locus that was never asked is not one that did not fire.
//
// # A locus is not a cause, and this package emits only loci
//
// A review of the record for what supports a *cause* and found that it supports
// a taxonomy of *locus* instead: where in the record a failure left its
// evidence, rather than what produced it. The nine values here are the
// jetsapi.incident.incident_locus vocabulary, and the ten-value
// IncidentClassification that sits beside them in the same table is a claim
// this package never makes. Reading a locus as a diagnosis is the known risk, and the
// structural defence is that nothing in this package can write one: a Finding
// has no classification field.
//
// # Why this is deterministic, which is not inherited from the detectors
//
// The detectors were decided as SQL rather than rules or a model, on the
// argument that every derivable failure mode is a GROUP BY or a predicate that
// Postgres already computes. The question was whether that argument transfers to
// triage and warned that it might not, because a taxonomy with a *benign*
// class has a boundary that is judgement.
//
// It transfers, and by a stronger route than the detectors' rather than the same one.
// Benign is a member of the *cause* taxonomy and not of the locus taxonomy, so
// the contested boundary is not in this package at all. What is left is nine
// predicates over the structural position of rows — which table holds a row,
// which status it holds, whether a row exists — and eight of the nine carry no
// numeric threshold whatever. The ninth does: locus rows_lost_silently is
// observe.VolumeCollapse, which judges each step against its own history and
// falls back to a floor, all of whose numbers are unsourced, and it is run here
// rather than reimplemented so that there is one set of thresholds to source
// rather than two. So the honest form of the answer is that triage
// is *more* clearly rule work than detection was, and the one place it is not
// is the one place it is a detector.
//
// # Three-valued, because "not checked" and "checked and absent" are different
//
// Every predicate returns Present, Absent or NotEvaluable, and the third is
// the reason this package is shaped the way it is. Two of the nine loci cannot
// be evaluated on an ordinary deployment: sink_failed_under_completed_worker
// reads jetsapi.pipeline_execution_channel_details, which no production
// environment measured on 2026-08-25 had, and written_not_arrived is a
// check against S3 or a database rather than a query, so it is NotEvaluable
// everywhere and always here. A boolean predicate would report both as *no
// failure of that kind*, which is a claim the record does not support. This is
// observe.ConfigConfounders' Read/Found/Note distinction applied to the verdict
// itself rather than to a qualifier on it.
//
// # Classify returns every verdict, not the ones that fired
//
// Criterion 44 asks for triage reported per class with denominators and no
// aggregate. A classifier that returns only what fired supplies numerators and
// no denominators — the count of sessions in which a locus was *evaluated and
// did not fire* is not recoverable from a list of hits. So Classify emits one
// Finding per locus per session unconditionally, eleven of them, and the denominator for a locus is the
// count of its Present and Absent verdicts, with the NotEvaluable ones excluded
// and reported separately rather than folded into either.
//
// # What this package does not do
//
// It does not write jetsapi.incident. The shadow package is the wiring and this
// is deliberately upstream of it: an Incident carries a severity that is
// NOT NULL and that no locus determines, and its classification
// transitions want an actor, which the incident transition record carries.
package triage

import (
	"fmt"
	"slices"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// The eleven loci: nine positions in a run's execution record, then two in a
// feed's deliveries. These are the members of
// jetsapi.incident's incident_locus_ck; TestLociMatchDDL asserts them against
// the generated DDL so a regeneration that adds or removes one fails the suite
// rather than an insert, which is the guard observe.TestVocabulariesMatchDDL
// gives the anomaly vocabularies.
//
// The order is how early the failure occurs, which is also
// roughly the order of how little the record retains about it — except for
// the last two, which are appended because the vocabulary is never
// renumbered. A delivery precedes the run that loads it.
const (
	// LocusRunNotStarted — the run header is terminal and there is no worker
	// row at all.
	LocusRunNotStarted = "run_not_started"
	// LocusStepNeverStarted — a step other runs of this source have is absent
	// from this one.
	LocusStepNeverStarted = "step_never_started"
	// LocusWorkerNotTerminated — a worker sits at 'in progress' under a header
	// that reached a terminal state.
	LocusWorkerNotTerminated = "worker_not_terminated"
	// LocusWorkerFailed — a worker reported 'failed'.
	LocusWorkerFailed = "worker_failed"
	// LocusSinkFailedUnderCompletedWorker — a DAG edge carries an error under a
	// worker that completed.
	LocusSinkFailedUnderCompletedWorker = "sink_failed_under_completed_worker"
	// LocusRowsLostSilently — counts collapsed with every status terminal and
	// clean.
	LocusRowsLostSilently = "rows_lost_silently"
	// LocusPerRecordFailuresReported — process_errors rows exist for the run.
	LocusPerRecordFailuresReported = "per_record_failures_reported"
	// LocusPerRecordFailuresUnreportable — an operator that can report
	// per-record errors is configured without an error channel, so its failures
	// have nowhere to go.
	LocusPerRecordFailuresUnreportable = "per_record_failures_unreportable"
	// LocusWrittenNotArrived — an edge names a destination the data is not at.
	LocusWrittenNotArrived = "written_not_arrived"
	// LocusDeliveryOverdue — no delivery of a feed arrived within its declared
	// cadence of the last one. Read from a feed anomaly with signal arrival. A
	// feed that stops has no session of its own, so the incident is keyed on
	// the last delivery's session, with the feed as the anomaly's subject;
	// deliveryLocus is where that is read.
	LocusDeliveryOverdue = "delivery_overdue"
	// LocusDeliveryDepartsProfile — a delivery that arrived departs from the
	// feed's declared profile. Read from a feed anomaly with any signal other
	// than arrival; the signal says which departure.
	LocusDeliveryDepartsProfile = "delivery_departs_profile"
)

// Loci is the vocabulary in section 9.4's order. A caller reporting per class
// iterates this rather than the findings it happens to hold, so a locus that
// never fires still has a row.
var Loci = []string{
	LocusRunNotStarted,
	LocusStepNeverStarted,
	LocusWorkerNotTerminated,
	LocusWorkerFailed,
	LocusSinkFailedUnderCompletedWorker,
	LocusRowsLostSilently,
	LocusPerRecordFailuresReported,
	LocusPerRecordFailuresUnreportable,
	LocusWrittenNotArrived,
	LocusDeliveryOverdue,
	LocusDeliveryDepartsProfile,
}

// DeliveryLoci are the two loci that are positions in a feed's deliveries
// rather than in a run's execution record. A caller that counts a locus over
// runs — the step census, the shard localisation — has nothing to count for
// these.
var DeliveryLoci = []string{LocusDeliveryOverdue, LocusDeliveryDepartsProfile}

// Verdict is one locus's answer for one run. It is three-valued rather than
// boolean, and NotEvaluable is the member the package exists to keep separate:
// it says the predicate could not be asked, not that it was asked and said no.
type Verdict string

const (
	// Present — the predicate holds. Subjects names the rows it holds over.
	Present Verdict = "present"
	// Absent — the predicate was evaluated and does not hold.
	Absent Verdict = "absent"
	// NotEvaluable — the predicate could not be evaluated. Basis says why, and
	// a caller must not count this as Absent.
	NotEvaluable Verdict = "not_evaluable"
)

var verdicts = []Verdict{Present, Absent, NotEvaluable}

// Finding is one locus's verdict for one run, with what it rests on and what
// it could not rule out.
//
// It deliberately has no severity and no classification. Severity is an
// Incident property that no locus determines and that the record carries no
// signal for; classification is the cause taxonomy and is the rca package's.
type Finding struct {
	SessionId string
	Locus     string
	Verdict   Verdict

	// Basis says what was compared against what, in the words an operator
	// reads. It is required for every verdict including Absent, on the
	// reasoning Anomaly.ExpectedBasis is required for: a verdict that cannot
	// say what it looked at is the signal an operator learns to ignore.
	Basis string

	// Confounders is what this verdict could not rule out, from the same
	// fourteen-member vocabulary the detectors write on an Anomaly and the same
	// one jetsapi.incident's incident_confounders_ck admits. It is deliberately
	// not a second vocabulary: an incident's qualifiers have to compare with
	// the anomaly's that gave rise to it.
	Confounders []string

	// StepRef and ShardRef localise the finding where the locus supplies them.
	// Four of the nine run loci supply no step at all and neither delivery
	// locus does, which is why they are optional —
	// section 9.6's "localisation is a property, not an identity".
	StepRef  string
	ShardRef *int64

	// Subjects are the record rows the verdict rests on: worker row keys, edge
	// keys, a step label, or, for a delivery locus, the anomaly ids, as
	// strings. Empty for an Absent verdict.
	Subjects []string

	// ClassifierRef names the classifier and its generation, so two generations
	// are told apart the way two detector generations are.
	ClassifierRef string

	// Codes are, at locus per_record_failures_reported only, the run's
	// process_errors rows grouped by error code, each with its stored rows, its
	// threshold and whether it reports (CodeCount). The finding stays one per
	// locus: it is present when any code reports, and a writer that keys
	// incidents by code splits it by Reporting. Nil at every other locus.
	Codes []CodeCount
}

// Reporting returns the codes that report, in Codes's order.
func (f *Finding) Reporting() []CodeCount {
	var out []CodeCount
	for _, c := range f.Codes {
		if c.Reports() {
			out = append(out, c)
		}
	}
	return out
}

// Fired reports whether this finding is a hit.
func (f *Finding) Fired() bool { return f.Verdict == Present }

// Validate checks the vocabularies, the required fields and the one
// cross-field invariant, in Go, before Postgres would. The database is the
// authority; this exists so the message names the field rather than the
// constraint — observe.Anomaly.Validate's stated reason, and the same
// arrangement.
func (f *Finding) Validate() error {
	if f.SessionId == "" {
		return fmt.Errorf("session_id is required: an incident cannot be keyed below the session")
	}
	if !slices.Contains(Loci, f.Locus) {
		return fmt.Errorf("locus %q is not in the vocabulary", f.Locus)
	}
	if !slices.Contains(verdicts, f.Verdict) {
		return fmt.Errorf("verdict %q is not one of present, absent, not_evaluable", f.Verdict)
	}
	if f.Basis == "" {
		return fmt.Errorf("basis is required, including for an absent verdict: a verdict that cannot say what it looked at is the signal an operator learns to ignore")
	}
	if f.ClassifierRef == "" {
		return fmt.Errorf("classifier_ref is required")
	}
	for _, c := range f.Confounders {
		if !observe.IsConfounder(c) {
			return fmt.Errorf("confounders holds %q, which is not in the anomaly confounder vocabulary", c)
		}
	}
	// jetsapi.incident's incident_step_confounder_ck, enforced here so the
	// message names the field. The reason does not depend on which step is
	// named: cpipes_step_id is a stage location rather than a step identity, so
	// any finding naming one inherits the ambiguity unconditionally.
	if f.StepRef != "" && !slices.Contains(f.Confounders, observe.ConfounderStepLabelAmbiguous) {
		return fmt.Errorf("step_ref is %q and step_label_ambiguous is not among the confounders: cpipes_step_id is a stage location rather than a step identity, so any finding naming a step inherits the ambiguity",
			f.StepRef)
	}
	return nil
}

// Report is one run's verdicts, one per locus.
type Report struct {
	SessionId string
	Findings  []Finding
	// Evaluable counts the findings that are not NotEvaluable. It is the
	// denominator criterion 44 asks for, at the grain of one run.
	Evaluable int
}

// Fired returns the loci that hold, in section 9.4's order.
func (r *Report) Fired() []Finding {
	var out []Finding
	for i := range r.Findings {
		if r.Findings[i].Fired() {
			out = append(out, r.Findings[i])
		}
	}
	return out
}

// Loci returns the names of the loci that hold, which is what a caller writing
// one incident per (session, locus) iterates. What splits a session
// was left open; the answer this package takes is that it does not
// choose: a session that exhibits three loci yields three findings and the
// caller writes three incidents, because incident_locus is single-valued and
// collapsing three verdicts into one would be a judgement the record does not
// make.
func (r *Report) Loci() []string {
	var out []string
	for i := range r.Findings {
		if r.Findings[i].Fired() {
			out = append(out, r.Findings[i].Locus)
		}
	}
	return out
}
