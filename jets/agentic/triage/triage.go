// Package triage is the deterministic half of Phase 4's agent track (task
// AC.1): it classifies one run into the nine loci of the Phase 4 plan's
// section 9.4, from the execution record and nothing else.
//
// # A locus is not a cause, and this package emits only loci
//
// AA.1 read the record for what supports a *cause* and found that it supports
// a taxonomy of *locus* instead: where in the record a failure left its
// evidence, rather than what produced it. The nine values here are the
// jetsapi.incident.incident_locus vocabulary, and the ten-value
// IncidentClassification that sits beside them in the same table is a claim
// this package never makes. Reading a locus as a diagnosis is R-27, and the
// structural defence is that nothing in this package can write one: a Finding
// has no classification field.
//
// # Why this is deterministic, which is Q-22 and is not inherited from N.3
//
// N.3 decided the detectors are SQL rather than rules or a model, on the
// argument that every derivable failure mode is a GROUP BY or a predicate that
// Postgres already computes. Q-22 asked whether that argument transfers to
// triage and warned that it might not, because a taxonomy with a *benign*
// class has a boundary that is judgement.
//
// It transfers, and by a stronger route than N.3's rather than the same one.
// Benign is a member of the *cause* taxonomy and not of the locus taxonomy, so
// the contested boundary is not in this package at all. What is left is nine
// predicates over the structural position of rows — which table holds a row,
// which status it holds, whether a row exists — and eight of the nine carry no
// numeric threshold whatever. The ninth does: locus rows_lost_silently is
// observe.VolumeCollapse, whose MinRatio and MinInput are unsourced (R-21),
// and it is run here rather than reimplemented so that there is one threshold
// to source rather than two. So the honest form of the answer is that triage
// is *more* clearly rule work than detection was, and the one place it is not
// is the one place it is a detector.
//
// # Three-valued, because "not checked" and "checked and absent" are different
//
// Every predicate returns Present, Absent or NotEvaluable, and the third is
// the reason this package is shaped the way it is. Two of the nine loci cannot
// be evaluated on an ordinary deployment: sink_failed_under_completed_worker
// reads jetsapi.pipeline_execution_channel_details, which no production
// environment measured on 2026-08-25 had (I-132), and written_not_arrived is a
// check against S3 or a database rather than a query, so it is NotEvaluable
// everywhere and always here. A boolean predicate would report both as *no
// failure of that kind*, which is a claim the record does not support. This is
// observe.ConfigConfounders' Read/Found/Note distinction applied to the verdict
// itself rather than to a qualifier on it.
//
// # Classify returns all nine verdicts, not the ones that fired
//
// Criterion 44 asks for triage reported per class with denominators and no
// aggregate. A classifier that returns only what fired supplies numerators and
// no denominators — the count of sessions in which a locus was *evaluated and
// did not fire* is not recoverable from a list of hits. So Classify emits nine
// Findings per session unconditionally, and the denominator for a locus is the
// count of its Present and Absent verdicts, with the NotEvaluable ones excluded
// and reported separately rather than folded into either.
//
// # What this package does not do
//
// It does not write jetsapi.incident. AC.3 is the shadow-mode wiring and this
// is deliberately upstream of it: an Incident carries a severity that is
// NOT NULL and that no locus determines (I-306), and its classification
// transitions want an actor that AB.2 is adding.
package triage

import (
	"fmt"
	"slices"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// The nine loci of the Phase 4 plan's section 9.4. These are the members of
// jetsapi.incident's incident_locus_ck; TestLociMatchDDL asserts them against
// the generated DDL so a regeneration that adds or removes one fails the suite
// rather than an insert, which is the guard observe.TestVocabulariesMatchDDL
// gives the anomaly vocabularies.
//
// The order is section 9.4's: how early the failure occurs, which is also
// roughly the order of how little the record retains about it.
const (
	// LocusRunNotStarted — the run header is terminal and there is no worker
	// row at all (P3 F76).
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
}

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
// signal for (I-306); classification is the cause taxonomy and is AC.2's.
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
	// Four of the nine supply no step at all, which is why they are optional —
	// section 9.6's "localisation is a property, not an identity".
	StepRef  string
	ShardRef *int64

	// Subjects are the record rows the verdict rests on: worker row keys, edge
	// keys, or a step label, as strings. Empty for an Absent verdict.
	Subjects []string

	// ClassifierRef names the classifier and its generation, so two generations
	// are told apart the way two detector generations are.
	ClassifierRef string
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
		return fmt.Errorf("session_id is required: an incident cannot be keyed below the session (F202)")
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
	// message names the field. F52's reason does not depend on which step is
	// named: cpipes_step_id is a stage location rather than a step identity, so
	// any finding naming one inherits the ambiguity unconditionally.
	if f.StepRef != "" && !slices.Contains(f.Confounders, observe.ConfounderStepLabelAmbiguous) {
		return fmt.Errorf("step_ref is %q and step_label_ambiguous is not among the confounders: cpipes_step_id is a stage location rather than a step identity (F52), so any finding naming a step inherits the ambiguity",
			f.StepRef)
	}
	return nil
}

// Report is one run's nine verdicts.
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
// one incident per (session, locus) iterates. Q-30 asked what splits a session
// and left it to this task; the answer this package takes is that it does not
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
