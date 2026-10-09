package triage

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/artisoft-io/jetstore/jets/agentic/observe"
)

// The nine predicates. Each is a method on Classifier returning exactly one
// Finding, and Classify calls all nine in section 9.4's order.
//
// # What each can and cannot distinguish is in the code rather than beside it
//
// Every NotEvaluable verdict below names the thing that could not be read, and
// every Present verdict's basis names what the record cannot say about it. That
// is deliberate duplication of the plan's section 9.4 *cannot see* column: the
// column is the argument and these strings are what an operator actually reads,
// and a cannot-see that lives only in a plan is one nobody sees at the moment
// it matters.

// Classifier is the triage step. It holds one detector and one number, and the
// asymmetry is the finding rather than an untidiness: eight of the nine loci
// are structural predicates with no threshold at all, and the ninth is
// observe.VolumeCollapse.
type Classifier struct {
	// Volume is locus rows_lost_silently. It is N.4's detector run rather than
	// reimplemented, so its two unsourced thresholds (R-21) are one number to
	// source rather than two, and a change to the detector moves the locus with
	// it.
	Volume observe.VolumeCollapse

	// MinPriorRuns is how many prior runs of a step the baseline must hold
	// before its absence from this run is called a locus. It is
	// StepRegression's threshold applied to a different predicate and for the
	// same reason: a step seen once is not a step this run was expected to
	// have. Unsourced, like the two above.
	MinPriorRuns int64

	// Generation is the trailing number of the classifier_ref, so two
	// generations of the same classifier are told apart the way two detector
	// generations are.
	Generation int
}

// Default is the starting point, not a recommendation. Its three numbers are
// unsourced in exactly the sense the observe package's file comment states:
// nothing in this repository has measured what a normal step census or a normal
// output ratio looks like for a JetStore run.
func Default() Classifier {
	return Classifier{
		Volume:       observe.DefaultVolumeCollapse(),
		MinPriorRuns: 3,
		Generation:   1,
	}
}

// Ref is the classifier_ref written on every finding.
func (c Classifier) Ref() string { return fmt.Sprintf("triage@%d", c.Generation) }

// Classify returns one Finding per locus, in section 9.4's order, always nine
// of them. See the package comment for why it does not return only the hits.
func (c Classifier) Classify(ev *Evidence) *Report {
	r := &Report{SessionId: ev.SessionId}
	r.Findings = []Finding{
		c.runNotStarted(ev),
		c.stepNeverStarted(ev),
		c.workerNotTerminated(ev),
		c.workerFailed(ev),
		c.sinkFailedUnderCompletedWorker(ev),
		c.rowsLostSilently(ev),
		c.perRecordFailuresReported(ev),
		c.perRecordFailuresUnreportable(ev),
		c.writtenNotArrived(ev),
	}
	for i := range r.Findings {
		r.Findings[i].SessionId = ev.SessionId
		r.Findings[i].ClassifierRef = c.Ref()
		if r.Findings[i].Verdict != NotEvaluable {
			r.Evaluable++
		}
	}
	return r
}

// notEvaluable is the constructor for the verdict this package exists to keep
// separate from Absent.
func notEvaluable(locus, why string, confounders ...string) Finding {
	return Finding{Locus: locus, Verdict: NotEvaluable, Basis: why, Confounders: confounders}
}

// recordUnavailable is the answer to all nine when the execution record itself
// is not deployed. It is nine NotEvaluable verdicts rather than nine absent
// ones, which is F107's lesson at the level of a classifier: a report about the
// world that cannot fail on a missing relation is the only kind worth asking.
func (c Classifier) recordUnavailable(locus string) Finding {
	return notEvaluable(locus,
		"jetsapi.pipeline_execution_status or jetsapi.pipeline_execution_details is not deployed on this "+
			"database, so no predicate over the execution record can be evaluated at all; "+
			"`update_db -migrateDb` is what installs them")
}

// --- Locus 1 -----------------------------------------------------------------

// runNotStarted is section 9.4 row 1: a terminal header with no worker row.
//
// Its stated *cannot see* is that a run which failed at sharding validation
// leaves no cpipes_execution_status row either, so the two most informative
// artefacts are absent at once. Implementing it found a second, which section
// 9.4 does not carry: **the worker record is purged on a clock the header is
// not** (F54), so in a header-outlives-detail deployment an old run whose
// worker rows have been purged is indistinguishable from a run that never
// started. The verdict is NotEvaluable in that case rather than Present, and
// observe.Extent.OldestWorker is what decides it.
func (c Classifier) runNotStarted(ev *Evidence) Finding {
	const locus = LocusRunNotStarted
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if ev.Header == nil {
		return notEvaluable(locus,
			fmt.Sprintf("no pipeline_execution_status row for session %q. The header is this locus's only "+
				"evidence, and its absence is two things the record does not separate: a run that was "+
				"never submitted, and a run whose header the six-month purge has taken (F54)", ev.SessionId))
	}
	rows := ev.workerRows()
	if !ev.Header.Terminal() {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"the run header of session %q is in status %q, which is not terminal, so the run has not yet "+
				"failed to start; it has %d worker rows",
			ev.SessionId, ev.Header.Status, len(rows))}
	}
	if len(rows) > 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"the run header of session %q is terminal at %q and the run has %d worker rows, so it started",
			ev.SessionId, ev.Header.Status, len(rows))}
	}
	// Terminal header, no worker rows. Before calling it, rule out the purge.
	if oldest := ev.Extent.OldestWorker; !oldest.IsZero() && ev.Header.StartTime.Before(oldest) {
		return notEvaluable(locus, fmt.Sprintf(
			"the run header of session %q is terminal at %q with no worker row, but the run started at %s "+
				"and the oldest worker row anywhere on this database is %s. The worker record is purged "+
				"against RETENTION_DAYS on a clock independent of the header's (F54), so a run older than "+
				"the worker record cannot be told from a run that never started. The deployment's regime "+
				"is %q",
			ev.SessionId, ev.Header.Status, ev.Header.StartTime.Format(timeFmt),
			oldest.Format(timeFmt), ev.Extent.Regime()),
			observe.ConfounderHistoryTruncated)
	}
	basis := fmt.Sprintf(
		"the run header of session %q is terminal at %q and there is no pipeline_execution_details row for "+
			"it at all, so no worker ever started (P3 F76). failure_details is the only evidence of why, "+
			"and it is one of four undeclared shapes: the state machine's six live catches all route to "+
			"one error-status task (F197) and the machine-readable Error field is discarded before the "+
			"database (F198)",
		ev.SessionId, ev.Header.Status)
	var conf []string
	if ev.Survey != nil && !ev.Survey.Read {
		basis += ". " + ev.Survey.Note + ", which is the second artefact this locus loses at once"
	} else if ev.Extent != nil && !ev.Extent.CpipesStatus {
		basis += ". jetsapi.cpipes_execution_status is not deployed, so the run's configuration could not " +
			"be consulted either"
	}
	if ev.Header.FailureDetails != "" {
		basis += fmt.Sprintf(". failure_details holds %d characters of text", len(ev.Header.FailureDetails))
	} else {
		basis += ". failure_details is empty, which the ErrorUpdate maps write literally when nothing " +
			"was caught (F197)"
	}
	return Finding{Locus: locus, Verdict: Present, Basis: basis, Confounders: conf,
		Subjects: []string{"header " + strconv.FormatInt(ev.Header.Key, 10)}}
}

// --- Locus 2 -----------------------------------------------------------------

// stepNeverStarted is section 9.4 row 2: a step other runs of this source have
// is absent from this one.
//
// Row 2's *cannot see* is that this is indistinguishable from three other
// things — a conditional step skipped by its `when`, a step not in this version
// of the config, and a step whose label collides with another's — and that F196
// makes the second uncheckable because no run names a workspace version.
//
// **The confounder vocabulary has no member for that** (I-303). Its fourteen
// members were derived for detection, step_label_ambiguous covers the third of
// the three and nothing covers the second, so the basis carries in prose what
// would otherwise be a qualifier. A member is proposed rather than added: the
// vocabulary is a generated CHECK on two tables and widening it on this task's
// authority is the unreviewed extraction gap 2b exists to prevent.
func (c Classifier) stepNeverStarted(ev *Evidence) Finding {
	const locus = LocusStepNeverStarted
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if ev.Prior == nil {
		return notEvaluable(locus,
			"no step history was gathered, so there is no set of steps this run could be missing one of. "+
				"Pass a non-zero baseline window to Gather")
	}
	if ev.Header == nil {
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has no run header, so it has no main_object_type and cannot be matched to a step "+
				"baseline: the source identity is (client, process_name, main_object_type, cpipes_step_id) "+
				"and three of the four are on the worker row while the fourth is only on the header (F98)",
			ev.SessionId), observe.ConfounderHistoryTruncated)
	}

	present := map[string]bool{}
	for _, r := range ev.workerRows() {
		present[r.StepId] = true
	}
	var missing []observe.StepBaseline
	var considered int
	for i := range ev.Prior.Baselines {
		b := &ev.Prior.Baselines[i]
		if b.Client != ev.Header.Client || b.ProcessName != ev.Header.ProcessName ||
			b.MainObjectType != ev.Header.MainObjectType {
			continue
		}
		if b.Runs < c.MinPriorRuns {
			continue
		}
		considered++
		if !present[b.StepId] {
			missing = append(missing, *b)
		}
	}
	if considered == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"the baseline holds no step of %s/%s/%s with at least %d prior runs, so there is no step this "+
				"run was expected to have. %d baselines were read over the window ending %s",
			ev.Header.Client, ev.Header.ProcessName, ev.Header.MainObjectType, c.MinPriorRuns,
			len(ev.Prior.Baselines), ev.Prior.Window.Until.Format(timeFmt)),
			ev.baselineConfounders()...)
	}
	if len(missing) == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"all %d steps of %s/%s/%s with at least %d prior runs have a worker row in session %q",
			considered, ev.Header.Client, ev.Header.ProcessName, ev.Header.MainObjectType,
			c.MinPriorRuns, ev.SessionId),
			Confounders: ev.baselineConfounders()}
	}

	conf := ev.baselineConfounders()
	var names, subjects []string
	anyEmpty := false
	for i := range missing {
		label := missing[i].StepId
		if label == "" {
			label = "(unlabelled)"
			anyEmpty = true
		}
		names = append(names, fmt.Sprintf("%s (%d prior runs, last seen %s)",
			label, missing[i].Runs, missing[i].LastSeen.Format(timeFmt)))
		subjects = append(subjects, "step "+label)
	}
	// A step named in a finding drags step_label_ambiguous in unconditionally,
	// which is jetsapi.incident's incident_step_confounder_ck and F52's reason:
	// cpipes_step_id is a stage location rather than a step identity.
	if !slices.Contains(conf, observe.ConfounderStepLabelAmbiguous) {
		conf = append(conf, observe.ConfounderStepLabelAmbiguous)
	}
	basis := fmt.Sprintf(
		"%d of the %d steps of %s/%s/%s with at least %d prior runs have no worker row in session %q: %s. "+
			"The record cannot tell that from three other things (P3 F52): a conditional step whose `when` "+
			"did not fire, a step that is not in the configuration this run used, and a step whose label "+
			"collides with another's. The second is not checkable at all, because no run is bound to a "+
			"workspace version (F196), and the confounder vocabulary has no member for it",
		len(missing), considered, ev.Header.Client, ev.Header.ProcessName, ev.Header.MainObjectType,
		c.MinPriorRuns, ev.SessionId, strings.Join(names, "; "))
	if anyEmpty {
		basis += ". One of the missing steps carries an empty cpipes_step_id, which ten reducing steps in " +
			"the corpus do, so that row of the baseline may be two steps"
	}
	stepRef := missing[0].StepId
	if len(missing) > 1 {
		// Naming one of several would be arbitrary; the subjects carry them all.
		stepRef = ""
	}
	f := Finding{Locus: locus, Verdict: Present, Basis: basis, Confounders: conf, Subjects: subjects}
	if stepRef != "" {
		f.StepRef = stepRef
	}
	return f
}

func (ev *Evidence) baselineConfounders() []string {
	var out []string
	if ev.Prior == nil {
		return out
	}
	if ev.Prior.Headerless > 0 || ev.Prior.Truncated() {
		out = append(out, observe.ConfounderHistoryTruncated)
	}
	if ev.Extent != nil && !ev.Extent.ChannelDetails {
		out = append(out, observe.ConfounderCrossStepJoinUnavailable)
	}
	return out
}

// --- Locus 3 -----------------------------------------------------------------

// workerNotTerminated is section 9.4 row 3: a worker at 'in progress' under a
// terminal header. The predicate is observe.WorkerRow.Stalled, which is
// section 12.2 row 4's and is called rather than restated.
//
// Row 3's *cannot see* is that nothing says why, and that the step aggregate
// **hides** the row rather than merely omitting it: cpipes_execution_status_details
// filters status != 'in progress' (F193), so the step's totals silently
// describe the workers that finished. That is carried in the basis because it
// is the thing an operator reading a green step summary most needs told.
func (c Classifier) workerNotTerminated(ev *Evidence) Finding {
	const locus = LocusWorkerNotTerminated
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	rows := ev.workerRows()
	if len(rows) == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has no worker rows, so there is none to be stalled. That is locus %s's observation, "+
				"not this one's", ev.SessionId, LocusRunNotStarted))
	}
	if ev.Header == nil {
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has %d worker rows and no run header, and the predicate is a worker that never "+
				"terminated *under a header that did*. Without the header a row at 'in progress' is a "+
				"running worker and a stalled one at once",
			ev.SessionId, len(rows)), observe.ConfounderHistoryTruncated)
	}
	var stalled []observe.WorkerRow
	for i := range rows {
		if rows[i].Stalled() {
			stalled = append(stalled, rows[i])
		}
	}
	if len(stalled) == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"all %d worker rows of session %q reached a terminal state under a header at %q",
			len(rows), ev.SessionId, ev.Header.Status)}
	}
	conf := []string{observe.ConfounderStallCauseUnknown}
	var subjects []string
	labelled := 0
	for i := range stalled {
		subjects = append(subjects, "worker "+strconv.FormatInt(stalled[i].Key, 10))
		if stalled[i].StepId != "" {
			labelled++
		}
	}
	basis := fmt.Sprintf(
		"%d of %d worker rows of session %q are still at 'in progress' under a header that reached %q. "+
			"The record says nothing about why: this locus collapses a killed task, an out-of-memory kill, "+
			"a lost node and a hung read into one observation. Their six counts are NULL rather than 0, "+
			"because only UpdatePipelineExecutionStatus sets them (F99), so no arithmetic over this run's "+
			"volumes includes them. **And none of them can say which step it is**: cpipes_step_id is set "+
			"by that same update and the insert names eleven columns without it, so a worker that never "+
			"terminated has an empty step label by construction, whatever its configuration said — %d of "+
			"the %d carry one. The step aggregate then does not merely omit these rows: "+
			"cpipes_execution_status_details is a GROUP BY filtered `status <> 'in progress'` (F193), so "+
			"the step's totals describe the workers that finished and read as a clean step",
		len(stalled), len(rows), ev.SessionId, ev.Header.Status, labelled, len(stalled))
	f := Finding{Locus: locus, Verdict: Present, Basis: basis, Subjects: subjects}
	if len(stalled) == 1 {
		shard := int64(stalled[0].ShardId)
		f.ShardRef = &shard
		if stalled[0].StepId != "" {
			f.StepRef = stalled[0].StepId
		}
	}
	// The label is ambiguous either way here, and for two different reasons: an
	// empty one is unknowable by the paragraph above, and a non-empty one is a
	// stage location rather than a step identity (F52).
	conf = append(conf, observe.ConfounderStepLabelAmbiguous)
	f.Confounders = conf
	return f
}

// --- Locus 4 -----------------------------------------------------------------

// workerFailed is section 9.4 row 4: a worker reported 'failed'.
//
// This is the simplest predicate in the table and its *cannot see* is the
// largest. error_message is strings.Join(processingErrors, ",") with eight
// heterogeneous feeders and no escaping (F191), so it cannot be split back into
// the errors that made it, and no column of the row carries a class (F199).
// **The confounder vocabulary has no member for that either** (I-303), so the
// basis says it in prose and reports the comma count as the lower bound it is.
func (c Classifier) workerFailed(ev *Evidence) Finding {
	const locus = LocusWorkerFailed
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	rows := ev.workerRows()
	if len(rows) == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has no worker rows, so no worker can have failed", ev.SessionId))
	}
	var failed []observe.WorkerRow
	for i := range rows {
		if rows[i].Status == observe.StatusFailed {
			failed = append(failed, rows[i])
		}
	}
	if len(failed) == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"none of the %d worker rows of session %q is at %q", len(rows), ev.SessionId, observe.StatusFailed)}
	}
	var conf, subjects []string
	segments := 0
	withMessage := 0
	anyEmptyStep := false
	for i := range failed {
		subjects = append(subjects, "worker "+strconv.FormatInt(failed[i].Key, 10))
		if failed[i].ErrorMessage != "" {
			withMessage++
			segments += strings.Count(failed[i].ErrorMessage, ",") + 1
		}
		if failed[i].StepId == "" {
			anyEmptyStep = true
		}
	}
	basis := fmt.Sprintf(
		"%d of %d worker rows of session %q are at 'failed'; %d carry an error_message. That column is "+
			"strings.Join(processingErrors, \",\") with eight append sites feeding it and no escaping "+
			"(F191), and the messages contain commas, so it cannot be split back into the errors that made "+
			"it: %d comma-separated segments is an upper bound on how many things went wrong and not a "+
			"count of them. No column of the row carries a class (F199), and jets/compute_pipes constructs "+
			"984 distinct errors that all arrive here as text",
		len(failed), len(rows), ev.SessionId, withMessage, segments)
	f := Finding{Locus: locus, Verdict: Present, Basis: basis, Subjects: subjects}
	if len(failed) == 1 {
		shard := int64(failed[0].ShardId)
		f.ShardRef = &shard
		if failed[0].StepId != "" {
			f.StepRef = failed[0].StepId
			conf = append(conf, observe.ConfounderStepLabelAmbiguous)
		}
	}
	if anyEmptyStep && !slices.Contains(conf, observe.ConfounderStepLabelAmbiguous) {
		conf = append(conf, observe.ConfounderStepLabelAmbiguous)
	}
	f.Confounders = conf
	return f
}

// --- Locus 5 -----------------------------------------------------------------

// sinkFailedUnderCompletedWorker is section 9.4 row 5: a DAG edge carrying an
// error under a worker that completed.
//
// **It is NotEvaluable on an ordinary deployment**, and that is the strongest
// case for this package being three-valued. jetsapi.pipeline_execution_channel_details
// is created by `update_db -migrateDb` and was absent from all four production
// environments measured on 2026-08-25 (I-132), so a boolean predicate would
// report *no sink failed* on every one of them.
//
// Row 5's own *cannot see* is that an empty child set has three producers the
// record does not separate: a step with no writer results, a child insert whose
// error the caller does not propagate, and a migration that has not run. This
// implementation separates the third from the other two — that is what Extent
// is for — and cannot separate the first two.
func (c Classifier) sinkFailedUnderCompletedWorker(ev *Evidence) Finding {
	const locus = LocusSinkFailedUnderCompletedWorker
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if !ev.Extent.ChannelDetails {
		return notEvaluable(locus,
			"jetsapi.pipeline_execution_channel_details is not deployed on this database, so no DAG edge "+
				"can be read. It arrives with `update_db -migrateDb` and was absent from all four "+
				"production environments measured on 2026-08-25 (I-132), so this is the ordinary state "+
				"rather than the exception",
			observe.ConfounderCrossStepJoinUnavailable)
	}
	if len(ev.Edges) == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"jetsapi.pipeline_execution_channel_details is deployed and holds no row for session %q. Three "+
				"things produce that and the record separates none of them: a run whose workers reported no "+
				"writer results at all, a child insert whose error InsertChannelExecutionDetails logged "+
				"while its caller carried on, and a run that predates the migration. Only the third is "+
				"ruled out here", ev.SessionId),
			observe.ConfounderCrossStepJoinUnavailable)
	}
	var hits []Edge
	for i := range ev.Edges {
		e := ev.Edges[i]
		if e.ErrorMessage == "" {
			continue
		}
		if e.ParentStatus != observe.StatusCompleted {
			continue
		}
		hits = append(hits, e)
	}
	if len(hits) == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"none of the %d channel-detail rows of session %q carries an error_message under a parent "+
				"worker at 'completed'", len(ev.Edges), ev.SessionId)}
	}
	var subjects []string
	folded := 0
	for i := range hits {
		subjects = append(subjects, "edge "+strconv.FormatInt(hits[i].Key, 10))
		if hits[i].SinksCount > 1 {
			folded++
		}
	}
	basis := fmt.Sprintf(
		"%d of %d channel-detail rows of session %q carry an error_message while their parent worker "+
			"reached 'completed', so the worker reported success and one of its sinks did not. An edge is "+
			"an aggregate over sinks: %d of the %d name no sink at all, because output_entity is blanked "+
			"when a row folds more than one (output_sinks_count > 1), so a message from one failing sink "+
			"of many names none of them",
		len(hits), len(ev.Edges), ev.SessionId, folded, len(hits))
	f := Finding{Locus: locus, Verdict: Present, Basis: basis, Subjects: subjects}
	if len(hits) == 1 && hits[0].ParentStepId != "" {
		f.StepRef = hits[0].ParentStepId
		f.Confounders = []string{observe.ConfounderStepLabelAmbiguous}
	}
	if len(hits) == 1 && hits[0].ParentShard != nil {
		shard := *hits[0].ParentShard
		f.ShardRef = &shard
	}
	return f
}

// --- Locus 6 -----------------------------------------------------------------

// rowsLostSilently is section 9.4 row 6: counts collapse with every status
// terminal and clean.
//
// **This is the one locus of the nine that is a detector rather than a
// predicate**, and the one that carries a threshold. It runs
// observe.VolumeCollapse rather than restating its rule, so the two unsourced
// numbers (R-21) stay one thing to source, and the confounders on the finding
// are the ones the detector would have written on the anomaly — including the
// four it reads from the run's configuration, which no structural predicate
// here needs.
func (c Classifier) rowsLostSilently(ev *Evidence) Finding {
	const locus = LocusRowsLostSilently
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if ev.Workers == nil || len(ev.Workers.Rows) == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"session %q has no worker rows, so there is no input and output count to compare", ev.SessionId))
	}
	completed := 0
	withCounts := 0
	for _, r := range ev.Workers.Rows {
		if r.Status != observe.StatusCompleted {
			continue
		}
		completed++
		if r.InputRecords != nil && r.OutputRecords != nil {
			withCounts++
		}
	}
	if completed == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"none of the %d worker rows of session %q reached 'completed'. The detector fires only on a "+
				"completed worker: a failed one has already reported itself through its status, and one "+
				"still in progress has NULL counts (F99)", len(ev.Workers.Rows), ev.SessionId))
	}
	if withCounts == 0 {
		return notEvaluable(locus, fmt.Sprintf(
			"%d worker rows of session %q reached 'completed' and none carries both an "+
				"input_records_count and an output_records_count, so no ratio exists to compare",
			completed, ev.SessionId))
	}

	anomalies := c.Volume.Detect(&observe.Evidence{
		Extent:  ev.Extent.Extent,
		Workers: ev.Workers,
		Configs: map[string]*observe.ConfigConfounders{ev.SessionId: ev.Config},
	})
	if len(anomalies) == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"%d of %d worker rows of session %q completed with both counts set, and none emitted fewer "+
				"than %.0f%% of what it read over an input of at least %d rows (%s)",
			withCounts, len(ev.Workers.Rows), ev.SessionId, 100*c.Volume.MinRatio, c.Volume.MinInput,
			c.Volume.Ref())}
	}
	var subjects, conf []string
	for i := range anomalies {
		subjects = append(subjects, "worker "+anomalies[i].SubjectRef)
		for _, x := range anomalies[i].Confounders {
			if !slices.Contains(conf, x) {
				conf = append(conf, x)
			}
		}
	}
	basis := fmt.Sprintf(
		"%s found %d of the %d completed worker rows of session %q emitting materially fewer rows than "+
			"they read. This is the one locus of the nine that carries a threshold, and both of its numbers "+
			"are unsourced (R-21): nothing in this repository has measured what a normal output ratio is "+
			"for a JetStore step. The first anomaly's own basis: %s",
		c.Volume.Ref(), len(anomalies), completed, ev.SessionId, anomalies[0].ExpectedBasis)
	f := Finding{Locus: locus, Verdict: Present, Basis: basis, Confounders: conf, Subjects: subjects}
	if len(anomalies) == 1 {
		for _, r := range ev.Workers.Rows {
			if strconv.FormatInt(r.Key, 10) != anomalies[0].SubjectRef {
				continue
			}
			shard := int64(r.ShardId)
			f.ShardRef = &shard
			if r.StepId != "" {
				f.StepRef = r.StepId
				if !slices.Contains(f.Confounders, observe.ConfounderStepLabelAmbiguous) {
					f.Confounders = append(f.Confounders, observe.ConfounderStepLabelAmbiguous)
				}
			}
		}
	}
	return f
}

// --- Locus 7 -----------------------------------------------------------------

// perRecordFailuresReported is section 9.4 row 7: process_errors rows exist for
// the run.
//
// Row 7's *cannot see* is dense and none of it is expressible as a confounder.
// The table carries no cpipes_step_id and no pipeline_execution_details_key
// (F190), so an error row joins to one row per step of the run; six sites
// construct one and no column says which (F186); a single configuration can
// route two operators into it (F189); the count is censored at a cap that
// differs by operator, 20 for jetrules and map_record and 50 for the inference
// operators (F187); and grouping_key has no writer at all (F185).
//
// The count is therefore reported as the lower bound it is, with the two
// columns a single operator family writes counted separately so a reader can
// see how much of the run's per-record evidence can say which column failed.
func (c Classifier) perRecordFailuresReported(ev *Evidence) Finding {
	const locus = LocusPerRecordFailuresReported
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if !ev.Extent.ProcessErrors || ev.Errors == nil {
		return notEvaluable(locus,
			"jetsapi.process_errors is not deployed on this database, so the absence of a per-record "+
				"failure cannot be told from the absence of the table")
	}
	if ev.Errors.Rows == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"jetsapi.process_errors holds no row for session %q. That is a weaker statement than it looks: "+
				"the table is optional, and locus %s is the case where a failing operator had no error "+
				"channel to report through at all",
			ev.SessionId, LocusPerRecordFailuresUnreportable)}
	}
	basis := fmt.Sprintf(
		"jetsapi.process_errors holds %d rows for session %q over %d shards, between %s and %s. "+
			"**Read the count as a lower bound**: reporting is capped per operator instance at 20 for "+
			"jetrules and map_record and 50 for the inference operators (F187), and past the cap the "+
			"operator logs once and keeps counting silently, so the censoring point varies with the "+
			"cluster size. The rows cannot be attributed to a step or an operator: the table carries no "+
			"cpipes_step_id and no pipeline_execution_details_key (F190), six sites construct a row and no "+
			"column says which (F186), and one configuration in the corpus routes two operators' error "+
			"channels into this same table (F189). %d rows name an input_column and %d carry a saved rete "+
			"session, which is as close as the table comes to saying which operator family wrote them; "+
			"grouping_key has no writer anywhere in the tree (F185)",
		ev.Errors.Rows, ev.SessionId, ev.Errors.Shards,
		ev.Errors.First.Format(timeFmt), ev.Errors.Last.Format(timeFmt),
		ev.Errors.WithInputColumn, ev.Errors.WithReteSession)
	return Finding{Locus: locus, Verdict: Present, Basis: basis,
		Subjects: []string{fmt.Sprintf("%d process_errors rows", ev.Errors.Rows)}}
}

// --- Locus 8 -----------------------------------------------------------------

// perRecordFailuresUnreportable is section 9.4 row 8: an operator that could
// report per-record errors is configured without an error channel, so its
// failures have nowhere to go.
//
// **Section 9.5 says eight of the nine loci are computable in SQL of the shape
// N.3 decided, and this row is the reason that is narrower than it reads.** Row
// 8's predicate is not over the execution record at all: it is over the run's
// stored *configuration*, a JSON document in a jetsapi column, and it is
// answered by walking that document in Go rather than by a GROUP BY. The count
// that survives is **seven** predicates in N.3's shape, this one in Go over a
// jetsapi column, and one — locus written_not_arrived — outside the database
// entirely.
//
// Row 8's *cannot see* column reads "Everything", and the reason the verdict
// here is ever anything but NotEvaluable is that the configuration is a second
// substrate the record does not have. What it inherits instead is the
// configuration's own three limits: at most one step's config, no row at all
// for a run that failed at sharding validation, and no environment (F195).
func (c Classifier) perRecordFailuresUnreportable(ev *Evidence) Finding {
	const locus = LocusPerRecordFailuresUnreportable
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	if !ev.Extent.CpipesStatus || ev.Survey == nil {
		return notEvaluable(locus,
			"jetsapi.cpipes_execution_status is not deployed on this database. This locus is not a "+
				"predicate over the execution record at all: an operator's failures have nowhere to go "+
				"when its configuration declares no error channel, and the configuration is the only "+
				"place that is written down")
	}
	if !ev.Survey.Read {
		return notEvaluable(locus, ev.Survey.Note+
			". This locus is a predicate over the configuration rather than over the execution record, so "+
			"an unread configuration leaves it unanswerable rather than absent")
	}
	ops := &ev.Survey.Operators
	if ops.Total() == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"the stored configuration of session %q holds no instance of the five operator types whose "+
				"row-level failures can reach process_errors (map_record, jetrules, ollama, embed, vllm), "+
				"so none of its %d other operators could have reported one in any configuration. %s",
			ev.SessionId, ops.Others, ev.Survey.Note)}
	}
	if ops.Unreportable() == 0 {
		return Finding{Locus: locus, Verdict: Absent, Basis: fmt.Sprintf(
			"all %d instances of the five reporting operator types in the stored configuration of session "+
				"%q declare an error channel. %s", ops.Total(), ev.SessionId, ev.Survey.Note)}
	}
	var parts, subjects []string
	for _, t := range errorChannelOperators {
		if n := ops.WithoutChannel[t]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, t))
			subjects = append(subjects, fmt.Sprintf("%s x%d", t, n))
		}
	}
	basis := fmt.Sprintf(
		"%d of the %d instances of the five reporting operator types in the stored configuration of "+
			"session %q declare no error channel: %s. Their row-level failures are logged and forgotten — "+
			"no process_errors row, no count, nothing in the execution record — so **this locus is "+
			"indistinguishable from a clean run by the record alone**, which is why it is answered from "+
			"the configuration. Over the 47 live configurations the corpus is at its most extreme here: "+
			"243 of 243 map_record instances declare none (F188). %s",
		ops.Unreportable(), ops.Total(), ev.SessionId, strings.Join(parts, ", "), ev.Survey.Note)
	return Finding{Locus: locus, Verdict: Present, Basis: basis, Subjects: subjects}
}

// --- Locus 9 -----------------------------------------------------------------

// writtenNotArrived is section 9.4 row 9: an edge names a destination the data
// is not at.
//
// **It is NotEvaluable here always, and that is the honest verdict rather than
// a gap in the implementation.** Section 9.4 says it in one clause — "it is a
// check against S3 or Postgres rather than a query" — and the consequence is
// that no predicate over jetsapi can answer it. So section 9.5's "eight of the
// nine are computable in SQL" is right that this one is not, and this package
// says so in every report rather than omitting the row: a locus that is never
// asked is not a locus that never fires, and criterion 44's denominators are
// what would lose the difference.
func (c Classifier) writtenNotArrived(ev *Evidence) Finding {
	const locus = LocusWrittenNotArrived
	if ev.Extent == nil || !ev.Extent.ExecutionRecord {
		return c.recordUnavailable(locus)
	}
	conf := []string{observe.ConfounderStagePrefixReused}
	if !ev.Extent.ChannelDetails {
		return notEvaluable(locus,
			"this locus needs an output_location to check, and it lives on "+
				"jetsapi.pipeline_execution_channel_details, which is not deployed on this database "+
				"(I-132). Even where it is, the check is against S3 or Postgres rather than a query over "+
				"jetsapi, so no predicate in this package can answer it",
			append(conf, observe.ConfounderCrossStepJoinUnavailable, observe.ConfounderNoPhysicalLocation)...)
	}
	located, blank := 0, 0
	for i := range ev.Edges {
		if ev.Edges[i].OutputLocation == "" {
			blank++
			continue
		}
		located++
	}
	if blank > 0 {
		conf = append(conf, observe.ConfounderNoPhysicalLocation)
	}
	return notEvaluable(locus, fmt.Sprintf(
		"session %q has %d channel-detail rows naming a physical output_location and %d naming none. "+
			"Whether the data is there is a check against S3 or a database rather than a query over "+
			"jetsapi, and this package does not make it. Two things would qualify the check if it were "+
			"made: an empty output_location under a known output_type means the sink has no physical "+
			"location — a memory channel never leaves the process — and must not be read as a lost write; "+
			"and a later run of the same session overwrites the stage prefix, so *absent* and *superseded* "+
			"are the same observation",
		ev.SessionId, located, blank), conf...)
}

// timeFmt is RFC3339 without the sub-second noise, which is what every basis
// string in observe uses and what an operator reads.
const timeFmt = "2006-01-02T15:04:05Z07:00"
